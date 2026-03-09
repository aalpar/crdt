package mvregister

import (
	"github.com/aalpar/crdt/dotcontext"
)

// Value wraps a user value to satisfy the dotcontext.Lattice constraint.
// For MVRegister, the join is trivial: two entries sharing the same dot
// always hold the same value, so Join simply returns the receiver.
type Value[V any] struct {
	V V
}

func (v Value[V]) Join(other Value[V]) Value[V] { return v }

// MVRegister is a multi-value register. Concurrent writes are all
// preserved — Values() returns every concurrently-written value.
// A subsequent write from any replica supersedes all existing values.
//
// Internally it is a DotFun[Value[V]] paired with a causal context.
type MVRegister[V any] struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotFun[Value[V]]]
}

// New creates an empty MVRegister for the given replica.
func New[V any](replicaID dotcontext.ReplicaID) *MVRegister[V] {
	return &MVRegister[V]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotFun[Value[V]]]{
			Store:   dotcontext.NewDotFun[Value[V]](),
			Context: dotcontext.New(),
		},
	}
}

// Values returns all concurrently-written values. In quiescent state
// (no unmerged concurrent writes), this returns zero or one value.
// The order is non-deterministic.
func (r *MVRegister[V]) Values() []V {
	var vals []V
	r.state.Store.Range(func(_ dotcontext.Dot, entry Value[V]) bool {
		vals = append(vals, entry.V)
		return true
	})
	return vals
}
