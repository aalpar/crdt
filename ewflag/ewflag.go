package ewflag

import (
	"github.com/aalpar/crdt/dotcontext"
)

// EWFlag is an enable-wins flag. Concurrent enable and disable resolves
// in favor of enable.
//
// Internally it is a bare DotSet paired with a causal context — the
// simplest possible delta-state CRDT. A non-empty dot set means enabled.
type EWFlag struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotSet]
}

// New creates a disabled EWFlag for the given replica.
func New(replicaID dotcontext.ReplicaID) *EWFlag {
	return &EWFlag{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotSet]{
			Store:   dotcontext.NewDotSet(),
			Context: dotcontext.New(),
		},
	}
}

// Enable sets the flag to true and returns a delta for replication.
func (f *EWFlag) Enable() *EWFlag {
	d := f.state.Context.Next(f.id)
	f.state.Store.Add(d)

	deltaStore := dotcontext.NewDotSet()
	deltaStore.Add(d)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)

	return &EWFlag{
		state: dotcontext.Causal[*dotcontext.DotSet]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Disable sets the flag to false and returns a delta for replication.
func (f *EWFlag) Disable() *EWFlag {
	deltaCtx := dotcontext.New()

	f.state.Store.Range(func(d dotcontext.Dot) bool {
		deltaCtx.Add(d)
		return true
	})

	f.state.Store = dotcontext.NewDotSet()

	return &EWFlag{
		state: dotcontext.Causal[*dotcontext.DotSet]{
			Store:   dotcontext.NewDotSet(),
			Context: deltaCtx,
		},
	}
}

// Value reports whether the flag is enabled.
func (f *EWFlag) Value() bool {
	return f.state.Store.Len() > 0
}

// Merge incorporates a delta or full state from another EWFlag.
func (f *EWFlag) Merge(other *EWFlag) {
	f.state = dotcontext.JoinDotSet(f.state, other.state)
}
