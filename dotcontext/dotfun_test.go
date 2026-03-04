package dotcontext

import "testing"

// maxInt is a simple lattice for testing: join = max.
type maxInt int

func (a maxInt) Join(b maxInt) maxInt {
	if b > a {
		return b
	}
	return a
}

func TestDotFunBasic(t *testing.T) {
	f := NewDotFun[maxInt]()
	d := Dot{ID: "a", Seq: 1}

	f.Set(d, 42)
	v, ok := f.Get(d)
	if !ok || v != 42 {
		t.Errorf("Get = (%v, %v), want (42, true)", v, ok)
	}

	if f.Len() != 1 {
		t.Errorf("len = %d, want 1", f.Len())
	}

	f.Remove(d)
	if _, ok := f.Get(d); ok {
		t.Error("should not have dot after remove")
	}
}

func TestDotFunDots(t *testing.T) {
	f := NewDotFun[maxInt]()
	f.Set(Dot{ID: "a", Seq: 1}, 10)
	f.Set(Dot{ID: "b", Seq: 2}, 20)

	ds := f.Dots()
	if ds.Len() != 2 {
		t.Errorf("Dots().Len() = %d, want 2", ds.Len())
	}
}
