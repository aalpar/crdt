package awset

import (
	"slices"
	"testing"
)

// FuzzAWSetConvergence fuzzes random sequences of Add/Remove operations
// on two replicas, merges both directions, and checks convergence.
//
// Operation encoding: 2 bytes per op.
//
//	byte 0: element = ["x","y","z"][b % 3]
//	byte 1: bit0 = 0 → Add, 1 → Remove
func FuzzAWSetConvergence(f *testing.F) {
	f.Add([]byte{0, 0}, []byte{1, 0})                   // a adds "x", b adds "y"
	f.Add([]byte{0, 0, 0, 1}, []byte{0, 0})             // a adds "x" then removes it, b adds "x"
	f.Add([]byte{0, 0, 1, 0, 2, 0}, []byte{0, 1, 1, 1}) // mixed

	elems := []string{"x", "y", "z"}

	f.Fuzz(func(t *testing.T, opsA, opsB []byte) {
		a := New[string]("a")
		b := New[string]("b")

		// Execute ops on each replica, collecting deltas.
		var deltasA, deltasB []*AWSet[string]
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

		// Merge all of b's deltas into a, and vice versa.
		for _, d := range deltasB {
			a.Merge(d)
		}
		for _, d := range deltasA {
			b.Merge(d)
		}

		// Convergence: both replicas should have the same elements.
		ae := a.Elements()
		be := b.Elements()
		slices.Sort(ae)
		slices.Sort(be)
		if !slices.Equal(ae, be) {
			t.Fatalf("divergence: a=%v b=%v", ae, be)
		}
	})
}
