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
	id    string
	state dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotSet]]
}

// New creates an empty AWSet for the given replica.
func New[E comparable](replicaID string) *AWSet[E] {
	return &AWSet[E]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotSet]]{
			Store:   dotcontext.NewDotMap[E, *dotcontext.DotSet](),
			Context: dotcontext.New(),
		},
	}
}

// Add inserts elem into the set and returns a delta for replication.
//
// A new dot is generated and added directly to the local state.
// We do not self-merge the delta because Next() already advances the
// causal context — merging back would see the dot in both contexts
// but not in the store, incorrectly treating it as removed.
func (s *AWSet[E]) Add(elem E) *AWSet[E] {
	d := s.state.Context.Next(s.id)

	// Update local store: add the new dot to this element's dot set.
	ds, ok := s.state.Store.Get(elem)
	if !ok {
		ds = dotcontext.NewDotSet()
		s.state.Store.Set(elem, ds)
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
func (s *AWSet[E]) Remove(elem E) *AWSet[E] {
	deltaCtx := dotcontext.New()

	if ds, ok := s.state.Store.Get(elem); ok {
		// Record all current dots in the delta's context.
		ds.Range(func(d dotcontext.Dot) bool {
			deltaCtx.Add(d)
			return true
		})
		// Remove from local state.
		s.state.Store.Delete(elem)
	}

	return &AWSet[E]{
		state: dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotSet]]{
			Store:   dotcontext.NewDotMap[E, *dotcontext.DotSet](),
			Context: deltaCtx,
		},
	}
}

// Has reports whether elem is in the set.
func (s *AWSet[E]) Has(elem E) bool {
	_, ok := s.state.Store.Get(elem)
	return ok
}

// Elements returns all elements currently in the set.
// The order is non-deterministic.
func (s *AWSet[E]) Elements() []E {
	return s.state.Store.Keys()
}

// Len returns the number of elements in the set.
func (s *AWSet[E]) Len() int {
	return s.state.Store.Len()
}

// Merge incorporates a delta or full state from another AWSet.
func (s *AWSet[E]) Merge(other *AWSet[E]) {
	s.state = dotcontext.JoinDotMap(s.state, other.state, joinDotSet)
}

// joinDotSet adapts JoinDotSet to the signature required by JoinDotMap.
func joinDotSet(a, b dotcontext.Causal[*dotcontext.DotSet]) dotcontext.Causal[*dotcontext.DotSet] {
	return dotcontext.JoinDotSet(a, b)
}
