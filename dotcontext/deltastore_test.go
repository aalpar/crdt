package dotcontext

import "testing"

func TestDeltaStoreNewEmpty(t *testing.T) {
	s := NewDeltaStore[int]()
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}
