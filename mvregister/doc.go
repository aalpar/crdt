// Package mvregister implements a multi-value register, a delta-state
// CRDT where concurrent writes are all preserved rather than resolved
// by a total order.
//
// The implementation composes DotFun[Value[V]] from the dotcontext
// package. Each write generates a new dot and associates it with the
// value. When concurrent writes produce multiple surviving dots after
// merge, the read query returns all coexisting values. A subsequent
// write from any replica replaces them with a single new value.
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package mvregister
