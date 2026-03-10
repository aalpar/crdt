// Package rga implements a replicated growable array, a delta-state
// CRDT for ordered sequences where elements are immutable once
// inserted.
//
// The implementation composes DotFun[Node[E]] from the dotcontext
// package. Each element is identified by a unique dot and stores a
// value plus a reference to its predecessor. Concurrent inserts at
// the same position are ordered deterministically by dot comparison
// (higher sequence number first, then higher replica ID).
//
// Deletion tombstones the element rather than removing it, preserving
// the tree structure that anchors ordering. Tombstoned elements are
// invisible in query results but persist in the store.
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package rga
