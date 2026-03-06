package replication

import (
	"testing"

	"github.com/aalpar/crdt/dotcontext"
)

func TestNewPeerTrackerEmpty(t *testing.T) {
	tr := NewPeerTracker()
	if tr.Len() != 0 {
		t.Errorf("Len() = %d, want 0", tr.Len())
	}
}

func TestAddPeer(t *testing.T) {
	tr := NewPeerTracker()
	cc := dotcontext.New()
	cc.Add(dotcontext.Dot{ID: "x", Seq: 1})

	tr.AddPeer("peer1", cc)

	if tr.Len() != 1 {
		t.Errorf("Len() = %d, want 1", tr.Len())
	}
	peers := tr.Peers()
	if len(peers) != 1 || peers[0] != "peer1" {
		t.Errorf("Peers() = %v, want [peer1]", peers)
	}
}

func TestAddPeerNilContext(t *testing.T) {
	tr := NewPeerTracker()
	tr.AddPeer("peer1", nil)

	if tr.Len() != 1 {
		t.Errorf("Len() = %d, want 1", tr.Len())
	}
}

func TestAddPeerDuplicate(t *testing.T) {
	tr := NewPeerTracker()
	cc1 := dotcontext.New()
	cc1.Add(dotcontext.Dot{ID: "x", Seq: 1})
	cc2 := dotcontext.New()
	cc2.Add(dotcontext.Dot{ID: "x", Seq: 5})

	tr.AddPeer("peer1", cc1)
	tr.AddPeer("peer1", cc2) // should be no-op

	if tr.Len() != 1 {
		t.Errorf("Len() = %d, want 1", tr.Len())
	}
}

func TestRemovePeer(t *testing.T) {
	tr := NewPeerTracker()
	tr.AddPeer("peer1", nil)
	tr.AddPeer("peer2", nil)
	tr.RemovePeer("peer1")

	if tr.Len() != 1 {
		t.Errorf("Len() = %d, want 1", tr.Len())
	}
}

func TestRemovePeerUnknown(t *testing.T) {
	tr := NewPeerTracker()
	tr.RemovePeer("ghost") // should not panic
	if tr.Len() != 0 {
		t.Errorf("Len() = %d, want 0", tr.Len())
	}
}

func TestAckMerges(t *testing.T) {
	tr := NewPeerTracker()
	tr.AddPeer("peer1", nil)

	// First ACK: peer knows about dot (a,1)
	cc1 := dotcontext.New()
	cc1.Add(dotcontext.Dot{ID: "a", Seq: 1})
	tr.Ack("peer1", cc1)

	// Second ACK: peer knows about dot (a,3) — out of order
	cc2 := dotcontext.New()
	cc2.Add(dotcontext.Dot{ID: "a", Seq: 3})
	tr.Ack("peer1", cc2)

	// Peer should now have both (a,1) and (a,3).
	// With local context at (a,1..3), pending should be just (a,2).
	local := dotcontext.New()
	local.Next("a") // a:1
	local.Next("a") // a:2
	local.Next("a") // a:3

	pending := tr.Pending("peer1", local)
	if pending == nil {
		t.Fatal("Pending() = nil, want (a,2)")
	}
	ranges := pending["a"]
	if len(ranges) != 1 || ranges[0].Lo != 2 || ranges[0].Hi != 2 {
		t.Errorf("Pending() ranges for a = %v, want [{2,2}]", ranges)
	}
}

func TestAckUnknownPeer(t *testing.T) {
	tr := NewPeerTracker()
	cc := dotcontext.New()
	cc.Add(dotcontext.Dot{ID: "a", Seq: 1})
	tr.Ack("ghost", cc) // should not panic
}

func TestCanGCAllAcked(t *testing.T) {
	tr := NewPeerTracker()
	dot := dotcontext.Dot{ID: "a", Seq: 1}

	cc := dotcontext.New()
	cc.Add(dot)

	tr.AddPeer("peer1", cc.Clone())
	tr.AddPeer("peer2", cc.Clone())

	if !tr.CanGC(dot) {
		t.Error("CanGC = false, want true (all peers have dot)")
	}
}

func TestCanGCSomeNotAcked(t *testing.T) {
	tr := NewPeerTracker()
	dot := dotcontext.Dot{ID: "a", Seq: 1}

	has := dotcontext.New()
	has.Add(dot)

	tr.AddPeer("peer1", has)
	tr.AddPeer("peer2", nil) // peer2 knows nothing

	if tr.CanGC(dot) {
		t.Error("CanGC = true, want false (peer2 lacks dot)")
	}
}

func TestCanGCNoPeers(t *testing.T) {
	tr := NewPeerTracker()
	dot := dotcontext.Dot{ID: "a", Seq: 1}

	if !tr.CanGC(dot) {
		t.Error("CanGC = false, want true (vacuous — no peers)")
	}
}

func TestCanGCAfterRemovePeer(t *testing.T) {
	tr := NewPeerTracker()
	dot := dotcontext.Dot{ID: "a", Seq: 1}

	has := dotcontext.New()
	has.Add(dot)

	tr.AddPeer("peer1", has)
	tr.AddPeer("peer2", nil) // blocks GC

	if tr.CanGC(dot) {
		t.Error("CanGC should be false before RemovePeer")
	}

	tr.RemovePeer("peer2")

	if !tr.CanGC(dot) {
		t.Error("CanGC should be true after removing lagging peer")
	}
}

func TestPendingUnknownPeer(t *testing.T) {
	tr := NewPeerTracker()
	local := dotcontext.New()
	local.Next("a")

	if got := tr.Pending("ghost", local); got != nil {
		t.Errorf("Pending(unknown) = %v, want nil", got)
	}
}

func TestPendingComposesWithFetch(t *testing.T) {
	// Local node has 3 dots in both its CausalContext and DeltaStore.
	local := dotcontext.New()
	d1 := local.Next("a") // a:1
	d2 := local.Next("a") // a:2
	d3 := local.Next("a") // a:3

	store := dotcontext.NewDeltaStore[string]()
	store.Add(d1, "delta1")
	store.Add(d2, "delta2")
	store.Add(d3, "delta3")

	// Peer has acknowledged only a:1.
	peerCC := dotcontext.New()
	peerCC.Add(d1)

	tracker := NewPeerTracker()
	tracker.AddPeer("peer1", peerCC)

	// Pending → Fetch: no adapter, no glue.
	pending := tracker.Pending("peer1", local)
	deltas := store.Fetch(pending)

	if len(deltas) != 2 {
		t.Fatalf("Fetch(Pending()) returned %d deltas, want 2", len(deltas))
	}
	if deltas[d2] != "delta2" {
		t.Errorf("missing delta for %v", d2)
	}
	if deltas[d3] != "delta3" {
		t.Errorf("missing delta for %v", d3)
	}
}
