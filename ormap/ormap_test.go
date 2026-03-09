package ormap

import (
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

// Helpers for ORMap[string, *DotSet] — a map of sets.

func joinDotSet(a, b dotcontext.Causal[*dotcontext.DotSet]) dotcontext.Causal[*dotcontext.DotSet] {
	return dotcontext.JoinDotSet(a, b)
}

func newSetMap(id dotcontext.ReplicaID) *ORMap[string, *dotcontext.DotSet] {
	return New[string, *dotcontext.DotSet](id, joinDotSet, dotcontext.NewDotSet)
}

func addDot(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotSet, delta *dotcontext.DotSet) {
	d := ctx.Next(id)
	v.Add(d)
	delta.Add(d)
}

func TestNewEmpty(t *testing.T) {
	c := qt.New(t)
	m := newSetMap("a")
	c.Assert(m.Len(), qt.Equals, 0)
	c.Assert(m.Has("x"), qt.IsFalse)
	c.Assert(m.Keys(), qt.HasLen, 0)
}

func TestApplyCreatesKey(t *testing.T) {
	c := qt.New(t)
	m := newSetMap("a")
	m.Apply("x", addDot)

	c.Assert(m.Has("x"), qt.IsTrue)
	c.Assert(m.Len(), qt.Equals, 1)
	v, ok := m.Get("x")
	c.Assert(ok, qt.IsTrue)
	c.Assert(v.Len(), qt.Equals, 1)
}

func TestApplyMultipleKeys(t *testing.T) {
	c := qt.New(t)
	m := newSetMap("a")
	m.Apply("x", addDot)
	m.Apply("y", addDot)
	m.Apply("z", addDot)

	keys := m.Keys()
	slices.Sort(keys)
	c.Assert(keys, qt.DeepEquals, []string{"x", "y", "z"})
}

func TestApplyAccumulatesDots(t *testing.T) {
	c := qt.New(t)
	m := newSetMap("a")
	m.Apply("x", addDot)
	m.Apply("x", addDot)

	v, _ := m.Get("x")
	c.Assert(v.Len(), qt.Equals, 2)
}

func TestApplyReturnsValidDelta(t *testing.T) {
	c := qt.New(t)
	m := newSetMap("a")
	delta := m.Apply("x", addDot)

	c.Assert(delta.Has("x"), qt.IsTrue)
	c.Assert(delta.Len(), qt.Equals, 1)
}

func TestRemove(t *testing.T) {
	c := qt.New(t)
	m := newSetMap("a")
	m.Apply("x", addDot)
	m.Apply("y", addDot)
	m.Remove("x")

	c.Assert(m.Has("x"), qt.IsFalse)
	c.Assert(m.Has("y"), qt.IsTrue)
}

func TestRemoveAbsent(t *testing.T) {
	c := qt.New(t)
	m := newSetMap("a")
	m.Remove("ghost") // should not panic
	c.Assert(m.Len(), qt.Equals, 0)
}

// --- Delta propagation ---

func TestMergeDelta(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")

	delta := a.Apply("x", addDot)
	b.Merge(delta)

	c.Assert(b.Has("x"), qt.IsTrue)
}

func TestMergeRemoveDelta(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")

	addDelta := a.Apply("x", addDot)
	b.Merge(addDelta)

	rmDelta := a.Remove("x")
	b.Merge(rmDelta)

	c.Assert(b.Has("x"), qt.IsFalse)
}

// --- Add-wins semantics ---

func TestAddWinsConcurrent(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")

	addDelta := a.Apply("x", addDot)
	b.Merge(addDelta)

	rmDelta := a.Remove("x")
	addDelta2 := b.Apply("x", addDot)

	a.Merge(addDelta2)
	b.Merge(rmDelta)

	c.Assert(a.Has("x"), qt.IsTrue)
	c.Assert(b.Has("x"), qt.IsTrue)
}

func TestConcurrentApplyDifferentKeys(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")

	da := a.Apply("x", addDot)
	db := b.Apply("y", addDot)

	a.Merge(db)
	b.Merge(da)

	for _, m := range []*ORMap[string, *dotcontext.DotSet]{a, b} {
		keys := m.Keys()
		slices.Sort(keys)
		c.Assert(keys, qt.DeepEquals, []string{"x", "y"})
	}
}

func TestConcurrentApplySameKey(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")

	da := a.Apply("x", addDot)
	db := b.Apply("x", addDot)

	a.Merge(db)
	b.Merge(da)

	for _, m := range []*ORMap[string, *dotcontext.DotSet]{a, b} {
		v, ok := m.Get("x")
		c.Assert(ok, qt.IsTrue)
		c.Assert(v.Len(), qt.Equals, 2)
	}
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	a.Apply("x", addDot)

	snapshot := newSetMap("x")
	snapshot.Merge(a)

	a.Merge(snapshot)
	a.Merge(snapshot)

	c.Assert(a.Len(), qt.Equals, 1)
}

func TestMergeCommutative(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")
	a.Apply("x", addDot)
	b.Apply("y", addDot)

	ab := newSetMap("x")
	ab.Merge(a)
	ab.Merge(b)

	ba := newSetMap("x")
	ba.Merge(b)
	ba.Merge(a)

	abKeys := ab.Keys()
	baKeys := ba.Keys()
	slices.Sort(abKeys)
	slices.Sort(baKeys)
	c.Assert(abKeys, qt.DeepEquals, baKeys)
}

func TestDeltaIncrementalEqualsFullMerge(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	d1 := a.Apply("x", addDot)
	d2 := a.Apply("y", addDot)

	inc := newSetMap("b")
	inc.Merge(d1)
	inc.Merge(d2)

	full := newSetMap("b")
	full.Merge(a)

	incKeys := inc.Keys()
	fullKeys := full.Keys()
	slices.Sort(incKeys)
	slices.Sort(fullKeys)
	c.Assert(incKeys, qt.DeepEquals, fullKeys)
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")
	x := newSetMap("c")

	da := a.Apply("x", addDot)
	db := b.Apply("y", addDot)
	dx := x.Apply("z", addDot)

	a.Merge(db)
	a.Merge(dx)
	b.Merge(da)
	b.Merge(dx)
	x.Merge(da)
	x.Merge(db)

	want := []string{"x", "y", "z"}
	for _, m := range []*ORMap[string, *dotcontext.DotSet]{a, b, x} {
		keys := m.Keys()
		slices.Sort(keys)
		c.Assert(keys, qt.DeepEquals, want)
	}
}

// --- Merge associativity ---

func TestMergeAssociative(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")
	x := newSetMap("c")

	a.Apply("x", addDot)
	a.Apply("y", addDot)
	b.Apply("y", addDot)
	b.Apply("z", addDot)
	x.Apply("x", addDot)
	x.Remove("x")
	x.Apply("w", addDot)

	// (a ⊔ b) ⊔ c
	ab := newSetMap("ab")
	ab.Merge(a)
	ab.Merge(b)
	abc := newSetMap("abc")
	abc.Merge(ab)
	abc.Merge(x)

	// a ⊔ (b ⊔ c)
	bc := newSetMap("bc")
	bc.Merge(b)
	bc.Merge(x)
	abc2 := newSetMap("abc2")
	abc2.Merge(a)
	abc2.Merge(bc)

	abcKeys := abc.Keys()
	abc2Keys := abc2.Keys()
	slices.Sort(abcKeys)
	slices.Sort(abc2Keys)
	c.Assert(abcKeys, qt.DeepEquals, abc2Keys)
}

// --- Delta-delta merge ---

func TestDeltaDeltaMerge(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	d1 := a.Apply("x", addDot)
	d2 := a.Apply("y", addDot)

	// Combine deltas, then apply.
	d1.Merge(d2)

	b := newSetMap("b")
	b.Merge(d1)

	keys := b.Keys()
	slices.Sort(keys)
	c.Assert(keys, qt.DeepEquals, []string{"x", "y"})
}

// --- Concurrent removes ---

func TestConcurrentRemovesSameKey(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")

	addDelta := a.Apply("x", addDot)
	b.Merge(addDelta)

	rmA := a.Remove("x")
	rmB := b.Remove("x")

	a.Merge(rmB)
	b.Merge(rmA)

	c.Assert(a.Has("x"), qt.IsFalse)
	c.Assert(b.Has("x"), qt.IsFalse)
}

// --- Remove then re-apply ---

func TestRemoveThenReApply(t *testing.T) {
	c := qt.New(t)
	m := newSetMap("a")
	m.Apply("x", addDot)
	m.Remove("x")
	c.Assert(m.Has("x"), qt.IsFalse)

	m.Apply("x", addDot)
	c.Assert(m.Has("x"), qt.IsTrue)
	c.Assert(m.Len(), qt.Equals, 1)
}

// --- Nested value merge after concurrent apply ---

func TestConcurrentApplySameKeyMergesValues(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")

	// Both apply to the same key concurrently — values should merge (union of dot sets).
	da := a.Apply("x", addDot)
	a.Apply("x", addDot) // a has 2 dots for "x"
	db := b.Apply("x", addDot)

	a.Merge(db)
	b.Merge(da)

	// After merge, both should have "x" with merged dots.
	c.Assert(a.Has("x"), qt.IsTrue)
	c.Assert(b.Has("x"), qt.IsTrue)

	vA, _ := a.Get("x")
	vB, _ := b.Get("x")
	c.Assert(vA.Len(), qt.Equals, 3) // 2 from a + 1 from b
	c.Assert(vB.Len(), qt.Equals, 2) // 1 from a's first delta + 1 from b
}

// --- Merge with empty map ---

func TestMergeIntoEmpty(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	a.Apply("x", addDot)
	a.Apply("y", addDot)

	b := newSetMap("b")
	b.Merge(a)

	keys := b.Keys()
	slices.Sort(keys)
	c.Assert(keys, qt.DeepEquals, []string{"x", "y"})
}

func TestMergeEmptyIntoPopulated(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	a.Apply("x", addDot)

	empty := newSetMap("b")
	a.Merge(empty)

	c.Assert(a.Has("x"), qt.IsTrue)
	c.Assert(a.Len(), qt.Equals, 1)
}

// --- Context accessor ---

func TestContext(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	a.Apply("x", addDot)

	ctx := a.Context()
	c.Assert(ctx, qt.Not(qt.IsNil))
	// The context should contain the dot generated by Apply.
	c.Assert(len(ctx.ReplicaIDs()), qt.Equals, 1)
	c.Assert(ctx.ReplicaIDs()[0], qt.Equals, dotcontext.ReplicaID("a"))
}

// --- State / FromCausal round-trip ---

func TestStateFromCausalRoundTrip(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	a.Apply("x", addDot)
	a.Apply("y", addDot)

	state := a.State()
	b := FromCausal(state, joinDotSet, dotcontext.NewDotSet)

	c.Assert(b.Has("x"), qt.IsTrue)
	c.Assert(b.Has("y"), qt.IsTrue)
	c.Assert(b.Len(), qt.Equals, 2)
}

func TestFromCausalDeltaMerge(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	delta := a.Apply("x", addDot)

	reconstructed := FromCausal(delta.State(), joinDotSet, dotcontext.NewDotSet)

	b := newSetMap("b")
	b.Merge(reconstructed)

	c.Assert(b.Has("x"), qt.IsTrue)
	c.Assert(b.Len(), qt.Equals, 1)
}

// --- Nested ORMap (ORMap[string, *DotMap[string, *DotSet]]) ---

func joinNestedDotMap(
	a, b dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]],
) dotcontext.Causal[*dotcontext.DotMap[string, *dotcontext.DotSet]] {
	return dotcontext.JoinDotMap(a, b, joinDotSet, dotcontext.NewDotSet)
}

func newNestedMap(id dotcontext.ReplicaID) *ORMap[string, *dotcontext.DotMap[string, *dotcontext.DotSet]] {
	return New[string, *dotcontext.DotMap[string, *dotcontext.DotSet]](
		id,
		joinNestedDotMap,
		func() *dotcontext.DotMap[string, *dotcontext.DotSet] {
			return dotcontext.NewDotMap[string, *dotcontext.DotSet]()
		},
	)
}

func TestNestedORMapConcurrentApply(t *testing.T) {
	c := qt.New(t)
	a := newNestedMap("a")
	b := newNestedMap("b")

	// a adds sub-key "file1" under key "dir".
	da := a.Apply("dir", func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotMap[string, *dotcontext.DotSet], delta *dotcontext.DotMap[string, *dotcontext.DotSet]) {
		d := ctx.Next(id)
		ds, ok := v.Get("file1")
		if !ok {
			ds = dotcontext.NewDotSet()
			v.Set("file1", ds)
		}
		ds.Add(d)
		deltaDS := dotcontext.NewDotSet()
		deltaDS.Add(d)
		delta.Set("file1", deltaDS)
	})

	// b concurrently adds sub-key "file2" under the same key "dir".
	db := b.Apply("dir", func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotMap[string, *dotcontext.DotSet], delta *dotcontext.DotMap[string, *dotcontext.DotSet]) {
		d := ctx.Next(id)
		ds, ok := v.Get("file2")
		if !ok {
			ds = dotcontext.NewDotSet()
			v.Set("file2", ds)
		}
		ds.Add(d)
		deltaDS := dotcontext.NewDotSet()
		deltaDS.Add(d)
		delta.Set("file2", deltaDS)
	})

	a.Merge(db)
	b.Merge(da)

	// Both should have "dir" with sub-keys "file1" and "file2".
	for _, m := range []*ORMap[string, *dotcontext.DotMap[string, *dotcontext.DotSet]]{a, b} {
		v, ok := m.Get("dir")
		c.Assert(ok, qt.IsTrue)
		_, hasFile1 := v.Get("file1")
		_, hasFile2 := v.Get("file2")
		c.Assert(hasFile1, qt.IsTrue)
		c.Assert(hasFile2, qt.IsTrue)
	}
}

func TestNestedORMapRemoveKeyPreservesOthers(t *testing.T) {
	c := qt.New(t)
	a := newNestedMap("a")
	b := newNestedMap("b")

	applyFile := func(key, subKey string) func(dotcontext.ReplicaID, *dotcontext.CausalContext, *dotcontext.DotMap[string, *dotcontext.DotSet], *dotcontext.DotMap[string, *dotcontext.DotSet]) {
		return func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotMap[string, *dotcontext.DotSet], delta *dotcontext.DotMap[string, *dotcontext.DotSet]) {
			d := ctx.Next(id)
			ds, ok := v.Get(subKey)
			if !ok {
				ds = dotcontext.NewDotSet()
				v.Set(subKey, ds)
			}
			ds.Add(d)
			deltaDS := dotcontext.NewDotSet()
			deltaDS.Add(d)
			delta.Set(subKey, deltaDS)
		}
	}

	d1 := a.Apply("dir1", applyFile("dir1", "f"))
	d2 := a.Apply("dir2", applyFile("dir2", "f"))
	b.Merge(d1)
	b.Merge(d2)

	// Remove dir1 — dir2 should survive.
	rmDelta := a.Remove("dir1")
	b.Merge(rmDelta)

	c.Assert(b.Has("dir1"), qt.IsFalse)
	c.Assert(b.Has("dir2"), qt.IsTrue)
}

// --- Apply with supersede (replace pattern) ---

func TestApplySupersede(t *testing.T) {
	c := qt.New(t)
	a := newSetMap("a")
	b := newSetMap("b")

	d1 := a.Apply("x", addDot)
	b.Merge(d1)

	d2 := a.Apply("x", func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotSet, delta *dotcontext.DotSet) {
		var old []dotcontext.Dot
		v.Range(func(d dotcontext.Dot) bool {
			old = append(old, d)
			return true
		})
		for _, d := range old {
			v.Remove(d)
		}
		d := ctx.Next(id)
		v.Add(d)
		delta.Add(d)
	})
	b.Merge(d2)

	v, ok := b.Get("x")
	c.Assert(ok, qt.IsTrue)
	c.Assert(v.Len(), qt.Equals, 1)
}

// --- DotFun values (map of counters) ---

type counterValue struct {
	n int64
}

func (cv counterValue) Join(other counterValue) counterValue {
	return cv
}

func joinDotFun(a, b dotcontext.Causal[*dotcontext.DotFun[counterValue]]) dotcontext.Causal[*dotcontext.DotFun[counterValue]] {
	return dotcontext.JoinDotFun(a, b)
}

func newCounterMap(id dotcontext.ReplicaID) *ORMap[string, *dotcontext.DotFun[counterValue]] {
	return New[string, *dotcontext.DotFun[counterValue]](
		id,
		joinDotFun,
		dotcontext.NewDotFun[counterValue],
	)
}

func TestDotFunValues(t *testing.T) {
	c := qt.New(t)
	a := newCounterMap("a")
	b := newCounterMap("b")

	da := a.Apply("hits", func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotFun[counterValue], delta *dotcontext.DotFun[counterValue]) {
		d := ctx.Next(id)
		v.Set(d, counterValue{n: 5})
		delta.Set(d, counterValue{n: 5})
	})
	b.Merge(da)

	db := b.Apply("hits", func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotFun[counterValue], delta *dotcontext.DotFun[counterValue]) {
		d := ctx.Next(id)
		v.Set(d, counterValue{n: 3})
		delta.Set(d, counterValue{n: 3})
	})
	a.Merge(db)

	for _, m := range []*ORMap[string, *dotcontext.DotFun[counterValue]]{a, b} {
		v, ok := m.Get("hits")
		c.Assert(ok, qt.IsTrue)
		c.Assert(v.Len(), qt.Equals, 2)
		var total int64
		v.Range(func(_ dotcontext.Dot, cv counterValue) bool {
			total += cv.n
			return true
		})
		c.Assert(total, qt.Equals, int64(8))
	}
}
