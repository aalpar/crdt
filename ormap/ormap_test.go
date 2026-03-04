package ormap

import (
	"slices"
	"testing"

	"github.com/aalpar/crdt/dotcontext"
)

// Helpers for ORMap[string, *DotSet] — a map of sets.

func joinDotSet(a, b dotcontext.Causal[*dotcontext.DotSet]) dotcontext.Causal[*dotcontext.DotSet] {
	return dotcontext.JoinDotSet(a, b)
}

func newSetMap(id dotcontext.ReplicaID) *ORMap[string, *dotcontext.DotSet] {
	return New[string, *dotcontext.DotSet](id, joinDotSet, dotcontext.NewDotSet)
}

// addDot is a common Apply fn: generate a dot and add it to both
// the local value and the delta.
func addDot(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotSet, delta *dotcontext.DotSet) {
	d := ctx.Next(id)
	v.Add(d)
	delta.Add(d)
}

func TestNewEmpty(t *testing.T) {
	m := newSetMap("a")
	if m.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", m.Len())
	}
	if m.Has("x") {
		t.Fatal("new map has key")
	}
	if keys := m.Keys(); len(keys) != 0 {
		t.Fatalf("Keys() = %v, want empty", keys)
	}
}

func TestApplyCreatesKey(t *testing.T) {
	m := newSetMap("a")
	m.Apply("x", addDot)

	if !m.Has("x") {
		t.Fatal("key missing after Apply")
	}
	if m.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", m.Len())
	}
	v, ok := m.Get("x")
	if !ok {
		t.Fatal("Get returned false")
	}
	if v.Len() != 1 {
		t.Fatalf("value has %d dots, want 1", v.Len())
	}
}

func TestApplyMultipleKeys(t *testing.T) {
	m := newSetMap("a")
	m.Apply("x", addDot)
	m.Apply("y", addDot)
	m.Apply("z", addDot)

	keys := m.Keys()
	slices.Sort(keys)
	want := []string{"x", "y", "z"}
	if !slices.Equal(keys, want) {
		t.Fatalf("Keys() = %v, want %v", keys, want)
	}
}

func TestApplyAccumulatesDots(t *testing.T) {
	m := newSetMap("a")
	m.Apply("x", addDot)
	m.Apply("x", addDot)

	v, _ := m.Get("x")
	if v.Len() != 2 {
		t.Fatalf("value has %d dots, want 2", v.Len())
	}
}

func TestApplyReturnsValidDelta(t *testing.T) {
	m := newSetMap("a")
	delta := m.Apply("x", addDot)

	if !delta.Has("x") {
		t.Fatal("delta missing key")
	}
	if delta.Len() != 1 {
		t.Fatalf("delta Len() = %d, want 1", delta.Len())
	}
}

func TestRemove(t *testing.T) {
	m := newSetMap("a")
	m.Apply("x", addDot)
	m.Apply("y", addDot)
	m.Remove("x")

	if m.Has("x") {
		t.Fatal("has x after Remove")
	}
	if !m.Has("y") {
		t.Fatal("missing y after Remove of x")
	}
}

func TestRemoveAbsent(t *testing.T) {
	m := newSetMap("a")
	m.Remove("ghost") // should not panic
	if m.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", m.Len())
	}
}

// --- Delta propagation ---

func TestMergeDelta(t *testing.T) {
	a := newSetMap("a")
	b := newSetMap("b")

	delta := a.Apply("x", addDot)
	b.Merge(delta)

	if !b.Has("x") {
		t.Fatal("b missing x after merging delta")
	}
}

func TestMergeRemoveDelta(t *testing.T) {
	a := newSetMap("a")
	b := newSetMap("b")

	addDelta := a.Apply("x", addDot)
	b.Merge(addDelta)

	rmDelta := a.Remove("x")
	b.Merge(rmDelta)

	if b.Has("x") {
		t.Fatal("b still has x after remove delta")
	}
}

// --- Add-wins semantics ---

func TestAddWinsConcurrent(t *testing.T) {
	a := newSetMap("a")
	b := newSetMap("b")

	addDelta := a.Apply("x", addDot)
	b.Merge(addDelta) // both have x

	// Concurrent: a removes, b adds.
	rmDelta := a.Remove("x")
	addDelta2 := b.Apply("x", addDot)

	a.Merge(addDelta2)
	b.Merge(rmDelta)

	if !a.Has("x") {
		t.Fatal("a: add-wins failed")
	}
	if !b.Has("x") {
		t.Fatal("b: add-wins failed")
	}
}

func TestConcurrentApplyDifferentKeys(t *testing.T) {
	a := newSetMap("a")
	b := newSetMap("b")

	da := a.Apply("x", addDot)
	db := b.Apply("y", addDot)

	a.Merge(db)
	b.Merge(da)

	for name, m := range map[string]*ORMap[string, *dotcontext.DotSet]{"a": a, "b": b} {
		keys := m.Keys()
		slices.Sort(keys)
		want := []string{"x", "y"}
		if !slices.Equal(keys, want) {
			t.Fatalf("%s: Keys() = %v, want %v", name, keys, want)
		}
	}
}

func TestConcurrentApplySameKey(t *testing.T) {
	a := newSetMap("a")
	b := newSetMap("b")

	da := a.Apply("x", addDot)
	db := b.Apply("x", addDot)

	a.Merge(db)
	b.Merge(da)

	// Both should have x with 2 dots (one from each replica).
	for name, m := range map[string]*ORMap[string, *dotcontext.DotSet]{"a": a, "b": b} {
		v, ok := m.Get("x")
		if !ok {
			t.Fatalf("%s: missing x", name)
		}
		if v.Len() != 2 {
			t.Fatalf("%s: x has %d dots, want 2", name, v.Len())
		}
	}
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	a := newSetMap("a")
	a.Apply("x", addDot)

	snapshot := newSetMap("x")
	snapshot.Merge(a)

	a.Merge(snapshot)
	a.Merge(snapshot)

	if a.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", a.Len())
	}
}

func TestMergeCommutative(t *testing.T) {
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
	if !slices.Equal(abKeys, baKeys) {
		t.Fatalf("not commutative: %v vs %v", abKeys, baKeys)
	}
}

func TestDeltaIncrementalEqualsFullMerge(t *testing.T) {
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
	if !slices.Equal(incKeys, fullKeys) {
		t.Fatalf("incremental %v != full %v", incKeys, fullKeys)
	}
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	a := newSetMap("a")
	b := newSetMap("b")
	c := newSetMap("c")

	da := a.Apply("x", addDot)
	db := b.Apply("y", addDot)
	dc := c.Apply("z", addDot)

	a.Merge(db)
	a.Merge(dc)
	b.Merge(da)
	b.Merge(dc)
	c.Merge(da)
	c.Merge(db)

	for name, m := range map[string]*ORMap[string, *dotcontext.DotSet]{"a": a, "b": b, "c": c} {
		keys := m.Keys()
		slices.Sort(keys)
		want := []string{"x", "y", "z"}
		if !slices.Equal(keys, want) {
			t.Fatalf("%s: Keys() = %v, want %v", name, keys, want)
		}
	}
}

// --- Apply with supersede (replace pattern) ---

func TestApplySupersede(t *testing.T) {
	a := newSetMap("a")
	b := newSetMap("b")

	// a adds a dot to "x".
	d1 := a.Apply("x", addDot)
	b.Merge(d1)

	// a supersedes the old dot with a new one.
	d2 := a.Apply("x", func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotSet, delta *dotcontext.DotSet) {
		// Remove old dots from v (Apply captures this in delta context).
		var old []dotcontext.Dot
		v.Range(func(d dotcontext.Dot) bool {
			old = append(old, d)
			return true
		})
		for _, d := range old {
			v.Remove(d)
		}
		// Add new dot to both v and delta.
		d := ctx.Next(id)
		v.Add(d)
		delta.Add(d)
	})
	b.Merge(d2)

	// b should have x with only 1 dot (the new one).
	v, ok := b.Get("x")
	if !ok {
		t.Fatal("b missing x")
	}
	if v.Len() != 1 {
		t.Fatalf("b has %d dots for x, want 1 (old should be superseded)", v.Len())
	}
}

// --- DotFun values (map of counters) ---

// counterValue for testing ORMap with DotFun values.
type counterValue struct {
	n int64
}

func (c counterValue) Join(other counterValue) counterValue {
	return c
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
	a := newCounterMap("a")
	b := newCounterMap("b")

	// a sets counter at "hits" to 5.
	da := a.Apply("hits", func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotFun[counterValue], delta *dotcontext.DotFun[counterValue]) {
		d := ctx.Next(id)
		v.Set(d, counterValue{n: 5})
		delta.Set(d, counterValue{n: 5})
	})
	b.Merge(da)

	// b sets counter at "hits" to 3 (concurrent).
	// Since b already has a's dot, this is an add, not a replace.
	db := b.Apply("hits", func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotFun[counterValue], delta *dotcontext.DotFun[counterValue]) {
		d := ctx.Next(id)
		v.Set(d, counterValue{n: 3})
		delta.Set(d, counterValue{n: 3})
	})
	a.Merge(db)

	// Both should have "hits" with 2 entries (one per replica).
	for name, m := range map[string]*ORMap[string, *dotcontext.DotFun[counterValue]]{"a": a, "b": b} {
		v, ok := m.Get("hits")
		if !ok {
			t.Fatalf("%s: missing hits", name)
		}
		if v.Len() != 2 {
			t.Fatalf("%s: hits has %d entries, want 2", name, v.Len())
		}
		var total int64
		v.Range(func(_ dotcontext.Dot, cv counterValue) bool {
			total += cv.n
			return true
		})
		if total != 8 {
			t.Fatalf("%s: total = %d, want 8", name, total)
		}
	}
}
