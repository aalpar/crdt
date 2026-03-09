package dotcontext

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestDeltaStoreOperations(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		s := NewDeltaStore[int]()
		c.Assert(s.Len(), qt.Equals, 0)
	})

	c.Run("AddGet", func(c *qt.C) {
		s := NewDeltaStore[string]()
		d := Dot{ID: "a", Seq: 1}

		s.Add(d, "delta-1")
		c.Assert(s.Len(), qt.Equals, 1)

		got, ok := s.Get(d)
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, "delta-1")
	})

	c.Run("GetMissing", func(c *qt.C) {
		s := NewDeltaStore[string]()
		_, ok := s.Get(Dot{ID: "x", Seq: 99})
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("Remove", func(c *qt.C) {
		s := NewDeltaStore[string]()
		d := Dot{ID: "a", Seq: 1}
		s.Add(d, "delta-1")
		s.Remove(d)

		c.Assert(s.Len(), qt.Equals, 0)
		_, ok := s.Get(d)
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("RemoveAbsent", func(c *qt.C) {
		s := NewDeltaStore[string]()
		s.Remove(Dot{ID: "x", Seq: 1}) // should not panic
		c.Assert(s.Len(), qt.Equals, 0)
	})
}

func TestDeltaStoreFetch(t *testing.T) {
	c := qt.New(t)

	c.Run("SingleRange", func(c *qt.C) {
		s := NewDeltaStore[string]()
		s.Add(Dot{ID: "a", Seq: 1}, "d1")
		s.Add(Dot{ID: "a", Seq: 2}, "d2")
		s.Add(Dot{ID: "a", Seq: 3}, "d3")
		s.Add(Dot{ID: "a", Seq: 5}, "d5") // gap at 4

		missing := map[ReplicaID][]SeqRange{
			"a": {{Lo: 2, Hi: 4}},
		}
		got := s.Fetch(missing)

		c.Assert(len(got), qt.Equals, 2)
		c.Assert(got[Dot{ID: "a", Seq: 2}], qt.Equals, "d2")
		c.Assert(got[Dot{ID: "a", Seq: 3}], qt.Equals, "d3")
	})

	c.Run("MultiReplica", func(c *qt.C) {
		s := NewDeltaStore[string]()
		s.Add(Dot{ID: "a", Seq: 1}, "a1")
		s.Add(Dot{ID: "a", Seq: 3}, "a3")
		s.Add(Dot{ID: "b", Seq: 2}, "b2")
		s.Add(Dot{ID: "c", Seq: 1}, "c1")

		missing := map[ReplicaID][]SeqRange{
			"a": {{Lo: 1, Hi: 1}, {Lo: 3, Hi: 3}},
			"b": {{Lo: 1, Hi: 5}},
		}
		got := s.Fetch(missing)

		c.Assert(len(got), qt.Equals, 3)
		c.Assert(got[Dot{ID: "a", Seq: 1}], qt.Equals, "a1")
		c.Assert(got[Dot{ID: "a", Seq: 3}], qt.Equals, "a3")
		c.Assert(got[Dot{ID: "b", Seq: 2}], qt.Equals, "b2")
		_, ok := got[Dot{ID: "c", Seq: 1}]
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("Empty", func(c *qt.C) {
		s := NewDeltaStore[string]()
		s.Add(Dot{ID: "a", Seq: 1}, "a1")

		c.Assert(s.Fetch(nil), qt.IsNil)
		c.Assert(s.Fetch(map[ReplicaID][]SeqRange{}), qt.IsNil)
	})

	c.Run("NoMatches", func(c *qt.C) {
		s := NewDeltaStore[string]()
		s.Add(Dot{ID: "a", Seq: 1}, "a1")

		missing := map[ReplicaID][]SeqRange{
			"b": {{Lo: 1, Hi: 10}},
		}
		c.Assert(s.Fetch(missing), qt.IsNil)
	})
}

func TestDeltaStoreDots(t *testing.T) {
	c := qt.New(t)

	c.Run("Empty", func(c *qt.C) {
		store := NewDeltaStore[int]()
		c.Assert(store.Dots(), qt.HasLen, 0)
	})

	c.Run("MultiReplica", func(c *qt.C) {
		store := NewDeltaStore[int]()
		d1 := Dot{ID: "a", Seq: 1}
		d2 := Dot{ID: "a", Seq: 2}
		d3 := Dot{ID: "b", Seq: 1}
		store.Add(d1, 10)
		store.Add(d2, 20)
		store.Add(d3, 30)

		dots := store.Dots()
		c.Assert(dots, qt.HasLen, 3)

		have := make(map[Dot]bool)
		for _, d := range dots {
			have[d] = true
		}
		for _, want := range []Dot{d1, d2, d3} {
			c.Assert(have[want], qt.IsTrue, qt.Commentf("missing %v", want))
		}
	})
}
