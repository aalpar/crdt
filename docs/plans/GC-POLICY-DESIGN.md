# GC Policy Design

## Context

The `DeltaStore` (piece 1) buffers deltas. The `PeerTracker` (piece 2) tracks
which peers have acknowledged which dots. The GC policy (piece 3) composes
both: scan the store, remove deltas that all peers have acknowledged.

## Design Decisions

### 1. GC after every Ack

The buffer is small in steady state (deltas live between creation and ACK).
The scan is O(buffer × peers), both small numbers. Running GC after every
ACK is the simplest trigger and the most eager — no stale deltas linger.

### 2. Free function, not a composing type

`GC[T any](store, tracker)` keeps `DeltaStore` and `PeerTracker` fully
independent. No wrapper type needed. The caller orchestrates:

```go
tracker.Ack(peerID, cc)
replication.GC(store, tracker)
```

### 3. Dots() snapshot for safe iteration

`DeltaStore.Dots()` returns a copy of all stored dots as `[]Dot`. The GC
function iterates this snapshot while calling `Remove()`, which is safe
because the snapshot is independent of the map.

## API

New method on `DeltaStore` (in `dotcontext/`):

```go
// Dots returns a snapshot of all stored dots.
func (s *DeltaStore[T]) Dots() []Dot
```

New function in `replication/`:

```go
// GC removes deltas from the store that all tracked peers have
// acknowledged. Returns the number of deltas removed.
func GC[T any](store *dotcontext.DeltaStore[T], tracker *PeerTracker) int
```

## Edge Cases

| Case | Behavior |
|------|----------|
| Empty store | Returns 0 |
| No peers registered | CanGC vacuously true — clears entire buffer |
| Dots() during Remove | Safe — Dots() returns a copy |

## What This Does NOT Cover

- When to call GC beyond "after every Ack" (caller's decision)
- Metrics or logging of GC activity
- Memory pressure–based GC (YAGNI — buffer is small)
