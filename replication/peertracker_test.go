package replication

import "testing"

func TestNewPeerTrackerEmpty(t *testing.T) {
	tr := NewPeerTracker()
	if tr.Len() != 0 {
		t.Errorf("Len() = %d, want 0", tr.Len())
	}
}
