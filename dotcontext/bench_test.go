package dotcontext

import (
	"fmt"
	"testing"
)

// --- CausalContext benchmarks ---

func BenchmarkContextNext(b *testing.B) {
	ctx := New()
	for b.Loop() {
		ctx.Next("a")
	}
}

func BenchmarkContextAdd(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("sequential/%d", n), func(b *testing.B) {
			for b.Loop() {
				ctx := New()
				for i := range n {
					ctx.Add(Dot{ID: "a", Seq: uint64(i + 1)})
				}
			}
		})
		b.Run(fmt.Sprintf("outliers/%d", n), func(b *testing.B) {
			for b.Loop() {
				ctx := New()
				for i := range n {
					// Add in reverse to create outliers.
					ctx.Add(Dot{ID: "a", Seq: uint64(n - i)})
				}
			}
		})
	}
}

func BenchmarkContextHas(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprintf("contiguous/%d", n), func(b *testing.B) {
			ctx := New()
			for i := range n {
				ctx.Add(Dot{ID: "a", Seq: uint64(i + 1)})
			}
			ctx.Compact()
			d := Dot{ID: "a", Seq: uint64(n / 2)}
			for b.Loop() {
				ctx.Has(d)
			}
		})
		b.Run(fmt.Sprintf("outlier/%d", n), func(b *testing.B) {
			ctx := New()
			// Add even numbers as outliers.
			for i := range n {
				ctx.Add(Dot{ID: "a", Seq: uint64(i*2 + 2)})
			}
			d := Dot{ID: "a", Seq: uint64(n)} // an outlier
			for b.Loop() {
				ctx.Has(d)
			}
		})
	}
}

func BenchmarkContextCompact(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			for b.Loop() {
				ctx := New()
				// Add in reverse to create n outliers.
				for i := range n {
					ctx.Add(Dot{ID: "a", Seq: uint64(n - i)})
				}
				ctx.Compact()
			}
		})
	}
}

func BenchmarkContextMerge(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("replicas/%d", n), func(b *testing.B) {
			a := New()
			bb := New()
			for i := range n {
				id := ReplicaID(fmt.Sprintf("r%d", i))
				a.Add(Dot{ID: id, Seq: 1})
				bb.Add(Dot{ID: id, Seq: 2})
			}
			a.Compact()
			bb.Compact()
			for b.Loop() {
				c := a.Clone()
				c.Merge(bb)
			}
		})
	}
}

// --- Join benchmarks ---

func makeCausalDotSet(nReplicas, dotsPerReplica int) Causal[*DotSet] {
	ds := NewDotSet()
	ctx := New()
	for r := range nReplicas {
		id := ReplicaID(fmt.Sprintf("r%d", r))
		for s := range dotsPerReplica {
			d := Dot{ID: id, Seq: uint64(s + 1)}
			ds.Add(d)
			ctx.Add(d)
		}
	}
	ctx.Compact()
	return Causal[*DotSet]{Store: ds, Context: ctx}
}

func BenchmarkJoinDotSet(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("dots/%d", size), func(b *testing.B) {
			a := makeCausalDotSet(1, size)
			bb := makeCausalDotSet(1, size)
			for b.Loop() {
				JoinDotSet(
					Causal[*DotSet]{Store: a.Store.Clone(), Context: a.Context.Clone()},
					Causal[*DotSet]{Store: bb.Store.Clone(), Context: bb.Context.Clone()},
				)
			}
		})
	}
	b.Run("disjoint/2x500", func(b *testing.B) {
		a := makeCausalDotSet(1, 500)
		bb := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
		for i := range 500 {
			d := Dot{ID: "other", Seq: uint64(i + 1)}
			bb.Store.Add(d)
			bb.Context.Add(d)
		}
		bb.Context.Compact()
		for b.Loop() {
			JoinDotSet(
				Causal[*DotSet]{Store: a.Store.Clone(), Context: a.Context.Clone()},
				Causal[*DotSet]{Store: bb.Store.Clone(), Context: bb.Context.Clone()},
			)
		}
	})
}

func BenchmarkJoinDotFun(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("dots/%d", size), func(b *testing.B) {
			makeDF := func(id ReplicaID) Causal[*DotFun[maxVal]] {
				df := NewDotFun[maxVal]()
				ctx := New()
				for s := range size {
					d := Dot{ID: id, Seq: uint64(s + 1)}
					df.Set(d, maxVal{n: int64(s)})
					ctx.Add(d)
				}
				ctx.Compact()
				return Causal[*DotFun[maxVal]]{Store: df, Context: ctx}
			}
			a := makeDF("a")
			bb := makeDF("b")
			for b.Loop() {
				JoinDotFun(
					Causal[*DotFun[maxVal]]{Store: a.Store.Clone(), Context: a.Context.Clone()},
					Causal[*DotFun[maxVal]]{Store: bb.Store.Clone(), Context: bb.Context.Clone()},
				)
			}
		})
	}
}

func BenchmarkJoinDotMap(b *testing.B) {
	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] {
		return JoinDotSet(x, y)
	}
	for _, nKeys := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("keys/%d", nKeys), func(b *testing.B) {
			makeDM := func(id ReplicaID) Causal[*DotMap[string, *DotSet]] {
				dm := NewDotMap[string, *DotSet]()
				ctx := New()
				for k := range nKeys {
					key := fmt.Sprintf("k%d", k)
					ds := NewDotSet()
					d := Dot{ID: id, Seq: uint64(k + 1)}
					ds.Add(d)
					dm.Set(key, ds)
					ctx.Add(d)
				}
				ctx.Compact()
				return Causal[*DotMap[string, *DotSet]]{Store: dm, Context: ctx}
			}
			a := makeDM("a")
			bb := makeDM("b")
			cloneDM := func(c Causal[*DotMap[string, *DotSet]]) Causal[*DotMap[string, *DotSet]] {
				store := NewDotMap[string, *DotSet]()
				c.Store.Range(func(k string, v *DotSet) bool {
					store.Set(k, v.Clone())
					return true
				})
				return Causal[*DotMap[string, *DotSet]]{Store: store, Context: c.Context.Clone()}
			}
			for b.Loop() {
				JoinDotMap(cloneDM(a), cloneDM(bb), joinDS, NewDotSet)
			}
		})
	}
}
