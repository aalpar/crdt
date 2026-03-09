# Replication

## From Merge to Network

The previous chapters covered the algebra: dots, contexts, stores, and
joins. All of that works in memory on a single machine. Now we need to
get deltas from one replica to another.

The replication pipeline has four stages:

```
mutate → buffer delta → compute what's missing → ship → merge → ack → GC
```

This chapter walks through each stage using the concrete types in the
`replication/` and `dotcontext/` packages.

## Stage 1: Buffer the Delta

When a CRDT mutator runs, it returns a delta — a minimal `Causal`
value. The caller stores this delta in a `DeltaStore`, indexed by the
dot that created it:

```go
delta := set.Add("x")
state := delta.State()

// Extract the dot (the delta contains exactly one new dot).
var d Dot
state.Store.Range(func(_ string, ds *DotSet) bool {
    ds.Range(func(dot Dot) bool {
        d = dot
        return false
    })
    return false
})

store.Add(d, state)
```

The `DeltaStore` is a simple `map[Dot]T`:

```go
type DeltaStore[T any] struct {
    deltas map[Dot]T
}
```

Indexing by dot means each delta is uniquely identified by the event
that created it. This enables efficient range queries later.

## Stage 2: Compute What's Missing

Before shipping deltas, the sender needs to know what the receiver is
missing. This is where `CausalContext.Missing()` comes in.

The receiver shares its causal context with the sender. The sender
compares:

```go
missing := receiver.Missing(sender)
// Returns map[ReplicaID][]SeqRange — the gaps per replica
```

`Missing` returns compressed `SeqRange` slices — inclusive `[Lo, Hi]`
ranges. For example, if the receiver has seen dots 1-5 and 8 from
replica "alice," and the sender has dots 1-10, `Missing` returns
`[{6, 7}, {9, 10}]` for "alice" — the two gaps.

The delta store's `Fetch` method accepts this format directly:

```go
deltas := store.Fetch(missing)
// Returns map[Dot]T — all stored deltas within the missing ranges
```

This composability — `Missing()` produces what `Fetch()` consumes — is
intentional. The pipeline is:

```go
missing := receiverCC.Missing(senderCC)
deltas  := store.Fetch(missing)
```

Two function calls, no intermediate transformation.

## Stage 3: Encode and Ship

Deltas are encoded to a binary stream using the codec system. The wire
format for a batch is:

```
[uint64: count] ([Dot] [delta via codec])*
```

The `WriteDeltaBatch` function packages the whole pipeline:

```go
func WriteDeltaBatch[T any](
    localCC  *CausalContext,
    remoteCC *CausalContext,
    store    *DeltaStore[T],
    codec    Codec[T],
    w        io.Writer,
) (int, error) {
    missing := remoteCC.Missing(localCC)
    deltas := store.Fetch(missing)
    // Encode deltas to w...
}
```

On the receiving end, `ReadDeltaBatch` decodes and applies:

```go
func ReadDeltaBatch[T any](
    codec Codec[T],
    r     io.Reader,
    apply func(Dot, T),
) (int, error) {
    // Decode from r, call apply for each delta...
}
```

The `apply` callback is where the receiver joins each delta into its
local state:

```go
ReadDeltaBatch(codec, &buf, func(_ Dot, delta CausalType) {
    localSet.Merge(awset.FromCausal[string](delta))
})
```

## The Codec System

Each dot store type has a corresponding codec. The codecs compose the
same way the stores do:

| Store Type | Codec | Wire Format |
|------------|-------|-------------|
| `Dot` | `DotCodec` | `[string: ID] [uint64: Seq]` |
| `*CausalContext` | `CausalContextCodec` | `[VV entries] [outliers]` |
| `*DotSet` | `DotSetCodec` | `[uint64: len] (Dot)*` |
| `*DotFun[V]` | `DotFunCodec[V]` | `[uint64: len] ([Dot] [V])*` |
| `*DotMap[K,V]` | `DotMapCodec[K,V]` | `[uint64: len] ([K] [V])*` |
| `Causal[T]` | `CausalCodec[T]` | `[T: store] [CausalContext]` |

The codecs nest just like the stores. An AWSet delta codec is:

```go
CausalCodec[*DotMap[string, *DotSet]]{
    StoreCodec: DotMapCodec[string, *DotSet]{
        KeyCodec:   StringCodec{},
        ValueCodec: DotSetCodec{},
    },
}
```

All length-prefixed fields cap allocations at `maxDecodeLen` (~1 million
entries) to prevent OOM from malformed input.

## Stage 4: Acknowledge

After receiving and applying deltas, the receiver's causal context has
advanced. The sender needs to know this so it can stop buffering deltas
the receiver no longer needs.

The `PeerTracker` maintains a causal context per peer:

```go
type PeerTracker struct {
    peers map[ReplicaID]*CausalContext
}
```

When a sync completes, the sender acknowledges by merging the
receiver's updated context:

```go
tracker.Ack(peerID, peerContext.Clone())
```

`Ack` merges the given context into the stored one for that peer:

```go
func (t *PeerTracker) Ack(id ReplicaID, cc *CausalContext) {
    stored, ok := t.peers[id]
    if !ok {
        return
    }
    stored.Merge(cc)
    stored.Compact()
}
```

Because contexts only grow (merge is monotonic), acknowledgments are
cumulative. A later ack subsumes all earlier ones.

## Stage 5: Garbage Collection

Deltas can be garbage collected once *all* tracked peers have
acknowledged them. The `GC` function checks each stored delta:

```go
func GC[T any](store *DeltaStore[T], tracker *PeerTracker) int {
    var removed int
    for _, d := range store.Dots() {
        if tracker.CanGC(d) {
            store.Remove(d)
            removed++
        }
    }
    return removed
}
```

`CanGC` is simple: does every peer's context contain this dot?

```go
func (t *PeerTracker) CanGC(d Dot) bool {
    for _, cc := range t.peers {
        if !cc.Has(d) {
            return false
        }
    }
    return true  // vacuously true if no peers
}
```

A delta stays buffered until the slowest peer has acknowledged it. This
means a permanently-offline peer will prevent GC. In practice, you'd
remove such peers from the tracker (`RemovePeer`), allowing their
deltas to be collected.

## The Full Lifecycle

Here's the complete cycle for two replicas, Alice and Bob:

```
1. Alice adds "x"
   → delta stored: dot (alice:1) → Causal[...]{x → {alice:1}}

2. Alice syncs to Bob
   → Bob's context is empty → Missing returns {alice: [1,1]}
   → Fetch returns the delta for alice:1
   → Encode → ship → Decode → Bob merges → Bob now has "x"

3. Bob acks
   → Alice's tracker: bob has seen {alice:1}

4. Alice GCs
   → CanGC(alice:1) → bob has it → remove from store
   → Store is now empty
```

For concurrent mutations with conflict resolution:

```
1. Alice adds "x" (alice:1). Bob adds "x" (bob:1). Both offline.

2. Sync alice → bob
   → Bob receives alice:1 delta, merges
   → "x" now has dots {alice:1, bob:1} — both adds survive

3. Sync bob → alice
   → Alice receives bob:1 delta, merges
   → "x" now has dots {alice:1, bob:1} on both sides
   → Converged.
```

For add-wins conflict:

```
1. Both have "x" with dot (alice:1), synced.

2. Alice re-adds "x" → new dot (alice:2).
   Bob removes "x" → his delta has context {alice:1}, empty store.

3. Sync alice → bob
   → Bob merges alice's delta: dot alice:2 not in bob's context → survives
   → "x" is back with dot {alice:2}

4. Sync bob → alice
   → Alice merges bob's remove delta: context {alice:1} kills nothing
     (alice:1 is already superseded by alice:2)
   → "x" survives on both.
```

## Remove Deltas: A Special Case

Remove operations don't generate new dots — they produce a delta with
an empty store and a context containing the removed dots. This means
remove deltas can't be indexed in the `DeltaStore` (which indexes by
the creating dot).

In the current design, remove information propagates via full-state
sync or by being embedded in subsequent add deltas. When a replica
removes an element and later adds a new one, the add delta's context
carries both the new dot and any old dots — effectively piggybacking
the removal information.

For systems that need standalone remove propagation, the delta can be
encoded and sent directly over the wire (the codec handles empty
stores correctly) — it just can't be stored in the dot-indexed delta
buffer.

## Properties

The replication layer preserves all the properties of the underlying
algebra:

- **Idempotent**: Receiving the same delta twice has no effect. The
  second merge produces the same state (tested in
  `TestE2EDuplicateDeltaDelivery`).

- **Order-independent**: Deltas can arrive out of order and the
  converged state is the same (tested in
  `TestE2EOutOfOrderDelivery`).

- **Incremental**: Only missing deltas are shipped. After a sync
  round, a subsequent round transfers nothing unless new mutations
  have occurred (tested in `TestE2EIncrementalSync`).

- **Conflict-preserving**: Add-wins, enable-wins, and LWW semantics
  survive the encode→decode→merge pipeline (tested for all five
  CRDTs in the e2e test suite).

[Previous: Building CRDTs from Composition](04-building-crdts.md)
