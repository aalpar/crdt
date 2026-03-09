package dotcontext

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestDotSetBasic(t *testing.T) {
	c := qt.New(t)
	s := NewDotSet()
	d := Dot{ID: "a", Seq: 1}

	c.Assert(s.Has(d), qt.IsFalse)

	s.Add(d)
	c.Assert(s.Has(d), qt.IsTrue)
	c.Assert(s.Len(), qt.Equals, 1)

	s.Remove(d)
	c.Assert(s.Has(d), qt.IsFalse)
}

func TestDotSetRange(t *testing.T) {
	c := qt.New(t)
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})
	s.Add(Dot{ID: "b", Seq: 2})

	count := 0
	s.Range(func(d Dot) bool {
		count++
		return true
	})
	c.Assert(count, qt.Equals, 2)
}

func TestDotSetRangeEarlyStop(t *testing.T) {
	c := qt.New(t)
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})
	s.Add(Dot{ID: "b", Seq: 2})
	s.Add(Dot{ID: "c", Seq: 3})

	count := 0
	s.Range(func(d Dot) bool {
		count++
		return false
	})
	c.Assert(count, qt.Equals, 1)
}

func TestDotSetAddDuplicate(t *testing.T) {
	c := qt.New(t)
	s := NewDotSet()
	d := Dot{ID: "a", Seq: 1}
	s.Add(d)
	s.Add(d)
	c.Assert(s.Len(), qt.Equals, 1)
}

func TestDotSetRemoveAbsent(t *testing.T) {
	c := qt.New(t)
	s := NewDotSet()
	s.Remove(Dot{ID: "x", Seq: 99}) // should not panic
	c.Assert(s.Len(), qt.Equals, 0)
}

func TestDotSetDotsReturnsClone(t *testing.T) {
	c := qt.New(t)
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})

	dots := s.Dots()
	dots.Add(Dot{ID: "b", Seq: 2})

	// Original should be unaffected.
	c.Assert(s.Len(), qt.Equals, 1)
	c.Assert(s.Has(Dot{ID: "b", Seq: 2}), qt.IsFalse)
}

func TestDotSetRangeEmpty(t *testing.T) {
	c := qt.New(t)
	s := NewDotSet()
	count := 0
	s.Range(func(d Dot) bool { count++; return true })
	c.Assert(count, qt.Equals, 0)
}

func TestDotSetClone(t *testing.T) {
	c := qt.New(t)
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})

	cl := s.Clone()
	cl.Add(Dot{ID: "b", Seq: 2})

	c.Assert(s.Has(Dot{ID: "b", Seq: 2}), qt.IsFalse)
}
