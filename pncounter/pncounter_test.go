package pncounter

import (
	"testing"
)

func TestNewZero(t *testing.T) {
	c := New("a")
	if v := c.Value(); v != 0 {
		t.Fatalf("Value() = %d, want 0", v)
	}
}

func TestIncrement(t *testing.T) {
	c := New("a")
	c.Increment(5)
	if v := c.Value(); v != 5 {
		t.Fatalf("Value() = %d, want 5", v)
	}
}

func TestMultipleIncrements(t *testing.T) {
	c := New("a")
	c.Increment(3)
	c.Increment(7)
	c.Increment(2)
	if v := c.Value(); v != 12 {
		t.Fatalf("Value() = %d, want 12", v)
	}
}

func TestDecrement(t *testing.T) {
	c := New("a")
	c.Increment(10)
	c.Decrement(4)
	if v := c.Value(); v != 6 {
		t.Fatalf("Value() = %d, want 6", v)
	}
}

func TestNegativeValue(t *testing.T) {
	c := New("a")
	c.Decrement(5)
	if v := c.Value(); v != -5 {
		t.Fatalf("Value() = %d, want -5", v)
	}
}

func TestIncrementReturnsValidDelta(t *testing.T) {
	c := New("a")
	delta := c.Increment(7)
	if v := delta.Value(); v != 7 {
		t.Fatalf("delta Value() = %d, want 7", v)
	}
}

// --- Concurrent operations ---

func TestConcurrentIncrements(t *testing.T) {
	a := New("a")
	b := New("b")

	da := a.Increment(3)
	db := b.Increment(5)

	a.Merge(db)
	b.Merge(da)

	// Both should see 3 + 5 = 8.
	for name, c := range map[string]*Counter{"a": a, "b": b} {
		if v := c.Value(); v != 8 {
			t.Fatalf("%s: Value() = %d, want 8", name, v)
		}
	}
}

func TestConcurrentIncrementDecrement(t *testing.T) {
	a := New("a")
	b := New("b")

	da := a.Increment(10)
	db := b.Decrement(3)

	a.Merge(db)
	b.Merge(da)

	for name, c := range map[string]*Counter{"a": a, "b": b} {
		if v := c.Value(); v != 7 {
			t.Fatalf("%s: Value() = %d, want 7", name, v)
		}
	}
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	a := New("a")
	a.Increment(5)

	snapshot := New("x")
	snapshot.Merge(a)

	a.Merge(snapshot)
	a.Merge(snapshot)

	if v := a.Value(); v != 5 {
		t.Fatalf("Value() = %d, want 5", v)
	}
}

func TestMergeCommutative(t *testing.T) {
	a := New("a")
	b := New("b")
	a.Increment(3)
	b.Increment(7)

	ab := New("x")
	ab.Merge(a)
	ab.Merge(b)

	ba := New("x")
	ba.Merge(b)
	ba.Merge(a)

	if ab.Value() != ba.Value() {
		t.Fatalf("not commutative: %d vs %d", ab.Value(), ba.Value())
	}
}

func TestDeltaIncrementalEqualsFullMerge(t *testing.T) {
	a := New("a")
	d1 := a.Increment(3)
	d2 := a.Increment(7)

	inc := New("b")
	inc.Merge(d1)
	inc.Merge(d2)

	full := New("b")
	full.Merge(a)

	if inc.Value() != full.Value() {
		t.Fatalf("incremental %d != full %d", inc.Value(), full.Value())
	}
}

// --- Delta propagation ---

func TestDeltaSupersedes(t *testing.T) {
	a := New("a")
	b := New("b")

	d1 := a.Increment(3)
	b.Merge(d1)

	d2 := a.Increment(7)
	b.Merge(d2)

	// b should see a's accumulated value (10), not 3 + 10.
	if v := b.Value(); v != 10 {
		t.Fatalf("Value() = %d, want 10", v)
	}
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	a := New("a")
	b := New("b")
	c := New("c")

	da := a.Increment(1)
	db := b.Increment(2)
	dc := c.Increment(3)

	a.Merge(db)
	a.Merge(dc)
	b.Merge(da)
	b.Merge(dc)
	c.Merge(da)
	c.Merge(db)

	for name, r := range map[string]*Counter{"a": a, "b": b, "c": c} {
		if v := r.Value(); v != 6 {
			t.Fatalf("%s: Value() = %d, want 6", name, v)
		}
	}
}

func TestThreeReplicaMixedOps(t *testing.T) {
	a := New("a")
	b := New("b")
	c := New("c")

	da := a.Increment(10)
	db := b.Decrement(3)
	dc := c.Increment(5)

	a.Merge(db)
	a.Merge(dc)
	b.Merge(da)
	b.Merge(dc)
	c.Merge(da)
	c.Merge(db)

	// 10 - 3 + 5 = 12
	for name, r := range map[string]*Counter{"a": a, "b": b, "c": c} {
		if v := r.Value(); v != 12 {
			t.Fatalf("%s: Value() = %d, want 12", name, v)
		}
	}
}

// --- Sequential operations after merge ---

func TestIncrementAfterMerge(t *testing.T) {
	a := New("a")
	b := New("b")

	da := a.Increment(5)
	b.Merge(da)

	// b increments its own counter.
	b.Increment(3)

	// b should see 5 (from a) + 3 (from b) = 8.
	if v := b.Value(); v != 8 {
		t.Fatalf("Value() = %d, want 8", v)
	}
}
