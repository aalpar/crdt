package dotcontext

import "testing"

func TestDotSetBasic(t *testing.T) {
	s := NewDotSet()
	d := Dot{ID: "a", Seq: 1}

	if s.Has(d) {
		t.Error("empty set should not have any dots")
	}

	s.Add(d)
	if !s.Has(d) {
		t.Error("should have dot after add")
	}
	if s.Len() != 1 {
		t.Errorf("len = %d, want 1", s.Len())
	}

	s.Remove(d)
	if s.Has(d) {
		t.Error("should not have dot after remove")
	}
}

func TestDotSetRange(t *testing.T) {
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})
	s.Add(Dot{ID: "b", Seq: 2})

	count := 0
	s.Range(func(d Dot) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("range visited %d dots, want 2", count)
	}
}

func TestDotSetRangeEarlyStop(t *testing.T) {
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})
	s.Add(Dot{ID: "b", Seq: 2})
	s.Add(Dot{ID: "c", Seq: 3})

	count := 0
	s.Range(func(d Dot) bool {
		count++
		return false // stop after first
	})
	if count != 1 {
		t.Errorf("range should have stopped after 1, visited %d", count)
	}
}

func TestDotSetClone(t *testing.T) {
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})

	c := s.Clone()
	c.Add(Dot{ID: "b", Seq: 2})

	if s.Has(Dot{ID: "b", Seq: 2}) {
		t.Error("clone mutation should not affect original")
	}
}
