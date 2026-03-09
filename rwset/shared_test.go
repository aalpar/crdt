package rwset

import (
	"slices"
	"testing"

	"github.com/aalpar/crdt/crdttest"
	"github.com/aalpar/crdt/dotcontext"
)

func TestSharedProperties(t *testing.T) {
	crdttest.Harness[*RWSet[string]]{
		New:   func(id string) *RWSet[string] { return New[string](dotcontext.ReplicaID(id)) },
		Merge: func(dst, src *RWSet[string]) { dst.Merge(src) },
		Equal: func(a, b *RWSet[string]) bool {
			ae := a.Elements()
			be := b.Elements()
			slices.Sort(ae)
			slices.Sort(be)
			return slices.Equal(ae, be)
		},
		Ops: []func(*RWSet[string]) *RWSet[string]{
			func(s *RWSet[string]) *RWSet[string] { return s.Add("x") },
			func(s *RWSet[string]) *RWSet[string] { return s.Add("y") },
			func(s *RWSet[string]) *RWSet[string] { return s.Add("z") },
			func(s *RWSet[string]) *RWSet[string] { return s.Add("w") },
			func(s *RWSet[string]) *RWSet[string] { return s.Add("v") },
		},
	}.Run(t)
}
