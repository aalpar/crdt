package replication

import (
	"testing"

	"github.com/aalpar/crdt/dotcontext"
)

func TestGCRemovesAcked(t *testing.T) {
	store := dotcontext.NewDeltaStore[string]()
	tracker := NewPeerTracker()

	d1 := dotcontext.Dot{ID: "a", Seq: 1}
	d2 := dotcontext.Dot{ID: "a", Seq: 2}
	store.Add(d1, "delta1")
	store.Add(d2, "delta2")

	cc := dotcontext.New()
	cc.Add(d1)
	cc.Add(d2)
	tracker.AddPeer("p1", cc.Clone())
	tracker.AddPeer("p2", cc.Clone())

	removed := GC(store, tracker)
	if removed != 2 {
		t.Errorf("GC() = %d, want 2", removed)
	}
	if store.Len() != 0 {
		t.Errorf("store.Len() = %d, want 0", store.Len())
	}
}

func TestGCKeepsUnacked(t *testing.T) {
	store := dotcontext.NewDeltaStore[string]()
	tracker := NewPeerTracker()

	d1 := dotcontext.Dot{ID: "a", Seq: 1}
	d2 := dotcontext.Dot{ID: "a", Seq: 2}
	store.Add(d1, "delta1")
	store.Add(d2, "delta2")

	ccFull := dotcontext.New()
	ccFull.Add(d1)
	ccFull.Add(d2)
	ccPartial := dotcontext.New()
	ccPartial.Add(d1)

	tracker.AddPeer("p1", ccFull)
	tracker.AddPeer("p2", ccPartial)

	removed := GC(store, tracker)
	if removed != 1 {
		t.Errorf("GC() = %d, want 1", removed)
	}
	if store.Len() != 1 {
		t.Errorf("store.Len() = %d, want 1", store.Len())
	}
	if _, ok := store.Get(d2); !ok {
		t.Error("d2 should still be in store")
	}
}

func TestGCEmptyStore(t *testing.T) {
	store := dotcontext.NewDeltaStore[string]()
	tracker := NewPeerTracker()
	tracker.AddPeer("p1", nil)

	removed := GC(store, tracker)
	if removed != 0 {
		t.Errorf("GC() = %d, want 0", removed)
	}
}

func TestGCNoPeers(t *testing.T) {
	store := dotcontext.NewDeltaStore[string]()
	tracker := NewPeerTracker()

	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, "delta1")
	store.Add(dotcontext.Dot{ID: "a", Seq: 2}, "delta2")

	removed := GC(store, tracker)
	if removed != 2 {
		t.Errorf("GC() = %d, want 2 (vacuous — no peers)", removed)
	}
	if store.Len() != 0 {
		t.Errorf("store.Len() = %d, want 0", store.Len())
	}
}
