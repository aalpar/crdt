package dwflag

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
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

func TestMergeProperties(t *testing.T) {
	c := qt.New(t)

	c.Run("Idempotent", func(c *qt.C) {
		a := New("a")
		a.Disable()

		snapshot := New("a")
		snapshot.Merge(a)

		a.Merge(snapshot)
		a.Merge(snapshot)

		c.Assert(a.Value(), qt.IsFalse)
	})

	c.Run("Commutative", func(c *qt.C) {
		a := New("a")
		b := New("b")
		a.Disable()

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

		a.Disable()
		b.Disable()
		b.Enable()
		x.Disable()

		// (a join b) join c
		ab := New("ab")
		ab.Merge(a)
		ab.Merge(b)
		abc := New("abc")
		abc.Merge(ab)
		abc.Merge(x)

		// a join (b join c)
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
		d1 := a.Disable()
		d2 := a.Enable()
		d3 := a.Disable()

		incremental := New("b")
		incremental.Merge(d1)
		incremental.Merge(d2)
		incremental.Merge(d3)

		full := New("b")
		full.Merge(a)

		c.Assert(incremental.Value(), qt.Equals, full.Value())
	})

	c.Run("EnableDelta", func(c *qt.C) {
		a := New("a")
		b := New("b")

		disableDelta := a.Disable()
		b.Merge(disableDelta)

		enableDelta := a.Enable()
		b.Merge(enableDelta)

		c.Assert(b.Value(), qt.IsTrue)
	})

	c.Run("DeltaDeltaMerge", func(c *qt.C) {
		a := New("a")
		d1 := a.Disable()
		d2 := a.Enable()
		d3 := a.Disable()

		// Merge deltas together, then apply combined delta.
		d1.Merge(d2)
		d1.Merge(d3)

		b := New("b")
		b.Merge(d1)

		c.Assert(b.Value(), qt.IsFalse)
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

func TestMergeWithEmpty(t *testing.T) {
	c := qt.New(t)

	c.Run("IntoEmpty", func(c *qt.C) {
		a := New("a")
		a.Disable()

		b := New("b")
		b.Merge(a)
		c.Assert(b.Value(), qt.IsFalse)
	})

	c.Run("EmptyIntoEnabled", func(c *qt.C) {
		a := New("a")
		// a is enabled by default

		empty := New("b")
		a.Merge(empty)
		c.Assert(a.Value(), qt.IsTrue)
	})

	c.Run("EmptyIntoDisabled", func(c *qt.C) {
		a := New("a")
		a.Disable()

		empty := New("b")
		a.Merge(empty)
		c.Assert(a.Value(), qt.IsFalse)
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

func TestConvergence(t *testing.T) {
	c := qt.New(t)

	c.Run("ThreeReplica", func(c *qt.C) {
		a := New("a")
		b := New("b")
		x := New("c")

		da := a.Disable()
		// b stays enabled
		dx := x.Disable()

		a.Merge(b)
		a.Merge(dx)
		b.Merge(da)
		b.Merge(dx)
		x.Merge(da)
		x.Merge(b)

		c.Assert(a.Value(), qt.Equals, b.Value())
		c.Assert(b.Value(), qt.Equals, x.Value())
		c.Assert(a.Value(), qt.IsFalse)
	})

	c.Run("FiveReplica", func(c *qt.C) {
		ids := []dotcontext.ReplicaID{"a", "b", "c", "d", "e"}
		replicas := make([]*DWFlag, len(ids))
		for i, id := range ids {
			replicas[i] = New(id)
		}

		// Mixed: most disable, one disable-then-enable.
		deltas := make([]*DWFlag, len(ids))
		deltas[0] = replicas[0].Disable()  // a disables
		deltas[1] = replicas[1].Disable()  // b disables
		replicas[2].Disable()              // c disables then enables
		deltas[2] = replicas[2].Enable()   // c's net effect: enable
		deltas[3] = replicas[3].Disable()  // d disables
		deltas[4] = replicas[4].Disable()  // e disables

		// Full mesh merge.
		for i := range replicas {
			for j := range replicas {
				if i != j {
					replicas[i].Merge(deltas[j])
				}
			}
		}

		// Disable wins: a, b, d, e all contributed disable dots that
		// c's enable context doesn't observe.
		for i, r := range replicas {
			c.Assert(r.Value(), qt.IsFalse, qt.Commentf("replica %s", ids[i]))
		}
	})
}
