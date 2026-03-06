package dotcontext

// DeltaStore is an in-memory buffer of deltas indexed by the dot that
// created them. It supports range queries composable with Missing().
type DeltaStore[T any] struct {
	deltas map[Dot]T
}

// NewDeltaStore returns an empty DeltaStore.
func NewDeltaStore[T any]() *DeltaStore[T] {
	return &DeltaStore[T]{deltas: make(map[Dot]T)}
}

// Len returns the number of stored deltas.
func (s *DeltaStore[T]) Len() int {
	return len(s.deltas)
}

// Add stores a delta indexed by the dot that created it.
func (s *DeltaStore[T]) Add(d Dot, delta T) {
	s.deltas[d] = delta
}

// Get retrieves a single delta by its dot.
func (s *DeltaStore[T]) Get(d Dot) (T, bool) {
	v, ok := s.deltas[d]
	return v, ok
}
