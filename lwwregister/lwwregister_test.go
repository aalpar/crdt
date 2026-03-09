package lwwregister

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		r := New[string]("a")
		_, _, ok := r.Value()
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("SetAndValue", func(c *qt.C) {
		r := New[string]("a")
		r.Set("hello", 1)

		v, ts, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "hello")
		c.Assert(ts, qt.Equals, int64(1))
	})

	c.Run("Overwrite", func(c *qt.C) {
		r := New[string]("a")
		r.Set("first", 1)
		r.Set("second", 2)

		v, ts, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "second")
		c.Assert(ts, qt.Equals, int64(2))
	})

	c.Run("IntegerRegister", func(c *qt.C) {
		r := New[int]("a")
		r.Set(42, 1)
		r.Set(99, 2)

		v, _, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, 99)
	})

	c.Run("OverwriteCleansStore", func(c *qt.C) {
		r := New[string]("a")
		r.Set("first", 1)
		r.Set("second", 2)
		r.Set("third", 3)

		// Store should have exactly one dot, not three.
		count := 0
		r.state.Store.Range(func(_ dotcontext.Dot, _ Timestamped[string]) bool {
			count++
			return true
		})
		c.Assert(count, qt.Equals, 1)
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Set", func(c *qt.C) {
		r := New[string]("a")
		delta := r.Set("x", 10)

		v, ts, ok := delta.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "x")
		c.Assert(ts, qt.Equals, int64(10))
	})
}

func TestConflictResolution(t *testing.T) {
	c := qt.New(t)

	c.Run("HigherTimestampWins", func(c *qt.C) {
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
	})

	c.Run("SameTimestampTiebreak", func(c *qt.C) {
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
	})

	c.Run("TiebreakHigherReplicaIDWins", func(c *qt.C) {
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
	})

	c.Run("ThreeWaySameTimestampTiebreak", func(c *qt.C) {
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
	})

	c.Run("OverwriteAfterConcurrentMerge", func(c *qt.C) {
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
	})
}

func TestTimestampedJoin(t *testing.T) {
	c := qt.New(t)

	c.Run("HigherWins", func(c *qt.C) {
		a := Timestamped[string]{Value: "old", Ts: 10}
		b := Timestamped[string]{Value: "new", Ts: 20}

		c.Assert(a.Join(b), qt.Equals, b)
		c.Assert(b.Join(a), qt.Equals, b) // commutative
	})

	c.Run("SameTimestamp", func(c *qt.C) {
		a := Timestamped[string]{Value: "a-val", Ts: 10}
		b := Timestamped[string]{Value: "b-val", Ts: 10}

		// Equal timestamps: receiver wins (first argument).
		c.Assert(a.Join(b), qt.Equals, a)
		c.Assert(b.Join(a), qt.Equals, b)
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("OverwriteDeltaSupersedes", func(c *qt.C) {
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
	})

	c.Run("ConcurrentWriteThenOverwrite", func(c *qt.C) {
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
	})
}

func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New[string]("a")
		a.Set("hello", 5)

		state := a.State()
		b := FromCausal[string](state)

		v, ts, ok := b.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "hello")
		c.Assert(ts, qt.Equals, int64(5))
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		delta := a.Set("x", 10)

		reconstructed := FromCausal[string](delta.State())

		b := New[string]("b")
		b.Merge(reconstructed)

		v, ts, ok := b.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "x")
		c.Assert(ts, qt.Equals, int64(10))
	})
}

