package replication

import (
	"bytes"
	"testing"

	"github.com/aalpar/crdt/dotcontext"
)

// int64Codec wraps dotcontext.Int64Codec to satisfy Codec[int64].
type int64Codec = dotcontext.Int64Codec

func TestWriteReadDeltaBatchRoundTrip(t *testing.T) {
	// Setup: two peers, local has deltas that remote is missing.
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
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("wrote %d deltas, want 2", n)
	}

	// Read
	received := make(map[dotcontext.Dot]int64)
	nRead, err := ReadDeltaBatch(codec, &buf, func(d dotcontext.Dot, v int64) {
		received[d] = v
	})
	if err != nil {
		t.Fatal(err)
	}
	if nRead != 2 {
		t.Errorf("read %d deltas, want 2", nRead)
	}
	if received[dotcontext.Dot{ID: "a", Seq: 2}] != 20 {
		t.Errorf("delta (a,2): got %d, want 20", received[dotcontext.Dot{ID: "a", Seq: 2}])
	}
	if received[dotcontext.Dot{ID: "b", Seq: 1}] != 30 {
		t.Errorf("delta (b,1): got %d, want 30", received[dotcontext.Dot{ID: "b", Seq: 1}])
	}
}

func TestWriteDeltaBatchFullySynced(t *testing.T) {
	cc := dotcontext.New()
	cc.Add(dotcontext.Dot{ID: "a", Seq: 1})

	store := dotcontext.NewDeltaStore[int64]()
	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, 10)

	// Remote has everything local has — nothing to send.
	var buf bytes.Buffer
	n, err := WriteDeltaBatch(cc, cc, store, int64Codec{}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("wrote %d deltas, want 0", n)
	}

	// Read side should get zero deltas.
	nRead, err := ReadDeltaBatch(int64Codec{}, &buf, func(d dotcontext.Dot, v int64) {
		t.Error("unexpected delta")
	})
	if err != nil {
		t.Fatal(err)
	}
	if nRead != 0 {
		t.Errorf("read %d deltas, want 0", nRead)
	}
}

func TestWriteDeltaBatchMissingNotInStore(t *testing.T) {
	// Remote is missing (a,2) but store doesn't have it (already GC'd).
	localCC := dotcontext.New()
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 1})
	localCC.Add(dotcontext.Dot{ID: "a", Seq: 2})

	remoteCC := dotcontext.New()
	remoteCC.Add(dotcontext.Dot{ID: "a", Seq: 1})

	store := dotcontext.NewDeltaStore[int64]()
	// Only (a,1) in store — (a,2) was GC'd.
	store.Add(dotcontext.Dot{ID: "a", Seq: 1}, 10)

	var buf bytes.Buffer
	n, err := WriteDeltaBatch(localCC, remoteCC, store, int64Codec{}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	// Store doesn't have (a,2), so nothing ships.
	if n != 0 {
		t.Errorf("wrote %d deltas, want 0", n)
	}
}
