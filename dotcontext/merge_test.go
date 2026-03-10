package dotcontext

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
)

// --- MergeDotSetStore ---

func TestMergeDotSetStore(t *testing.T) {
	c := qt.New(t)

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	c.Run("BothHave", func(c *qt.C) {
		// Intersection: dot survives.
		state := NewDotSet()
		state.Add(da)
		ctxS := New()
		ctxS.Add(da)

		delta := NewDotSet()
		delta.Add(da)
		ctxD := New()
		ctxD.Add(da)

		MergeDotSetStore(state, delta, ctxS, ctxD)
		c.Assert(state.Has(da), qt.IsTrue)
		c.Assert(state.Len(), qt.Equals, 1)
	})

	c.Run("OnlyStateNotObserved", func(c *qt.C) {
		// State has dot, delta hasn't observed it → survives.
		state := NewDotSet()
		state.Add(da)
		ctxS := New()
		ctxS.Add(da)

		delta := NewDotSet()
		ctxD := New()

		MergeDotSetStore(state, delta, ctxS, ctxD)
		c.Assert(state.Has(da), qt.IsTrue)
	})

	c.Run("OnlyStateObserved", func(c *qt.C) {
		// State has dot, delta observed it (removed) → gone.
		state := NewDotSet()
		state.Add(da)
		ctxS := New()
		ctxS.Add(da)

		delta := NewDotSet()
		ctxD := New()
		ctxD.Add(da)

		MergeDotSetStore(state, delta, ctxS, ctxD)
		c.Assert(state.Has(da), qt.IsFalse)
		c.Assert(state.Len(), qt.Equals, 0)
	})

	c.Run("OnlyDeltaNotObserved", func(c *qt.C) {
		// Delta has dot, state hasn't observed it → added.
		state := NewDotSet()
		ctxS := New()

		delta := NewDotSet()
		delta.Add(db)
		ctxD := New()
		ctxD.Add(db)

		MergeDotSetStore(state, delta, ctxS, ctxD)
		c.Assert(state.Has(db), qt.IsTrue)
	})

	c.Run("OnlyDeltaObserved", func(c *qt.C) {
		// Delta has dot, state already observed it → not added.
		state := NewDotSet()
		ctxS := New()
		ctxS.Add(db) // observed but not in store

		delta := NewDotSet()
		delta.Add(db)
		ctxD := New()
		ctxD.Add(db)

		MergeDotSetStore(state, delta, ctxS, ctxD)
		c.Assert(state.Has(db), qt.IsFalse)
	})

	c.Run("ConcurrentAdds", func(c *qt.C) {
		state := NewDotSet()
		state.Add(da)
		ctxS := New()
		ctxS.Add(da)

		delta := NewDotSet()
		delta.Add(db)
		ctxD := New()
		ctxD.Add(db)

		MergeDotSetStore(state, delta, ctxS, ctxD)
		c.Assert(state.Has(da), qt.IsTrue)
		c.Assert(state.Has(db), qt.IsTrue)
	})

	c.Run("BothEmpty", func(c *qt.C) {
		state := NewDotSet()
		ctxS := New()
		delta := NewDotSet()
		ctxD := New()

		MergeDotSetStore(state, delta, ctxS, ctxD)
		c.Assert(state.Len(), qt.Equals, 0)
	})
}

// --- MergeDotFunStore ---

func TestMergeDotFunStore(t *testing.T) {
	c := qt.New(t)

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	c.Run("SharedDot", func(c *qt.C) {
		state := NewDotFun[maxInt]()
		state.Set(da, 3)
		ctxS := New()
		ctxS.Add(da)

		delta := NewDotFun[maxInt]()
		delta.Set(da, 7)
		ctxD := New()
		ctxD.Add(da)

		MergeDotFunStore(state, delta, ctxS, ctxD)
		v, ok := state.Get(da)
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, maxInt(7)) // max(3,7) = 7
	})

	c.Run("Disjoint", func(c *qt.C) {
		state := NewDotFun[maxInt]()
		state.Set(da, 10)
		ctxS := New()
		ctxS.Add(da)

		delta := NewDotFun[maxInt]()
		delta.Set(db, 20)
		ctxD := New()
		ctxD.Add(db)

		MergeDotFunStore(state, delta, ctxS, ctxD)
		va, _ := state.Get(da)
		vb, _ := state.Get(db)
		c.Assert(va, qt.Equals, maxInt(10))
		c.Assert(vb, qt.Equals, maxInt(20))
	})

	c.Run("Removed", func(c *qt.C) {
		state := NewDotFun[maxInt]()
		state.Set(da, 5)
		ctxS := New()
		ctxS.Add(da)

		delta := NewDotFun[maxInt]()
		ctxD := New()
		ctxD.Add(da) // observed but not in store → removed

		MergeDotFunStore(state, delta, ctxS, ctxD)
		_, ok := state.Get(da)
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("BothEmpty", func(c *qt.C) {
		state := NewDotFun[maxInt]()
		ctxS := New()
		delta := NewDotFun[maxInt]()
		ctxD := New()

		MergeDotFunStore(state, delta, ctxS, ctxD)
		c.Assert(state.Len(), qt.Equals, 0)
	})
}

// --- MergeDotSet (Causal-level) ---

func TestMergeDotSet(t *testing.T) {
	c := qt.New(t)

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	c.Run("StoreAndContextMerged", func(c *qt.C) {
		state := &Causal[*DotSet]{Store: NewDotSet(), Context: New()}
		state.Store.Add(da)
		state.Context.Add(da)

		delta := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
		delta.Context.Add(db) // observed but removed

		MergeDotSet(state, delta)

		c.Assert(state.Store.Has(da), qt.IsTrue)
		c.Assert(state.Context.Has(da), qt.IsTrue)
		c.Assert(state.Context.Has(db), qt.IsTrue)
	})
}

// --- MergeDotFun (Causal-level) ---

func TestMergeDotFun(t *testing.T) {
	c := qt.New(t)

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	c.Run("StoreAndContextMerged", func(c *qt.C) {
		state := &Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
		state.Store.Set(da, 42)
		state.Context.Add(da)

		delta := Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: New()}
		delta.Store.Set(db, 99)
		delta.Context.Add(db)

		MergeDotFun(state, delta)

		va, _ := state.Store.Get(da)
		vb, _ := state.Store.Get(db)
		c.Assert(va, qt.Equals, maxInt(42))
		c.Assert(vb, qt.Equals, maxInt(99))
		c.Assert(state.Context.Has(da), qt.IsTrue)
		c.Assert(state.Context.Has(db), qt.IsTrue)
	})
}

// --- MergeDotMap (Causal-level) ---

func TestMergeDotMap(t *testing.T) {
	c := qt.New(t)

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	c.Run("SharedKey", func(c *qt.C) {
		state := &Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		sa := NewDotSet()
		sa.Add(da)
		state.Store.Set("key", sa)
		state.Context.Add(da)

		delta := Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		sb := NewDotSet()
		sb.Add(db)
		delta.Store.Set("key", sb)
		delta.Context.Add(db)

		MergeDotMap(state, delta, MergeDotSetStore, NewDotSet)

		v, ok := state.Store.Get("key")
		c.Assert(ok, qt.IsTrue)
		c.Assert(v.Has(da), qt.IsTrue)
		c.Assert(v.Has(db), qt.IsTrue)
	})

	c.Run("KeyOnlyInState", func(c *qt.C) {
		state := &Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		sa := NewDotSet()
		sa.Add(da)
		state.Store.Set("key", sa)
		state.Context.Add(da)

		delta := Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}

		MergeDotMap(state, delta, MergeDotSetStore, NewDotSet)

		v, ok := state.Store.Get("key")
		c.Assert(ok, qt.IsTrue)
		c.Assert(v.Has(da), qt.IsTrue)
	})

	c.Run("KeyRemovedByContext", func(c *qt.C) {
		state := &Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		sa := NewDotSet()
		sa.Add(da)
		state.Store.Set("x", sa)
		state.Context.Add(da)

		delta := Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		delta.Context.Add(da) // observed but removed

		MergeDotMap(state, delta, MergeDotSetStore, NewDotSet)

		_, ok := state.Store.Get("x")
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("KeyOnlyInDelta", func(c *qt.C) {
		state := &Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}

		delta := Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		sb := NewDotSet()
		sb.Add(db)
		delta.Store.Set("y", sb)
		delta.Context.Add(db)

		MergeDotMap(state, delta, MergeDotSetStore, NewDotSet)

		v, ok := state.Store.Get("y")
		c.Assert(ok, qt.IsTrue)
		c.Assert(v.Has(db), qt.IsTrue)
	})

	c.Run("ContextMerged", func(c *qt.C) {
		state := &Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		state.Context.Add(da)

		delta := Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		delta.Context.Add(db)

		MergeDotMap(state, delta, MergeDotSetStore, NewDotSet)

		c.Assert(state.Context.Has(da), qt.IsTrue)
		c.Assert(state.Context.Has(db), qt.IsTrue)
	})

	c.Run("BothEmpty", func(c *qt.C) {
		state := &Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		delta := Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}

		MergeDotMap(state, delta, MergeDotSetStore, NewDotSet)

		c.Assert(state.Store.Len(), qt.Equals, 0)
	})
}

// --- Equivalence: Merge produces the same result as Join ---

func TestMergeEqualsJoinDotSet(t *testing.T) {
	c := qt.New(t)

	type testCase struct {
		name string
		a    func() Causal[*DotSet]
		b    func() Causal[*DotSet]
	}

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}
	dc := Dot{ID: "c", Seq: 1}

	tests := []testCase{
		{
			name: "BothHave",
			a:    func() Causal[*DotSet] { return makeTestCausalDotSet([]Dot{da}) },
			b:    func() Causal[*DotSet] { return makeTestCausalDotSet([]Dot{da}) },
		},
		{
			name: "ConcurrentAdds",
			a:    func() Causal[*DotSet] { return makeTestCausalDotSet([]Dot{da}) },
			b:    func() Causal[*DotSet] { return makeTestCausalDotSet([]Dot{db}) },
		},
		{
			name: "OneSideRemoved",
			a: func() Causal[*DotSet] { return makeTestCausalDotSet([]Dot{da}) },
			b: func() Causal[*DotSet] {
				cc := New()
				cc.Add(da) // observed but not in store
				return Causal[*DotSet]{Store: NewDotSet(), Context: cc}
			},
		},
		{
			name: "ThreeDots",
			a:    func() Causal[*DotSet] { return makeTestCausalDotSet([]Dot{da, db}) },
			b:    func() Causal[*DotSet] { return makeTestCausalDotSet([]Dot{db, dc}) },
		},
		{
			name: "BothEmpty",
			a: func() Causal[*DotSet] {
				return Causal[*DotSet]{Store: NewDotSet(), Context: New()}
			},
			b: func() Causal[*DotSet] {
				return Causal[*DotSet]{Store: NewDotSet(), Context: New()}
			},
		},
	}

	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			// Allocating join.
			joined := JoinDotSet(tc.a(), tc.b())

			// In-place merge.
			state := tc.a()
			delta := tc.b()
			MergeDotSet(&state, delta)

			c.Assert(dotSetEqual(state.Store, joined.Store), qt.IsTrue,
				qt.Commentf("store mismatch"))
			c.Assert(state.Context.Has(Dot{ID: "a", Seq: 1}),
				qt.Equals, joined.Context.Has(Dot{ID: "a", Seq: 1}))
			c.Assert(state.Context.Has(Dot{ID: "b", Seq: 1}),
				qt.Equals, joined.Context.Has(Dot{ID: "b", Seq: 1}))
			c.Assert(state.Context.Has(Dot{ID: "c", Seq: 1}),
				qt.Equals, joined.Context.Has(Dot{ID: "c", Seq: 1}))
		})
	}
}

func TestMergeEqualsJoinDotFun(t *testing.T) {
	c := qt.New(t)

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	type testCase struct {
		name string
		a    func() Causal[*DotFun[maxInt]]
		b    func() Causal[*DotFun[maxInt]]
	}

	makeFun := func(pairs ...any) func() Causal[*DotFun[maxInt]] {
		return func() Causal[*DotFun[maxInt]] {
			f := NewDotFun[maxInt]()
			cc := New()
			for i := 0; i < len(pairs); i += 2 {
				d := pairs[i].(Dot)
				v := pairs[i+1].(maxInt)
				f.Set(d, v)
				cc.Add(d)
			}
			return Causal[*DotFun[maxInt]]{Store: f, Context: cc}
		}
	}

	tests := []testCase{
		{
			name: "SharedDot",
			a:    makeFun(da, maxInt(3)),
			b:    makeFun(da, maxInt(7)),
		},
		{
			name: "Disjoint",
			a:    makeFun(da, maxInt(10)),
			b:    makeFun(db, maxInt(20)),
		},
		{
			name: "Removed",
			a:    makeFun(da, maxInt(5)),
			b: func() Causal[*DotFun[maxInt]] {
				cc := New()
				cc.Add(da)
				return Causal[*DotFun[maxInt]]{Store: NewDotFun[maxInt](), Context: cc}
			},
		},
	}

	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			joined := JoinDotFun(tc.a(), tc.b())

			state := tc.a()
			delta := tc.b()
			MergeDotFun(&state, delta)

			c.Assert(state.Store.Len(), qt.Equals, joined.Store.Len())
			state.Store.Range(func(d Dot, v maxInt) bool {
				jv, ok := joined.Store.Get(d)
				c.Assert(ok, qt.IsTrue, qt.Commentf("dot %v missing from join", d))
				c.Assert(v, qt.Equals, jv, qt.Commentf("dot %v value mismatch", d))
				return true
			})
		})
	}
}

func TestMergeEqualsJoinDotMap(t *testing.T) {
	c := qt.New(t)

	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	type testCase struct {
		name string
		a    func() Causal[*DotMap[string, *DotSet]]
		b    func() Causal[*DotMap[string, *DotSet]]
	}

	makeDM := func(entries map[string][]Dot, extraCtx ...Dot) func() Causal[*DotMap[string, *DotSet]] {
		return func() Causal[*DotMap[string, *DotSet]] {
			dm := NewDotMap[string, *DotSet]()
			cc := New()
			for k, dots := range entries {
				ds := NewDotSet()
				for _, d := range dots {
					ds.Add(d)
					cc.Add(d)
				}
				dm.Set(k, ds)
			}
			for _, d := range extraCtx {
				cc.Add(d)
			}
			return Causal[*DotMap[string, *DotSet]]{Store: dm, Context: cc}
		}
	}

	tests := []testCase{
		{
			name: "SharedKey",
			a:    makeDM(map[string][]Dot{"key": {da}}),
			b:    makeDM(map[string][]Dot{"key": {db}}),
		},
		{
			name: "KeyOnlyInA",
			a:    makeDM(map[string][]Dot{"key": {da}}),
			b:    makeDM(nil),
		},
		{
			name: "KeyRemovedByContext",
			a: makeDM(map[string][]Dot{"x": {da}}),
			b: makeDM(nil, da), // observed but removed
		},
		{
			name: "KeyOnlyInB",
			a: makeDM(nil),
			b: makeDM(map[string][]Dot{"y": {db}}),
		},
	}

	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			joined := JoinDotMap(tc.a(), tc.b(), JoinDotSetStore, NewDotSet)

			state := tc.a()
			delta := tc.b()
			MergeDotMap(&state, delta, MergeDotSetStore, NewDotSet)

			c.Assert(state.Store.Len(), qt.Equals, joined.Store.Len(),
				qt.Commentf("key count mismatch"))
			state.Store.Range(func(k string, v *DotSet) bool {
				jv, ok := joined.Store.Get(k)
				c.Assert(ok, qt.IsTrue, qt.Commentf("key %q missing from join", k))
				c.Assert(dotSetEqual(v, jv), qt.IsTrue,
					qt.Commentf("key %q dots differ", k))
				return true
			})
		})
	}
}

// --- Large store: merge small delta into large state ---

func TestMergeLargeStateTinyDelta(t *testing.T) {
	c := qt.New(t)

	c.Run("DotMap", func(c *qt.C) {
		const n = 1000
		// Large state: 1000 keys, each with one dot from replica "a".
		state := &Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		for i := uint64(1); i <= n; i++ {
			d := Dot{ID: "a", Seq: i}
			ds := NewDotSet()
			ds.Add(d)
			state.Store.Set(fmt.Sprintf("k%d", i), ds)
			state.Context.Add(d)
		}

		// Tiny delta: 1 new key from replica "b".
		db := Dot{ID: "b", Seq: 1}
		delta := Causal[*DotMap[string, *DotSet]]{
			Store: NewDotMap[string, *DotSet](), Context: New(),
		}
		deltaDS := NewDotSet()
		deltaDS.Add(db)
		delta.Store.Set("new_key", deltaDS)
		delta.Context.Add(db)

		MergeDotMap(state, delta, MergeDotSetStore, NewDotSet)

		// All original keys survive, new key added.
		c.Assert(state.Store.Len(), qt.Equals, n+1)
		v, ok := state.Store.Get("new_key")
		c.Assert(ok, qt.IsTrue)
		c.Assert(v.Has(db), qt.IsTrue)

		// Spot-check original key.
		v2, ok := state.Store.Get("k500")
		c.Assert(ok, qt.IsTrue)
		c.Assert(v2.Has(Dot{ID: "a", Seq: 500}), qt.IsTrue)
	})
}
