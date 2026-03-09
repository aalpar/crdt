package awset

import (
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		s := New[string]("a")
		c.Assert(s.Len(), qt.Equals, 0)
		c.Assert(s.Has("x"), qt.IsFalse)
		c.Assert(s.Elements(), qt.HasLen, 0)
	})

	c.Run("AddHas", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Add("y")

		c.Assert(s.Has("x"), qt.IsTrue)
		c.Assert(s.Has("y"), qt.IsTrue)
		c.Assert(s.Has("z"), qt.IsFalse)
		c.Assert(s.Len(), qt.Equals, 2)
	})

	c.Run("Remove", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Add("y")
		s.Remove("x")

		c.Assert(s.Has("x"), qt.IsFalse)
		c.Assert(s.Has("y"), qt.IsTrue)
		c.Assert(s.Len(), qt.Equals, 1)
	})

	c.Run("RemoveAbsent", func(c *qt.C) {
		s := New[string]("a")
		s.Remove("ghost") // should not panic
		c.Assert(s.Len(), qt.Equals, 0)
	})

	c.Run("Elements", func(c *qt.C) {
		s := New[int]("a")
		s.Add(3)
		s.Add(1)
		s.Add(2)

		elems := s.Elements()
		slices.Sort(elems)
		c.Assert(elems, qt.DeepEquals, []int{1, 2, 3})
	})

	c.Run("IntElements", func(c *qt.C) {
		s := New[int]("a")
		s.Add(42)
		s.Add(7)

		c.Assert(s.Has(42), qt.IsTrue)
		c.Assert(s.Has(99), qt.IsFalse)
		c.Assert(s.Len(), qt.Equals, 2)
	})

	c.Run("RemoveThenReadd", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Remove("x")
		c.Assert(s.Has("x"), qt.IsFalse)

		s.Add("x")
		c.Assert(s.Has("x"), qt.IsTrue)
	})

	c.Run("DuplicateAddSameReplica", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Add("x")

		c.Assert(s.Has("x"), qt.IsTrue)
		c.Assert(s.Len(), qt.Equals, 1)

		// Element should have two dots (one per Add call).
		ds, ok := s.state.Store.Get("x")
		c.Assert(ok, qt.IsTrue)
		count := 0
		ds.Range(func(_ dotcontext.Dot) bool { count++; return true })
		c.Assert(count, qt.Equals, 2)
	})

	c.Run("RemoveIsolation", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Add("y")
		s.Add("z")
		s.Remove("y")

		c.Assert(s.Has("x"), qt.IsTrue)
		c.Assert(s.Has("y"), qt.IsFalse)
		c.Assert(s.Has("z"), qt.IsTrue)
		c.Assert(s.Len(), qt.Equals, 2)
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Add", func(c *qt.C) {
		s := New[string]("a")
		delta := s.Add("x")

		c.Assert(delta.Has("x"), qt.IsTrue)
		c.Assert(delta.Len(), qt.Equals, 1)
	})

	c.Run("Remove", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		delta := s.Remove("x")
		c.Assert(delta.Len(), qt.Equals, 0)
	})
}

func TestAddWins(t *testing.T) {
	c := qt.New(t)

	c.Run("ConcurrentAddRemove", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		addDelta := a.Add("x")
		b.Merge(addDelta)

		rmDelta := a.Remove("x")
		addDelta2 := b.Add("x")

		a.Merge(addDelta2)
		b.Merge(rmDelta)

		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(b.Has("x"), qt.IsTrue)
	})

	c.Run("ConcurrentAddsSameElement", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Add("x")
		db := b.Add("x")

		a.Merge(db)
		b.Merge(da)

		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(b.Has("x"), qt.IsTrue)
	})

	c.Run("ConcurrentRemovesSameElement", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		// Both see "x".
		addDelta := a.Add("x")
		b.Merge(addDelta)

		// Both remove "x" concurrently.
		rmA := a.Remove("x")
		rmB := b.Remove("x")

		a.Merge(rmB)
		b.Merge(rmA)

		c.Assert(a.Has("x"), qt.IsFalse)
		c.Assert(b.Has("x"), qt.IsFalse)
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("RemoveDelta", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		addDelta := a.Add("x")
		b.Merge(addDelta)

		rmDelta := a.Remove("x")
		b.Merge(rmDelta)

		c.Assert(b.Has("x"), qt.IsFalse)
	})

}

func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New[string]("a")
		a.Add("x")
		a.Add("y")

		// Serialize and reconstruct.
		state := a.State()
		b := FromCausal[string](state)

		c.Assert(b.Has("x"), qt.IsTrue)
		c.Assert(b.Has("y"), qt.IsTrue)
		c.Assert(b.Len(), qt.Equals, 2)
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		delta := a.Add("x")

		// Reconstruct the delta via FromCausal and merge it.
		reconstructed := FromCausal[string](delta.State())

		b := New[string]("b")
		b.Merge(reconstructed)

		c.Assert(b.Has("x"), qt.IsTrue)
		c.Assert(b.Len(), qt.Equals, 1)
	})
}

