package lwwregister

import (
	"github.com/aalpar/crdt/dotcontext"
)

// timestamped pairs a value with a wall-clock or logical timestamp.
// It implements dotcontext.Lattice: join picks the later timestamp.
// In practice, JoinDotFun only calls Join when both sides have the
// same dot — which means the same event, so values are identical and
// the choice is trivially correct.
type timestamped[V any] struct {
	value V
	ts    int64
}

func (t timestamped[V]) Join(other timestamped[V]) timestamped[V] {
	if other.ts > t.ts {
		return other
	}
	return t
}

// LWWRegister is a last-writer-wins register. Concurrent writes
// resolve by picking the value with the highest timestamp, breaking
// ties deterministically by replica ID.
//
// Internally it is a DotFun[timestamped[V]] paired with a causal context.
// Each write generates a single dot; after merge, multiple dots may
// coexist (concurrent writes) and Value() resolves them.
type LWWRegister[V any] struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotFun[timestamped[V]]]
}

// New creates an empty LWWRegister for the given replica.
func New[V any](replicaID dotcontext.ReplicaID) *LWWRegister[V] {
	return &LWWRegister[V]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotFun[timestamped[V]]]{
			Store:   dotcontext.NewDotFun[timestamped[V]](),
			Context: dotcontext.New(),
		},
	}
}

// Set writes a value with the given timestamp and returns a delta
// for replication.
//
// The previous value (if any) is superseded: its dots are recorded
// in the delta's context so that remote replicas remove them on merge.
// The caller controls the timestamp — use wall-clock, logical clock,
// or any monotonic source.
func (r *LWWRegister[V]) Set(value V, timestamp int64) *LWWRegister[V] {
	// Collect old dots before generating the new one.
	var oldDots []dotcontext.Dot
	r.state.Store.Range(func(d dotcontext.Dot, _ timestamped[V]) bool {
		oldDots = append(oldDots, d)
		return true
	})

	// Generate new dot and write locally.
	d := r.state.Context.Next(r.id)
	for _, od := range oldDots {
		r.state.Store.Remove(od)
	}
	entry := timestamped[V]{value: value, ts: timestamp}
	r.state.Store.Set(d, entry)

	// Build delta: store has only the new entry, context has
	// the new dot plus all old dots (so receivers remove them).
	deltaStore := dotcontext.NewDotFun[timestamped[V]]()
	deltaStore.Set(d, entry)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)
	for _, od := range oldDots {
		deltaCtx.Add(od)
	}

	return &LWWRegister[V]{
		state: dotcontext.Causal[*dotcontext.DotFun[timestamped[V]]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Value returns the current value, its timestamp, and whether the
// register has been set. If multiple dots exist (concurrent writes),
// the entry with the highest timestamp wins; ties are broken by
// replica ID (lexicographic, higher wins) for determinism.
func (r *LWWRegister[V]) Value() (V, int64, bool) {
	var bestDot dotcontext.Dot
	var best timestamped[V]
	found := false

	r.state.Store.Range(func(d dotcontext.Dot, e timestamped[V]) bool {
		if !found || e.ts > best.ts ||
			(e.ts == best.ts && d.ID > bestDot.ID) {
			bestDot = d
			best = e
			found = true
		}
		return true
	})

	if !found {
		var zero V
		return zero, 0, false
	}
	return best.value, best.ts, true
}

// Merge incorporates a delta or full state from another register.
func (r *LWWRegister[V]) Merge(other *LWWRegister[V]) {
	r.state = dotcontext.JoinDotFun(r.state, other.state)
}
