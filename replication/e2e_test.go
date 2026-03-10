package replication

import (
	"bytes"
	"io"
	"sort"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/awset"
	"github.com/aalpar/crdt/dotcontext"
	"github.com/aalpar/crdt/ewflag"
	"github.com/aalpar/crdt/lwwregister"
	"github.com/aalpar/crdt/mvregister"
	"github.com/aalpar/crdt/ormap"
	"github.com/aalpar/crdt/pncounter"
	"github.com/aalpar/crdt/rwset"
)

func newAWSetCodec() dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotSet]] {
	return dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotSet]]{
		StoreCodec: dotcontext.DotMapCodec[string, *dotcontext.DotSet]{
			KeyCodec:   dotcontext.StringCodec{},
			ValueCodec: dotcontext.DotSetCodec{},
		},
	}
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
func sync(src, dst *replica, codec dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotSet]]) (int, error) {
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
	codec := newAWSetCodec()

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
	codec := newAWSetCodec()

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
	codec := newAWSetCodec()

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
	codec := newAWSetCodec()

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
	codec := newAWSetCodec()

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

// TestE2EDuplicateDeltaDelivery verifies that applying the same delta
// batch twice is idempotent — the second merge has no effect.
func TestE2EDuplicateDeltaDelivery(t *testing.T) {
	c := qt.New(t)
	codec := newAWSetCodec()

	alice := newReplica("alice", "bob")
	bob := newReplica("bob", "alice")

	alice.add("x")
	alice.add("y")

	// Encode all of alice's deltas as if bob has nothing.
	var buf bytes.Buffer
	n, err := WriteDeltaBatch(
		alice.set.State().Context,
		dotcontext.New(),
		alice.store,
		codec,
		&buf,
	)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 2)
	encoded := buf.Bytes()

	applyBatch := func() {
		_, err := ReadDeltaBatch(codec, bytes.NewReader(encoded),
			func(_ dotcontext.Dot, delta dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]) {
				bob.set.Merge(awset.FromCausal[string](delta))
			})
		c.Assert(err, qt.IsNil)
	}

	// First delivery.
	applyBatch()
	c.Assert(sortedElements(bob.set), qt.DeepEquals, []string{"x", "y"})

	// Duplicate delivery — exact same bytes, merge must be idempotent.
	applyBatch()
	c.Assert(sortedElements(bob.set), qt.DeepEquals, []string{"x", "y"})
}

// TestE2EOutOfOrderDelivery verifies that deltas applied in a different
// order than they were generated produce the same converged state.
func TestE2EOutOfOrderDelivery(t *testing.T) {
	c := qt.New(t)
	codec := newAWSetCodec()

	alice := newReplica("alice", "bob")

	// Alice adds three elements, generating dots (alice:1), (alice:2), (alice:3).
	alice.add("a")
	alice.add("b")
	alice.add("c")

	// Encode all deltas.
	var buf bytes.Buffer
	n, err := WriteDeltaBatch(
		alice.set.State().Context,
		dotcontext.New(),
		alice.store,
		codec,
		&buf,
	)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 3)

	// Decode to get individual deltas.
	type taggedDelta struct {
		dot   dotcontext.Dot
		delta dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]
	}
	var deltas []taggedDelta
	ReadDeltaBatch(codec, &buf, func(d dotcontext.Dot, delta dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]]) {
		deltas = append(deltas, taggedDelta{d, delta})
	})
	c.Assert(len(deltas), qt.Equals, 3)

	// Apply in forward order to bob1.
	bob1 := newReplica("bob", "alice")
	for _, d := range deltas {
		bob1.set.Merge(awset.FromCausal[string](d.delta))
	}

	// Apply in reverse order to bob2.
	bob2 := newReplica("bob", "alice")
	for i := len(deltas) - 1; i >= 0; i-- {
		bob2.set.Merge(awset.FromCausal[string](deltas[i].delta))
	}

	// Both must converge to the same state.
	c.Assert(sortedElements(bob1.set), qt.DeepEquals, []string{"a", "b", "c"})
	c.Assert(sortedElements(bob2.set), qt.DeepEquals, []string{"a", "b", "c"})
}

// --- LWWRegister codec ---

type timestampedStringCodec struct{}

func (timestampedStringCodec) Encode(w io.Writer, v lwwregister.Timestamped[string]) error {
	if err := (dotcontext.StringCodec{}).Encode(w, v.Value); err != nil {
		return err
	}
	return (dotcontext.Int64Codec{}).Encode(w, v.Ts)
}

func (timestampedStringCodec) Decode(r io.Reader) (lwwregister.Timestamped[string], error) {
	val, err := (dotcontext.StringCodec{}).Decode(r)
	if err != nil {
		return lwwregister.Timestamped[string]{}, err
	}
	ts, err := (dotcontext.Int64Codec{}).Decode(r)
	if err != nil {
		return lwwregister.Timestamped[string]{}, err
	}
	return lwwregister.Timestamped[string]{Value: val, Ts: ts}, nil
}

func newLWWCodec() dotcontext.CausalCodec[*dotcontext.DotFun[lwwregister.Timestamped[string]]] {
	return dotcontext.CausalCodec[*dotcontext.DotFun[lwwregister.Timestamped[string]]]{
		StoreCodec: dotcontext.DotFunCodec[lwwregister.Timestamped[string]]{
			ValueCodec: timestampedStringCodec{},
		},
	}
}

// TestE2ELWWRegisterAcrossWire verifies that LWWRegister deltas
// (DotFun-based, unlike AWSet's DotMap) survive encode→decode→merge
// and that LWW conflict resolution works across the wire.
func TestE2ELWWRegisterAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newLWWCodec()

	alice := lwwregister.New[string]("alice")
	bob := lwwregister.New[string]("bob")

	// Concurrent writes: alice at ts=100, bob at ts=200.
	aliceDelta := alice.Set("hello", 100)
	bobDelta := bob.Set("world", 200)

	// Encode both deltas.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(lwwregister.FromCausal[string](decodedA))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(lwwregister.FromCausal[string](decodedB))

	// Both converge to "world" (higher timestamp wins).
	aliceVal, aliceTs, aliceOk := alice.Value()
	bobVal, bobTs, bobOk := bob.Value()
	c.Assert(aliceOk, qt.IsTrue)
	c.Assert(bobOk, qt.IsTrue)
	c.Assert(aliceVal, qt.Equals, "world")
	c.Assert(bobVal, qt.Equals, "world")
	c.Assert(aliceTs, qt.Equals, int64(200))
	c.Assert(bobTs, qt.Equals, int64(200))
}

// --- PNCounter codec ---

type counterValueCodec struct{}

func (counterValueCodec) Encode(w io.Writer, v pncounter.CounterValue) error {
	return (dotcontext.Int64Codec{}).Encode(w, v.N)
}

func (counterValueCodec) Decode(r io.Reader) (pncounter.CounterValue, error) {
	n, err := (dotcontext.Int64Codec{}).Decode(r)
	return pncounter.CounterValue{N: n}, err
}

func newCounterCodec() dotcontext.CausalCodec[*dotcontext.DotFun[pncounter.CounterValue]] {
	return dotcontext.CausalCodec[*dotcontext.DotFun[pncounter.CounterValue]]{
		StoreCodec: dotcontext.DotFunCodec[pncounter.CounterValue]{
			ValueCodec: counterValueCodec{},
		},
	}
}

// TestE2EPNCounterAcrossWire verifies that PNCounter deltas
// (DotFun-based with CounterValue lattice) survive encode→decode→merge
// and that concurrent increments sum correctly across the wire.
func TestE2EPNCounterAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newCounterCodec()

	alice := pncounter.New("alice")
	bob := pncounter.New("bob")

	// Concurrent increments: alice +5, bob +3.
	aliceDelta := alice.Increment(5)
	bobDelta := bob.Increment(3)

	// Encode both deltas.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(pncounter.FromCausal(decodedA))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(pncounter.FromCausal(decodedB))

	// Both converge to 8 (sum of contributions).
	c.Assert(alice.Value(), qt.Equals, int64(8))
	c.Assert(bob.Value(), qt.Equals, int64(8))
}

// --- EWFlag codec ---

func newFlagCodec() dotcontext.CausalCodec[*dotcontext.DotSet] {
	return dotcontext.CausalCodec[*dotcontext.DotSet]{
		StoreCodec: dotcontext.DotSetCodec{},
	}
}

// TestE2EEWFlagAcrossWire verifies that EWFlag deltas (DotSet-based)
// survive encode→decode→merge and that enable-wins conflict resolution
// works across the wire.
func TestE2EEWFlagAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newFlagCodec()

	alice := ewflag.New("alice")
	bob := ewflag.New("bob")

	// Both start enabled by syncing alice's enable.
	enableDelta := alice.Enable()
	var buf bytes.Buffer
	c.Assert(codec.Encode(&buf, enableDelta.State()), qt.IsNil)
	decoded, err := codec.Decode(&buf)
	c.Assert(err, qt.IsNil)
	bob.Merge(ewflag.FromCausal(decoded))
	c.Assert(alice.Value(), qt.IsTrue)
	c.Assert(bob.Value(), qt.IsTrue)

	// Concurrent: alice re-enables (new dot), bob disables.
	aliceDelta := alice.Enable()
	bobDelta := bob.Disable()

	// Encode both deltas.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(ewflag.FromCausal(decodedA))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(ewflag.FromCausal(decodedB))

	// Enable wins: both should be enabled.
	c.Assert(alice.Value(), qt.IsTrue, qt.Commentf("enable-wins: flag must be true"))
	c.Assert(bob.Value(), qt.IsTrue, qt.Commentf("enable-wins: flag must be true"))
}

// TestE2EORMapAddWinsAcrossWire verifies that ORMap deltas survive
// encode→decode→merge and that concurrent add + remove of the same
// key resolves to add-wins across the wire.
//
// Uses ORMap[string, *DotSet] — structurally the same wire format
// as AWSet, but exercising ORMap's recursive JoinDotMap merge path.
func TestE2EORMapAddWinsAcrossWire(t *testing.T) {
	c := qt.New(t)
	// ORMap[string, *DotSet] shares AWSet's wire format.
	codec := newAWSetCodec()

	newMap := func(id string) *ormap.ORMap[string, *dotcontext.DotSet] {
		return ormap.New[string, *dotcontext.DotSet](
			dotcontext.ReplicaID(id),
			dotcontext.MergeDotSetStore,
			dotcontext.NewDotSet,
		)
	}

	applyAdd := func(m *ormap.ORMap[string, *dotcontext.DotSet], key string) *ormap.ORMap[string, *dotcontext.DotSet] {
		return m.Apply(key, func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotSet, delta *dotcontext.DotSet) {
			d := ctx.Next(id)
			v.Add(d)
			delta.Add(d)
		})
	}

	alice := newMap("alice")
	bob := newMap("bob")

	// Alice adds key "x". Sync to bob.
	aliceDelta := applyAdd(alice, "x")
	var buf bytes.Buffer
	c.Assert(codec.Encode(&buf, aliceDelta.State()), qt.IsNil)
	decoded, err := codec.Decode(&buf)
	c.Assert(err, qt.IsNil)
	bob.Merge(ormap.FromCausal(decoded, dotcontext.MergeDotSetStore, dotcontext.NewDotSet))
	c.Assert(alice.Has("x"), qt.IsTrue)
	c.Assert(bob.Has("x"), qt.IsTrue)

	// Concurrent: alice adds to "x" again (new dot), bob removes "x".
	aliceDelta2 := applyAdd(alice, "x")
	bobDelta := bob.Remove("x")

	// Encode both.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta2.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(ormap.FromCausal(decodedA, dotcontext.MergeDotSetStore, dotcontext.NewDotSet))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(ormap.FromCausal(decodedB, dotcontext.MergeDotSetStore, dotcontext.NewDotSet))

	// Add wins: "x" must survive on both replicas.
	c.Assert(alice.Has("x"), qt.IsTrue, qt.Commentf("add-wins: x must survive"))
	c.Assert(bob.Has("x"), qt.IsTrue, qt.Commentf("add-wins: x must survive"))
}

// TestE2EORMapNestedMergeAcrossWire verifies that concurrent Apply
// operations on the same key recursively merge nested values
// across the wire — both dots survive under the same key.
func TestE2EORMapNestedMergeAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newAWSetCodec()

	newMap := func(id string) *ormap.ORMap[string, *dotcontext.DotSet] {
		return ormap.New[string, *dotcontext.DotSet](
			dotcontext.ReplicaID(id),
			dotcontext.MergeDotSetStore,
			dotcontext.NewDotSet,
		)
	}

	applyAdd := func(m *ormap.ORMap[string, *dotcontext.DotSet], key string) *ormap.ORMap[string, *dotcontext.DotSet] {
		return m.Apply(key, func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotSet, delta *dotcontext.DotSet) {
			d := ctx.Next(id)
			v.Add(d)
			delta.Add(d)
		})
	}

	alice := newMap("alice")
	bob := newMap("bob")

	// Concurrent: both add to key "x" independently.
	aliceDelta := applyAdd(alice, "x")
	bobDelta := applyAdd(bob, "x")

	// Encode both.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(ormap.FromCausal(decodedA, dotcontext.MergeDotSetStore, dotcontext.NewDotSet))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(ormap.FromCausal(decodedB, dotcontext.MergeDotSetStore, dotcontext.NewDotSet))

	// Both dots survive under key "x" (recursive DotSet join).
	aliceV, aliceOk := alice.Get("x")
	bobV, bobOk := bob.Get("x")
	c.Assert(aliceOk, qt.IsTrue)
	c.Assert(bobOk, qt.IsTrue)
	c.Assert(aliceV.Len(), qt.Equals, 2, qt.Commentf("both dots must survive"))
	c.Assert(bobV.Len(), qt.Equals, 2, qt.Commentf("both dots must survive"))
}

// TestE2ELWWTiebreakAcrossWire verifies that same-timestamp tiebreak
// (lexicographic on replica ID) works across the wire.
func TestE2ELWWTiebreakAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newLWWCodec()

	alice := lwwregister.New[string]("alice")
	bob := lwwregister.New[string]("bob")

	// Same timestamp: tiebreak on replica ID ("bob" > "alice").
	aliceDelta := alice.Set("from-alice", 100)
	bobDelta := bob.Set("from-bob", 100)

	// Encode both deltas.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(lwwregister.FromCausal[string](decodedA))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(lwwregister.FromCausal[string](decodedB))

	// Both converge to "from-bob" (lexicographic tiebreak: "bob" > "alice").
	aliceVal, _, _ := alice.Value()
	bobVal, _, _ := bob.Value()
	c.Assert(aliceVal, qt.Equals, "from-bob", qt.Commentf("tiebreak: bob > alice"))
	c.Assert(bobVal, qt.Equals, "from-bob", qt.Commentf("tiebreak: bob > alice"))
}

// TestE2EGCThenWriteDeltaBatch verifies the full lifecycle:
// add deltas → ack peers → GC → WriteDeltaBatch handles the post-GC
// store gracefully (Missing still returns ranges, but Fetch finds
// nothing because GC already removed them).
func TestE2EGCThenWriteDeltaBatch(t *testing.T) {
	c := qt.New(t)
	codec := newAWSetCodec()

	alice := newReplica("alice", "bob")
	bob := newReplica("bob", "alice")

	// Alice adds two elements.
	alice.add("x")
	alice.add("y")
	c.Assert(alice.store.Len(), qt.Equals, 2)

	// Sync alice → bob.
	n, err := sync(alice, bob, codec)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 2)

	// Bob has acked. GC should remove alice's deltas.
	removed := GC(alice.store, alice.tracker)
	c.Assert(removed, qt.Equals, 2)
	c.Assert(alice.store.Len(), qt.Equals, 0)

	// Now a new peer "carol" joins, knowing nothing.
	alice.tracker.AddPeer("carol", nil)
	carolCC := dotcontext.New() // carol knows nothing

	// WriteDeltaBatch: carol is missing alice's dots (a:1,a:2) per
	// Missing(), but the DeltaStore is empty after GC. WriteDeltaBatch
	// should write zero deltas without error.
	var buf bytes.Buffer
	nWritten, err := WriteDeltaBatch(
		alice.set.State().Context,
		carolCC,
		alice.store,
		codec,
		&buf,
	)
	c.Assert(err, qt.IsNil)
	c.Assert(nWritten, qt.Equals, 0)
}

// TestE2EPNCounterDecrementAcrossWire verifies that negative contributions
// (decrements) survive encode→decode→merge and produce the correct sum.
func TestE2EPNCounterDecrementAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newCounterCodec()

	alice := pncounter.New("alice")
	bob := pncounter.New("bob")

	// Concurrent: alice +10, bob -3.
	aliceDelta := alice.Increment(10)
	bobDelta := bob.Decrement(3)

	// Encode both deltas.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(pncounter.FromCausal(decodedA))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(pncounter.FromCausal(decodedB))

	// Both converge to 7 (10 + (-3)).
	c.Assert(alice.Value(), qt.Equals, int64(7))
	c.Assert(bob.Value(), qt.Equals, int64(7))
}

// --- MVRegister codec ---

type mvrValueStringCodec struct{}

func (mvrValueStringCodec) Encode(w io.Writer, v mvregister.Entry[string]) error {
	return (dotcontext.StringCodec{}).Encode(w, v.Val)
}

func (mvrValueStringCodec) Decode(r io.Reader) (mvregister.Entry[string], error) {
	s, err := (dotcontext.StringCodec{}).Decode(r)
	return mvregister.Entry[string]{Val: s}, err
}

func newMVRCodec() dotcontext.CausalCodec[*dotcontext.DotFun[mvregister.Entry[string]]] {
	return dotcontext.CausalCodec[*dotcontext.DotFun[mvregister.Entry[string]]]{
		StoreCodec: dotcontext.DotFunCodec[mvregister.Entry[string]]{
			ValueCodec: mvrValueStringCodec{},
		},
	}
}

// TestE2EMVRegisterAcrossWire verifies that MVRegister deltas
// (DotFun-based, like LWWRegister but with multi-value semantics)
// survive encode→decode→merge and that concurrent writes produce
// multiple values rather than picking a winner.
func TestE2EMVRegisterAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newMVRCodec()

	alice := mvregister.New[string]("alice")
	bob := mvregister.New[string]("bob")

	// Concurrent writes.
	aliceDelta := alice.Write("from-alice")
	bobDelta := bob.Write("from-bob")

	// Encode both deltas.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(mvregister.FromCausal[string](decodedA))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(mvregister.FromCausal[string](decodedB))

	// Both converge to both values (no winner — all preserved).
	expected := []string{"from-alice", "from-bob"}
	for _, r := range []*mvregister.MVRegister[string]{alice, bob} {
		vals := r.Values()
		sort.Strings(vals)
		c.Assert(vals, qt.DeepEquals, expected)
	}
}

// --- RWSet codec ---

// presenceCodec encodes rwset.Presence as a single byte (1=active, 0=inactive).
type presenceCodec struct{}

func (presenceCodec) Encode(w io.Writer, v rwset.Presence) error {
	var b byte
	if v.Active {
		b = 1
	}
	_, err := w.Write([]byte{b})
	return err
}

func (presenceCodec) Decode(r io.Reader) (rwset.Presence, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return rwset.Presence{}, err
	}
	return rwset.Presence{Active: buf[0] == 1}, nil
}

func newRWSetCodec() dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotFun[rwset.Presence]]] {
	return dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotFun[rwset.Presence]]]{
		StoreCodec: dotcontext.DotMapCodec[string, *dotcontext.DotFun[rwset.Presence]]{
			KeyCodec: dotcontext.StringCodec{},
			ValueCodec: dotcontext.DotFunCodec[rwset.Presence]{
				ValueCodec: presenceCodec{},
			},
		},
	}
}

// TestE2ERWSetRemoveWinsAcrossWire verifies that RWSet deltas
// (DotMap[E, *DotFun[Presence]] — the deepest wire nesting) survive
// encode→decode→merge and that remove-wins conflict resolution
// works across the wire.
func TestE2ERWSetRemoveWinsAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newRWSetCodec()

	alice := rwset.New[string]("alice")
	bob := rwset.New[string]("bob")

	// Both start with "x" by syncing alice's add.
	aliceAdd := alice.Add("x")
	var buf bytes.Buffer
	c.Assert(codec.Encode(&buf, aliceAdd.State()), qt.IsNil)
	decoded, err := codec.Decode(&buf)
	c.Assert(err, qt.IsNil)
	bob.Merge(rwset.FromCausal[string](decoded))
	c.Assert(alice.Has("x"), qt.IsTrue)
	c.Assert(bob.Has("x"), qt.IsTrue)

	// Concurrent: alice re-adds "x" (new dot, Active), bob removes "x" (new dot, Inactive).
	aliceDelta := alice.Add("x")
	bobDelta := bob.Remove("x")

	// Encode both deltas.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(rwset.FromCausal[string](decodedA))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(rwset.FromCausal[string](decodedB))

	// Remove wins: bob's tombstone dot survives alongside alice's add dot.
	// Has requires ALL dots to be Active, so the tombstone wins.
	c.Assert(alice.Has("x"), qt.IsFalse, qt.Commentf("remove-wins: x must be absent"))
	c.Assert(bob.Has("x"), qt.IsFalse, qt.Commentf("remove-wins: x must be absent"))
}

// TestE2ERWSetConcurrentAddsAcrossWire verifies that concurrent adds
// from different replicas both survive across the wire — both dots are
// Active so the element is present.
func TestE2ERWSetConcurrentAddsAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newRWSetCodec()

	alice := rwset.New[string]("alice")
	bob := rwset.New[string]("bob")

	// Concurrent adds of the same element.
	aliceDelta := alice.Add("x")
	bobDelta := bob.Add("x")

	// Encode both deltas.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(rwset.FromCausal[string](decodedA))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(rwset.FromCausal[string](decodedB))

	// Both dots are Active, so element is present.
	c.Assert(alice.Has("x"), qt.IsTrue, qt.Commentf("concurrent adds: x must be present"))
	c.Assert(bob.Has("x"), qt.IsTrue, qt.Commentf("concurrent adds: x must be present"))
}

// TestE2ERWSetReAddAfterRemoveAcrossWire verifies the add→remove→re-add
// cycle across the wire: after a remove-wins resolution, a subsequent
// add from the same replica restores the element.
func TestE2ERWSetReAddAfterRemoveAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newRWSetCodec()

	alice := rwset.New[string]("alice")
	bob := rwset.New[string]("bob")

	// Alice adds "x", syncs to bob.
	d1 := alice.Add("x")
	var buf1 bytes.Buffer
	c.Assert(codec.Encode(&buf1, d1.State()), qt.IsNil)
	dec1, err := codec.Decode(&buf1)
	c.Assert(err, qt.IsNil)
	bob.Merge(rwset.FromCausal[string](dec1))

	// Alice removes "x", syncs to bob.
	d2 := alice.Remove("x")
	var buf2 bytes.Buffer
	c.Assert(codec.Encode(&buf2, d2.State()), qt.IsNil)
	dec2, err := codec.Decode(&buf2)
	c.Assert(err, qt.IsNil)
	bob.Merge(rwset.FromCausal[string](dec2))
	c.Assert(bob.Has("x"), qt.IsFalse)

	// Alice re-adds "x", syncs to bob — new dot supersedes the tombstone.
	d3 := alice.Add("x")
	var buf3 bytes.Buffer
	c.Assert(codec.Encode(&buf3, d3.State()), qt.IsNil)
	dec3, err := codec.Decode(&buf3)
	c.Assert(err, qt.IsNil)
	bob.Merge(rwset.FromCausal[string](dec3))

	c.Assert(alice.Has("x"), qt.IsTrue, qt.Commentf("re-add must restore element"))
	c.Assert(bob.Has("x"), qt.IsTrue, qt.Commentf("re-add must restore element on remote"))
}
