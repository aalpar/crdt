package dotcontext

import "testing"

func TestDotString(t *testing.T) {
	d := Dot{ID: "a", Seq: 3}
	if got := d.String(); got != "a:3" {
		t.Errorf("Dot.String() = %q, want %q", got, "a:3")
	}
}

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
