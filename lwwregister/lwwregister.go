package lwwregister

import (
	"github.com/aalpar/crdt/dotcontext"
)

// Timestamped pairs a value with a wall-clock or logical timestamp.
// It implements dotcontext.Lattice: join picks the later timestamp.
// Under normal operation, JoinDotFun only calls Join when both sides
// share the same dot, meaning the values should be identical. The
// timestamp comparison provides a deterministic fallback if they differ.
type Timestamped[V any] struct {
	Value V
	Ts    int64
}

func (p Timestamped[V]) Join(other Timestamped[V]) Timestamped[V] {
	if other.Ts > p.Ts {
		return other
	}
	return p
}

// LWWRegister is a last-writer-wins register. Concurrent writes
// resolve by picking the value with the highest timestamp, breaking
// ties deterministically by replica ID.
//
// Internally it is a DotFun[Timestamped[V]] paired with a causal context.
// Each write generates a single dot; after merge, multiple dots may
// coexist (concurrent writes) and Value() resolves them.
type LWWRegister[V any] struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotFun[Timestamped[V]]]
}

// New creates an empty LWWRegister for the given replica.
func New[V any](replicaID dotcontext.ReplicaID) *LWWRegister[V] {
	q := &LWWRegister[V]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotFun[Timestamped[V]]]{
			Store:   dotcontext.NewDotFun[Timestamped[V]](),
			Context: dotcontext.New(),
		},
	}
	return q
}

// Set writes a value with the given timestamp and returns a delta
// for replication.
//
// The previous value (if any) is superseded: its dots are recorded
// in the delta's context so that remote replicas remove them on merge.
// The caller controls the timestamp — use wall-clock, logical clock,
// or any monotonic source.
func (p *LWWRegister[V]) Set(value V, timestamp int64) *LWWRegister[V] {
	// Collect old dots before generating the new one.
	var oldDots []dotcontext.Dot
	p.state.Store.Range(func(d dotcontext.Dot, _ Timestamped[V]) bool {
		oldDots = append(oldDots, d)
		return true
	})

	// Generate new dot and write locally.
	d := p.state.Context.Next(p.id)
	for _, od := range oldDots {
		p.state.Store.Remove(od)
	}
	entry := Timestamped[V]{Value: value, Ts: timestamp}
	p.state.Store.Set(d, entry)

	// Build delta: store has only the new entry, context has
	// the new dot plus all old dots (so receivers remove them).
	deltaStore := dotcontext.NewDotFun[Timestamped[V]]()
	deltaStore.Set(d, entry)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)
	for _, od := range oldDots {
		deltaCtx.Add(od)
	}

	return &LWWRegister[V]{
		state: dotcontext.Causal[*dotcontext.DotFun[Timestamped[V]]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Value returns the current value, its timestamp, and whether the
// register has been set. If multiple dots exist (concurrent writes),
// the entry with the highest timestamp wins; ties are broken by
// the dot's replica ID (lexicographic, higher wins) for determinism.
func (p *LWWRegister[V]) Value() (V, int64, bool) {
	var bestDot dotcontext.Dot
	var best Timestamped[V]
	found := false

	p.state.Store.Range(func(d dotcontext.Dot, e Timestamped[V]) bool {
		if !found || e.Ts > best.Ts ||
			(e.Ts == best.Ts && d.ID > bestDot.ID) {
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
	return best.Value, best.Ts, true
}

// State returns the LWWRegister's internal Causal state for serialization.
func (p *LWWRegister[V]) State() dotcontext.Causal[*dotcontext.DotFun[Timestamped[V]]] {
	return p.state
}

// FromCausal constructs a LWWRegister from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal[V any](state dotcontext.Causal[*dotcontext.DotFun[Timestamped[V]]]) *LWWRegister[V] {
	return &LWWRegister[V]{state: state}
}

// Merge incorporates a delta or full state from another register.
func (p *LWWRegister[V]) Merge(other *LWWRegister[V]) {
	p.state = dotcontext.JoinDotFun(p.state, other.state)
}
