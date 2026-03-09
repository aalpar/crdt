package lwwregister

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		r := New[string]("a")
		_, _, ok := r.Value()
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("SetAndValue", func(c *qt.C) {
		r := New[string]("a")
		r.Set("hello", 1)

		v, ts, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "hello")
		c.Assert(ts, qt.Equals, int64(1))
	})

	c.Run("Overwrite", func(c *qt.C) {
		r := New[string]("a")
		r.Set("first", 1)
		r.Set("second", 2)

		v, ts, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "second")
		c.Assert(ts, qt.Equals, int64(2))
	})

	c.Run("IntegerRegister", func(c *qt.C) {
		r := New[int]("a")
		r.Set(42, 1)
		r.Set(99, 2)

		v, _, ok := r.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, 99)
	})

	c.Run("OverwriteCleansStore", func(c *qt.C) {
		r := New[string]("a")
		r.Set("first", 1)
		r.Set("second", 2)
		r.Set("third", 3)

		// Store should have exactly one dot, not three.
		count := 0
		r.state.Store.Range(func(_ dotcontext.Dot, _ Timestamped[string]) bool {
			count++
			return true
		})
		c.Assert(count, qt.Equals, 1)
	})
}

func TestDeltaReturn(t *testing.T) {
	c := qt.New(t)

	c.Run("Set", func(c *qt.C) {
		r := New[string]("a")
		delta := r.Set("x", 10)

		v, ts, ok := delta.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "x")
		c.Assert(ts, qt.Equals, int64(10))
	})
}

func TestConflictResolution(t *testing.T) {
	c := qt.New(t)

	c.Run("HigherTimestampWins", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Set("from-a", 10)
		db := b.Set("from-b", 20)

		a.Merge(db)
		b.Merge(da)

		for _, r := range []*LWWRegister[string]{a, b} {
			v, ts, ok := r.Value()
			c.Assert(ok, qt.IsTrue)
			c.Assert(v, qt.Equals, "from-b")
			c.Assert(ts, qt.Equals, int64(20))
		}
	})

	c.Run("SameTimestampTiebreak", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Set("from-a", 10)
		db := b.Set("from-b", 10)

		a.Merge(db)
		b.Merge(da)

		for _, r := range []*LWWRegister[string]{a, b} {
			v, _, ok := r.Value()
			c.Assert(ok, qt.IsTrue)
			c.Assert(v, qt.Equals, "from-b")
		}
	})

	c.Run("TiebreakHigherReplicaIDWins", func(c *qt.C) {
		// "z" > "a" lexicographically — "z" should win the tiebreak.
		a := New[string]("a")
		z := New[string]("z")

		da := a.Set("from-a", 10)
		dz := z.Set("from-z", 10)

		a.Merge(dz)
		z.Merge(da)

		for _, r := range []*LWWRegister[string]{a, z} {
			v, _, ok := r.Value()
			c.Assert(ok, qt.IsTrue)
			c.Assert(v, qt.Equals, "from-z")
		}
	})

	c.Run("ThreeWaySameTimestampTiebreak", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		da := a.Set("from-a", 5)
		db := b.Set("from-b", 5)
		dx := x.Set("from-c", 5)

		a.Merge(db)
		a.Merge(dx)
		b.Merge(da)
		b.Merge(dx)
		x.Merge(da)
		x.Merge(db)

		// All converge; "c" > "b" > "a" so "from-c" wins.
		for _, r := range []*LWWRegister[string]{a, b, x} {
			v, _, ok := r.Value()
			c.Assert(ok, qt.IsTrue)
			c.Assert(v, qt.Equals, "from-c")
		}
	})

	c.Run("OverwriteAfterConcurrentMerge", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Set("from-a", 1)
		db := b.Set("from-b", 2)

		// a sees both values (concurrent).
		a.Merge(db)

		// a overwrites — delta context should include both old dots.
		d3 := a.Set("resolved", 3)

		// Fresh replica merges only the overwrite delta.
		x := New[string]("x")
		x.Merge(da)
		x.Merge(db)
		x.Merge(d3)

		v, ts, ok := x.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "resolved")
		c.Assert(ts, qt.Equals, int64(3))
	})
}

func TestTimestampedJoin(t *testing.T) {
	c := qt.New(t)

	c.Run("HigherWins", func(c *qt.C) {
		a := Timestamped[string]{Value: "old", Ts: 10}
		b := Timestamped[string]{Value: "new", Ts: 20}

		c.Assert(a.Join(b), qt.Equals, b)
		c.Assert(b.Join(a), qt.Equals, b) // commutative
	})

	c.Run("SameTimestamp", func(c *qt.C) {
		a := Timestamped[string]{Value: "a-val", Ts: 10}
		b := Timestamped[string]{Value: "b-val", Ts: 10}

		// Equal timestamps: receiver wins (first argument).
		c.Assert(a.Join(b), qt.Equals, a)
		c.Assert(b.Join(a), qt.Equals, b)
	})
}

func TestMergeProperties(t *testing.T) {
	c := qt.New(t)

	c.Run("Idempotent", func(c *qt.C) {
		a := New[string]("a")
		a.Set("x", 5)

		snapshot := New[string]("a")
		snapshot.Merge(a)

		a.Merge(snapshot)
		a.Merge(snapshot)

		v, ts, ok := a.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "x")
		c.Assert(ts, qt.Equals, int64(5))
	})

	c.Run("Commutative", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		a.Set("va", 1)
		b.Set("vb", 2)

		ab := New[string]("x")
		ab.Merge(a)
		ab.Merge(b)

		ba := New[string]("x")
		ba.Merge(b)
		ba.Merge(a)

		vAB, tsAB, _ := ab.Value()
		vBA, tsBA, _ := ba.Value()
		c.Assert(vAB, qt.Equals, vBA)
		c.Assert(tsAB, qt.Equals, tsBA)
	})

	c.Run("Associative", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		a.Set("va", 5)
		b.Set("vb", 10)
		x.Set("vc", 3)

		// (a ⊔ b) ⊔ c
		ab := New[string]("ab")
		ab.Merge(a)
		ab.Merge(b)
		abc := New[string]("abc")
		abc.Merge(ab)
		abc.Merge(x)

		// a ⊔ (b ⊔ c)
		bc := New[string]("bc")
		bc.Merge(b)
		bc.Merge(x)
		abc2 := New[string]("abc2")
		abc2.Merge(a)
		abc2.Merge(bc)

		v1, ts1, _ := abc.Value()
		v2, ts2, _ := abc2.Value()
		c.Assert(v1, qt.Equals, v2)
		c.Assert(ts1, qt.Equals, ts2)
	})
}

func TestDeltaPropagation(t *testing.T) {
	c := qt.New(t)

	c.Run("IncrementalEqualsFullMerge", func(c *qt.C) {
		a := New[string]("a")
		d1 := a.Set("first", 1)
		d2 := a.Set("second", 2)

		inc := New[string]("b")
		inc.Merge(d1)
		inc.Merge(d2)

		full := New[string]("b")
		full.Merge(a)

		vInc, tsInc, _ := inc.Value()
		vFull, tsFull, _ := full.Value()
		c.Assert(vInc, qt.Equals, vFull)
		c.Assert(tsInc, qt.Equals, tsFull)
	})

	c.Run("OverwriteDeltaSupersedes", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		d1 := a.Set("first", 1)
		b.Merge(d1)

		d2 := a.Set("second", 2)
		b.Merge(d2)

		v, ts, ok := b.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "second")
		c.Assert(ts, qt.Equals, int64(2))
	})

	c.Run("ConcurrentWriteThenOverwrite", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")

		da := a.Set("from-a", 10)
		db := b.Set("from-b", 20)

		a.Merge(db)
		b.Merge(da)

		d3 := a.Set("from-a-again", 30)
		b.Merge(d3)

		v, ts, ok := b.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "from-a-again")
		c.Assert(ts, qt.Equals, int64(30))
	})

	c.Run("DeltaDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		d1 := a.Set("first", 1)
		d2 := a.Set("second", 2)

		// Combine deltas, then apply.
		d1.Merge(d2)

		b := New[string]("b")
		b.Merge(d1)

		v, ts, ok := b.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "second")
		c.Assert(ts, qt.Equals, int64(2))
	})
}

func TestMergeWithEmpty(t *testing.T) {
	c := qt.New(t)

	c.Run("IntoEmpty", func(c *qt.C) {
		a := New[string]("a")
		a.Set("hello", 5)

		b := New[string]("b")
		b.Merge(a)

		v, ts, ok := b.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "hello")
		c.Assert(ts, qt.Equals, int64(5))
	})

	c.Run("EmptyIntoSet", func(c *qt.C) {
		a := New[string]("a")
		a.Set("hello", 5)

		empty := New[string]("b")
		a.Merge(empty)

		v, ts, ok := a.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "hello")
		c.Assert(ts, qt.Equals, int64(5))
	})
}

func TestStateRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("StateFromCausal", func(c *qt.C) {
		a := New[string]("a")
		a.Set("hello", 5)

		state := a.State()
		b := FromCausal[string](state)

		v, ts, ok := b.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "hello")
		c.Assert(ts, qt.Equals, int64(5))
	})

	c.Run("FromCausalDeltaMerge", func(c *qt.C) {
		a := New[string]("a")
		delta := a.Set("x", 10)

		reconstructed := FromCausal[string](delta.State())

		b := New[string]("b")
		b.Merge(reconstructed)

		v, ts, ok := b.Value()
		c.Assert(ok, qt.IsTrue)
		c.Assert(v, qt.Equals, "x")
		c.Assert(ts, qt.Equals, int64(10))
	})
}

func TestConvergence(t *testing.T) {
	c := qt.New(t)

	c.Run("ThreeReplica", func(c *qt.C) {
		a := New[string]("a")
		b := New[string]("b")
		x := New[string]("c")

		da := a.Set("va", 1)
		db := b.Set("vb", 2)
		dx := x.Set("vc", 3)

		a.Merge(db)
		a.Merge(dx)
		b.Merge(da)
		b.Merge(dx)
		x.Merge(da)
		x.Merge(db)

		for _, r := range []*LWWRegister[string]{a, b, x} {
			v, ts, ok := r.Value()
			c.Assert(ok, qt.IsTrue)
			c.Assert(v, qt.Equals, "vc")
			c.Assert(ts, qt.Equals, int64(3))
		}
	})

	c.Run("FiveReplica", func(c *qt.C) {
		ids := []dotcontext.ReplicaID{"a", "b", "c", "d", "e"}
		replicas := make([]*LWWRegister[string], len(ids))
		for i, id := range ids {
			replicas[i] = New[string](id)
		}

		// Each replica writes at a different timestamp.
		deltas := make([]*LWWRegister[string], len(ids))
		deltas[0] = replicas[0].Set("from-a", 10)
		deltas[1] = replicas[1].Set("from-b", 50) // highest ts — winner
		deltas[2] = replicas[2].Set("from-c", 30)
		deltas[3] = replicas[3].Set("from-d", 20)
		deltas[4] = replicas[4].Set("from-e", 40)

		// Full mesh merge.
		for i := range replicas {
			for j := range replicas {
				if i != j {
					replicas[i].Merge(deltas[j])
				}
			}
		}

		// All converge to "from-b" (highest timestamp).
		for i, r := range replicas {
			v, ts, ok := r.Value()
			c.Assert(ok, qt.IsTrue)
			c.Assert(v, qt.Equals, "from-b", qt.Commentf("replica %s", ids[i]))
			c.Assert(ts, qt.Equals, int64(50))
		}
	})
}
