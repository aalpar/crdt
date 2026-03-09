# What CRDTs Are and Why They Exist

## The Problem: Two People Edit the Same Thing

Imagine two users, Alice and Bob, each with their own copy of a shopping list.
They're both offline — on a plane, in a subway, wherever. Alice adds "milk."
Bob adds "eggs." Later, they reconnect. What should the merged list contain?

Obviously: milk and eggs. Both additions are valid. A sane system keeps both.

Now make it harder. They both start with a list containing "bread." Alice
removes "bread." Bob, who hasn't seen Alice's removal, adds "butter." When they
sync, what happens to "bread"?

This is where things get interesting — and where most naive approaches break.

## Why Not Just Use a Database?

The traditional answer to "two people editing the same thing" is: don't let
them. Put a single database in the middle. Every write goes through it. The
database decides the order. Problem solved.

But this requires that everyone can reach the database at all times. If the
network partitions — or if the database is down, or if you're building a
peer-to-peer system, or a local-first application — you're stuck. Users
can't work until the central authority is reachable again.

What if each replica could work independently and then merge later?

## The Merge Problem

If replicas can diverge, you need a way to merge them back together. This
is the core problem. And the merge function can't be arbitrary — it needs
to have specific mathematical properties, or things go wrong in subtle ways.

Consider three replicas: A, B, and C. Replica A sends its state to B, and
separately to C. Then B and C merge with each other. The result should be
the same regardless of the order these merges happen. If A merges with B
first and then with C, or if A merges with C first and then with B — the
final state must be identical.

This gives us three requirements for the merge function:

1. **Commutative**: merge(a, b) = merge(b, a). The order of the two
   arguments doesn't matter.
2. **Associative**: merge(merge(a, b), c) = merge(a, merge(b, c)). The
   grouping doesn't matter.
3. **Idempotent**: merge(a, a) = a. Merging something with itself is a
   no-op. This matters because in a real network, you might receive the
   same update twice.

A function with all three properties is called a *join* on a
*semilattice*. That's the mathematical name, but the intuition is simpler:
it's a merge where values can only "go up" (accumulate information) and
never "go down" (lose information). There's always a well-defined result,
no matter how many times or in what order you merge.

This is what a CRDT is: a **Conflict-free Replicated Data Type**. A data
structure with a merge function that forms a semilattice. No coordination
needed. No central authority. Replicas merge freely and always converge.

## Three Flavors of CRDT

CRDTs come in three varieties, distinguished by *what* gets replicated:

**State-based (CvRDTs)** send the entire state. Replica A sends its whole
shopping list to B. B merges it with its own list. Simple, but expensive:
if the state is large, you're shipping a lot of data even if only one item
changed.

**Operation-based (CmRDTs)** send the operations. Instead of sending the
whole list, A sends "I added milk." This is compact, but it requires
reliable, exactly-once delivery — if an operation gets lost or delivered
twice, replicas diverge.

**Delta-state CRDTs** are the sweet spot. Instead of shipping the entire
state, you ship just the *part that changed* — the delta. A delta is itself
a valid state (it follows the same semilattice rules), so it can be merged
with the same join function. But unlike operation-based CRDTs, deltas are
idempotent: if you receive the same delta twice, merging it again has no
effect. You get the bandwidth efficiency of operations with the robustness
of state-based merging.

This project implements delta-state CRDTs, following the framework described
by Almeida, Shoker, and Baquero in their 2018 paper, "Delta State
Replicated Data Types."

## What Makes Delta-State Different

In a state-based CRDT, every mutator (add, remove, increment) produces a
new complete state. You merge entire states.

In a delta-state CRDT, every mutator produces a *delta* — a small fragment
of state that captures just the mutation. The delta follows the same
algebraic rules as a full state (it's a valid `Causal` value, as we'll see),
so the same join function works for both:

- Merging a delta into a remote state
- Merging two full states
- Merging two deltas together

This uniformity is the key design insight. There's one merge function, and
it works on everything.

## What's Next

The machinery that makes this work rests on a deceptively simple idea:
*giving every mutation a unique identity*. This identity — called a *dot* —
is how the system tracks what each replica has seen and what it hasn't.
Understanding dots is the key to understanding everything else in this
library.

[Next: Dots and Causality](02-dots-and-causality.md)
