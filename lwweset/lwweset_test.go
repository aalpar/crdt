package lwweset

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
		s.Add("x", 1)
		s.Add("y", 2)

		c.Assert(s.Has("x"), qt.IsTrue)
		c.Assert(s.Has("y"), qt.IsTrue)
		c.Assert(s.Has("z"), qt.IsFalse)
		c.Assert(s.Len(), qt.Equals, 2)
	})

	c.Run("RemoveAbsent", func(c *qt.C) {
		s := New[string]()
		s.Remove("ghost", 1) // never added
		c.Assert(s.Has("ghost"), qt.IsFalse)
		c.Assert(s.Len(), qt.Equals, 0)
	})

	c.Run("AddThenRemove", func(c *qt.C) {
		s := New[string]()
		s.Add("x", 1)
		s.Remove("x", 2)

		c.Assert(s.Has("x"), qt.IsFalse)
		c.Assert(s.Len(), qt.Equals, 0)
	})

	c.Run("RemoveThenReadd", func(c *qt.C) {
		s := New[string]()
		s.Add("x", 1)
		s.Remove("x", 2)
		s.Add("x", 3)

		c.Assert(s.Has("x"), qt.IsTrue)
		c.Assert(s.Len(), qt.Equals, 1)
	})

	c.Run("Elements", func(c *qt.C) {
		s := New[int]()
		s.Add(3, 1)
		s.Add(1, 2)
		s.Add(2, 3)

		elems := s.Elements()
		slices.Sort(elems)
		c.Assert(elems, qt.DeepEquals, []int{1, 2, 3})
	})
}

func TestTimestampResolution(t *testing.T) {
	c := qt.New(t)

	c.Run("HigherAddWins", func(c *qt.C) {
		s := New[string]()
		s.Add("x", 5)
		s.Remove("x", 3) // remove ts < add ts

		c.Assert(s.Has("x"), qt.IsTrue)
	})

	c.Run("HigherRemoveWins", func(c *qt.C) {
		s := New[string]()
		s.Add("x", 3)
		s.Remove("x", 5) // remove ts > add ts

		c.Assert(s.Has("x"), qt.IsFalse)
	})

	c.Run("EqualTimestampAddWins", func(c *qt.C) {
		s := New[string]()
		s.Add("x", 5)
		s.Remove("x", 5) // same timestamp: add wins

		c.Assert(s.Has("x"), qt.IsTrue)
	})

	c.Run("LaterAddOverridesRemove", func(c *qt.C) {
		s := New[string]()
		s.Remove("x", 10) // remove arrives before add
		s.Add("x", 15)    // later add wins

		c.Assert(s.Has("x"), qt.IsTrue)
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("AddDeltaContainsOnlyAddedElem", func(c *qt.C) {
		s := New[string]()
		s.Add("x", 1)
		delta := s.Add("y", 2)

		c.Assert(delta.Has("y"), qt.IsTrue)
		c.Assert(delta.Has("x"), qt.IsFalse)
		c.Assert(delta.Len(), qt.Equals, 1)
	})

	c.Run("RemoveDeltaDoesNotAddElement", func(c *qt.C) {
		s := New[string]()
		s.Add("x", 1)
		delta := s.Remove("x", 2)

		c.Assert(delta.Has("x"), qt.IsFalse)
		c.Assert(delta.Len(), qt.Equals, 0)
	})
}

func TestMerge(t *testing.T) {
	c := qt.New(t)

	c.Run("MergeAdds", func(c *qt.C) {
		a := New[string]()
		b := New[string]()

		da := a.Add("x", 1)
		db := b.Add("y", 2)

		a.Merge(db)
		b.Merge(da)

		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(a.Has("y"), qt.IsTrue)
		c.Assert(b.Has("x"), qt.IsTrue)
		c.Assert(b.Has("y"), qt.IsTrue)
	})

	c.Run("ConcurrentAddRemoveLaterAddWins", func(c *qt.C) {
		a := New[string]()
		b := New[string]()

		// a adds x at ts=5, b removes x at ts=3 (concurrent, different replicas)
		da := a.Add("x", 5)
		db := b.Remove("x", 3)

		a.Merge(db)
		b.Merge(da)

		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(b.Has("x"), qt.IsTrue)
	})

	c.Run("ConcurrentAddRemoveLaterRemoveWins", func(c *qt.C) {
		a := New[string]()
		b := New[string]()

		// a adds x at ts=3, b removes x at ts=5 (concurrent, different replicas)
		da := a.Add("x", 3)
		db := b.Remove("x", 5)

		a.Merge(db)
		b.Merge(da)

		c.Assert(a.Has("x"), qt.IsFalse)
		c.Assert(b.Has("x"), qt.IsFalse)
	})

	c.Run("RemoveSeenBeforeAdd", func(c *qt.C) {
		// b gets the remove delta before it has seen the add
		a := New[string]()
		b := New[string]()

		a.Add("x", 5)
		rmDelta := a.Remove("x", 7)

		b.Merge(rmDelta) // b sees remove first
		c.Assert(b.Has("x"), qt.IsFalse)

		addDelta := New[string]()
		addDelta.added["x"] = 5 // synthesize the add delta
		b.Merge(addDelta)       // b now gets the add (ts=5 < rm ts=7)

		c.Assert(b.Has("x"), qt.IsFalse) // remove still wins
	})

	c.Run("MaxTimestampWinsOnMerge", func(c *qt.C) {
		a := New[string]()
		b := New[string]()

		a.Add("x", 3)
		b.Add("x", 7) // b has a later add

		a.Merge(b)
		c.Assert(a.added["x"], qt.Equals, int64(7))
	})
}
