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
