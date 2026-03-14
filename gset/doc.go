// Package gset implements a grow-only set (GSet), a delta-state CRDT
// where elements can only be added, never removed. Merge is set union.
//
// GSet does not use causal dot stores — there are no concurrent
// add/remove conflicts to resolve. Merge correctness follows directly
// from set union being commutative, associative, and idempotent.
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package gset
