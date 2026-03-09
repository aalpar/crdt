package dotcontext

// JoinDotSetStore merges two DotSets using the set join formula:
//
//	(s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁)
//
// ctxA and ctxB are the causal contexts for dot-survival checks.
// This is the store-only variant — it does not compute a merged context.
func JoinDotSetStore(a, b *DotSet, ctxA, ctxB *CausalContext) *DotSet {
	result := NewDotSet()

	// Dots in a: keep if in b (intersection) or unobserved by b (s₁ \ c₂).
	a.Range(func(d Dot) bool {
		if b.Has(d) || !ctxB.Has(d) {
			result.Add(d)
		}
		return true
	})

	// Dots only in b: keep if unobserved by a (s₂ \ c₁).
	b.Range(func(d Dot) bool {
		if !a.Has(d) && !ctxA.Has(d) {
			result.Add(d)
		}
		return true
	})

	return result
}

// JoinDotSet merges two Causal[*DotSet] values using the set join formula.
func JoinDotSet(a, b Causal[*DotSet]) Causal[*DotSet] {
	result := JoinDotSetStore(a.Store, b.Store, a.Context, b.Context)
	ctx := a.Context.Clone()
	ctx.Merge(b.Context)
	ctx.Compact()
	return Causal[*DotSet]{Store: result, Context: ctx}
}

// JoinDotFunStore merges two DotFuns using the lattice join formula.
// For dots present in both stores, values are joined via the Lattice.
// For dots in only one store, the dot survives if the other side
// hasn't observed it (not in the other's causal context).
// This is the store-only variant — it does not compute a merged context.
func JoinDotFunStore[V Lattice[V]](a, b *DotFun[V], ctxA, ctxB *CausalContext) *DotFun[V] {
	result := NewDotFun[V]()

	// Dots in a: keep if also in b (join values) or if not observed by b.
	a.Range(func(d Dot, va V) bool {
		if vb, ok := b.Get(d); ok {
			result.Set(d, va.Join(vb))
		} else if !ctxB.Has(d) {
			result.Set(d, va)
		}
		return true
	})

	// Dots in b only: keep if not observed by a.
	b.Range(func(d Dot, vb V) bool {
		if _, ok := a.Get(d); !ok && !ctxA.Has(d) {
			result.Set(d, vb)
		}
		return true
	})

	return result
}

// JoinDotFun merges two Causal[*DotFun[V]] values.
func JoinDotFun[V Lattice[V]](a, b Causal[*DotFun[V]]) Causal[*DotFun[V]] {
	result := JoinDotFunStore(a.Store, b.Store, a.Context, b.Context)
	ctx := a.Context.Clone()
	ctx.Merge(b.Context)
	ctx.Compact()
	return Causal[*DotFun[V]]{Store: result, Context: ctx}
}

// JoinDotMap merges two Causal[*DotMap[K,V]] values.
// joinV is the store-only nested join for values of type V — the caller
// provides it because Go generics can't dispatch recursive joins
// through interfaces. emptyV constructs a zero-value DotStore, used
// when a key exists on only one side and must be joined against an
// empty counterpart.
func JoinDotMap[K comparable, V DotStore](
	a, b Causal[*DotMap[K, V]],
	joinV func(V, V, *CausalContext, *CausalContext) V,
	emptyV func() V,
) Causal[*DotMap[K, V]] {
	result := NewDotMap[K, V]()

	// Keys in a.
	a.Store.Range(func(k K, va V) bool {
		if vb, ok := b.Store.Get(k); ok {
			// Key in both: recursive join.
			joined := joinV(va, vb, a.Context, b.Context)
			if joined.Dots().Len() > 0 {
				result.Set(k, joined)
			}
		} else {
			// Key only in a: join with empty (b's context still applies).
			joined := joinV(va, emptyV(), a.Context, b.Context)
			if joined.Dots().Len() > 0 {
				result.Set(k, joined)
			}
		}
		return true
	})

	// Keys only in b.
	b.Store.Range(func(k K, vb V) bool {
		if _, ok := a.Store.Get(k); ok {
			return true // already handled above
		}
		joined := joinV(emptyV(), vb, a.Context, b.Context)
		if joined.Dots().Len() > 0 {
			result.Set(k, joined)
		}
		return true
	})

	ctx := a.Context.Clone()
	ctx.Merge(b.Context)
	ctx.Compact()

	return Causal[*DotMap[K, V]]{Store: result, Context: ctx}
}
