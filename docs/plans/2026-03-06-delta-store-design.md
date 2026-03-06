# Delta Store Design

## Context

`CausalContext.Missing(remote)` answers "what dots am I behind on?" The delta
store answers the complement: "given those dots, which deltas should I send?"

This document covers piece 1 — the pure data structure. Peer tracking (piece 2)
and GC policy (piece 3) are strategy-layer concerns, not covered here.

## Design Decisions

### 1. Eager-push buffer, not a history log

The delta store is an in-memory buffer for eager push. Mutator fires → delta
stored → shipped to peers → ACK'd → GC'd. Short-lived.

Anti-entropy (computing diffs from current state + `Missing()`) is a separate
mechanism that serves as crash recovery and big-gap fallback. The delta store
is not involved in anti-entropy.

Consequence: in-memory is fine. Crash → lose buffer → anti-entropy covers you.

### 2. One delta per dot

Each CRDT mutator call produces one delta containing one dot. The store indexes
by that dot. `Missing()` ranges map directly to lookups.

Remove operations (AWSet.Remove, EWFlag.Disable) produce deltas with no new
dots. These are not indexed in the delta store — they are a peer-tracker
concern (piece 2). Anti-entropy ensures eventual delivery regardless.

### 3. Replication fanout rule (future — informs GC)

Not implemented here, but shapes the design:

- Client nodes must deliver deltas to at least 2 peers
- Peer/server nodes must deliver deltas to at least 1 peer

A delta is safe to GC when the required number of ACKs arrive. The store
provides `Remove(dot)` as the GC primitive; the policy lives elsewhere.

## API

```go
type DeltaStore[T any] struct {
    deltas map[Dot]T
}

func NewDeltaStore[T any]() *DeltaStore[T]

// Add stores a delta indexed by the dot that created it.
func (s *DeltaStore[T]) Add(d Dot, delta T)

// Get retrieves a single delta by its dot.
func (s *DeltaStore[T]) Get(d Dot) (T, bool)

// Fetch returns all stored deltas whose dots fall within the given ranges.
// Takes Missing()'s return type directly for composability:
//   store.Fetch(local.Missing(remote))
func (s *DeltaStore[T]) Fetch(missing map[ReplicaID][]SeqRange) map[Dot]T

// Remove deletes a delta by its dot (GC primitive).
func (s *DeltaStore[T]) Remove(d Dot)

// Len returns the number of stored deltas.
func (s *DeltaStore[T]) Len() int
```

### Design choices

- **Generic over `T any`** — the store doesn't inspect deltas, just stores and
  retrieves. Each CRDT instance gets its own typed store.
- **`Fetch` composes with `Missing()`** — same `map[ReplicaID][]SeqRange` type.
- **`map[Dot]T` internally** — O(n) scan for `Fetch`. Acceptable because the
  buffer is small in steady state (deltas live between creation and ACK).
- **No sorted index** — premature for an ephemeral buffer. If profiling shows
  `Fetch` is hot, a `map[ReplicaID]map[uint64]T` two-level index is the
  obvious next step.

## Relationship to existing code

- Lives in `dotcontext/` alongside `Missing()`, `SeqRange`, `Dot`, `ReplicaID`
- Uses no types from layer 2 (CRDTs) — pure algebra
- `Fetch` input type matches `Missing()` output type by construction

## What this does NOT cover

- Per-peer ACK tracking (piece 2, strategy layer)
- GC policy / fanout requirements (piece 3, strategy layer)
- Remove-delta delivery (piece 2)
- Anti-entropy mechanism (separate from delta store)
- Wire protocol / serialization
