package gset

import (
	"slices"
	"testing"

	"github.com/aalpar/crdt/crdttest"
)

func TestSharedProperties(t *testing.T) {
	crdttest.Harness[*GSet[string]]{
		New:   func(_ string) *GSet[string] { return New[string]() },
		Merge: func(dst, src *GSet[string]) { dst.Merge(src) },
		Equal: func(a, b *GSet[string]) bool {
			ae := a.Elements()
			be := b.Elements()
			slices.Sort(ae)
			slices.Sort(be)
			return slices.Equal(ae, be)
		},
		Ops: []func(*GSet[string]) *GSet[string]{
			func(s *GSet[string]) *GSet[string] { return s.Add("x") },
			func(s *GSet[string]) *GSet[string] { return s.Add("y") },
			func(s *GSet[string]) *GSet[string] { return s.Add("z") },
			func(s *GSet[string]) *GSet[string] { return s.Add("w") },
			func(s *GSet[string]) *GSet[string] { return s.Add("v") },
		},
	}.Run(t)
}
