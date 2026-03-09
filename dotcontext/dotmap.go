package dotcontext

// DotMap maps keys to nested dot stores: K → V where V is a DotStore.
// Used by awset (DotMap[E, *DotSet]) and ORMap.
type DotMap[K comparable, V DotStore] struct {
	entries map[K]V
}

// NewDotMap returns an empty DotMap.
func NewDotMap[K comparable, V DotStore]() *DotMap[K, V] {
	q := &DotMap[K, V]{entries: make(map[K]V)}
	return q
}

// Set associates a key with a dot store.
func (p *DotMap[K, V]) Set(k K, v V) {
	p.entries[k] = v
}

// Get returns the dot store for a key and whether it exists.
func (p *DotMap[K, V]) Get(k K) (V, bool) {
	v, ok := p.entries[k]
	return v, ok
}

// Delete removes a key.
func (p *DotMap[K, V]) Delete(k K) {
	delete(p.entries, k)
}

// Len returns the number of keys.
func (p *DotMap[K, V]) Len() int {
	return len(p.entries)
}

// Range calls fn for each key-value pair. If fn returns false, iteration stops.
func (p *DotMap[K, V]) Range(fn func(K, V) bool) {
	for k, v := range p.entries {
		if !fn(k, v) {
			return
		}
	}
}

// Keys returns all keys.
func (p *DotMap[K, V]) Keys() []K {
	keys := make([]K, 0, len(p.entries))
	for k := range p.entries {
		keys = append(keys, k)
	}
	return keys
}

// Clone returns a shallow copy of the DotMap. Keys are copied but nested
// DotStore values are shared.
func (p *DotMap[K, V]) Clone() *DotMap[K, V] {
	dm := &DotMap[K, V]{entries: make(map[K]V, len(p.entries))}
	for k, v := range p.entries {
		dm.entries[k] = v
	}
	return dm
}

// Dots returns the union of dots across all nested stores (DotStore implementation).
func (p *DotMap[K, V]) Dots() *DotSet {
	ds := NewDotSet()
	for _, v := range p.entries {
		v.Dots().Range(func(d Dot) bool {
			ds.Add(d)
			return true
		})
	}
	return ds
}
