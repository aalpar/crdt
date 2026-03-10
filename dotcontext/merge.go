package dotcontext

// MergeDotSetStore merges delta into state in place using the set join formula:
//
//	keep:   (state ∩ delta) ∪ (state \ ctxDelta)
//	add:    delta \ ctxState
//	remove: state ∩ ctxDelta \ delta
//
// This is the in-place equivalent of JoinDotSetStore. The state is mutated;
// the delta, ctxState, and ctxDelta are read-only.
func MergeDotSetStore(state, delta *DotSet, ctxState, ctxDelta *CausalContext) {
	// Remove from state: dots observed by delta's context but not in delta.
	state.Range(func(d Dot) bool {
		if !delta.Has(d) && ctxDelta.Has(d) {
			state.Remove(d)
		}
		return true
	})

	// Add from delta: dots not yet observed by state.
	delta.Range(func(d Dot) bool {
		if !ctxState.Has(d) {
			state.Add(d)
		}
		return true
	})
}

// MergeDotFunStore merges delta into state in place using the lattice join
// formula. For dots in both stores, values are joined via the Lattice.
// This is the in-place equivalent of JoinDotFunStore.
func MergeDotFunStore[V Lattice[V]](state, delta *DotFun[V], ctxState, ctxDelta *CausalContext) {
	// Dots in state: join if in delta, remove if observed by delta.
	state.Range(func(d Dot, va V) bool {
		if vb, ok := delta.Get(d); ok {
			state.Set(d, va.Join(vb))
		} else if ctxDelta.Has(d) {
			state.Remove(d)
		}
		return true
	})

	// Dots in delta only: add if not observed by state.
	delta.Range(func(d Dot, vb V) bool {
		if _, ok := state.Get(d); !ok && !ctxState.Has(d) {
			state.Set(d, vb)
		}
		return true
	})
}

// MergeDotSet merges a delta into state in place (store + context).
// This is the in-place equivalent of JoinDotSet.
func MergeDotSet(state *Causal[*DotSet], delta Causal[*DotSet]) {
	MergeDotSetStore(state.Store, delta.Store, state.Context, delta.Context)
	state.Context.Merge(delta.Context)
	state.Context.Compact()
}

// MergeDotFun merges a delta into state in place (store + context).
// This is the in-place equivalent of JoinDotFun.
func MergeDotFun[V Lattice[V]](state *Causal[*DotFun[V]], delta Causal[*DotFun[V]]) {
	MergeDotFunStore(state.Store, delta.Store, state.Context, delta.Context)
	state.Context.Merge(delta.Context)
	state.Context.Compact()
}

// MergeDotMapStore merges delta into state in place using the recursive
// DotMap join formula. mergeV is the in-place nested merge for values of
// type V. emptyV constructs a zero-value DotStore for keys present on
// only one side.
// This is the store-only in-place equivalent of JoinDotMap's store logic.
func MergeDotMapStore[K comparable, V DotStore](
	state, delta *DotMap[K, V],
	ctxState, ctxDelta *CausalContext,
	mergeV func(V, V, *CausalContext, *CausalContext),
	emptyV func() V,
) {
	empty := emptyV()

	// Keys in state: merge with delta value or empty.
	var toDelete []K
	state.Range(func(k K, va V) bool {
		if vb, ok := delta.Get(k); ok {
			mergeV(va, vb, ctxState, ctxDelta)
		} else {
			mergeV(va, empty, ctxState, ctxDelta)
		}
		if !va.HasDots() {
			toDelete = append(toDelete, k)
		}
		return true
	})
	for _, k := range toDelete {
		state.Delete(k)
	}

	// Keys only in delta: create new value, merge, add if non-empty.
	delta.Range(func(k K, vb V) bool {
		if _, ok := state.Get(k); ok {
			return true // already handled above
		}
		newV := emptyV()
		mergeV(newV, vb, ctxState, ctxDelta)
		if newV.HasDots() {
			state.Set(k, newV)
		}
		return true
	})
}

// MergeDotMap merges a delta into state in place (store + context).
// This is the in-place equivalent of JoinDotMap.
func MergeDotMap[K comparable, V DotStore](
	state *Causal[*DotMap[K, V]],
	delta Causal[*DotMap[K, V]],
	mergeV func(V, V, *CausalContext, *CausalContext),
	emptyV func() V,
) {
	MergeDotMapStore(state.Store, delta.Store, state.Context, delta.Context, mergeV, emptyV)
	state.Context.Merge(delta.Context)
	state.Context.Compact()
}
