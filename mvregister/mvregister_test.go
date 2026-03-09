package mvregister

import (
	"sort"
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

func TestMergeProperties(t *testing.T) {
	c := qt.New(t)

	c.Run("Idempotent", func(c *qt.C) {
		a := New[string]("a")
		a.Write("x")

		snapshot := New[string]("a")
		snapshot.Merge(a)

		a.Merge(snapshot)
		a.Merge(snapshot)

		c.Assert(a.Values(), qt.HasLen, 1)
		c.Assert(a.Values()[0], qt.Equals, "x")
	})

	c.Run("Commutative", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		a.Write("va")
		b.Write("vb")

		ab := New[string]("x")
		ab.Merge(a)
		ab.Merge(b)

		ba := New[string]("x")
		ba.Merge(b)
		ba.Merge(a)

		valsAB := ab.Values()
		valsBA := ba.Values()
		sort.Strings(valsAB)
		sort.Strings(valsBA)
		c.Assert(valsAB, qt.DeepEquals, valsBA)
	})

	c.Run("Associative", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		a.Write("va")
		b.Write("vb")
		x.Write("vc")

		// (a ⊔ b) ⊔ c
		ab := New[string]("ab")
		ab.Merge(a)
		ab.Merge(b)
		abc := New[string]("abc")
		abc.Merge(ab)
		abc.Merge(x)

		// a ⊔ (b ⊔ c)
		bc := New[string]("bc")
		bc.Merge(b)
		bc.Merge(x)
		abc2 := New[string]("abc2")
		abc2.Merge(a)
		abc2.Merge(bc)

		vals1 := abc.Values()
		vals2 := abc2.Values()
		sort.Strings(vals1)
		sort.Strings(vals2)
		c.Assert(vals1, qt.DeepEquals, vals2)
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("IncrementalEqualsFullMerge", func(c *qt.C) {
		a := New[string]("a")
		d1 := a.Write("first")
		d2 := a.Write("second")

		inc := New[string]("b")
		inc.Merge(d1)
		inc.Merge(d2)

		full := New[string]("b")
		full.Merge(a)

		incVals := inc.Values()
		fullVals := full.Values()
		sort.Strings(incVals)
		sort.Strings(fullVals)
		c.Assert(incVals, qt.DeepEquals, fullVals)
	})

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

	c.Run("DeltaDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		d1 := a.Write("first")
		d2 := a.Write("second")

		// Combine deltas, then apply.
		d1.Merge(d2)

		b := New[string]("b")
		b.Merge(d1)

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

func TestMergeWithEmpty(t *testing.T) {
	c := qt.New(t)

	c.Run("IntoEmpty", func(c *qt.C) {
		a := New[string]("a")
		a.Write("hello")

		b := New[string]("b")
		b.Merge(a)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "hello")
	})

	c.Run("EmptyIntoSet", func(c *qt.C) {
		a := New[string]("a")
		a.Write("hello")

		empty := New[string]("b")
		a.Merge(empty)

		vals := a.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "hello")
	})
}
