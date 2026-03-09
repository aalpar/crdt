package mvregister

import (
	"slices"
	"testing"

	"github.com/aalpar/crdt/crdttest"
	"github.com/aalpar/crdt/dotcontext"
)

func TestSharedProperties(t *testing.T) {
	crdttest.Harness[*MVRegister[string]]{
		New: func(id string) *MVRegister[string] { return New[string](dotcontext.ReplicaID(id)) },
		Merge: func(dst, src *MVRegister[string]) { dst.Merge(src) },
		Equal: func(a, b *MVRegister[string]) bool {
			av := a.Values()
			bv := b.Values()
			slices.Sort(av)
			slices.Sort(bv)
			return slices.Equal(av, bv)
		},
		Ops: []func(*MVRegister[string]) *MVRegister[string]{
			func(r *MVRegister[string]) *MVRegister[string] { return r.Write("alpha") },
			func(r *MVRegister[string]) *MVRegister[string] { return r.Write("beta") },
			func(r *MVRegister[string]) *MVRegister[string] { return r.Write("gamma") },
			func(r *MVRegister[string]) *MVRegister[string] { return r.Write("delta") },
			func(r *MVRegister[string]) *MVRegister[string] { return r.Write("epsilon") },
		},
	}.Run(t)
}
