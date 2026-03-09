package ewflag

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewDisabled", func(c *qt.C) {
		f := New("a")
		c.Assert(f.Value(), qt.IsFalse)
	})

	c.Run("EnableDisable", func(c *qt.C) {
		f := New("a")
		f.Enable()
		c.Assert(f.Value(), qt.IsTrue)
		f.Disable()
		c.Assert(f.Value(), qt.IsFalse)
	})

	c.Run("ReEnable", func(c *qt.C) {
		f := New("a")
		f.Enable()
		f.Disable()
		f.Enable()
		c.Assert(f.Value(), qt.IsTrue)
	})

	c.Run("DisableWhenAlreadyDisabled", func(c *qt.C) {
		f := New("a")
		f.Disable() // should not panic
		c.Assert(f.Value(), qt.IsFalse)
	})

	c.Run("MultipleEnablesSameReplica", func(c *qt.C) {
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
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Enable", func(c *qt.C) {
		f := New("a")
		delta := f.Enable()
		c.Assert(delta.Value(), qt.IsTrue)
	})

	c.Run("Disable", func(c *qt.C) {
		f := New("a")
		f.Enable()
		delta := f.Disable()
		c.Assert(delta.Value(), qt.IsFalse)
	})
}

func TestEnableWins(t *testing.T) {
	c := qt.New(t)

	c.Run("ConcurrentEnableDisable", func(c *qt.C) {
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
	})

	c.Run("ConcurrentEnables", func(c *qt.C) {
		a := New("a")
		b := New("b")

		da := a.Enable()
		db := b.Enable()

		a.Merge(db)
		b.Merge(da)

		c.Assert(a.Value(), qt.IsTrue)
		c.Assert(b.Value(), qt.IsTrue)
	})

	c.Run("ConcurrentDisables", func(c *qt.C) {
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
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("DisableDelta", func(c *qt.C) {
		a := New("a")
		b := New("b")

		enableDelta := a.Enable()
		b.Merge(enableDelta)

		disableDelta := a.Disable()
		b.Merge(disableDelta)

		c.Assert(b.Value(), qt.IsFalse)
	})

	c.Run("DisableNoOpDeltaHarmless", func(c *qt.C) {
		a := New("a")
		b := New("b")

		// a is disabled; its disable delta has empty context.
		noop := a.Disable()

		b.Enable()
		b.Merge(noop)

		// b's enable should survive — noop delta observes nothing.
		c.Assert(b.Value(), qt.IsTrue)
	})
}

func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New("a")
		a.Enable()

		state := a.State()
		b := FromCausal(state)

		c.Assert(b.Value(), qt.IsTrue)
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New("a")
		delta := a.Enable()

		reconstructed := FromCausal(delta.State())

		b := New("b")
		b.Merge(reconstructed)

		c.Assert(b.Value(), qt.IsTrue)
	})
}

