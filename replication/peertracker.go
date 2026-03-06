package replication

import "github.com/aalpar/crdt/dotcontext"

// PeerTracker tracks per-peer acknowledged state for delta GC.
// It stores a CausalContext per peer representing what that peer
// has observed. Not thread-safe — caller handles synchronization.
type PeerTracker struct {
	peers map[dotcontext.ReplicaID]*dotcontext.CausalContext
}

// NewPeerTracker returns an empty PeerTracker with no registered peers.
func NewPeerTracker() *PeerTracker {
	return &PeerTracker{peers: make(map[dotcontext.ReplicaID]*dotcontext.CausalContext)}
}

// Len returns the number of registered peers.
func (t *PeerTracker) Len() int {
	return len(t.peers)
}
