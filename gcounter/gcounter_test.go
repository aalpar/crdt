package gcounter

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewZero", func(c *qt.C) {
		r := New("a")
		c.Assert(r.Value(), qt.Equals, uint64(0))
	})

	c.Run("Increment", func(c *qt.C) {
		r := New("a")
		r.Increment(5)
		c.Assert(r.Value(), qt.Equals, uint64(5))
	})

	c.Run("MultipleIncrements", func(c *qt.C) {
		r := New("a")
		r.Increment(3)
		r.Increment(7)
		r.Increment(2)
		c.Assert(r.Value(), qt.Equals, uint64(12))
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Increment", func(c *qt.C) {
		r := New("a")
		delta := r.Increment(7)
		c.Assert(delta.Value(), qt.Equals, uint64(7))
	})

	c.Run("AccumulatedDelta", func(c *qt.C) {
		r := New("a")
		r.Increment(3)
		delta := r.Increment(7)
		c.Assert(delta.Value(), qt.Equals, uint64(10))
	})
}

func TestIncrementReplacesOnlyOwnDot(t *testing.T) {
	c := qt.New(t)

	c.Run("OneDotPerReplica", func(c *qt.C) {
		r := New("a")
		r.Increment(1)
		r.Increment(2)
		r.Increment(3)

		count := 0
		r.state.Store.Range(func(_ dotcontext.Dot, _ GValue) bool {
			count++
			return true
		})
		c.Assert(count, qt.Equals, 1)
		c.Assert(r.Value(), qt.Equals, uint64(6))
	})

	c.Run("ForeignDotsUntouched", func(c *qt.C) {
		a := New("a")
		b := New("b")

		da := a.Increment(3)
		db := b.Increment(7)

		a.Merge(db)

		// a's store should have exactly two dots: one per replica.
		count := 0
		a.state.Store.Range(func(_ dotcontext.Dot, _ GValue) bool {
			count++
			return true
		})
		c.Assert(count, qt.Equals, 2)
		c.Assert(a.Value(), qt.Equals, uint64(10))

		// Incrementing a should replace only a's dot, not b's.
		a.Increment(5)
		count = 0
		a.state.Store.Range(func(_ dotcontext.Dot, _ GValue) bool {
			count++
			return true
		})
		c.Assert(count, qt.Equals, 2)
		c.Assert(a.Value(), qt.Equals, uint64(15))

		_ = da
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("DeltaSupersedes", func(c *qt.C) {
		a := New("a")
		b := New("b")

		d1 := a.Increment(3)
		b.Merge(d1)

		d2 := a.Increment(7)
		b.Merge(d2)

		c.Assert(b.Value(), qt.Equals, uint64(10))
	})

	c.Run("IncrementAfterMerge", func(c *qt.C) {
		a := New("a")
		b := New("b")

		da := a.Increment(5)
		b.Merge(da)

		b.Increment(3)

		c.Assert(b.Value(), qt.Equals, uint64(8))
	})
}

func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New("a")
		a.Increment(5)
		a.Increment(3)

		state := a.State()
		b := FromCausal(state)

		c.Assert(b.Value(), qt.Equals, uint64(8))
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New("a")
		delta := a.Increment(7)

		reconstructed := FromCausal(delta.State())

		b := New("b")
		b.Merge(reconstructed)

		c.Assert(b.Value(), qt.Equals, uint64(7))
	})
}

