package pncounter

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
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

// --- State / FromCausal round-trip ---

func TestStateFromCausalRoundTrip(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	a.Increment(5)
	a.Increment(3)

	state := a.State()
	b := FromCausal(state)

	c.Assert(b.Value(), qt.Equals, int64(8))
}

func TestFromCausalDeltaMerge(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	delta := a.Increment(7)

	reconstructed := FromCausal(delta.State())

	b := New("b")
	b.Merge(reconstructed)

	c.Assert(b.Value(), qt.Equals, int64(7))
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

// --- Merge associativity ---

func TestMergeAssociative(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")
	x := New("c")

	a.Increment(10)
	b.Decrement(3)
	x.Increment(7)
	x.Decrement(2)

	// (a ⊔ b) ⊔ c
	ab := New("ab")
	ab.Merge(a)
	ab.Merge(b)
	abc := New("abc")
	abc.Merge(ab)
	abc.Merge(x)

	// a ⊔ (b ⊔ c)
	bc := New("bc")
	bc.Merge(b)
	bc.Merge(x)
	abc2 := New("abc2")
	abc2.Merge(a)
	abc2.Merge(bc)

	c.Assert(abc.Value(), qt.Equals, abc2.Value())
}

// --- Delta-delta merge ---

func TestDeltaDeltaMerge(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	d1 := a.Increment(3)
	d2 := a.Increment(7)

	// Combine deltas, then apply.
	d1.Merge(d2)

	b := New("b")
	b.Merge(d1)

	c.Assert(b.Value(), qt.Equals, int64(10))
}

// --- One dot per replica ---

func TestOneDotPerReplica(t *testing.T) {
	c := qt.New(t)
	r := New("a")
	r.Increment(1)
	r.Increment(2)
	r.Increment(3)
	r.Decrement(1)

	count := 0
	r.state.Store.Range(func(_ dotcontext.Dot, _ CounterValue) bool {
		count++
		return true
	})
	c.Assert(count, qt.Equals, 1)
	c.Assert(r.Value(), qt.Equals, int64(5))
}

func TestIncrementReplacesOnlyOwnDot(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")

	da := a.Increment(3)
	db := b.Increment(7)

	a.Merge(db)

	// a's store should have exactly two dots: one per replica.
	count := 0
	a.state.Store.Range(func(_ dotcontext.Dot, _ CounterValue) bool {
		count++
		return true
	})
	c.Assert(count, qt.Equals, 2)
	c.Assert(a.Value(), qt.Equals, int64(10))

	// Incrementing a should replace only a's dot, not b's.
	a.Increment(5)
	count = 0
	a.state.Store.Range(func(_ dotcontext.Dot, _ CounterValue) bool {
		count++
		return true
	})
	c.Assert(count, qt.Equals, 2)
	c.Assert(a.Value(), qt.Equals, int64(15))

	_ = da
}

// --- Decrement to zero ---

func TestDecrementToZero(t *testing.T) {
	c := qt.New(t)
	r := New("a")
	r.Increment(5)
	r.Decrement(5)
	c.Assert(r.Value(), qt.Equals, int64(0))
}

// --- Merge with empty counter ---

func TestMergeIntoEmpty(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	a.Increment(42)

	b := New("b")
	b.Merge(a)
	c.Assert(b.Value(), qt.Equals, int64(42))
}

func TestMergeEmptyIntoSet(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	a.Increment(42)

	empty := New("b")
	a.Merge(empty)
	c.Assert(a.Value(), qt.Equals, int64(42))
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
