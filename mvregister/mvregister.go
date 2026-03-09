package mvregister

import (
	"github.com/aalpar/crdt/dotcontext"
)

// Entry wraps a user value to satisfy the dotcontext.Lattice constraint.
// For MVRegister, the join is trivial: two entries sharing the same dot
// always hold the same value, so Join simply returns the receiver.
type Entry[V any] struct {
	Val V
}

func (v Entry[V]) Join(other Entry[V]) Entry[V] { return v }

// MVRegister is a multi-value register. Concurrent writes are all
// preserved — Values() returns every concurrently-written value.
// A subsequent write from any replica supersedes all existing values.
//
// Internally it is a DotFun[Entry[V]] paired with a causal context.
type MVRegister[V any] struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotFun[Entry[V]]]
}

// New creates an empty MVRegister for the given replica.
func New[V any](replicaID dotcontext.ReplicaID) *MVRegister[V] {
	return &MVRegister[V]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotFun[Entry[V]]]{
			Store:   dotcontext.NewDotFun[Entry[V]](),
			Context: dotcontext.New(),
		},
	}
}

// Write sets the register value and returns a delta for replication.
//
// All previous values are superseded: their dots are recorded in the
// delta's context so that remote replicas remove them on merge.
func (r *MVRegister[V]) Write(v V) *MVRegister[V] {
	// Collect old dots before generating the new one.
	var oldDots []dotcontext.Dot
	r.state.Store.Range(func(d dotcontext.Dot, _ Entry[V]) bool {
		oldDots = append(oldDots, d)
		return true
	})

	// Generate new dot and write locally.
	d := r.state.Context.Next(r.id)
	for _, od := range oldDots {
		r.state.Store.Remove(od)
	}
	entry := Entry[V]{Val: v}
	r.state.Store.Set(d, entry)

	// Build delta: store has only the new entry, context has
	// the new dot plus all old dots (so receivers remove them).
	deltaStore := dotcontext.NewDotFun[Entry[V]]()
	deltaStore.Set(d, entry)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)
	for _, od := range oldDots {
		deltaCtx.Add(od)
	}

	return &MVRegister[V]{
		state: dotcontext.Causal[*dotcontext.DotFun[Entry[V]]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// State returns the MVRegister's internal Causal state for serialization.
func (r *MVRegister[V]) State() dotcontext.Causal[*dotcontext.DotFun[Entry[V]]] {
	return r.state
}

// FromCausal constructs an MVRegister from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal[V any](state dotcontext.Causal[*dotcontext.DotFun[Entry[V]]]) *MVRegister[V] {
	return &MVRegister[V]{state: state}
}

// Merge incorporates a delta or full state from another register.
func (r *MVRegister[V]) Merge(other *MVRegister[V]) {
	r.state = dotcontext.JoinDotFun(r.state, other.state)
}

// Values returns all concurrently-written values. In quiescent state
// (no unmerged concurrent writes), this returns zero or one value.
// The order is non-deterministic.
func (r *MVRegister[V]) Values() []V {
	var vals []V
	r.state.Store.Range(func(_ dotcontext.Dot, entry Entry[V]) bool {
		vals = append(vals, entry.Val)
		return true
	})
	return vals
}
