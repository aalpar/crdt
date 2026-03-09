# MVRegister Design

Multi-value register: concurrent writes are all preserved, not resolved.

## Composition

```
Causal[*DotFun[mvrValue[V]]]
```

- `mvrValue[V any]` — unexported wrapper satisfying `Lattice` trivially (same-dot values are identical, join returns self)
- V constraint: `any` — no comparability requirement
- Follows LWWRegister precedent: LWW wraps V in `Timestamped[V]`, MVR wraps V in `mvrValue[V]`

## API

```go
func New[V any](replicaID ReplicaID) *MVRegister[V]
func (r *MVRegister[V]) Write(v V) *MVRegister[V]     // returns delta
func (r *MVRegister[V]) Values() []V                   // all concurrent values
func (r *MVRegister[V]) Merge(other *MVRegister[V])
func (r *MVRegister[V]) State() Causal[*DotFun[Value[V]]]
func FromCausal[V any](state Causal[*DotFun[Value[V]]]) *MVRegister[V]
```

`Value[V]` is exported (needed for codec/serialization). `Join` is trivial.

## Write mutator

Identical structure to LWWRegister.Set, minus the timestamp:

1. Collect old dots
2. Generate new dot via `Context.Next(id)`
3. Remove old dots from local store, add new entry
4. Delta: store = {new dot → value}, context = {new dot} ∪ {old dots}

## Values query

Range over DotFun entries, collect all values. After merge of concurrent writes, multiple dots coexist — each is a distinct concurrent value.

## Conflict semantics

| Scenario | Result |
|----------|--------|
| Single writer | 1 value |
| Concurrent writes from N replicas | N values (all preserved) |
| Sequential writes (causally ordered) | 1 value (old dots removed) |
