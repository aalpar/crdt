package rwset

import (
	"slices"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		s := New[string]("a")
		c.Assert(s.Len(), qt.Equals, 0)
		c.Assert(s.Has("x"), qt.IsFalse)
		c.Assert(s.Elements(), qt.HasLen, 0)
	})

	c.Run("AddHas", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Add("y")

		c.Assert(s.Has("x"), qt.IsTrue)
		c.Assert(s.Has("y"), qt.IsTrue)
		c.Assert(s.Has("z"), qt.IsFalse)
		c.Assert(s.Len(), qt.Equals, 2)
	})

	c.Run("AddRemoveHas", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Add("y")
		s.Remove("x")

		c.Assert(s.Has("x"), qt.IsFalse)
		c.Assert(s.Has("y"), qt.IsTrue)
		c.Assert(s.Len(), qt.Equals, 1)
	})

	c.Run("ReAdd", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		s.Remove("x")
		c.Assert(s.Has("x"), qt.IsFalse)

		s.Add("x")
		c.Assert(s.Has("x"), qt.IsTrue)
	})

	c.Run("RemoveNonExistent", func(c *qt.C) {
		s := New[string]("a")
		s.Remove("ghost") // should not panic, no dot generated
		c.Assert(s.Len(), qt.Equals, 0)
	})

	c.Run("Elements", func(c *qt.C) {
		s := New[int]("a")
		s.Add(3)
		s.Add(1)
		s.Add(2)

		elems := s.Elements()
		slices.Sort(elems)
		c.Assert(elems, qt.DeepEquals, []int{1, 2, 3})
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Add", func(c *qt.C) {
		s := New[string]("a")
		delta := s.Add("x")

		c.Assert(delta.Has("x"), qt.IsTrue)
		c.Assert(delta.Len(), qt.Equals, 1)
	})

	c.Run("Remove", func(c *qt.C) {
		s := New[string]("a")
		s.Add("x")
		delta := s.Remove("x")

		// Remove delta has the element with Active=false — not "present".
		c.Assert(delta.Has("x"), qt.IsFalse)
	})
}

func TestRemoveWins(t *testing.T) {
	c := qt.New(t)

	c.Run("ConcurrentAddRemove", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		// Both replicas see "x".
		addDelta := a.Add("x")
		b.Merge(addDelta)

		// Concurrent: a re-adds "x", b removes "x".
		reAddDelta := a.Add("x")
		removeDelta := b.Remove("x")

		a.Merge(removeDelta)
		b.Merge(reAddDelta)

		// Remove wins: the tombstone dot from b survives alongside a's add dot.
		// Has requires ALL dots to be Active, so the tombstone makes it absent.
		c.Assert(a.Has("x"), qt.IsFalse, qt.Commentf("replica a: remove should win"))
		c.Assert(b.Has("x"), qt.IsFalse, qt.Commentf("replica b: remove should win"))
	})

	c.Run("ConcurrentAdds", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Add("x")
		db := b.Add("x")

		a.Merge(db)
		b.Merge(da)

		// Both added concurrently — both dots are Active, so element is present.
		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(b.Has("x"), qt.IsTrue)
	})

	c.Run("ConcurrentRemoves", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		// Both see "x".
		addDelta := a.Add("x")
		b.Merge(addDelta)

		// Both remove concurrently.
		rmA := a.Remove("x")
		rmB := b.Remove("x")

		a.Merge(rmB)
		b.Merge(rmA)

		c.Assert(a.Has("x"), qt.IsFalse)
		c.Assert(b.Has("x"), qt.IsFalse)
	})
}

func TestMergeProperties(t *testing.T) {
	c := qt.New(t)

	c.Run("Idempotent", func(c *qt.C) {
		a := New[string]("a")
		a.Add("x")
		a.Add("y")

		snapshot := New[string]("a")
		snapshot.Merge(a)

		a.Merge(snapshot)
		a.Merge(snapshot)

		c.Assert(a.Len(), qt.Equals, 2)
		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(a.Has("y"), qt.IsTrue)
	})

	c.Run("Commutative", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		a.Add("x")
		b.Add("y")

		ab := New[string]("ab")
		ab.Merge(a)
		ab.Merge(b)

		ba := New[string]("ba")
		ba.Merge(b)
		ba.Merge(a)

		abElems := ab.Elements()
		baElems := ba.Elements()
		slices.Sort(abElems)
		slices.Sort(baElems)

		c.Assert(abElems, qt.DeepEquals, baElems)
	})

	c.Run("Associative", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		a.Add("x")
		a.Add("y")
		b.Add("y")
		b.Add("z")
		x.Add("x")
		x.Remove("x")
		x.Add("w")

		// (a join b) join c
		ab := New[string]("ab")
		ab.Merge(a)
		ab.Merge(b)
		abc := New[string]("abc")
		abc.Merge(ab)
		abc.Merge(x)

		// a join (b join c)
		bc := New[string]("bc")
		bc.Merge(b)
		bc.Merge(x)
		abc2 := New[string]("abc2")
		abc2.Merge(a)
		abc2.Merge(bc)

		abcElems := abc.Elements()
		abc2Elems := abc2.Elements()
		slices.Sort(abcElems)
		slices.Sort(abc2Elems)

		c.Assert(abcElems, qt.DeepEquals, abc2Elems)
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("IncrementalEqualsFullMerge", func(c *qt.C) {
		a := New[string]("a")
		d1 := a.Add("x")
		d2 := a.Add("y")

		bIncremental := New[string]("b")
		bIncremental.Merge(d1)
		bIncremental.Merge(d2)

		bFull := New[string]("b")
		bFull.Merge(a)

		incElems := bIncremental.Elements()
		fullElems := bFull.Elements()
		slices.Sort(incElems)
		slices.Sort(fullElems)

		c.Assert(incElems, qt.DeepEquals, fullElems)
	})

	c.Run("DeltaDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		d1 := a.Add("x")
		d2 := a.Add("y")

		// Merge two deltas together, then apply combined delta.
		d1.Merge(d2)

		b := New[string]("b")
		b.Merge(d1)

		c.Assert(b.Has("x"), qt.IsTrue)
		c.Assert(b.Has("y"), qt.IsTrue)
		c.Assert(b.Len(), qt.Equals, 2)
	})
}

func TestMergeWithEmpty(t *testing.T) {
	c := qt.New(t)

	c.Run("IntoEmpty", func(c *qt.C) {
		a := New[string]("a")
		a.Add("x")
		a.Add("y")

		b := New[string]("b")
		b.Merge(a)
		c.Assert(b.Len(), qt.Equals, 2)
		c.Assert(b.Has("x"), qt.IsTrue)
		c.Assert(b.Has("y"), qt.IsTrue)
	})

	c.Run("EmptyIntoPopulated", func(c *qt.C) {
		a := New[string]("a")
		a.Add("x")
		a.Add("y")

		empty := New[string]("b")
		a.Merge(empty)
		c.Assert(a.Len(), qt.Equals, 2)
		c.Assert(a.Has("x"), qt.IsTrue)
		c.Assert(a.Has("y"), qt.IsTrue)
	})
}

func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New[string]("a")
		a.Add("x")
		a.Add("y")

		// Serialize and reconstruct.
		state := a.State()
		b := FromCausal[string](state)

		c.Assert(b.Has("x"), qt.IsTrue)
		c.Assert(b.Has("y"), qt.IsTrue)
		c.Assert(b.Len(), qt.Equals, 2)
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		delta := a.Add("x")

		// Reconstruct the delta via FromCausal and merge it.
		reconstructed := FromCausal[string](delta.State())

		b := New[string]("b")
		b.Merge(reconstructed)

		c.Assert(b.Has("x"), qt.IsTrue)
		c.Assert(b.Len(), qt.Equals, 1)
	})
}

func TestConvergence(t *testing.T) {
	c := qt.New(t)

	c.Run("ThreeReplica", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		da := a.Add("x")
		db := b.Add("y")
		dx := x.Add("z")

		a.Merge(db)
		a.Merge(dx)
		b.Merge(da)
		b.Merge(dx)
		x.Merge(da)
		x.Merge(db)

		want := []string{"x", "y", "z"}
		for _, r := range []*RWSet[string]{a, b, x} {
			elems := r.Elements()
			slices.Sort(elems)
			c.Assert(elems, qt.DeepEquals, want)
		}
	})

	c.Run("FiveReplica", func(c *qt.C) {
		ids := []dotcontext.ReplicaID{"a", "b", "c", "d", "e"}
		replicas := make([]*RWSet[string], len(ids))
		for i, id := range ids {
			replicas[i] = New[string](id)
		}

		// Each replica performs different operations.
		deltas := make([]*RWSet[string], len(ids))
		deltas[0] = replicas[0].Add("x")   // a adds "x"
		deltas[1] = replicas[1].Add("y")   // b adds "y"
		deltas[2] = replicas[2].Add("z")   // c adds "z"
		replicas[3].Add("w")               // d adds "w" (no delta saved)
		deltas[3] = replicas[3].Remove("w") // d removes "w" immediately
		deltas[4] = replicas[4].Add("x")   // e also adds "x" concurrently

		// Full mesh: every replica merges every other replica's delta.
		for i := range replicas {
			for j := range replicas {
				if i != j && deltas[j] != nil {
					replicas[i].Merge(deltas[j])
				}
			}
		}

		// All should converge: {"x", "y", "z"}.
		// "w" was added then removed by d — no concurrent add, so it stays removed.
		// "x" was added by both a and e — concurrent adds both survive, all Active.
		want := []string{"x", "y", "z"}
		for i, r := range replicas {
			elems := r.Elements()
			slices.Sort(elems)
			c.Assert(elems, qt.DeepEquals, want, qt.Commentf("replica %s", ids[i]))
		}
	})
}
