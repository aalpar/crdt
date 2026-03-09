package pncounter

import (
	"testing"

	"github.com/aalpar/crdt/crdttest"
	"github.com/aalpar/crdt/dotcontext"
)

func TestSharedProperties(t *testing.T) {
	crdttest.Harness[*Counter]{
		New:   func(id string) *Counter { return New(dotcontext.ReplicaID(id)) },
		Merge: func(dst, src *Counter) { dst.Merge(src) },
		Equal: func(a, b *Counter) bool { return a.Value() == b.Value() },
		Ops: []func(*Counter) *Counter{
			func(c *Counter) *Counter { return c.Increment(5) },
			func(c *Counter) *Counter { return c.Increment(3) },
			func(c *Counter) *Counter { return c.Decrement(2) },
			func(c *Counter) *Counter { return c.Increment(10) },
			func(c *Counter) *Counter { return c.Decrement(4) },
		},
	}.Run(t)
}
