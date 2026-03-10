package ormap

import (
	"slices"
	"testing"

	"github.com/aalpar/crdt/dotcontext"
)

// FuzzORMapConvergence fuzzes random sequences of Apply/Remove operations
// on two replicas, merges both directions, and checks convergence.
//
// Uses ORMap[string, *DotSet] (map of sets). Each Apply adds a single
// dot to the value's DotSet.
//
// Operation encoding: 2 bytes per op.
//
//	byte 0: key = ["x","y","z"][b % 3]
//	byte 1: bit0 = 0 → Apply (add dot), 1 → Remove
func FuzzORMapConvergence(f *testing.F) {
	f.Add([]byte{0, 0}, []byte{1, 0})                   // a applies "x", b applies "y"
	f.Add([]byte{0, 0, 0, 1}, []byte{0, 0})             // a applies then removes "x", b applies "x"
	f.Add([]byte{0, 0, 1, 0, 2, 0}, []byte{0, 1, 1, 1}) // mixed
	f.Add([]byte{0, 0, 0, 1}, []byte{0, 0, 0, 1})       // both apply then remove same key

	keys := []string{"x", "y", "z"}

	addDot := func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotSet, delta *dotcontext.DotSet) {
		d := ctx.Next(id)
		v.Add(d)
		delta.Add(d)
	}

	f.Fuzz(func(t *testing.T, opsA, opsB []byte) {
		a := newSetMap("a")
		b := newSetMap("b")

		var deltasA, deltasB []*ORMap[string, *dotcontext.DotSet]
		for i := 0; i+1 < len(opsA); i += 2 {
			key := keys[opsA[i]%3]
			if opsA[i+1]&1 == 0 {
				deltasA = append(deltasA, a.Apply(key, addDot))
			} else {
				deltasA = append(deltasA, a.Remove(key))
			}
		}
		for i := 0; i+1 < len(opsB); i += 2 {
			key := keys[opsB[i]%3]
			if opsB[i+1]&1 == 0 {
				deltasB = append(deltasB, b.Apply(key, addDot))
			} else {
				deltasB = append(deltasB, b.Remove(key))
			}
		}

		for _, d := range deltasB {
			a.Merge(d)
		}
		for _, d := range deltasA {
			b.Merge(d)
		}

		ak := a.Keys()
		bk := b.Keys()
		slices.Sort(ak)
		slices.Sort(bk)
		if !slices.Equal(ak, bk) {
			t.Fatalf("key divergence: a=%v b=%v", ak, bk)
		}

		// Values must also converge: same dot count per key.
		for _, k := range ak {
			va, _ := a.Get(k)
			vb, _ := b.Get(k)
			if va.Len() != vb.Len() {
				t.Fatalf("value divergence at key %q: a has %d dots, b has %d dots", k, va.Len(), vb.Len())
			}
		}
	})
}

// FuzzORMapNestedConvergence fuzzes a nested ORMap[string, *DotMap[string, *DotSet]].
// Each Apply targets an outer key + sub-key, adding a dot into the inner DotSet.
// This exercises the recursive MergeDotMap path (DotMap → DotMap → DotSet).
//
// Operation encoding: 3 bytes per op.
//
//	byte 0: outer key = ["a","b","c"][b % 3]
//	byte 1: sub-key   = ["p","q","r"][b % 3]  (used by Apply, ignored by Remove)
//	byte 2: bit0 = 0 → Apply, 1 → Remove outer key
func FuzzORMapNestedConvergence(f *testing.F) {
	f.Add([]byte{0, 0, 0}, []byte{1, 1, 0})                         // a applies a/p, b applies b/q
	f.Add([]byte{0, 0, 0, 0, 0, 1}, []byte{0, 0, 0})               // a applies a/p then removes a, b applies a/p
	f.Add([]byte{0, 0, 0, 0, 1, 0}, []byte{0, 2, 0})               // same outer key, different sub-keys
	f.Add([]byte{0, 0, 0, 1, 1, 0, 2, 2, 0}, []byte{0, 0, 1, 1, 1, 1}) // mixed

	outerKeys := []string{"a", "b", "c"}
	subKeys := []string{"p", "q", "r"}

	type nestedMap = *dotcontext.DotMap[string, *dotcontext.DotSet]

	applySubKey := func(sub string) func(dotcontext.ReplicaID, *dotcontext.CausalContext, nestedMap, nestedMap) {
		return func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v nestedMap, delta nestedMap) {
			d := ctx.Next(id)

			ds, ok := v.Get(sub)
			if !ok {
				ds = dotcontext.NewDotSet()
				v.Set(sub, ds)
			}
			ds.Add(d)

			deltaDS := dotcontext.NewDotSet()
			deltaDS.Add(d)
			delta.Set(sub, deltaDS)
		}
	}

	f.Fuzz(func(t *testing.T, opsA, opsB []byte) {
		a := newNestedMap("a")
		b := newNestedMap("b")

		var deltasA, deltasB []*ORMap[string, nestedMap]
		for i := 0; i+2 < len(opsA); i += 3 {
			key := outerKeys[opsA[i]%3]
			sub := subKeys[opsA[i+1]%3]
			if opsA[i+2]&1 == 0 {
				deltasA = append(deltasA, a.Apply(key, applySubKey(sub)))
			} else {
				deltasA = append(deltasA, a.Remove(key))
			}
		}
		for i := 0; i+2 < len(opsB); i += 3 {
			key := outerKeys[opsB[i]%3]
			sub := subKeys[opsB[i+1]%3]
			if opsB[i+2]&1 == 0 {
				deltasB = append(deltasB, b.Apply(key, applySubKey(sub)))
			} else {
				deltasB = append(deltasB, b.Remove(key))
			}
		}

		for _, d := range deltasB {
			a.Merge(d)
		}
		for _, d := range deltasA {
			b.Merge(d)
		}

		// Outer keys must converge.
		ak := a.Keys()
		bk := b.Keys()
		slices.Sort(ak)
		slices.Sort(bk)
		if !slices.Equal(ak, bk) {
			t.Fatalf("outer key divergence: a=%v b=%v", ak, bk)
		}

		// Inner structure must also converge: same sub-keys, same dot counts.
		for _, k := range ak {
			va, _ := a.Get(k)
			vb, _ := b.Get(k)

			vaKeys := va.Keys()
			vbKeys := vb.Keys()
			slices.Sort(vaKeys)
			slices.Sort(vbKeys)
			if !slices.Equal(vaKeys, vbKeys) {
				t.Fatalf("sub-key divergence at %q: a=%v b=%v", k, vaKeys, vbKeys)
			}
			for _, sk := range vaKeys {
				sa, _ := va.Get(sk)
				sb, _ := vb.Get(sk)
				if sa.Len() != sb.Len() {
					t.Fatalf("dot divergence at %q/%q: a=%d b=%d", k, sk, sa.Len(), sb.Len())
				}
			}
		}
	})
}
