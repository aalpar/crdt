package awset

import (
	"slices"
	"testing"

	"github.com/aalpar/crdt/crdttest"
	"github.com/aalpar/crdt/dotcontext"
)

func TestSharedProperties(t *testing.T) {
	crdttest.Harness[*AWSet[string]]{
		New:   func(id string) *AWSet[string] { return New[string](dotcontext.ReplicaID(id)) },
		Merge: func(dst, src *AWSet[string]) { dst.Merge(src) },
		Equal: func(a, b *AWSet[string]) bool {
			ae := a.Elements()
			be := b.Elements()
			slices.Sort(ae)
			slices.Sort(be)
			return slices.Equal(ae, be)
		},
		Ops: []func(*AWSet[string]) *AWSet[string]{
			func(s *AWSet[string]) *AWSet[string] { return s.Add("x") },
			func(s *AWSet[string]) *AWSet[string] { return s.Add("y") },
			func(s *AWSet[string]) *AWSet[string] { return s.Add("z") },
			func(s *AWSet[string]) *AWSet[string] { return s.Add("w") },
			func(s *AWSet[string]) *AWSet[string] { return s.Add("v") },
		},
	}.Run(t)
}
