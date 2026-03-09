package dotcontext

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestDotMapBasic(t *testing.T) {
	c := qt.New(t)
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
}

func TestDotMapDeleteAbsent(t *testing.T) {
	c := qt.New(t)
	m := NewDotMap[string, *DotSet]()
	m.Delete("ghost") // should not panic
	c.Assert(m.Len(), qt.Equals, 0)
}

func TestDotMapRange(t *testing.T) {
	c := qt.New(t)
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
}

func TestDotMapRangeEarlyStop(t *testing.T) {
	c := qt.New(t)
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
}

func TestDotMapKeys(t *testing.T) {
	c := qt.New(t)
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
}

func TestDotMapDots(t *testing.T) {
	c := qt.New(t)
	m := NewDotMap[string, *DotSet]()

	s1 := NewDotSet()
	s1.Add(Dot{ID: "a", Seq: 1})
	m.Set("k1", s1)

	s2 := NewDotSet()
	s2.Add(Dot{ID: "b", Seq: 2})
	m.Set("k2", s2)

	c.Assert(m.Dots().Len(), qt.Equals, 2)
}
