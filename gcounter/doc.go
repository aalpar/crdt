// Package gcounter implements a grow-only counter (G-Counter),
// a delta-state CRDT that supports only increment.
//
// The implementation composes DotFun[GValue] from the dotcontext
// package. Each replica maintains a single dot mapping to its
// accumulated uint64 contribution. Increment supersedes the old dot
// with a new one carrying the updated total. The global counter value
// is the sum of all entries in the DotFun across all replicas.
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package gcounter
