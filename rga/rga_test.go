package rga

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		r := New[string]("a")
		c.Assert(r.Len(), qt.Equals, 0)
		c.Assert(r.Elements(), qt.HasLen, 0)
	})

	c.Run("InsertAtHead", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAfter(dotcontext.Dot{}, "hello")

		elems := r.Elements()
		c.Assert(elems, qt.HasLen, 1)
		c.Assert(elems[0].Value, qt.Equals, "hello")
	})

	c.Run("InsertMultiple", func(c *qt.C) {
		r := New[string]("a")
		// Insert "a" at head.
		r.InsertAfter(dotcontext.Dot{}, "a")
		elemA := r.Elements()[0]

		// Insert "b" after "a".
		r.InsertAfter(elemA.ID, "b")

		elems := r.Elements()
		c.Assert(elems, qt.HasLen, 2)
		c.Assert(elems[0].Value, qt.Equals, "a")
		c.Assert(elems[1].Value, qt.Equals, "b")
	})
}

func TestDelete(t *testing.T) {
	c := qt.New(t)

	c.Run("DeleteElement", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAfter(dotcontext.Dot{}, "x")
		elem := r.Elements()[0]

		r.Delete(elem.ID)
		c.Assert(r.Len(), qt.Equals, 0)
	})

	c.Run("DeletePreservesOrdering", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAfter(dotcontext.Dot{}, "a")
		elemA := r.Elements()[0]
		r.InsertAfter(elemA.ID, "b")
		elemB := r.Elements()[1]
		r.InsertAfter(elemB.ID, "c")

		// Delete "b" (middle element).
		r.Delete(elemB.ID)

		elems := r.Elements()
		c.Assert(elems, qt.HasLen, 2)
		c.Assert(elems[0].Value, qt.Equals, "a")
		c.Assert(elems[1].Value, qt.Equals, "c")
	})

	c.Run("DeleteAlreadyDeleted", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAfter(dotcontext.Dot{}, "x")
		elem := r.Elements()[0]

		r.Delete(elem.ID)
		delta := r.Delete(elem.ID)

		// Second delete is a no-op: empty delta.
		c.Assert(delta.Len(), qt.Equals, 0)
	})

	c.Run("DeleteNonexistent", func(c *qt.C) {
		r := New[string]("a")
		delta := r.Delete(dotcontext.Dot{ID: "z", Seq: 99})
		c.Assert(delta.Len(), qt.Equals, 0)
	})
}

func TestAt(t *testing.T) {
	c := qt.New(t)

	c.Run("ValidIndex", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAfter(dotcontext.Dot{}, "first")
		elemFirst := r.Elements()[0]
		r.InsertAfter(elemFirst.ID, "second")

		e0, ok0 := r.At(0)
		c.Assert(ok0, qt.IsTrue)
		c.Assert(e0.Value, qt.Equals, "first")

		e1, ok1 := r.At(1)
		c.Assert(ok1, qt.IsTrue)
		c.Assert(e1.Value, qt.Equals, "second")
	})

	c.Run("OutOfBounds", func(c *qt.C) {
		r := New[string]("a")

		_, ok := r.At(0)
		c.Assert(ok, qt.IsFalse)

		r.InsertAfter(dotcontext.Dot{}, "x")

		_, ok = r.At(r.Len())
		c.Assert(ok, qt.IsFalse)

		_, ok = r.At(-1)
		c.Assert(ok, qt.IsFalse)
	})
}

func TestInsertAt(t *testing.T) {
	c := qt.New(t)

	c.Run("InsertAtZero", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAt(0, "a")
		r.InsertAt(0, "b")

		elems := r.Elements()
		c.Assert(elems, qt.HasLen, 2)
		// "b" has higher seq at head, so it goes first.
		c.Assert(elems[0].Value, qt.Equals, "b")
		c.Assert(elems[1].Value, qt.Equals, "a")
	})

	c.Run("InsertAtEnd", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAt(0, "a")
		r.InsertAt(1, "b")
		r.InsertAt(2, "c")

		elems := r.Elements()
		c.Assert(elems, qt.HasLen, 3)
		c.Assert(elems[0].Value, qt.Equals, "a")
		c.Assert(elems[1].Value, qt.Equals, "b")
		c.Assert(elems[2].Value, qt.Equals, "c")
	})

	c.Run("InsertAtMiddle", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAt(0, "a")
		r.InsertAt(1, "c")
		r.InsertAt(1, "b")

		elems := r.Elements()
		c.Assert(elems, qt.HasLen, 3)
		c.Assert(elems[0].Value, qt.Equals, "a")
		c.Assert(elems[1].Value, qt.Equals, "b")
		c.Assert(elems[2].Value, qt.Equals, "c")
	})
}

func TestDeleteAt(t *testing.T) {
	c := qt.New(t)

	c.Run("DeleteFirst", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAt(0, "a")
		r.InsertAt(1, "b")

		r.DeleteAt(0)

		elems := r.Elements()
		c.Assert(elems, qt.HasLen, 1)
		c.Assert(elems[0].Value, qt.Equals, "b")
	})

	c.Run("DeleteLast", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAt(0, "a")
		r.InsertAt(1, "b")

		r.DeleteAt(1)

		elems := r.Elements()
		c.Assert(elems, qt.HasLen, 1)
		c.Assert(elems[0].Value, qt.Equals, "a")
	})

	c.Run("DeleteOutOfBounds", func(c *qt.C) {
		r := New[string]("a")
		delta := r.DeleteAt(0)
		c.Assert(delta.Len(), qt.Equals, 0)
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("InsertAfterReturnsDelta", func(c *qt.C) {
		r := New[string]("a")
		delta := r.InsertAfter(dotcontext.Dot{}, "hello")

		elems := delta.Elements()
		c.Assert(elems, qt.HasLen, 1)
		c.Assert(elems[0].Value, qt.Equals, "hello")
	})
}
