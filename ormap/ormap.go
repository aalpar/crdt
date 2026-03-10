package ormap

import (
	"github.com/aalpar/crdt/dotcontext"
)

// ORMap is an observed-remove map. Keys map to nested CRDT values
// of type V (any DotStore). Concurrent add and remove of the same
// key resolves in favor of add.
type ORMap[K comparable, V dotcontext.DotStore] struct {
	id     dotcontext.ReplicaID
	state  dotcontext.Causal[*dotcontext.DotMap[K, V]]
	mergeV func(V, V, *dotcontext.CausalContext, *dotcontext.CausalContext)
	emptyV func() V
}

// New creates an empty ORMap for the given replica.
//
// mergeV is the in-place nested merge for values of type V.
// emptyV returns a new empty value of type V.
func New[K comparable, V dotcontext.DotStore](
	replicaID dotcontext.ReplicaID,
	mergeV func(V, V, *dotcontext.CausalContext, *dotcontext.CausalContext),
	emptyV func() V,
) *ORMap[K, V] {
	q := &ORMap[K, V]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotMap[K, V]]{
			Store:   dotcontext.NewDotMap[K, V](),
			Context: dotcontext.New(),
		},
		mergeV: mergeV,
		emptyV: emptyV,
	}
	return q
}

// Apply mutates the value at key and returns a delta for replication.
//
// fn receives the replica ID, the ORMap's causal context for dot
// generation, the current value at key (mutate in place; if the key
// does not yet exist, v is a new empty value via emptyV), and a
// fresh delta value to populate with just the delta entries.
//
// fn must:
//   - Generate dots via ctx.Next(id)
//   - Add new dots/entries to BOTH v (local state) and delta
//     (the delta store is NOT derived automatically — fn must
//     write to it explicitly)
//   - To supersede old entries: remove them from v (the diff is
//     captured automatically in the delta's context)
//
// Local state is mutated directly — no self-merge.
func (p *ORMap[K, V]) Apply(key K, fn func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v V, delta V)) *ORMap[K, V] {
	v, ok := p.state.Store.Get(key)
	if !ok {
		v = p.emptyV()
		p.state.Store.Set(key, v)
	}

	dotsBefore := v.Dots()
	deltaV := p.emptyV()

	fn(p.id, p.state.Context, v, deltaV)

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
		mergeV: p.mergeV,
		emptyV: p.emptyV,
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
func (p *ORMap[K, V]) Remove(key K) *ORMap[K, V] {
	deltaCtx := dotcontext.New()

	if v, ok := p.state.Store.Get(key); ok {
		v.Dots().Range(func(d dotcontext.Dot) bool {
			deltaCtx.Add(d)
			return true
		})
		p.state.Store.Delete(key)
	}

	return &ORMap[K, V]{
		mergeV: p.mergeV,
		emptyV: p.emptyV,
		state: dotcontext.Causal[*dotcontext.DotMap[K, V]]{
			Store:   dotcontext.NewDotMap[K, V](),
			Context: deltaCtx,
		},
	}
}

// Get returns the value at key and whether the key exists.
func (p *ORMap[K, V]) Get(key K) (V, bool) {
	return p.state.Store.Get(key)
}

// Has reports whether key is in the map.
func (p *ORMap[K, V]) Has(key K) bool {
	_, ok := p.state.Store.Get(key)
	return ok
}

// Keys returns all keys currently in the map.
// The order is non-deterministic.
func (p *ORMap[K, V]) Keys() []K {
	return p.state.Store.Keys()
}

// Len returns the number of keys in the map.
func (p *ORMap[K, V]) Len() int {
	return p.state.Store.Len()
}

// Context returns the ORMap's causal context. Callers need this to
// allocate dots when building fn callbacks for Apply.
func (p *ORMap[K, V]) Context() *dotcontext.CausalContext {
	return p.state.Context
}

// State returns the ORMap's internal Causal state for serialization.
func (p *ORMap[K, V]) State() dotcontext.Causal[*dotcontext.DotMap[K, V]] {
	return p.state
}

// FromCausal constructs an ORMap from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal[K comparable, V dotcontext.DotStore](
	state dotcontext.Causal[*dotcontext.DotMap[K, V]],
	mergeV func(V, V, *dotcontext.CausalContext, *dotcontext.CausalContext),
	emptyV func() V,
) *ORMap[K, V] {
	return &ORMap[K, V]{
		state:  state,
		mergeV: mergeV,
		emptyV: emptyV,
	}
}

// Merge incorporates a delta or full state from another ORMap.
func (p *ORMap[K, V]) Merge(other *ORMap[K, V]) {
	dotcontext.MergeDotMap(&p.state, other.state, p.mergeV, p.emptyV)
}
