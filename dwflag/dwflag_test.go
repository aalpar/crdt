package dwflag

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEnabled", func(c *qt.C) {
		f := New("a")
		c.Assert(f.Value(), qt.IsTrue)
	})

	c.Run("EnableDisable", func(c *qt.C) {
		f := New("a")
		f.Disable()
		c.Assert(f.Value(), qt.IsFalse)
		f.Enable()
		c.Assert(f.Value(), qt.IsTrue)
	})

	c.Run("ReDisable", func(c *qt.C) {
		f := New("a")
		f.Disable()
		f.Enable()
		f.Disable()
		c.Assert(f.Value(), qt.IsFalse)
	})

	c.Run("EnableWhenAlreadyEnabled", func(c *qt.C) {
		f := New("a")
		f.Enable() // should not panic
		c.Assert(f.Value(), qt.IsTrue)
	})

	c.Run("MultipleDisablesSameReplica", func(c *qt.C) {
		f := New("a")
		f.Disable()
		f.Disable()
		f.Disable()

		c.Assert(f.Value(), qt.IsFalse)

		// Three disables = three dots in the store.
		c.Assert(f.state.Store.Len(), qt.Equals, 3)

		// A single enable clears all of them.
		f.Enable()
		c.Assert(f.Value(), qt.IsTrue)
		c.Assert(f.state.Store.Len(), qt.Equals, 0)
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Disable", func(c *qt.C) {
		f := New("a")
		delta := f.Disable()
		c.Assert(delta.Value(), qt.IsFalse)
	})

	c.Run("Enable", func(c *qt.C) {
		f := New("a")
		f.Disable()
		delta := f.Enable()
		c.Assert(delta.Value(), qt.IsTrue)
	})
}

func TestDisableWins(t *testing.T) {
	c := qt.New(t)

	c.Run("ConcurrentEnableDisable", func(c *qt.C) {
		// Both replicas share a disabled state.
		a := New("a")
		disableDelta := a.Disable()
		b := New("b")
		b.Merge(disableDelta)

		// a enables (clears dots), b disables (adds new dot).
		enableDelta := a.Enable()
		reDisableDelta := b.Disable()

		a.Merge(reDisableDelta)
		b.Merge(enableDelta)

		// Disable wins: b's new dot survives a's enable context.
		c.Assert(a.Value(), qt.IsFalse)
		c.Assert(b.Value(), qt.IsFalse)
	})

	c.Run("ConcurrentEnables", func(c *qt.C) {
		// Both replicas share a disabled state.
		a := New("a")
		b := New("b")
		disA := a.Disable()
		disB := b.Disable()
		a.Merge(disB)
		b.Merge(disA)

		// Both enable concurrently.
		enA := a.Enable()
		enB := b.Enable()

		a.Merge(enB)
		b.Merge(enA)

		c.Assert(a.Value(), qt.IsTrue)
		c.Assert(b.Value(), qt.IsTrue)
	})

	c.Run("ConcurrentDisables", func(c *qt.C) {
		a := New("a")
		b := New("b")

		// Both disable concurrently.
		da := a.Disable()
		db := b.Disable()

		a.Merge(db)
		b.Merge(da)

		c.Assert(a.Value(), qt.IsFalse)
		c.Assert(b.Value(), qt.IsFalse)
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("EnableDelta", func(c *qt.C) {
		a := New("a")
		b := New("b")

		disableDelta := a.Disable()
		b.Merge(disableDelta)

		enableDelta := a.Enable()
		b.Merge(enableDelta)

		c.Assert(b.Value(), qt.IsTrue)
	})

	c.Run("EnableNoOpDeltaHarmless", func(c *qt.C) {
		a := New("a")
		b := New("b")

		// a is enabled; its enable delta has empty context.
		noop := a.Enable()

		b.Disable()
		b.Merge(noop)

		// b's disable should survive — noop delta observes nothing.
		c.Assert(b.Value(), qt.IsFalse)
	})
}

func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New("a")
		a.Disable()

		state := a.State()
		b := FromCausal(state)

		c.Assert(b.Value(), qt.IsFalse)
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New("a")
		delta := a.Disable()

		reconstructed := FromCausal(delta.State())

		b := New("b")
		b.Merge(reconstructed)

		c.Assert(b.Value(), qt.IsFalse)
	})
}

