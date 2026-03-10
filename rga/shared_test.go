package rga

import (
	"testing"

	"github.com/aalpar/crdt/crdttest"
	"github.com/aalpar/crdt/dotcontext"
)

func TestSharedProperties(t *testing.T) {
	crdttest.Harness[*RGA[string]]{
		New: func(id string) *RGA[string] { return New[string](dotcontext.ReplicaID(id)) },
		Merge: func(dst, src *RGA[string]) { dst.Merge(src) },
		Equal: func(a, b *RGA[string]) bool {
			ae := a.Elements()
			be := b.Elements()
			if len(ae) != len(be) {
				return false
			}
			for i := range ae {
				if ae[i].Value != be[i].Value {
					return false
				}
			}
			return true
		},
		Ops: []func(*RGA[string]) *RGA[string]{
			func(r *RGA[string]) *RGA[string] { return r.InsertAfter(dotcontext.Dot{}, "a") },
			func(r *RGA[string]) *RGA[string] { return r.InsertAfter(dotcontext.Dot{}, "b") },
			func(r *RGA[string]) *RGA[string] {
				elems := r.Elements()
				if len(elems) > 0 {
					return r.InsertAfter(elems[len(elems)-1].ID, "c")
				}
				return r.InsertAfter(dotcontext.Dot{}, "c")
			},
			func(r *RGA[string]) *RGA[string] {
				elems := r.Elements()
				if len(elems) > 0 {
					return r.Delete(elems[0].ID)
				}
				return r.InsertAfter(dotcontext.Dot{}, "d")
			},
			func(r *RGA[string]) *RGA[string] {
				elems := r.Elements()
				if len(elems) > 0 {
					return r.InsertAfter(elems[0].ID, "e")
				}
				return r.InsertAfter(dotcontext.Dot{}, "e")
			},
		},
	}.Run(t)
}
