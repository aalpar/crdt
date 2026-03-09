package ewflag

import (
	"testing"

	"github.com/aalpar/crdt/crdttest"
	"github.com/aalpar/crdt/dotcontext"
)

func TestSharedProperties(t *testing.T) {
	crdttest.Harness[*EWFlag]{
		New:   func(id string) *EWFlag { return New(dotcontext.ReplicaID(id)) },
		Merge: func(dst, src *EWFlag) { dst.Merge(src) },
		Equal: func(a, b *EWFlag) bool { return a.Value() == b.Value() },
		Ops: []func(*EWFlag) *EWFlag{
			func(f *EWFlag) *EWFlag { return f.Enable() },
			func(f *EWFlag) *EWFlag { return f.Disable() },
			func(f *EWFlag) *EWFlag { return f.Enable() },
			func(f *EWFlag) *EWFlag { return f.Disable() },
			func(f *EWFlag) *EWFlag { return f.Enable() },
		},
	}.Run(t)
}
