package replication

import (
	"io"

	"github.com/aalpar/crdt/dotcontext"
)

// WriteDeltaBatch computes what remoteCC is missing from localCC,
// fetches matching deltas from store, and encodes them to w.
//
// Returns the number of deltas written. If remote is fully caught up,
// writes a zero count and returns 0.
func WriteDeltaBatch[T any](
	localCC *dotcontext.CausalContext,
	remoteCC *dotcontext.CausalContext,
	store *dotcontext.DeltaStore[T],
	deltaCodec dotcontext.Codec[T],
	w io.Writer,
) (int, error) {
	missing := remoteCC.Missing(localCC)
	deltas := store.Fetch(missing)
	codec := dotcontext.DeltaBatchCodec[T]{DeltaCodec: deltaCodec}
	if err := codec.Encode(w, deltas); err != nil {
		return 0, err
	}
	return len(deltas), nil
}

// ReadDeltaBatch decodes deltas from r and calls apply for each one.
// The apply callback is where the caller joins deltas into local state.
//
// Returns the number of deltas read.
func ReadDeltaBatch[T any](
	deltaCodec dotcontext.Codec[T],
	r io.Reader,
	apply func(dotcontext.Dot, T),
) (int, error) {
	codec := dotcontext.DeltaBatchCodec[T]{DeltaCodec: deltaCodec}
	deltas, err := codec.Decode(r)
	if err != nil {
		return 0, err
	}
	for d, delta := range deltas {
		apply(d, delta)
	}
	return len(deltas), nil
}
