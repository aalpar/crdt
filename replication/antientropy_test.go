package replication

import (
	"bytes"
	"errors"
	"io"
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

// --- End-to-end with real CRDT deltas ---

// causalDotSetCodec encodes Causal[*DotSet] for use as delta values.
type causalDotSetCodec struct {
	inner dotcontext.CausalCodec[*dotcontext.DotSet]
}

func newCausalDotSetCodec() causalDotSetCodec {
	return causalDotSetCodec{
		inner: dotcontext.CausalCodec[*dotcontext.DotSet]{
			StoreCodec: dotcontext.DotSetCodec{},
		},
	}
}

func (c causalDotSetCodec) Encode(w io.Writer, v dotcontext.Causal[*dotcontext.DotSet]) error {
	return c.inner.Encode(w, v)
}

func (c causalDotSetCodec) Decode(r io.Reader) (dotcontext.Causal[*dotcontext.DotSet], error) {
	return c.inner.Decode(r)
}

func TestWriteReadDeltaBatchCausalDeltas(t *testing.T) {
	c := qt.New(t)
	codec := newCausalDotSetCodec()

	// Simulate an AWSet-like replica: two adds produce two Causal[*DotSet] deltas.
	localCC := dotcontext.New()
	d1 := localCC.Next("a") // a:1
	d2 := localCC.Next("a") // a:2

	store := dotcontext.NewDeltaStore[dotcontext.Causal[*dotcontext.DotSet]]()

	// Delta for d1: store={d1}, context={d1}.
	ds1 := dotcontext.NewDotSet()
	ds1.Add(d1)
	ctx1 := dotcontext.New()
	ctx1.Add(d1)
	store.Add(d1, dotcontext.Causal[*dotcontext.DotSet]{Store: ds1, Context: ctx1})

	// Delta for d2: store={d2}, context={d2}.
	ds2 := dotcontext.NewDotSet()
	ds2.Add(d2)
	ctx2 := dotcontext.New()
	ctx2.Add(d2)
	store.Add(d2, dotcontext.Causal[*dotcontext.DotSet]{Store: ds2, Context: ctx2})

	// Remote has seen d1 but not d2.
	remoteCC := dotcontext.New()
	remoteCC.Add(d1)

	// Write missing deltas.
	var buf bytes.Buffer
	n, err := WriteDeltaBatch(localCC, remoteCC, store, codec, &buf)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)

	// Read and merge into a fresh DotSet state.
	result := dotcontext.Causal[*dotcontext.DotSet]{
		Store:   dotcontext.NewDotSet(),
		Context: dotcontext.New(),
	}
	// Pre-populate with what remote already has.
	result.Store.Add(d1)
	result.Context.Add(d1)

	nRead, err := ReadDeltaBatch(codec, &buf, func(d dotcontext.Dot, delta dotcontext.Causal[*dotcontext.DotSet]) {
		result = dotcontext.JoinDotSet(result, delta)
	})
	c.Assert(err, qt.IsNil)
	c.Assert(nRead, qt.Equals, 1)

	// Result should have both dots.
	c.Assert(result.Store.Has(d1), qt.IsTrue)
	c.Assert(result.Store.Has(d2), qt.IsTrue)
	c.Assert(result.Context.Has(d1), qt.IsTrue)
	c.Assert(result.Context.Has(d2), qt.IsTrue)
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

// --- Error propagation ---

var errBrokenWriter = errors.New("broken writer")

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errBrokenWriter }

// TestWriteDeltaBatchWriterError exercises the error path at
// antientropy.go:24-25 where codec.Encode fails.
func TestWriteDeltaBatchWriterError(t *testing.T) {
	c := qt.New(t)

	localCC := dotcontext.New()
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 1})

	remoteCC := dotcontext.New()
	// Remote is missing (a,1).

	store := dotcontext.NewDeltaStore[int64]()
	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, 10)

	_, err := WriteDeltaBatch(localCC, remoteCC, store, int64Codec{}, brokenWriter{})
	c.Assert(err, qt.IsNotNil)
}

// TestReadDeltaBatchDecodeError exercises the error path at
// antientropy.go:40-42 where codec.Decode fails on truncated input.
func TestReadDeltaBatchDecodeError(t *testing.T) {
	c := qt.New(t)

	// Encode a valid batch, then truncate by one byte.
	localCC := dotcontext.New()
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 1})

	remoteCC := dotcontext.New()

	store := dotcontext.NewDeltaStore[int64]()
	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, 10)

	var buf bytes.Buffer
	n, err := WriteDeltaBatch(localCC, remoteCC, store, int64Codec{}, &buf)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)

	data := buf.Bytes()
	truncated := bytes.NewReader(data[:len(data)-1])

	_, err = ReadDeltaBatch(int64Codec{}, truncated, func(d dotcontext.Dot, v int64) {
		c.Fatal("unexpected apply call on truncated input")
	})
	c.Assert(err, qt.IsNotNil)
}
