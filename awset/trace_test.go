package awset

import (
	"testing"

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
	A := New[string]("A")
	B := New[string]("B")
	C := New[string]("C")

	// ── Step 1: A adds "x" ─────────────────────────────────────────
	// A generates dot (A,1) and maps "x" → {(A,1)}.
	// A's causal context: {A:1}
	deltaA1 := A.Add("x")

	assertHasElem(t, A, "x", "step 1: A should have x")
	assertDots(t, A, "x", []dotcontext.Dot{{ID: "A", Seq: 1}}, "step 1: A.store[x]")
	assertContext(t, A, map[string]uint64{"A": 1}, "step 1: A.context")

	// ── Step 2: B adds "x" concurrently ────────────────────────────
	// B hasn't seen A's operation. B generates dot (B,1).
	// B's causal context: {B:1}
	deltaB1 := B.Add("x")

	assertHasElem(t, B, "x", "step 2: B should have x")
	assertDots(t, B, "x", []dotcontext.Dot{{ID: "B", Seq: 1}}, "step 2: B.store[x]")
	assertContext(t, B, map[string]uint64{"B": 1}, "step 2: B.context")

	// ── Step 3: A and B merge ──────────────────────────────────────
	// Both replicas exchange full states.
	//
	// When A merges B's delta:
	//   For key "x", JoinDotSet runs on:
	//     a.store = {(A,1)}, a.context = {A:1}
	//     b.store = {(B,1)}, b.context = {B:1}
	//
	//   (A,1) ∈ a.store, (A,1) ∉ b.store, (A,1) ∉ b.context → KEEP (s₁ \ c₂)
	//   (B,1) ∈ b.store, (B,1) ∉ a.store, (B,1) ∉ a.context → KEEP (s₂ \ c₁)
	//
	//   Result: "x" → {(A,1), (B,1)}, context = {A:1, B:1}
	A.Merge(deltaB1)
	B.Merge(deltaA1)

	assertDots(t, A, "x", []dotcontext.Dot{
		{ID: "A", Seq: 1},
		{ID: "B", Seq: 1},
	}, "step 3: A.store[x] after merge")
	assertContext(t, A, map[string]uint64{"A": 1, "B": 1}, "step 3: A.context")

	assertDots(t, B, "x", []dotcontext.Dot{
		{ID: "A", Seq: 1},
		{ID: "B", Seq: 1},
	}, "step 3: B.store[x] after merge")
	assertContext(t, B, map[string]uint64{"A": 1, "B": 1}, "step 3: B.context")

	// ── Step 4: A removes "x" ──────────────────────────────────────
	// A's remove delta has:
	//   store: empty (no dots — the element is gone)
	//   context: {(A,1), (B,1)} (the dots A observed for "x")
	//
	// A's local state after remove:
	//   store: "x" gone entirely
	//   context: still {A:1, B:1}
	deltaRm := A.Remove("x")
	_ = deltaRm // we'll use this in step 7

	assertNotHasElem(t, A, "x", "step 4: A should not have x")
	assertContext(t, A, map[string]uint64{"A": 1, "B": 1}, "step 4: A.context unchanged")

	// ── Step 5: B adds "x" again (hasn't seen A's remove) ─────────
	// B generates dot (B,2). B still has "x" → {(A,1),(B,1)} from step 3.
	// After this add: "x" → {(A,1),(B,1),(B,2)}, context = {A:1, B:2}
	deltaB2 := B.Add("x")

	assertDots(t, B, "x", []dotcontext.Dot{
		{ID: "A", Seq: 1},
		{ID: "B", Seq: 1},
		{ID: "B", Seq: 2},
	}, "step 5: B.store[x] after re-add")
	assertContext(t, B, map[string]uint64{"A": 1, "B": 2}, "step 5: B.context")

	// ── Step 6: A merges B's add delta ─────────────────────────────
	//
	// B's delta from step 5:
	//   store: "x" → {(B,2)}
	//   context: {(B,2)}   (just the new dot)
	//
	// A's current state:
	//   store: empty (removed "x" in step 4)
	//   context: {A:1, B:1}
	//
	// JoinDotMap runs JoinDotSet for key "x":
	//   a.store = {}        a.context = {A:1, B:1}
	//   b.store = {(B,2)}   b.context = {(B,2)}
	//
	// TODO(aalpar): Before reading further, predict:
	//   Which term of (s₁ ∩ s₂) ∪ (s₁ \ c₂) ∪ (s₂ \ c₁) keeps (B,2) alive?
	//   What is A's resulting dot set for "x"?
	//   What is A's resulting causal context?
	//
	// Write your predictions here, then uncomment the assertions below
	// to verify.

	A.Merge(deltaB2)

	assertHasElem(t, A, "x", "step 6: A should have x — add wins")
	assertDots(t, A, "x", []dotcontext.Dot{
		{ID: "B", Seq: 2},
	}, "step 6: A.store[x] — only the new dot survives")
	assertContext(t, A, map[string]uint64{"A": 1, "B": 2}, "step 6: A.context")

	// ── Step 7: C merges A's full state ────────────────────────────
	//
	// C has seen nothing. C's state:
	//   store: empty
	//   context: empty
	//
	// After merging A (whose state we just computed in step 6):
	//
	// TODO(aalpar): Predict C's state after merging A.
	//   What elements does C have?
	//   What dots? What context?

	C.Merge(A)

	assertHasElem(t, C, "x", "step 7: C should have x")
	assertContext(t, C, map[string]uint64{"A": 1, "B": 2}, "step 7: C.context")

	// ── Step 8: B merges A's remove delta from step 4 ──────────────
	//
	// This is the "add-wins" moment. B's current state (from step 5):
	//   store: "x" → {(A,1),(B,1),(B,2)}
	//   context: {A:1, B:2}
	//
	// A's remove delta (from step 4):
	//   store: empty
	//   context: {(A,1),(B,1)}
	//
	// TODO(aalpar): Predict which dots survive in B after this merge.
	//   (A,1) is in both B's store and the remove's context — what happens?
	//   (B,1) is in both B's store and the remove's context — what happens?
	//   (B,2) is in B's store but NOT in the remove's context — what happens?

	B.Merge(deltaRm)

	assertHasElem(t, B, "x", "step 8: B should still have x — add wins")
	assertDots(t, B, "x", []dotcontext.Dot{
		{ID: "B", Seq: 2},
	}, "step 8: only (B,2) survives — the concurrent add")
}

// ── Test helpers ────────────────────────────────────────────────────

func assertHasElem[E comparable](t *testing.T, s *AWSet[E], elem E, msg string) {
	t.Helper()
	if !s.Has(elem) {
		t.Fatalf("%s: element not found", msg)
	}
}

func assertNotHasElem[E comparable](t *testing.T, s *AWSet[E], elem E, msg string) {
	t.Helper()
	if s.Has(elem) {
		t.Fatalf("%s: element should not be present", msg)
	}
}

// assertDots checks that the given element's dot set contains exactly the
// expected dots (order-independent).
func assertDots[E comparable](t *testing.T, s *AWSet[E], elem E, want []dotcontext.Dot, msg string) {
	t.Helper()
	ds, ok := s.state.Store.Get(elem)
	if !ok {
		if len(want) == 0 {
			return
		}
		t.Fatalf("%s: element not in store, want dots %v", msg, want)
	}

	got := make(map[dotcontext.Dot]struct{})
	ds.Range(func(d dotcontext.Dot) bool {
		got[d] = struct{}{}
		return true
	})

	if len(got) != len(want) {
		t.Fatalf("%s: got %d dots %v, want %d dots %v", msg, len(got), got, len(want), want)
	}
	for _, d := range want {
		if _, ok := got[d]; !ok {
			t.Fatalf("%s: missing dot %v in %v", msg, d, got)
		}
	}
}

// assertContext checks the version vector portion of the causal context.
func assertContext[E comparable](t *testing.T, s *AWSet[E], wantVV map[string]uint64, msg string) {
	t.Helper()
	ctx := s.state.Context
	for id, wantSeq := range wantVV {
		if got := ctx.Max(id); got != wantSeq {
			t.Fatalf("%s: context[%s] = %d, want %d", msg, id, got, wantSeq)
		}
	}
}
