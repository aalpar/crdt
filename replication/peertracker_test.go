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
