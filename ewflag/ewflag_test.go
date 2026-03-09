package ewflag

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNewDisabled(t *testing.T) {
	c := qt.New(t)
	f := New("a")
	c.Assert(f.Value(), qt.IsFalse)
}

func TestEnableDisable(t *testing.T) {
	c := qt.New(t)
	f := New("a")
	f.Enable()
	c.Assert(f.Value(), qt.IsTrue)
	f.Disable()
	c.Assert(f.Value(), qt.IsFalse)
}

func TestReEnable(t *testing.T) {
	c := qt.New(t)
	f := New("a")
	f.Enable()
	f.Disable()
	f.Enable()
	c.Assert(f.Value(), qt.IsTrue)
}

func TestEnableReturnsValidDelta(t *testing.T) {
	c := qt.New(t)
	f := New("a")
	delta := f.Enable()
	c.Assert(delta.Value(), qt.IsTrue)
}

func TestDisableReturnsValidDelta(t *testing.T) {
	c := qt.New(t)
	f := New("a")
	f.Enable()
	delta := f.Disable()
	c.Assert(delta.Value(), qt.IsFalse)
}

func TestDisableWhenAlreadyDisabled(t *testing.T) {
	c := qt.New(t)
	f := New("a")
	f.Disable() // should not panic
	c.Assert(f.Value(), qt.IsFalse)
}

// --- Enable-wins semantics ---

func TestEnableWinsConcurrent(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	enableDelta := a.Enable()
	b := New("b")
	b.Merge(enableDelta)

	disableDelta := a.Disable()
	reEnableDelta := b.Enable()

	a.Merge(reEnableDelta)
	b.Merge(disableDelta)

	c.Assert(a.Value(), qt.IsTrue)
	c.Assert(b.Value(), qt.IsTrue)
}

func TestConcurrentEnables(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")

	da := a.Enable()
	db := b.Enable()

	a.Merge(db)
	b.Merge(da)

	c.Assert(a.Value(), qt.IsTrue)
	c.Assert(b.Value(), qt.IsTrue)
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	a.Enable()

	snapshot := New("a")
	snapshot.Merge(a)

	a.Merge(snapshot)
	a.Merge(snapshot)

	c.Assert(a.Value(), qt.IsTrue)
}

func TestMergeCommutative(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")
	a.Enable()

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
	d1 := a.Enable()
	d2 := a.Disable()
	d3 := a.Enable()

	incremental := New("b")
	incremental.Merge(d1)
	incremental.Merge(d2)
	incremental.Merge(d3)

	full := New("b")
	full.Merge(a)

	c.Assert(incremental.Value(), qt.Equals, full.Value())
}

func TestMergeDisableDelta(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")

	enableDelta := a.Enable()
	b.Merge(enableDelta)

	disableDelta := a.Disable()
	b.Merge(disableDelta)

	c.Assert(b.Value(), qt.IsFalse)
}

// --- Merge associativity ---

func TestMergeAssociative(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")
	x := New("c")

	a.Enable()
	b.Enable()
	b.Disable()
	x.Enable()

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

// --- Concurrent disables ---

func TestConcurrentDisablesSameFlag(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")

	// Both see enabled.
	enableDelta := a.Enable()
	b.Merge(enableDelta)

	// Both disable concurrently.
	disA := a.Disable()
	disB := b.Disable()

	a.Merge(disB)
	b.Merge(disA)

	c.Assert(a.Value(), qt.IsFalse)
	c.Assert(b.Value(), qt.IsFalse)
}

// --- Enable multiple times accumulates dots ---

func TestMultipleEnablesSameReplica(t *testing.T) {
	c := qt.New(t)
	f := New("a")
	f.Enable()
	f.Enable()
	f.Enable()

	c.Assert(f.Value(), qt.IsTrue)

	// Three enables = three dots in the store.
	c.Assert(f.state.Store.Len(), qt.Equals, 3)

	// A single disable clears all of them.
	f.Disable()
	c.Assert(f.Value(), qt.IsFalse)
	c.Assert(f.state.Store.Len(), qt.Equals, 0)
}

// --- Delta-delta merge ---

func TestDeltaDeltaMerge(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	d1 := a.Enable()
	d2 := a.Disable()
	d3 := a.Enable()

	// Merge deltas together, then apply combined delta.
	d1.Merge(d2)
	d1.Merge(d3)

	b := New("b")
	b.Merge(d1)

	c.Assert(b.Value(), qt.IsTrue)
}

// --- Disable no-op delta is harmless ---

func TestDisableNoOpDeltaMerge(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")

	// a is disabled; its disable delta has empty context.
	noop := a.Disable()

	b.Enable()
	b.Merge(noop)

	// b's enable should survive — noop delta observes nothing.
	c.Assert(b.Value(), qt.IsTrue)
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	c := qt.New(t)
	a := New("a")
	b := New("b")
	x := New("c")

	da := a.Enable()
	// b stays disabled
	dx := x.Enable()

	a.Merge(b)
	a.Merge(dx)
	b.Merge(da)
	b.Merge(dx)
	x.Merge(da)
	x.Merge(b)

	c.Assert(a.Value(), qt.Equals, b.Value())
	c.Assert(b.Value(), qt.Equals, x.Value())
	c.Assert(a.Value(), qt.IsTrue)
}
