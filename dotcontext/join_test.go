package dotcontext

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// --- JoinDotSet tests ---

func TestJoinDotSetBothHave(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(d)
	a.Context.Add(d)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	b.Store.Add(d)
	b.Context.Add(d)

	result := JoinDotSet(a, b)
	c.Assert(result.Store.Has(d), qt.IsTrue)
}

func TestJoinDotSetOnlyOneHasNotObserved(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(d)
	a.Context.Add(d)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}

	result := JoinDotSet(a, b)
	c.Assert(result.Store.Has(d), qt.IsTrue)
}

func TestJoinDotSetOnlyOneHasButObserved(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(d)
	a.Context.Add(d)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	b.Context.Add(d) // observed but not in store → removed

	result := JoinDotSet(a, b)
	c.Assert(result.Store.Has(d), qt.IsFalse)
}

func TestJoinDotSetConcurrentAdds(t *testing.T) {
	c := qt.New(t)
	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(da)
	a.Context.Add(da)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	b.Store.Add(db)
	b.Context.Add(db)

	result := JoinDotSet(a, b)
	c.Assert(result.Store.Has(da), qt.IsTrue)
	c.Assert(result.Store.Has(db), qt.IsTrue)
}

// --- JoinDotFun tests ---

func TestJoinDotFunSharedDot(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	a.Store.Set(d, 3)
	a.Context.Add(d)

	b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	b.Store.Set(d, 7)
	b.Context.Add(d)

	result := JoinDotFun(a, b)
	v, ok := result.Store.Get(d)
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, maxInt(7))
}

func TestJoinDotFunDisjoint(t *testing.T) {
	c := qt.New(t)
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
	c.Assert(va, qt.Equals, maxInt(10))
	c.Assert(vb, qt.Equals, maxInt(20))
}

func TestJoinDotFunRemoved(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "a", Seq: 1}

	a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	a.Store.Set(d, 5)
	a.Context.Add(d)

	b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	b.Context.Add(d) // observed but not in store → removed

	result := JoinDotFun(a, b)
	_, ok := result.Store.Get(d)
	c.Assert(ok, qt.IsFalse)
}

// --- JoinDotMap tests ---

func TestJoinDotMapSharedKey(t *testing.T) {
	c := qt.New(t)
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
	result := JoinDotMap(a, b, joinDS, NewDotSet)

	v, ok := result.Store.Get("key")
	c.Assert(ok, qt.IsTrue)
	c.Assert(v.Has(da), qt.IsTrue)
	c.Assert(v.Has(db), qt.IsTrue)
}

func TestJoinDotMapKeyOnlyOneSide(t *testing.T) {
	c := qt.New(t)
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

	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] {
		return JoinDotSet(x, y)
	}
	result := JoinDotMap(a, b, joinDS, NewDotSet)

	v, ok := result.Store.Get("key")
	c.Assert(ok, qt.IsTrue)
	c.Assert(v.Has(d), qt.IsTrue)
}

// --- JoinDotSet edge cases ---

func TestJoinDotSetBothEmpty(t *testing.T) {
	c := qt.New(t)
	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	result := JoinDotSet(a, b)
	c.Assert(result.Store.Len(), qt.Equals, 0)
}

func TestJoinDotSetNonEmptyWithEmpty(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "a", Seq: 1}
	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(d)
	a.Context.Add(d)

	empty := Causal[*DotSet]{Store: NewDotSet(), Context: New()}

	// a ⊔ empty — dot survives (not in empty's context).
	result := JoinDotSet(a, empty)
	c.Assert(result.Store.Has(d), qt.IsTrue)

	// empty ⊔ a — same by commutativity.
	result2 := JoinDotSet(empty, a)
	c.Assert(result2.Store.Has(d), qt.IsTrue)
}

func TestJoinDotSetContextMerged(t *testing.T) {
	c := qt.New(t)
	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(da)
	a.Context.Add(da)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	b.Context.Add(db) // observed but removed

	result := JoinDotSet(a, b)
	// Output context should contain both.
	c.Assert(result.Context.Has(da), qt.IsTrue)
	c.Assert(result.Context.Has(db), qt.IsTrue)
}

// --- JoinDotFun edge cases ---

func TestJoinDotFunBothEmpty(t *testing.T) {
	c := qt.New(t)
	a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	result := JoinDotFun(a, b)
	c.Assert(result.Store.Len(), qt.Equals, 0)
}

func TestJoinDotFunNonEmptyWithEmpty(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "a", Seq: 1}
	a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	a.Store.Set(d, 42)
	a.Context.Add(d)

	empty := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}

	result := JoinDotFun(a, empty)
	v, ok := result.Store.Get(d)
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, maxInt(42))
}

// --- JoinDotMap edge cases ---

func TestJoinDotMapBothEmpty(t *testing.T) {
	c := qt.New(t)
	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] { return JoinDotSet(x, y) }
	a := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
	b := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
	result := JoinDotMap(a, b, joinDS, NewDotSet)
	c.Assert(result.Store.Len(), qt.Equals, 0)
}

func TestJoinDotMapKeyRemovedByContext(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "a", Seq: 1}
	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] { return JoinDotSet(x, y) }

	// a has key "x" with dot d.
	a := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
	sa := NewDotSet()
	sa.Add(d)
	a.Store.Set("x", sa)
	a.Context.Add(d)

	// b has no keys but has observed d (removal).
	b := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
	b.Context.Add(d)

	result := JoinDotMap(a, b, joinDS, NewDotSet)
	// Key "x" should be gone — its only dot is in b's context.
	_, ok := result.Store.Get("x")
	c.Assert(ok, qt.IsFalse)
}

func TestJoinDotMapKeyOnlyInB(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "b", Seq: 1}
	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] { return JoinDotSet(x, y) }

	a := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}

	b := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
	sb := NewDotSet()
	sb.Add(d)
	b.Store.Set("y", sb)
	b.Context.Add(d)

	result := JoinDotMap(a, b, joinDS, NewDotSet)
	v, ok := result.Store.Get("y")
	c.Assert(ok, qt.IsTrue)
	c.Assert(v.Has(d), qt.IsTrue)
}

func TestJoinDotMapContextMerged(t *testing.T) {
	c := qt.New(t)
	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}
	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] { return JoinDotSet(x, y) }

	a := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
	a.Context.Add(da)

	b := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
	b.Context.Add(db)

	result := JoinDotMap(a, b, joinDS, NewDotSet)
	c.Assert(result.Context.Has(da), qt.IsTrue)
	c.Assert(result.Context.Has(db), qt.IsTrue)
}

// --- Semilattice property tests ---

func makeTestCausalDotSet(dots []Dot) Causal[*DotSet] {
	s := NewDotSet()
	cc := New()
	for _, d := range dots {
		s.Add(d)
		cc.Add(d)
	}
	return Causal[*DotSet]{Store: s, Context: cc}
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
	c := qt.New(t)
	a := makeTestCausalDotSet([]Dot{{ID: "a", Seq: 1}, {ID: "b", Seq: 2}})
	result := JoinDotSet(a, a)
	c.Assert(dotSetEqual(result.Store, a.Store), qt.IsTrue)
}

func TestJoinDotSetCommutative(t *testing.T) {
	c := qt.New(t)
	a := makeTestCausalDotSet([]Dot{{ID: "a", Seq: 1}})
	b := makeTestCausalDotSet([]Dot{{ID: "b", Seq: 1}})

	ab := JoinDotSet(a, b)
	ba := JoinDotSet(b, a)
	c.Assert(dotSetEqual(ab.Store, ba.Store), qt.IsTrue)
}

func TestJoinDotSetAssociative(t *testing.T) {
	c := qt.New(t)
	a := makeTestCausalDotSet([]Dot{{ID: "a", Seq: 1}})
	b := makeTestCausalDotSet([]Dot{{ID: "b", Seq: 1}})
	x := makeTestCausalDotSet([]Dot{{ID: "c", Seq: 1}})

	ab_x := JoinDotSet(JoinDotSet(a, b), x)
	a_bx := JoinDotSet(a, JoinDotSet(b, x))
	c.Assert(dotSetEqual(ab_x.Store, a_bx.Store), qt.IsTrue)
}
