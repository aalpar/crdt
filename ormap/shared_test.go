package ormap

import (
	"slices"
	"testing"

	"github.com/aalpar/crdt/crdttest"
	"github.com/aalpar/crdt/dotcontext"
)

func TestSharedProperties(t *testing.T) {
	newSetMap := func(id string) *ORMap[string, *dotcontext.DotSet] {
		return New[string, *dotcontext.DotSet](
			dotcontext.ReplicaID(id),
			dotcontext.MergeDotSetStore,
			dotcontext.NewDotSet,
		)
	}
	addKey := func(key string) func(*ORMap[string, *dotcontext.DotSet]) *ORMap[string, *dotcontext.DotSet] {
		return func(m *ORMap[string, *dotcontext.DotSet]) *ORMap[string, *dotcontext.DotSet] {
			return m.Apply(key, func(id dotcontext.ReplicaID, ctx *dotcontext.CausalContext, v *dotcontext.DotSet, delta *dotcontext.DotSet) {
				d := ctx.Next(id)
				v.Add(d)
				delta.Add(d)
			})
		}
	}
	crdttest.Harness[*ORMap[string, *dotcontext.DotSet]]{
		New:   newSetMap,
		Merge: func(dst, src *ORMap[string, *dotcontext.DotSet]) { dst.Merge(src) },
		Equal: func(a, b *ORMap[string, *dotcontext.DotSet]) bool {
			ak := a.Keys()
			bk := b.Keys()
			slices.Sort(ak)
			slices.Sort(bk)
			return slices.Equal(ak, bk)
		},
		Ops: []func(*ORMap[string, *dotcontext.DotSet]) *ORMap[string, *dotcontext.DotSet]{
			addKey("k1"),
			addKey("k2"),
			addKey("k3"),
			addKey("k4"),
			addKey("k5"),
		},
	}.Run(t)
}
