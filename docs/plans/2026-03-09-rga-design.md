# RGA Design — Replicated Growable Array

Delta-state RGA composing dotcontext types. Immutable elements, insert/delete only.

## Composition

```
Causal[*DotFun[Node[E]]]
```

Each sequence element is a dot in the DotFun. `Node[E]` stores:

- `Value E` — the element
- `After Dot` — predecessor (zero Dot = head of list)
- `Deleted bool` — tombstone flag

`Node[E]` implements `Lattice[Node[E]]`: `Join` produces `Deleted = a.Deleted || b.Deleted` (delete wins, monotonic).

## Why tombstones

Standard delta-state deletion (dot in context, absent from store) won't work here. Deleted nodes serve as **tree anchors** — their children reference them via `After`. Without the anchor, children become orphans and ordering breaks. Tombstoned nodes remain in the DotFun but are invisible in `Elements()`.

Trade-off: tombstones accumulate forever. GC requires all replicas to agree a subtree can be pruned — deferred for v0.x.

## Ordering

The sequence is a forest rooted at the head sentinel (zero Dot). Each node's `After` field points to its parent. Linearization walks the tree depth-first.

**Sibling tiebreak**: concurrent inserts after the same parent are ordered by dot comparison: `Seq` descending, then `ID` descending. Higher dot goes left (earlier in sequence). Matches original RGA semantics (Roh et al. 2011).

**Orphan handling**: if a node references a parent not yet in the local DotFun (out-of-order delivery), it attaches to head during linearization. Converges once all deltas arrive.

## Mutators

### InsertAfter(after Dot, value E) → delta

1. `dot := ctx.Next(id)` — allocate new dot
2. `node := Node[E]{Value: value, After: after, Deleted: false}`
3. Mutate local state: `store.Set(dot, node)`
4. Delta: DotFun with `{dot → node}`, context with `{dot}`

### Delete(at Dot) → delta

1. Look up `node, ok := store.Get(at)` — no-op if missing or already deleted
2. Mutate local state: `store.Set(at, Node{..., Deleted: true})`
3. Delta: DotFun with `{at → Node{..., Deleted: true}}`, context with `{at}`

Note: unlike AWSet/LWWRegister, Delete does **not** remove the dot from the store. It upgrades the tombstone flag via the lattice join.

### InsertAt(index int, value E) → delta

Convenience: linearize, find the dot at `index-1` (or zero Dot if index=0), delegate to `InsertAfter`.

### DeleteAt(index int) → delta

Convenience: linearize, find the dot at `index`, delegate to `Delete`.

## Merge

```go
func (r *RGA[E]) Merge(other *RGA[E]) {
    dotcontext.MergeDotFun(&r.state, other.state)
}
```

`MergeDotFunStore` handles everything:
- Dots in both stores: `node.Join(other)` → `Deleted` wins
- Dots in delta but not state (and not in state's context): added
- Dots in state but observed by delta's context and not in delta's store: removed

The third case (causal removal) shouldn't occur in normal RGA operation — tombstoned nodes are always present in the store. But it's handled correctly by the existing merge machinery.

## API

```go
type RGA[E comparable] struct { ... }

func New[E comparable](replicaID ReplicaID) *RGA[E]
func (r *RGA[E]) InsertAfter(after Dot, value E) *RGA[E]
func (r *RGA[E]) Delete(at Dot) *RGA[E]
func (r *RGA[E]) InsertAt(index int, value E) *RGA[E]
func (r *RGA[E]) DeleteAt(index int) *RGA[E]
func (r *RGA[E]) Elements() []Element[E]
func (r *RGA[E]) At(index int) (Element[E], bool)
func (r *RGA[E]) Len() int
func (r *RGA[E]) Merge(other *RGA[E])
func (r *RGA[E]) State() Causal[*DotFun[Node[E]]]
func FromCausal[E comparable](state Causal[*DotFun[Node[E]]]) *RGA[E]

type Element[E comparable] struct {
    ID    Dot
    Value E
}

type Node[E comparable] struct {
    Value   E
    After   Dot
    Deleted bool
}
```

## Codec

Composes existing codecs:

```
CausalCodec[*DotFun[Node[E]]]{
    StoreCodec: DotFunCodec[Node[E]]{
        ValueCodec: NodeCodec[E]{
            ValueCodec: <E's codec>,
        },
    },
}
```

`NodeCodec[E]` encodes: value (via E's codec) + After (DotCodec) + Deleted (single byte).

## Files

```
rga/
  rga.go          — type, constructor, mutators, merge, accessors
  doc.go          — package documentation
  rga_test.go     — unit tests (insert, delete, concurrent ops, linearization)
  shared_test.go  — crdttest.Harness configuration
```

## Test harness ops (≥5)

1. InsertAfter(zero, "a") — insert at head
2. InsertAfter(zero, "b") — different value at head
3. InsertAfter(last, "c") — append at tail
4. Delete(first) — delete first visible element
5. InsertAfter(first, "d") — insert after first visible element

## Alternatives considered

| Approach | Pros | Cons |
|----------|------|------|
| **DotFun[Node] + tombstone** ✓ | Single store, uses existing merge | Tombstone accumulation |
| DotMap[Dot, *DotSet] | No tombstones | Dot-as-key confusing, loses value |
| ORMap + ordering metadata | Reuses ORMap | Over-engineered for immutable elements |
