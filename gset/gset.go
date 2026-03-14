package gset

// GSet is a grow-only set. Elements can only be added, never removed.
//
// Unlike other CRDTs in this library, GSet requires no causal context:
// there are no remove operations, so no conflict between add and remove
// is possible. The semilattice join is plain set union.
type GSet[E comparable] struct {
	elems map[E]struct{}
}

// New creates an empty GSet.
func New[E comparable]() *GSet[E] {
	return &GSet[E]{elems: make(map[E]struct{})}
}

// Add inserts elem into the set and returns a delta for replication.
// If elem is already present, local state is unchanged and the returned
// delta is a singleton set containing elem.
func (s *GSet[E]) Add(elem E) *GSet[E] {
	s.elems[elem] = struct{}{}
	return &GSet[E]{elems: map[E]struct{}{elem: {}}}
}

// Has reports whether elem is in the set.
func (s *GSet[E]) Has(elem E) bool {
	_, ok := s.elems[elem]
	return ok
}

// Elements returns all elements currently in the set.
// The order is non-deterministic.
func (s *GSet[E]) Elements() []E {
	elems := make([]E, 0, len(s.elems))
	for e := range s.elems {
		elems = append(elems, e)
	}
	return elems
}

// Len returns the number of elements in the set.
func (s *GSet[E]) Len() int {
	return len(s.elems)
}

// Merge incorporates a delta or full state from another GSet.
// Merge is set union: every element in other is added to s.
func (s *GSet[E]) Merge(other *GSet[E]) {
	for e := range other.elems {
		s.elems[e] = struct{}{}
	}
}
