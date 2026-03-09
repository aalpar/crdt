package ewflag

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
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

func TestMergeProperties(t *testing.T) {
	c := qt.New(t)

	c.Run("Idempotent", func(c *qt.C) {
		a := New("a")
		a.Enable()

		snapshot := New("a")
		snapshot.Merge(a)

		a.Merge(snapshot)
		a.Merge(snapshot)

		c.Assert(a.Value(), qt.IsTrue)
	})

	c.Run("Commutative", func(c *qt.C) {
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
	})

	c.Run("Associative", func(c *qt.C) {
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
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("IncrementalEqualsFullMerge", func(c *qt.C) {
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
	})

	c.Run("DisableDelta", func(c *qt.C) {
		a := New("a")
		b := New("b")

		enableDelta := a.Enable()
		b.Merge(enableDelta)

		disableDelta := a.Disable()
		b.Merge(disableDelta)

		c.Assert(b.Value(), qt.IsFalse)
	})

	c.Run("DeltaDeltaMerge", func(c *qt.C) {
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

func TestMergeWithEmpty(t *testing.T) {
	c := qt.New(t)

	c.Run("IntoEmpty", func(c *qt.C) {
		a := New("a")
		a.Enable()

		b := New("b")
		b.Merge(a)
		c.Assert(b.Value(), qt.IsTrue)
	})

	c.Run("EmptyIntoEnabled", func(c *qt.C) {
		a := New("a")
		a.Enable()

		empty := New("b")
		a.Merge(empty)
		c.Assert(a.Value(), qt.IsTrue)
	})

	c.Run("EmptyIntoDisabled", func(c *qt.C) {
		a := New("a")
		// a is disabled by default

		empty := New("b")
		a.Merge(empty)
		c.Assert(a.Value(), qt.IsFalse)
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

func TestConvergence(t *testing.T) {
	c := qt.New(t)

	c.Run("ThreeReplica", func(c *qt.C) {
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
	})

	c.Run("FiveReplica", func(c *qt.C) {
		ids := []dotcontext.ReplicaID{"a", "b", "c", "d", "e"}
		replicas := make([]*EWFlag, len(ids))
		for i, id := range ids {
			replicas[i] = New(id)
		}

		// Mixed: most enable, one enable-then-disable.
		deltas := make([]*EWFlag, len(ids))
		deltas[0] = replicas[0].Enable()  // a enables
		deltas[1] = replicas[1].Enable()  // b enables
		replicas[2].Enable()              // c enables then disables
		deltas[2] = replicas[2].Disable() // c's net effect: disable
		deltas[3] = replicas[3].Enable()  // d enables
		deltas[4] = replicas[4].Enable()  // e enables

		// Full mesh merge.
		for i := range replicas {
			for j := range replicas {
				if i != j {
					replicas[i].Merge(deltas[j])
				}
			}
		}

		// Enable wins: a, b, d, e all contributed enable dots that
		// c's disable context doesn't observe.
		for i, r := range replicas {
			c.Assert(r.Value(), qt.IsTrue, qt.Commentf("replica %s", ids[i]))
		}
	})
}
