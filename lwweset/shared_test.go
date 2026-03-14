package lwweset

import (
	"slices"
	"testing"

	"github.com/aalpar/crdt/crdttest"
)

func TestSharedProperties(t *testing.T) {
	crdttest.Harness[*LWWESet[string]]{
		New:   func(_ string) *LWWESet[string] { return New[string]() },
		Merge: func(dst, src *LWWESet[string]) { dst.Merge(src) },
		Equal: func(a, b *LWWESet[string]) bool {
			ae := a.Elements()
			be := b.Elements()
			slices.Sort(ae)
			slices.Sort(be)
			return slices.Equal(ae, be)
		},
		Ops: []func(*LWWESet[string]) *LWWESet[string]{
			func(s *LWWESet[string]) *LWWESet[string] { return s.Add("x", 1) },
			func(s *LWWESet[string]) *LWWESet[string] { return s.Add("y", 2) },
			func(s *LWWESet[string]) *LWWESet[string] { return s.Add("z", 3) },
			func(s *LWWESet[string]) *LWWESet[string] { return s.Add("w", 4) },
			func(s *LWWESet[string]) *LWWESet[string] { return s.Add("v", 5) },
		},
	}.Run(t)
}
