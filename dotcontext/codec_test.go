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

func TestDotCodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := DotCodec{}
	d := Dot{ID: "replica-1", Seq: 42}
	if err := c.Encode(&buf, d); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != d {
		t.Errorf("got %v, want %v", got, d)
	}
}

func TestCausalContextCodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := CausalContextCodec{}

	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 2})
	cc.Add(Dot{ID: "b", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 5}) // outlier (gap at 3,4)

	if err := c.Encode(&buf, cc); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}

	// Check version vector
	if got.Max("a") != cc.Max("a") {
		t.Errorf("vv[a]: got %d, want %d", got.Max("a"), cc.Max("a"))
	}
	if got.Max("b") != cc.Max("b") {
		t.Errorf("vv[b]: got %d, want %d", got.Max("b"), cc.Max("b"))
	}
	// Check outlier survived
	if !got.Has(Dot{ID: "a", Seq: 5}) {
		t.Error("outlier (a,5) missing after round-trip")
	}
	// Check non-outlier not falsely present
	if got.Has(Dot{ID: "a", Seq: 4}) {
		t.Error("(a,4) should not be present")
	}
}

func TestCausalContextCodecEmpty(t *testing.T) {
	var buf bytes.Buffer
	c := CausalContextCodec{}
	cc := New()
	if err := c.Encode(&buf, cc); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Max("anything") != 0 {
		t.Error("empty context should have zero max")
	}
}
