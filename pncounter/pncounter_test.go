package pncounter

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNewZero(t *testing.T) {
	c := qt.New(t)
	r := New("a")
	c.Assert(r.Value(), qt.Equals, int64(0))
}

func TestIncrement(t *testing.T) {
	c := qt.New(t)
	r := New("a")
	r.Increment(5)
	c.Assert(r.Value(), qt.Equals, int64(5))
}

func TestMultipleIncrements(t *testing.T) {
	c := qt.New(t)
	r := New("a")
	r.Increment(3)
	r.Increment(7)
	r.Increment(2)
	c.Assert(r.Value(), qt.Equals, int64(12))
}

func TestDecrement(t *testing.T) {
	c := qt.New(t)
	r := New("a")
	r.Increment(10)
	r.Decrement(4)
	c.Assert(r.Value(), qt.Equals, int64(6))
}

func TestNegativeValue(t *testing.T) {
	c := qt.New(t)
	r := New("a")
	r.Decrement(5)
	c.Assert(r.Value(), qt.Equals, int64(-5))
}

func TestIncrementReturnsValidDelta(t *testing.T) {
	c := qt.New(t)
	r := New("a")
	delta := r.Increment(7)
	c.Assert(delta.Value(), qt.Equals, int64(7))
}

// --- Concurrent operations ---

func TestConcurrentIncrements(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")

	da := a.Increment(3)
	db := b.Increment(5)

	a.Merge(db)
	b.Merge(da)

	c.Assert(a.Value(), qt.Equals, int64(8))
	c.Assert(b.Value(), qt.Equals, int64(8))
}

func TestConcurrentIncrementDecrement(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")

	da := a.Increment(10)
	db := b.Decrement(3)

	a.Merge(db)
	b.Merge(da)

	c.Assert(a.Value(), qt.Equals, int64(7))
	c.Assert(b.Value(), qt.Equals, int64(7))
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	a.Increment(5)

	snapshot := New("x")
	snapshot.Merge(a)

	a.Merge(snapshot)
	a.Merge(snapshot)

	c.Assert(a.Value(), qt.Equals, int64(5))
}

func TestMergeCommutative(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")
	a.Increment(3)
	b.Increment(7)

	ab := New("x")
	ab.Merge(a)
	ab.Merge(b)

	ba := New("x")
	ba.Merge(b)
	ba.Merge(a)

	c.Assert(ab.Value(), qt.Equals, ba.Value())
}

func TestDeltaIncrementalEqualsFullMerge(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	d1 := a.Increment(3)
	d2 := a.Increment(7)

	inc := New("b")
	inc.Merge(d1)
	inc.Merge(d2)

	full := New("b")
	full.Merge(a)

	c.Assert(inc.Value(), qt.Equals, full.Value())
}

// --- Delta propagation ---

func TestDeltaSupersedes(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")

	d1 := a.Increment(3)
	b.Merge(d1)

	d2 := a.Increment(7)
	b.Merge(d2)

	c.Assert(b.Value(), qt.Equals, int64(10))
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")
	x := New("c")

	da := a.Increment(1)
	db := b.Increment(2)
	dx := x.Increment(3)

	a.Merge(db)
	a.Merge(dx)
	b.Merge(da)
	b.Merge(dx)
	x.Merge(da)
	x.Merge(db)

	for _, r := range []*Counter{a, b, x} {
		c.Assert(r.Value(), qt.Equals, int64(6))
	}
}

func TestThreeReplicaMixedOps(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")
	x := New("c")

	da := a.Increment(10)
	db := b.Decrement(3)
	dx := x.Increment(5)

	a.Merge(db)
	a.Merge(dx)
	b.Merge(da)
	b.Merge(dx)
	x.Merge(da)
	x.Merge(db)

	for _, r := range []*Counter{a, b, x} {
		c.Assert(r.Value(), qt.Equals, int64(12))
	}
}

// --- Sequential operations after merge ---

func TestIncrementAfterMerge(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")

	da := a.Increment(5)
	b.Merge(da)

	b.Increment(3)

	c.Assert(b.Value(), qt.Equals, int64(8))
}
