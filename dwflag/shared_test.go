package dwflag

import (
	"testing"

	"github.com/aalpar/crdt/crdttest"
	"github.com/aalpar/crdt/dotcontext"
)

func TestSharedProperties(t *testing.T) {
	crdttest.Harness[*DWFlag]{
		New:   func(id string) *DWFlag { return New(dotcontext.ReplicaID(id)) },
		Merge: func(dst, src *DWFlag) { dst.Merge(src) },
		Equal: func(a, b *DWFlag) bool { return a.Value() == b.Value() },
		Ops: []func(*DWFlag) *DWFlag{
			func(f *DWFlag) *DWFlag { return f.Disable() },
			func(f *DWFlag) *DWFlag { return f.Enable() },
			func(f *DWFlag) *DWFlag { return f.Disable() },
			func(f *DWFlag) *DWFlag { return f.Enable() },
			func(f *DWFlag) *DWFlag { return f.Disable() },
		},
	}.Run(t)
}
