package ormap

import (
	"github.com/aalpar/crdt/dotcontext"
)

// ORMap is an observed-remove map. Keys map to nested CRDT values
// of type V (any DotStore). Concurrent add and remove of the same
// key resolves in favor of add.
type ORMap[K comparable, V dotcontext.DotStore] struct {
	id     string
	state  dotcontext.Causal[*dotcontext.DotMap[K, V]]
	joinV  func(dotcontext.Causal[V], dotcontext.Causal[V]) dotcontext.Causal[V]
	emptyV func() V
}

// New creates an empty ORMap for the given replica.
//
// joinV is the nested join function for merging values of type V.
// emptyV returns a new empty value of type V.
func New[K comparable, V dotcontext.DotStore](
	replicaID string,
	joinV func(dotcontext.Causal[V], dotcontext.Causal[V]) dotcontext.Causal[V],
	emptyV func() V,
) *ORMap[K, V] {
	return &ORMap[K, V]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotMap[K, V]]{
			Store:   dotcontext.NewDotMap[K, V](),
			Context: dotcontext.New(),
		},
		joinV:  joinV,
		emptyV: emptyV,
	}
}

// Apply mutates the value at key and returns a delta for replication.
//
// fn receives the replica ID, the ORMap's causal context for dot
// generation, the current value at key (mutate in place), and a
// fresh delta value to populate with just the delta entries.
//
// fn must:
//   - Generate dots via ctx.Next(id)
//   - Add new dots/entries to BOTH v (local state) and delta
//   - To supersede old entries: remove them from v (the diff is
//     captured automatically in the delta's context)
//
// Local state is mutated directly — no self-merge.
func (m *ORMap[K, V]) Apply(key K, fn func(id string, ctx *dotcontext.CausalContext, v V, delta V)) *ORMap[K, V] {
	v, ok := m.state.Store.Get(key)
	if !ok {
		v = m.emptyV()
		m.state.Store.Set(key, v)
	}

	dotsBefore := v.Dots()
	deltaV := m.emptyV()

	fn(m.id, m.state.Context, v, deltaV)

	dotsAfter := v.Dots()

	// Delta context: new dots (added by fn) + removed dots (superseded by fn).
	deltaCtx := dotcontext.New()
	dotsAfter.Range(func(d dotcontext.Dot) bool {
		if !dotsBefore.Has(d) {
			deltaCtx.Add(d)
		}
		return true
	})
	dotsBefore.Range(func(d dotcontext.Dot) bool {
		if !dotsAfter.Has(d) {
			deltaCtx.Add(d)
		}
		return true
	})

	deltaStore := dotcontext.NewDotMap[K, V]()
	deltaStore.Set(key, deltaV)

	return &ORMap[K, V]{
		joinV:  m.joinV,
		emptyV: m.emptyV,
		state: dotcontext.Causal[*dotcontext.DotMap[K, V]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Remove deletes key from the map and returns a delta for replication.
//
// The delta carries an empty store but a context containing all dots
// of the removed value. When merged with a concurrent Apply (which
// introduces new, unobserved dots), the Apply's dots survive.
func (m *ORMap[K, V]) Remove(key K) *ORMap[K, V] {
	deltaCtx := dotcontext.New()

	if v, ok := m.state.Store.Get(key); ok {
		v.Dots().Range(func(d dotcontext.Dot) bool {
			deltaCtx.Add(d)
			return true
		})
		m.state.Store.Delete(key)
	}

	return &ORMap[K, V]{
		joinV:  m.joinV,
		emptyV: m.emptyV,
		state: dotcontext.Causal[*dotcontext.DotMap[K, V]]{
			Store:   dotcontext.NewDotMap[K, V](),
			Context: deltaCtx,
		},
	}
}

// Get returns the value at key and whether the key exists.
func (m *ORMap[K, V]) Get(key K) (V, bool) {
	return m.state.Store.Get(key)
}

// Has reports whether key is in the map.
func (m *ORMap[K, V]) Has(key K) bool {
	_, ok := m.state.Store.Get(key)
	return ok
}

// Keys returns all keys currently in the map.
// The order is non-deterministic.
func (m *ORMap[K, V]) Keys() []K {
	return m.state.Store.Keys()
}

// Len returns the number of keys in the map.
func (m *ORMap[K, V]) Len() int {
	return m.state.Store.Len()
}

// Context returns the ORMap's causal context.
func (m *ORMap[K, V]) Context() *dotcontext.CausalContext {
	return m.state.Context
}

// Merge incorporates a delta or full state from another ORMap.
func (m *ORMap[K, V]) Merge(other *ORMap[K, V]) {
	m.state = dotcontext.JoinDotMap(m.state, other.state, m.joinV, m.emptyV)
}
