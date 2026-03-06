# Peer Tracker Design

## Context

The `DeltaStore` (piece 1) buffers deltas indexed by the dot that created them.
The per-peer tracker (piece 2) answers two questions the delta store cannot:

1. **Which peers have acknowledged which dots?** — determines when a delta is
   safe to GC from the store.
2. **What does a given peer still need?** — determines what to eager-push.

This document covers the peer tracker data structure. GC orchestration policy
(piece 3) is a separate concern.

## Design Decisions

### 1. CausalContext per peer

Each peer's acknowledged state is represented as a `CausalContext`. This is the
natural choice: `CausalContext` already tracks which dots have been observed,
handles outliers (out-of-order ACKs), and compresses contiguous ranges into a
version vector.

A simpler version-vector-only representation would lose outlier information. If
a peer ACKs dot (a,5) before (a,4) — possible with concurrent pushes — the
version vector can only record the contiguous frontier. The tracker would
think the peer still needs (a,5) and re-push it. Harmless (idempotent join)
but wasteful. CausalContext avoids this.

### 2. Ack merges, never replaces

A peer's knowledge can only grow. `Ack()` merges the incoming context into the
stored context using `CausalContext.Merge()`. This is correct even when a peer
sends a stale CausalContext (e.g., from anti-entropy initiation before
receiving recent pushes) — merge takes the max per-replica, preserving
everything the tracker already knows.

### 3. Remove-deltas are fire-and-forget

Remove operations (AWSet.Remove, EWFlag.Disable) produce deltas with no new
dot. They cannot be indexed in the DeltaStore. The tracker does not buffer or
track remove delivery.

Removes are pushed eagerly on first attempt. If the push fails, anti-entropy
(full state comparison via `Missing()`) handles eventual delivery. The stale
window — time between push failure and next anti-entropy round — only exists
for transient push failures to borderline-unreachable peers, who are already
behind on adds too.

### 4. n is implicit

`CanGC(dot)` checks all registered peers. For n-k subsystem nodes, peers = the
n servers. For a client, peers = its server connections. The tracker doesn't
know about n — it just knows its peer set. GC-at-n falls out naturally.

### 5. Deliberately conservative

A peer that goes permanently offline and is never removed blocks GC forever.
This is intentional. The tracker has no timers, no timeouts, no notion of
"too long." Temporal judgments (is this peer dead?) belong to the failure
detector. `RemovePeer()` is the release valve — calling it is a
failure-detection decision, not a replication decision.

### 6. Separate from DeltaStore

The tracker is pure peer state. It does not own or reference the DeltaStore.
A higher-level coordinator composes both. This keeps the types decoupled and
independently testable.

Composition at the call site:

```go
pending := tracker.Pending(peerID, localCC)
deltas  := store.Fetch(pending)
// send deltas to peer
```

GC at the call site:

```go
for _, dot := range store.Dots() {
    if tracker.CanGC(dot) {
        store.Remove(dot)
    }
}
```

### 7. Not thread-safe

Follows the DeltaStore convention. The caller handles synchronization.

## Package

Lives in `replication/`, a new package. Imports `dotcontext` for `ReplicaID`,
`Dot`, `CausalContext`, `SeqRange`. No other dependencies.

`ReplicaID` serves dual roles: it identifies dot producers (inside
CausalContext) and peers (as tracker map keys). Same type, different semantic
roles. No new type needed.

## API

```go
package replication

import "github.com/aalpar/crdt/dotcontext"

type PeerTracker struct {
    peers map[dotcontext.ReplicaID]*dotcontext.CausalContext
}

func NewPeerTracker() *PeerTracker

// AddPeer registers a peer with its initial known state.
// A nil context is treated as empty (peer knows nothing).
// Adding an already-known peer is a no-op.
func (t *PeerTracker) AddPeer(id dotcontext.ReplicaID, cc *dotcontext.CausalContext)

// RemovePeer deregisters a peer. Unknown peers are ignored.
func (t *PeerTracker) RemovePeer(id dotcontext.ReplicaID)

// Ack merges the given context into the peer's stored context.
// Handles both individual dot ACKs (small context) and wholesale
// context updates (anti-entropy CC exchange).
// Unknown peers are ignored — AddPeer first.
func (t *PeerTracker) Ack(id dotcontext.ReplicaID, cc *dotcontext.CausalContext)

// CanGC reports whether all known peers have observed the given dot.
// Returns true if no peers are registered (vacuous truth).
func (t *PeerTracker) CanGC(d dotcontext.Dot) bool

// Pending returns the dots that the local context has but the named
// peer does not, in the same format as CausalContext.Missing().
// Returns nil for unknown peers.
func (t *PeerTracker) Pending(
    id dotcontext.ReplicaID,
    local *dotcontext.CausalContext,
) map[dotcontext.ReplicaID][]dotcontext.SeqRange

// Peers returns all registered peer IDs. Order is non-deterministic.
func (t *PeerTracker) Peers() []dotcontext.ReplicaID

// Len returns the number of registered peers.
func (t *PeerTracker) Len() int
```

## Edge Cases

| Case | Behavior |
|------|----------|
| `AddPeer` with nil context | Treated as empty context |
| `AddPeer` for known peer | No-op |
| `RemovePeer` for unknown peer | No-op |
| `Ack` for unknown peer | No-op |
| `CanGC` with zero peers | true (vacuous) |
| `CanGC` after removing lagging peer | May become true |
| `Pending` for unknown peer | nil |

## Testing Plan

Tests in `replication/peertracker_test.go`:

| Test | Behavior |
|------|----------|
| `TestNewEmpty` | Zero peers, Len() == 0 |
| `TestAddPeer` | Peer in Peers(), Len increments |
| `TestAddPeerNilContext` | nil cc → empty, peer registered |
| `TestAddPeerDuplicate` | Second add is no-op |
| `TestRemovePeer` | Peer gone, Len decrements |
| `TestRemovePeerUnknown` | No panic |
| `TestAckMerges` | Merges into stored context |
| `TestAckUnknownPeer` | No-op |
| `TestCanGCAllAcked` | true when all peers have dot |
| `TestCanGCSomeNotAcked` | false when any peer lacks dot |
| `TestCanGCNoPeers` | true (vacuous) |
| `TestCanGCAfterRemovePeer` | Removing lagging peer can unblock GC |
| `TestPendingComposesWithFetch` | Output feeds DeltaStore.Fetch() |
| `TestPendingUnknownPeer` | nil |

## What This Does NOT Cover

- **Remove-delta delivery** — fire-and-forget + anti-entropy
- **Anti-entropy mechanism** — separate protocol concern
- **Wire protocol** — how ACKs are serialized and transmitted
- **Concurrency** — not thread-safe, caller synchronizes
- **Peer discovery** — tracker is told about peers, doesn't find them
- **GC orchestration** — tracker answers CanGC; when to scan is policy (piece 3)
- **Failure detection** — a failed peer blocks GC until RemovePeer is called
