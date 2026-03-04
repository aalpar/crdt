package dotcontext

// DotMap maps keys to nested dot stores: K → V where V is a DotStore.
// Used by awset (DotMap[string, *DotSet]) and ORMap.
type DotMap[K comparable, V DotStore] struct {
	entries map[K]V
}

// NewDotMap returns an empty DotMap.
func NewDotMap[K comparable, V DotStore]() *DotMap[K, V] {
	return &DotMap[K, V]{entries: make(map[K]V)}
}

// Set associates a key with a dot store.
func (m *DotMap[K, V]) Set(k K, v V) {
	m.entries[k] = v
}

// Get returns the dot store for a key and whether it exists.
func (m *DotMap[K, V]) Get(k K) (V, bool) {
	v, ok := m.entries[k]
	return v, ok
}

// Delete removes a key.
func (m *DotMap[K, V]) Delete(k K) {
	delete(m.entries, k)
}

// Len returns the number of keys.
func (m *DotMap[K, V]) Len() int {
	return len(m.entries)
}

// Range calls fn for each key-value pair. If fn returns false, iteration stops.
func (m *DotMap[K, V]) Range(fn func(K, V) bool) {
	for k, v := range m.entries {
		if !fn(k, v) {
			return
		}
	}
}

// Keys returns all keys.
func (m *DotMap[K, V]) Keys() []K {
	keys := make([]K, 0, len(m.entries))
	for k := range m.entries {
		keys = append(keys, k)
	}
	return keys
}

// Dots returns the union of dots across all nested stores (DotStore implementation).
func (m *DotMap[K, V]) Dots() *DotSet {
	ds := NewDotSet()
	for _, v := range m.entries {
		v.Dots().Range(func(d Dot) bool {
			ds.Add(d)
			return true
		})
	}
	return ds
}
