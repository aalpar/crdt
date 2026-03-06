package dotcontext

import (
	"encoding/binary"
	"io"
)

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
