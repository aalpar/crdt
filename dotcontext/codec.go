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
