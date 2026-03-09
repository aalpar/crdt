package dotcontext

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestCausalContextNew(t *testing.T) {
	c := qt.New(t)
	cc := New()
	c.Assert(cc.Has(Dot{ID: "a", Seq: 1}), qt.IsFalse)
	c.Assert(cc.Max("a"), qt.Equals, uint64(0))
}

func TestCausalContextNext(t *testing.T) {
	c := qt.New(t)
	cc := New()
	d1 := cc.Next("a")
	d2 := cc.Next("a")
	d3 := cc.Next("b")

	c.Assert(d1, qt.Equals, Dot{ID: "a", Seq: 1})
	c.Assert(d2, qt.Equals, Dot{ID: "a", Seq: 2})
	c.Assert(d3, qt.Equals, Dot{ID: "b", Seq: 1})
	c.Assert(cc.Has(d1), qt.IsTrue)
	c.Assert(cc.Has(d2), qt.IsTrue)
	c.Assert(cc.Has(d3), qt.IsTrue)
}

func TestCausalContextAdd(t *testing.T) {
	c := qt.New(t)
	cc := New()

	// Contiguous: goes directly into version vector.
	cc.Add(Dot{ID: "a", Seq: 1})
	c.Assert(cc.Has(Dot{ID: "a", Seq: 1}), qt.IsTrue)

	// Gap: becomes an outlier.
	cc.Add(Dot{ID: "a", Seq: 3})
	c.Assert(cc.Has(Dot{ID: "a", Seq: 3}), qt.IsTrue)
	c.Assert(cc.Has(Dot{ID: "a", Seq: 2}), qt.IsFalse)
}

func TestCausalContextMax(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 5}) // outlier

	c.Assert(cc.Max("a"), qt.Equals, uint64(5))
	c.Assert(cc.Max("b"), qt.Equals, uint64(0))
}

func TestCausalContextMerge(t *testing.T) {
	c := qt.New(t)
	a := New()
	a.Add(Dot{ID: "x", Seq: 1})
	a.Add(Dot{ID: "x", Seq: 2})

	b := New()
	b.Add(Dot{ID: "x", Seq: 1})
	b.Add(Dot{ID: "x", Seq: 2})
	b.Add(Dot{ID: "x", Seq: 3})
	b.Add(Dot{ID: "y", Seq: 1})

	a.Merge(b)

	for _, d := range []Dot{
		{ID: "x", Seq: 1},
		{ID: "x", Seq: 2},
		{ID: "x", Seq: 3},
		{ID: "y", Seq: 1},
	} {
		c.Assert(a.Has(d), qt.IsTrue, qt.Commentf("dot %v", d))
	}
}

func TestCausalContextCompact(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 3}) // outlier
	cc.Add(Dot{ID: "a", Seq: 5}) // outlier
	cc.Add(Dot{ID: "a", Seq: 2}) // fills gap → 3 should compact

	cc.Compact()

	c.Assert(cc.Has(Dot{ID: "a", Seq: 3}), qt.IsTrue)
	c.Assert(cc.Has(Dot{ID: "a", Seq: 4}), qt.IsFalse)
	c.Assert(cc.Has(Dot{ID: "a", Seq: 5}), qt.IsTrue)
}

func TestCausalContextCompactFull(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 3})
	cc.Add(Dot{ID: "a", Seq: 5})
	cc.Add(Dot{ID: "a", Seq: 2})
	cc.Add(Dot{ID: "a", Seq: 4})

	cc.Compact()

	for i := uint64(1); i <= 5; i++ {
		c.Assert(cc.Has(Dot{ID: "a", Seq: i}), qt.IsTrue, qt.Commentf("a:%d", i))
	}
}

func TestCausalContextCompactThenNext(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 3}) // outlier
	cc.Add(Dot{ID: "a", Seq: 2}) // fills gap

	cc.Compact()

	d := cc.Next("a")
	c.Assert(d.Seq, qt.Equals, uint64(4))
}

func TestCausalContextClone(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 3})

	cl := cc.Clone()
	cl.Add(Dot{ID: "b", Seq: 1})

	c.Assert(cc.Has(Dot{ID: "b", Seq: 1}), qt.IsFalse)
}

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
	c := qt.New(t)
	local := New()
	remote := New()
	got := local.Missing(remote)
	c.Assert(len(got), qt.Equals, 0)
}

func TestCausalContextMissingLocalEmpty(t *testing.T) {
	c := qt.New(t)
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
	c.Assert(missingEqual(got, want), qt.IsTrue, qt.Commentf("got %v, want %v", got, want))
}

func TestCausalContextMissingAlreadySynced(t *testing.T) {
	c := qt.New(t)
	local := New()
	local.Next("a")
	local.Next("a")

	remote := local.Clone()

	got := local.Missing(remote)
	c.Assert(len(got), qt.Equals, 0)
}

func TestCausalContextMissingPartiallyBehind(t *testing.T) {
	c := qt.New(t)
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
	c.Assert(missingEqual(got, want), qt.IsTrue, qt.Commentf("got %v, want %v", got, want))
}

func TestCausalContextMissingHolePunch(t *testing.T) {
	c := qt.New(t)
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
	c.Assert(missingEqual(got, want), qt.IsTrue, qt.Commentf("got %v, want %v", got, want))
}

func TestCausalContextMissingHolePunchMultiple(t *testing.T) {
	c := qt.New(t)
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
	c.Assert(missingEqual(got, want), qt.IsTrue, qt.Commentf("got %v, want %v", got, want))
}

func TestCausalContextMissingRemoteOutliers(t *testing.T) {
	c := qt.New(t)
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
	c.Assert(missingEqual(got, want), qt.IsTrue, qt.Commentf("got %v, want %v", got, want))
}

func TestCausalContextMissingRemoteOutlierAlreadyObserved(t *testing.T) {
	c := qt.New(t)
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
	c.Assert(len(got), qt.Equals, 0)
}

func TestCausalContextMissingMixed(t *testing.T) {
	c := qt.New(t)
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
	c.Assert(missingEqual(got, want), qt.IsTrue, qt.Commentf("got %v, want %v", got, want))
}

// --- Add edge cases ---

func TestCausalContextAddDuplicate(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 1}) // duplicate — already in VV
	c.Assert(cc.Has(Dot{ID: "a", Seq: 1}), qt.IsTrue)
	c.Assert(cc.Max("a"), qt.Equals, uint64(1))
}

func TestCausalContextAddDuplicateOutlier(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 3}) // outlier
	cc.Add(Dot{ID: "a", Seq: 3}) // duplicate outlier
	c.Assert(cc.Has(Dot{ID: "a", Seq: 3}), qt.IsTrue)
}

// --- Merge properties ---

func TestCausalContextMergeIdempotent(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 2})
	cc.Add(Dot{ID: "b", Seq: 1})

	before := cc.Clone()
	cc.Merge(cc)

	c.Assert(cc.Max("a"), qt.Equals, before.Max("a"))
	c.Assert(cc.Max("b"), qt.Equals, before.Max("b"))
}

func TestCausalContextMergeCommutative(t *testing.T) {
	c := qt.New(t)
	a := New()
	a.Add(Dot{ID: "x", Seq: 1})
	a.Add(Dot{ID: "x", Seq: 2})

	b := New()
	b.Add(Dot{ID: "x", Seq: 3})
	b.Add(Dot{ID: "y", Seq: 1})

	ab := a.Clone()
	ab.Merge(b)

	ba := b.Clone()
	ba.Merge(a)

	for _, d := range []Dot{
		{ID: "x", Seq: 1}, {ID: "x", Seq: 2},
		{ID: "x", Seq: 3}, {ID: "y", Seq: 1},
	} {
		c.Assert(ab.Has(d), qt.Equals, ba.Has(d), qt.Commentf("dot %v", d))
	}
}

// --- Compact edge cases ---

func TestCausalContextCompactEmpty(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Compact() // should not panic
	c.Assert(cc.Max("a"), qt.Equals, uint64(0))
}

func TestCausalContextCompactNoOutliers(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Next("a")
	cc.Next("a")
	cc.Compact() // nothing to promote
	c.Assert(cc.Max("a"), qt.Equals, uint64(2))
}

// --- ReplicaIDs ---

func TestCausalContextReplicaIDs(t *testing.T) {
	c := qt.New(t)
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "b", Seq: 1})
	cc.Add(Dot{ID: "c", Seq: 5}) // outlier — c only via outliers

	ids := cc.ReplicaIDs()
	seen := make(map[ReplicaID]bool)
	for _, id := range ids {
		seen[id] = true
	}
	c.Assert(seen["a"], qt.IsTrue)
	c.Assert(seen["b"], qt.IsTrue)
	c.Assert(seen["c"], qt.IsTrue)
	c.Assert(len(ids), qt.Equals, 3)
}

func TestCausalContextReplicaIDsEmpty(t *testing.T) {
	c := qt.New(t)
	cc := New()
	c.Assert(cc.ReplicaIDs(), qt.HasLen, 0)
}

func TestMergeRanges(t *testing.T) {
	c := qt.New(t)

	tcs := []struct {
		name string
		in0  []SeqRange
		out  []SeqRange
	}{
		{name: "empty", in0: nil, out: nil},
		{name: "single", in0: []SeqRange{{Lo: 3, Hi: 5}}, out: []SeqRange{{Lo: 3, Hi: 5}}},
		{name: "no overlap", in0: []SeqRange{{Lo: 1, Hi: 2}, {Lo: 5, Hi: 7}}, out: []SeqRange{{Lo: 1, Hi: 2}, {Lo: 5, Hi: 7}}},
		{name: "adjacent merge", in0: []SeqRange{{Lo: 1, Hi: 3}, {Lo: 4, Hi: 6}}, out: []SeqRange{{Lo: 1, Hi: 6}}},
		{name: "overlapping merge", in0: []SeqRange{{Lo: 1, Hi: 5}, {Lo: 3, Hi: 8}}, out: []SeqRange{{Lo: 1, Hi: 8}}},
		{name: "unsorted input", in0: []SeqRange{{Lo: 5, Hi: 7}, {Lo: 1, Hi: 2}}, out: []SeqRange{{Lo: 1, Hi: 2}, {Lo: 5, Hi: 7}}},
		{name: "three ranges merge to one", in0: []SeqRange{{Lo: 1, Hi: 3}, {Lo: 7, Hi: 7}, {Lo: 4, Hi: 6}}, out: []SeqRange{{Lo: 1, Hi: 7}}},
		{name: "contained range", in0: []SeqRange{{Lo: 1, Hi: 10}, {Lo: 3, Hi: 5}}, out: []SeqRange{{Lo: 1, Hi: 10}}},
	}
	for _, tc := range tcs {
		c.Run(tc.name, func(c *qt.C) {
			got := mergeRanges(tc.in0)
			c.Assert(len(got), qt.Equals, len(tc.out))
			for i := range got {
				c.Assert(got[i], qt.Equals, tc.out[i])
			}
		})
	}
}
