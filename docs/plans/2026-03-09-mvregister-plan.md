# MVRegister Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `mvregister` package — a multi-value register CRDT where concurrent writes are all preserved.

**Architecture:** `Causal[*DotFun[Value[V]]]` where `Value[V]` is a trivial lattice wrapper. Mutator structure identical to `LWWRegister.Set` minus the timestamp. Query returns all coexisting values instead of picking a winner.

**Tech Stack:** Go, `dotcontext` package (zero external dependencies), `quicktest` for tests.

---

### Task 1: Package scaffold + Value wrapper + New constructor

**Files:**
- Create: `mvregister/doc.go`
- Create: `mvregister/mvregister.go`
- Create: `mvregister/mvregister_test.go`

**Step 1: Write the failing test**

In `mvregister/mvregister_test.go`:

```go
package mvregister

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		r := New[string]("a")
		c.Assert(r.Values(), qt.HasLen, 0)
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./mvregister/ -run TestBasicOps/NewEmpty -v`
Expected: FAIL — package does not exist.

**Step 3: Write minimal implementation**

In `mvregister/doc.go`:

```go
// Package mvregister implements a multi-value register, a delta-state
// CRDT where concurrent writes are all preserved rather than resolved
// by a total order.
//
// The implementation composes DotFun[Value[V]] from the dotcontext
// package. Each write generates a new dot and associates it with the
// value. When concurrent writes produce multiple surviving dots after
// merge, the read query returns all coexisting values. A subsequent
// write from any replica replaces them with a single new value.
//
// Mutators return deltas suitable for replication. Merge incorporates
// a delta or full state from another replica.
package mvregister
```

In `mvregister/mvregister.go`:

```go
package mvregister

import (
	"github.com/aalpar/crdt/dotcontext"
)

// Value wraps a user value to satisfy the dotcontext.Lattice constraint.
// For MVRegister, the join is trivial: two entries sharing the same dot
// always hold the same value, so Join simply returns the receiver.
type Value[V any] struct {
	V V
}

func (v Value[V]) Join(other Value[V]) Value[V] { return v }

// MVRegister is a multi-value register. Concurrent writes are all
// preserved — Values() returns every concurrently-written value.
// A subsequent write from any replica supersedes all existing values.
//
// Internally it is a DotFun[Value[V]] paired with a causal context.
type MVRegister[V any] struct {
	id    dotcontext.ReplicaID
	state dotcontext.Causal[*dotcontext.DotFun[Value[V]]]
}

// New creates an empty MVRegister for the given replica.
func New[V any](replicaID dotcontext.ReplicaID) *MVRegister[V] {
	return &MVRegister[V]{
		id: replicaID,
		state: dotcontext.Causal[*dotcontext.DotFun[Value[V]]]{
			Store:   dotcontext.NewDotFun[Value[V]](),
			Context: dotcontext.New(),
		},
	}
}

// Values returns all concurrently-written values. In quiescent state
// (no unmerged concurrent writes), this returns zero or one value.
// The order is non-deterministic.
func (r *MVRegister[V]) Values() []V {
	var vals []V
	r.state.Store.Range(func(_ dotcontext.Dot, entry Value[V]) bool {
		vals = append(vals, entry.V)
		return true
	})
	return vals
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./mvregister/ -run TestBasicOps/NewEmpty -v`
Expected: PASS

**Step 5: Commit**

```
git add mvregister/
git commit -m "feat(mvregister): package scaffold with Value wrapper and New constructor"
```

---

### Task 2: Write mutator + basic tests

**Files:**
- Modify: `mvregister/mvregister.go`
- Modify: `mvregister/mvregister_test.go`

**Step 1: Write the failing tests**

Add to `TestBasicOps` in `mvregister/mvregister_test.go`:

```go
	c.Run("WriteAndValues", func(c *qt.C) {
		r := New[string]("a")
		r.Write("hello")

		vals := r.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "hello")
	})

	c.Run("OverwriteReplaces", func(c *qt.C) {
		r := New[string]("a")
		r.Write("first")
		r.Write("second")

		vals := r.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "second")
	})

	c.Run("OverwriteCleansStore", func(c *qt.C) {
		r := New[string]("a")
		r.Write("first")
		r.Write("second")
		r.Write("third")

		// Store should have exactly one dot, not three.
		c.Assert(r.state.Store.Len(), qt.Equals, 1)
	})
```

**Step 2: Run tests to verify they fail**

Run: `go test ./mvregister/ -run TestBasicOps -v`
Expected: FAIL — `Write` method not defined.

**Step 3: Write the Write mutator**

Add to `mvregister/mvregister.go`:

```go
// Write sets the register value and returns a delta for replication.
//
// All previous values are superseded: their dots are recorded in the
// delta's context so that remote replicas remove them on merge.
func (r *MVRegister[V]) Write(v V) *MVRegister[V] {
	// Collect old dots before generating the new one.
	var oldDots []dotcontext.Dot
	r.state.Store.Range(func(d dotcontext.Dot, _ Value[V]) bool {
		oldDots = append(oldDots, d)
		return true
	})

	// Generate new dot and write locally.
	d := r.state.Context.Next(r.id)
	for _, od := range oldDots {
		r.state.Store.Remove(od)
	}
	entry := Value[V]{V: v}
	r.state.Store.Set(d, entry)

	// Build delta.
	deltaStore := dotcontext.NewDotFun[Value[V]]()
	deltaStore.Set(d, entry)

	deltaCtx := dotcontext.New()
	deltaCtx.Add(d)
	for _, od := range oldDots {
		deltaCtx.Add(od)
	}

	return &MVRegister[V]{
		state: dotcontext.Causal[*dotcontext.DotFun[Value[V]]]{
			Store:   deltaStore,
			Context: deltaCtx,
		},
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./mvregister/ -run TestBasicOps -v`
Expected: PASS

**Step 5: Commit**

```
git add mvregister/
git commit -m "feat(mvregister): Write mutator — generates dot, supersedes old values"
```

---

### Task 3: Merge + delta return + conflict semantics tests

**Files:**
- Modify: `mvregister/mvregister.go`
- Modify: `mvregister/mvregister_test.go`

**Step 1: Write failing tests**

Add to `mvregister/mvregister_test.go`:

```go
func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Write", func(c *qt.C) {
		r := New[string]("a")
		delta := r.Write("x")

		vals := delta.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "x")
	})
}

func TestConflictResolution(t *testing.T) {
	c := qt.New(t)

	c.Run("ConcurrentWritesPreserved", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Write("from-a")
		db := b.Write("from-b")

		a.Merge(db)
		b.Merge(da)

		// Both values survive — no winner.
		for _, r := range []*MVRegister[string]{a, b} {
			vals := r.Values()
			c.Assert(vals, qt.HasLen, 2)
			c.Assert(vals, qt.ContentEquals, []string{"from-a", "from-b"})
		}
	})

	c.Run("SequentialWriteResolves", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Write("from-a")
		db := b.Write("from-b")

		// Both merge — now concurrent values coexist.
		a.Merge(db)
		b.Merge(da)

		// a writes again, superseding both concurrent values.
		d3 := a.Write("resolved")
		b.Merge(d3)

		for _, r := range []*MVRegister[string]{a, b} {
			vals := r.Values()
			c.Assert(vals, qt.HasLen, 1)
			c.Assert(vals[0], qt.Equals, "resolved")
		}
	})

	c.Run("ThreeWayConcurrentPreserved", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		da := a.Write("from-a")
		db := b.Write("from-b")
		dx := x.Write("from-c")

		a.Merge(db)
		a.Merge(dx)
		b.Merge(da)
		b.Merge(dx)
		x.Merge(da)
		x.Merge(db)

		for _, r := range []*MVRegister[string]{a, b, x} {
			vals := r.Values()
			c.Assert(vals, qt.HasLen, 3)
			c.Assert(vals, qt.ContentEquals, []string{"from-a", "from-b", "from-c"})
		}
	})
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./mvregister/ -run "TestDeltaReturn|TestConflictResolution" -v`
Expected: FAIL — `Merge` method not defined.

**Step 3: Write Merge**

Add to `mvregister/mvregister.go`:

```go
// Merge incorporates a delta or full state from another register.
func (r *MVRegister[V]) Merge(other *MVRegister[V]) {
	r.state = dotcontext.JoinDotFun(r.state, other.state)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./mvregister/ -run "TestDeltaReturn|TestConflictResolution" -v`
Expected: PASS

**Step 5: Commit**

```
git add mvregister/
git commit -m "feat(mvregister): Merge + conflict resolution — concurrent writes preserved"
```

---

### Task 4: Semilattice properties + delta propagation + edge cases

**Files:**
- Modify: `mvregister/mvregister_test.go`

**Step 1: Write the tests**

Add to `mvregister/mvregister_test.go`:

```go
func TestMergeProperties(t *testing.T) {
	c := qt.New(t)

	c.Run("Idempotent", func(c *qt.C) {
		a := New[string]("a")
		a.Write("x")

		snapshot := New[string]("a")
		snapshot.Merge(a)

		a.Merge(snapshot)
		a.Merge(snapshot)

		c.Assert(a.Values(), qt.HasLen, 1)
		c.Assert(a.Values()[0], qt.Equals, "x")
	})

	c.Run("Commutative", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		a.Write("va")
		b.Write("vb")

		ab := New[string]("x")
		ab.Merge(a)
		ab.Merge(b)

		ba := New[string]("x")
		ba.Merge(b)
		ba.Merge(a)

		valsAB := ab.Values()
		valsBA := ba.Values()
		sort.Strings(valsAB)
		sort.Strings(valsBA)
		c.Assert(valsAB, qt.DeepEquals, valsBA)
	})

	c.Run("Associative", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		a.Write("va")
		b.Write("vb")
		x.Write("vc")

		// (a ⊔ b) ⊔ c
		ab := New[string]("ab")
		ab.Merge(a)
		ab.Merge(b)
		abc := New[string]("abc")
		abc.Merge(ab)
		abc.Merge(x)

		// a ⊔ (b ⊔ c)
		bc := New[string]("bc")
		bc.Merge(b)
		bc.Merge(x)
		abc2 := New[string]("abc2")
		abc2.Merge(a)
		abc2.Merge(bc)

		vals1 := abc.Values()
		vals2 := abc2.Values()
		sort.Strings(vals1)
		sort.Strings(vals2)
		c.Assert(vals1, qt.DeepEquals, vals2)
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("IncrementalEqualsFullMerge", func(c *qt.C) {
		a := New[string]("a")
		d1 := a.Write("first")
		d2 := a.Write("second")

		inc := New[string]("b")
		inc.Merge(d1)
		inc.Merge(d2)

		full := New[string]("b")
		full.Merge(a)

		incVals := inc.Values()
		fullVals := full.Values()
		sort.Strings(incVals)
		sort.Strings(fullVals)
		c.Assert(incVals, qt.DeepEquals, fullVals)
	})

	c.Run("OverwriteDeltaSupersedes", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		d1 := a.Write("first")
		b.Merge(d1)

		d2 := a.Write("second")
		b.Merge(d2)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "second")
	})

	c.Run("DeltaDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		d1 := a.Write("first")
		d2 := a.Write("second")

		// Combine deltas, then apply.
		d1.Merge(d2)

		b := New[string]("b")
		b.Merge(d1)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "second")
	})

	c.Run("ConcurrentWriteThenOverwrite", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Write("from-a")
		db := b.Write("from-b")

		a.Merge(db)
		b.Merge(da)

		d3 := a.Write("from-a-again")
		b.Merge(d3)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "from-a-again")
	})
}

func TestMergeWithEmpty(t *testing.T) {
	c := qt.New(t)

	c.Run("IntoEmpty", func(c *qt.C) {
		a := New[string]("a")
		a.Write("hello")

		b := New[string]("b")
		b.Merge(a)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "hello")
	})

	c.Run("EmptyIntoSet", func(c *qt.C) {
		a := New[string]("a")
		a.Write("hello")

		empty := New[string]("b")
		a.Merge(empty)

		vals := a.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "hello")
	})
}
```

Note: add `"sort"` to the imports.

**Step 2: Run tests**

Run: `go test ./mvregister/ -v`
Expected: PASS (Merge already implemented in Task 3)

**Step 3: Commit**

```
git add mvregister/
git commit -m "test(mvregister): semilattice properties, delta propagation, edge cases"
```

---

### Task 5: State/FromCausal round-trip + convergence tests

**Files:**
- Modify: `mvregister/mvregister.go`
- Modify: `mvregister/mvregister_test.go`

**Step 1: Write failing tests**

Add to `mvregister/mvregister_test.go`:

```go
func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New[string]("a")
		a.Write("hello")

		state := a.State()
		b := FromCausal[string](state)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "hello")
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		delta := a.Write("x")

		reconstructed := FromCausal[string](delta.State())

		b := New[string]("b")
		b.Merge(reconstructed)

		vals := b.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "x")
	})
}

func TestConvergence(t *testing.T) {
	c := qt.New(t)

	c.Run("ThreeReplica", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		da := a.Write("va")
		db := b.Write("vb")
		dx := x.Write("vc")

		a.Merge(db)
		a.Merge(dx)
		b.Merge(da)
		b.Merge(dx)
		x.Merge(da)
		x.Merge(db)

		// All converge to the same set of 3 concurrent values.
		expected := []string{"va", "vb", "vc"}
		for _, r := range []*MVRegister[string]{a, b, x} {
			vals := r.Values()
			sort.Strings(vals)
			c.Assert(vals, qt.DeepEquals, expected)
		}
	})

	c.Run("FiveReplica", func(c *qt.C) {
		ids := []dotcontext.ReplicaID{"a", "b", "c", "d", "e"}
		replicas := make([]*MVRegister[string], len(ids))
		for i, id := range ids {
			replicas[i] = New[string](id)
		}

		deltas := make([]*MVRegister[string], len(ids))
		deltas[0] = replicas[0].Write("from-a")
		deltas[1] = replicas[1].Write("from-b")
		deltas[2] = replicas[2].Write("from-c")
		deltas[3] = replicas[3].Write("from-d")
		deltas[4] = replicas[4].Write("from-e")

		// Full mesh merge.
		for i := range replicas {
			for j := range replicas {
				if i != j {
					replicas[i].Merge(deltas[j])
				}
			}
		}

		// All converge to the same 5 concurrent values.
		expected := []string{"from-a", "from-b", "from-c", "from-d", "from-e"}
		sort.Strings(expected)
		for i, r := range replicas {
			vals := r.Values()
			sort.Strings(vals)
			c.Assert(vals, qt.DeepEquals, expected, qt.Commentf("replica %s", ids[i]))
		}
	})
}
```

Note: add `"github.com/aalpar/crdt/dotcontext"` to imports.

**Step 2: Run tests to verify they fail**

Run: `go test ./mvregister/ -run "TestStateRoundTrip|TestConvergence" -v`
Expected: FAIL — `State` and `FromCausal` not defined.

**Step 3: Write State and FromCausal**

Add to `mvregister/mvregister.go`:

```go
// State returns the MVRegister's internal Causal state for serialization.
func (r *MVRegister[V]) State() dotcontext.Causal[*dotcontext.DotFun[Value[V]]] {
	return r.state
}

// FromCausal constructs an MVRegister from a decoded Causal value.
// Used to reconstruct deltas from the wire for merging.
func FromCausal[V any](state dotcontext.Causal[*dotcontext.DotFun[Value[V]]]) *MVRegister[V] {
	return &MVRegister[V]{state: state}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./mvregister/ -v`
Expected: PASS

**Step 5: Commit**

```
git add mvregister/
git commit -m "feat(mvregister): State/FromCausal round-trip + convergence tests"
```

---

### Task 6: E2E replication test

**Files:**
- Modify: `replication/e2e_test.go`

**Step 1: Write the e2e test**

Add to `replication/e2e_test.go` — a codec for `Value[string]`, a delta codec, and the e2e test:

```go
// --- MVRegister codec ---

type mvrValueStringCodec struct{}

func (mvrValueStringCodec) Encode(w io.Writer, v mvregister.Value[string]) error {
	return (dotcontext.StringCodec{}).Encode(w, v.V)
}

func (mvrValueStringCodec) Decode(r io.Reader) (mvregister.Value[string], error) {
	s, err := (dotcontext.StringCodec{}).Decode(r)
	return mvregister.Value[string]{V: s}, err
}

type mvrDeltaCodec struct {
	inner dotcontext.CausalCodec[*dotcontext.DotFun[mvregister.Value[string]]]
}

func newMVRDeltaCodec() mvrDeltaCodec {
	return mvrDeltaCodec{
		inner: dotcontext.CausalCodec[*dotcontext.DotFun[mvregister.Value[string]]]{
			StoreCodec: dotcontext.DotFunCodec[mvregister.Value[string]]{
				ValueCodec: mvrValueStringCodec{},
			},
		},
	}
}

func (c mvrDeltaCodec) Encode(w io.Writer, v dotcontext.Causal[*dotcontext.DotFun[mvregister.Value[string]]]) error {
	return c.inner.Encode(w, v)
}

func (c mvrDeltaCodec) Decode(r io.Reader) (dotcontext.Causal[*dotcontext.DotFun[mvregister.Value[string]]], error) {
	return c.inner.Decode(r)
}

func TestE2EMVRegisterAcrossWire(t *testing.T) {
	c := qt.New(t)
	codec := newMVRDeltaCodec()

	alice := mvregister.New[string]("alice")
	bob := mvregister.New[string]("bob")

	// Concurrent writes.
	aliceDelta := alice.Write("from-alice")
	bobDelta := bob.Write("from-bob")

	// Encode both deltas.
	var bufA, bufB bytes.Buffer
	c.Assert(codec.Encode(&bufA, aliceDelta.State()), qt.IsNil)
	c.Assert(codec.Encode(&bufB, bobDelta.State()), qt.IsNil)

	// Cross-merge.
	decodedA, err := codec.Decode(&bufA)
	c.Assert(err, qt.IsNil)
	bob.Merge(mvregister.FromCausal[string](decodedA))

	decodedB, err := codec.Decode(&bufB)
	c.Assert(err, qt.IsNil)
	alice.Merge(mvregister.FromCausal[string](decodedB))

	// Both converge to both values (no winner — all preserved).
	expected := []string{"from-alice", "from-bob"}
	for _, r := range []*mvregister.MVRegister[string]{alice, bob} {
		vals := r.Values()
		sort.Strings(vals)
		c.Assert(vals, qt.DeepEquals, expected)
	}
}
```

Add `"github.com/aalpar/crdt/mvregister"` to the imports.

**Step 2: Run test**

Run: `go test ./replication/ -run TestE2EMVRegisterAcrossWire -v`
Expected: PASS

**Step 3: Commit**

```
git add replication/e2e_test.go
git commit -m "test(replication): e2e MVRegister encode→decode→merge with concurrent values"
```

---

### Task 7: Update TODO.md + full suite verification

**Files:**
- Modify: `TODO.md`

**Step 1: Run full test suite**

Run: `make lint && make && make test`
Expected: All pass.

**Step 2: Run race detector**

Run: `go test -race ./...`
Expected: No races.

**Step 3: Update TODO.md**

Mark `mvregister` as done: `- [x] `mvregister` — multi-value register (concurrent writes preserved, not LWW)`

Update the test count if it changed.

**Step 4: Commit**

```
git add TODO.md
git commit -m "docs: mark mvregister complete in TODO"
```
