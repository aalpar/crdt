package replication

import (
	"bytes"
	"io"
	"sort"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/awset"
	"github.com/aalpar/crdt/dotcontext"
)

// awsetDeltaCodec encodes AWSet deltas (Causal[*DotMap[string, *DotSet]])
// for transmission over the replication pipeline.
type awsetDeltaCodec struct {
	inner dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotSet]]
}

func newAWSetDeltaCodec() awsetDeltaCodec {
	return awsetDeltaCodec{
		inner: dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotSet]]{
			StoreCodec: dotcontext.DotMapCodec[string, *dotcontext.DotSet]{
				KeyCodec:   dotcontext.StringCodec{},
				ValueCodec: dotcontext.DotSetCodec{},
			},
		},
	}
}

func (c awsetDeltaCodec) Encode(w io.Writer, v dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]) error {
	return c.inner.Encode(w, v)
}

func (c awsetDeltaCodec) Decode(r io.Reader) (dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]], error) {
	return c.inner.Decode(r)
}

// replica bundles the state a single node maintains for replication.
type replica struct {
	id      dotcontext.ReplicaID
	set     *awset.AWSet[string]
	store   *dotcontext.DeltaStore[dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]]
	tracker *PeerTracker
}

func newReplica(id string, peers ...string) *replica {
	r := &replica{
		id:      dotcontext.ReplicaID(id),
		set:     awset.New[string](dotcontext.ReplicaID(id)),
		store:   dotcontext.NewDeltaStore[dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]](),
		tracker: NewPeerTracker(),
	}
	for _, p := range peers {
		r.tracker.AddPeer(dotcontext.ReplicaID(p), nil)
	}
	return r
}

// add inserts an element and buffers the delta for replication.
func (r *replica) add(elem string) {
	delta := r.set.Add(elem)
	state := delta.State()
	// The delta contains exactly one new dot; extract it.
	var d dotcontext.Dot
	state.Store.Range(func(_ string, ds *dotcontext.DotSet) bool {
		ds.Range(func(dot dotcontext.Dot) bool {
			d = dot
			return false
		})
		return false
	})
	r.store.Add(d, state)
}

// sync sends missing deltas from src to dst over an in-memory wire.
// Returns the number of deltas transferred.
func sync(src, dst *replica, codec awsetDeltaCodec) (int, error) {
	var buf bytes.Buffer
	n, err := WriteDeltaBatch(
		src.set.State().Context,
		dst.set.State().Context,
		src.store,
		codec,
		&buf,
	)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}

	nRead, err := ReadDeltaBatch(codec, &buf, func(_ dotcontext.Dot, delta dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]) {
		dst.set.Merge(awset.FromCausal[string](delta))
	})
	if err != nil {
		return 0, err
	}

	// Ack: tell src that dst now has everything src sent.
	src.tracker.Ack(dst.id, dst.set.State().Context.Clone())

	return nRead, nil
}

func sortedElements(s *awset.AWSet[string]) []string {
	elems := s.Elements()
	sort.Strings(elems)
	return elems
}

// TestE2EReplicationCycle exercises the full pipeline:
//
//	mutate → store delta → WriteDeltaBatch → ReadDeltaBatch → Merge → Ack → GC
//
// Two replicas perform concurrent mutations, sync bidirectionally,
// and verify convergence. Then GC cleans up acknowledged deltas.
func TestE2EReplicationCycle(t *testing.T) {
	c := qt.New(t)
	codec := newAWSetDeltaCodec()

	alice := newReplica("alice", "bob")
	bob := newReplica("bob", "alice")

	// --- Phase 1: Concurrent mutations ---
	alice.add("x")
	alice.add("y")
	bob.add("y") // concurrent add of same element
	bob.add("z")

	c.Assert(sortedElements(alice.set), qt.DeepEquals, []string{"x", "y"})
	c.Assert(sortedElements(bob.set), qt.DeepEquals, []string{"y", "z"})

	// --- Phase 2: Sync alice → bob ---
	n, err := sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 2) // bob was missing alice's two adds

	// Bob now has {x, y, z}. "y" has dots from both replicas (add-wins).
	c.Assert(sortedElements(bob.set), qt.DeepEquals, []string{"x", "y", "z"})

	// --- Phase 3: Sync bob → alice ---
	n, err = sync(bob, alice, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 2) // alice was missing bob's two adds

	// Both converge.
	c.Assert(sortedElements(alice.set), qt.DeepEquals, []string{"x", "y", "z"})
	c.Assert(sortedElements(bob.set), qt.DeepEquals, []string{"x", "y", "z"})

	// --- Phase 4: Fully synced → no deltas transferred ---
	n, err = sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)

	// --- Phase 5: GC after mutual acknowledgment ---
	// Alice's tracker knows bob has acked everything.
	removed := GC(alice.store, alice.tracker)
	c.Assert(removed, qt.Equals, 2) // alice's two original deltas
	c.Assert(alice.store.Len(), qt.Equals, 0)
}

// TestE2EThreeReplicaMesh exercises a three-node full-mesh sync.
// Each replica mutates independently, then all pairs sync until
// convergence, followed by GC.
func TestE2EThreeReplicaMesh(t *testing.T) {
	c := qt.New(t)
	codec := newAWSetDeltaCodec()

	r1 := newReplica("r1", "r2", "r3")
	r2 := newReplica("r2", "r1", "r3")
	r3 := newReplica("r3", "r1", "r2")

	// Each replica adds a unique element.
	r1.add("a")
	r2.add("b")
	r3.add("c")

	// Full mesh sync: r1→r2, r1→r3, r2→r1, r2→r3, r3→r1, r3→r2.
	pairs := [][2]*replica{{r1, r2}, {r1, r3}, {r2, r1}, {r2, r3}, {r3, r1}, {r3, r2}}
	for _, pair := range pairs {
		_, err := sync(pair[0], pair[1], codec)
		c.Assert(err, qt.IsNil)
	}

	// All three converge on {a, b, c}.
	expected := []string{"a", "b", "c"}
	c.Assert(sortedElements(r1.set), qt.DeepEquals, expected)
	c.Assert(sortedElements(r2.set), qt.DeepEquals, expected)
	c.Assert(sortedElements(r3.set), qt.DeepEquals, expected)

	// Second sync round: nothing to transfer.
	for _, pair := range pairs {
		n, err := sync(pair[0], pair[1], codec)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 0)
	}

	// GC: each replica's single delta should be collectible
	// once all peers have acked.
	for _, r := range []*replica{r1, r2, r3} {
		removed := GC(r.store, r.tracker)
		c.Assert(removed, qt.Equals, 1)
		c.Assert(r.store.Len(), qt.Equals, 0)
	}
}

// TestE2EAddWinsAcrossWire verifies that the add-wins conflict
// resolution survives the encode→decode→merge pipeline.
// Alice adds "x", Bob concurrently removes "x", sync resolves to add-wins.
func TestE2EAddWinsAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newAWSetDeltaCodec()

	alice := newReplica("alice", "bob")
	bob := newReplica("bob", "alice")

	// Both start with "x" by syncing alice's add.
	alice.add("x")
	_, err := sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(bob.set.Has("x"), qt.IsTrue)

	// Concurrent: alice re-adds "x" (new dot), bob removes "x".
	alice.add("x")
	bob.set.Remove("x")
	c.Assert(alice.set.Has("x"), qt.IsTrue)
	c.Assert(bob.set.Has("x"), qt.IsFalse)

	// Sync alice → bob: alice's new add-dot is unobserved by bob's
	// remove context, so it survives → add wins.
	_, err = sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(bob.set.Has("x"), qt.IsTrue, qt.Commentf("add-wins: x must survive"))

	// Sync bob → alice: bob's remove delta (if any) won't affect
	// alice's newer dot.
	_, err = sync(bob, alice, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(alice.set.Has("x"), qt.IsTrue, qt.Commentf("add-wins: x must survive on alice too"))
}

// TestE2EIncrementalSync verifies that multiple sync rounds correctly
// transfer only new deltas each time.
func TestE2EIncrementalSync(t *testing.T) {
	c := qt.New(t)
	codec := newAWSetDeltaCodec()

	alice := newReplica("alice", "bob")
	bob := newReplica("bob", "alice")

	// Round 1: alice adds "a".
	alice.add("a")
	n, err := sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
	c.Assert(bob.set.Has("a"), qt.IsTrue)

	// Round 2: alice adds "b". Only "b" should transfer.
	alice.add("b")
	n, err = sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
	c.Assert(bob.set.Has("b"), qt.IsTrue)

	// Round 3: alice adds "c" and "d". Both should transfer.
	alice.add("c")
	alice.add("d")
	n, err = sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 2)
	c.Assert(sortedElements(bob.set), qt.DeepEquals, []string{"a", "b", "c", "d"})

	// Round 4: nothing new.
	n, err = sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)
}

// TestE2ERemoveDeltaAcrossWire verifies that a remove delta (empty store,
// non-empty context) survives encode→decode→merge and produces the
// correct result on the receiving replica.
//
// Remove deltas generate no new dot, so they can't be keyed in the
// DeltaStore or discovered via Missing()/Fetch(). This test encodes
// the delta directly to verify the wire format handles the empty-store
// case and the merge correctly removes the element.
func TestE2ERemoveDeltaAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newAWSetDeltaCodec()

	alice := newReplica("alice", "bob")
	bob := newReplica("bob", "alice")

	// Both replicas start with "x" and "y".
	alice.add("x")
	alice.add("y")
	_, err := sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(bob.set.Has("x"), qt.IsTrue)
	c.Assert(bob.set.Has("y"), qt.IsTrue)

	// Bob removes "y". The delta has an empty store and a context
	// containing y's dots — encoding this exercises the empty-DotMap
	// wire path.
	removeDelta := bob.set.Remove("y")
	c.Assert(bob.set.Has("y"), qt.IsFalse)

	// Encode the remove delta directly (bypassing DeltaStore).
	var buf bytes.Buffer
	err = codec.Encode(&buf, removeDelta.State())
	c.Assert(err, qt.IsNil)

	// Decode on Alice's side.
	decoded, err := codec.Decode(&buf)
	c.Assert(err, qt.IsNil)

	// The decoded delta should have an empty store but a context that
	// contains the dots that were in y's DotSet.
	c.Assert(decoded.Store.Len(), qt.Equals, 0)
	c.Assert(len(decoded.Context.ReplicaIDs()) > 0, qt.IsTrue)

	// Merge into Alice — "y" should be removed, "x" survives.
	alice.set.Merge(awset.FromCausal[string](decoded))
	c.Assert(alice.set.Has("x"), qt.IsTrue, qt.Commentf("x must survive the remove of y"))
	c.Assert(alice.set.Has("y"), qt.IsFalse, qt.Commentf("y must be removed"))
}
