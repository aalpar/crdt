// Package rwset implements a remove-wins observed-remove set (RWSet),
// a delta-state CRDT where concurrent add and remove of the same
// element resolves in favor of remove.
//
// The implementation composes DotMap[E, *DotFun[Presence]] from the
// dotcontext package. Each element maps to a DotFun tracking whether
// its last operation was an add or remove. Both Add and Remove generate
// dots (unlike AWSet where only Add generates dots). An element is
// present iff all its surviving dots have Active=true — a single
// tombstone (Active=false) dot means the element is considered removed.
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package rwset
