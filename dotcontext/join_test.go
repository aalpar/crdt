package dotcontext

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
)

// --- helpers ---

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

// --- JoinDotSet table-driven tests ---

func TestJoinDotSet(t *testing.T) {
	c := qt.New(t)

	type testCase struct {
		name string
		a    func() Causal[*DotSet]
		b    func() Causal[*DotSet]
		check func(c *qt.C, result Causal[*DotSet])
	}

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	tests := []testCase{
		{
			name: "BothHave",
			a: func() Causal[*DotSet] {
				a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				a.Store.Add(da)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotSet] {
				b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				b.Store.Add(da)
				b.Context.Add(da)
				return b
			},
			check: func(c *qt.C, result Causal[*DotSet]) {
				c.Assert(result.Store.Has(da), qt.IsTrue)
			},
		},
		{
			name: "OnlyOneHasNotObserved",
			a: func() Causal[*DotSet] {
				a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				a.Store.Add(da)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotSet] {
				return Causal[*DotSet]{Store: NewDotSet(), Context: New()}
			},
			check: func(c *qt.C, result Causal[*DotSet]) {
				c.Assert(result.Store.Has(da), qt.IsTrue)
			},
		},
		{
			name: "OnlyOneHasButObserved",
			a: func() Causal[*DotSet] {
				a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				a.Store.Add(da)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotSet] {
				b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				b.Context.Add(da) // observed but not in store -> removed
				return b
			},
			check: func(c *qt.C, result Causal[*DotSet]) {
				c.Assert(result.Store.Has(da), qt.IsFalse)
			},
		},
		{
			name: "ConcurrentAdds",
			a: func() Causal[*DotSet] {
				a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				a.Store.Add(da)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotSet] {
				b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				b.Store.Add(db)
				b.Context.Add(db)
				return b
			},
			check: func(c *qt.C, result Causal[*DotSet]) {
				c.Assert(result.Store.Has(da), qt.IsTrue)
				c.Assert(result.Store.Has(db), qt.IsTrue)
			},
		},
		{
			name: "BothEmpty",
			a: func() Causal[*DotSet] {
				return Causal[*DotSet]{Store: NewDotSet(), Context: New()}
			},
			b: func() Causal[*DotSet] {
				return Causal[*DotSet]{Store: NewDotSet(), Context: New()}
			},
			check: func(c *qt.C, result Causal[*DotSet]) {
				c.Assert(result.Store.Len(), qt.Equals, 0)
			},
		},
		{
			name: "NonEmptyWithEmpty",
			a: func() Causal[*DotSet] {
				a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				a.Store.Add(da)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotSet] {
				return Causal[*DotSet]{Store: NewDotSet(), Context: New()}
			},
			check: func(c *qt.C, result Causal[*DotSet]) {
				// a join empty -- dot survives (not in empty's context).
				c.Assert(result.Store.Has(da), qt.IsTrue)
				// empty join a -- same by commutativity.
				// (Verified by re-joining in reverse order.)
			},
		},
		{
			name: "ContextMerged",
			a: func() Causal[*DotSet] {
				a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				a.Store.Add(da)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotSet] {
				b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
				b.Context.Add(db) // observed but removed
				return b
			},
			check: func(c *qt.C, result Causal[*DotSet]) {
				// Output context should contain both.
				c.Assert(result.Context.Has(da), qt.IsTrue)
				c.Assert(result.Context.Has(db), qt.IsTrue)
			},
		},
	}

	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			a := tc.a()
			b := tc.b()
			result := JoinDotSet(a, b)
			tc.check(c, result)
		})
	}

	// NonEmptyWithEmpty: also verify commutativity (empty join a).
	c.Run("NonEmptyWithEmptyReverse", func(c *qt.C) {
		a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
		a.Store.Add(da)
		a.Context.Add(da)
		empty := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
		result := JoinDotSet(empty, a)
		c.Assert(result.Store.Has(da), qt.IsTrue)
	})
}

// --- JoinDotFun table-driven tests ---

func TestJoinDotFun(t *testing.T) {
	c := qt.New(t)

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	type testCase struct {
		name  string
		a     func() Causal[*DotFun[maxInt]]
		b     func() Causal[*DotFun[maxInt]]
		check func(c *qt.C, result Causal[*DotFun[maxInt]])
	}

	tests := []testCase{
		{
			name: "SharedDot",
			a: func() Causal[*DotFun[maxInt]] {
				a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
				a.Store.Set(da, 3)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotFun[maxInt]] {
				b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
				b.Store.Set(da, 7)
				b.Context.Add(da)
				return b
			},
			check: func(c *qt.C, result Causal[*DotFun[maxInt]]) {
				v, ok := result.Store.Get(da)
				c.Assert(ok, qt.IsTrue)
				c.Assert(v, qt.Equals, maxInt(7))
			},
		},
		{
			name: "Disjoint",
			a: func() Causal[*DotFun[maxInt]] {
				a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
				a.Store.Set(da, 10)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotFun[maxInt]] {
				b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
				b.Store.Set(db, 20)
				b.Context.Add(db)
				return b
			},
			check: func(c *qt.C, result Causal[*DotFun[maxInt]]) {
				va, _ := result.Store.Get(da)
				vb, _ := result.Store.Get(db)
				c.Assert(va, qt.Equals, maxInt(10))
				c.Assert(vb, qt.Equals, maxInt(20))
			},
		},
		{
			name: "Removed",
			a: func() Causal[*DotFun[maxInt]] {
				a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
				a.Store.Set(da, 5)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotFun[maxInt]] {
				b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
				b.Context.Add(da) // observed but not in store -> removed
				return b
			},
			check: func(c *qt.C, result Causal[*DotFun[maxInt]]) {
				_, ok := result.Store.Get(da)
				c.Assert(ok, qt.IsFalse)
			},
		},
		{
			name: "BothEmpty",
			a: func() Causal[*DotFun[maxInt]] {
				return Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
			},
			b: func() Causal[*DotFun[maxInt]] {
				return Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
			},
			check: func(c *qt.C, result Causal[*DotFun[maxInt]]) {
				c.Assert(result.Store.Len(), qt.Equals, 0)
			},
		},
		{
			name: "NonEmptyWithEmpty",
			a: func() Causal[*DotFun[maxInt]] {
				a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
				a.Store.Set(da, 42)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotFun[maxInt]] {
				return Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
			},
			check: func(c *qt.C, result Causal[*DotFun[maxInt]]) {
				v, ok := result.Store.Get(da)
				c.Assert(ok, qt.IsTrue)
				c.Assert(v, qt.Equals, maxInt(42))
			},
		},
	}

	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			a := tc.a()
			b := tc.b()
			result := JoinDotFun(a, b)
			tc.check(c, result)
		})
	}
}

// --- JoinDotMap table-driven tests ---

func TestJoinDotMap(t *testing.T) {
	c := qt.New(t)

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}
	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] { return JoinDotSet(x, y) }

	type testCase struct {
		name  string
		a     func() Causal[*DotMap[string, *DotSet]]
		b     func() Causal[*DotMap[string, *DotSet]]
		check func(c *qt.C, result Causal[*DotMap[string, *DotSet]])
	}

	tests := []testCase{
		{
			name: "SharedKey",
			a: func() Causal[*DotMap[string, *DotSet]] {
				a := Causal[*DotMap[string, *DotSet]]{
					Store:   NewDotMap[string, *DotSet](),
					Context: New(),
				}
				sa := NewDotSet()
				sa.Add(da)
				a.Store.Set("key", sa)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotMap[string, *DotSet]] {
				b := Causal[*DotMap[string, *DotSet]]{
					Store:   NewDotMap[string, *DotSet](),
					Context: New(),
				}
				sb := NewDotSet()
				sb.Add(db)
				b.Store.Set("key", sb)
				b.Context.Add(db)
				return b
			},
			check: func(c *qt.C, result Causal[*DotMap[string, *DotSet]]) {
				v, ok := result.Store.Get("key")
				c.Assert(ok, qt.IsTrue)
				c.Assert(v.Has(da), qt.IsTrue)
				c.Assert(v.Has(db), qt.IsTrue)
			},
		},
		{
			name: "KeyOnlyOneSide",
			a: func() Causal[*DotMap[string, *DotSet]] {
				a := Causal[*DotMap[string, *DotSet]]{
					Store:   NewDotMap[string, *DotSet](),
					Context: New(),
				}
				sa := NewDotSet()
				sa.Add(da)
				a.Store.Set("key", sa)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotMap[string, *DotSet]] {
				return Causal[*DotMap[string, *DotSet]]{
					Store:   NewDotMap[string, *DotSet](),
					Context: New(),
				}
			},
			check: func(c *qt.C, result Causal[*DotMap[string, *DotSet]]) {
				v, ok := result.Store.Get("key")
				c.Assert(ok, qt.IsTrue)
				c.Assert(v.Has(da), qt.IsTrue)
			},
		},
		{
			name: "BothEmpty",
			a: func() Causal[*DotMap[string, *DotSet]] {
				return Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
			},
			b: func() Causal[*DotMap[string, *DotSet]] {
				return Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
			},
			check: func(c *qt.C, result Causal[*DotMap[string, *DotSet]]) {
				c.Assert(result.Store.Len(), qt.Equals, 0)
			},
		},
		{
			name: "KeyRemovedByContext",
			a: func() Causal[*DotMap[string, *DotSet]] {
				// a has key "x" with dot d.
				a := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
				sa := NewDotSet()
				sa.Add(da)
				a.Store.Set("x", sa)
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotMap[string, *DotSet]] {
				// b has no keys but has observed d (removal).
				b := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
				b.Context.Add(da)
				return b
			},
			check: func(c *qt.C, result Causal[*DotMap[string, *DotSet]]) {
				// Key "x" should be gone -- its only dot is in b's context.
				_, ok := result.Store.Get("x")
				c.Assert(ok, qt.IsFalse)
			},
		},
		{
			name: "KeyOnlyInB",
			a: func() Causal[*DotMap[string, *DotSet]] {
				return Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
			},
			b: func() Causal[*DotMap[string, *DotSet]] {
				b := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
				sb := NewDotSet()
				sb.Add(db)
				b.Store.Set("y", sb)
				b.Context.Add(db)
				return b
			},
			check: func(c *qt.C, result Causal[*DotMap[string, *DotSet]]) {
				v, ok := result.Store.Get("y")
				c.Assert(ok, qt.IsTrue)
				c.Assert(v.Has(db), qt.IsTrue)
			},
		},
		{
			name: "ContextMerged",
			a: func() Causal[*DotMap[string, *DotSet]] {
				a := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
				a.Context.Add(da)
				return a
			},
			b: func() Causal[*DotMap[string, *DotSet]] {
				b := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
				b.Context.Add(db)
				return b
			},
			check: func(c *qt.C, result Causal[*DotMap[string, *DotSet]]) {
				c.Assert(result.Context.Has(da), qt.IsTrue)
				c.Assert(result.Context.Has(db), qt.IsTrue)
			},
		},
	}

	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			a := tc.a()
			b := tc.b()
			result := JoinDotMap(a, b, joinDS, NewDotSet)
			tc.check(c, result)
		})
	}
}

// --- JoinDotSet semilattice property tests ---

func TestJoinDotSetSemilattice(t *testing.T) {
	c := qt.New(t)

	c.Run("Idempotent", func(c *qt.C) {
		a := makeTestCausalDotSet([]Dot{{ID: "a", Seq: 1}, {ID: "b", Seq: 2}})
		result := JoinDotSet(a, a)
		c.Assert(dotSetEqual(result.Store, a.Store), qt.IsTrue)
	})

	c.Run("Commutative", func(c *qt.C) {
		a := makeTestCausalDotSet([]Dot{{ID: "a", Seq: 1}})
		b := makeTestCausalDotSet([]Dot{{ID: "b", Seq: 1}})

		ab := JoinDotSet(a, b)
		ba := JoinDotSet(b, a)
		c.Assert(dotSetEqual(ab.Store, ba.Store), qt.IsTrue)
	})

	c.Run("Associative", func(c *qt.C) {
		a := makeTestCausalDotSet([]Dot{{ID: "a", Seq: 1}})
		b := makeTestCausalDotSet([]Dot{{ID: "b", Seq: 1}})
		x := makeTestCausalDotSet([]Dot{{ID: "c", Seq: 1}})

		ab_x := JoinDotSet(JoinDotSet(a, b), x)
		a_bx := JoinDotSet(a, JoinDotSet(b, x))
		c.Assert(dotSetEqual(ab_x.Store, a_bx.Store), qt.IsTrue)
	})
}

// --- JoinDotFun associativity (hand-crafted) ---

func TestJoinDotFunAssociative(t *testing.T) {
	c := qt.New(t)
	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}
	dc := Dot{ID: "c", Seq: 1}

	a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	a.Store.Set(da, 10)
	a.Context.Add(da)

	b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	b.Store.Set(db, 20)
	b.Context.Add(db)
	b.Context.Add(da) // b has observed a's dot (simulates remove)

	x := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
	x.Store.Set(dc, 30)
	x.Context.Add(dc)

	// (a join b) join c
	ab := JoinDotFun(a, b)
	abc := JoinDotFun(ab, x)

	// a join (b join c)
	bc := JoinDotFun(b, x)
	abc2 := JoinDotFun(a, bc)

	c.Assert(abc.Store.Len(), qt.Equals, abc2.Store.Len())
	abc.Store.Range(func(d Dot, v maxInt) bool {
		v2, ok := abc2.Store.Get(d)
		c.Assert(ok, qt.IsTrue, qt.Commentf("dot %v missing", d))
		c.Assert(v, qt.Equals, v2, qt.Commentf("dot %v value mismatch", d))
		return true
	})
}

// --- JoinDotMap associativity (hand-crafted) ---

func TestJoinDotMapAssociative(t *testing.T) {
	c := qt.New(t)
	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] { return JoinDotSet(x, y) }

	makeSide := func(key string, d Dot, extraCtx ...Dot) Causal[*DotMap[string, *DotSet]] {
		dm := NewDotMap[string, *DotSet]()
		ds := NewDotSet()
		ds.Add(d)
		dm.Set(key, ds)
		ctx := New()
		ctx.Add(d)
		for _, ed := range extraCtx {
			ctx.Add(ed)
		}
		return Causal[*DotMap[string, *DotSet]]{Store: dm, Context: ctx}
	}

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}
	dc := Dot{ID: "c", Seq: 1}

	a := makeSide("x", da)
	b := makeSide("x", db, da) // b observed a's dot
	x := makeSide("y", dc)     // different key

	// (a join b) join c
	ab := JoinDotMap(a, b, joinDS, NewDotSet)
	abc := JoinDotMap(ab, x, joinDS, NewDotSet)

	// a join (b join c)
	bc := JoinDotMap(b, x, joinDS, NewDotSet)
	abc2 := JoinDotMap(a, bc, joinDS, NewDotSet)

	// Compare keys.
	abcKeys := abc.Store.Keys()
	abc2Keys := abc2.Store.Keys()
	c.Assert(len(abcKeys), qt.Equals, len(abc2Keys))

	// Compare dot contents per key.
	abc.Store.Range(func(k string, v *DotSet) bool {
		v2, ok := abc2.Store.Get(k)
		c.Assert(ok, qt.IsTrue, qt.Commentf("key %q missing", k))
		c.Assert(dotSetEqual(v, v2), qt.IsTrue, qt.Commentf("key %q dots differ", k))
		return true
	})
}

// --- Large store stress tests ---

func TestJoinLargeStore(t *testing.T) {
	c := qt.New(t)

	c.Run("DotSet", func(c *qt.C) {
		const n = 1000

		// Side a: replicas r0..r9, each with 100 contiguous dots.
		a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
		for r := 0; r < 10; r++ {
			id := ReplicaID(fmt.Sprintf("r%d", r))
			for s := uint64(1); s <= uint64(n/10); s++ {
				d := Dot{ID: id, Seq: s}
				a.Store.Add(d)
				a.Context.Add(d)
			}
		}

		// Side b: overlaps on r0..r4 (same dots), plus r10..r14 (new).
		b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
		for r := 0; r < 10; r++ {
			id := ReplicaID(fmt.Sprintf("r%d", r+5))
			for s := uint64(1); s <= uint64(n/10); s++ {
				d := Dot{ID: id, Seq: s}
				b.Store.Add(d)
				b.Context.Add(d)
			}
		}

		result := JoinDotSet(a, b)

		// All dots from both sides should survive (no removes -- both
		// sides only add, with partial overlap).
		c.Assert(result.Store.Len() > n, qt.IsTrue,
			qt.Commentf("expected >%d dots, got %d", n, result.Store.Len()))

		// Idempotent: joining again should not change result.
		result2 := JoinDotSet(result, result)
		c.Assert(dotSetEqual(result.Store, result2.Store), qt.IsTrue)
	})

	c.Run("DotFun", func(c *qt.C) {
		const n = 1000

		a := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
		b := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}

		for i := uint64(1); i <= n; i++ {
			da := Dot{ID: "a", Seq: i}
			a.Store.Set(da, maxInt(i))
			a.Context.Add(da)

			db := Dot{ID: "b", Seq: i}
			b.Store.Set(db, maxInt(i*10))
			b.Context.Add(db)
		}

		result := JoinDotFun(a, b)

		// All dots from both sides should survive (disjoint replicas).
		c.Assert(result.Store.Len(), qt.Equals, 2*n)

		// Spot check values.
		va, ok := result.Store.Get(Dot{ID: "a", Seq: 500})
		c.Assert(ok, qt.IsTrue)
		c.Assert(va, qt.Equals, maxInt(500))

		vb, ok := result.Store.Get(Dot{ID: "b", Seq: 500})
		c.Assert(ok, qt.IsTrue)
		c.Assert(vb, qt.Equals, maxInt(5000))
	})

	c.Run("DotMap", func(c *qt.C) {
		const n = 1000
		joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] { return JoinDotSet(x, y) }

		a := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}
		b := Causal[*DotMap[string, *DotSet]]{Store: NewDotMap[string, *DotSet](), Context: New()}

		for i := uint64(1); i <= n; i++ {
			key := fmt.Sprintf("k%d", i)

			da := Dot{ID: "a", Seq: i}
			dsA := NewDotSet()
			dsA.Add(da)
			a.Store.Set(key, dsA)
			a.Context.Add(da)

			db := Dot{ID: "b", Seq: i}
			dsB := NewDotSet()
			dsB.Add(db)
			b.Store.Set(key, dsB)
			b.Context.Add(db)
		}

		result := JoinDotMap(a, b, joinDS, NewDotSet)

		// Each key should have 2 dots (one from each side).
		c.Assert(result.Store.Len(), qt.Equals, n)
		result.Store.Range(func(k string, ds *DotSet) bool {
			c.Assert(ds.Len(), qt.Equals, 2, qt.Commentf("key %s", k))
			return true
		})
	})
}
