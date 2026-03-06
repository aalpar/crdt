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

func TestDeltaStoreRemove(t *testing.T) {
	s := NewDeltaStore[string]()
	d := Dot{ID: "a", Seq: 1}
	s.Add(d, "delta-1")
	s.Remove(d)

	if s.Len() != 0 {
		t.Errorf("Len() after Remove = %d, want 0", s.Len())
	}
	if _, ok := s.Get(d); ok {
		t.Error("Get returned ok after Remove")
	}
}

func TestDeltaStoreRemoveAbsent(t *testing.T) {
	s := NewDeltaStore[string]()
	s.Remove(Dot{ID: "x", Seq: 1}) // should not panic
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}

func TestDeltaStoreFetchSingleRange(t *testing.T) {
	s := NewDeltaStore[string]()
	s.Add(Dot{ID: "a", Seq: 1}, "d1")
	s.Add(Dot{ID: "a", Seq: 2}, "d2")
	s.Add(Dot{ID: "a", Seq: 3}, "d3")
	s.Add(Dot{ID: "a", Seq: 5}, "d5") // gap at 4

	missing := map[ReplicaID][]SeqRange{
		"a": {{Lo: 2, Hi: 4}},
	}
	got := s.Fetch(missing)

	// Should return a:2 and a:3. a:4 is not in store. a:1 and a:5 outside range.
	if len(got) != 2 {
		t.Fatalf("Fetch() returned %d deltas, want 2: %v", len(got), got)
	}
	if got[Dot{ID: "a", Seq: 2}] != "d2" {
		t.Error("missing a:2")
	}
	if got[Dot{ID: "a", Seq: 3}] != "d3" {
		t.Error("missing a:3")
	}
}
