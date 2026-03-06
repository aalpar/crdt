package dotcontext

import "testing"

func TestCausalContextNew(t *testing.T) {
	c := New()
	if c.Has(Dot{ID: "a", Seq: 1}) {
		t.Error("new context should not contain any dots")
	}
	if c.Max("a") != 0 {
		t.Error("max of unseen replica should be 0")
	}
}

func TestCausalContextNext(t *testing.T) {
	c := New()
	d1 := c.Next("a")
	d2 := c.Next("a")
	d3 := c.Next("b")

	if d1 != (Dot{ID: "a", Seq: 1}) {
		t.Errorf("first dot = %v, want a:1", d1)
	}
	if d2 != (Dot{ID: "a", Seq: 2}) {
		t.Errorf("second dot = %v, want a:2", d2)
	}
	if d3 != (Dot{ID: "b", Seq: 1}) {
		t.Errorf("third dot = %v, want b:1", d3)
	}
	if !c.Has(d1) || !c.Has(d2) || !c.Has(d3) {
		t.Error("context should contain all generated dots")
	}
}

func TestCausalContextAdd(t *testing.T) {
	c := New()

	// Contiguous: goes directly into version vector.
	c.Add(Dot{ID: "a", Seq: 1})
	if !c.Has(Dot{ID: "a", Seq: 1}) {
		t.Error("should have a:1")
	}

	// Gap: becomes an outlier.
	c.Add(Dot{ID: "a", Seq: 3})
	if !c.Has(Dot{ID: "a", Seq: 3}) {
		t.Error("should have a:3 as outlier")
	}
	if c.Has(Dot{ID: "a", Seq: 2}) {
		t.Error("should not have a:2 yet")
	}
}

func TestCausalContextMax(t *testing.T) {
	c := New()
	c.Add(Dot{ID: "a", Seq: 1})
	c.Add(Dot{ID: "a", Seq: 5}) // outlier

	if got := c.Max("a"); got != 5 {
		t.Errorf("Max(a) = %d, want 5", got)
	}
	if got := c.Max("b"); got != 0 {
		t.Errorf("Max(b) = %d, want 0", got)
	}
}

func TestCausalContextMerge(t *testing.T) {
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
		if !a.Has(d) {
			t.Errorf("after merge, should have %v", d)
		}
	}
}

func TestCausalContextCompact(t *testing.T) {
	c := New()
	c.Add(Dot{ID: "a", Seq: 1})
	c.Add(Dot{ID: "a", Seq: 3}) // outlier
	c.Add(Dot{ID: "a", Seq: 5}) // outlier
	c.Add(Dot{ID: "a", Seq: 2}) // fills gap → 3 should compact

	c.Compact()

	// After compacting: vv["a"] should be 3 (1,2,3 contiguous).
	// 5 remains an outlier (4 is missing).
	if !c.Has(Dot{ID: "a", Seq: 3}) {
		t.Error("should have a:3 after compact")
	}
	if c.Has(Dot{ID: "a", Seq: 4}) {
		t.Error("should not have a:4")
	}
	if !c.Has(Dot{ID: "a", Seq: 5}) {
		t.Error("should still have a:5 as outlier")
	}
}

func TestCausalContextCompactFull(t *testing.T) {
	c := New()
	// Add 1, 3, 5, then fill gaps 2, 4.
	c.Add(Dot{ID: "a", Seq: 1})
	c.Add(Dot{ID: "a", Seq: 3})
	c.Add(Dot{ID: "a", Seq: 5})
	c.Add(Dot{ID: "a", Seq: 2})
	c.Add(Dot{ID: "a", Seq: 4})

	c.Compact()

	// All should be in version vector now, no outliers.
	for i := uint64(1); i <= 5; i++ {
		if !c.Has(Dot{ID: "a", Seq: i}) {
			t.Errorf("should have a:%d after full compact", i)
		}
	}
}

func TestCausalContextCompactThenNext(t *testing.T) {
	// Without Compact, Next("a") would return a:3 — a collision
	// with the existing outlier. Compact must promote outliers so
	// Next skips past them.
	c := New()
	c.Add(Dot{ID: "a", Seq: 1})
	c.Add(Dot{ID: "a", Seq: 3}) // outlier
	c.Add(Dot{ID: "a", Seq: 2}) // fills gap

	c.Compact()

	d := c.Next("a")
	if d.Seq != 4 {
		t.Errorf("Next after compact = a:%d, want a:4", d.Seq)
	}
}

func TestCausalContextClone(t *testing.T) {
	c := New()
	c.Add(Dot{ID: "a", Seq: 1})
	c.Add(Dot{ID: "a", Seq: 3})

	cc := c.Clone()
	cc.Add(Dot{ID: "b", Seq: 1})

	if c.Has(Dot{ID: "b", Seq: 1}) {
		t.Error("clone mutation should not affect original")
	}
}

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
