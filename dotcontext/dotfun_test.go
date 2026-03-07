package dotcontext

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// maxInt is a simple lattice for testing: join = max.
type maxInt int

func (a maxInt) Join(b maxInt) maxInt {
	if b > a {
		return b
	}
	return a
}

func TestDotFunBasic(t *testing.T) {
	c := qt.New(t)
	f := NewDotFun[maxInt]()
	d := Dot{ID: "a", Seq: 1}

	f.Set(d, 42)
	v, ok := f.Get(d)
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, maxInt(42))
	c.Assert(f.Len(), qt.Equals, 1)

	f.Remove(d)
	_, ok = f.Get(d)
	c.Assert(ok, qt.IsFalse)
}

func TestDotFunDots(t *testing.T) {
	c := qt.New(t)
	f := NewDotFun[maxInt]()
	f.Set(Dot{ID: "a", Seq: 1}, 10)
	f.Set(Dot{ID: "b", Seq: 2}, 20)

	c.Assert(f.Dots().Len(), qt.Equals, 2)
}
