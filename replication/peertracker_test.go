package replication

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestPeerTrackerOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		tr := NewPeerTracker()
		c.Assert(tr.Len(), qt.Equals, 0)
	})
	c.Run("AddPeer", func(c *qt.C) {
		tr := NewPeerTracker()
		cc := dotcontext.New()
		cc.Add(dotcontext.Dot{ID: "x", Seq: 1})

		tr.AddPeer("peer1", cc)

		c.Assert(tr.Len(), qt.Equals, 1)
		peers := tr.Peers()
		c.Assert(peers, qt.HasLen, 1)
		c.Assert(peers[0], qt.Equals, dotcontext.ReplicaID("peer1"))
	})
	c.Run("AddPeerNilContext", func(c *qt.C) {
		tr := NewPeerTracker()
		tr.AddPeer("peer1", nil)
		c.Assert(tr.Len(), qt.Equals, 1)
	})
	c.Run("AddPeerDuplicate", func(c *qt.C) {
		tr := NewPeerTracker()
		cc1 := dotcontext.New()
		cc1.Add(dotcontext.Dot{ID: "x", Seq: 1})
		cc2 := dotcontext.New()
		cc2.Add(dotcontext.Dot{ID: "x", Seq: 5})

		tr.AddPeer("peer1", cc1)
		tr.AddPeer("peer1", cc2) // should be no-op
		c.Assert(tr.Len(), qt.Equals, 1)
	})
	c.Run("RemovePeer", func(c *qt.C) {
		tr := NewPeerTracker()
		tr.AddPeer("peer1", nil)
		tr.AddPeer("peer2", nil)
		tr.RemovePeer("peer1")
		c.Assert(tr.Len(), qt.Equals, 1)
	})
	c.Run("RemovePeerUnknown", func(c *qt.C) {
		tr := NewPeerTracker()
		tr.RemovePeer("ghost") // should not panic
		c.Assert(tr.Len(), qt.Equals, 0)
	})
}

func TestPeerTrackerAck(t *testing.T) {
	c := qt.New(t)

	c.Run("Merges", func(c *qt.C) {
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
	})
	c.Run("UnknownPeer", func(c *qt.C) {
		tr := NewPeerTracker()
		cc := dotcontext.New()
		cc.Add(dotcontext.Dot{ID: "a", Seq: 1})
		tr.Ack("ghost", cc) // should not panic
	})
	c.Run("ProgressiveAdvancement", func(c *qt.C) {
		tr := NewPeerTracker()
		tr.AddPeer("peer1", nil)

		local := dotcontext.New()
		local.Next("a") // a:1
		local.Next("a") // a:2
		local.Next("a") // a:3

		// Initially all pending.
		pending := tr.Pending("peer1", local)
		c.Assert(pending["a"], qt.HasLen, 1)
		c.Assert(pending["a"][0], qt.Equals, dotcontext.SeqRange{Lo: 1, Hi: 3})

		// Ack a:1.
		ack := dotcontext.New()
		ack.Add(dotcontext.Dot{ID: "a", Seq: 1})
		tr.Ack("peer1", ack)

		pending = tr.Pending("peer1", local)
		c.Assert(pending["a"], qt.HasLen, 1)
		c.Assert(pending["a"][0], qt.Equals, dotcontext.SeqRange{Lo: 2, Hi: 3})

		// Ack up to a:3.
		ack2 := dotcontext.New()
		ack2.Add(dotcontext.Dot{ID: "a", Seq: 2})
		ack2.Add(dotcontext.Dot{ID: "a", Seq: 3})
		tr.Ack("peer1", ack2)

		pending = tr.Pending("peer1", local)
		c.Assert(pending, qt.IsNil) // fully synced
	})
	c.Run("ThenCanGC", func(c *qt.C) {
		tr := NewPeerTracker()
		dot := dotcontext.Dot{ID: "a", Seq: 1}

		tr.AddPeer("peer1", nil)
		tr.AddPeer("peer2", nil)

		c.Assert(tr.CanGC(dot), qt.IsFalse)

		ack1 := dotcontext.New()
		ack1.Add(dot)
		tr.Ack("peer1", ack1)

		c.Assert(tr.CanGC(dot), qt.IsFalse) // peer2 still behind

		ack2 := dotcontext.New()
		ack2.Add(dot)
		tr.Ack("peer2", ack2)

		c.Assert(tr.CanGC(dot), qt.IsTrue) // both acked
	})
}

func TestPeerTrackerCanGC(t *testing.T) {
	c := qt.New(t)

	c.Run("AllAcked", func(c *qt.C) {
		tr := NewPeerTracker()
		dot := dotcontext.Dot{ID: "a", Seq: 1}

		cc := dotcontext.New()
		cc.Add(dot)

		tr.AddPeer("peer1", cc.Clone())
		tr.AddPeer("peer2", cc.Clone())

		c.Assert(tr.CanGC(dot), qt.IsTrue)
	})
	c.Run("SomeNotAcked", func(c *qt.C) {
		tr := NewPeerTracker()
		dot := dotcontext.Dot{ID: "a", Seq: 1}

		has := dotcontext.New()
		has.Add(dot)

		tr.AddPeer("peer1", has)
		tr.AddPeer("peer2", nil)

		c.Assert(tr.CanGC(dot), qt.IsFalse)
	})
	c.Run("NoPeers", func(c *qt.C) {
		tr := NewPeerTracker()
		dot := dotcontext.Dot{ID: "a", Seq: 1}
		c.Assert(tr.CanGC(dot), qt.IsTrue)
	})
	c.Run("AfterRemovePeer", func(c *qt.C) {
		tr := NewPeerTracker()
		dot := dotcontext.Dot{ID: "a", Seq: 1}

		has := dotcontext.New()
		has.Add(dot)

		tr.AddPeer("peer1", has)
		tr.AddPeer("peer2", nil) // blocks GC

		c.Assert(tr.CanGC(dot), qt.IsFalse)

		tr.RemovePeer("peer2")

		c.Assert(tr.CanGC(dot), qt.IsTrue)
	})
}

func TestPeerTrackerPending(t *testing.T) {
	c := qt.New(t)

	c.Run("UnknownPeer", func(c *qt.C) {
		tr := NewPeerTracker()
		local := dotcontext.New()
		local.Next("a")

		c.Assert(tr.Pending("ghost", local), qt.IsNil)
	})
	c.Run("FullySynced", func(c *qt.C) {
		tr := NewPeerTracker()
		local := dotcontext.New()
		local.Next("a")

		tr.AddPeer("peer1", local.Clone())

		c.Assert(tr.Pending("peer1", local), qt.IsNil)
	})
	c.Run("ComposesWithFetch", func(c *qt.C) {
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
	})
}
