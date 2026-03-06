# DeltaStore Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement `DeltaStore[T]` — an in-memory delta buffer indexed by `Dot`, with range queries composable with `Missing()`.

**Architecture:** Generic struct in `dotcontext/` with `map[Dot]T` internals. Five methods: `Add`, `Get`, `Fetch`, `Remove`, `Len`. No dependencies on layer-2 CRDTs.

**Tech Stack:** Go stdlib only. Generics (`T any`). Tests use `testing` package — no helpers beyond what exists.

---

### Task 1: Empty store — NewDeltaStore and Len

**Files:**
- Create: `dotcontext/deltastore.go`
- Create: `dotcontext/deltastore_test.go`

**Step 1: Write the failing test**

In `dotcontext/deltastore_test.go`:

```go
package dotcontext

import "testing"

func TestDeltaStoreNewEmpty(t *testing.T) {
	s := NewDeltaStore[int]()
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run TestDeltaStoreNewEmpty -v`
Expected: FAIL — `NewDeltaStore` not defined.

**Step 3: Write minimal implementation**

In `dotcontext/deltastore.go`:

```go
package dotcontext

// DeltaStore is an in-memory buffer of deltas indexed by the dot that
// created them. It supports range queries composable with Missing().
type DeltaStore[T any] struct {
	deltas map[Dot]T
}

// NewDeltaStore returns an empty DeltaStore.
func NewDeltaStore[T any]() *DeltaStore[T] {
	return &DeltaStore[T]{deltas: make(map[Dot]T)}
}

// Len returns the number of stored deltas.
func (s *DeltaStore[T]) Len() int {
	return len(s.deltas)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run TestDeltaStoreNewEmpty -v`
Expected: PASS

**Step 5: Commit**

```
git add dotcontext/deltastore.go dotcontext/deltastore_test.go
git commit -m "dotcontext: add DeltaStore with NewDeltaStore and Len"
```

---

### Task 2: Add and Get

**Files:**
- Modify: `dotcontext/deltastore.go`
- Modify: `dotcontext/deltastore_test.go`

**Step 1: Write the failing test**

Append to `dotcontext/deltastore_test.go`:

```go
func TestDeltaStoreAddGet(t *testing.T) {
	s := NewDeltaStore[string]()
	d := Dot{ID: "a", Seq: 1}

	s.Add(d, "delta-1")

	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1", s.Len())
	}

	got, ok := s.Get(d)
	if !ok {
		t.Fatal("Get returned !ok for stored dot")
	}
	if got != "delta-1" {
		t.Errorf("Get() = %q, want %q", got, "delta-1")
	}
}

func TestDeltaStoreGetMissing(t *testing.T) {
	s := NewDeltaStore[string]()
	_, ok := s.Get(Dot{ID: "x", Seq: 99})
	if ok {
		t.Error("Get returned ok for absent dot")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run TestDeltaStore -v`
Expected: FAIL — `Add` and `Get` not defined.

**Step 3: Write minimal implementation**

Append to `dotcontext/deltastore.go`:

```go
// Add stores a delta indexed by the dot that created it.
func (s *DeltaStore[T]) Add(d Dot, delta T) {
	s.deltas[d] = delta
}

// Get retrieves a single delta by its dot.
func (s *DeltaStore[T]) Get(d Dot) (T, bool) {
	v, ok := s.deltas[d]
	return v, ok
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run TestDeltaStore -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```
git add dotcontext/deltastore.go dotcontext/deltastore_test.go
git commit -m "dotcontext: DeltaStore Add and Get"
```

---

### Task 3: Remove

**Files:**
- Modify: `dotcontext/deltastore.go`
- Modify: `dotcontext/deltastore_test.go`

**Step 1: Write the failing test**

```go
func TestDeltaStoreRemove(t *testing.T) {
	s := NewDeltaStore[string]()
	d := Dot{ID: "a", Seq: 1}
	s.Add(d, "delta-1")
	s.Remove(d)

	if s.Len() != 0 {
		t.Errorf("Len() after Remove = %d, want 0", s.Len())
	}
	if _, ok := s.Get(d); ok {
		t.Error("Get returned ok after Remove")
	}
}

func TestDeltaStoreRemoveAbsent(t *testing.T) {
	s := NewDeltaStore[string]()
	s.Remove(Dot{ID: "x", Seq: 1}) // should not panic
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run TestDeltaStoreRemove -v`
Expected: FAIL — `Remove` not defined.

**Step 3: Write minimal implementation**

```go
// Remove deletes a delta by its dot.
func (s *DeltaStore[T]) Remove(d Dot) {
	delete(s.deltas, d)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run TestDeltaStoreRemove -v`
Expected: PASS

**Step 5: Commit**

```
git add dotcontext/deltastore.go dotcontext/deltastore_test.go
git commit -m "dotcontext: DeltaStore Remove"
```

---

### Task 4: Fetch — single replica, single range

**Files:**
- Modify: `dotcontext/deltastore.go`
- Modify: `dotcontext/deltastore_test.go`

**Step 1: Write the failing test**

```go
func TestDeltaStoreFetchSingleRange(t *testing.T) {
	s := NewDeltaStore[string]()
	s.Add(Dot{ID: "a", Seq: 1}, "d1")
	s.Add(Dot{ID: "a", Seq: 2}, "d2")
	s.Add(Dot{ID: "a", Seq: 3}, "d3")
	s.Add(Dot{ID: "a", Seq: 5}, "d5") // gap at 4

	missing := map[ReplicaID][]SeqRange{
		"a": {{Lo: 2, Hi: 4}},
	}
	got := s.Fetch(missing)

	// Should return a:2 and a:3. a:4 is not in store. a:1 and a:5 outside range.
	if len(got) != 2 {
		t.Fatalf("Fetch() returned %d deltas, want 2: %v", len(got), got)
	}
	if got[Dot{ID: "a", Seq: 2}] != "d2" {
		t.Error("missing a:2")
	}
	if got[Dot{ID: "a", Seq: 3}] != "d3" {
		t.Error("missing a:3")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run TestDeltaStoreFetchSingleRange -v`
Expected: FAIL — `Fetch` not defined.

**Step 3: Write minimal implementation**

```go
// Fetch returns all stored deltas whose dots fall within the given ranges.
// The input format matches Missing()'s return type for direct composability:
//
//	store.Fetch(local.Missing(remote))
func (s *DeltaStore[T]) Fetch(missing map[ReplicaID][]SeqRange) map[Dot]T {
	if len(missing) == 0 {
		return nil
	}
	result := make(map[Dot]T)
	for d, delta := range s.deltas {
		ranges, ok := missing[d.ID]
		if !ok {
			continue
		}
		for _, r := range ranges {
			if d.Seq >= r.Lo && d.Seq <= r.Hi {
				result[d] = delta
				break
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run TestDeltaStoreFetch -v`
Expected: PASS

**Step 5: Commit**

```
git add dotcontext/deltastore.go dotcontext/deltastore_test.go
git commit -m "dotcontext: DeltaStore Fetch with single-range test"
```

---

### Task 5: Fetch — multi-replica, multi-range, empty cases

**Files:**
- Modify: `dotcontext/deltastore_test.go`

**Step 1: Write tests for remaining Fetch cases**

```go
func TestDeltaStoreFetchMultiReplica(t *testing.T) {
	s := NewDeltaStore[string]()
	s.Add(Dot{ID: "a", Seq: 1}, "a1")
	s.Add(Dot{ID: "a", Seq: 3}, "a3")
	s.Add(Dot{ID: "b", Seq: 2}, "b2")
	s.Add(Dot{ID: "c", Seq: 1}, "c1")

	missing := map[ReplicaID][]SeqRange{
		"a": {{Lo: 1, Hi: 1}, {Lo: 3, Hi: 3}},
		"b": {{Lo: 1, Hi: 5}},
	}
	got := s.Fetch(missing)

	if len(got) != 3 {
		t.Fatalf("Fetch() returned %d deltas, want 3: %v", len(got), got)
	}
	if got[Dot{ID: "a", Seq: 1}] != "a1" {
		t.Error("missing a:1")
	}
	if got[Dot{ID: "a", Seq: 3}] != "a3" {
		t.Error("missing a:3")
	}
	if got[Dot{ID: "b", Seq: 2}] != "b2" {
		t.Error("missing b:2")
	}
	// c:1 should NOT be in result — c not in missing
	if _, ok := got[Dot{ID: "c", Seq: 1}]; ok {
		t.Error("c:1 should not be returned")
	}
}

func TestDeltaStoreFetchEmpty(t *testing.T) {
	s := NewDeltaStore[string]()
	s.Add(Dot{ID: "a", Seq: 1}, "a1")

	got := s.Fetch(nil)
	if got != nil {
		t.Errorf("Fetch(nil) = %v, want nil", got)
	}

	got = s.Fetch(map[ReplicaID][]SeqRange{})
	if got != nil {
		t.Errorf("Fetch(empty) = %v, want nil", got)
	}
}

func TestDeltaStoreFetchNoMatches(t *testing.T) {
	s := NewDeltaStore[string]()
	s.Add(Dot{ID: "a", Seq: 1}, "a1")

	missing := map[ReplicaID][]SeqRange{
		"b": {{Lo: 1, Hi: 10}},
	}
	got := s.Fetch(missing)
	if got != nil {
		t.Errorf("Fetch with no matches = %v, want nil", got)
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run TestDeltaStoreFetch -v`
Expected: PASS (all 4 Fetch tests)

**Step 3: Commit**

```
git add dotcontext/deltastore_test.go
git commit -m "dotcontext: DeltaStore Fetch multi-replica and edge-case tests"
```

---

### Task 6: Full suite + race detector

**Files:** None (verification only)

**Step 1: Run full test suite**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && make lint && make && make test`
Expected: All pass, no lint errors.

**Step 2: Run race detector**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test -race ./dotcontext/ -run TestDeltaStore -v`
Expected: PASS, no races.

**Step 3: Run all tests to verify nothing broke**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test -race ./...`
Expected: All packages pass.
