package dotcontext

import (
	"testing"
)

// --- Parsing helpers ---
// Fuzz bytes → CRDT structures. Encoding per dot:
//   3 bytes: (replica_index, sequence, flags)
//   replica = ["a","b","c"][byte % 3]
//   seq     = byte % 8 + 1  (range 1..8)
//   flags   = bit0: in store, bit1: in context (forced if bit0)

var fuzzReplicas = []ReplicaID{"a", "b", "c"}

func parseCausalDotSet(data []byte) Causal[*DotSet] {
	ds := NewDotSet()
	ctx := New()
	for i := 0; i+2 < len(data); i += 3 {
		id := fuzzReplicas[data[i]%3]
		seq := uint64(data[i+1]%8) + 1
		flags := data[i+2]
		d := Dot{ID: id, Seq: seq}
		if flags&1 != 0 {
			ds.Add(d)
			ctx.Add(d)
		} else if flags&2 != 0 {
			ctx.Add(d)
		}
	}
	ctx.Compact()
	return Causal[*DotSet]{Store: ds, Context: ctx}
}

type maxVal struct{ n int64 }

func (m maxVal) Join(other maxVal) maxVal {
	if other.n > m.n {
		return other
	}
	return m
}

func parseCausalDotFun(data []byte) Causal[*DotFun[maxVal]] {
	df := NewDotFun[maxVal]()
	ctx := New()
	for i := 0; i+3 < len(data); i += 4 {
		id := fuzzReplicas[data[i]%3]
		seq := uint64(data[i+1]%8) + 1
		flags := data[i+2]
		val := int64(data[i+3]) - 128
		d := Dot{ID: id, Seq: seq}
		if flags&1 != 0 {
			df.Set(d, maxVal{n: val})
			ctx.Add(d)
		} else if flags&2 != 0 {
			ctx.Add(d)
		}
	}
	ctx.Compact()
	return Causal[*DotFun[maxVal]]{Store: df, Context: ctx}
}

func parseCausalDotMap(data []byte) Causal[*DotMap[string, *DotSet]] {
	dm := NewDotMap[string, *DotSet]()
	ctx := New()
	keys := []string{"x", "y", "z"}
	for i := 0; i+3 < len(data); i += 4 {
		key := keys[data[i]%3]
		id := fuzzReplicas[data[i+1]%3]
		seq := uint64(data[i+2]%8) + 1
		flags := data[i+3]
		d := Dot{ID: id, Seq: seq}
		if flags&1 != 0 {
			ds, ok := dm.Get(key)
			if !ok {
				ds = NewDotSet()
				dm.Set(key, ds)
			}
			ds.Add(d)
			ctx.Add(d)
		} else if flags&2 != 0 {
			ctx.Add(d)
		}
	}
	ctx.Compact()
	return Causal[*DotMap[string, *DotSet]]{Store: dm, Context: ctx}
}

// --- Clone helpers ---

func cloneCausalDotSet(c Causal[*DotSet]) Causal[*DotSet] {
	return Causal[*DotSet]{Store: c.Store.Clone(), Context: c.Context.Clone()}
}

func cloneCausalDotFun(c Causal[*DotFun[maxVal]]) Causal[*DotFun[maxVal]] {
	return Causal[*DotFun[maxVal]]{Store: c.Store.Clone(), Context: c.Context.Clone()}
}

func cloneCausalDotMap(c Causal[*DotMap[string, *DotSet]]) Causal[*DotMap[string, *DotSet]] {
	store := NewDotMap[string, *DotSet]()
	c.Store.Range(func(k string, v *DotSet) bool {
		store.Set(k, v.Clone())
		return true
	})
	return Causal[*DotMap[string, *DotSet]]{Store: store, Context: c.Context.Clone()}
}

// --- Equality helpers ---

func equalDotSet(a, b *DotSet) bool {
	if a.Len() != b.Len() {
		return false
	}
	eq := true
	a.Range(func(d Dot) bool {
		if !b.Has(d) {
			eq = false
			return false
		}
		return true
	})
	return eq
}

func equalContext(a, b *CausalContext) bool {
	if len(a.vv) != len(b.vv) {
		return false
	}
	for id, seq := range a.vv {
		if b.vv[id] != seq {
			return false
		}
	}
	if len(a.outliers) != len(b.outliers) {
		return false
	}
	for d := range a.outliers {
		if _, ok := b.outliers[d]; !ok {
			return false
		}
	}
	return true
}

func equalCausalDotSet(a, b Causal[*DotSet]) bool {
	return equalDotSet(a.Store, b.Store) && equalContext(a.Context, b.Context)
}

func equalDotFun(a, b *DotFun[maxVal]) bool {
	if a.Len() != b.Len() {
		return false
	}
	eq := true
	a.Range(func(d Dot, va maxVal) bool {
		vb, ok := b.Get(d)
		if !ok || va.n != vb.n {
			eq = false
			return false
		}
		return true
	})
	return eq
}

func equalCausalDotFun(a, b Causal[*DotFun[maxVal]]) bool {
	return equalDotFun(a.Store, b.Store) && equalContext(a.Context, b.Context)
}

func equalDotMap(a, b *DotMap[string, *DotSet]) bool {
	if a.Len() != b.Len() {
		return false
	}
	eq := true
	a.Range(func(k string, va *DotSet) bool {
		vb, ok := b.Get(k)
		if !ok || !equalDotSet(va, vb) {
			eq = false
			return false
		}
		return true
	})
	return eq
}

func equalCausalDotMap(a, b Causal[*DotMap[string, *DotSet]]) bool {
	return equalDotMap(a.Store, b.Store) && equalContext(a.Context, b.Context)
}

// --- Fuzz targets ---

func FuzzJoinDotSetSemilattice(f *testing.F) {
	f.Add([]byte{}, []byte{}, []byte{})
	f.Add([]byte{0, 0, 3}, []byte{0, 0, 3}, []byte{0, 0, 3})
	f.Add([]byte{0, 0, 3}, []byte{1, 0, 3}, []byte{2, 0, 3})
	f.Add([]byte{0, 0, 3, 0, 1, 2}, []byte{0, 0, 2, 0, 1, 3}, []byte{1, 0, 3})

	f.Fuzz(func(t *testing.T, dataA, dataB, dataC []byte) {
		a := parseCausalDotSet(dataA)
		b := parseCausalDotSet(dataB)
		c := parseCausalDotSet(dataC)

		// Idempotent: join(a, a) == a
		aa := JoinDotSet(cloneCausalDotSet(a), cloneCausalDotSet(a))
		if !equalCausalDotSet(a, aa) {
			t.Fatal("idempotent violation")
		}

		// Commutative: join(a, b) == join(b, a)
		ab := JoinDotSet(cloneCausalDotSet(a), cloneCausalDotSet(b))
		ba := JoinDotSet(cloneCausalDotSet(b), cloneCausalDotSet(a))
		if !equalCausalDotSet(ab, ba) {
			t.Fatal("commutative violation")
		}

		// Associative: join(join(a, b), c) == join(a, join(b, c))
		ab_c := JoinDotSet(
			JoinDotSet(cloneCausalDotSet(a), cloneCausalDotSet(b)),
			cloneCausalDotSet(c),
		)
		a_bc := JoinDotSet(
			cloneCausalDotSet(a),
			JoinDotSet(cloneCausalDotSet(b), cloneCausalDotSet(c)),
		)
		if !equalCausalDotSet(ab_c, a_bc) {
			t.Fatal("associative violation")
		}
	})
}

func FuzzJoinDotFunSemilattice(f *testing.F) {
	f.Add([]byte{}, []byte{}, []byte{})
	f.Add([]byte{0, 0, 3, 50}, []byte{0, 0, 3, 50}, []byte{0, 0, 3, 50})
	f.Add([]byte{0, 0, 3, 100}, []byte{1, 0, 3, 200}, []byte{2, 0, 3, 0})

	f.Fuzz(func(t *testing.T, dataA, dataB, dataC []byte) {
		a := parseCausalDotFun(dataA)
		b := parseCausalDotFun(dataB)
		c := parseCausalDotFun(dataC)

		// Idempotent
		aa := JoinDotFun(cloneCausalDotFun(a), cloneCausalDotFun(a))
		if !equalCausalDotFun(a, aa) {
			t.Fatal("idempotent violation")
		}

		// Commutative
		ab := JoinDotFun(cloneCausalDotFun(a), cloneCausalDotFun(b))
		ba := JoinDotFun(cloneCausalDotFun(b), cloneCausalDotFun(a))
		if !equalCausalDotFun(ab, ba) {
			t.Fatal("commutative violation")
		}

		// Associative
		ab_c := JoinDotFun(
			JoinDotFun(cloneCausalDotFun(a), cloneCausalDotFun(b)),
			cloneCausalDotFun(c),
		)
		a_bc := JoinDotFun(
			cloneCausalDotFun(a),
			JoinDotFun(cloneCausalDotFun(b), cloneCausalDotFun(c)),
		)
		if !equalCausalDotFun(ab_c, a_bc) {
			t.Fatal("associative violation")
		}
	})
}

func FuzzJoinDotMapSemilattice(f *testing.F) {
	f.Add([]byte{}, []byte{}, []byte{})
	f.Add([]byte{0, 0, 0, 3}, []byte{0, 0, 0, 3}, []byte{0, 0, 0, 3})
	f.Add([]byte{0, 0, 0, 3}, []byte{1, 1, 0, 3}, []byte{2, 2, 0, 3})

	joinDS := func(x, y Causal[*DotSet]) Causal[*DotSet] {
		return JoinDotSet(x, y)
	}

	f.Fuzz(func(t *testing.T, dataA, dataB, dataC []byte) {
		a := parseCausalDotMap(dataA)
		b := parseCausalDotMap(dataB)
		c := parseCausalDotMap(dataC)

		// Idempotent
		aa := JoinDotMap(cloneCausalDotMap(a), cloneCausalDotMap(a), joinDS, NewDotSet)
		if !equalCausalDotMap(a, aa) {
			t.Fatal("idempotent violation")
		}

		// Commutative
		ab := JoinDotMap(cloneCausalDotMap(a), cloneCausalDotMap(b), joinDS, NewDotSet)
		ba := JoinDotMap(cloneCausalDotMap(b), cloneCausalDotMap(a), joinDS, NewDotSet)
		if !equalCausalDotMap(ab, ba) {
			t.Fatal("commutative violation")
		}

		// Associative
		ab_c := JoinDotMap(
			JoinDotMap(cloneCausalDotMap(a), cloneCausalDotMap(b), joinDS, NewDotSet),
			cloneCausalDotMap(c),
			joinDS, NewDotSet,
		)
		a_bc := JoinDotMap(
			cloneCausalDotMap(a),
			JoinDotMap(cloneCausalDotMap(b), cloneCausalDotMap(c), joinDS, NewDotSet),
			joinDS, NewDotSet,
		)
		if !equalCausalDotMap(ab_c, a_bc) {
			t.Fatal("associative violation")
		}
	})
}
