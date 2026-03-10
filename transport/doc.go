// Package transport provides network transport for delta-state CRDT
// replication. Two layers: Conn wraps a net.Conn with length-prefixed,
// type-tagged message framing; Transport manages a pool of Conns and
// dispatches incoming messages to a Handler.
//
// The transport is agnostic to payload contents — it moves tagged byte
// slices. The caller owns serialization (via dotcontext codecs) and
// replication logic (via PeerTracker, DeltaStore, etc.).
//
// Conn is safe for concurrent sends from multiple goroutines.
// Receive must be called from a single goroutine (the read loop).
// Transport is safe for concurrent use.
package transport
