package dotcontext

import (
	"bytes"
	"errors"
	"io"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestScalarCodecRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("String", func(c *qt.C) {
		var buf bytes.Buffer
		sc := StringCodec{}
		c.Assert(sc.Encode(&buf, "hello"), qt.IsNil)
		got, err := sc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.Equals, "hello")
	})
	c.Run("Uint64", func(c *qt.C) {
		var buf bytes.Buffer
		uc := Uint64Codec{}
		c.Assert(uc.Encode(&buf, 42), qt.IsNil)
		got, err := uc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.Equals, uint64(42))
	})
	c.Run("Int64", func(c *qt.C) {
		var buf bytes.Buffer
		ic := Int64Codec{}
		c.Assert(ic.Encode(&buf, -7), qt.IsNil)
		got, err := ic.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.Equals, int64(-7))
	})
	c.Run("Dot", func(c *qt.C) {
		var buf bytes.Buffer
		dc := DotCodec{}
		d := Dot{ID: "replica-1", Seq: 42}
		c.Assert(dc.Encode(&buf, d), qt.IsNil)
		got, err := dc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.Equals, d)
	})
	c.Run("SeqRange", func(c *qt.C) {
		var buf bytes.Buffer
		rc := SeqRangeCodec{}
		r := SeqRange{Lo: 3, Hi: 7}
		c.Assert(rc.Encode(&buf, r), qt.IsNil)
		got, err := rc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.Equals, r)
	})
}

func TestStoreCodecRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("DotSet", func(c *qt.C) {
		var buf bytes.Buffer
		dsc := DotSetCodec{}
		ds := NewDotSet()
		ds.Add(Dot{ID: "a", Seq: 1})
		ds.Add(Dot{ID: "b", Seq: 3})

		c.Assert(dsc.Encode(&buf, ds), qt.IsNil)
		got, err := dsc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(dotSetEqual(ds, got), qt.IsTrue)
	})
	c.Run("DotFun", func(c *qt.C) {
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
	})
	c.Run("DotMap", func(c *qt.C) {
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
	})
	c.Run("Missing", func(c *qt.C) {
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
	})
	c.Run("DeltaBatch", func(c *qt.C) {
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
	})
}

func TestCodecEmptyRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("String", func(c *qt.C) {
		var buf bytes.Buffer
		sc := StringCodec{}
		c.Assert(sc.Encode(&buf, ""), qt.IsNil)
		got, err := sc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.Equals, "")
	})
	c.Run("DotSet", func(c *qt.C) {
		var buf bytes.Buffer
		dsc := DotSetCodec{}
		ds := NewDotSet()
		c.Assert(dsc.Encode(&buf, ds), qt.IsNil)
		got, err := dsc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Len(), qt.Equals, 0)
	})
	c.Run("DotFun", func(c *qt.C) {
		var buf bytes.Buffer
		fc := DotFunCodec[maxInt]{ValueCodec: maxIntCodec{}}
		df := NewDotFun[maxInt]()
		c.Assert(fc.Encode(&buf, df), qt.IsNil)
		got, err := fc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Len(), qt.Equals, 0)
	})
	c.Run("DotMap", func(c *qt.C) {
		var buf bytes.Buffer
		mc := DotMapCodec[string, *DotSet]{
			KeyCodec:   StringCodec{},
			ValueCodec: DotSetCodec{},
		}
		dm := NewDotMap[string, *DotSet]()
		c.Assert(mc.Encode(&buf, dm), qt.IsNil)
		got, err := mc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Len(), qt.Equals, 0)
	})
	c.Run("Missing", func(c *qt.C) {
		var buf bytes.Buffer
		mc := MissingCodec{}
		c.Assert(mc.Encode(&buf, nil), qt.IsNil)
		got, err := mc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.IsNil)
	})
	c.Run("DeltaBatch", func(c *qt.C) {
		var buf bytes.Buffer
		bc := DeltaBatchCodec[int64]{DeltaCodec: Int64Codec{}}
		c.Assert(bc.Encode(&buf, nil), qt.IsNil)
		got, err := bc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.IsNil)
	})
}

func TestCausalContextCodec(t *testing.T) {
	c := qt.New(t)

	c.Run("RoundTrip", func(c *qt.C) {
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
	})
	c.Run("Empty", func(c *qt.C) {
		var buf bytes.Buffer
		ccc := CausalContextCodec{}
		cc := New()
		c.Assert(ccc.Encode(&buf, cc), qt.IsNil)
		got, err := ccc.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Max("anything"), qt.Equals, uint64(0))
	})
	c.Run("PreservesOutliers", func(c *qt.C) {
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
	})
}

type maxIntCodec struct{}

func (maxIntCodec) Encode(w io.Writer, v maxInt) error {
	return (Int64Codec{}).Encode(w, int64(v))
}

func (maxIntCodec) Decode(r io.Reader) (maxInt, error) {
	n, err := (Int64Codec{}).Decode(r)
	return maxInt(n), err
}

func TestDotMapCodecNested(t *testing.T) {
	c := qt.New(t)

	c.Run("TwoLevel", func(c *qt.C) {
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
	})
	c.Run("ThreeLevel", func(c *qt.C) {
		var buf bytes.Buffer

		level1 := DotMapCodec[string, *DotSet]{
			KeyCodec: StringCodec{}, ValueCodec: DotSetCodec{},
		}
		level2 := DotMapCodec[string, *DotMap[string, *DotSet]]{
			KeyCodec: StringCodec{}, ValueCodec: &level1,
		}
		level3 := DotMapCodec[string, *DotMap[string, *DotMap[string, *DotSet]]]{
			KeyCodec: StringCodec{}, ValueCodec: &level2,
		}

		// Build: outer["a"]["b"]["c"] = DotSet{(w1:1)}
		leaf := NewDotSet()
		leaf.Add(Dot{ID: "w1", Seq: 1})

		inner := NewDotMap[string, *DotSet]()
		inner.Set("c", leaf)

		mid := NewDotMap[string, *DotMap[string, *DotSet]]()
		mid.Set("b", inner)

		outer := NewDotMap[string, *DotMap[string, *DotMap[string, *DotSet]]]()
		outer.Set("a", mid)

		c.Assert(level3.Encode(&buf, outer), qt.IsNil)
		got, err := level3.Decode(&buf)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Len(), qt.Equals, 1)

		gotMid, ok := got.Get("a")
		c.Assert(ok, qt.IsTrue)
		gotInner, ok := gotMid.Get("b")
		c.Assert(ok, qt.IsTrue)
		gotLeaf, ok := gotInner.Get("c")
		c.Assert(ok, qt.IsTrue)
		c.Assert(gotLeaf.Has(Dot{ID: "w1", Seq: 1}), qt.IsTrue)
	})
}

func TestCausalCodecRoundTrip(t *testing.T) {
	c := qt.New(t)

	c.Run("DotSet", func(c *qt.C) {
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
	})
	c.Run("NestedDotMap", func(c *qt.C) {
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
	})
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

func TestDotFunCodecMultiField(t *testing.T) {
	c := qt.New(t)

	c.Run("Struct", func(c *qt.C) {
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
	})
	c.Run("PreservesJoin", func(c *qt.C) {
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
	})
}

// --- maxDecodeLen boundary ---

func TestDecodeRejectsOversizedLength(t *testing.T) {
	c := qt.New(t)

	// Encode a uint64 length that exceeds maxDecodeLen, followed by
	// enough padding that the decoder reaches the length check.
	var buf bytes.Buffer
	Uint64Codec{}.Encode(&buf, maxDecodeLen+1)

	// Each decoder that uses a length prefix should reject this.
	_, err := (DotSetCodec{}).Decode(bytes.NewReader(buf.Bytes()))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Matches, `.*exceeds max.*`)

	_, err = (StringCodec{}).Decode(bytes.NewReader(buf.Bytes()))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Matches, `.*exceeds max.*`)

	_, err = (CausalContextCodec{}).Decode(bytes.NewReader(buf.Bytes()))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Matches, `.*exceeds max.*`)

	_, err = (MissingCodec{}).Decode(bytes.NewReader(buf.Bytes()))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Matches, `.*exceeds max.*`)

	_, err = (DeltaBatchCodec[int64]{DeltaCodec: Int64Codec{}}).Decode(bytes.NewReader(buf.Bytes()))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Matches, `.*exceeds max.*`)
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

// --- Encode / Decode error propagation ---

var errBrokenWriter = errors.New("broken writer")

// brokenWriter is an io.Writer that always returns errBrokenWriter.
type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errBrokenWriter }

// limitWriter succeeds for the first n bytes, then returns errBrokenWriter.
type limitWriter struct{ n int }

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return 0, errBrokenWriter
	}
	if len(p) <= w.n {
		w.n -= len(p)
		return len(p), nil
	}
	written := w.n
	w.n = 0
	return written, errBrokenWriter
}

func TestEncodePropagatesWriterError(t *testing.T) {
	c := qt.New(t)
	w := brokenWriter{}

	c.Run("StringCodec", func(c *qt.C) {
		c.Assert(StringCodec{}.Encode(w, "hello"), qt.IsNotNil)
	})
	c.Run("Uint64Codec", func(c *qt.C) {
		c.Assert(Uint64Codec{}.Encode(w, 42), qt.IsNotNil)
	})
	c.Run("Int64Codec", func(c *qt.C) {
		c.Assert(Int64Codec{}.Encode(w, -7), qt.IsNotNil)
	})
	c.Run("DotCodec", func(c *qt.C) {
		c.Assert(DotCodec{}.Encode(w, Dot{ID: "a", Seq: 1}), qt.IsNotNil)
	})
	c.Run("CausalContextCodec", func(c *qt.C) {
		cc := New()
		cc.Add(Dot{ID: "a", Seq: 1})
		c.Assert(CausalContextCodec{}.Encode(w, cc), qt.IsNotNil)
	})
	c.Run("DotSetCodec", func(c *qt.C) {
		ds := NewDotSet()
		ds.Add(Dot{ID: "a", Seq: 1})
		c.Assert(DotSetCodec{}.Encode(w, ds), qt.IsNotNil)
	})
	c.Run("DotFunCodec", func(c *qt.C) {
		df := NewDotFun[maxInt]()
		df.Set(Dot{ID: "a", Seq: 1}, maxInt(10))
		c.Assert((DotFunCodec[maxInt]{ValueCodec: maxIntCodec{}}).Encode(w, df), qt.IsNotNil)
	})
	c.Run("DotMapCodec", func(c *qt.C) {
		dm := NewDotMap[string, *DotSet]()
		ds := NewDotSet()
		ds.Add(Dot{ID: "a", Seq: 1})
		dm.Set("key", ds)
		mc := DotMapCodec[string, *DotSet]{KeyCodec: StringCodec{}, ValueCodec: DotSetCodec{}}
		c.Assert(mc.Encode(w, dm), qt.IsNotNil)
	})
	c.Run("CausalCodec", func(c *qt.C) {
		ds := NewDotSet()
		ds.Add(Dot{ID: "a", Seq: 1})
		ctx := New()
		ctx.Add(Dot{ID: "a", Seq: 1})
		cc := CausalCodec[*DotSet]{StoreCodec: DotSetCodec{}}
		c.Assert(cc.Encode(w, Causal[*DotSet]{Store: ds, Context: ctx}), qt.IsNotNil)
	})
	c.Run("SeqRangeCodec", func(c *qt.C) {
		c.Assert(SeqRangeCodec{}.Encode(w, SeqRange{Lo: 1, Hi: 5}), qt.IsNotNil)
	})
	c.Run("MissingCodec", func(c *qt.C) {
		m := map[ReplicaID][]SeqRange{"a": {{Lo: 1, Hi: 3}}}
		c.Assert(MissingCodec{}.Encode(w, m), qt.IsNotNil)
	})
	c.Run("DeltaBatchCodec", func(c *qt.C) {
		deltas := map[Dot]int64{{ID: "a", Seq: 1}: 100}
		bc := DeltaBatchCodec[int64]{DeltaCodec: Int64Codec{}}
		c.Assert(bc.Encode(w, deltas), qt.IsNotNil)
	})
}

// TestEncodeMidStreamWriterError verifies that codecs using Range-based
// iteration (DotSet, DotFun, DotMap) propagate errors that occur after
// the length prefix has been written successfully.
func TestEncodeMidStreamWriterError(t *testing.T) {
	c := qt.New(t)

	// Encoded size of Dot{ID: single-char, Seq: _} = 8 (string len) + 1 (char) + 8 (seq) = 17.
	const dotSize = 17

	c.Run("DotSetCodec", func(c *qt.C) {
		ds := NewDotSet()
		ds.Add(Dot{ID: "a", Seq: 1})
		ds.Add(Dot{ID: "b", Seq: 2})
		// Allow length prefix (8) + one dot (17), fail on second dot.
		w := &limitWriter{n: 8 + dotSize}
		c.Assert(DotSetCodec{}.Encode(w, ds), qt.IsNotNil)
	})
	c.Run("DotFunCodec", func(c *qt.C) {
		df := NewDotFun[maxInt]()
		df.Set(Dot{ID: "a", Seq: 1}, maxInt(10))
		df.Set(Dot{ID: "b", Seq: 2}, maxInt(20))
		// Allow length prefix (8) + one dot (17) + one int64 value (8), fail on second pair.
		w := &limitWriter{n: 8 + dotSize + 8}
		c.Assert((DotFunCodec[maxInt]{ValueCodec: maxIntCodec{}}).Encode(w, df), qt.IsNotNil)
	})
	c.Run("DotMapCodec", func(c *qt.C) {
		dm := NewDotMap[string, *DotSet]()
		ds1 := NewDotSet()
		ds1.Add(Dot{ID: "a", Seq: 1})
		dm.Set("k1", ds1)
		ds2 := NewDotSet()
		ds2.Add(Dot{ID: "b", Seq: 2})
		dm.Set("k2", ds2)
		// One entry: key "k1" (8+2) + dotset (8+17) = 35. Allow prefix (8) + one entry.
		w := &limitWriter{n: 8 + (8 + 2) + (8 + dotSize)}
		mc := DotMapCodec[string, *DotSet]{KeyCodec: StringCodec{}, ValueCodec: DotSetCodec{}}
		c.Assert(mc.Encode(w, dm), qt.IsNotNil)
	})
}

func TestDecodeTruncatedInput(t *testing.T) {
	c := qt.New(t)

	// truncated encodes valid data, then returns a reader missing the last byte.
	truncated := func(encode func(*bytes.Buffer)) io.Reader {
		var buf bytes.Buffer
		encode(&buf)
		data := buf.Bytes()
		return bytes.NewReader(data[:len(data)-1])
	}

	c.Run("StringCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) { StringCodec{}.Encode(buf, "hello") })
		_, err := StringCodec{}.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("Uint64Codec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) { Uint64Codec{}.Encode(buf, 42) })
		_, err := Uint64Codec{}.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("Int64Codec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) { Int64Codec{}.Encode(buf, -7) })
		_, err := Int64Codec{}.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("DotCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) { DotCodec{}.Encode(buf, Dot{ID: "a", Seq: 1}) })
		_, err := DotCodec{}.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("CausalContextCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) {
			cc := New()
			cc.Add(Dot{ID: "a", Seq: 1})
			CausalContextCodec{}.Encode(buf, cc)
		})
		_, err := CausalContextCodec{}.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("DotSetCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) {
			ds := NewDotSet()
			ds.Add(Dot{ID: "a", Seq: 1})
			DotSetCodec{}.Encode(buf, ds)
		})
		_, err := DotSetCodec{}.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("DotFunCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) {
			df := NewDotFun[maxInt]()
			df.Set(Dot{ID: "a", Seq: 1}, maxInt(10))
			(DotFunCodec[maxInt]{ValueCodec: maxIntCodec{}}).Encode(buf, df)
		})
		_, err := (DotFunCodec[maxInt]{ValueCodec: maxIntCodec{}}).Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("DotMapCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) {
			dm := NewDotMap[string, *DotSet]()
			ds := NewDotSet()
			ds.Add(Dot{ID: "a", Seq: 1})
			dm.Set("key", ds)
			mc := DotMapCodec[string, *DotSet]{KeyCodec: StringCodec{}, ValueCodec: DotSetCodec{}}
			mc.Encode(buf, dm)
		})
		mc := DotMapCodec[string, *DotSet]{KeyCodec: StringCodec{}, ValueCodec: DotSetCodec{}}
		_, err := mc.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("CausalCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) {
			ds := NewDotSet()
			ds.Add(Dot{ID: "a", Seq: 1})
			ctx := New()
			ctx.Add(Dot{ID: "a", Seq: 1})
			cc := CausalCodec[*DotSet]{StoreCodec: DotSetCodec{}}
			cc.Encode(buf, Causal[*DotSet]{Store: ds, Context: ctx})
		})
		cc := CausalCodec[*DotSet]{StoreCodec: DotSetCodec{}}
		_, err := cc.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("SeqRangeCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) { SeqRangeCodec{}.Encode(buf, SeqRange{Lo: 1, Hi: 5}) })
		_, err := SeqRangeCodec{}.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("MissingCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) {
			m := map[ReplicaID][]SeqRange{"a": {{Lo: 1, Hi: 3}}}
			MissingCodec{}.Encode(buf, m)
		})
		_, err := MissingCodec{}.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
	c.Run("DeltaBatchCodec", func(c *qt.C) {
		r := truncated(func(buf *bytes.Buffer) {
			deltas := map[Dot]int64{{ID: "a", Seq: 1}: 100}
			bc := DeltaBatchCodec[int64]{DeltaCodec: Int64Codec{}}
			bc.Encode(buf, deltas)
		})
		bc := DeltaBatchCodec[int64]{DeltaCodec: Int64Codec{}}
		_, err := bc.Decode(r)
		c.Assert(err, qt.IsNotNil)
	})
}

// TestDecodeAtExactMaxDecodeLen verifies the boundary: length == maxDecodeLen
// passes the length check (fails later on insufficient data), while
// length == maxDecodeLen+1 is rejected with "exceeds max" (tested above).
func TestDecodeAtExactMaxDecodeLen(t *testing.T) {
	c := qt.New(t)

	// Encode a length prefix of exactly maxDecodeLen.
	var buf bytes.Buffer
	Uint64Codec{}.Encode(&buf, maxDecodeLen)
	header := buf.Bytes()

	// Each decoder should pass the length check but fail on
	// insufficient data — error must NOT be "exceeds max".
	check := func(c *qt.C, err error) {
		c.Helper()
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Not(qt.Matches), `.*exceeds max.*`)
	}

	c.Run("StringCodec", func(c *qt.C) {
		_, err := (StringCodec{}).Decode(bytes.NewReader(header))
		check(c, err)
	})
	c.Run("DotSetCodec", func(c *qt.C) {
		_, err := (DotSetCodec{}).Decode(bytes.NewReader(header))
		check(c, err)
	})
	c.Run("DotFunCodec", func(c *qt.C) {
		_, err := (DotFunCodec[maxInt]{ValueCodec: maxIntCodec{}}).Decode(bytes.NewReader(header))
		check(c, err)
	})
	c.Run("DotMapCodec", func(c *qt.C) {
		mc := DotMapCodec[string, *DotSet]{KeyCodec: StringCodec{}, ValueCodec: DotSetCodec{}}
		_, err := mc.Decode(bytes.NewReader(header))
		check(c, err)
	})
	c.Run("CausalContextCodec", func(c *qt.C) {
		_, err := (CausalContextCodec{}).Decode(bytes.NewReader(header))
		check(c, err)
	})
	c.Run("MissingCodec", func(c *qt.C) {
		_, err := (MissingCodec{}).Decode(bytes.NewReader(header))
		check(c, err)
	})
	c.Run("DeltaBatchCodec", func(c *qt.C) {
		_, err := (DeltaBatchCodec[int64]{DeltaCodec: Int64Codec{}}).Decode(bytes.NewReader(header))
		check(c, err)
	})
}
