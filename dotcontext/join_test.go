package dotcontext

import (
	"testing"
)

// --- JoinDotSet tests ---

func TestJoinDotSetBothHave(t *testing.T) {
	// Dot in both stores → survives (intersection term).
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(d)
	a.Context.Add(d)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	b.Store.Add(d)
	b.Context.Add(d)

	result := JoinDotSet(a, b)
	if !result.Store.Has(d) {
		t.Error("dot in both stores should survive join")
	}
}

func TestJoinDotSetOnlyOneHasNotObserved(t *testing.T) {
	// Dot in a but not observed by b → survives (s₁ \ c₂ term).
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(d)
	a.Context.Add(d)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	// b has never seen this dot

	result := JoinDotSet(a, b)
	if !result.Store.Has(d) {
		t.Error("dot in a, unobserved by b, should survive")
	}
}

func TestJoinDotSetOnlyOneHasButObserved(t *testing.T) {
	// Dot in a's store, but b's context has observed it (b removed it).
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(d)
	a.Context.Add(d)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	b.Context.Add(d) // observed but not in store → removed

	result := JoinDotSet(a, b)
	if result.Store.Has(d) {
		t.Error("dot removed by b should not survive")
	}
}

func TestJoinDotSetConcurrentAdds(t *testing.T) {
	// Two replicas concurrently add different dots.
	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(da)
	a.Context.Add(da)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	b.Store.Add(db)
	b.Context.Add(db)

	result := JoinDotSet(a, b)
	if !result.Store.Has(da) || !result.Store.Has(db) {
		t.Error("concurrent adds should both survive")
	}
}

// --- JoinDotFun tests ---

func TestJoinDotFunSharedDot(t *testing.T) {
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	a.Store.Set(d, 3)
	a.Context.Add(d)

	b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	b.Store.Set(d, 7)
	b.Context.Add(d)

	result := JoinDotFun(a, b)
	v, ok := result.Store.Get(d)
	if !ok || v != 7 {
		t.Errorf("joined value = %v, want 7 (max)", v)
	}
}

func TestJoinDotFunDisjoint(t *testing.T) {
	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	a.Store.Set(da, 10)
	a.Context.Add(da)

	b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	b.Store.Set(db, 20)
	b.Context.Add(db)

	result := JoinDotFun(a, b)
	va, _ := result.Store.Get(da)
	vb, _ := result.Store.Get(db)
	if va != 10 || vb != 20 {
		t.Errorf("disjoint dots: got a=%v b=%v, want 10 and 20", va, vb)
	}
}

func TestJoinDotFunRemoved(t *testing.T) {
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	a.Store.Set(d, 5)
	a.Context.Add(d)

	b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	b.Context.Add(d) // observed but not in store → removed

	result := JoinDotFun(a, b)
	if _, ok := result.Store.Get(d); ok {
		t.Error("dot removed by b should not survive")
	}
}

// --- JoinDotMap tests ---

func TestJoinDotMapSharedKey(t *testing.T) {
	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	a := Causal[*DotMap[string, *DotSet]]{
		Store:   NewDotMap[string, *DotSet](),
		Context: New(),
	}
	sa := NewDotSet()
	sa.Add(da)
	a.Store.Set("key", sa)
	a.Context.Add(da)

	b := Causal[*DotMap[string, *DotSet]]{
		Store:   NewDotMap[string, *DotSet](),
		Context: New(),
	}
	sb := NewDotSet()
	sb.Add(db)
	b.Store.Set("key", sb)
	b.Context.Add(db)

	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] {
		return JoinDotSet(x, y)
	}
	result := JoinDotMap(a, b, joinDS)

	v, ok := result.Store.Get("key")
	if !ok {
		t.Fatal("should have 'key' after join")
	}
	if !v.Has(da) || !v.Has(db) {
		t.Error("both dots should survive in shared key join")
	}
}

func TestJoinDotMapKeyOnlyOneSide(t *testing.T) {
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotMap[string, *DotSet]]{
		Store:   NewDotMap[string, *DotSet](),
		Context: New(),
	}
	sa := NewDotSet()
	sa.Add(d)
	a.Store.Set("key", sa)
	a.Context.Add(d)

	b := Causal[*DotMap[string, *DotSet]]{
		Store:   NewDotMap[string, *DotSet](),
		Context: New(),
	}
	// b has never seen "key" or the dot

	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] {
		return JoinDotSet(x, y)
	}
	result := JoinDotMap(a, b, joinDS)

	v, ok := result.Store.Get("key")
	if !ok {
		t.Fatal("key with unobserved dot should survive")
	}
	if !v.Has(d) {
		t.Error("dot should survive when unobserved by other side")
	}
}

// --- Semilattice property tests ---

func makeTestCausalDotSet(dots []Dot) Causal[*DotSet] {
	s := NewDotSet()
	c := New()
	for _, d := range dots {
		s.Add(d)
		c.Add(d)
	}
	return Causal[*DotSet]{Store: s, Context: c}
}

func dotSetEqual(a, b *DotSet) bool {
	if a.Len() != b.Len() {
		return false
	}
	equal := true
	a.Range(func(d Dot) bool {
		if !b.Has(d) {
			equal = false
			return false
		}
		return true
	})
	return equal
}

func TestJoinDotSetIdempotent(t *testing.T) {
	a := makeTestCausalDotSet([]Dot{{ID: "a", Seq: 1}, {ID: "b", Seq: 2}})
	result := JoinDotSet(a, a)
	if !dotSetEqual(result.Store, a.Store) {
		t.Error("join(a, a) should equal a (idempotent)")
	}
}

func TestJoinDotSetCommutative(t *testing.T) {
	a := makeTestCausalDotSet([]Dot{{ID: "a", Seq: 1}})
	b := makeTestCausalDotSet([]Dot{{ID: "b", Seq: 1}})

	ab := JoinDotSet(a, b)
	ba := JoinDotSet(b, a)
	if !dotSetEqual(ab.Store, ba.Store) {
		t.Error("join(a, b) should equal join(b, a) (commutative)")
	}
}

func TestJoinDotSetAssociative(t *testing.T) {
	a := makeTestCausalDotSet([]Dot{{ID: "a", Seq: 1}})
	b := makeTestCausalDotSet([]Dot{{ID: "b", Seq: 1}})
	c := makeTestCausalDotSet([]Dot{{ID: "c", Seq: 1}})

	ab_c := JoinDotSet(JoinDotSet(a, b), c)
	a_bc := JoinDotSet(a, JoinDotSet(b, c))
	if !dotSetEqual(ab_c.Store, a_bc.Store) {
		t.Error("join(join(a,b),c) should equal join(a,join(b,c)) (associative)")
	}
}
