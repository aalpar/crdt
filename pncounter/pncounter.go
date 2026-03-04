package pncounter

import (
	"github.com/aalpar/crdt/dotcontext"
)

// counterValue holds a replica's accumulated contribution.
// It implements dotcontext.Lattice. In practice, JoinDotFun only
// calls Join when both sides have the same dot (same event), so
// the values are identical and either is correct.
type counterValue struct {
	n int64
}

func (p counterValue) Join(other counterValue) counterValue {
	return p
}

// Counter is a positive-negative counter. Each replica tracks its
// accumulated contribution as a single dot in a DotFun. The global
// value is the sum of all entries.
type Counter struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotFun[counterValue]]
}

// New creates a counter at zero for the given replica.
func New(replicaID dotcontext.ReplicaID) *Counter {
	q := &Counter{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotFun[counterValue]]{
			Store:   dotcontext.NewDotFun[counterValue](),
			Context: dotcontext.New(),
		},
	}
	return q
}

// Increment adds n to the counter and returns a delta for replication.
// Use a negative n to decrement.
func (p *Counter) Increment(n int64) *Counter {
	// Find this replica's current dot and value.
	var oldDot dotcontext.Dot
	var oldValue int64
	hasOld := false

	p.state.Store.Range(func(d dotcontext.Dot, v counterValue) bool {
		if d.ID == p.id {
			oldDot = d
			oldValue = v.n
			hasOld = true
			return false
		}
		return true
	})

	// Generate new dot and update local state.
	d := p.state.Context.Next(p.id)
	if hasOld {
		p.state.Store.Remove(oldDot)
	}
	newVal := counterValue{n: oldValue + n}
	p.state.Store.Set(d, newVal)

	// Build delta: new entry + context covering old dot.
	deltaStore := dotcontext.NewDotFun[counterValue]()
	deltaStore.Set(d, newVal)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)
	if hasOld {
		deltaCtx.Add(oldDot)
	}

	return &Counter{
		state: dotcontext.Causal[*dotcontext.DotFun[counterValue]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}

// Decrement subtracts n from the counter and returns a delta.
func (p *Counter) Decrement(n int64) *Counter {
	return p.Increment(-n)
}

// Value returns the current counter total (sum of all replica contributions).
func (p *Counter) Value() int64 {
	var total int64
	p.state.Store.Range(func(_ dotcontext.Dot, v counterValue) bool {
		total += v.n
		return true
	})
	return total
}

// Merge incorporates a delta or full state from another counter.
func (p *Counter) Merge(other *Counter) {
	p.state = dotcontext.JoinDotFun(p.state, other.state)
}
