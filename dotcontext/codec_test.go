package dotcontext

import (
	"bytes"
	"testing"
)

func TestStringCodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := StringCodec{}
	if err := c.Encode(&buf, "hello"); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestStringCodecEmpty(t *testing.T) {
	var buf bytes.Buffer
	c := StringCodec{}
	if err := c.Encode(&buf, ""); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestUint64CodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := Uint64Codec{}
	if err := c.Encode(&buf, 42); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Errorf("got %d, want 42", got)
	}
}

func TestInt64CodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := Int64Codec{}
	if err := c.Encode(&buf, -7); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != -7 {
		t.Errorf("got %d, want -7", got)
	}
}
