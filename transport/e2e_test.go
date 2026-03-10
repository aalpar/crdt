package transport

import (
	"bytes"
	"net"
	"sort"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/awset"
	"github.com/aalpar/crdt/dotcontext"
	"github.com/aalpar/crdt/replication"
)

// awsetCodec returns a CausalCodec for AWSet[string] deltas.
func awsetCodec() dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotSet]] {
	return dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotSet]]{
		StoreCodec: dotcontext.DotMapCodec[string, *dotcontext.DotSet]{
			KeyCodec:   dotcontext.StringCodec{},
			ValueCodec: dotcontext.DotSetCodec{},
		},
	}
}

// netReplica bundles AWSet state, delta storage, peer tracking, and a
// transport into a single node that replicates over the network.
type netReplica struct {
	id      string
	set     *awset.AWSet[string]
	store   *dotcontext.DeltaStore[dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]]
	tracker *replication.PeerTracker
	tr      *Transport
	codec   dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotSet]]

	mu      sync.Mutex
	merged  chan struct{} // signaled after each OnDeltaBatch merge
	acked   chan struct{} // signaled after each OnAck processing
}

func newNetReplica(id string) *netReplica {
	r := &netReplica{
		id:      id,
		set:     awset.New[string](dotcontext.ReplicaID(id)),
		store:   dotcontext.NewDeltaStore[dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]](),
		tracker: replication.NewPeerTracker(),
		codec:   awsetCodec(),
		merged:  make(chan struct{}, 16),
		acked:   make(chan struct{}, 16),
	}
	r.tr = New(id, r)
	return r
}

// add inserts an element into the local AWSet, stores the delta,
// encodes a delta batch for each connected peer, and sends it.
func (r *netReplica) add(t *testing.T, elem string) {
	t.Helper()

	// Mutate and encode under lock — handlers also access set/store.
	r.mu.Lock()
	delta := r.set.Add(elem)
	state := delta.State()

	// Extract the single new dot from the delta and store it.
	state.Store.Range(func(_ string, ds *dotcontext.DotSet) bool {
		ds.Range(func(d dotcontext.Dot) bool {
			r.store.Add(d, state)
			return false
		})
		return false
	})

	// Build encoded payloads while holding the lock.
	type pending struct {
		peerID  string
		payload []byte
	}
	var sends []pending
	for _, peerID := range r.tr.Peers() {
		var buf bytes.Buffer
		n, err := replication.WriteDeltaBatch(
			r.set.State().Context,
			dotcontext.New(), // empty CC: send everything; merge is idempotent
			r.store,
			r.codec,
			&buf,
		)
		if err != nil {
			r.mu.Unlock()
			t.Fatalf("WriteDeltaBatch to %s: %v", peerID, err)
		}
		if n > 0 {
			sends = append(sends, pending{peerID, buf.Bytes()})
		}
	}
	r.mu.Unlock()

	// Send outside the lock — Transport.SendDeltaBatch has its own mutex.
	for _, s := range sends {
		if err := r.tr.SendDeltaBatch(s.peerID, s.payload); err != nil {
			t.Fatalf("SendDeltaBatch to %s: %v", s.peerID, err)
		}
	}
}

// elements returns sorted elements from the AWSet (under lock).
func (r *netReplica) elements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	elems := r.set.Elements()
	sort.Strings(elems)
	return elems
}

// --- Handler interface ---

func (r *netReplica) OnPeerConnect(peerID string) {
	r.mu.Lock()
	r.tracker.AddPeer(dotcontext.ReplicaID(peerID), nil)
	cc := r.set.State().Context.Clone()
	r.mu.Unlock()

	// Send our causal context as an Ack so the peer knows what we have.
	var buf bytes.Buffer
	if err := (dotcontext.CausalContextCodec{}).Encode(&buf, cc); err != nil {
		return
	}
	r.tr.SendAck(peerID, buf.Bytes())
}

func (r *netReplica) OnAck(peerID string, payload []byte) {
	cc, err := (dotcontext.CausalContextCodec{}).Decode(bytes.NewReader(payload))
	if err != nil {
		return
	}

	r.mu.Lock()
	r.tracker.Ack(dotcontext.ReplicaID(peerID), cc)

	// Compute and send missing deltas.
	var buf bytes.Buffer
	n, err := replication.WriteDeltaBatch(
		r.set.State().Context,
		cc,
		r.store,
		r.codec,
		&buf,
	)
	r.mu.Unlock()

	if err != nil || n == 0 {
		select {
		case r.acked <- struct{}{}:
		default:
		}
		return
	}

	r.tr.SendDeltaBatch(peerID, buf.Bytes())

	select {
	case r.acked <- struct{}{}:
	default:
	}
}

func (r *netReplica) OnDeltaBatch(peerID string, payload []byte) {
	r.mu.Lock()
	_, err := replication.ReadDeltaBatch(r.codec, bytes.NewReader(payload),
		func(_ dotcontext.Dot, delta dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]) {
			r.set.Merge(awset.FromCausal[string](delta))
		})
	cc := r.set.State().Context.Clone()
	r.mu.Unlock()

	if err != nil {
		return
	}

	// Send Ack with our updated context.
	var buf bytes.Buffer
	if err := (dotcontext.CausalContextCodec{}).Encode(&buf, cc); err != nil {
		return
	}
	r.tr.SendAck(peerID, buf.Bytes())

	select {
	case r.merged <- struct{}{}:
	default:
	}
}

func (r *netReplica) OnPeerDisconnect(peerID string, _ error) {
	r.mu.Lock()
	r.tracker.RemovePeer(dotcontext.ReplicaID(peerID))
	r.mu.Unlock()
}

// TestE2EAWSetOverTransport wires AWSet + DeltaStore + PeerTracker +
// Transport into a full replication cycle over TCP and verifies convergence.
func TestE2EAWSetOverTransport(t *testing.T) {
	c := qt.New(t)

	alice := newNetReplica("alice")
	bob := newNetReplica("bob")

	// Start alice listening.
	ln, err := listenLocal(t)
	c.Assert(err, qt.IsNil)
	go alice.tr.Listen(ln)

	// Bob connects to alice.
	c.Assert(bob.tr.Connect(ln.Addr().String()), qt.IsNil)
	t.Cleanup(func() {
		alice.tr.Close()
		bob.tr.Close()
	})

	// Wait for the initial Ack exchange to complete.
	// OnPeerConnect sends an Ack, the other side processes it in OnAck.
	// Both sides fire acked once each.
	<-alice.acked
	<-bob.acked

	// --- Alice adds "x" and pushes to bob ---
	alice.add(t, "x")

	// Wait for bob to merge.
	<-bob.merged

	c.Assert(bob.elements(), qt.DeepEquals, []string{"x"})

	// Wait for alice to receive bob's ack (triggered by bob's OnDeltaBatch).
	<-alice.acked

	// --- Bob adds "y" and pushes to alice ---
	bob.add(t, "y")
	<-alice.merged
	<-bob.acked

	// Both replicas converge on {"x", "y"}.
	c.Assert(alice.elements(), qt.DeepEquals, []string{"x", "y"})
	c.Assert(bob.elements(), qt.DeepEquals, []string{"x", "y"})
}

// TestE2EBidirectionalConcurrent verifies that concurrent mutations on
// both replicas propagate and converge via the transport layer.
func TestE2EBidirectionalConcurrent(t *testing.T) {
	c := qt.New(t)

	alice := newNetReplica("alice")
	bob := newNetReplica("bob")

	ln, err := listenLocal(t)
	c.Assert(err, qt.IsNil)
	go alice.tr.Listen(ln)

	c.Assert(bob.tr.Connect(ln.Addr().String()), qt.IsNil)
	t.Cleanup(func() {
		alice.tr.Close()
		bob.tr.Close()
	})

	// Wait for initial handshake acks.
	<-alice.acked
	<-bob.acked

	// Both add elements concurrently.
	alice.add(t, "a")
	bob.add(t, "b")

	// Wait for both merges and the subsequent acks.
	<-alice.merged
	<-bob.merged
	<-alice.acked
	<-bob.acked

	// Both should converge to {"a", "b"}.
	c.Assert(alice.elements(), qt.DeepEquals, []string{"a", "b"})
	c.Assert(bob.elements(), qt.DeepEquals, []string{"a", "b"})
}

func listenLocal(t *testing.T) (*net.TCPListener, error) {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	return net.ListenTCP("tcp", addr)
}
