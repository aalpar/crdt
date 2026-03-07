package awset

import (
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNewEmpty(t *testing.T) {
	c := qt.New(t)
	s := New[string]("a")
	c.Assert(s.Len(), qt.Equals, 0)
	c.Assert(s.Has("x"), qt.IsFalse)
	c.Assert(s.Elements(), qt.HasLen, 0)
}

func TestAddHas(t *testing.T) {
	c := qt.New(t)
	s := New[string]("a")
	s.Add("x")
	s.Add("y")

	c.Assert(s.Has("x"), qt.IsTrue)
	c.Assert(s.Has("y"), qt.IsTrue)
	c.Assert(s.Has("z"), qt.IsFalse)
	c.Assert(s.Len(), qt.Equals, 2)
}

func TestRemove(t *testing.T) {
	c := qt.New(t)
	s := New[string]("a")
	s.Add("x")
	s.Add("y")
	s.Remove("x")

	c.Assert(s.Has("x"), qt.IsFalse)
	c.Assert(s.Has("y"), qt.IsTrue)
	c.Assert(s.Len(), qt.Equals, 1)
}

func TestRemoveAbsent(t *testing.T) {
	c := qt.New(t)
	s := New[string]("a")
	s.Remove("ghost") // should not panic
	c.Assert(s.Len(), qt.Equals, 0)
}

func TestElements(t *testing.T) {
	c := qt.New(t)
	s := New[int]("a")
	s.Add(3)
	s.Add(1)
	s.Add(2)

	elems := s.Elements()
	slices.Sort(elems)
	c.Assert(elems, qt.DeepEquals, []int{1, 2, 3})
}

func TestAddReturnsValidDelta(t *testing.T) {
	c := qt.New(t)
	s := New[string]("a")
	delta := s.Add("x")

	c.Assert(delta.Has("x"), qt.IsTrue)
	c.Assert(delta.Len(), qt.Equals, 1)
}

func TestRemoveReturnsValidDelta(t *testing.T) {
	c := qt.New(t)
	s := New[string]("a")
	s.Add("x")
	delta := s.Remove("x")
	c.Assert(delta.Len(), qt.Equals, 0)
}

// --- Add-wins semantics ---

func TestAddWinsConcurrent(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")

	addDelta := a.Add("x")
	b.Merge(addDelta)

	rmDelta := a.Remove("x")
	addDelta2 := b.Add("x")

	a.Merge(addDelta2)
	b.Merge(rmDelta)

	c.Assert(a.Has("x"), qt.IsTrue)
	c.Assert(b.Has("x"), qt.IsTrue)
}

func TestConcurrentAddsSameElement(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")

	da := a.Add("x")
	db := b.Add("x")

	a.Merge(db)
	b.Merge(da)

	c.Assert(a.Has("x"), qt.IsTrue)
	c.Assert(b.Has("x"), qt.IsTrue)
}

func TestRemoveThenReadd(t *testing.T) {
	c := qt.New(t)
	s := New[string]("a")
	s.Add("x")
	s.Remove("x")
	c.Assert(s.Has("x"), qt.IsFalse)

	s.Add("x")
	c.Assert(s.Has("x"), qt.IsTrue)
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	a.Add("x")
	a.Add("y")

	snapshot := New[string]("a")
	snapshot.Merge(a)

	a.Merge(snapshot)
	a.Merge(snapshot)

	c.Assert(a.Len(), qt.Equals, 2)
}

func TestMergeCommutative(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")
	a.Add("x")
	b.Add("y")

	ab := New[string]("a")
	ab.Merge(a)
	ab.Merge(b)

	ba := New[string]("b")
	ba.Merge(b)
	ba.Merge(a)

	abElems := ab.Elements()
	baElems := ba.Elements()
	slices.Sort(abElems)
	slices.Sort(baElems)

	c.Assert(abElems, qt.DeepEquals, baElems)
}

func TestDeltaIncrementalEqualsFullMerge(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	d1 := a.Add("x")
	d2 := a.Add("y")

	bIncremental := New[string]("b")
	bIncremental.Merge(d1)
	bIncremental.Merge(d2)

	bFull := New[string]("b")
	bFull.Merge(a)

	incElems := bIncremental.Elements()
	fullElems := bFull.Elements()
	slices.Sort(incElems)
	slices.Sort(fullElems)

	c.Assert(incElems, qt.DeepEquals, fullElems)
}

func TestMergeRemoveDelta(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")

	addDelta := a.Add("x")
	b.Merge(addDelta)

	rmDelta := a.Remove("x")
	b.Merge(rmDelta)

	c.Assert(b.Has("x"), qt.IsFalse)
}

// --- Integer elements ---

func TestIntElements(t *testing.T) {
	c := qt.New(t)
	s := New[int]("a")
	s.Add(42)
	s.Add(7)

	c.Assert(s.Has(42), qt.IsTrue)
	c.Assert(s.Has(99), qt.IsFalse)
	c.Assert(s.Len(), qt.Equals, 2)
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")
	x := New[string]("c")

	da := a.Add("x")
	db := b.Add("y")
	dx := x.Add("z")

	a.Merge(db)
	a.Merge(dx)
	b.Merge(da)
	b.Merge(dx)
	x.Merge(da)
	x.Merge(db)

	want := []string{"x", "y", "z"}
	for _, r := range []*AWSet[string]{a, b, x} {
		elems := r.Elements()
		slices.Sort(elems)
		c.Assert(elems, qt.DeepEquals, want)
	}
}
