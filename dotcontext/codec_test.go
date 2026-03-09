package dotcontext

import (
	"bytes"
	"io"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestStringCodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	sc := StringCodec{}
	c.Assert(sc.Encode(&buf, "hello"), qt.IsNil)
	got, err := sc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, "hello")
}

func TestStringCodecEmpty(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	sc := StringCodec{}
	c.Assert(sc.Encode(&buf, ""), qt.IsNil)
	got, err := sc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, "")
}

func TestUint64CodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	uc := Uint64Codec{}
	c.Assert(uc.Encode(&buf, 42), qt.IsNil)
	got, err := uc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, uint64(42))
}

func TestInt64CodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	ic := Int64Codec{}
	c.Assert(ic.Encode(&buf, -7), qt.IsNil)
	got, err := ic.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, int64(-7))
}

func TestDotCodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	dc := DotCodec{}
	d := Dot{ID: "replica-1", Seq: 42}
	c.Assert(dc.Encode(&buf, d), qt.IsNil)
	got, err := dc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, d)
}

func TestCausalContextCodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	ccc := CausalContextCodec{}

	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 2})
	cc.Add(Dot{ID: "b", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 5}) // outlier (gap at 3,4)

	c.Assert(ccc.Encode(&buf, cc), qt.IsNil)
	got, err := ccc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Max("a"), qt.Equals, cc.Max("a"))
	c.Assert(got.Max("b"), qt.Equals, cc.Max("b"))
	c.Assert(got.Has(Dot{ID: "a", Seq: 5}), qt.IsTrue)
	c.Assert(got.Has(Dot{ID: "a", Seq: 4}), qt.IsFalse)
}

func TestCausalContextCodecEmpty(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	ccc := CausalContextCodec{}
	cc := New()
	c.Assert(ccc.Encode(&buf, cc), qt.IsNil)
	got, err := ccc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Max("anything"), qt.Equals, uint64(0))
}

func TestDotSetCodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	dsc := DotSetCodec{}
	ds := NewDotSet()
	ds.Add(Dot{ID: "a", Seq: 1})
	ds.Add(Dot{ID: "b", Seq: 3})

	c.Assert(dsc.Encode(&buf, ds), qt.IsNil)
	got, err := dsc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(dotSetEqual(ds, got), qt.IsTrue)
}

func TestDotSetCodecEmpty(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	dsc := DotSetCodec{}
	ds := NewDotSet()
	c.Assert(dsc.Encode(&buf, ds), qt.IsNil)
	got, err := dsc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Len(), qt.Equals, 0)
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
	c := qt.New(t)
	var buf bytes.Buffer
	fc := DotFunCodec[maxInt]{ValueCodec: maxIntCodec{}}

	df := NewDotFun[maxInt]()
	df.Set(Dot{ID: "a", Seq: 1}, maxInt(10))
	df.Set(Dot{ID: "b", Seq: 2}, maxInt(-5))

	c.Assert(fc.Encode(&buf, df), qt.IsNil)
	got, err := fc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Len(), qt.Equals, 2)

	v, ok := got.Get(Dot{ID: "a", Seq: 1})
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, maxInt(10))

	v, ok = got.Get(Dot{ID: "b", Seq: 2})
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, maxInt(-5))
}

func TestDotMapCodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	mc := DotMapCodec[string, *DotSet]{
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

	c.Assert(mc.Encode(&buf, dm), qt.IsNil)
	got, err := mc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Len(), qt.Equals, 2)

	v1, ok := got.Get("key1")
	c.Assert(ok, qt.IsTrue)
	c.Assert(v1.Len(), qt.Equals, 1)
	c.Assert(v1.Has(Dot{ID: "a", Seq: 1}), qt.IsTrue)

	v2, ok := got.Get("key2")
	c.Assert(ok, qt.IsTrue)
	c.Assert(v2.Len(), qt.Equals, 2)
}

func TestDotMapCodecNested(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	inner := DotMapCodec[string, *DotSet]{
		KeyCodec:   StringCodec{},
		ValueCodec: DotSetCodec{},
	}
	mc := DotMapCodec[string, *DotMap[string, *DotSet]]{
		KeyCodec:   StringCodec{},
		ValueCodec: &inner,
	}

	outer := NewDotMap[string, *DotMap[string, *DotSet]]()
	innerMap := NewDotMap[string, *DotSet]()
	ds := NewDotSet()
	ds.Add(Dot{ID: "w1", Seq: 1})
	innerMap.Set("file-a", ds)
	outer.Set("hash-abc", innerMap)

	c.Assert(mc.Encode(&buf, outer), qt.IsNil)
	got, err := mc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Len(), qt.Equals, 1)

	im, ok := got.Get("hash-abc")
	c.Assert(ok, qt.IsTrue)
	ids, ok := im.Get("file-a")
	c.Assert(ok, qt.IsTrue)
	c.Assert(ids.Has(Dot{ID: "w1", Seq: 1}), qt.IsTrue)
}

func TestCausalCodecDotSetRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	cc := CausalCodec[*DotSet]{StoreCodec: DotSetCodec{}}

	ds := NewDotSet()
	ds.Add(Dot{ID: "a", Seq: 1})
	ctx := New()
	ctx.Add(Dot{ID: "a", Seq: 1})
	causal := Causal[*DotSet]{Store: ds, Context: ctx}

	c.Assert(cc.Encode(&buf, causal), qt.IsNil)
	got, err := cc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(dotSetEqual(got.Store, ds), qt.IsTrue)
	c.Assert(got.Context.Has(Dot{ID: "a", Seq: 1}), qt.IsTrue)
}

func TestSeqRangeCodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	rc := SeqRangeCodec{}
	r := SeqRange{Lo: 3, Hi: 7}
	c.Assert(rc.Encode(&buf, r), qt.IsNil)
	got, err := rc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, r)
}

func TestMissingCodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	mc := MissingCodec{}
	m := map[ReplicaID][]SeqRange{
		"a": {{Lo: 1, Hi: 3}, {Lo: 7, Hi: 10}},
		"b": {{Lo: 5, Hi: 5}},
	}
	c.Assert(mc.Encode(&buf, m), qt.IsNil)
	got, err := mc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(len(got), qt.Equals, 2)

	aRanges := got["a"]
	c.Assert(aRanges, qt.HasLen, 2)
	c.Assert(aRanges[0], qt.Equals, SeqRange{Lo: 1, Hi: 3})
	c.Assert(aRanges[1], qt.Equals, SeqRange{Lo: 7, Hi: 10})

	bRanges := got["b"]
	c.Assert(bRanges, qt.HasLen, 1)
	c.Assert(bRanges[0], qt.Equals, SeqRange{Lo: 5, Hi: 5})
}

func TestMissingCodecEmpty(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	mc := MissingCodec{}
	c.Assert(mc.Encode(&buf, nil), qt.IsNil)
	got, err := mc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsNil)
}

func TestDeltaBatchCodecRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	bc := DeltaBatchCodec[int64]{DeltaCodec: Int64Codec{}}
	deltas := map[Dot]int64{
		{ID: "a", Seq: 1}: 100,
		{ID: "b", Seq: 3}: -42,
	}
	c.Assert(bc.Encode(&buf, deltas), qt.IsNil)
	got, err := bc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(len(got), qt.Equals, 2)
	c.Assert(got[Dot{ID: "a", Seq: 1}], qt.Equals, int64(100))
	c.Assert(got[Dot{ID: "b", Seq: 3}], qt.Equals, int64(-42))
}

func TestDeltaBatchCodecEmpty(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	bc := DeltaBatchCodec[int64]{DeltaCodec: Int64Codec{}}
	c.Assert(bc.Encode(&buf, nil), qt.IsNil)
	got, err := bc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsNil)
}

func TestCausalCodecNestedDotMapRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	inner := DotMapCodec[string, *DotSet]{
		KeyCodec:   StringCodec{},
		ValueCodec: DotSetCodec{},
	}
	cc := CausalCodec[*DotMap[string, *DotMap[string, *DotSet]]]{
		StoreCodec: &DotMapCodec[string, *DotMap[string, *DotSet]]{
			KeyCodec:   StringCodec{},
			ValueCodec: &inner,
		},
	}

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

	c.Assert(cc.Encode(&buf, causal), qt.IsNil)
	got, err := cc.Decode(&buf)
	c.Assert(err, qt.IsNil)

	om, ok := got.Store.Get("hash-abc")
	c.Assert(ok, qt.IsTrue)
	ids, ok := om.Get("file-a")
	c.Assert(ok, qt.IsTrue)
	c.Assert(ids.Has(Dot{ID: "w1", Seq: 1}), qt.IsTrue)
	c.Assert(got.Context.Has(Dot{ID: "w1", Seq: 1}), qt.IsTrue)
}

// --- Codec round-trip preserves join semantics ---

func TestCausalCodecRoundTripPreservesJoin(t *testing.T) {
	c := qt.New(t)

	// Build two Causal[*DotSet] values.
	da := Dot{ID: "a", Seq: 1}
	db := Dot{ID: "b", Seq: 1}

	a := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	a.Store.Add(da)
	a.Context.Add(da)

	b := Causal[*DotSet]{Store: NewDotSet(), Context: New()}
	b.Store.Add(db)
	b.Context.Add(db)

	// Join before encoding.
	directJoin := JoinDotSet(a, b)

	// Encode and decode both, then join.
	cc := CausalCodec[*DotSet]{StoreCodec: DotSetCodec{}}

	var bufA bytes.Buffer
	c.Assert(cc.Encode(&bufA, a), qt.IsNil)
	decodedA, err := cc.Decode(&bufA)
	c.Assert(err, qt.IsNil)

	var bufB bytes.Buffer
	c.Assert(cc.Encode(&bufB, b), qt.IsNil)
	decodedB, err := cc.Decode(&bufB)
	c.Assert(err, qt.IsNil)

	roundTripJoin := JoinDotSet(decodedA, decodedB)

	// Results should be identical.
	c.Assert(dotSetEqual(directJoin.Store, roundTripJoin.Store), qt.IsTrue)
	c.Assert(directJoin.Context.Has(da), qt.Equals, roundTripJoin.Context.Has(da))
	c.Assert(directJoin.Context.Has(db), qt.Equals, roundTripJoin.Context.Has(db))
}

func TestCausalContextCodecPreservesOutliers(t *testing.T) {
	c := qt.New(t)
	ccc := CausalContextCodec{}

	cc := New()
	cc.Add(Dot{ID: "a", Seq: 1})
	cc.Add(Dot{ID: "a", Seq: 3}) // outlier
	cc.Add(Dot{ID: "a", Seq: 5}) // outlier

	var buf bytes.Buffer
	c.Assert(ccc.Encode(&buf, cc), qt.IsNil)
	decoded, err := ccc.Decode(&buf)
	c.Assert(err, qt.IsNil)

	// Outliers should survive encoding — not compacted during codec.
	c.Assert(decoded.Has(Dot{ID: "a", Seq: 1}), qt.IsTrue)
	c.Assert(decoded.Has(Dot{ID: "a", Seq: 2}), qt.IsFalse) // gap
	c.Assert(decoded.Has(Dot{ID: "a", Seq: 3}), qt.IsTrue)
	c.Assert(decoded.Has(Dot{ID: "a", Seq: 4}), qt.IsFalse) // gap
	c.Assert(decoded.Has(Dot{ID: "a", Seq: 5}), qt.IsTrue)
}

// --- DotFun codec with multi-field struct lattice ---

// timestamped pairs a value with a timestamp, similar to lwwregister's internal type.
type timestamped struct {
	value string
	ts    int64
}

func (a timestamped) Join(b timestamped) timestamped {
	if b.ts > a.ts {
		return b
	}
	return a
}

type timestampedCodec struct{}

func (timestampedCodec) Encode(w io.Writer, v timestamped) error {
	if err := (StringCodec{}).Encode(w, v.value); err != nil {
		return err
	}
	return (Int64Codec{}).Encode(w, v.ts)
}

func (timestampedCodec) Decode(r io.Reader) (timestamped, error) {
	val, err := (StringCodec{}).Decode(r)
	if err != nil {
		return timestamped{}, err
	}
	ts, err := (Int64Codec{}).Decode(r)
	if err != nil {
		return timestamped{}, err
	}
	return timestamped{value: val, ts: ts}, nil
}

func TestDotFunCodecMultiFieldStruct(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	fc := DotFunCodec[timestamped]{ValueCodec: timestampedCodec{}}

	df := NewDotFun[timestamped]()
	df.Set(Dot{ID: "a", Seq: 1}, timestamped{value: "hello", ts: 100})
	df.Set(Dot{ID: "b", Seq: 2}, timestamped{value: "world", ts: 200})

	c.Assert(fc.Encode(&buf, df), qt.IsNil)
	got, err := fc.Decode(&buf)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Len(), qt.Equals, 2)

	v1, ok := got.Get(Dot{ID: "a", Seq: 1})
	c.Assert(ok, qt.IsTrue)
	c.Assert(v1.value, qt.Equals, "hello")
	c.Assert(v1.ts, qt.Equals, int64(100))

	v2, ok := got.Get(Dot{ID: "b", Seq: 2})
	c.Assert(ok, qt.IsTrue)
	c.Assert(v2.value, qt.Equals, "world")
	c.Assert(v2.ts, qt.Equals, int64(200))
}

func TestDotFunCodecMultiFieldPreservesJoin(t *testing.T) {
	c := qt.New(t)
	fc := DotFunCodec[timestamped]{ValueCodec: timestampedCodec{}}
	d := Dot{ID: "a", Seq: 1}

	// Two sides with the same dot but different values.
	a := Causal[*DotFun[timestamped]]{Store: NewDotFun[timestamped](), Context: New()}
	a.Store.Set(d, timestamped{value: "old", ts: 10})
	a.Context.Add(d)

	b := Causal[*DotFun[timestamped]]{Store: NewDotFun[timestamped](), Context: New()}
	b.Store.Set(d, timestamped{value: "new", ts: 20})
	b.Context.Add(d)

	// Join before encoding.
	directJoin := JoinDotFun(a, b)

	// Encode, decode, then join.
	var bufA, bufB bytes.Buffer
	c.Assert(fc.Encode(&bufA, a.Store), qt.IsNil)
	decodedA, err := fc.Decode(&bufA)
	c.Assert(err, qt.IsNil)

	c.Assert(fc.Encode(&bufB, b.Store), qt.IsNil)
	decodedB, err := fc.Decode(&bufB)
	c.Assert(err, qt.IsNil)

	roundTripJoin := JoinDotFun(
		Causal[*DotFun[timestamped]]{Store: decodedA, Context: a.Context.Clone()},
		Causal[*DotFun[timestamped]]{Store: decodedB, Context: b.Context.Clone()},
	)

	// The lattice join should pick ts=20 ("new").
	v, ok := directJoin.Store.Get(d)
	c.Assert(ok, qt.IsTrue)
	c.Assert(v.value, qt.Equals, "new")
	c.Assert(v.ts, qt.Equals, int64(20))

	v2, ok := roundTripJoin.Store.Get(d)
	c.Assert(ok, qt.IsTrue)
	c.Assert(v2.value, qt.Equals, v.value)
	c.Assert(v2.ts, qt.Equals, v.ts)
}

// --- Fuzz targets ---

func FuzzDecodeDotSet(f *testing.F) {
	var buf bytes.Buffer
	(DotSetCodec{}).Encode(&buf, NewDotSet())
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		(DotSetCodec{}).Decode(r)
	})
}

func FuzzDecodeCausalContext(f *testing.F) {
	var buf bytes.Buffer
	(CausalContextCodec{}).Encode(&buf, New())
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		(CausalContextCodec{}).Decode(r)
	})
}

func FuzzDecodeCausalDotSet(f *testing.F) {
	var buf bytes.Buffer
	c := CausalCodec[*DotSet]{StoreCodec: DotSetCodec{}}
	c.Encode(&buf, Causal[*DotSet]{Store: NewDotSet(), Context: New()})
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		c.Decode(r)
	})
}

func FuzzDecodeMissing(f *testing.F) {
	var buf bytes.Buffer
	(MissingCodec{}).Encode(&buf, nil)
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		(MissingCodec{}).Decode(r)
	})
}

func FuzzDecodeDeltaBatch(f *testing.F) {
	var buf bytes.Buffer
	c := DeltaBatchCodec[int64]{DeltaCodec: Int64Codec{}}
	c.Encode(&buf, nil)
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		c.Decode(r)
	})
}
