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
	q := &DotFun[V]{entries: make(map[Dot]V)}
	return q
}

// newDotFunSized returns a DotFun with a pre-sized map.
func newDotFunSized[V Lattice[V]](n int) *DotFun[V] {
	return &DotFun[V]{entries: make(map[Dot]V, n)}
}

// Set associates a value with a dot.
func (p *DotFun[V]) Set(d Dot, v V) {
	p.entries[d] = v
}

// Get returns the value for a dot and whether it exists.
func (p *DotFun[V]) Get(d Dot) (V, bool) {
	v, ok := p.entries[d]
	return v, ok
}

// Remove deletes the mapping for a dot.
func (p *DotFun[V]) Remove(d Dot) {
	delete(p.entries, d)
}

// Len returns the number of entries.
func (p *DotFun[V]) Len() int {
	return len(p.entries)
}

// Range calls fn for each dot-value pair. If fn returns false, iteration stops.
func (p *DotFun[V]) Range(fn func(Dot, V) bool) {
	for d, v := range p.entries {
		if !fn(d, v) {
			return
		}
	}
}

// HasDots reports whether the DotFun has any entries.
func (p *DotFun[V]) HasDots() bool {
	return len(p.entries) > 0
}

// Dots returns the set of dots in this DotFun (DotStore implementation).
func (p *DotFun[V]) Dots() *DotSet {
	ds := NewDotSet()
	for d := range p.entries {
		ds.Add(d)
	}
	return ds
}

// CloneStore returns a copy as a DotStore (interface implementation).
func (p *DotFun[V]) CloneStore() DotStore { return p.Clone() }

// Clone copies the map entries by value. For value-typed V this is a
// deep copy; for pointer-typed V the values are shared.
func (p *DotFun[V]) Clone() *DotFun[V] {
	nf := &DotFun[V]{entries: make(map[Dot]V, len(p.entries))}
	for d, v := range p.entries {
		nf.entries[d] = v
	}
	return nf
}
