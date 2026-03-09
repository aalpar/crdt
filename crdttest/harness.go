// Package crdttest provides shared property tests for delta-state CRDT types.
//
// All delta-state CRDTs must satisfy the same semilattice properties:
// idempotent, commutative, and associative merge. They must also support
// incremental delta propagation and convergence under full-mesh merge.
// This package tests those properties generically so each CRDT only
// needs to supply a constructor, merge function, equality check, and
// a set of mutation operations.
package crdttest

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// Harness configures shared property tests for a CRDT type T.
//
// T is typically a pointer to a CRDT struct. Ops must mutate the
// receiver in place and return a delta of the same type T suitable
// for passing to Merge. At least 5 Ops are required for the full suite.
type Harness[T any] struct {
	// New creates a fresh CRDT for the given replica ID.
	New func(id string) T

	// Merge joins src into dst.
	Merge func(dst, src T)

	// Equal reports whether two CRDTs have equivalent observable state.
	Equal func(a, b T) bool

	// Ops are mutation functions. Each mutates the receiver and returns
	// a delta suitable for Merge. Provide at least 5 distinct operations.
	Ops []func(T) T
}

// Run executes all shared property tests.
func (h Harness[T]) Run(t *testing.T) {
	if len(h.Ops) < 5 {
		t.Fatalf("Harness requires at least 5 Ops, got %d", len(h.Ops))
	}
	t.Run("MergeProperties", h.testMergeProperties)
	t.Run("DeltaPropagation", h.testDeltaPropagation)
	t.Run("MergeWithEmpty", h.testMergeWithEmpty)
	t.Run("Convergence", h.testConvergence)
}

func (h Harness[T]) testMergeProperties(t *testing.T) {
	c := qt.New(t)

	c.Run("Idempotent", func(c *qt.C) {
		a := h.New("a")
		b := h.New("b")
		h.Ops[0](a)

		once := h.New("once")
		h.Merge(once, a)
		h.Merge(once, b)

		twice := h.New("twice")
		h.Merge(twice, a)
		h.Merge(twice, b)
		h.Merge(twice, b)

		c.Assert(h.Equal(once, twice), qt.IsTrue)
	})

	c.Run("Commutative", func(c *qt.C) {
		a := h.New("a")
		b := h.New("b")
		h.Ops[0](a)
		h.Ops[1](b)

		ab := h.New("ab")
		h.Merge(ab, a)
		h.Merge(ab, b)

		ba := h.New("ba")
		h.Merge(ba, b)
		h.Merge(ba, a)

		c.Assert(h.Equal(ab, ba), qt.IsTrue)
	})

	c.Run("Associative", func(c *qt.C) {
		a := h.New("a")
		b := h.New("b")
		x := h.New("c")
		h.Ops[0](a)
		h.Ops[1](b)
		h.Ops[2](x)

		// (a ⊔ b) ⊔ c
		ab := h.New("ab")
		h.Merge(ab, a)
		h.Merge(ab, b)
		abc := h.New("abc")
		h.Merge(abc, ab)
		h.Merge(abc, x)

		// a ⊔ (b ⊔ c)
		bc := h.New("bc")
		h.Merge(bc, b)
		h.Merge(bc, x)
		abc2 := h.New("abc2")
		h.Merge(abc2, a)
		h.Merge(abc2, bc)

		c.Assert(h.Equal(abc, abc2), qt.IsTrue)
	})
}

func (h Harness[T]) testDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("IncrementalEqualsFullMerge", func(c *qt.C) {
		a := h.New("a")
		d1 := h.Ops[0](a)
		d2 := h.Ops[1](a)
		d3 := h.Ops[2](a)

		incremental := h.New("inc")
		h.Merge(incremental, d1)
		h.Merge(incremental, d2)
		h.Merge(incremental, d3)

		full := h.New("full")
		h.Merge(full, a)

		c.Assert(h.Equal(incremental, full), qt.IsTrue)
	})

	c.Run("DeltaDeltaMerge", func(c *qt.C) {
		a := h.New("a")
		d1 := h.Ops[0](a)
		d2 := h.Ops[1](a)
		d3 := h.Ops[2](a)

		// Merge deltas together, then apply combined delta.
		h.Merge(d1, d2)
		h.Merge(d1, d3)

		b := h.New("b")
		h.Merge(b, d1)

		full := h.New("full")
		h.Merge(full, a)

		c.Assert(h.Equal(b, full), qt.IsTrue)
	})
}

func (h Harness[T]) testMergeWithEmpty(t *testing.T) {
	c := qt.New(t)

	c.Run("IntoEmpty", func(c *qt.C) {
		a := h.New("a")
		h.Ops[0](a)

		b := h.New("b")
		h.Merge(b, a)
		c.Assert(h.Equal(a, b), qt.IsTrue)
	})

	c.Run("EmptyIntoPopulated", func(c *qt.C) {
		a := h.New("a")
		h.Ops[0](a)

		before := h.New("before")
		h.Merge(before, a)

		empty := h.New("empty")
		h.Merge(a, empty)

		c.Assert(h.Equal(a, before), qt.IsTrue)
	})
}

func (h Harness[T]) testConvergence(t *testing.T) {
	c := qt.New(t)

	c.Run("ThreeReplica", func(c *qt.C) {
		a := h.New("a")
		b := h.New("b")
		x := h.New("c")

		da := h.Ops[0](a)
		db := h.Ops[1](b)
		dx := h.Ops[2](x)

		h.Merge(a, db)
		h.Merge(a, dx)
		h.Merge(b, da)
		h.Merge(b, dx)
		h.Merge(x, da)
		h.Merge(x, db)

		c.Assert(h.Equal(a, b), qt.IsTrue)
		c.Assert(h.Equal(b, x), qt.IsTrue)
	})

	c.Run("FiveReplica", func(c *qt.C) {
		ids := []string{"a", "b", "c", "d", "e"}
		replicas := make([]T, len(ids))
		deltas := make([]T, len(ids))
		for i, id := range ids {
			replicas[i] = h.New(id)
			deltas[i] = h.Ops[i](replicas[i])
		}

		for i := range replicas {
			for j := range replicas {
				if i != j {
					h.Merge(replicas[i], deltas[j])
				}
			}
		}

		for i := 1; i < len(replicas); i++ {
			c.Assert(h.Equal(replicas[0], replicas[i]), qt.IsTrue,
				qt.Commentf("replica 0 != replica %d", i))
		}
	})
}
