package dotcontext

import "sort"

// DeltaStore is an in-memory buffer of deltas indexed by the dot that
// created them. It supports range queries composable with Missing().
//
// A per-replica secondary index of sorted sequence numbers accelerates
// Fetch from O(|store| × |ranges|) to O(Σ_r |ranges_r|×log(|dots_per_r|) + |hits|).
type DeltaStore[T any] struct {
	deltas    map[Dot]T
	byReplica map[ReplicaID][]uint64 // sorted seq values per replica
}

// NewDeltaStore returns an empty DeltaStore.
func NewDeltaStore[T any]() *DeltaStore[T] {
	return &DeltaStore[T]{
		deltas:    make(map[Dot]T),
		byReplica: make(map[ReplicaID][]uint64),
	}
}

// Len returns the number of stored deltas.
func (s *DeltaStore[T]) Len() int {
	return len(s.deltas)
}

// Add stores a delta indexed by the dot that created it.
// Adding the same dot twice updates the value and is a no-op for the index.
func (s *DeltaStore[T]) Add(d Dot, delta T) {
	s.deltas[d] = delta
	seqs := s.byReplica[d.ID]
	i := sort.Search(len(seqs), func(i int) bool { return seqs[i] >= d.Seq })
	if i == len(seqs) {
		// Common case: sequential dots append to the tail.
		s.byReplica[d.ID] = append(seqs, d.Seq)
	} else if seqs[i] != d.Seq {
		// Out-of-order insertion: shift right and insert.
		seqs = append(seqs, 0)
		copy(seqs[i+1:], seqs[i:])
		seqs[i] = d.Seq
		s.byReplica[d.ID] = seqs
	}
	// seqs[i] == d.Seq: already in index, value updated above.
}

// Get retrieves a single delta by its dot.
func (s *DeltaStore[T]) Get(d Dot) (T, bool) {
	v, ok := s.deltas[d]
	return v, ok
}

// Remove deletes a delta by its dot.
func (s *DeltaStore[T]) Remove(d Dot) {
	delete(s.deltas, d)
	seqs := s.byReplica[d.ID]
	i := sort.Search(len(seqs), func(i int) bool { return seqs[i] >= d.Seq })
	if i < len(seqs) && seqs[i] == d.Seq {
		seqs = append(seqs[:i], seqs[i+1:]...)
		if len(seqs) == 0 {
			delete(s.byReplica, d.ID)
		} else {
			s.byReplica[d.ID] = seqs
		}
	}
}

// Dots returns a snapshot of all stored dots.
func (s *DeltaStore[T]) Dots() []Dot {
	dots := make([]Dot, 0, len(s.deltas))
	for d := range s.deltas {
		dots = append(dots, d)
	}
	return dots
}

// Fetch returns all stored deltas whose dots fall within the given ranges.
// The input format matches Missing()'s return type for direct composability:
//
//	store.Fetch(remote.Missing(local))
//
// remote.Missing(local) returns the dots local has observed but remote
// has not — i.e., the deltas this node should send.
func (s *DeltaStore[T]) Fetch(missing map[ReplicaID][]SeqRange) map[Dot]T {
	if len(missing) == 0 {
		return nil
	}
	var result map[Dot]T
	for id, ranges := range missing {
		seqs, ok := s.byReplica[id]
		if !ok {
			continue
		}
		for _, r := range ranges {
			lo := sort.Search(len(seqs), func(i int) bool { return seqs[i] >= r.Lo })
			for j := lo; j < len(seqs) && seqs[j] <= r.Hi; j++ {
				if result == nil {
					result = make(map[Dot]T)
				}
				d := Dot{ID: id, Seq: seqs[j]}
				result[d] = s.deltas[d]
			}
		}
	}
	return result
}
