package lwweset

// LWWESet is a last-writer-wins element set. Each element carries
// an add timestamp and a remove timestamp; an element is present
// when its add timestamp is greater than or equal to its remove
// timestamp (add-wins on equal timestamps).
//
// The timestamp source is controlled by the caller. Use a monotonic
// source (wall clock, Lamport clock, etc.) to ensure meaningful
// ordering. Identical timestamps on concurrent add and remove of the
// same element resolve in favor of add.
//
// Like GSet, LWWESet requires no causal dot stores — timestamps
// provide a total order that makes per-replica dot tracking redundant.
type LWWESet[E comparable] struct {
	added   map[E]int64
	removed map[E]int64
}

// New creates an empty LWWESet.
func New[E comparable]() *LWWESet[E] {
	return &LWWESet[E]{
		added:   make(map[E]int64),
		removed: make(map[E]int64),
	}
}

// Add inserts elem with the given timestamp and returns a delta for
// replication. If a higher add timestamp for elem already exists
// locally, local state is unchanged but the delta still carries ts.
func (s *LWWESet[E]) Add(elem E, ts int64) *LWWESet[E] {
	if ts > s.added[elem] {
		s.added[elem] = ts
	}
	return &LWWESet[E]{
		added:   map[E]int64{elem: ts},
		removed: make(map[E]int64),
	}
}

// Remove marks elem absent with the given timestamp and returns a
// delta for replication. An element is absent when its remove
// timestamp is strictly greater than its add timestamp.
func (s *LWWESet[E]) Remove(elem E, ts int64) *LWWESet[E] {
	if ts > s.removed[elem] {
		s.removed[elem] = ts
	}
	return &LWWESet[E]{
		added:   make(map[E]int64),
		removed: map[E]int64{elem: ts},
	}
}

// Has reports whether elem is currently in the set.
// An element is present iff it has been added (add timestamp > 0)
// and its add timestamp is greater than or equal to its remove timestamp.
func (s *LWWESet[E]) Has(elem E) bool {
	a, ok := s.added[elem]
	return ok && a >= s.removed[elem]
}

// Elements returns all elements currently in the set.
// The order is non-deterministic.
func (s *LWWESet[E]) Elements() []E {
	elems := make([]E, 0)
	for e, a := range s.added {
		if a >= s.removed[e] {
			elems = append(elems, e)
		}
	}
	return elems
}

// Len returns the number of elements currently in the set.
func (s *LWWESet[E]) Len() int {
	n := 0
	for e, a := range s.added {
		if a >= s.removed[e] {
			n++
		}
	}
	return n
}

// Merge incorporates a delta or full state from another LWWESet.
// For each element, Merge takes the maximum of the add timestamps
// and the maximum of the remove timestamps.
func (s *LWWESet[E]) Merge(other *LWWESet[E]) {
	for e, ts := range other.added {
		if ts > s.added[e] {
			s.added[e] = ts
		}
	}
	for e, ts := range other.removed {
		if ts > s.removed[e] {
			s.removed[e] = ts
		}
	}
}
