package dwflag

import (
	"github.com/aalpar/crdt/dotcontext"
)

// DWFlag is a disable-wins flag. Concurrent enable and disable resolves
// in favor of disable.
//
// Internally it is a bare DotSet paired with a causal context — the
// exact dual of EWFlag. A non-empty dot set means disabled; empty means
// enabled.
type DWFlag struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotSet]
}

// New creates an enabled DWFlag for the given replica.
func New(replicaID dotcontext.ReplicaID) *DWFlag {
	q := &DWFlag{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotSet]{
			Store:   dotcontext.NewDotSet(),
			Context: dotcontext.New(),
		},
	}
	return q
}

// Disable sets the flag to false and returns a delta for replication.
func (p *DWFlag) Disable() *DWFlag {
	d := p.state.Context.Next(p.id)
	p.state.Store.Add(d)

	deltaStore := dotcontext.NewDotSet()
	deltaStore.Add(d)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)

	return &DWFlag{
		state: dotcontext.Causal[*dotcontext.DotSet]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Enable sets the flag to true and returns a delta for replication.
func (p *DWFlag) Enable() *DWFlag {
	deltaCtx := dotcontext.New()

	p.state.Store.Range(func(d dotcontext.Dot) bool {
		deltaCtx.Add(d)
		return true
	})

	p.state.Store = dotcontext.NewDotSet()

	return &DWFlag{
		state: dotcontext.Causal[*dotcontext.DotSet]{
			Store:   dotcontext.NewDotSet(),
			Context: deltaCtx,
		},
	}
}

// Value reports whether the flag is enabled.
func (p *DWFlag) Value() bool {
	return p.state.Store.Len() == 0
}

// State returns the DWFlag's internal Causal state for serialization.
func (p *DWFlag) State() dotcontext.Causal[*dotcontext.DotSet] {
	return p.state
}

// FromCausal constructs a DWFlag from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal(state dotcontext.Causal[*dotcontext.DotSet]) *DWFlag {
	return &DWFlag{state: state}
}

// Merge incorporates a delta or full state from another DWFlag.
func (p *DWFlag) Merge(other *DWFlag) {
	p.state = dotcontext.JoinDotSet(p.state, other.state)
}
