package dotcontext

import (
	"bytes"
	"io"
	"testing"
)

// FuzzCodecRoundTripDotSet builds a Causal[*DotSet] from fuzz bytes,
// encodes it, decodes it, and verifies equality.
func FuzzCodecRoundTripDotSet(f *testing.F) {
	f.Add([]byte{0, 0, 3})
	f.Add([]byte{0, 0, 3, 1, 0, 3, 2, 0, 2})
	f.Add([]byte{})

	codec := CausalCodec[*DotSet]{StoreCodec: DotSetCodec{}}

	f.Fuzz(func(t *testing.T, data []byte) {
		original := parseCausalDotSet(data)

		var buf bytes.Buffer
		if err := codec.Encode(&buf, original); err != nil {
			t.Fatalf("encode: %v", err)
		}

		decoded, err := codec.Decode(&buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}

		if !equalCausalDotSet(original, decoded) {
			t.Fatal("round-trip mismatch")
		}
	})
}

// FuzzCodecRoundTripDotFun builds a Causal[*DotFun[maxVal]] from fuzz
// bytes, encodes it, decodes it, and verifies equality.
func FuzzCodecRoundTripDotFun(f *testing.F) {
	f.Add([]byte{0, 0, 3, 50})
	f.Add([]byte{0, 0, 3, 100, 1, 0, 3, 200})
	f.Add([]byte{})

	valCodec := Int64Codec{} // maxVal.n is int64
	codec := CausalCodec[*DotFun[maxVal]]{
		StoreCodec: DotFunCodec[maxVal]{ValueCodec: maxValCodec{inner: valCodec}},
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		original := parseCausalDotFun(data)

		var buf bytes.Buffer
		if err := codec.Encode(&buf, original); err != nil {
			t.Fatalf("encode: %v", err)
		}

		decoded, err := codec.Decode(&buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}

		if !equalCausalDotFun(original, decoded) {
			t.Fatal("round-trip mismatch")
		}
	})
}

// FuzzCodecRoundTripDotMap builds a Causal[*DotMap[string, *DotSet]]
// from fuzz bytes, encodes it, decodes it, and verifies equality.
func FuzzCodecRoundTripDotMap(f *testing.F) {
	f.Add([]byte{0, 0, 0, 3})
	f.Add([]byte{0, 0, 0, 3, 1, 1, 0, 3})
	f.Add([]byte{})

	codec := CausalCodec[*DotMap[string, *DotSet]]{
		StoreCodec: DotMapCodec[string, *DotSet]{
			KeyCodec:   StringCodec{},
			ValueCodec: DotSetCodec{},
		},
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		original := parseCausalDotMap(data)

		var buf bytes.Buffer
		if err := codec.Encode(&buf, original); err != nil {
			t.Fatalf("encode: %v", err)
		}

		decoded, err := codec.Decode(&buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}

		if !equalCausalDotMap(original, decoded) {
			t.Fatal("round-trip mismatch")
		}
	})
}

// maxValCodec encodes maxVal as its int64 field.
type maxValCodec struct {
	inner Int64Codec
}

func (c maxValCodec) Encode(w io.Writer, v maxVal) error {
	return c.inner.Encode(w, v.n)
}

func (c maxValCodec) Decode(r io.Reader) (maxVal, error) {
	n, err := c.inner.Decode(r)
	return maxVal{n: n}, err
}
