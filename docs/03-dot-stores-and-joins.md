# Dot Stores and Joins

## The Algebra Layer

We've established that every mutation gets a unique dot, and every
replica tracks which dots it has observed via a causal context. Now we
need the data structures that *hold* the dots alongside the actual data,
and the join functions that merge them.

This library provides three dot store types, each serving a different
purpose. They stack on top of each other like building blocks — simpler
stores nest inside more complex ones.

## DotSet: A Set of Dots

The simplest dot store is a set of dots. No keys, no values — just dots.

```go
type DotSet struct {
    dots map[Dot]struct{}
}
```

A DotSet answers one question: "which dots are currently active?" It's
used when the mere presence or absence of dots carries all the meaning
you need. The EWFlag CRDT, for example, is just a DotSet: if the set is
non-empty, the flag is enabled. If empty, it's disabled.

DotSet also appears as the leaf node inside the AWSet, where each
element maps to a DotSet of the dots that witness its addition.

## DotFun: Dots with Values

Sometimes each dot needs to carry a value. A counter needs to track
each replica's contribution. A register needs to track each concurrent
write's value and timestamp.

```go
type DotFun[V Lattice[V]] struct {
    entries map[Dot]V
}
```

DotFun maps each dot to a value of type `V`, where `V` must implement
the `Lattice` interface:

```go
type Lattice[T any] interface {
    Join(other T) T
}
```

This constraint exists for one specific situation: when two replicas
both have the same dot with potentially different values. In practice,
this only happens if the same dot was modified by different code paths
(which doesn't happen in this implementation — a dot is created once
and never modified). But the algebra requires it for completeness, and
the `Join` on the value type makes the merge well-defined in all cases.

The PNCounter uses `DotFun[CounterValue]` where each dot maps to a
replica's accumulated count. The LWWRegister uses
`DotFun[Timestamped[V]]` where each dot maps to a timestamped value.

## DotMap: Keys to Nested Stores

The most powerful store maps keys to nested dot stores:

```go
type DotMap[K comparable, V DotStore] struct {
    entries map[K]V
}
```

A `DotMap[string, *DotSet]` maps string keys to dot sets. This is
exactly the structure of an AWSet: each element (key) maps to the set
of dots that witness its addition. A `DotMap[string, V]` where `V` is
itself a DotMap gives you nested maps — which is how ORMap works.

The nesting is recursive. A DotMap's values can be DotSets, DotFuns,
or other DotMaps. The join function recurses accordingly.

## The DotStore Interface

All three stores implement a common interface:

```go
type DotStore interface {
    Dots() *DotSet
}
```

`Dots()` returns all active dots in the store. This is how the join
functions inspect a store's contents: they iterate the dots and check
each one against the other side's causal context to decide what
survives.

## Causal: The Unit of Replication

A dot store alone isn't enough for merging. You also need the causal
context. The `Causal` type pairs them:

```go
type Causal[T DotStore] struct {
    Store   T
    Context *CausalContext
}
```

Every merge operation — whether joining two full states or applying a
delta — works on `Causal` values. A delta is a `Causal` value. A full
state is a `Causal` value. The join function takes two `Causal` values
and produces one. This uniformity is the heart of the delta-state
framework.

## JoinDotSet: The Foundational Formula

The join of two `Causal[*DotSet]` values implements the formula from
the paper:

```
result = (s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁)
```

In words: a dot survives if **both sides have it** (intersection), or
**one side has it and the other hasn't observed it** (set minus context).

Let's walk through this with concrete dots. Alice has store `{a:1, a:2}`
and context `{a:1, a:2}`. Bob has store `{a:1, b:1}` and context
`{a:1, b:1}`.

Step by step:

| Dot | In Alice's store? | In Bob's store? | In Alice's context? | In Bob's context? | Survives? | Why? |
|-----|-------------------|-----------------|--------------------|--------------------|-----------|------|
| a:1 | Yes | Yes | Yes | Yes | **Yes** | Intersection: both have it |
| a:2 | Yes | No | Yes | No | **Yes** | s₁ \ c₂: Alice has it, Bob hasn't seen it |
| b:1 | No | Yes | No | Yes | **Yes** | s₂ \ c₁: Bob has it, Alice hasn't seen it |

Now consider what happens if Alice had *observed* dot b:1 (it's in her
context) but it's not in her store — meaning she saw it and removed it:

| Dot | In Alice's store? | In Bob's store? | In Alice's context? | In Bob's context? | Survives? | Why? |
|-----|-------------------|-----------------|--------------------|--------------------|-----------|------|
| b:1 | No | Yes | **Yes** | Yes | **No** | Not in intersection (Alice's store lacks it), and Alice's context *has* seen it, so it doesn't survive via s₂ \ c₁ |

That's the removal mechanism. Alice's context says "I've seen b:1" —
and since it's not in her store, she must have removed it. The join
respects this removal by killing the dot on Bob's side too.

Here's the implementation:

```go
func JoinDotSet(a, b Causal[*DotSet]) Causal[*DotSet] {
    result := NewDotSet()

    // Dots in a: keep if also in b, or if b hasn't seen them.
    a.Store.Range(func(d Dot) bool {
        if b.Store.Has(d) || !b.Context.Has(d) {
            result.Add(d)
        }
        return true
    })

    // Dots only in b: keep if a hasn't seen them.
    b.Store.Range(func(d Dot) bool {
        if !a.Store.Has(d) && !a.Context.Has(d) {
            result.Add(d)
        }
        return true
    })

    ctx := a.Context.Clone()
    ctx.Merge(b.Context)
    ctx.Compact()

    return Causal[*DotSet]{Store: result, Context: ctx}
}
```

The first loop handles the "intersection or unobserved by b" cases.
The second handles "only in b and unobserved by a." The merged context
is the union of both contexts (we now know everything both sides knew).

## JoinDotFun: Values Get Joined Too

`JoinDotFun` follows the same pattern as `JoinDotSet` — the survival
logic is identical. The only addition: when both sides have the same
dot, the *values* are joined via the Lattice interface.

```go
func JoinDotFun[V Lattice[V]](a, b Causal[*DotFun[V]]) Causal[*DotFun[V]] {
    result := NewDotFun[V]()

    a.Store.Range(func(d Dot, va V) bool {
        if vb, ok := b.Store.Get(d); ok {
            result.Set(d, va.Join(vb))   // same dot on both sides: join values
        } else if !b.Context.Has(d) {
            result.Set(d, va)             // unobserved by b: survives
        }
        return true
    })

    b.Store.Range(func(d Dot, vb V) bool {
        if _, ok := a.Store.Get(d); !ok && !a.Context.Has(d) {
            result.Set(d, vb)             // unobserved by a: survives
        }
        return true
    })

    ctx := a.Context.Clone()
    ctx.Merge(b.Context)
    ctx.Compact()

    return Causal[*DotFun[V]]{Store: result, Context: ctx}
}
```

In practice, `va.Join(vb)` is rarely meaningful — if both sides have
the same dot, they must have received it from the same source, so the
values are identical. But the algebra demands it, and it costs nothing.

## JoinDotMap: Recursive Nesting

`JoinDotMap` is where the algebra gets recursive. A DotMap maps keys
to nested dot stores. When both sides have the same key, their values
need to be joined — but the value type is generic (`V DotStore`), so
the join function for `V` can't be hardcoded. It's passed as a
parameter:

```go
func JoinDotMap[K comparable, V DotStore](
    a, b Causal[*DotMap[K, V]],
    joinV func(Causal[V], Causal[V]) Causal[V],
    emptyV func() V,
) Causal[*DotMap[K, V]] {
```

The two callbacks are:

- **`joinV`**: how to merge two values of type `V`. For an AWSet
  (where `V` is `*DotSet`), this is `JoinDotSet`. For a nested ORMap,
  this would be another `JoinDotMap` call — recursion.

- **`emptyV`**: constructs an empty `V`. Needed when a key exists on
  one side but not the other — the missing side is treated as "empty
  store, with my context" rather than "nonexistent."

The three cases for each key:

1. **Key in both**: recursively join the two values.
2. **Key only in a**: join a's value with an empty value (b's context
   still applies — if b observed and removed dots in this key's value,
   those removals take effect).
3. **Key only in b**: symmetric to case 2.

After joining, if the result has zero dots, the key is dropped entirely.
This is how key removal works: when all of a key's dots are subsumed by
the other side's context, the recursive join produces an empty store,
and the key disappears.

```go
    // Keys in both: recursive join.
    a.Store.Range(func(k K, va V) bool {
        if vb, ok := b.Store.Get(k); ok {
            joined := joinV(
                Causal[V]{Store: va, Context: a.Context},
                Causal[V]{Store: vb, Context: b.Context},
            )
            if joined.Store.Dots().Len() > 0 {
                result.Set(k, joined.Store)
            }
        } else {
            // Key only in a: join against empty + b's context.
            joined := joinV(
                Causal[V]{Store: va, Context: a.Context},
                Causal[V]{Store: emptyV(), Context: b.Context},
            )
            if joined.Store.Dots().Len() > 0 {
                result.Set(k, joined.Store)
            }
        }
        return true
    })
```

The "join against empty" case is subtle but critical. It's not "keep
if one side has it." It's "merge against nothing, but the other side's
context still applies." If b's context contains all of a-key's dots,
the join produces an empty result, and the key is removed. This is
exactly what should happen: b saw those dots and chose not to keep them.

## Why Callbacks Instead of Interfaces?

You might wonder: why pass `joinV` and `emptyV` as function parameters
instead of requiring `V` to have a `Join` method?

The reason is that `V` is a `DotStore`, and joining dot stores requires
a `Causal` wrapper (store + context). Go's generics can't express
"V has a Join method that takes `Causal[V]`" as a constraint in a way
that enables the recursive dispatch. The callback approach lets the
caller wire up the recursion explicitly. For AWSet, it's:

```go
func (p *AWSet[E]) Merge(other *AWSet[E]) {
    p.state = JoinDotMap(p.state, other.state, joinDotSet, NewDotSet)
}
```

Where `joinDotSet` is a thin adapter:

```go
func joinDotSet(a, b Causal[*DotSet]) Causal[*DotSet] {
    return JoinDotSet(a, b)
}
```

For a nested ORMap, the same pattern recurses arbitrarily deep.

## The Pattern Across All Three Joins

Despite the different store types, all three joins share the same
structure:

1. For each item in a's store: keep if b also has it (join values if
   applicable) or if b hasn't observed it.
2. For each item only in b's store: keep if a hasn't observed it.
3. Merge both contexts. Compact.

The "observed but absent" interpretation is the same everywhere. The
only variation is what "item" means (a dot, a dot-value pair, a
key-store pair) and how values are joined when both sides have the
same item.

This uniformity is the payoff of the dot-based design. Three join
functions, one pattern, arbitrary nesting.

[Previous: Dots and Causality](02-dots-and-causality.md) |
[Next: Building CRDTs from Composition](04-building-crdts.md)
