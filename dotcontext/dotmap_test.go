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
