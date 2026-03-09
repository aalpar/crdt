package replication

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestGCRemovesAcked(t *testing.T) {
	c := qt.New(t)
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

	c.Assert(GC(store, tracker), qt.Equals, 2)
	c.Assert(store.Len(), qt.Equals, 0)
}

func TestGCKeepsUnacked(t *testing.T) {
	c := qt.New(t)
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

	c.Assert(GC(store, tracker), qt.Equals, 1)
	c.Assert(store.Len(), qt.Equals, 1)
	_, ok := store.Get(d2)
	c.Assert(ok, qt.IsTrue)
}

func TestGCEmptyStore(t *testing.T) {
	c := qt.New(t)
	store := dotcontext.NewDeltaStore[string]()
	tracker := NewPeerTracker()
	tracker.AddPeer("p1", nil)

	c.Assert(GC(store, tracker), qt.Equals, 0)
}

func TestGCIdempotent(t *testing.T) {
	c := qt.New(t)
	store := dotcontext.NewDeltaStore[string]()
	tracker := NewPeerTracker()

	d := dotcontext.Dot{ID: "a", Seq: 1}
	store.Add(d, "delta1")

	cc := dotcontext.New()
	cc.Add(d)
	tracker.AddPeer("p1", cc)

	c.Assert(GC(store, tracker), qt.Equals, 1)
	c.Assert(GC(store, tracker), qt.Equals, 0) // nothing left
	c.Assert(store.Len(), qt.Equals, 0)
}

func TestGCAfterAck(t *testing.T) {
	c := qt.New(t)
	store := dotcontext.NewDeltaStore[string]()
	tracker := NewPeerTracker()

	d1 := dotcontext.Dot{ID: "a", Seq: 1}
	d2 := dotcontext.Dot{ID: "a", Seq: 2}
	store.Add(d1, "delta1")
	store.Add(d2, "delta2")

	tracker.AddPeer("p1", nil)

	// Nothing acked — can't GC.
	c.Assert(GC(store, tracker), qt.Equals, 0)
	c.Assert(store.Len(), qt.Equals, 2)

	// Ack d1 only.
	ack := dotcontext.New()
	ack.Add(d1)
	tracker.Ack("p1", ack)

	c.Assert(GC(store, tracker), qt.Equals, 1)
	c.Assert(store.Len(), qt.Equals, 1)
	_, ok := store.Get(d1)
	c.Assert(ok, qt.IsFalse)
	_, ok = store.Get(d2)
	c.Assert(ok, qt.IsTrue)

	// Ack d2.
	ack2 := dotcontext.New()
	ack2.Add(d2)
	tracker.Ack("p1", ack2)

	c.Assert(GC(store, tracker), qt.Equals, 1)
	c.Assert(store.Len(), qt.Equals, 0)
}

func TestGCMultiReplica(t *testing.T) {
	c := qt.New(t)
	store := dotcontext.NewDeltaStore[string]()
	tracker := NewPeerTracker()

	da := dotcontext.Dot{ID: "a", Seq: 1}
	db := dotcontext.Dot{ID: "b", Seq: 1}
	store.Add(da, "delta-a")
	store.Add(db, "delta-b")

	cc := dotcontext.New()
	cc.Add(da)
	cc.Add(db)
	tracker.AddPeer("p1", cc)

	c.Assert(GC(store, tracker), qt.Equals, 2)
	c.Assert(store.Len(), qt.Equals, 0)
}

func TestGCNoPeers(t *testing.T) {
	c := qt.New(t)
	store := dotcontext.NewDeltaStore[string]()
	tracker := NewPeerTracker()

	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, "delta1")
	store.Add(dotcontext.Dot{ID: "a", Seq: 2}, "delta2")

	c.Assert(GC(store, tracker), qt.Equals, 2)
	c.Assert(store.Len(), qt.Equals, 0)
}
