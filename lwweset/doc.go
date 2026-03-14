// Package lwweset implements a last-writer-wins element set (LWWESet),
// a delta-state CRDT where each element independently carries an add
// timestamp and a remove timestamp. An element is present when its
// add timestamp is greater than or equal to its remove timestamp.
//
// Unlike AWSet and RWSet, conflict resolution does not use causal dot
// stores — timestamps provide a total order that makes causal tracking
// redundant. The timestamp source (wall clock, logical clock, Lamport
// clock) is supplied by the caller.
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package lwweset
