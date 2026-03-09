package rwset

import (
	"github.com/aalpar/crdt/dotcontext"
)

// Presence tracks whether a dot represents an add (Active=true) or
// a remove (Active=false). It implements dotcontext.Lattice. Under
// normal operation, JoinDotFun only calls Join when both sides share
// the same dot, so values are identical and either is correct.
type Presence struct {
	Active bool
}

// Join satisfies dotcontext.Lattice. Same dot implies same value,
// so either side is correct.
func (p Presence) Join(other Presence) Presence {
	return p
}

// RWSet is a remove-wins observed-remove set. Concurrent add and
// remove of the same element resolves in favor of remove.
//
// Internally it is a DotMap[E, *DotFun[Presence]] paired with a
// causal context. Both Add and Remove generate dots. An element is
// present only when all its surviving dots are Active.
type RWSet[E comparable] struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotFun[Presence]]]
}

// New creates an empty RWSet for the given replica.
func New[E comparable](replicaID dotcontext.ReplicaID) *RWSet[E] {
	return &RWSet[E]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotFun[Presence]]]{
			Store:   dotcontext.NewDotMap[E, *dotcontext.DotFun[Presence]](),
			Context: dotcontext.New(),
		},
	}
}

// Add inserts elem into the set and returns a delta for replication.
//
// A new dot is generated with Active=true. All existing dots for this
// element (both adds and removes) are superseded: removed from the
// local DotFun and recorded in the delta's context so remote replicas
// remove them on merge.
func (p *RWSet[E]) Add(elem E) *RWSet[E] {
	d := p.state.Context.Next(p.id)

	df, ok := p.state.Store.Get(elem)
	if !ok {
		df = dotcontext.NewDotFun[Presence]()
		p.state.Store.Set(elem, df)
	}

	// Collect old dots before mutating.
	var oldDots []dotcontext.Dot
	df.Range(func(od dotcontext.Dot, _ Presence) bool {
		oldDots = append(oldDots, od)
		return true
	})

	// Supersede old dots in local state.
	for _, od := range oldDots {
		df.Remove(od)
	}

	// Add new dot to local state.
	df.Set(d, Presence{Active: true})

	// Build delta: store has only the new entry, context has
	// the new dot plus all superseded dots.
	deltaDF := dotcontext.NewDotFun[Presence]()
	deltaDF.Set(d, Presence{Active: true})
	deltaStore := dotcontext.NewDotMap[E, *dotcontext.DotFun[Presence]]()
	deltaStore.Set(elem, deltaDF)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)
	for _, od := range oldDots {
		deltaCtx.Add(od)
	}

	return &RWSet[E]{
		state: dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotFun[Presence]]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Remove deletes elem from the set and returns a delta for replication.
//
// If elem is not in the store, an empty delta is returned (no dot
// generated). Otherwise a new dot with Active=false is generated,
// superseding all existing dots for the element.
func (p *RWSet[E]) Remove(elem E) *RWSet[E] {
	df, ok := p.state.Store.Get(elem)
	if !ok {
		// Element not present — no-op, like AWSet Remove of non-existent element.
		return &RWSet[E]{
			state: dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotFun[Presence]]]{
				Store:   dotcontext.NewDotMap[E, *dotcontext.DotFun[Presence]](),
				Context: dotcontext.New(),
			},
		}
	}

	// Collect old dots before mutating.
	var oldDots []dotcontext.Dot
	df.Range(func(od dotcontext.Dot, _ Presence) bool {
		oldDots = append(oldDots, od)
		return true
	})

	d := p.state.Context.Next(p.id)

	// Supersede old dots in local state.
	for _, od := range oldDots {
		df.Remove(od)
	}

	// Add tombstone dot to local state.
	df.Set(d, Presence{Active: false})

	// Build delta: store has only the tombstone, context has
	// the new dot plus all superseded dots.
	deltaDF := dotcontext.NewDotFun[Presence]()
	deltaDF.Set(d, Presence{Active: false})
	deltaStore := dotcontext.NewDotMap[E, *dotcontext.DotFun[Presence]]()
	deltaStore.Set(elem, deltaDF)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)
	for _, od := range oldDots {
		deltaCtx.Add(od)
	}

	return &RWSet[E]{
		state: dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotFun[Presence]]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Has reports whether elem is in the set. An element is present iff
// its DotFun has entries and ALL entries are Active (no tombstones).
func (p *RWSet[E]) Has(elem E) bool {
	df, ok := p.state.Store.Get(elem)
	if !ok {
		return false
	}
	if df.Len() == 0 {
		return false
	}
	has := true
	df.Range(func(_ dotcontext.Dot, v Presence) bool {
		if !v.Active {
			has = false
			return false
		}
		return true
	})
	return has
}

// Elements returns all elements currently in the set (where Has is true).
// The order is non-deterministic.
func (p *RWSet[E]) Elements() []E {
	var elems []E
	p.state.Store.Range(func(k E, _ *dotcontext.DotFun[Presence]) bool {
		if p.Has(k) {
			elems = append(elems, k)
		}
		return true
	})
	return elems
}

// Len returns the number of elements in the set (where Has is true).
func (p *RWSet[E]) Len() int {
	n := 0
	p.state.Store.Range(func(k E, _ *dotcontext.DotFun[Presence]) bool {
		if p.Has(k) {
			n++
		}
		return true
	})
	return n
}

// State returns the RWSet's internal Causal state for serialization.
func (p *RWSet[E]) State() dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotFun[Presence]]] {
	return p.state
}

// FromCausal constructs an RWSet from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal[E comparable](
	state dotcontext.Causal[*dotcontext.DotMap[E, *dotcontext.DotFun[Presence]]],
) *RWSet[E] {
	return &RWSet[E]{state: state}
}

// Merge incorporates a delta or full state from another RWSet.
func (p *RWSet[E]) Merge(other *RWSet[E]) {
	p.state = dotcontext.JoinDotMap(p.state, other.state, joinPresenceFunStore, dotcontext.NewDotFun[Presence])
}

// joinPresenceFunStore is the store-only join for DotFun[Presence],
// instantiating the generic JoinDotFunStore for use as a JoinDotMap callback.
func joinPresenceFunStore(a, b *dotcontext.DotFun[Presence], ctxA, ctxB *dotcontext.CausalContext) *dotcontext.DotFun[Presence] {
	return dotcontext.JoinDotFunStore(a, b, ctxA, ctxB)
}
