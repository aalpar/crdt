package mvregister

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBasicOps(t *testing.T) {
	c := qt.New(t)

	c.Run("NewEmpty", func(c *qt.C) {
		r := New[string]("a")
		c.Assert(r.Values(), qt.HasLen, 0)
	})

	c.Run("WriteAndValues", func(c *qt.C) {
		r := New[string]("a")
		r.Write("hello")

		vals := r.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "hello")
	})

	c.Run("OverwriteReplaces", func(c *qt.C) {
		r := New[string]("a")
		r.Write("first")
		r.Write("second")

		vals := r.Values()
		c.Assert(vals, qt.HasLen, 1)
		c.Assert(vals[0], qt.Equals, "second")
	})

	c.Run("OverwriteCleansStore", func(c *qt.C) {
		r := New[string]("a")
		r.Write("first")
		r.Write("second")
		r.Write("third")

		// Store should have exactly one dot, not three.
		c.Assert(r.state.Store.Len(), qt.Equals, 1)
	})
}
