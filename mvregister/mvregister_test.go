package mvregister

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		r := New[string]("a")
		c.Assert(r.Values(), qt.HasLen, 0)
	})

	c.Run("WriteAndValues", func(c *qt.C) {
		r := New[string]("a")
		r.Write("hello")

		vals := r.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "hello")
	})

	c.Run("OverwriteReplaces", func(c *qt.C) {
		r := New[string]("a")
		r.Write("first")
		r.Write("second")

		vals := r.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "second")
	})

	c.Run("OverwriteCleansStore", func(c *qt.C) {
		r := New[string]("a")
		r.Write("first")
		r.Write("second")
		r.Write("third")

		// Store should have exactly one dot, not three.
		c.Assert(r.state.Store.Len(), qt.Equals, 1)
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Write", func(c *qt.C) {
		r := New[string]("a")
		delta := r.Write("x")

		vals := delta.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "x")
	})
}

func TestConflictResolution(t *testing.T) {
	c := qt.New(t)

	c.Run("ConcurrentWritesPreserved", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Write("from-a")
		db := b.Write("from-b")

		a.Merge(db)
		b.Merge(da)

		// Both values survive — no winner.
		for _, r := range []*MVRegister[string]{a, b} {
			vals := r.Values()
			c.Assert(vals, qt.HasLen, 2)
			c.Assert(vals, qt.ContentEquals, []string{"from-a", "from-b"})
		}
	})

	c.Run("SequentialWriteResolves", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Write("from-a")
		db := b.Write("from-b")

		// Both merge — now concurrent values coexist.
		a.Merge(db)
		b.Merge(da)

		// a writes again, superseding both concurrent values.
		d3 := a.Write("resolved")
		b.Merge(d3)

		for _, r := range []*MVRegister[string]{a, b} {
			vals := r.Values()
			c.Assert(vals, qt.HasLen, 1)
			c.Assert(vals[0], qt.Equals, "resolved")
		}
	})

	c.Run("ThreeWayConcurrentPreserved", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		da := a.Write("from-a")
		db := b.Write("from-b")
		dx := x.Write("from-c")

		a.Merge(db)
		a.Merge(dx)
		b.Merge(da)
		b.Merge(dx)
		x.Merge(da)
		x.Merge(db)

		for _, r := range []*MVRegister[string]{a, b, x} {
			vals := r.Values()
			c.Assert(vals, qt.HasLen, 3)
			c.Assert(vals, qt.ContentEquals, []string{"from-a", "from-b", "from-c"})
		}
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("OverwriteDeltaSupersedes", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		d1 := a.Write("first")
		b.Merge(d1)

		d2 := a.Write("second")
		b.Merge(d2)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "second")
	})

	c.Run("ConcurrentWriteThenOverwrite", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Write("from-a")
		db := b.Write("from-b")

		a.Merge(db)
		b.Merge(da)

		d3 := a.Write("from-a-again")
		b.Merge(d3)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "from-a-again")
	})
}

func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New[string]("a")
		a.Write("hello")

		state := a.State()
		b := FromCausal[string](state)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "hello")
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		delta := a.Write("x")

		reconstructed := FromCausal[string](delta.State())

		b := New[string]("b")
		b.Merge(reconstructed)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "x")
	})
}

