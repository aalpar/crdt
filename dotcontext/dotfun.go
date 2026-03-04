package dotcontext

// Lattice is a constraint for types that form a join-semilattice.
// Join must be idempotent, commutative, and associative.
type Lattice[T any] interface {
	Join(other T) T
}

// DotFun maps dots to lattice values: (I × N) → V.
// Used by lwwregister and counters where each dot carries a mergeable value.
type DotFun[V Lattice[V]] struct {
	entries map[Dot]V
}

// NewDotFun returns an empty DotFun.
func NewDotFun[V Lattice[V]]() *DotFun[V] {
	return &DotFun[V]{entries: make(map[Dot]V)}
}

// Set associates a value with a dot.
func (f *DotFun[V]) Set(d Dot, v V) {
	f.entries[d] = v
}

// Get returns the value for a dot and whether it exists.
func (f *DotFun[V]) Get(d Dot) (V, bool) {
	v, ok := f.entries[d]
	return v, ok
}

// Remove deletes the mapping for a dot.
func (f *DotFun[V]) Remove(d Dot) {
	delete(f.entries, d)
}

// Len returns the number of entries.
func (f *DotFun[V]) Len() int {
	return len(f.entries)
}

// Range calls fn for each dot-value pair. If fn returns false, iteration stops.
func (f *DotFun[V]) Range(fn func(Dot, V) bool) {
	for d, v := range f.entries {
		if !fn(d, v) {
			return
		}
	}
}

// Dots returns the set of dots in this DotFun (DotStore implementation).
func (f *DotFun[V]) Dots() *DotSet {
	ds := NewDotSet()
	for d := range f.entries {
		ds.Add(d)
	}
	return ds
}

// Clone returns a deep copy.
func (f *DotFun[V]) Clone() *DotFun[V] {
	nf := &DotFun[V]{entries: make(map[Dot]V, len(f.entries))}
	for d, v := range f.entries {
		nf.entries[d] = v
	}
	return nf
}
