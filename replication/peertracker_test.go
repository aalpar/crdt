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

// TODO(aalpar): Write TestPendingComposesWithFetch.
//
// This test proves the two types (PeerTracker + DeltaStore) compose
// without coupling. The pattern:
//
//   pending := tracker.Pending(peerID, localCC)
//   deltas  := store.Fetch(pending)
//
// Set up: a local node with 3 dots (a:1, a:2, a:3) in both its
// CausalContext and DeltaStore. A peer that has acknowledged a:1 only.
// Pending should return a:2..3. Fetch with that should return
// exactly the deltas for a:2 and a:3.
//
// This is the key design property — the composability of Pending's
// output with DeltaStore.Fetch's input.
