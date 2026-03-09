package pncounter

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewZero", func(c *qt.C) {
		r := New("a")
		c.Assert(r.Value(), qt.Equals, int64(0))
	})

	c.Run("Increment", func(c *qt.C) {
		r := New("a")
		r.Increment(5)
		c.Assert(r.Value(), qt.Equals, int64(5))
	})

	c.Run("MultipleIncrements", func(c *qt.C) {
		r := New("a")
		r.Increment(3)
		r.Increment(7)
		r.Increment(2)
		c.Assert(r.Value(), qt.Equals, int64(12))
	})

	c.Run("Decrement", func(c *qt.C) {
		r := New("a")
		r.Increment(10)
		r.Decrement(4)
		c.Assert(r.Value(), qt.Equals, int64(6))
	})

	c.Run("NegativeValue", func(c *qt.C) {
		r := New("a")
		r.Decrement(5)
		c.Assert(r.Value(), qt.Equals, int64(-5))
	})

	c.Run("DecrementToZero", func(c *qt.C) {
		r := New("a")
		r.Increment(5)
		r.Decrement(5)
		c.Assert(r.Value(), qt.Equals, int64(0))
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Increment", func(c *qt.C) {
		r := New("a")
		delta := r.Increment(7)
		c.Assert(delta.Value(), qt.Equals, int64(7))
	})
}

func TestConcurrentOps(t *testing.T) {
	c := qt.New(t)

	c.Run("Increments", func(c *qt.C) {
		a := New("a")
		b := New("b")

		da := a.Increment(3)
		db := b.Increment(5)

		a.Merge(db)
		b.Merge(da)

		c.Assert(a.Value(), qt.Equals, int64(8))
		c.Assert(b.Value(), qt.Equals, int64(8))
	})

	c.Run("IncrementDecrement", func(c *qt.C) {
		a := New("a")
		b := New("b")

		da := a.Increment(10)
		db := b.Decrement(3)

		a.Merge(db)
		b.Merge(da)

		c.Assert(a.Value(), qt.Equals, int64(7))
		c.Assert(b.Value(), qt.Equals, int64(7))
	})
}

func TestMergeProperties(t *testing.T) {
	c := qt.New(t)

	c.Run("Idempotent", func(c *qt.C) {
		a := New("a")
		a.Increment(5)

		snapshot := New("x")
		snapshot.Merge(a)

		a.Merge(snapshot)
		a.Merge(snapshot)

		c.Assert(a.Value(), qt.Equals, int64(5))
	})

	c.Run("Commutative", func(c *qt.C) {
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
	})

	c.Run("Associative", func(c *qt.C) {
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
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("IncrementalEqualsFullMerge", func(c *qt.C) {
		a := New("a")
		d1 := a.Increment(3)
		d2 := a.Increment(7)

		inc := New("b")
		inc.Merge(d1)
		inc.Merge(d2)

		full := New("b")
		full.Merge(a)

		c.Assert(inc.Value(), qt.Equals, full.Value())
	})

	c.Run("DeltaSupersedes", func(c *qt.C) {
		a := New("a")
		b := New("b")

		d1 := a.Increment(3)
		b.Merge(d1)

		d2 := a.Increment(7)
		b.Merge(d2)

		c.Assert(b.Value(), qt.Equals, int64(10))
	})

	c.Run("DeltaDeltaMerge", func(c *qt.C) {
		a := New("a")
		d1 := a.Increment(3)
		d2 := a.Increment(7)

		// Combine deltas, then apply.
		d1.Merge(d2)

		b := New("b")
		b.Merge(d1)

		c.Assert(b.Value(), qt.Equals, int64(10))
	})

	c.Run("IncrementAfterMerge", func(c *qt.C) {
		a := New("a")
		b := New("b")

		da := a.Increment(5)
		b.Merge(da)

		b.Increment(3)

		c.Assert(b.Value(), qt.Equals, int64(8))
	})
}

func TestOneDotPerReplica(t *testing.T) {
	c := qt.New(t)

	c.Run("MultipleOps", func(c *qt.C) {
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
	})

	c.Run("IncrementReplacesOnlyOwnDot", func(c *qt.C) {
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
	})
}

func TestMergeWithEmpty(t *testing.T) {
	c := qt.New(t)

	c.Run("IntoEmpty", func(c *qt.C) {
		a := New("a")
		a.Increment(42)

		b := New("b")
		b.Merge(a)
		c.Assert(b.Value(), qt.Equals, int64(42))
	})

	c.Run("EmptyIntoSet", func(c *qt.C) {
		a := New("a")
		a.Increment(42)

		empty := New("b")
		a.Merge(empty)
		c.Assert(a.Value(), qt.Equals, int64(42))
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

		c.Assert(b.Value(), qt.Equals, int64(8))
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New("a")
		delta := a.Increment(7)

		reconstructed := FromCausal(delta.State())

		b := New("b")
		b.Merge(reconstructed)

		c.Assert(b.Value(), qt.Equals, int64(7))
	})
}

func TestConvergence(t *testing.T) {
	c := qt.New(t)

	c.Run("ThreeReplica", func(c *qt.C) {
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
	})

	c.Run("ThreeReplicaMixedOps", func(c *qt.C) {
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
	})

	c.Run("FiveReplica", func(c *qt.C) {
		ids := []dotcontext.ReplicaID{"a", "b", "c", "d", "e"}
		replicas := make([]*Counter, len(ids))
		for i, id := range ids {
			replicas[i] = New(id)
		}

		// Mixed operations: some increment, some decrement.
		deltas := make([]*Counter, len(ids))
		deltas[0] = replicas[0].Increment(10) // +10
		deltas[1] = replicas[1].Decrement(3)  // -3
		deltas[2] = replicas[2].Increment(7)  // +7
		replicas[3].Increment(5)              // +5 (not saved as delta)
		deltas[3] = replicas[3].Decrement(2)  // cumulative = 3
		deltas[4] = replicas[4].Increment(1)  // +1

		// Full mesh merge.
		for i := range replicas {
			for j := range replicas {
				if i != j {
					replicas[i].Merge(deltas[j])
				}
			}
		}

		// 10 + (-3) + 7 + 3 + 1 = 18
		for i, r := range replicas {
			c.Assert(r.Value(), qt.Equals, int64(18), qt.Commentf("replica %s", ids[i]))
		}
	})
}
