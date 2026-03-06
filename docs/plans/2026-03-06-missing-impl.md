# CausalContext.Missing() Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `Missing(remote)` to CausalContext — computes which dots remote has that the receiver doesn't, returned as compressed `SeqRange` per replica.

**Architecture:** New `SeqRange` type in `dot.go`. `Missing()` method and helpers in `context.go`. Three-phase algorithm: VV comparison, hole-punching from local outliers, remote outlier collection, then merge.

**Tech Stack:** Go stdlib only. `sort` package for range merging.

**Design doc:** `docs/plans/2026-03-06-missing-design.md`

---

### Task 1: SeqRange type

**Files:**
- Modify: `dotcontext/dot.go` (append after `Dot.String()`)
- Test: `dotcontext/dot_test.go`

**Step 1: Write the failing test**

In `dotcontext/dot_test.go`, add:

```go
func TestSeqRangeString(t *testing.T) {
	tcs := []struct {
		in0 SeqRange
		out string
	}{
		{in0: SeqRange{Lo: 1, Hi: 1}, out: "[1,1]"},
		{in0: SeqRange{Lo: 3, Hi: 7}, out: "[3,7]"},
	}
	for _, tc := range tcs {
		got := tc.in0.String()
		if got != tc.out {
			t.Errorf("SeqRange%v.String() = %q, want %q", tc.in0, got, tc.out)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dotcontext/ -run TestSeqRangeString -v`
Expected: FAIL — `SeqRange` undefined.

**Step 3: Write minimal implementation**

In `dotcontext/dot.go`, after `Dot.String()`:

```go
// SeqRange is an inclusive range of sequence numbers [Lo, Hi].
type SeqRange struct {
	Lo, Hi uint64
}

// String returns "[Lo,Hi]".
func (p SeqRange) String() string {
	return fmt.Sprintf("[%d,%d]", p.Lo, p.Hi)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./dotcontext/ -run TestSeqRangeString -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dotcontext/dot.go dotcontext/dot_test.go
git commit -m "dotcontext: add SeqRange type"
```

---

### Task 2: mergeRanges helper

Internal helper used by `Missing()`. Sorts ranges by Lo, merges overlapping and adjacent ranges.

**Files:**
- Modify: `dotcontext/context.go` (append at end)
- Test: `dotcontext/context_test.go`

**Step 1: Write the failing test**

In `dotcontext/context_test.go`, add:

```go
func TestMergeRanges(t *testing.T) {
	tcs := []struct {
		name string
		in0  []SeqRange
		out  []SeqRange
	}{
		{
			name: "empty",
			in0:  nil,
			out:  nil,
		},
		{
			name: "single",
			in0:  []SeqRange{{Lo: 3, Hi: 5}},
			out:  []SeqRange{{Lo: 3, Hi: 5}},
		},
		{
			name: "no overlap",
			in0:  []SeqRange{{Lo: 1, Hi: 2}, {Lo: 5, Hi: 7}},
			out:  []SeqRange{{Lo: 1, Hi: 2}, {Lo: 5, Hi: 7}},
		},
		{
			name: "adjacent merge",
			in0:  []SeqRange{{Lo: 1, Hi: 3}, {Lo: 4, Hi: 6}},
			out:  []SeqRange{{Lo: 1, Hi: 6}},
		},
		{
			name: "overlapping merge",
			in0:  []SeqRange{{Lo: 1, Hi: 5}, {Lo: 3, Hi: 8}},
			out:  []SeqRange{{Lo: 1, Hi: 8}},
		},
		{
			name: "unsorted input",
			in0:  []SeqRange{{Lo: 5, Hi: 7}, {Lo: 1, Hi: 2}},
			out:  []SeqRange{{Lo: 1, Hi: 2}, {Lo: 5, Hi: 7}},
		},
		{
			name: "three ranges merge to one",
			in0:  []SeqRange{{Lo: 1, Hi: 3}, {Lo: 7, Hi: 7}, {Lo: 4, Hi: 6}},
			out:  []SeqRange{{Lo: 1, Hi: 7}},
		},
		{
			name: "contained range",
			in0:  []SeqRange{{Lo: 1, Hi: 10}, {Lo: 3, Hi: 5}},
			out:  []SeqRange{{Lo: 1, Hi: 10}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeRanges(tc.in0)
			if len(got) != len(tc.out) {
				t.Fatalf("mergeRanges(%v) = %v, want %v", tc.in0, got, tc.out)
			}
			for i := range got {
				if got[i] != tc.out[i] {
					t.Errorf("mergeRanges(%v)[%d] = %v, want %v", tc.in0, i, got[i], tc.out[i])
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./dotcontext/ -run TestMergeRanges -v`
Expected: FAIL — `mergeRanges` undefined.

**Step 3: Write minimal implementation**

In `dotcontext/context.go`, add import `"sort"` and append:

```go
// mergeRanges sorts ranges by Lo and merges overlapping or adjacent ranges.
func mergeRanges(ranges []SeqRange) []SeqRange {
	if len(ranges) <= 1 {
		return ranges
	}
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Lo < ranges[j].Lo
	})
	q := ranges[:1]
	for _, r := range ranges[1:] {
		last := &q[len(q)-1]
		if r.Lo <= last.Hi+1 {
			if r.Hi > last.Hi {
				last.Hi = r.Hi
			}
		} else {
			q = append(q, r)
		}
	}
	return q
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./dotcontext/ -run TestMergeRanges -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dotcontext/context.go dotcontext/context_test.go
git commit -m "dotcontext: add mergeRanges helper"
```

---

### Task 3: Missing() — VV-only cases (no outliers)

The simplest Missing() scenarios: both contexts have only version vectors, no outliers.

**Files:**
- Modify: `dotcontext/context.go` (append Missing method)
- Test: `dotcontext/context_test.go`

**Step 1: Write the failing tests**

In `dotcontext/context_test.go`, add a helper and tests:

```go
func missingEqual(got, want map[ReplicaID][]SeqRange) bool {
	if len(got) != len(want) {
		return false
	}
	for id, wantRanges := range want {
		gotRanges, ok := got[id]
		if !ok {
			return false
		}
		if len(gotRanges) != len(wantRanges) {
			return false
		}
		for i := range gotRanges {
			if gotRanges[i] != wantRanges[i] {
				return false
			}
		}
	}
	return true
}

func TestCausalContextMissingBothEmpty(t *testing.T) {
	local := New()
	remote := New()
	got := local.Missing(remote)
	if len(got) != 0 {
		t.Errorf("Missing between two empty contexts = %v, want empty", got)
	}
}

func TestCausalContextMissingLocalEmpty(t *testing.T) {
	local := New()
	remote := New()
	remote.Next("a")
	remote.Next("a")
	remote.Next("b")

	got := local.Missing(remote)
	want := map[ReplicaID][]SeqRange{
		"a": {{Lo: 1, Hi: 2}},
		"b": {{Lo: 1, Hi: 1}},
	}
	if !missingEqual(got, want) {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}

func TestCausalContextMissingAlreadySynced(t *testing.T) {
	local := New()
	local.Next("a")
	local.Next("a")

	remote := local.Clone()

	got := local.Missing(remote)
	if len(got) != 0 {
		t.Errorf("Missing between synced contexts = %v, want empty", got)
	}
}

func TestCausalContextMissingPartiallyBehind(t *testing.T) {
	local := New()
	local.Next("a") // a:1
	local.Next("a") // a:2
	local.Next("b") // b:1

	remote := New()
	remote.Next("a") // a:1
	remote.Next("a") // a:2
	remote.Next("a") // a:3
	remote.Next("a") // a:4
	remote.Next("b") // b:1

	got := local.Missing(remote)
	want := map[ReplicaID][]SeqRange{
		"a": {{Lo: 3, Hi: 4}},
	}
	if !missingEqual(got, want) {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./dotcontext/ -run TestCausalContextMissing -v`
Expected: FAIL — `Missing` undefined.

**Step 3: Write minimal implementation (VV comparison only)**

In `dotcontext/context.go`, append:

```go
// Missing returns the dots that remote has observed but p has not,
// grouped by replica and compressed into sorted, non-overlapping,
// non-adjacent SeqRange slices.
func (p *CausalContext) Missing(remote *CausalContext) map[ReplicaID][]SeqRange {
	q := make(map[ReplicaID][]SeqRange)

	// Phase 1: version vector comparison.
	for id, remoteSeq := range remote.vv {
		localSeq := p.vv[id]
		if remoteSeq <= localSeq {
			continue
		}
		q[id] = []SeqRange{{Lo: localSeq + 1, Hi: remoteSeq}}
	}

	// Clean up empty entries.
	for id, ranges := range q {
		if len(ranges) == 0 {
			delete(q, id)
		}
	}
	if len(q) == 0 {
		return nil
	}
	return q
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./dotcontext/ -run TestCausalContextMissing -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dotcontext/context.go dotcontext/context_test.go
git commit -m "dotcontext: add Missing() with VV comparison"
```

---

### Task 4: Missing() — hole-punching (local outliers subtract from VV range)

When local has outlier dots that fall within the VV-derived missing range, those dots are already observed and must be subtracted.

**Files:**
- Modify: `dotcontext/context.go` (extend Missing)
- Test: `dotcontext/context_test.go`

**Step 1: Write the failing test**

```go
func TestCausalContextMissingHolePunch(t *testing.T) {
	// local: VV{a:3}, outliers{a:5}
	// remote: VV{a:7}
	// Naive missing: {4,7}. But local has a:5, so: {4,4}, {6,7}.
	local := New()
	local.Add(Dot{ID: "a", Seq: 1})
	local.Add(Dot{ID: "a", Seq: 2})
	local.Add(Dot{ID: "a", Seq: 3})
	local.Add(Dot{ID: "a", Seq: 5}) // outlier — gap at 4

	remote := New()
	for i := 0; i < 7; i++ {
		remote.Next("a")
	}

	got := local.Missing(remote)
	want := map[ReplicaID][]SeqRange{
		"a": {{Lo: 4, Hi: 4}, {Lo: 6, Hi: 7}},
	}
	if !missingEqual(got, want) {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}

func TestCausalContextMissingHolePunchMultiple(t *testing.T) {
	// local: VV{a:2}, outliers{a:4, a:6}
	// remote: VV{a:8}
	// Naive: {3,8}. After punch: {3,3}, {5,5}, {7,8}.
	local := New()
	local.Add(Dot{ID: "a", Seq: 1})
	local.Add(Dot{ID: "a", Seq: 2})
	local.Add(Dot{ID: "a", Seq: 4}) // outlier
	local.Add(Dot{ID: "a", Seq: 6}) // outlier

	remote := New()
	for i := 0; i < 8; i++ {
		remote.Next("a")
	}

	got := local.Missing(remote)
	want := map[ReplicaID][]SeqRange{
		"a": {{Lo: 3, Hi: 3}, {Lo: 5, Hi: 5}, {Lo: 7, Hi: 8}},
	}
	if !missingEqual(got, want) {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./dotcontext/ -run TestCausalContextMissingHolePunch -v`
Expected: FAIL — returns `{4,7}` instead of `{4,4}, {6,7}`.

**Step 3: Extend Missing() with hole-punching**

Replace the phase 1 block in `Missing()`:

```go
	// Phase 1: version vector comparison with hole-punching.
	for id, remoteSeq := range remote.vv {
		localSeq := p.vv[id]
		if remoteSeq <= localSeq {
			continue
		}
		lo := localSeq + 1

		// Collect local outliers for this replica within [lo, remoteSeq].
		var holes []uint64
		for d := range p.outliers {
			if d.ID == id && d.Seq >= lo && d.Seq <= remoteSeq {
				holes = append(holes, d.Seq)
			}
		}

		if len(holes) == 0 {
			q[id] = append(q[id], SeqRange{Lo: lo, Hi: remoteSeq})
			continue
		}

		sort.Slice(holes, func(i, j int) bool { return holes[i] < holes[j] })
		cursor := lo
		for _, h := range holes {
			if h > cursor {
				q[id] = append(q[id], SeqRange{Lo: cursor, Hi: h - 1})
			}
			cursor = h + 1
		}
		if cursor <= remoteSeq {
			q[id] = append(q[id], SeqRange{Lo: cursor, Hi: remoteSeq})
		}
	}
```

Add `"sort"` to imports if not already present.

**Step 4: Run tests to verify they pass**

Run: `go test ./dotcontext/ -run TestCausalContextMissing -v`
Expected: ALL PASS (including prior VV-only tests).

**Step 5: Commit**

```bash
git add dotcontext/context.go dotcontext/context_test.go
git commit -m "dotcontext: Missing() hole-punching for local outliers"
```

---

### Task 5: Missing() — remote outliers

Dots in `remote.outliers` that local hasn't observed.

**Files:**
- Modify: `dotcontext/context.go` (extend Missing)
- Test: `dotcontext/context_test.go`

**Step 1: Write the failing test**

```go
func TestCausalContextMissingRemoteOutliers(t *testing.T) {
	// local: VV{a:3}
	// remote: VV{a:3}, outliers{a:5, a:7}
	// VV comparison: nothing (equal). Remote outliers: {5,5}, {7,7}.
	local := New()
	for i := 0; i < 3; i++ {
		local.Next("a")
	}

	remote := local.Clone()
	remote.Add(Dot{ID: "a", Seq: 5}) // outlier
	remote.Add(Dot{ID: "a", Seq: 7}) // outlier

	got := local.Missing(remote)
	want := map[ReplicaID][]SeqRange{
		"a": {{Lo: 5, Hi: 5}, {Lo: 7, Hi: 7}},
	}
	if !missingEqual(got, want) {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}

func TestCausalContextMissingRemoteOutlierAlreadyObserved(t *testing.T) {
	// local: VV{a:5}
	// remote: VV{a:3}, outliers{a:4}
	// VV comparison: nothing (local ahead). Remote outlier a:4: local Has it (4 <= 5). Skip.
	local := New()
	for i := 0; i < 5; i++ {
		local.Next("a")
	}

	remote := New()
	for i := 0; i < 3; i++ {
		remote.Next("a")
	}
	remote.Add(Dot{ID: "a", Seq: 4}) // outlier, but local has it via VV

	got := local.Missing(remote)
	if len(got) != 0 {
		t.Errorf("Missing() = %v, want empty", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./dotcontext/ -run TestCausalContextMissingRemoteOutlier -v`
Expected: FAIL — remote outlier phase not yet implemented.

**Step 3: Add phase 2 to Missing()**

After the phase 1 loop, before the cleanup block:

```go
	// Phase 2: remote outliers not observed locally.
	for d := range remote.outliers {
		if !p.Has(d) {
			q[d.ID] = append(q[d.ID], SeqRange{Lo: d.Seq, Hi: d.Seq})
		}
	}

	// Phase 3: merge ranges per replica (outlier singletons may be adjacent to VV ranges).
	for id, ranges := range q {
		q[id] = mergeRanges(ranges)
	}
```

Remove the old cleanup block and replace with:

```go
	if len(q) == 0 {
		return nil
	}
	return q
```

**Step 4: Run ALL Missing tests**

Run: `go test ./dotcontext/ -run TestCausalContextMissing -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add dotcontext/context.go dotcontext/context_test.go
git commit -m "dotcontext: Missing() remote outliers and range merging"
```

---

### Task 6: Missing() — mixed scenario and symmetry test

Validate a realistic scenario with both VV gaps and outliers on both sides.

**Files:**
- Test: `dotcontext/context_test.go`

**Step 1: Write the test**

```go
func TestCausalContextMissingMixed(t *testing.T) {
	// local:  VV{a:3, b:2}, outliers{a:6}
	// remote: VV{a:5, b:4, c:1}, outliers{a:8}
	//
	// Replica a: VV range {4,5}, hole-punch a:6 → stays {4,5} (6 > 5, outside range).
	//            Remote outlier a:8: !local.Has(a:8) → {8,8}.
	//            Result: {4,5}, {8,8}.
	// Replica b: VV range {3,4}. No local outliers for b.
	//            Result: {3,4}.
	// Replica c: VV range {1,1}. New replica.
	//            Result: {1,1}.
	local := New()
	local.Add(Dot{ID: "a", Seq: 1})
	local.Add(Dot{ID: "a", Seq: 2})
	local.Add(Dot{ID: "a", Seq: 3})
	local.Add(Dot{ID: "a", Seq: 6}) // outlier (gap at 4,5)
	local.Add(Dot{ID: "b", Seq: 1})
	local.Add(Dot{ID: "b", Seq: 2})

	remote := New()
	for i := 0; i < 5; i++ {
		remote.Next("a")
	}
	for i := 0; i < 4; i++ {
		remote.Next("b")
	}
	remote.Next("c")
	remote.Add(Dot{ID: "a", Seq: 8}) // outlier

	got := local.Missing(remote)
	want := map[ReplicaID][]SeqRange{
		"a": {{Lo: 4, Hi: 5}, {Lo: 8, Hi: 8}},
		"b": {{Lo: 3, Hi: 4}},
		"c": {{Lo: 1, Hi: 1}},
	}
	if !missingEqual(got, want) {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}
```

**Step 2: Run test**

Run: `go test ./dotcontext/ -run TestCausalContextMissingMixed -v`
Expected: PASS (no new code needed — validates existing implementation).

**Step 3: Commit**

```bash
git add dotcontext/context_test.go
git commit -m "dotcontext: add mixed-scenario Missing() test"
```

---

### Task 7: Full suite validation

Run the full test suite, lint, and verify nothing broke.

**Step 1: Run full validation**

```bash
cd /Users/aalpar/projects/crdt-projects/crdt && make lint && make && make test
```

Expected: All 108+ tests pass, no lint errors.

**Step 2: Run race detector**

```bash
make test-race
```

Expected: PASS with no data races.
