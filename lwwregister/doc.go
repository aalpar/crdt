// Package lwwregister implements a last-writer-wins register, a
// delta-state CRDT where concurrent writes to the same register
// resolve by picking the value with the highest timestamp.
//
// The implementation composes DotFun[timestamped] from the dotcontext
// package. Each write generates a new dot and associates it with a
// timestamped value. When concurrent writes produce multiple surviving
// dots after merge, the read query picks the entry with the highest
// timestamp, breaking ties by replica ID for determinism.
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package lwwregister
