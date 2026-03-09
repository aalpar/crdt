package lwwregister

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestNewEmpty(t *testing.T) {
	c := qt.New(t)
	r := New[string]("a")
	_, _, ok := r.Value()
	c.Assert(ok, qt.IsFalse)
}

func TestSetAndValue(t *testing.T) {
	c := qt.New(t)
	r := New[string]("a")
	r.Set("hello", 1)

	v, ts, ok := r.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "hello")
	c.Assert(ts, qt.Equals, int64(1))
}

func TestOverwrite(t *testing.T) {
	c := qt.New(t)
	r := New[string]("a")
	r.Set("first", 1)
	r.Set("second", 2)

	v, ts, ok := r.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "second")
	c.Assert(ts, qt.Equals, int64(2))
}

func TestSetReturnsValidDelta(t *testing.T) {
	c := qt.New(t)
	r := New[string]("a")
	delta := r.Set("x", 10)

	v, ts, ok := delta.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "x")
	c.Assert(ts, qt.Equals, int64(10))
}

// --- Concurrent write resolution ---

func TestConcurrentWriteHigherTimestampWins(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")

	da := a.Set("from-a", 10)
	db := b.Set("from-b", 20)

	a.Merge(db)
	b.Merge(da)

	for _, r := range []*LWWRegister[string]{a, b} {
		v, ts, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "from-b")
		c.Assert(ts, qt.Equals, int64(20))
	}
}

func TestConcurrentWriteSameTimestampTiebreak(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")

	da := a.Set("from-a", 10)
	db := b.Set("from-b", 10)

	a.Merge(db)
	b.Merge(da)

	for _, r := range []*LWWRegister[string]{a, b} {
		v, _, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "from-b")
	}
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	a.Set("x", 5)

	snapshot := New[string]("a")
	snapshot.Merge(a)

	a.Merge(snapshot)
	a.Merge(snapshot)

	v, ts, ok := a.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "x")
	c.Assert(ts, qt.Equals, int64(5))
}

func TestMergeCommutative(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")
	a.Set("va", 1)
	b.Set("vb", 2)

	ab := New[string]("x")
	ab.Merge(a)
	ab.Merge(b)

	ba := New[string]("x")
	ba.Merge(b)
	ba.Merge(a)

	vAB, tsAB, _ := ab.Value()
	vBA, tsBA, _ := ba.Value()
	c.Assert(vAB, qt.Equals, vBA)
	c.Assert(tsAB, qt.Equals, tsBA)
}

func TestDeltaIncrementalEqualsFullMerge(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	d1 := a.Set("first", 1)
	d2 := a.Set("second", 2)

	inc := New[string]("b")
	inc.Merge(d1)
	inc.Merge(d2)

	full := New[string]("b")
	full.Merge(a)

	vInc, tsInc, _ := inc.Value()
	vFull, tsFull, _ := full.Value()
	c.Assert(vInc, qt.Equals, vFull)
	c.Assert(tsInc, qt.Equals, tsFull)
}

// --- Overwrite propagation ---

func TestOverwriteDeltaSupersedes(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")

	d1 := a.Set("first", 1)
	b.Merge(d1)

	d2 := a.Set("second", 2)
	b.Merge(d2)

	v, ts, ok := b.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "second")
	c.Assert(ts, qt.Equals, int64(2))
}

func TestConcurrentWriteThenOverwrite(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")

	da := a.Set("from-a", 10)
	db := b.Set("from-b", 20)

	a.Merge(db)
	b.Merge(da)

	d3 := a.Set("from-a-again", 30)
	b.Merge(d3)

	v, ts, ok := b.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "from-a-again")
	c.Assert(ts, qt.Equals, int64(30))
}

// --- Integer values ---

func TestIntegerRegister(t *testing.T) {
	c := qt.New(t)
	r := New[int]("a")
	r.Set(42, 1)
	r.Set(99, 2)

	v, _, ok := r.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, 99)
}

// --- Merge associativity ---

func TestMergeAssociative(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")
	x := New[string]("c")

	a.Set("va", 5)
	b.Set("vb", 10)
	x.Set("vc", 3)

	// (a ⊔ b) ⊔ c
	ab := New[string]("ab")
	ab.Merge(a)
	ab.Merge(b)
	abc := New[string]("abc")
	abc.Merge(ab)
	abc.Merge(x)

	// a ⊔ (b ⊔ c)
	bc := New[string]("bc")
	bc.Merge(b)
	bc.Merge(x)
	abc2 := New[string]("abc2")
	abc2.Merge(a)
	abc2.Merge(bc)

	v1, ts1, _ := abc.Value()
	v2, ts2, _ := abc2.Value()
	c.Assert(v1, qt.Equals, v2)
	c.Assert(ts1, qt.Equals, ts2)
}

// --- Delta-delta merge ---

func TestDeltaDeltaMerge(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	d1 := a.Set("first", 1)
	d2 := a.Set("second", 2)

	// Combine deltas, then apply.
	d1.Merge(d2)

	b := New[string]("b")
	b.Merge(d1)

	v, ts, ok := b.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "second")
	c.Assert(ts, qt.Equals, int64(2))
}

// --- Tiebreak direction ---

func TestTiebreakHigherReplicaIDWins(t *testing.T) {
	c := qt.New(t)
	// "z" > "a" lexicographically — "z" should win the tiebreak.
	a := New[string]("a")
	z := New[string]("z")

	da := a.Set("from-a", 10)
	dz := z.Set("from-z", 10)

	a.Merge(dz)
	z.Merge(da)

	for _, r := range []*LWWRegister[string]{a, z} {
		v, _, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "from-z")
	}
}

func TestThreeWaySameTimestampTiebreak(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")
	x := New[string]("c")

	da := a.Set("from-a", 5)
	db := b.Set("from-b", 5)
	dx := x.Set("from-c", 5)

	a.Merge(db)
	a.Merge(dx)
	b.Merge(da)
	b.Merge(dx)
	x.Merge(da)
	x.Merge(db)

	// All converge; "c" > "b" > "a" so "from-c" wins.
	for _, r := range []*LWWRegister[string]{a, b, x} {
		v, _, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "from-c")
	}
}

// --- Overwrite clears old dots ---

func TestOverwriteCleansStore(t *testing.T) {
	c := qt.New(t)
	r := New[string]("a")
	r.Set("first", 1)
	r.Set("second", 2)
	r.Set("third", 3)

	// Store should have exactly one dot, not three.
	count := 0
	r.state.Store.Range(func(_ dotcontext.Dot, _ timestamped[string]) bool {
		count++
		return true
	})
	c.Assert(count, qt.Equals, 1)
}

// --- Overwrite after concurrent merge includes foreign dots ---

func TestOverwriteAfterConcurrentMerge(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")

	da := a.Set("from-a", 1)
	db := b.Set("from-b", 2)

	// a sees both values (concurrent).
	a.Merge(db)

	// a overwrites — delta context should include both old dots.
	d3 := a.Set("resolved", 3)

	// Fresh replica merges only the overwrite delta.
	x := New[string]("x")
	x.Merge(da)
	x.Merge(db)
	x.Merge(d3)

	v, ts, ok := x.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "resolved")
	c.Assert(ts, qt.Equals, int64(3))
}

// --- Merge with empty register ---

func TestMergeIntoEmpty(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	a.Set("hello", 5)

	b := New[string]("b")
	b.Merge(a)

	v, ts, ok := b.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "hello")
	c.Assert(ts, qt.Equals, int64(5))
}

func TestMergeEmptyIntoSet(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	a.Set("hello", 5)

	empty := New[string]("b")
	a.Merge(empty)

	v, ts, ok := a.Value()
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, "hello")
	c.Assert(ts, qt.Equals, int64(5))
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	c := qt.New(t)
	a := New[string]("a")
	b := New[string]("b")
	x := New[string]("c")

	da := a.Set("va", 1)
	db := b.Set("vb", 2)
	dx := x.Set("vc", 3)

	a.Merge(db)
	a.Merge(dx)
	b.Merge(da)
	b.Merge(dx)
	x.Merge(da)
	x.Merge(db)

	for _, r := range []*LWWRegister[string]{a, b, x} {
		v, ts, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "vc")
		c.Assert(ts, qt.Equals, int64(3))
	}
}
