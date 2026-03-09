package rwset

import (
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"
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

	c.Run("AddRemoveHas", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Add("y")
		s.Remove("x")

		c.Assert(s.Has("x"), qt.IsFalse)
		c.Assert(s.Has("y"), qt.IsTrue)
		c.Assert(s.Len(), qt.Equals, 1)
	})

	c.Run("ReAdd", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Remove("x")
		c.Assert(s.Has("x"), qt.IsFalse)

		s.Add("x")
		c.Assert(s.Has("x"), qt.IsTrue)
	})

	c.Run("RemoveNonExistent", func(c *qt.C) {
		s := New[string]("a")
		s.Remove("ghost") // should not panic, no dot generated
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

		// Remove delta has the element with Active=false — not "present".
		c.Assert(delta.Has("x"), qt.IsFalse)
	})
}

func TestRemoveWins(t *testing.T) {
	c := qt.New(t)

	c.Run("ConcurrentAddRemove", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		// Both replicas see "x".
		addDelta := a.Add("x")
		b.Merge(addDelta)

		// Concurrent: a re-adds "x", b removes "x".
		reAddDelta := a.Add("x")
		removeDelta := b.Remove("x")

		a.Merge(removeDelta)
		b.Merge(reAddDelta)

		// Remove wins: the tombstone dot from b survives alongside a's add dot.
		// Has requires ALL dots to be Active, so the tombstone makes it absent.
		c.Assert(a.Has("x"), qt.IsFalse, qt.Commentf("replica a: remove should win"))
		c.Assert(b.Has("x"), qt.IsFalse, qt.Commentf("replica b: remove should win"))
	})

	c.Run("ConcurrentAdds", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Add("x")
		db := b.Add("x")

		a.Merge(db)
		b.Merge(da)

		// Both added concurrently — both dots are Active, so element is present.
		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(b.Has("x"), qt.IsTrue)
	})

	c.Run("ConcurrentRemoves", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		// Both see "x".
		addDelta := a.Add("x")
		b.Merge(addDelta)

		// Both remove concurrently.
		rmA := a.Remove("x")
		rmB := b.Remove("x")

		a.Merge(rmB)
		b.Merge(rmA)

		c.Assert(a.Has("x"), qt.IsFalse)
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

