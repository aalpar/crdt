package dotcontext

import "testing"

func TestDeltaStoreNewEmpty(t *testing.T) {
	s := NewDeltaStore[int]()
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}

func TestDeltaStoreAddGet(t *testing.T) {
	s := NewDeltaStore[string]()
	d := Dot{ID: "a", Seq: 1}

	s.Add(d, "delta-1")

	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1", s.Len())
	}

	got, ok := s.Get(d)
	if !ok {
		t.Fatal("Get returned !ok for stored dot")
	}
	if got != "delta-1" {
		t.Errorf("Get() = %q, want %q", got, "delta-1")
	}
}

func TestDeltaStoreGetMissing(t *testing.T) {
	s := NewDeltaStore[string]()
	_, ok := s.Get(Dot{ID: "x", Seq: 99})
	if ok {
		t.Error("Get returned ok for absent dot")
	}
}
