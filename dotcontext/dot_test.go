package dotcontext

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestDotString(t *testing.T) {
	c := qt.New(t)
	d := Dot{ID: "a", Seq: 3}
	c.Assert(d.String(), qt.Equals, "a:3")
}

func TestSeqRangeString(t *testing.T) {
	c := qt.New(t)

	tcs := []struct {
		in0 SeqRange
		out string
	}{
		{in0: SeqRange{Lo: 1, Hi: 1}, out: "[1,1]"},
		{in0: SeqRange{Lo: 3, Hi: 7}, out: "[3,7]"},
	}
	for _, tc := range tcs {
		c.Check(tc.in0.String(), qt.Equals, tc.out)
	}
}
