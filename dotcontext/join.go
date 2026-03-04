package dotcontext

// JoinDotSet merges two Causal[*DotSet] values using the set join formula:
//
//	(s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁)
//
// A dot survives if both sides have it (intersection), or one side has it
// and the other's causal context hasn't observed it (difference).
//
// TODO(user): implement the three-term set formula.
func JoinDotSet(a, b Causal[*DotSet]) Causal[*DotSet] {
	result := NewDotSet()

	// USER IMPLEMENTS THIS — ~10 lines
	// Hint: iterate a.Store, iterate b.Store, apply the formula above.
	_ = result

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
// through interfaces.
func JoinDotMap[K comparable, V DotStore](
	a, b Causal[*DotMap[K, V]],
	joinV func(Causal[V], Causal[V]) Causal[V],
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
			result.Set(k, joined.Store)
		} else {
			// Key only in a: join with empty (b's context still applies).
			joined := joinV(
				Causal[V]{Store: va, Context: a.Context},
				Causal[V]{Store: emptyOf(va), Context: b.Context},
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
			Causal[V]{Store: emptyOf(vb), Context: a.Context},
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

// emptyOf returns an empty DotStore of the same concrete type.
// This uses a type switch over the known DotStore implementations.
func emptyOf[V DotStore](v V) V {
	switch any(v).(type) {
	case *DotSet:
		return any(NewDotSet()).(V)
	default:
		// For unknown types, return the zero value.
		// This is a limitation — new DotStore implementations
		// must be added here or provide their own empty constructor.
		var zero V
		return zero
	}
}
