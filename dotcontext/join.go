package dotcontext

// JoinDotSet merges two Causal[*DotSet] values using the set join formula:
//
//	(s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁)
//
// A dot survives if both sides have it (intersection), or one side has it
// and the other's causal context hasn't observed it (difference).
func JoinDotSet(a, b Causal[*DotSet]) Causal[*DotSet] {
	result := NewDotSet()

	// Dots in a: keep if in b (intersection) or unobserved by b (s₁ \ c₂).
	a.Store.Range(func(d Dot) bool {
		if b.Store.Has(d) || !b.Context.Has(d) {
			result.Add(d)
		}
		return true
	})

	// Dots only in b: keep if unobserved by a (s₂ \ c₁).
	b.Store.Range(func(d Dot) bool {
		if !a.Store.Has(d) && !a.Context.Has(d) {
			result.Add(d)
		}
		return true
	})

	ctx := a.Context.Clone()
	ctx.Merge(b.Context)
	ctx.Compact()

	return Causal[*DotSet]{Store: result, Context: ctx}
}

// JoinDotFun merges two Causal[*DotFun[V]] values.
// For dots present in both stores, values are joined via the Lattice.
// For dots in only one store, the dot survives if the other side
// hasn't observed it (not in the other's causal context).
func JoinDotFun[V Lattice[V]](a, b Causal[*DotFun[V]]) Causal[*DotFun[V]] {
	result := NewDotFun[V]()

	// Dots in a: keep if also in b (join values) or if not observed by b.
	a.Store.Range(func(d Dot, va V) bool {
		if vb, ok := b.Store.Get(d); ok {
			result.Set(d, va.Join(vb))
		} else if !b.Context.Has(d) {
			result.Set(d, va)
		}
		return true
	})

	// Dots in b only: keep if not observed by a.
	b.Store.Range(func(d Dot, vb V) bool {
		if _, ok := a.Store.Get(d); !ok && !a.Context.Has(d) {
			result.Set(d, vb)
		}
		return true
	})

	ctx := a.Context.Clone()
	ctx.Merge(b.Context)
	ctx.Compact()

	return Causal[*DotFun[V]]{Store: result, Context: ctx}
}

// JoinDotMap merges two Causal[*DotMap[K,V]] values.
// joinV is the nested join function for the value type V — the caller
// provides it because Go generics can't dispatch recursive joins
// through interfaces. emptyV constructs a zero-value DotStore, used
// when a key exists on only one side and must be joined against an
// empty counterpart.
func JoinDotMap[K comparable, V DotStore](
	a, b Causal[*DotMap[K, V]],
	joinV func(Causal[V], Causal[V]) Causal[V],
	emptyV func() V,
) Causal[*DotMap[K, V]] {
	result := NewDotMap[K, V]()

	// Keys in a.
	a.Store.Range(func(k K, va V) bool {
		if vb, ok := b.Store.Get(k); ok {
			// Key in both: recursive join.
			joined := joinV(
				Causal[V]{Store: va, Context: a.Context},
				Causal[V]{Store: vb, Context: b.Context},
			)
			if joined.Store.Dots().Len() > 0 {
				result.Set(k, joined.Store)
			}
		} else {
			// Key only in a: join with empty (b's context still applies).
			joined := joinV(
				Causal[V]{Store: va, Context: a.Context},
				Causal[V]{Store: emptyV(), Context: b.Context},
			)
			// Only keep if the result has dots (not fully subsumed).
			if joined.Store.Dots().Len() > 0 {
				result.Set(k, joined.Store)
			}
		}
		return true
	})

	// Keys only in b.
	b.Store.Range(func(k K, vb V) bool {
		if _, ok := a.Store.Get(k); ok {
			return true // already handled above
		}
		joined := joinV(
			Causal[V]{Store: emptyV(), Context: a.Context},
			Causal[V]{Store: vb, Context: b.Context},
		)
		if joined.Store.Dots().Len() > 0 {
			result.Set(k, joined.Store)
		}
		return true
	})

	ctx := a.Context.Clone()
	ctx.Merge(b.Context)
	ctx.Compact()

	return Causal[*DotMap[K, V]]{Store: result, Context: ctx}
}
