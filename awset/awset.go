package awset

import (
	"github.com/aalpar/crdt/dotcontext"
)

// AWSet is an add-wins observed-remove set. Concurrent add and remove
// of the same element resolves in favor of add.
//
// Internally it is a DotMap[E, *DotSet] paired with a causal context.
// Each element maps to the set of dots that witness its addition.
type AWSet[E comparable] struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotSet]]
}

// New creates an empty AWSet for the given replica.
func New[E comparable](replicaID dotcontext.ReplicaID) *AWSet[E] {
	q := &AWSet[E]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotSet]]{
			Store:   dotcontext.NewDotMap[E, *dotcontext.DotSet](),
			Context: dotcontext.New(),
		},
	}
	return q
}

// Add inserts elem into the set and returns a delta for replication.
//
// A new dot is generated and added directly to the local state.
// We do not self-merge the delta because the local state is mutated
// directly (Next advances the context, and the dot is added to the
// store). Merging back is unnecessary and would violate the delta-state
// protocol: deltas are meant for remote replicas only.
func (p *AWSet[E]) Add(elem E) *AWSet[E] {
	d := p.state.Context.Next(p.id)

	// Update local store: add the new dot to this element's dot set.
	ds, ok := p.state.Store.Get(elem)
	if !ok {
		ds = dotcontext.NewDotSet()
		p.state.Store.Set(elem, ds)
	}
	ds.Add(d)

	// Build delta: element → {d}, context = {d}.
	deltaStore := dotcontext.NewDotMap[E, *dotcontext.DotSet]()
	deltaDS := dotcontext.NewDotSet()
	deltaDS.Add(d)
	deltaStore.Set(elem, deltaDS)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)

	return &AWSet[E]{
		state: dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotSet]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Remove deletes elem from the set and returns a delta for replication.
//
// The delta carries an empty store but a context containing all dots
// currently associated with elem. When merged with a concurrent add
// (which introduces a new, unobserved dot), the add's dot survives —
// hence "add-wins."
func (p *AWSet[E]) Remove(elem E) *AWSet[E] {
	deltaCtx := dotcontext.New()

	if ds, ok := p.state.Store.Get(elem); ok {
		// Record all current dots in the delta's context.
		ds.Range(func(d dotcontext.Dot) bool {
			deltaCtx.Add(d)
			return true
		})
		// Remove from local state.
		p.state.Store.Delete(elem)
	}

	return &AWSet[E]{
		state: dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotSet]]{
			Store:   dotcontext.NewDotMap[E, *dotcontext.DotSet](),
			Context: deltaCtx,
		},
	}
}

// Has reports whether elem is in the set.
func (p *AWSet[E]) Has(elem E) bool {
	_, ok := p.state.Store.Get(elem)
	return ok
}

// Elements returns all elements currently in the set.
// The order is non-deterministic.
func (p *AWSet[E]) Elements() []E {
	return p.state.Store.Keys()
}

// Len returns the number of elements in the set.
func (p *AWSet[E]) Len() int {
	return p.state.Store.Len()
}

// State returns the AWSet's internal Causal state for serialization.
func (p *AWSet[E]) State() dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotSet]] {
	return p.state
}

// FromCausal constructs an AWSet from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal[E comparable](
	state dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotSet]],
) *AWSet[E] {
	return &AWSet[E]{state: state}
}

// Merge incorporates a delta or full state from another AWSet.
func (p *AWSet[E]) Merge(other *AWSet[E]) {
	dotcontext.MergeDotMap(&p.state, other.state, dotcontext.MergeDotSetStore, dotcontext.NewDotSet)
}
