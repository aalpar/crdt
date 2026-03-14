package gset

import (
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		s := New[string]()
		c.Assert(s.Len(), qt.Equals, 0)
		c.Assert(s.Has("x"), qt.IsFalse)
		c.Assert(s.Elements(), qt.HasLen, 0)
	})

	c.Run("AddHas", func(c *qt.C) {
		s := New[string]()
		s.Add("x")
		s.Add("y")

		c.Assert(s.Has("x"), qt.IsTrue)
		c.Assert(s.Has("y"), qt.IsTrue)
		c.Assert(s.Has("z"), qt.IsFalse)
		c.Assert(s.Len(), qt.Equals, 2)
	})

	c.Run("AddIdempotent", func(c *qt.C) {
		s := New[string]()
		s.Add("x")
		s.Add("x")

		c.Assert(s.Has("x"), qt.IsTrue)
		c.Assert(s.Len(), qt.Equals, 1)
	})

	c.Run("Elements", func(c *qt.C) {
		s := New[int]()
		s.Add(3)
		s.Add(1)
		s.Add(2)

		elems := s.Elements()
		slices.Sort(elems)
		c.Assert(elems, qt.DeepEquals, []int{1, 2, 3})
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("AddReturnsSingleton", func(c *qt.C) {
		s := New[string]()
		delta := s.Add("x")

		c.Assert(delta.Has("x"), qt.IsTrue)
		c.Assert(delta.Len(), qt.Equals, 1)
	})

	c.Run("DeltaDoesNotContainOtherElements", func(c *qt.C) {
		s := New[string]()
		s.Add("x")
		delta := s.Add("y")

		c.Assert(delta.Has("y"), qt.IsTrue)
		c.Assert(delta.Has("x"), qt.IsFalse)
		c.Assert(delta.Len(), qt.Equals, 1)
	})

	c.Run("DeltaAlreadyPresentElement", func(c *qt.C) {
		s := New[string]()
		s.Add("x")
		delta := s.Add("x") // already present

		c.Assert(delta.Has("x"), qt.IsTrue)
		c.Assert(delta.Len(), qt.Equals, 1)
		c.Assert(s.Len(), qt.Equals, 1)
	})
}

func TestMerge(t *testing.T) {
	c := qt.New(t)

	c.Run("UnionOfDisjointSets", func(c *qt.C) {
		a := New[string]()
		b := New[string]()
		a.Add("x")
		b.Add("y")

		a.Merge(b)
		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(a.Has("y"), qt.IsTrue)
		c.Assert(a.Len(), qt.Equals, 2)
	})

	c.Run("UnionOfOverlappingSets", func(c *qt.C) {
		a := New[string]()
		b := New[string]()
		a.Add("x")
		a.Add("y")
		b.Add("y")
		b.Add("z")

		a.Merge(b)
		c.Assert(a.Len(), qt.Equals, 3)
	})

	c.Run("ConcurrentAdds", func(c *qt.C) {
		a := New[string]()
		b := New[string]()

		da := a.Add("x")
		db := b.Add("y")

		a.Merge(db)
		b.Merge(da)

		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(a.Has("y"), qt.IsTrue)
		c.Assert(b.Has("x"), qt.IsTrue)
		c.Assert(b.Has("y"), qt.IsTrue)
	})

	c.Run("MergeIntoEmpty", func(c *qt.C) {
		a := New[string]()
		a.Add("x")
		a.Add("y")

		b := New[string]()
		b.Merge(a)

		c.Assert(b.Has("x"), qt.IsTrue)
		c.Assert(b.Has("y"), qt.IsTrue)
		c.Assert(b.Len(), qt.Equals, 2)
	})

	c.Run("MergeEmptyIntoPopulated", func(c *qt.C) {
		a := New[string]()
		a.Add("x")

		empty := New[string]()
		a.Merge(empty)

		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(a.Len(), qt.Equals, 1)
	})
}
