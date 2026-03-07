package replication

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestNewPeerTrackerEmpty(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	c.Assert(tr.Len(), qt.Equals, 0)
}

func TestAddPeer(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	cc := dotcontext.New()
	cc.Add(dotcontext.Dot{ID: "x", Seq: 1})

	tr.AddPeer("peer1", cc)

	c.Assert(tr.Len(), qt.Equals, 1)
	peers := tr.Peers()
	c.Assert(peers, qt.HasLen, 1)
	c.Assert(peers[0], qt.Equals, dotcontext.ReplicaID("peer1"))
}

func TestAddPeerNilContext(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	tr.AddPeer("peer1", nil)
	c.Assert(tr.Len(), qt.Equals, 1)
}

func TestAddPeerDuplicate(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	cc1 := dotcontext.New()
	cc1.Add(dotcontext.Dot{ID: "x", Seq: 1})
	cc2 := dotcontext.New()
	cc2.Add(dotcontext.Dot{ID: "x", Seq: 5})

	tr.AddPeer("peer1", cc1)
	tr.AddPeer("peer1", cc2) // should be no-op
	c.Assert(tr.Len(), qt.Equals, 1)
}

func TestRemovePeer(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	tr.AddPeer("peer1", nil)
	tr.AddPeer("peer2", nil)
	tr.RemovePeer("peer1")
	c.Assert(tr.Len(), qt.Equals, 1)
}

func TestRemovePeerUnknown(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	tr.RemovePeer("ghost") // should not panic
	c.Assert(tr.Len(), qt.Equals, 0)
}

func TestAckMerges(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	tr.AddPeer("peer1", nil)

	cc1 := dotcontext.New()
	cc1.Add(dotcontext.Dot{ID: "a", Seq: 1})
	tr.Ack("peer1", cc1)

	cc2 := dotcontext.New()
	cc2.Add(dotcontext.Dot{ID: "a", Seq: 3})
	tr.Ack("peer1", cc2)

	local := dotcontext.New()
	local.Next("a") // a:1
	local.Next("a") // a:2
	local.Next("a") // a:3

	pending := tr.Pending("peer1", local)
	c.Assert(pending, qt.IsNotNil)
	ranges := pending["a"]
	c.Assert(ranges, qt.HasLen, 1)
	c.Assert(ranges[0].Lo, qt.Equals, uint64(2))
	c.Assert(ranges[0].Hi, qt.Equals, uint64(2))
}

func TestAckUnknownPeer(t *testing.T) {
	tr := NewPeerTracker()
	cc := dotcontext.New()
	cc.Add(dotcontext.Dot{ID: "a", Seq: 1})
	tr.Ack("ghost", cc) // should not panic
}

func TestCanGCAllAcked(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	dot := dotcontext.Dot{ID: "a", Seq: 1}

	cc := dotcontext.New()
	cc.Add(dot)

	tr.AddPeer("peer1", cc.Clone())
	tr.AddPeer("peer2", cc.Clone())

	c.Assert(tr.CanGC(dot), qt.IsTrue)
}

func TestCanGCSomeNotAcked(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	dot := dotcontext.Dot{ID: "a", Seq: 1}

	has := dotcontext.New()
	has.Add(dot)

	tr.AddPeer("peer1", has)
	tr.AddPeer("peer2", nil)

	c.Assert(tr.CanGC(dot), qt.IsFalse)
}

func TestCanGCNoPeers(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	dot := dotcontext.Dot{ID: "a", Seq: 1}
	c.Assert(tr.CanGC(dot), qt.IsTrue)
}

func TestCanGCAfterRemovePeer(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	dot := dotcontext.Dot{ID: "a", Seq: 1}

	has := dotcontext.New()
	has.Add(dot)

	tr.AddPeer("peer1", has)
	tr.AddPeer("peer2", nil) // blocks GC

	c.Assert(tr.CanGC(dot), qt.IsFalse)

	tr.RemovePeer("peer2")

	c.Assert(tr.CanGC(dot), qt.IsTrue)
}

func TestPendingUnknownPeer(t *testing.T) {
	c := qt.New(t)
	tr := NewPeerTracker()
	local := dotcontext.New()
	local.Next("a")

	c.Assert(tr.Pending("ghost", local), qt.IsNil)
}

func TestPendingComposesWithFetch(t *testing.T) {
	c := qt.New(t)
	local := dotcontext.New()
	d1 := local.Next("a") // a:1
	d2 := local.Next("a") // a:2
	d3 := local.Next("a") // a:3

	store := dotcontext.NewDeltaStore[string]()
	store.Add(d1, "delta1")
	store.Add(d2, "delta2")
	store.Add(d3, "delta3")

	peerCC := dotcontext.New()
	peerCC.Add(d1)

	tracker := NewPeerTracker()
	tracker.AddPeer("peer1", peerCC)

	pending := tracker.Pending("peer1", local)
	deltas := store.Fetch(pending)

	c.Assert(deltas, qt.HasLen, 2)
	c.Assert(deltas[d2], qt.Equals, "delta2")
	c.Assert(deltas[d3], qt.Equals, "delta3")
}
