# Dots and Causality

## The Tracking Problem

When two replicas merge, the join function needs to answer a fundamental
question for every piece of data: *should this survive the merge, or not?*

Consider Alice and Bob, each with a set containing "bread." Alice removes
"bread." Bob hasn't seen the removal yet — his set still has "bread."
They merge. What should happen?

If the merge function just takes the union of both sets, "bread" comes
back. Alice's removal is lost. If it takes the intersection, Bob's
un-removed "bread" disappears — but so would anything Bob added that
Alice hasn't seen yet.

Neither union nor intersection is correct. The merge needs to distinguish
between two very different situations:

1. Bob has "bread" because he **hasn't seen** Alice's removal yet.
2. Bob has "bread" because he **re-added** it after Alice's removal.

In case 1, "bread" should disappear. In case 2, it should survive. But
from the perspective of the raw data — both sets contain "bread" — the
two cases look identical. The merge function needs more information.

## Dots: Giving Every Mutation an Identity

The solution is to tag every mutation with a unique identifier. In this
library, that identifier is called a **dot**.

A dot is a pair: (replica ID, sequence number). Alice's first mutation
generates dot (alice, 1). Her second generates (alice, 2). Bob's first
generates (bob, 1). And so on.

```
type Dot struct {
    ID  ReplicaID  // which replica
    Seq uint64     // monotonically increasing per replica
}
```

Every operation that *introduces* new state — Add, Set, Increment,
Enable — generates a new dot. Operations that only *remove* state
(Remove, Disable) do not generate dots; they record existing dots in
the delta's causal context to signal "I saw these and removed them."

The dot is the *event identity*. It tells you: this specific mutation
happened on this specific replica at this specific point in its history.

Now the merge function has what it needs. Instead of "the set contains
bread," we have "the set contains bread *witnessed by dot (bob, 3)*."
And instead of just knowing what's in the set, each replica also tracks
which dots it has *observed* — even dots whose effects have been undone.

## The Causal Context: What Have I Seen?

Each replica maintains a **causal context**: the set of all dots it has
ever observed. When Alice removes "bread," she records all of bread's
dots in her causal context. The data is gone from her set, but her
context remembers: "I saw (bob, 3), and I chose to remove it."

Now the merge can distinguish our two cases:

- Bob has "bread" witnessed by (bob, 3), and Alice's context contains
  (bob, 3) → Alice has **seen and removed** it. It should not survive.
- Bob has "bread" witnessed by (bob, 5), and Alice's context does **not**
  contain (bob, 5) → This is a **new addition** Alice hasn't seen. It
  should survive.

This is the core insight of the entire framework: **the causal context
makes absence meaningful**. A dot that's missing from the store but
present in the context means "I saw this and it was removed." A dot
that's missing from both store and context means "I haven't seen this
yet."

## Compressing the Context: Version Vectors

If every replica tracks every dot it has seen, the causal context could
grow without bound. But there's a pattern to exploit: for any given
replica, its dots are usually contiguous from 1. If Alice has seen dots
(bob, 1), (bob, 2), (bob, 3), (bob, 4), we can compress that to
"bob → 4" — a single number per replica. This is a **version vector**.

```
type CausalContext struct {
    vv       map[ReplicaID]uint64       // version vector
    outliers map[Dot]struct{}           // dots above the frontier
}
```

The version vector says "I've seen all dots from this replica up to
sequence number N." For the common case — sequential operations,
no gaps — this is perfect compression: O(replicas) instead of O(events).

But gaps can happen. If a delta arrives out of order — say you receive
(bob, 5) before (bob, 4) — the version vector can't represent "I've seen
1, 2, 3, and 5 but not 4." That's what **outliers** are for: dots above
the contiguous frontier that arrived out of sequence.

When the gap fills in (dot 4 arrives), `Compact()` promotes the outliers
back into the version vector:

```go
func (p *CausalContext) Compact() {
    changed := true
    for changed {
        changed = false
        for d := range p.outliers {
            if d.Seq <= p.vv[d.ID] {
                // Redundant: already covered.
                delete(p.outliers, d)
                changed = true
            } else if d.Seq == p.vv[d.ID]+1 {
                // Contiguous: promote to version vector.
                p.vv[d.ID] = d.Seq
                delete(p.outliers, d)
                changed = true
            }
        }
    }
}
```

After compaction, if the outliers are empty, the version vector alone
captures the complete state of what this replica has observed. In steady
state — when deltas arrive roughly in order — the outlier set stays
small or empty.

## Generating Dots

Every replica generates its own dots through a single function:

```go
func (p *CausalContext) Next(id ReplicaID) Dot {
    seq := p.vv[id] + 1
    p.vv[id] = seq
    return Dot{ID: id, Seq: seq}
}
```

This is the sole dot allocator. Because the sequence number comes from
the version vector, and `Next` immediately advances it, dots are
guaranteed unique — no two operations on the same replica will ever
produce the same dot, and dots from different replicas have different
IDs.

Notice that `Next` both *creates* the dot and *records it as observed*
in one step (by advancing the version vector). This is important: after
calling `Next`, the causal context already contains the new dot. There's
no separate "add to context" step needed for locally generated events.

## The Has Question

The most important operation on a causal context is `Has`: has this
replica observed a given dot?

```go
func (p *CausalContext) Has(d Dot) bool {
    if d.Seq <= p.vv[d.ID] {
        return true
    }
    _, ok := p.outliers[d]
    return ok
}
```

First check the version vector: if the dot's sequence number is at or
below the frontier, we've seen it. Otherwise, check the outliers. This
is what the join functions call repeatedly to decide what survives a
merge.

## Missing: What Do You Have That I Don't?

Replication needs the inverse question: given two contexts, what dots
does the remote have that I'm missing?

```go
missing := local.Missing(remote)
// Returns map[ReplicaID][]SeqRange — compressed gap ranges per replica
```

This is how the anti-entropy protocol works: compare contexts, compute
the gap, fetch only the deltas that fill it. No redundant data shipped.

The result is compressed into `SeqRange` slices (inclusive `[Lo, Hi]`
ranges), which compose directly with the delta store's `Fetch` method.
The pipeline is:

```
local.Missing(remote) → store.Fetch(missing) → encode → ship → decode → merge
```

## The Separation of Store and Context

The interplay between the dot store (what data is currently live) and
the causal context (what dots have been observed) is the entire mechanism.
A dot can be in three states relative to a replica:

| In Store? | In Context? | Meaning |
|-----------|-------------|---------|
| Yes | Yes | Active: this data is live |
| No | Yes | Removed: we saw it and removed it |
| No | No | Unknown: we haven't seen this dot yet |
| Yes | No | (Invalid — can't have data we haven't seen) |

The fourth state is impossible by construction: generating a dot via
`Next()` always adds it to the context immediately.

This three-state interpretation is what drives the join formulas. A dot
in someone's store survives the merge if the other side either also has
it (intersection) or hasn't observed it (the other's context doesn't
contain it). A dot that one side has observed but removed — present in
context, absent from store — will kill the corresponding dot on the
other side during merge.

[Previous: What CRDTs Are](01-what-are-crdts.md) |
[Next: Dot Stores and Joins](03-dot-stores-and-joins.md)
