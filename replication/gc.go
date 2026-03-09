package replication

import "github.com/aalpar/crdt/dotcontext"

// GC removes deltas from the store that all tracked peers have
// acknowledged. Returns the number of deltas removed.
// Not safe for concurrent use — caller handles synchronization.
func GC[T any](store *dotcontext.DeltaStore[T], tracker *PeerTracker) int {
	var removed int
	for _, d := range store.Dots() {
		if tracker.CanGC(d) {
			store.Remove(d)
			removed++
		}
	}
	return removed
}
