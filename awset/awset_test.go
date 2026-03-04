package awset

import (
	"slices"
	"testing"
)

func TestNewEmpty(t *testing.T) {
	s := New[string]("a")
	if s.Len() != 0 {
		t.Fatalf("new set has %d elements, want 0", s.Len())
	}
	if s.Has("x") {
		t.Fatal("new set contains element")
	}
	if elems := s.Elements(); len(elems) != 0 {
		t.Fatalf("Elements() = %v, want empty", elems)
	}
}

func TestAddHas(t *testing.T) {
	s := New[string]("a")
	s.Add("x")
	s.Add("y")

	if !s.Has("x") {
		t.Fatal("missing x after Add")
	}
	if !s.Has("y") {
		t.Fatal("missing y after Add")
	}
	if s.Has("z") {
		t.Fatal("has z without Add")
	}
	if s.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", s.Len())
	}
}

func TestRemove(t *testing.T) {
	s := New[string]("a")
	s.Add("x")
	s.Add("y")
	s.Remove("x")

	if s.Has("x") {
		t.Fatal("has x after Remove")
	}
	if !s.Has("y") {
		t.Fatal("missing y after Remove of x")
	}
	if s.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", s.Len())
	}
}

func TestRemoveAbsent(t *testing.T) {
	s := New[string]("a")
	s.Remove("ghost") // should not panic
	if s.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", s.Len())
	}
}

func TestElements(t *testing.T) {
	s := New[int]("a")
	s.Add(3)
	s.Add(1)
	s.Add(2)

	elems := s.Elements()
	slices.Sort(elems)
	want := []int{1, 2, 3}
	if !slices.Equal(elems, want) {
		t.Fatalf("Elements() = %v, want %v", elems, want)
	}
}

func TestAddReturnsValidDelta(t *testing.T) {
	s := New[string]("a")
	delta := s.Add("x")

	// Delta should contain exactly x.
	if !delta.Has("x") {
		t.Fatal("delta missing added element")
	}
	if delta.Len() != 1 {
		t.Fatalf("delta Len() = %d, want 1", delta.Len())
	}
}

func TestRemoveReturnsValidDelta(t *testing.T) {
	s := New[string]("a")
	s.Add("x")
	delta := s.Remove("x")

	// Delta store should be empty (remove has no elements in store).
	if delta.Len() != 0 {
		t.Fatalf("remove delta Len() = %d, want 0", delta.Len())
	}
}

// --- Add-wins semantics ---

func TestAddWinsConcurrent(t *testing.T) {
	// Two replicas start with the same state (x present).
	a := New[string]("a")
	b := New[string]("b")

	addDelta := a.Add("x")
	b.Merge(addDelta) // both see x

	// Concurrent operations: a removes x, b adds x.
	rmDelta := a.Remove("x")
	addDelta2 := b.Add("x")

	// a receives b's add.
	a.Merge(addDelta2)
	// b receives a's remove.
	b.Merge(rmDelta)

	// Both should have x — add wins over concurrent remove.
	if !a.Has("x") {
		t.Fatal("a: add-wins failed, x missing")
	}
	if !b.Has("x") {
		t.Fatal("b: add-wins failed, x missing")
	}
}

func TestConcurrentAddsSameElement(t *testing.T) {
	// Two replicas independently add the same element.
	a := New[string]("a")
	b := New[string]("b")

	da := a.Add("x")
	db := b.Add("x")

	a.Merge(db)
	b.Merge(da)

	// Both should have x. The underlying DotSet has two dots.
	if !a.Has("x") {
		t.Fatal("a missing x after concurrent adds")
	}
	if !b.Has("x") {
		t.Fatal("b missing x after concurrent adds")
	}
}

func TestRemoveThenReadd(t *testing.T) {
	s := New[string]("a")
	s.Add("x")
	s.Remove("x")

	if s.Has("x") {
		t.Fatal("has x after remove")
	}

	s.Add("x")
	if !s.Has("x") {
		t.Fatal("missing x after re-add")
	}
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	a := New[string]("a")
	a.Add("x")
	a.Add("y")

	// Clone state by creating fresh set and merging.
	snapshot := New[string]("a")
	snapshot.Merge(a)

	// Merge a into itself (via snapshot) twice.
	a.Merge(snapshot)
	a.Merge(snapshot)

	if a.Len() != 2 {
		t.Fatalf("after idempotent merge, Len() = %d, want 2", a.Len())
	}
}

func TestMergeCommutative(t *testing.T) {
	a := New[string]("a")
	b := New[string]("b")
	a.Add("x")
	b.Add("y")

	// ab: merge b into copy of a.
	ab := New[string]("a")
	ab.Merge(a)
	ab.Merge(b)

	// ba: merge a into copy of b.
	ba := New[string]("b")
	ba.Merge(b)
	ba.Merge(a)

	abElems := ab.Elements()
	baElems := ba.Elements()
	slices.Sort(abElems)
	slices.Sort(baElems)

	if !slices.Equal(abElems, baElems) {
		t.Fatalf("merge not commutative: %v vs %v", abElems, baElems)
	}
}

func TestDeltaIncrementalEqualsFullMerge(t *testing.T) {
	// Build state on replica a with two operations.
	a := New[string]("a")
	d1 := a.Add("x")
	d2 := a.Add("y")

	// Replica b gets deltas incrementally.
	bIncremental := New[string]("b")
	bIncremental.Merge(d1)
	bIncremental.Merge(d2)

	// Replica c gets full state at once.
	bFull := New[string]("b")
	bFull.Merge(a)

	incElems := bIncremental.Elements()
	fullElems := bFull.Elements()
	slices.Sort(incElems)
	slices.Sort(fullElems)

	if !slices.Equal(incElems, fullElems) {
		t.Fatalf("incremental %v != full %v", incElems, fullElems)
	}
}

func TestMergeRemoveDelta(t *testing.T) {
	a := New[string]("a")
	b := New[string]("b")

	// a adds x, shares with b.
	addDelta := a.Add("x")
	b.Merge(addDelta)

	// a removes x, shares delta with b.
	rmDelta := a.Remove("x")
	b.Merge(rmDelta)

	if b.Has("x") {
		t.Fatal("b still has x after receiving remove delta")
	}
}

// --- Integer elements ---

func TestIntElements(t *testing.T) {
	s := New[int]("a")
	s.Add(42)
	s.Add(7)

	if !s.Has(42) {
		t.Fatal("missing 42")
	}
	if s.Has(99) {
		t.Fatal("has 99 without Add")
	}
	if s.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", s.Len())
	}
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	a := New[string]("a")
	b := New[string]("b")
	c := New[string]("c")

	da := a.Add("x")
	db := b.Add("y")
	dc := c.Add("z")

	// Each replica merges the other two deltas.
	a.Merge(db)
	a.Merge(dc)
	b.Merge(da)
	b.Merge(dc)
	c.Merge(da)
	c.Merge(db)

	for _, r := range []*AWSet[string]{a, b, c} {
		elems := r.Elements()
		slices.Sort(elems)
		want := []string{"x", "y", "z"}
		if !slices.Equal(elems, want) {
			t.Fatalf("replica has %v, want %v", elems, want)
		}
	}
}
