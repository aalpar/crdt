package dotcontext

import (
	"bytes"
	"io"
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

func TestDotSetCodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := DotSetCodec{}
	ds := NewDotSet()
	ds.Add(Dot{ID: "a", Seq: 1})
	ds.Add(Dot{ID: "b", Seq: 3})

	if err := c.Encode(&buf, ds); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !dotSetEqual(ds, got) {
		t.Error("DotSet round-trip failed")
	}
}

func TestDotSetCodecEmpty(t *testing.T) {
	var buf bytes.Buffer
	c := DotSetCodec{}
	ds := NewDotSet()
	if err := c.Encode(&buf, ds); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != 0 {
		t.Error("empty DotSet should decode to empty")
	}
}

type maxIntCodec struct{}

func (maxIntCodec) Encode(w io.Writer, v maxInt) error {
	return (Int64Codec{}).Encode(w, int64(v))
}

func (maxIntCodec) Decode(r io.Reader) (maxInt, error) {
	n, err := (Int64Codec{}).Decode(r)
	return maxInt(n), err
}

func TestDotFunCodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := DotFunCodec[maxInt]{ValueCodec: maxIntCodec{}}

	df := NewDotFun[maxInt]()
	df.Set(Dot{ID: "a", Seq: 1}, maxInt(10))
	df.Set(Dot{ID: "b", Seq: 2}, maxInt(-5))

	if err := c.Encode(&buf, df); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != 2 {
		t.Fatalf("got %d entries, want 2", got.Len())
	}
	v, ok := got.Get(Dot{ID: "a", Seq: 1})
	if !ok || v != 10 {
		t.Errorf("entry (a,1): got %v/%v, want 10/true", v, ok)
	}
	v, ok = got.Get(Dot{ID: "b", Seq: 2})
	if !ok || v != -5 {
		t.Errorf("entry (b,2): got %v/%v, want -5/true", v, ok)
	}
}

func TestDotMapCodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := DotMapCodec[string, *DotSet]{
		KeyCodec:   StringCodec{},
		ValueCodec: DotSetCodec{},
	}

	dm := NewDotMap[string, *DotSet]()
	ds1 := NewDotSet()
	ds1.Add(Dot{ID: "a", Seq: 1})
	dm.Set("key1", ds1)

	ds2 := NewDotSet()
	ds2.Add(Dot{ID: "b", Seq: 2})
	ds2.Add(Dot{ID: "b", Seq: 3})
	dm.Set("key2", ds2)

	if err := c.Encode(&buf, dm); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != 2 {
		t.Fatalf("got %d keys, want 2", got.Len())
	}
	v1, ok := got.Get("key1")
	if !ok || v1.Len() != 1 || !v1.Has(Dot{ID: "a", Seq: 1}) {
		t.Error("key1 mismatch")
	}
	v2, ok := got.Get("key2")
	if !ok || v2.Len() != 2 {
		t.Error("key2 mismatch")
	}
}

func TestDotMapCodecNested(t *testing.T) {
	// Two-level nesting: DotMap[string, *DotMap[string, *DotSet]]
	// This is the BlockRef structure.
	var buf bytes.Buffer
	inner := DotMapCodec[string, *DotSet]{
		KeyCodec:   StringCodec{},
		ValueCodec: DotSetCodec{},
	}
	c := DotMapCodec[string, *DotMap[string, *DotSet]]{
		KeyCodec:   StringCodec{},
		ValueCodec: &inner,
	}

	outer := NewDotMap[string, *DotMap[string, *DotSet]]()
	innerMap := NewDotMap[string, *DotSet]()
	ds := NewDotSet()
	ds.Add(Dot{ID: "w1", Seq: 1})
	innerMap.Set("file-a", ds)
	outer.Set("hash-abc", innerMap)

	if err := c.Encode(&buf, outer); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != 1 {
		t.Fatal("outer map should have 1 entry")
	}
	im, ok := got.Get("hash-abc")
	if !ok {
		t.Fatal("missing hash-abc")
	}
	ids, ok := im.Get("file-a")
	if !ok || !ids.Has(Dot{ID: "w1", Seq: 1}) {
		t.Error("inner dot missing")
	}
}

func TestCausalCodecDotSetRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := CausalCodec[*DotSet]{StoreCodec: DotSetCodec{}}

	ds := NewDotSet()
	ds.Add(Dot{ID: "a", Seq: 1})
	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	causal := Causal[*DotSet]{Store: ds, Context: cc}

	if err := c.Encode(&buf, causal); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !dotSetEqual(got.Store, ds) {
		t.Error("store mismatch")
	}
	if !got.Context.Has(Dot{ID: "a", Seq: 1}) {
		t.Error("context missing dot")
	}
}

func TestCausalCodecNestedDotMapRoundTrip(t *testing.T) {
	// Full BlockRef delta shape: Causal[*DotMap[string, *DotMap[string, *DotSet]]]
	var buf bytes.Buffer
	inner := DotMapCodec[string, *DotSet]{
		KeyCodec:   StringCodec{},
		ValueCodec: DotSetCodec{},
	}
	c := CausalCodec[*DotMap[string, *DotMap[string, *DotSet]]]{
		StoreCodec: &DotMapCodec[string, *DotMap[string, *DotSet]]{
			KeyCodec:   StringCodec{},
			ValueCodec: &inner,
		},
	}

	// Build a realistic delta
	innerMap := NewDotMap[string, *DotSet]()
	ds := NewDotSet()
	ds.Add(Dot{ID: "w1", Seq: 1})
	innerMap.Set("file-a", ds)

	outerMap := NewDotMap[string, *DotMap[string, *DotSet]]()
	outerMap.Set("hash-abc", innerMap)

	ctx := New()
	ctx.Add(Dot{ID: "w1", Seq: 1})

	causal := Causal[*DotMap[string, *DotMap[string, *DotSet]]]{
		Store:   outerMap,
		Context: ctx,
	}

	if err := c.Encode(&buf, causal); err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}

	// Verify store
	om, ok := got.Store.Get("hash-abc")
	if !ok {
		t.Fatal("missing hash-abc in decoded store")
	}
	ids, ok := om.Get("file-a")
	if !ok || !ids.Has(Dot{ID: "w1", Seq: 1}) {
		t.Error("inner dot missing after round-trip")
	}
	// Verify context
	if !got.Context.Has(Dot{ID: "w1", Seq: 1}) {
		t.Error("context dot missing")
	}
}
