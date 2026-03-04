package lwwregister

import (
	"testing"
)

func TestNewEmpty(t *testing.T) {
	r := New[string]("a")
	_, _, ok := r.Value()
	if ok {
		t.Fatal("new register reports a value")
	}
}

func TestSetAndValue(t *testing.T) {
	r := New[string]("a")
	r.Set("hello", 1)

	v, ts, ok := r.Value()
	if !ok {
		t.Fatal("register empty after Set")
	}
	if v != "hello" || ts != 1 {
		t.Fatalf("Value() = (%q, %d), want (\"hello\", 1)", v, ts)
	}
}

func TestOverwrite(t *testing.T) {
	r := New[string]("a")
	r.Set("first", 1)
	r.Set("second", 2)

	v, ts, ok := r.Value()
	if !ok {
		t.Fatal("register empty after overwrite")
	}
	if v != "second" || ts != 2 {
		t.Fatalf("Value() = (%q, %d), want (\"second\", 2)", v, ts)
	}
}

func TestSetReturnsValidDelta(t *testing.T) {
	r := New[string]("a")
	delta := r.Set("x", 10)

	v, ts, ok := delta.Value()
	if !ok {
		t.Fatal("delta has no value")
	}
	if v != "x" || ts != 10 {
		t.Fatalf("delta Value() = (%q, %d), want (\"x\", 10)", v, ts)
	}
}

// --- Concurrent write resolution ---

func TestConcurrentWriteHigherTimestampWins(t *testing.T) {
	a := New[string]("a")
	b := New[string]("b")

	da := a.Set("from-a", 10)
	db := b.Set("from-b", 20)

	// Cross-merge.
	a.Merge(db)
	b.Merge(da)

	// Both should resolve to "from-b" (higher timestamp).
	for _, r := range []*LWWRegister[string]{a, b} {
		v, ts, ok := r.Value()
		if !ok {
			t.Fatal("register empty after merge")
		}
		if v != "from-b" || ts != 20 {
			t.Fatalf("Value() = (%q, %d), want (\"from-b\", 20)", v, ts)
		}
	}
}

func TestConcurrentWriteSameTimestampTiebreak(t *testing.T) {
	a := New[string]("a")
	b := New[string]("b")

	da := a.Set("from-a", 10)
	db := b.Set("from-b", 10) // same timestamp

	a.Merge(db)
	b.Merge(da)

	// Tiebreak: higher replica ID wins. "b" > "a" lexicographically.
	for _, r := range []*LWWRegister[string]{a, b} {
		v, _, ok := r.Value()
		if !ok {
			t.Fatal("register empty after merge")
		}
		if v != "from-b" {
			t.Fatalf("Value() = %q, want \"from-b\" (replica b wins tiebreak)", v)
		}
	}
}

// --- Merge properties ---

func TestMergeIdempotent(t *testing.T) {
	a := New[string]("a")
	a.Set("x", 5)

	snapshot := New[string]("a")
	snapshot.Merge(a)

	a.Merge(snapshot)
	a.Merge(snapshot)

	v, ts, ok := a.Value()
	if !ok || v != "x" || ts != 5 {
		t.Fatalf("after idempotent merge: (%q, %d, %v), want (\"x\", 5, true)", v, ts, ok)
	}
}

func TestMergeCommutative(t *testing.T) {
	a := New[string]("a")
	b := New[string]("b")
	a.Set("va", 1)
	b.Set("vb", 2)

	ab := New[string]("x")
	ab.Merge(a)
	ab.Merge(b)

	ba := New[string]("x")
	ba.Merge(b)
	ba.Merge(a)

	vAB, tsAB, _ := ab.Value()
	vBA, tsBA, _ := ba.Value()
	if vAB != vBA || tsAB != tsBA {
		t.Fatalf("not commutative: (%q,%d) vs (%q,%d)", vAB, tsAB, vBA, tsBA)
	}
}

func TestDeltaIncrementalEqualsFullMerge(t *testing.T) {
	a := New[string]("a")
	d1 := a.Set("first", 1)
	d2 := a.Set("second", 2)

	// Incremental: apply deltas one by one.
	inc := New[string]("b")
	inc.Merge(d1)
	inc.Merge(d2)

	// Full: merge entire state.
	full := New[string]("b")
	full.Merge(a)

	vInc, tsInc, _ := inc.Value()
	vFull, tsFull, _ := full.Value()
	if vInc != vFull || tsInc != tsFull {
		t.Fatalf("incremental (%q,%d) != full (%q,%d)", vInc, tsInc, vFull, tsFull)
	}
}

// --- Overwrite propagation ---

func TestOverwriteDeltaSupersedes(t *testing.T) {
	a := New[string]("a")
	b := New[string]("b")

	// a sets "first", shares with b.
	d1 := a.Set("first", 1)
	b.Merge(d1)

	// a overwrites with "second", shares delta with b.
	d2 := a.Set("second", 2)
	b.Merge(d2)

	v, ts, ok := b.Value()
	if !ok || v != "second" || ts != 2 {
		t.Fatalf("b.Value() = (%q, %d, %v), want (\"second\", 2, true)", v, ts, ok)
	}
}

func TestConcurrentWriteThenOverwrite(t *testing.T) {
	a := New[string]("a")
	b := New[string]("b")

	// Concurrent writes.
	da := a.Set("from-a", 10)
	db := b.Set("from-b", 20)

	// b wins (higher timestamp). Both merge.
	a.Merge(db)
	b.Merge(da)

	// a overwrites with even higher timestamp.
	d3 := a.Set("from-a-again", 30)
	b.Merge(d3)

	v, ts, ok := b.Value()
	if !ok || v != "from-a-again" || ts != 30 {
		t.Fatalf("b.Value() = (%q, %d, %v), want (\"from-a-again\", 30, true)", v, ts, ok)
	}
}

// --- Integer values ---

func TestIntegerRegister(t *testing.T) {
	r := New[int]("a")
	r.Set(42, 1)
	r.Set(99, 2)

	v, _, ok := r.Value()
	if !ok || v != 99 {
		t.Fatalf("Value() = (%d, _, %v), want (99, _, true)", v, ok)
	}
}

// --- Three-replica convergence ---

func TestThreeReplicaConvergence(t *testing.T) {
	a := New[string]("a")
	b := New[string]("b")
	c := New[string]("c")

	da := a.Set("va", 1)
	db := b.Set("vb", 2)
	dc := c.Set("vc", 3) // highest timestamp

	// Each merges the other two.
	a.Merge(db)
	a.Merge(dc)
	b.Merge(da)
	b.Merge(dc)
	c.Merge(da)
	c.Merge(db)

	for name, r := range map[string]*LWWRegister[string]{"a": a, "b": b, "c": c} {
		v, ts, ok := r.Value()
		if !ok || v != "vc" || ts != 3 {
			t.Fatalf("%s: Value() = (%q, %d, %v), want (\"vc\", 3, true)", name, v, ts, ok)
		}
	}
}
