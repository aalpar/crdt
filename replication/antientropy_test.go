package replication

import (
	"bytes"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/aalpar/crdt/dotcontext"
)

// int64Codec wraps dotcontext.Int64Codec to satisfy Codec[int64].
type int64Codec = dotcontext.Int64Codec

func TestWriteReadDeltaBatchRoundTrip(t *testing.T) {
	c := qt.New(t)

	localCC := dotcontext.New()
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 1})
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 2})
	localCC.Add(dotcontext.Dot{ID: "b", Seq: 1})

	remoteCC := dotcontext.New()
	remoteCC.Add(dotcontext.Dot{ID: "a", Seq: 1})
	// Remote is missing (a,2) and (b,1).

	store := dotcontext.NewDeltaStore[int64]()
	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, 10)
	store.Add(dotcontext.Dot{ID: "a", Seq: 2}, 20)
	store.Add(dotcontext.Dot{ID: "b", Seq: 1}, 30)

	// Write
	var buf bytes.Buffer
	codec := int64Codec{}
	n, err := WriteDeltaBatch(localCC, remoteCC, store, codec, &buf)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 2)

	// Read
	received := make(map[dotcontext.Dot]int64)
	nRead, err := ReadDeltaBatch(codec, &buf, func(d dotcontext.Dot, v int64) {
		received[d] = v
	})
	c.Assert(err, qt.IsNil)
	c.Assert(nRead, qt.Equals, 2)
	c.Assert(received[dotcontext.Dot{ID: "a", Seq: 2}], qt.Equals, int64(20))
	c.Assert(received[dotcontext.Dot{ID: "b", Seq: 1}], qt.Equals, int64(30))
}

func TestWriteDeltaBatchFullySynced(t *testing.T) {
	c := qt.New(t)
	cc := dotcontext.New()
	cc.Add(dotcontext.Dot{ID: "a", Seq: 1})

	store := dotcontext.NewDeltaStore[int64]()
	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, 10)

	var buf bytes.Buffer
	n, err := WriteDeltaBatch(cc, cc, store, int64Codec{}, &buf)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)

	nRead, err := ReadDeltaBatch(int64Codec{}, &buf, func(d dotcontext.Dot, v int64) {
		c.Fatal("unexpected delta")
	})
	c.Assert(err, qt.IsNil)
	c.Assert(nRead, qt.Equals, 0)
}

func TestWriteReadDeltaBatchMultiReplica(t *testing.T) {
	c := qt.New(t)

	localCC := dotcontext.New()
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 1})
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 2})
	localCC.Add(dotcontext.Dot{ID: "b", Seq: 1})
	localCC.Add(dotcontext.Dot{ID: "c", Seq: 1})

	remoteCC := dotcontext.New()
	// Remote is missing everything.

	store := dotcontext.NewDeltaStore[int64]()
	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, 10)
	store.Add(dotcontext.Dot{ID: "a", Seq: 2}, 20)
	store.Add(dotcontext.Dot{ID: "b", Seq: 1}, 30)
	store.Add(dotcontext.Dot{ID: "c", Seq: 1}, 40)

	var buf bytes.Buffer
	n, err := WriteDeltaBatch(localCC, remoteCC, store, int64Codec{}, &buf)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 4)

	received := make(map[dotcontext.Dot]int64)
	nRead, err := ReadDeltaBatch(int64Codec{}, &buf, func(d dotcontext.Dot, v int64) {
		received[d] = v
	})
	c.Assert(err, qt.IsNil)
	c.Assert(nRead, qt.Equals, 4)
}

func TestWriteReadDeltaBatchBothEmpty(t *testing.T) {
	c := qt.New(t)
	localCC := dotcontext.New()
	remoteCC := dotcontext.New()
	store := dotcontext.NewDeltaStore[int64]()

	var buf bytes.Buffer
	n, err := WriteDeltaBatch(localCC, remoteCC, store, int64Codec{}, &buf)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)

	nRead, err := ReadDeltaBatch(int64Codec{}, &buf, func(d dotcontext.Dot, v int64) {
		c.Fatal("unexpected delta")
	})
	c.Assert(err, qt.IsNil)
	c.Assert(nRead, qt.Equals, 0)
}

func TestWriteDeltaBatchEmptyStore(t *testing.T) {
	c := qt.New(t)
	localCC := dotcontext.New()
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 1})

	remoteCC := dotcontext.New()
	store := dotcontext.NewDeltaStore[int64]()

	var buf bytes.Buffer
	n, err := WriteDeltaBatch(localCC, remoteCC, store, int64Codec{}, &buf)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)
}

func TestWriteDeltaBatchMissingNotInStore(t *testing.T) {
	c := qt.New(t)

	localCC := dotcontext.New()
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 1})
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 2})

	remoteCC := dotcontext.New()
	remoteCC.Add(dotcontext.Dot{ID: "a", Seq: 1})

	store := dotcontext.NewDeltaStore[int64]()
	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, 10)

	var buf bytes.Buffer
	n, err := WriteDeltaBatch(localCC, remoteCC, store, int64Codec{}, &buf)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)
}
