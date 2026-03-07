package dotcontext

import (
	"encoding/binary"
	"fmt"
	"io"
)

// maxDecodeLen caps the element count or byte length that any decoder will
// allocate from a length prefix. This prevents OOM from malformed input
// (e.g. a fuzzed uint64 that claims millions of entries).
const maxDecodeLen = 1 << 20 // ~1 million

// Codec encodes/decodes values of type T to/from a binary stream.
type Codec[T any] interface {
	Encode(w io.Writer, v T) error
	Decode(r io.Reader) (T, error)
}

// StringCodec encodes strings as uint64 length prefix + raw bytes.
type StringCodec struct{}

func (StringCodec) Encode(w io.Writer, v string) error {
	if err := binary.Write(w, binary.LittleEndian, uint64(len(v))); err != nil {
		return err
	}
	_, err := io.WriteString(w, v)
	return err
}

func (StringCodec) Decode(r io.Reader) (string, error) {
	var n uint64
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	if n > maxDecodeLen {
		return "", fmt.Errorf("string length %d exceeds max %d", n, maxDecodeLen)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// Uint64Codec encodes uint64 as 8 bytes little-endian.
type Uint64Codec struct{}

func (Uint64Codec) Encode(w io.Writer, v uint64) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func (Uint64Codec) Decode(r io.Reader) (uint64, error) {
	var v uint64
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

// Int64Codec encodes int64 as 8 bytes little-endian.
type Int64Codec struct{}

func (Int64Codec) Encode(w io.Writer, v int64) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func (Int64Codec) Decode(r io.Reader) (int64, error) {
	var v int64
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

// DotCodec encodes a Dot as [string: ID] [uint64: Seq].
type DotCodec struct{}

func (DotCodec) Encode(w io.Writer, d Dot) error {
	if err := (StringCodec{}).Encode(w, string(d.ID)); err != nil {
		return err
	}
	return (Uint64Codec{}).Encode(w, d.Seq)
}

func (DotCodec) Decode(r io.Reader) (Dot, error) {
	id, err := (StringCodec{}).Decode(r)
	if err != nil {
		return Dot{}, err
	}
	seq, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return Dot{}, err
	}
	return Dot{ID: ReplicaID(id), Seq: seq}, nil
}

// CausalContextCodec encodes a CausalContext as:
// [uint64: vv_len] ([string: replicaID] [uint64: seq])* [uint64: outliers_len] (Dot)*
type CausalContextCodec struct{}

func (CausalContextCodec) Encode(w io.Writer, cc *CausalContext) error {
	// Version vector
	if err := (Uint64Codec{}).Encode(w, uint64(len(cc.vv))); err != nil {
		return err
	}
	for id, seq := range cc.vv {
		if err := (StringCodec{}).Encode(w, string(id)); err != nil {
			return err
		}
		if err := (Uint64Codec{}).Encode(w, seq); err != nil {
			return err
		}
	}
	// Outliers
	if err := (Uint64Codec{}).Encode(w, uint64(len(cc.outliers))); err != nil {
		return err
	}
	dc := DotCodec{}
	for d := range cc.outliers {
		if err := dc.Encode(w, d); err != nil {
			return err
		}
	}
	return nil
}

func (CausalContextCodec) Decode(r io.Reader) (*CausalContext, error) {
	cc := New()
	// Version vector
	vvLen, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return nil, err
	}
	if vvLen > maxDecodeLen {
		return nil, fmt.Errorf("version vector length %d exceeds max %d", vvLen, maxDecodeLen)
	}
	for i := uint64(0); i < vvLen; i++ {
		id, err := (StringCodec{}).Decode(r)
		if err != nil {
			return nil, err
		}
		seq, err := (Uint64Codec{}).Decode(r)
		if err != nil {
			return nil, err
		}
		cc.vv[ReplicaID(id)] = seq
	}
	// Outliers
	outLen, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return nil, err
	}
	if outLen > maxDecodeLen {
		return nil, fmt.Errorf("outlier count %d exceeds max %d", outLen, maxDecodeLen)
	}
	dc := DotCodec{}
	for i := uint64(0); i < outLen; i++ {
		d, err := dc.Decode(r)
		if err != nil {
			return nil, err
		}
		cc.outliers[d] = struct{}{}
	}
	return cc, nil
}

// DotSetCodec encodes a DotSet as [uint64: len] (Dot)*
type DotSetCodec struct{}

func (DotSetCodec) Encode(w io.Writer, ds *DotSet) error {
	if err := (Uint64Codec{}).Encode(w, uint64(ds.Len())); err != nil {
		return err
	}
	dc := DotCodec{}
	var encErr error
	ds.Range(func(d Dot) bool {
		if err := dc.Encode(w, d); err != nil {
			encErr = err
			return false
		}
		return true
	})
	return encErr
}

func (DotSetCodec) Decode(r io.Reader) (*DotSet, error) {
	n, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return nil, err
	}
	if n > maxDecodeLen {
		return nil, fmt.Errorf("dot set length %d exceeds max %d", n, maxDecodeLen)
	}
	ds := NewDotSet()
	dc := DotCodec{}
	for i := uint64(0); i < n; i++ {
		d, err := dc.Decode(r)
		if err != nil {
			return nil, err
		}
		ds.Add(d)
	}
	return ds, nil
}

// DotFunCodec encodes a DotFun[V] as [uint64: len] ([Dot] [V])*
type DotFunCodec[V Lattice[V]] struct {
	ValueCodec Codec[V]
}

func (c DotFunCodec[V]) Encode(w io.Writer, df *DotFun[V]) error {
	if err := (Uint64Codec{}).Encode(w, uint64(df.Len())); err != nil {
		return err
	}
	dc := DotCodec{}
	var encErr error
	df.Range(func(d Dot, v V) bool {
		if err := dc.Encode(w, d); err != nil {
			encErr = err
			return false
		}
		if err := c.ValueCodec.Encode(w, v); err != nil {
			encErr = err
			return false
		}
		return true
	})
	return encErr
}

func (c DotFunCodec[V]) Decode(r io.Reader) (*DotFun[V], error) {
	n, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return nil, err
	}
	if n > maxDecodeLen {
		return nil, fmt.Errorf("dot fun length %d exceeds max %d", n, maxDecodeLen)
	}
	df := NewDotFun[V]()
	dc := DotCodec{}
	for i := uint64(0); i < n; i++ {
		d, err := dc.Decode(r)
		if err != nil {
			return nil, err
		}
		v, err := c.ValueCodec.Decode(r)
		if err != nil {
			return nil, err
		}
		df.Set(d, v)
	}
	return df, nil
}

// DotMapCodec encodes a DotMap[K,V] as [uint64: len] ([K] [V])*
type DotMapCodec[K comparable, V DotStore] struct {
	KeyCodec   Codec[K]
	ValueCodec Codec[V]
}

func (c DotMapCodec[K, V]) Encode(w io.Writer, dm *DotMap[K, V]) error {
	if err := (Uint64Codec{}).Encode(w, uint64(dm.Len())); err != nil {
		return err
	}
	var encErr error
	dm.Range(func(k K, v V) bool {
		if err := c.KeyCodec.Encode(w, k); err != nil {
			encErr = err
			return false
		}
		if err := c.ValueCodec.Encode(w, v); err != nil {
			encErr = err
			return false
		}
		return true
	})
	return encErr
}

func (c DotMapCodec[K, V]) Decode(r io.Reader) (*DotMap[K, V], error) {
	n, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return nil, err
	}
	if n > maxDecodeLen {
		return nil, fmt.Errorf("dot map length %d exceeds max %d", n, maxDecodeLen)
	}
	dm := NewDotMap[K, V]()
	for i := uint64(0); i < n; i++ {
		k, err := c.KeyCodec.Decode(r)
		if err != nil {
			return nil, err
		}
		v, err := c.ValueCodec.Decode(r)
		if err != nil {
			return nil, err
		}
		dm.Set(k, v)
	}
	return dm, nil
}

// SeqRangeCodec encodes a SeqRange as [uint64: Lo] [uint64: Hi].
type SeqRangeCodec struct{}

func (SeqRangeCodec) Encode(w io.Writer, r SeqRange) error {
	if err := (Uint64Codec{}).Encode(w, r.Lo); err != nil {
		return err
	}
	return (Uint64Codec{}).Encode(w, r.Hi)
}

func (SeqRangeCodec) Decode(r io.Reader) (SeqRange, error) {
	lo, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return SeqRange{}, err
	}
	hi, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return SeqRange{}, err
	}
	return SeqRange{Lo: lo, Hi: hi}, nil
}

// MissingCodec encodes a Missing result (map[ReplicaID][]SeqRange) as:
// [uint64: num_replicas] ([string: replicaID] [uint64: num_ranges] (SeqRange)*)*
type MissingCodec struct{}

func (MissingCodec) Encode(w io.Writer, m map[ReplicaID][]SeqRange) error {
	if err := (Uint64Codec{}).Encode(w, uint64(len(m))); err != nil {
		return err
	}
	sc := StringCodec{}
	rc := SeqRangeCodec{}
	for id, ranges := range m {
		if err := sc.Encode(w, string(id)); err != nil {
			return err
		}
		if err := (Uint64Codec{}).Encode(w, uint64(len(ranges))); err != nil {
			return err
		}
		for _, r := range ranges {
			if err := rc.Encode(w, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (MissingCodec) Decode(r io.Reader) (map[ReplicaID][]SeqRange, error) {
	n, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return nil, err
	}
	if n > maxDecodeLen {
		return nil, fmt.Errorf("missing map length %d exceeds max %d", n, maxDecodeLen)
	}
	if n == 0 {
		return nil, nil
	}
	m := make(map[ReplicaID][]SeqRange, n)
	sc := StringCodec{}
	rc := SeqRangeCodec{}
	for i := uint64(0); i < n; i++ {
		id, err := sc.Decode(r)
		if err != nil {
			return nil, err
		}
		numRanges, err := (Uint64Codec{}).Decode(r)
		if err != nil {
			return nil, err
		}
		if numRanges > maxDecodeLen {
			return nil, fmt.Errorf("range count %d exceeds max %d", numRanges, maxDecodeLen)
		}
		ranges := make([]SeqRange, numRanges)
		for j := uint64(0); j < numRanges; j++ {
			ranges[j], err = rc.Decode(r)
			if err != nil {
				return nil, err
			}
		}
		m[ReplicaID(id)] = ranges
	}
	return m, nil
}

// DeltaBatchCodec encodes a batch of (Dot, T) pairs as:
// [uint64: count] ([Dot] [T: via delta codec])*
//
// This is the wire format for shipping deltas over TCP. The delta codec
// is caller-supplied, same pattern as DotFunCodec.
type DeltaBatchCodec[T any] struct {
	DeltaCodec Codec[T]
}

func (c DeltaBatchCodec[T]) Encode(w io.Writer, deltas map[Dot]T) error {
	if err := (Uint64Codec{}).Encode(w, uint64(len(deltas))); err != nil {
		return err
	}
	dc := DotCodec{}
	for d, delta := range deltas {
		if err := dc.Encode(w, d); err != nil {
			return err
		}
		if err := c.DeltaCodec.Encode(w, delta); err != nil {
			return err
		}
	}
	return nil
}

func (c DeltaBatchCodec[T]) Decode(r io.Reader) (map[Dot]T, error) {
	n, err := (Uint64Codec{}).Decode(r)
	if err != nil {
		return nil, err
	}
	if n > maxDecodeLen {
		return nil, fmt.Errorf("delta batch length %d exceeds max %d", n, maxDecodeLen)
	}
	if n == 0 {
		return nil, nil
	}
	deltas := make(map[Dot]T, n)
	dc := DotCodec{}
	for i := uint64(0); i < n; i++ {
		d, err := dc.Decode(r)
		if err != nil {
			return nil, err
		}
		delta, err := c.DeltaCodec.Decode(r)
		if err != nil {
			return nil, err
		}
		deltas[d] = delta
	}
	return deltas, nil
}

// CausalCodec encodes a Causal[T] as [T: store] [CausalContext].
type CausalCodec[T DotStore] struct {
	StoreCodec Codec[T]
}

func (c CausalCodec[T]) Encode(w io.Writer, v Causal[T]) error {
	if err := c.StoreCodec.Encode(w, v.Store); err != nil {
		return err
	}
	return (CausalContextCodec{}).Encode(w, v.Context)
}

func (c CausalCodec[T]) Decode(r io.Reader) (Causal[T], error) {
	store, err := c.StoreCodec.Decode(r)
	if err != nil {
		var zero Causal[T]
		return zero, err
	}
	ctx, err := (CausalContextCodec{}).Decode(r)
	if err != nil {
		var zero Causal[T]
		return zero, err
	}
	return Causal[T]{Store: store, Context: ctx}, nil
}
