package rga

import (
	"bytes"
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

func TestConcurrentInserts(t *testing.T) {
	c := qt.New(t)

	c.Run("ConcurrentAtHead", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.InsertAfter(dotcontext.Dot{}, "from-a")
		db := b.InsertAfter(dotcontext.Dot{}, "from-b")

		a.Merge(db)
		b.Merge(da)

		ae := a.Elements()
		be := b.Elements()
		c.Assert(len(ae), qt.Equals, 2)
		c.Assert(len(be), qt.Equals, 2)
		// Both converge to the same order.
		c.Assert(ae[0].Value, qt.Equals, be[0].Value)
		c.Assert(ae[1].Value, qt.Equals, be[1].Value)
	})

	c.Run("ConcurrentAfterSameElement", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		// Both see the root element.
		da0 := a.InsertAfter(dotcontext.Dot{}, "root")
		b.Merge(da0)

		rootDot := a.Elements()[0].ID

		// Concurrent inserts after root.
		da := a.InsertAfter(rootDot, "from-a")
		db := b.InsertAfter(rootDot, "from-b")

		a.Merge(db)
		b.Merge(da)

		ae := a.Elements()
		be := b.Elements()
		c.Assert(len(ae), qt.Equals, 3)
		// Converge.
		for i := range ae {
			c.Assert(ae[i].Value, qt.Equals, be[i].Value)
		}
		// Root is first.
		c.Assert(ae[0].Value, qt.Equals, "root")
	})

	c.Run("InsertAfterDeletedNode", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		// a inserts "x", replicates to b.
		da0 := a.InsertAfter(dotcontext.Dot{}, "x")
		b.Merge(da0)

		xDot := a.Elements()[0].ID

		// a deletes "x", b inserts "y" after "x" — concurrently.
		ddel := a.Delete(xDot)
		dins := b.InsertAfter(xDot, "y")

		a.Merge(dins)
		b.Merge(ddel)

		// Both converge: "x" deleted, "y" survives.
		ae := a.Elements()
		be := b.Elements()
		c.Assert(len(ae), qt.Equals, 1)
		c.Assert(ae[0].Value, qt.Equals, "y")
		c.Assert(len(be), qt.Equals, 1)
		c.Assert(be[0].Value, qt.Equals, "y")
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("IncrementalDeltas", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		d1 := a.InsertAfter(dotcontext.Dot{}, "x")
		b.Merge(d1)

		xDot := a.Elements()[0].ID
		d2 := a.InsertAfter(xDot, "y")
		b.Merge(d2)

		ae := a.Elements()
		be := b.Elements()
		c.Assert(len(ae), qt.Equals, len(be))
		for i := range ae {
			c.Assert(ae[i].Value, qt.Equals, be[i].Value)
		}
	})

	c.Run("DeleteDeltaPropagation", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		d1 := a.InsertAfter(dotcontext.Dot{}, "x")
		b.Merge(d1)

		xDot := a.Elements()[0].ID
		d2 := a.Delete(xDot)
		b.Merge(d2)

		c.Assert(a.Len(), qt.Equals, 0)
		c.Assert(b.Len(), qt.Equals, 0)
	})
}

func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New[string]("a")
		a.InsertAfter(dotcontext.Dot{}, "x")
		xDot := a.Elements()[0].ID
		a.InsertAfter(xDot, "y")

		state := a.State()
		b := FromCausal[string](state)

		be := b.Elements()
		c.Assert(be, qt.HasLen, 2)
		c.Assert(be[0].Value, qt.Equals, "x")
		c.Assert(be[1].Value, qt.Equals, "y")
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		delta := a.InsertAfter(dotcontext.Dot{}, "x")

		reconstructed := FromCausal[string](delta.State())
		b := New[string]("b")
		b.Merge(reconstructed)

		be := b.Elements()
		c.Assert(be, qt.HasLen, 1)
		c.Assert(be[0].Value, qt.Equals, "x")
	})
}

func TestCodecRoundTrip(t *testing.T) {
	c := qt.New(t)

	codec := dotcontext.CausalCodec[*dotcontext.DotFun[Node[string]]]{
		StoreCodec: dotcontext.DotFunCodec[Node[string]]{
			ValueCodec: NodeCodec[string]{ValueCodec: dotcontext.StringCodec{}},
		},
	}

	c.Run("EmptyRGA", func(c *qt.C) {
		r := New[string]("a")
		var buf bytes.Buffer
		c.Assert(codec.Encode(&buf, r.State()), qt.IsNil)

		decoded, err := codec.Decode(&buf)
		c.Assert(err, qt.IsNil)

		got := FromCausal[string](decoded)
		c.Assert(got.Len(), qt.Equals, 0)
	})

	c.Run("WithElements", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAfter(dotcontext.Dot{}, "hello")
		hDot := r.Elements()[0].ID
		r.InsertAfter(hDot, "world")

		var buf bytes.Buffer
		c.Assert(codec.Encode(&buf, r.State()), qt.IsNil)

		decoded, err := codec.Decode(&buf)
		c.Assert(err, qt.IsNil)

		got := FromCausal[string](decoded)
		elems := got.Elements()
		c.Assert(elems, qt.HasLen, 2)
		c.Assert(elems[0].Value, qt.Equals, "hello")
		c.Assert(elems[1].Value, qt.Equals, "world")
	})

	c.Run("WithTombstones", func(c *qt.C) {
		r := New[string]("a")
		r.InsertAfter(dotcontext.Dot{}, "alive")
		aDot := r.Elements()[0].ID
		r.InsertAfter(aDot, "dead")
		dDot := r.Elements()[1].ID
		r.InsertAfter(dDot, "also-alive")
		r.Delete(dDot)

		var buf bytes.Buffer
		c.Assert(codec.Encode(&buf, r.State()), qt.IsNil)

		decoded, err := codec.Decode(&buf)
		c.Assert(err, qt.IsNil)

		got := FromCausal[string](decoded)
		elems := got.Elements()
		c.Assert(elems, qt.HasLen, 2)
		c.Assert(elems[0].Value, qt.Equals, "alive")
		c.Assert(elems[1].Value, qt.Equals, "also-alive")
	})

	c.Run("DeltaRoundTrip", func(c *qt.C) {
		a := New[string]("a")
		delta := a.InsertAfter(dotcontext.Dot{}, "x")

		var buf bytes.Buffer
		c.Assert(codec.Encode(&buf, delta.State()), qt.IsNil)

		decoded, err := codec.Decode(&buf)
		c.Assert(err, qt.IsNil)

		b := New[string]("b")
		b.Merge(FromCausal[string](decoded))

		elems := b.Elements()
		c.Assert(elems, qt.HasLen, 1)
		c.Assert(elems[0].Value, qt.Equals, "x")
	})
}
