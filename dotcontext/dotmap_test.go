package dotcontext

import "testing"

func TestDotMapBasic(t *testing.T) {
	m := NewDotMap[string, *DotSet]()
	s := NewDotSet()
	s.Add(Dot{ID: "a", Seq: 1})
	m.Set("key1", s)

	got, ok := m.Get("key1")
	if !ok {
		t.Fatal("should have key1")
	}
	if !got.Has(Dot{ID: "a", Seq: 1}) {
		t.Error("value should contain a:1")
	}

	if m.Len() != 1 {
		t.Errorf("len = %d, want 1", m.Len())
	}

	m.Delete("key1")
	if _, ok := m.Get("key1"); ok {
		t.Error("should not have key1 after delete")
	}
}

func TestDotMapDots(t *testing.T) {
	m := NewDotMap[string, *DotSet]()

	s1 := NewDotSet()
	s1.Add(Dot{ID: "a", Seq: 1})
	m.Set("k1", s1)

	s2 := NewDotSet()
	s2.Add(Dot{ID: "b", Seq: 2})
	m.Set("k2", s2)

	dots := m.Dots()
	if dots.Len() != 2 {
		t.Errorf("Dots().Len() = %d, want 2", dots.Len())
	}
}
