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

func (c counterValue) Join(other counterValue) counterValue {
	return c
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
	return &Counter{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotFun[counterValue]]{
			Store:   dotcontext.NewDotFun[counterValue](),
			Context: dotcontext.New(),
		},
	}
}

// Increment adds n to the counter and returns a delta for replication.
// Use a negative n to decrement.
func (c *Counter) Increment(n int64) *Counter {
	// Find this replica's current dot and value.
	var oldDot dotcontext.Dot
	var oldValue int64
	hasOld := false

	c.state.Store.Range(func(d dotcontext.Dot, v counterValue) bool {
		if d.ID == c.id {
			oldDot = d
			oldValue = v.n
			hasOld = true
			return false
		}
		return true
	})

	// Generate new dot and update local state.
	d := c.state.Context.Next(c.id)
	if hasOld {
		c.state.Store.Remove(oldDot)
	}
	newVal := counterValue{n: oldValue + n}
	c.state.Store.Set(d, newVal)

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
func (c *Counter) Decrement(n int64) *Counter {
	return c.Increment(-n)
}

// Value returns the current counter total (sum of all replica contributions).
func (c *Counter) Value() int64 {
	var total int64
	c.state.Store.Range(func(_ dotcontext.Dot, v counterValue) bool {
		total += v.n
		return true
	})
	return total
}

// Merge incorporates a delta or full state from another counter.
func (c *Counter) Merge(other *Counter) {
	c.state = dotcontext.JoinDotFun(c.state, other.state)
}
