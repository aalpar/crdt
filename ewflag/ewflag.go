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
	q := &EWFlag{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotSet]{
			Store:   dotcontext.NewDotSet(),
			Context: dotcontext.New(),
		},
	}
	return q
}

// Enable sets the flag to true and returns a delta for replication.
func (p *EWFlag) Enable() *EWFlag {
	d := p.state.Context.Next(p.id)
	p.state.Store.Add(d)

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
func (p *EWFlag) Disable() *EWFlag {
	deltaCtx := dotcontext.New()

	p.state.Store.Range(func(d dotcontext.Dot) bool {
		deltaCtx.Add(d)
		return true
	})

	p.state.Store = dotcontext.NewDotSet()

	return &EWFlag{
		state: dotcontext.Causal[*dotcontext.DotSet]{
			Store:   dotcontext.NewDotSet(),
			Context: deltaCtx,
		},
	}
}

// Value reports whether the flag is enabled.
func (p *EWFlag) Value() bool {
	return p.state.Store.Len() > 0
}

// State returns the EWFlag's internal Causal state for serialization.
func (p *EWFlag) State() dotcontext.Causal[*dotcontext.DotSet] {
	return p.state
}

// FromCausal constructs an EWFlag from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal(state dotcontext.Causal[*dotcontext.DotSet]) *EWFlag {
	return &EWFlag{state: state}
}

// Merge incorporates a delta or full state from another EWFlag.
func (p *EWFlag) Merge(other *EWFlag) {
	p.state = dotcontext.JoinDotSet(p.state, other.state)
}
