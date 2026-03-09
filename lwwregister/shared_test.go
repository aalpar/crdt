package lwwregister

import (
	"testing"

	"github.com/aalpar/crdt/crdttest"
	"github.com/aalpar/crdt/dotcontext"
)

func TestSharedProperties(t *testing.T) {
	ts := int64(0)
	nextTs := func() int64 { ts++; return ts }
	crdttest.Harness[*LWWRegister[string]]{
		New: func(id string) *LWWRegister[string] { return New[string](dotcontext.ReplicaID(id)) },
		Merge: func(dst, src *LWWRegister[string]) { dst.Merge(src) },
		Equal: func(a, b *LWWRegister[string]) bool {
			av, ats, aok := a.Value()
			bv, bts, bok := b.Value()
			return av == bv && ats == bts && aok == bok
		},
		Ops: []func(*LWWRegister[string]) *LWWRegister[string]{
			func(r *LWWRegister[string]) *LWWRegister[string] { return r.Set("alpha", nextTs()) },
			func(r *LWWRegister[string]) *LWWRegister[string] { return r.Set("beta", nextTs()) },
			func(r *LWWRegister[string]) *LWWRegister[string] { return r.Set("gamma", nextTs()) },
			func(r *LWWRegister[string]) *LWWRegister[string] { return r.Set("delta", nextTs()) },
			func(r *LWWRegister[string]) *LWWRegister[string] { return r.Set("epsilon", nextTs()) },
		},
	}.Run(t)
}
