package awset

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

// TestBaqueroDecompositionTrace walks through a 3-node scenario step by step,
// asserting on dot store contents and causal context at each point.
//
// The scenario exercises all three terms of the JoinDotSet formula:
//
//	(s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁)
//
// Scenario:
//
//	Step 1: A adds "x"                    → dot (A,1)
//	Step 2: B adds "x" concurrently       → dot (B,1)
//	Step 3: A and B merge                 → both have "x" with {(A,1),(B,1)}
//	Step 4: A removes "x"                 → A's store empty, context has A:1,B:1
//	Step 5: B adds "x" again (before seeing A's remove) → dot (B,2)
//	Step 6: A merges B's new add delta    → "x" survives: (B,2) not in A's context
//	Step 7: C (who saw nothing) merges A  → convergence check
func TestBaqueroDecompositionTrace(t *testing.T) {
	c := qt.New(t)
	A := New[string]("A")
	B := New[string]("B")
	C := New[string]("C")

	// ── Step 1: A adds "x" ─────────────────────────────────────────
	deltaA1 := A.Add("x")

	c.Assert(A.Has("x"), qt.IsTrue)
	assertDots(c, A, "x", []dotcontext.Dot{{ID: "A", Seq: 1}})
	assertContext(c, A, map[dotcontext.ReplicaID]uint64{"A": 1})

	// ── Step 2: B adds "x" concurrently ────────────────────────────
	deltaB1 := B.Add("x")

	c.Assert(B.Has("x"), qt.IsTrue)
	assertDots(c, B, "x", []dotcontext.Dot{{ID: "B", Seq: 1}})
	assertContext(c, B, map[dotcontext.ReplicaID]uint64{"B": 1})

	// ── Step 3: A and B merge ──────────────────────────────────────
	A.Merge(deltaB1)
	B.Merge(deltaA1)

	assertDots(c, A, "x", []dotcontext.Dot{
		{ID: "A", Seq: 1},
		{ID: "B", Seq: 1},
	})
	assertContext(c, A, map[dotcontext.ReplicaID]uint64{"A": 1, "B": 1})

	assertDots(c, B, "x", []dotcontext.Dot{
		{ID: "A", Seq: 1},
		{ID: "B", Seq: 1},
	})
	assertContext(c, B, map[dotcontext.ReplicaID]uint64{"A": 1, "B": 1})

	// ── Step 4: A removes "x" ──────────────────────────────────────
	deltaRm := A.Remove("x")
	_ = deltaRm

	c.Assert(A.Has("x"), qt.IsFalse)
	assertContext(c, A, map[dotcontext.ReplicaID]uint64{"A": 1, "B": 1})

	// ── Step 5: B adds "x" again (hasn't seen A's remove) ─────────
	deltaB2 := B.Add("x")

	assertDots(c, B, "x", []dotcontext.Dot{
		{ID: "A", Seq: 1},
		{ID: "B", Seq: 1},
		{ID: "B", Seq: 2},
	})
	assertContext(c, B, map[dotcontext.ReplicaID]uint64{"A": 1, "B": 2})

	// ── Step 6: A merges B's add delta ─────────────────────────────
	//
	// TODO(aalpar): Before reading further, predict:
	//   Which term of (s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁) keeps (B,2) alive?
	//   What is A's resulting dot set for "x"?
	//   What is A's resulting causal context?

	A.Merge(deltaB2)

	c.Assert(A.Has("x"), qt.IsTrue)
	assertDots(c, A, "x", []dotcontext.Dot{
		{ID: "B", Seq: 2},
	})
	assertContext(c, A, map[dotcontext.ReplicaID]uint64{"A": 1, "B": 2})

	// ── Step 7: C merges A's full state ────────────────────────────
	//
	// TODO(aalpar): Predict C's state after merging A.

	C.Merge(A)

	c.Assert(C.Has("x"), qt.IsTrue)
	assertContext(c, C, map[dotcontext.ReplicaID]uint64{"A": 1, "B": 2})

	// ── Step 8: B merges A's remove delta from step 4 ──────────────
	//
	// TODO(aalpar): Predict which dots survive in B after this merge.

	B.Merge(deltaRm)

	c.Assert(B.Has("x"), qt.IsTrue)
	assertDots(c, B, "x", []dotcontext.Dot{
		{ID: "B", Seq: 2},
	})
}

// ── Test helpers ────────────────────────────────────────────────────

// assertDots checks that the given element's dot set contains exactly the
// expected dots (order-independent).
func assertDots[E comparable](c *qt.C, s *AWSet[E], elem E, want []dotcontext.Dot) {
	c.Helper()
	ds, ok := s.state.Store.Get(elem)
	if !ok {
		if len(want) == 0 {
			return
		}
		c.Fatalf("element not in store, want dots %v", want)
	}

	got := make(map[dotcontext.Dot]struct{})
	ds.Range(func(d dotcontext.Dot) bool {
		got[d] = struct{}{}
		return true
	})

	c.Assert(len(got), qt.Equals, len(want), qt.Commentf("dots: got %v, want %v", got, want))
	for _, d := range want {
		_, ok := got[d]
		c.Assert(ok, qt.IsTrue, qt.Commentf("missing dot %v in %v", d, got))
	}
}

// assertContext checks the version vector portion of the causal context.
func assertContext[E comparable](c *qt.C, s *AWSet[E], wantVV map[dotcontext.ReplicaID]uint64) {
	c.Helper()
	ctx := s.state.Context
	for id, wantSeq := range wantVV {
		c.Assert(ctx.Max(id), qt.Equals, wantSeq, qt.Commentf("context[%s]", id))
	}
}
