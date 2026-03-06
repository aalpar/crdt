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

// AddPeer registers a peer with its initial known state.
// A nil context is treated as empty (peer knows nothing).
// Adding an already-known peer is a no-op.
func (t *PeerTracker) AddPeer(id dotcontext.ReplicaID, cc *dotcontext.CausalContext) {
	if _, ok := t.peers[id]; ok {
		return
	}
	if cc == nil {
		cc = dotcontext.New()
	}
	t.peers[id] = cc
}

// Peers returns all registered peer IDs. Order is non-deterministic.
func (t *PeerTracker) Peers() []dotcontext.ReplicaID {
	ids := make([]dotcontext.ReplicaID, 0, len(t.peers))
	for id := range t.peers {
		ids = append(ids, id)
	}
	return ids
}
