package dotcontext

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestDotMapOperations(t *testing.T) {
	c := qt.New(t)

	c.Run("Basic", func(c *qt.C) {
		m := NewDotMap[string, *DotSet]()
		s := NewDotSet()
		s.Add(Dot{ID: "a", Seq: 1})
		m.Set("key1", s)

		got, ok := m.Get("key1")
		c.Assert(ok, qt.IsTrue)
		c.Assert(got.Has(Dot{ID: "a", Seq: 1}), qt.IsTrue)
		c.Assert(m.Len(), qt.Equals, 1)

		m.Delete("key1")
		_, ok = m.Get("key1")
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("DeleteAbsent", func(c *qt.C) {
		m := NewDotMap[string, *DotSet]()
		m.Delete("ghost") // should not panic
		c.Assert(m.Len(), qt.Equals, 0)
	})
}

func TestDotMapRange(t *testing.T) {
	c := qt.New(t)

	c.Run("FullIteration", func(c *qt.C) {
		m := NewDotMap[string, *DotSet]()
		s1 := NewDotSet()
		s1.Add(Dot{ID: "a", Seq: 1})
		m.Set("k1", s1)
		s2 := NewDotSet()
		s2.Add(Dot{ID: "b", Seq: 1})
		m.Set("k2", s2)

		keys := make(map[string]bool)
		m.Range(func(k string, _ *DotSet) bool {
			keys[k] = true
			return true
		})
		c.Assert(keys["k1"], qt.IsTrue)
		c.Assert(keys["k2"], qt.IsTrue)
		c.Assert(len(keys), qt.Equals, 2)
	})

	c.Run("EarlyStop", func(c *qt.C) {
		m := NewDotMap[string, *DotSet]()
		m.Set("k1", NewDotSet())
		m.Set("k2", NewDotSet())
		m.Set("k3", NewDotSet())

		count := 0
		m.Range(func(_ string, _ *DotSet) bool {
			count++
			return false
		})
		c.Assert(count, qt.Equals, 1)
	})

	c.Run("Empty", func(c *qt.C) {
		m := NewDotMap[string, *DotSet]()
		count := 0
		m.Range(func(_ string, _ *DotSet) bool {
			count++
			return true
		})
		c.Assert(count, qt.Equals, 0)
	})
}

func TestDotMapKeys(t *testing.T) {
	c := qt.New(t)

	c.Run("TwoKeys", func(c *qt.C) {
		m := NewDotMap[string, *DotSet]()
		m.Set("a", NewDotSet())
		m.Set("b", NewDotSet())

		keys := m.Keys()
		c.Assert(len(keys), qt.Equals, 2)
		seen := make(map[string]bool)
		for _, k := range keys {
			seen[k] = true
		}
		c.Assert(seen["a"], qt.IsTrue)
		c.Assert(seen["b"], qt.IsTrue)
	})

	c.Run("Empty", func(c *qt.C) {
		m := NewDotMap[string, *DotSet]()
		c.Assert(m.Keys(), qt.HasLen, 0)
	})
}

func TestDotMapDots(t *testing.T) {
	c := qt.New(t)

	c.Run("Count", func(c *qt.C) {
		m := NewDotMap[string, *DotSet]()

		s1 := NewDotSet()
		s1.Add(Dot{ID: "a", Seq: 1})
		m.Set("k1", s1)

		s2 := NewDotSet()
		s2.Add(Dot{ID: "b", Seq: 2})
		m.Set("k2", s2)

		c.Assert(m.Dots().Len(), qt.Equals, 2)
	})

	c.Run("ReturnsClone", func(c *qt.C) {
		m := NewDotMap[string, *DotSet]()
		s := NewDotSet()
		s.Add(Dot{ID: "a", Seq: 1})
		m.Set("key", s)

		dots := m.Dots()
		dots.Add(Dot{ID: "b", Seq: 2})

		// Original should be unaffected.
		c.Assert(m.Dots().Len(), qt.Equals, 1)
	})
}

func TestDotMapClone(t *testing.T) {
	c := qt.New(t)
	m := NewDotMap[string, *DotSet]()
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})
	m.Set("k1", s)

	cl := m.Clone()
	cl.Set("k2", NewDotSet())

	c.Assert(m.Len(), qt.Equals, 1)
	_, ok := m.Get("k2")
	c.Assert(ok, qt.IsFalse)
}
