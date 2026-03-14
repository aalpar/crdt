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

// RemovePeer deregisters a peer. Unknown peers are ignored.
func (t *PeerTracker) RemovePeer(id dotcontext.ReplicaID) {
	delete(t.peers, id)
}

// Ack merges the given context into the peer's stored context, then
// compacts. Works with any context size — from a single-dot delta
// acknowledgment to a full state exchange.
// Unknown peers are ignored — AddPeer first.
func (t *PeerTracker) Ack(id dotcontext.ReplicaID, cc *dotcontext.CausalContext) {
	stored, ok := t.peers[id]
	if !ok {
		return
	}
	stored.Merge(cc)
	stored.Compact()
}

// Pending returns the dots that the local context has but the named
// peer does not, in the same format as CausalContext.Missing().
// Returns nil for unknown peers. Also returns nil when the peer is
// fully caught up (no pending dots).
func (t *PeerTracker) Pending(id dotcontext.ReplicaID, local *dotcontext.CausalContext) map[dotcontext.ReplicaID][]dotcontext.SeqRange {
	stored, ok := t.peers[id]
	if !ok {
		return nil
	}
	return stored.Missing(local)
}

// CanGC reports whether all known peers have observed the given dot.
// Returns true if no peers are registered (vacuous truth).
func (t *PeerTracker) CanGC(d dotcontext.Dot) bool {
	for _, cc := range t.peers {
		if !cc.Has(d) {
			return false
		}
	}
	return true
}

// BlockedBy returns the peer IDs that have NOT observed the given dot.
// Returns nil if all peers have observed it (or no peers are registered).
// This is the diagnostic inverse of CanGC — it answers "who's blocking?"
// rather than "can I proceed?"
func (t *PeerTracker) BlockedBy(d dotcontext.Dot) []dotcontext.ReplicaID {
	var blockers []dotcontext.ReplicaID
	for id, cc := range t.peers {
		if !cc.Has(d) {
			blockers = append(blockers, id)
		}
	}
	return blockers
}

// PeerStatus reports a single peer's replication lag relative to the
// local causal context.
type PeerStatus struct {
	ID      dotcontext.ReplicaID
	Pending map[dotcontext.ReplicaID][]dotcontext.SeqRange // what this peer is missing
	Behind  int                                            // total missing dot count
}

// Status returns per-peer replication state relative to the local
// causal context. Every registered peer gets an entry. Peers that
// are fully caught up have Behind == 0 and Pending == nil.
func (t *PeerTracker) Status(local *dotcontext.CausalContext) []PeerStatus {
	result := make([]PeerStatus, 0, len(t.peers))
	for id, cc := range t.peers {
		pending := cc.Missing(local)
		var behind int
		for _, ranges := range pending {
			for _, r := range ranges {
				behind += int(r.Hi-r.Lo) + 1
			}
		}
		result = append(result, PeerStatus{
			ID:      id,
			Pending: pending,
			Behind:  behind,
		})
	}
	return result
}
