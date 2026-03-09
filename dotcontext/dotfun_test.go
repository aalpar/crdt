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

func TestDotFunOperations(t *testing.T) {
	c := qt.New(t)

	c.Run("Basic", func(c *qt.C) {
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
	})

	c.Run("SetOverwrite", func(c *qt.C) {
		f := NewDotFun[maxInt]()
		d := Dot{ID: "a", Seq: 1}
		f.Set(d, 10)
		f.Set(d, 42) // overwrite
		v, ok := f.Get(d)
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, maxInt(42))
		c.Assert(f.Len(), qt.Equals, 1)
	})

	c.Run("RemoveAbsent", func(c *qt.C) {
		f := NewDotFun[maxInt]()
		f.Remove(Dot{ID: "x", Seq: 99}) // should not panic
		c.Assert(f.Len(), qt.Equals, 0)
	})
}

func TestDotFunRange(t *testing.T) {
	c := qt.New(t)

	c.Run("FullIteration", func(c *qt.C) {
		f := NewDotFun[maxInt]()
		f.Set(Dot{ID: "a", Seq: 1}, 10)
		f.Set(Dot{ID: "b", Seq: 2}, 20)

		var total maxInt
		f.Range(func(_ Dot, v maxInt) bool {
			total += v
			return true
		})
		c.Assert(total, qt.Equals, maxInt(30))
	})

	c.Run("EarlyStop", func(c *qt.C) {
		f := NewDotFun[maxInt]()
		f.Set(Dot{ID: "a", Seq: 1}, 10)
		f.Set(Dot{ID: "b", Seq: 2}, 20)
		f.Set(Dot{ID: "c", Seq: 3}, 30)

		count := 0
		f.Range(func(_ Dot, _ maxInt) bool {
			count++
			return false
		})
		c.Assert(count, qt.Equals, 1)
	})

	c.Run("Empty", func(c *qt.C) {
		f := NewDotFun[maxInt]()
		count := 0
		f.Range(func(_ Dot, _ maxInt) bool {
			count++
			return true
		})
		c.Assert(count, qt.Equals, 0)
	})
}

func TestDotFunDots(t *testing.T) {
	c := qt.New(t)

	c.Run("Count", func(c *qt.C) {
		f := NewDotFun[maxInt]()
		f.Set(Dot{ID: "a", Seq: 1}, 10)
		f.Set(Dot{ID: "b", Seq: 2}, 20)

		c.Assert(f.Dots().Len(), qt.Equals, 2)
	})

	c.Run("ReturnsClone", func(c *qt.C) {
		f := NewDotFun[maxInt]()
		f.Set(Dot{ID: "a", Seq: 1}, 10)

		dots := f.Dots()
		dots.Add(Dot{ID: "b", Seq: 2})

		// Original should be unaffected.
		c.Assert(f.Len(), qt.Equals, 1)
		_, ok := f.Get(Dot{ID: "b", Seq: 2})
		c.Assert(ok, qt.IsFalse)
	})
}

func TestDotFunClone(t *testing.T) {
	c := qt.New(t)
	f := NewDotFun[maxInt]()
	d := Dot{ID: "a", Seq: 1}
	f.Set(d, 42)

	cl := f.Clone()
	cl.Set(Dot{ID: "b", Seq: 2}, 99)

	c.Assert(f.Len(), qt.Equals, 1)
	_, ok := f.Get(Dot{ID: "b", Seq: 2})
	c.Assert(ok, qt.IsFalse)
}
