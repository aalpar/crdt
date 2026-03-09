# Building CRDTs from Composition

## The Recipe

Every CRDT in this library follows the same recipe:

1. Pick a dot store type (`DotSet`, `DotFun`, `DotMap`) that can
   represent your data.
2. Pair it with a `CausalContext` in a `Causal` value.
3. Write mutators that generate dots via `Context.Next()`, mutate local
   state directly, and return a delta (a minimal `Causal` value
   containing just the changes).
4. Merge uses the corresponding join function.

The choice of dot store determines what the CRDT can express. The join
function determines how conflicts resolve. Everything else is
composition.

## The Delta Mutator Pattern

Mutators that *introduce* new state (Add, Set, Increment, Enable)
all follow one pattern. It's worth seeing the full shape before
diving into individual CRDTs:

```go
func (s *SomeCRDT) Mutate(args...) *SomeCRDT {
    // 1. Generate a new dot from the shared causal context.
    d := s.state.Context.Next(s.id)

    // 2. Mutate local state directly.
    //    (Add d to the store, remove old dots, etc.)

    // 3. Build a delta: a minimal Causal value.
    //    Store: only the new/changed data.
    //    Context: the new dot + any old dots being superseded.

    // 4. Return the delta (NOT merged back into self).
    return &SomeCRDT{state: delta}
}
```

Mutators that only *remove* state (Remove, Disable) follow a
different shape: no dot generation, just record existing dots in the
delta's context and clear them from the local store.

Two things to note:

**The mutator does NOT merge the delta back into itself.** `Next()`
already advanced the causal context, so the local state is already
up-to-date. If you merged the delta back, the join function would see
the dot in both the store *and* the context, potentially misinterpreting
it. This "no self-merge" rule is a critical invariant.

**The delta is itself a valid CRDT value.** The caller ships it to
remote replicas, who merge it using the same join function they'd use
for a full state. The receiver doesn't need to know whether it's
getting a delta or a full state — the merge is the same either way.

## EWFlag: The Simplest CRDT

An enable-wins flag is the minimal delta-state CRDT. Its state is
`Causal[*DotSet]` — a bare dot set plus a causal context. The flag
is enabled if the dot set is non-empty.

**Enable** generates a dot and adds it to the set:

```go
func (p *EWFlag) Enable() *EWFlag {
    d := p.state.Context.Next(p.id)
    p.state.Store.Add(d)

    // Delta: one dot in store, one dot in context.
    deltaStore := NewDotSet()
    deltaStore.Add(d)
    deltaCtx := New()
    deltaCtx.Add(d)
    return &EWFlag{state: Causal[*DotSet]{Store: deltaStore, Context: deltaCtx}}
}
```

**Disable** clears the dot set and records the old dots in the delta's
context:

```go
func (p *EWFlag) Disable() *EWFlag {
    deltaCtx := New()
    p.state.Store.Range(func(d Dot) bool {
        deltaCtx.Add(d)
        return true
    })
    p.state.Store = NewDotSet()  // clear local store

    // Delta: empty store, context has the old dots.
    return &EWFlag{state: Causal[*DotSet]{Store: NewDotSet(), Context: deltaCtx}}
}
```

**Why enable wins**: Suppose Alice enables (generating dot a:1) and Bob
concurrently disables. Bob's disable delta has an empty store and a
context containing whatever dots were in his store at the time. When
merged, Alice's dot a:1 is not in Bob's context (he hasn't seen it),
so it survives. The flag stays enabled. The new, unseen dot always beats
the removal.

**Merge** is just `JoinDotSet`:

```go
func (p *EWFlag) Merge(other *EWFlag) {
    p.state = JoinDotSet(p.state, other.state)
}
```

The entire CRDT is ~30 lines of real logic. Everything else is the
algebra doing its job.

## AWSet: Add-Wins Set

An add-wins set stores elements. Concurrent add and remove of the same
element resolves in favor of add.

Its state is `Causal[*DotMap[E, *DotSet]]` — each element maps to a
set of dots that witness its addition.

**Add** generates a new dot and adds it to the element's dot set:

```go
func (p *AWSet[E]) Add(elem E) *AWSet[E] {
    d := p.state.Context.Next(p.id)

    ds, ok := p.state.Store.Get(elem)
    if !ok {
        ds = NewDotSet()
        p.state.Store.Set(elem, ds)
    }
    ds.Add(d)

    // Delta: elem → {d}, context = {d}
    deltaStore := NewDotMap[E, *DotSet]()
    deltaDS := NewDotSet()
    deltaDS.Add(d)
    deltaStore.Set(elem, deltaDS)
    deltaCtx := New()
    deltaCtx.Add(d)

    return &AWSet[E]{state: Causal[...]{Store: deltaStore, Context: deltaCtx}}
}
```

Each `Add` produces a new dot, even if the element is already present.
After three adds of "x" by the same replica, "x" has three dots. This
accumulation is what makes add-wins work: a remove kills specific dots,
but a concurrent add introduces a *new* dot that the remover hasn't
seen.

**Remove** records the element's current dots in the delta's context
and removes the element from local state:

```go
func (p *AWSet[E]) Remove(elem E) *AWSet[E] {
    deltaCtx := New()
    if ds, ok := p.state.Store.Get(elem); ok {
        ds.Range(func(d Dot) bool {
            deltaCtx.Add(d)
            return true
        })
        p.state.Store.Delete(elem)
    }
    // Delta: empty store, context = element's dots.
    return &AWSet[E]{state: Causal[...]{Store: NewDotMap[E, *DotSet](), Context: deltaCtx}}
}
```

The remove delta is *context-only* — its store is empty. When merged
into a remote replica, the dots in the context are treated as "seen and
removed," killing any matching dots on the remote side. But any new
dots (concurrent adds) that aren't in the context survive.

**Merge** uses `JoinDotMap` with `JoinDotSet` as the nested join:

```go
func (p *AWSet[E]) Merge(other *AWSet[E]) {
    p.state = JoinDotMap(p.state, other.state, joinDotSet, NewDotSet)
}
```

## LWWRegister: Last Writer Wins

A register holds a single value. Concurrent writes are resolved by
timestamp: highest timestamp wins, with lexicographic replica ID as
tiebreaker.

Its state is `Causal[*DotFun[Timestamped[V]]]` — each dot maps to a
value paired with its timestamp.

```go
type Timestamped[V any] struct {
    Value V
    Ts    int64
}
```

**Set** generates a new dot, removes all old dots from the local store,
and records them in the delta's context:

```go
func (p *LWWRegister[V]) Set(value V, timestamp int64) *LWWRegister[V] {
    // Collect old dots.
    var oldDots []Dot
    p.state.Store.Range(func(d Dot, _ Timestamped[V]) bool {
        oldDots = append(oldDots, d)
        return true
    })

    // New dot, remove old ones.
    d := p.state.Context.Next(p.id)
    for _, od := range oldDots {
        p.state.Store.Remove(od)
    }
    entry := Timestamped[V]{Value: value, Ts: timestamp}
    p.state.Store.Set(d, entry)

    // Delta: new entry + context covering old dots.
    deltaStore := NewDotFun[Timestamped[V]]()
    deltaStore.Set(d, entry)
    deltaCtx := New()
    deltaCtx.Add(d)
    for _, od := range oldDots {
        deltaCtx.Add(od)
    }
    return &LWWRegister[V]{state: ...}
}
```

After a `Set`, the local store contains exactly one dot. But after
merging concurrent writes from different replicas, the store might
temporarily contain multiple dots (one per concurrent writer).

**Value** resolves concurrent entries by picking the highest timestamp,
breaking ties on replica ID:

```go
func (p *LWWRegister[V]) Value() (V, int64, bool) {
    var bestDot Dot
    var best Timestamped[V]
    found := false

    p.state.Store.Range(func(d Dot, e Timestamped[V]) bool {
        if !found || e.Ts > best.Ts ||
            (e.Ts == best.Ts && d.ID > bestDot.ID) {
            bestDot = d
            best = e
            found = true
        }
        return true
    })
    // ...
}
```

The conflict resolution is entirely in the query — the merge itself
just preserves all concurrent entries via `JoinDotFun`. This is a
common pattern: the join keeps everything that should survive, and the
read path resolves conflicts.

## PNCounter: Positive-Negative Counter

A counter where each replica tracks its accumulated contribution. The
global value is the sum of all contributions.

Its state is `Causal[*DotFun[CounterValue]]` — same structure as
LWWRegister, but with a different value type and different semantics.

```go
type CounterValue struct {
    N int64
}
```

**Increment** finds this replica's current dot (if any), removes it,
and creates a new dot with the updated sum:

```go
func (p *Counter) Increment(n int64) *Counter {
    // Find our current dot.
    var oldDot Dot
    var oldValue int64
    hasOld := false
    p.state.Store.Range(func(d Dot, v CounterValue) bool {
        if d.ID == p.id {
            oldDot = d
            oldValue = v.N
            hasOld = true
            return false
        }
        return true
    })

    // New dot with accumulated value.
    d := p.state.Context.Next(p.id)
    if hasOld {
        p.state.Store.Remove(oldDot)
    }
    newVal := CounterValue{N: oldValue + n}
    p.state.Store.Set(d, newVal)

    // Delta: new entry + context covering old dot.
    // ...
}
```

Each replica has at most one dot in the store at any time. Incrementing
replaces it — the value is cumulative. The global counter value is
the sum of all entries:

```go
func (p *Counter) Value() int64 {
    var total int64
    p.state.Store.Range(func(_ Dot, v CounterValue) bool {
        total += v.N
        return true
    })
    return total
}
```

`Decrement` is just `Increment(-n)`.

## ORMap: Observed-Remove Map

An ORMap maps keys to nested CRDT values. It's the most general
structure: the value type `V` can be any `DotStore`, enabling
arbitrary nesting.

```go
type ORMap[K comparable, V DotStore] struct {
    id     ReplicaID
    state  Causal[*DotMap[K, V]]
    joinV  func(Causal[V], Causal[V]) Causal[V]
    emptyV func() V
}
```

ORMap stores its value join function and empty constructor, so it can
pass them to `JoinDotMap` on merge.

**Apply** is the general-purpose mutator. It takes a key and a callback
that mutates the value and populates a delta:

```go
func (p *ORMap[K, V]) Apply(key K, fn func(
    id ReplicaID,
    ctx *CausalContext,
    v V,          // the current value — mutate in place
    delta V,      // a fresh empty value — populate with delta entries
)) *ORMap[K, V] {
```

The callback receives the replica ID and context (for dot generation),
the current value (to mutate), and an empty delta value (to populate).
After the callback runs, `Apply` compares the before/after dots to
determine which dots were added and removed, and builds the delta's
context from the difference.

This design lets ORMap work with any nested store type without knowing
its specifics. The callback is where the actual logic lives — ORMap
just handles the delta bookkeeping.

**Remove** follows the same pattern as AWSet's remove: record all of
the key's dots in the context, delete the key locally.

**Merge** delegates to `JoinDotMap`:

```go
func (p *ORMap[K, V]) Merge(other *ORMap[K, V]) {
    p.state = JoinDotMap(p.state, other.state, p.joinV, p.emptyV)
}
```

## The Composition Table

| CRDT | Dot Store | Value Type | Join Function |
|------|-----------|------------|---------------|
| EWFlag | `*DotSet` | — | `JoinDotSet` |
| AWSet[E] | `*DotMap[E, *DotSet]` | — | `JoinDotMap` with `JoinDotSet` |
| LWWRegister[V] | `*DotFun[Timestamped[V]]` | `Timestamped[V]` | `JoinDotFun` |
| PNCounter | `*DotFun[CounterValue]` | `CounterValue` | `JoinDotFun` |
| ORMap[K, V] | `*DotMap[K, V]` | Any `DotStore` | `JoinDotMap` with caller-supplied `joinV` |

Five CRDTs, three join functions, three dot store types. No CRDT
implements its own merge logic — they all delegate to the algebra layer.

## Why This Decomposition Matters

The power of this approach is that correctness is proven once, at the
algebra layer, and inherited by every CRDT that composes it. If
`JoinDotSet` is commutative, associative, and idempotent (and the
fuzz tests verify this), then EWFlag's merge is automatically correct.
If `JoinDotMap` correctly recurses, then AWSet, ORMap, and any future
DotMap-based CRDT get correct merging for free.

Adding a new CRDT means choosing a composition and writing mutators.
The merge is already done.

[Previous: Dot Stores and Joins](03-dot-stores-and-joins.md) |
[Next: Replication](05-replication.md)
