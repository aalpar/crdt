package gcounter

import (
	"github.com/aalpar/crdt/dotcontext"
)

// GValue holds a replica's accumulated contribution.
// It implements dotcontext.Lattice. Under normal operation, JoinDotFun
// only calls Join when both sides share the same dot, so values are
// identical and either is correct. Join returns the receiver unchanged.
//
// NOTE: pncounter.Counter follows the same find-own-dot/replace/build-delta
// pattern with int64 instead of uint64. Changes to the Increment logic
// here should be mirrored there (and vice versa).
type GValue struct {
	N uint64
}

func (g GValue) Join(other GValue) GValue {
	return g
}

// Counter is a grow-only counter. Each replica tracks its accumulated
// contribution as a single dot in a DotFun. The global value is the
// sum of all entries.
type Counter struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotFun[GValue]]
}

// New creates a counter at zero for the given replica.
func New(replicaID dotcontext.ReplicaID) *Counter {
	return &Counter{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotFun[GValue]]{
			Store:   dotcontext.NewDotFun[GValue](),
			Context: dotcontext.New(),
		},
	}
}

// Increment adds n to the counter and returns a delta for replication.
func (g *Counter) Increment(n uint64) *Counter {
	// Find this replica's current dot and value.
	var oldDot dotcontext.Dot
	var oldValue uint64
	hasOld := false

	g.state.Store.Range(func(d dotcontext.Dot, v GValue) bool {
		if d.ID == g.id {
			oldDot = d
			oldValue = v.N
			hasOld = true
			return false
		}
		return true
	})

	// Generate new dot and update local state.
	d := g.state.Context.Next(g.id)
	if hasOld {
		g.state.Store.Remove(oldDot)
	}
	newVal := GValue{N: oldValue + n}
	g.state.Store.Set(d, newVal)

	// Build delta: new entry + context covering old dot.
	deltaStore := dotcontext.NewDotFun[GValue]()
	deltaStore.Set(d, newVal)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)
	if hasOld {
		deltaCtx.Add(oldDot)
	}

	return &Counter{
		state: dotcontext.Causal[*dotcontext.DotFun[GValue]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Value returns the current counter total (sum of all replica contributions).
func (g *Counter) Value() uint64 {
	var total uint64
	g.state.Store.Range(func(_ dotcontext.Dot, v GValue) bool {
		total += v.N
		return true
	})
	return total
}

// State returns the Counter's internal Causal state for serialization.
func (g *Counter) State() dotcontext.Causal[*dotcontext.DotFun[GValue]] {
	return g.state
}

// FromCausal constructs a Counter from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal(state dotcontext.Causal[*dotcontext.DotFun[GValue]]) *Counter {
	return &Counter{state: state}
}

// Merge incorporates a delta or full state from another counter.
func (g *Counter) Merge(other *Counter) {
	dotcontext.MergeDotFun(&g.state, other.state)
}
