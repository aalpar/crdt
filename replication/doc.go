// Package replication provides delta-state anti-entropy primitives:
// delta batch encoding/decoding, per-peer acknowledgment tracking,
// and garbage collection of fully-acknowledged deltas.
//
// WriteDeltaBatch and ReadDeltaBatch handle the wire protocol for
// shipping deltas between replicas. PeerTracker records what each
// peer has observed, and GC removes deltas that all peers have
// acknowledged.
//
// Not safe for concurrent use — caller handles synchronization.
package replication
