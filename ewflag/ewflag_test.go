package ewflag

import (
	"testing"
)

func TestNewDisabled(t *testing.T) {
	f := New("a")
	if f.Value() {
		t.Fatal("new flag is enabled, want disabled")
	}
}

func TestEnableDisable(t *testing.T) {
	f := New("a")
	f.Enable()
	if !f.Value() {
		t.Fatal("flag disabled after Enable")
	}
	f.Disable()
	if f.Value() {
		t.Fatal("flag enabled after Disable")
	}
}

func TestReEnable(t *testing.T) {
	f := New("a")
	f.Enable()
	f.Disable()
	f.Enable()
	if !f.Value() {
		t.Fatal("flag disabled after re-Enable")
	}
}

func TestEnableReturnsValidDelta(t *testing.T) {
	f := New("a")
	delta := f.Enable()
	if !delta.Value() {
		t.Fatal("enable delta is disabled")
	}
}

func TestDisableReturnsValidDelta(t *testing.T) {
	f := New("a")
	f.Enable()
	delta := f.Disable()
	if delta.Value() {
		t.Fatal("disable delta is enabled")
	}
}

func TestDisableWhenAlreadyDisabled(t *testing.T) {
	f := New("a")
	f.Disable() // should not panic
	if f.Value() {
		t.Fatal("flag enabled after Disable on already-disabled flag")
	}
}

// --- Enable-wins semantics ---

// Two replicas start with the flag enabled (both have seen the same dot).
// Concurrently, one disables and the other enables.
// After exchanging deltas, both should converge to enabled.
//
// Hint: follow the pattern in awset TestAddWinsConcurrent.
// Set up two replicas, sync them, then do concurrent ops and cross-merge.

func TestConcurrentEnables(t *testing.T) {
	a := New("a")
	b := New("b")

	da := a.Enable()
	db := b.Enable()

	a.Merge(db)
	b.Merge(da)

	if !a.Value() {
		t.Fatal("a: disabled after concurrent enables")
	}
	if !b.Value() {
		t.Fatal("b: disabled after concurrent enables")
	}
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	a := New("a")
	a.Enable()

	snapshot := New("a")
	snapshot.Merge(a)

	a.Merge(snapshot)
	a.Merge(snapshot)

	if !a.Value() {
		t.Fatal("flag disabled after idempotent merge")
	}
}

func TestMergeCommutative(t *testing.T) {
	a := New("a")
	b := New("b")
	a.Enable()
	// b stays disabled

	ab := New("x")
	ab.Merge(a)
	ab.Merge(b)

	ba := New("x")
	ba.Merge(b)
	ba.Merge(a)

	if ab.Value() != ba.Value() {
		t.Fatalf("merge not commutative: ab=%v, ba=%v", ab.Value(), ba.Value())
	}
}

func TestDeltaIncrementalEqualsFullMerge(t *testing.T) {
	a := New("a")
	d1 := a.Enable()
	d2 := a.Disable()
	d3 := a.Enable()

	incremental := New("b")
	incremental.Merge(d1)
	incremental.Merge(d2)
	incremental.Merge(d3)

	full := New("b")
	full.Merge(a)

	if incremental.Value() != full.Value() {
		t.Fatalf("incremental=%v != full=%v", incremental.Value(), full.Value())
	}
}

func TestMergeDisableDelta(t *testing.T) {
	a := New("a")
	b := New("b")

	enableDelta := a.Enable()
	b.Merge(enableDelta)

	disableDelta := a.Disable()
	b.Merge(disableDelta)

	if b.Value() {
		t.Fatal("b still enabled after receiving disable delta")
	}
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	a := New("a")
	b := New("b")
	c := New("c")

	da := a.Enable()
	// b stays disabled
	dc := c.Enable()

	a.Merge(b)
	a.Merge(dc)
	b.Merge(da)
	b.Merge(dc)
	c.Merge(da)
	c.Merge(b)

	if a.Value() != b.Value() || b.Value() != c.Value() {
		t.Fatalf("not converged: a=%v, b=%v, c=%v", a.Value(), b.Value(), c.Value())
	}
	if !a.Value() {
		t.Fatal("converged to disabled, want enabled (two of three enabled)")
	}
}
