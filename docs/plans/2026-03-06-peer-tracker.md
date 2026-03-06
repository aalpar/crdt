# PeerTracker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the PeerTracker type in a new `replication/` package — per-peer ACK tracking for delta GC.

**Architecture:** PeerTracker stores one CausalContext per known peer. It answers "can this dot be GC'd?" (all peers have it) and "what does this peer need?" (delegates to CausalContext.Missing). It is composed alongside DeltaStore by the caller — no ownership between them.

**Tech Stack:** Go, stdlib only. Imports `github.com/aalpar/crdt/dotcontext`.

**Design doc:** `docs/plans/PEER-TRACKER-DESIGN.md`

---

### Task 1: NewPeerTracker and Len

**Files:**
- Create: `replication/peertracker.go`
- Create: `replication/peertracker_test.go`

**Step 1: Write the failing test**

In `replication/peertracker_test.go`:

```go
package replication

import "testing"

func TestNewPeerTrackerEmpty(t *testing.T) {
	tr := NewPeerTracker()
	if tr.Len() != 0 {
		t.Errorf("Len() = %d, want 0", tr.Len())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./replication/ -run TestNewPeerTrackerEmpty -v`
Expected: FAIL — `NewPeerTracker` not defined

**Step 3: Write minimal implementation**

In `replication/peertracker.go`:

```go
package replication

import "github.com/aalpar/crdt/dotcontext"

// PeerTracker tracks per-peer acknowledged state for delta GC.
// It stores a CausalContext per peer representing what that peer
// has observed. Not thread-safe — caller handles synchronization.
type PeerTracker struct {
	peers map[dotcontext.ReplicaID]*dotcontext.CausalContext
}

// NewPeerTracker returns an empty PeerTracker with no registered peers.
func NewPeerTracker() *PeerTracker {
	return &PeerTracker{peers: make(map[dotcontext.ReplicaID]*dotcontext.CausalContext)}
}

// Len returns the number of registered peers.
func (t *PeerTracker) Len() int {
	return len(t.peers)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./replication/ -run TestNewPeerTrackerEmpty -v`
Expected: PASS

**Step 5: Commit**

```
git add replication/peertracker.go replication/peertracker_test.go
git commit -m "replication: PeerTracker with NewPeerTracker and Len"
```

---

### Task 2: AddPeer and Peers

**Files:**
- Modify: `replication/peertracker.go`
- Modify: `replication/peertracker_test.go`

**Step 1: Write the failing tests**

Append to `replication/peertracker_test.go`:

```go
import "github.com/aalpar/crdt/dotcontext"

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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./replication/ -run "TestAddPeer" -v`
Expected: FAIL — `AddPeer` and `Peers` not defined

**Step 3: Write minimal implementation**

Add to `replication/peertracker.go`:

```go
// AddPeer registers a peer with its initial known state.
// A nil context is treated as empty (peer knows nothing).
// Adding an already-known peer is a no-op.
func (t *PeerTracker) AddPeer(id dotcontext.ReplicaID, cc *dotcontext.CausalContext) {
	if _, ok := t.peers[id]; ok {
		return
	}
	if cc == nil {
		cc = dotcontext.New()
	}
	t.peers[id] = cc
}

// Peers returns all registered peer IDs. Order is non-deterministic.
func (t *PeerTracker) Peers() []dotcontext.ReplicaID {
	ids := make([]dotcontext.ReplicaID, 0, len(t.peers))
	for id := range t.peers {
		ids = append(ids, id)
	}
	return ids
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./replication/ -run "TestAddPeer" -v`
Expected: PASS (all three)

**Step 5: Commit**

```
git add replication/peertracker.go replication/peertracker_test.go
git commit -m "replication: PeerTracker AddPeer and Peers"
```

---

### Task 3: RemovePeer

**Files:**
- Modify: `replication/peertracker.go`
- Modify: `replication/peertracker_test.go`

**Step 1: Write the failing tests**

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./replication/ -run "TestRemovePeer" -v`
Expected: FAIL — `RemovePeer` not defined

**Step 3: Write minimal implementation**

```go
// RemovePeer deregisters a peer. Unknown peers are ignored.
func (t *PeerTracker) RemovePeer(id dotcontext.ReplicaID) {
	delete(t.peers, id)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./replication/ -run "TestRemovePeer" -v`
Expected: PASS

**Step 5: Commit**

```
git add replication/peertracker.go replication/peertracker_test.go
git commit -m "replication: PeerTracker RemovePeer"
```

---

### Task 4: Ack

**Files:**
- Modify: `replication/peertracker.go`
- Modify: `replication/peertracker_test.go`

**Step 1: Write the failing tests**

```go
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
	// CanGC is not implemented yet, so verify via Pending.
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
```

Note: This test uses `Pending` — implement `Ack` and `Pending` together in this task since the test verifies merge behavior through `Pending`.

**Step 2: Run tests to verify they fail**

Run: `go test ./replication/ -run "TestAck" -v`
Expected: FAIL — `Ack` and `Pending` not defined

**Step 3: Write minimal implementation**

```go
// Ack merges the given context into the peer's stored context.
// Handles both individual dot ACKs (small context) and wholesale
// context updates (anti-entropy CC exchange).
// Unknown peers are ignored — AddPeer first.
func (t *PeerTracker) Ack(id dotcontext.ReplicaID, cc *dotcontext.CausalContext) {
	stored, ok := t.peers[id]
	if !ok {
		return
	}
	stored.Merge(cc)
	stored.Compact()
}

// Pending returns the dots that the local context has but the named
// peer does not, in the same format as CausalContext.Missing().
// Returns nil for unknown peers.
func (t *PeerTracker) Pending(id dotcontext.ReplicaID, local *dotcontext.CausalContext) map[dotcontext.ReplicaID][]dotcontext.SeqRange {
	stored, ok := t.peers[id]
	if !ok {
		return nil
	}
	return stored.Missing(local)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./replication/ -run "TestAck" -v`
Expected: PASS

**Step 5: Commit**

```
git add replication/peertracker.go replication/peertracker_test.go
git commit -m "replication: PeerTracker Ack and Pending"
```

---

### Task 5: CanGC

**Files:**
- Modify: `replication/peertracker.go`
- Modify: `replication/peertracker_test.go`

**Step 1: Write the failing tests**

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./replication/ -run "TestCanGC" -v`
Expected: FAIL — `CanGC` not defined

**Step 3: Write minimal implementation**

```go
// CanGC reports whether all known peers have observed the given dot.
// Returns true if no peers are registered (vacuous truth).
func (t *PeerTracker) CanGC(d dotcontext.Dot) bool {
	for _, cc := range t.peers {
		if !cc.Has(d) {
			return false
		}
	}
	return true
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./replication/ -run "TestCanGC" -v`
Expected: PASS

**Step 5: Commit**

```
git add replication/peertracker.go replication/peertracker_test.go
git commit -m "replication: PeerTracker CanGC"
```

---

### Task 6: Pending edge cases and composability test

**Files:**
- Modify: `replication/peertracker_test.go`

**Step 1: Write the tests**

```go
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
```

**Step 2: Run tests to verify they pass (the new test) and the TODO is visible**

Run: `go test ./replication/ -run "TestPending" -v`
Expected: PASS for TestPendingUnknownPeer

**Step 3: Commit**

```
git add replication/peertracker_test.go
git commit -m "replication: Pending edge cases and composability TODO"
```

---

### Task 7: Full suite and lint

**Step 1: Run full test suite**

Run: `make lint && make && make test`
Expected: All pass, no lint errors

**Step 2: Verify no regressions in existing packages**

Run: `go test -race ./...`
Expected: All pass

**Step 3: Commit if any adjustments needed, otherwise done**
