package rwset

import (
	"slices"
	"testing"
)

// FuzzRWSetConvergence fuzzes random sequences of Add/Remove operations
// on two replicas, merges both directions, and checks convergence.
//
// Operation encoding: 2 bytes per op.
//
//	byte 0: element = ["x","y","z"][b % 3]
//	byte 1: bit0 = 0 → Add, 1 → Remove
func FuzzRWSetConvergence(f *testing.F) {
	f.Add([]byte{0, 0}, []byte{1, 0})                   // a adds "x", b adds "y"
	f.Add([]byte{0, 0, 0, 1}, []byte{0, 0})             // a adds then removes "x", b adds "x"
	f.Add([]byte{0, 0, 1, 0, 2, 0}, []byte{0, 1, 1, 1}) // mixed
	f.Add([]byte{0, 0, 0, 1}, []byte{0, 0, 0, 1})       // both add then remove same element

	elems := []string{"x", "y", "z"}

	f.Fuzz(func(t *testing.T, opsA, opsB []byte) {
		a := New[string]("a")
		b := New[string]("b")

		var deltasA, deltasB []*RWSet[string]
		for i := 0; i+1 < len(opsA); i += 2 {
			elem := elems[opsA[i]%3]
			if opsA[i+1]&1 == 0 {
				deltasA = append(deltasA, a.Add(elem))
			} else {
				deltasA = append(deltasA, a.Remove(elem))
			}
		}
		for i := 0; i+1 < len(opsB); i += 2 {
			elem := elems[opsB[i]%3]
			if opsB[i+1]&1 == 0 {
				deltasB = append(deltasB, b.Add(elem))
			} else {
				deltasB = append(deltasB, b.Remove(elem))
			}
		}

		for _, d := range deltasB {
			a.Merge(d)
		}
		for _, d := range deltasA {
			b.Merge(d)
		}

		ae := a.Elements()
		be := b.Elements()
		slices.Sort(ae)
		slices.Sort(be)
		if !slices.Equal(ae, be) {
			t.Fatalf("divergence: a=%v b=%v", ae, be)
		}
	})
}
