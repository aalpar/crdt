# Delta Serialization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add binary encode/decode for all CRDT delta types so tessera can send deltas over the wire.

**Architecture:** A `Codec[T]` interface in `dotcontext/` with composable implementations. Primitive codecs for string/uint64/int64. Core codecs for Dot, CausalContext, DotSet. Generic codecs (DotFunCodec, DotMapCodec, CausalCodec) take caller-supplied codecs for their type parameters. All same-package, zero external deps, little-endian fixed-width encoding.

**Tech Stack:** Go stdlib only (`io`, `encoding/binary`). Tests use `bytes.Buffer`.

**Design doc:** `docs/plans/2026-03-05-delta-serialization-design.md`

---

### Task 1: Codec Interface + Primitive Codecs

**Files:**
- Create: `dotcontext/codec.go`
- Create: `dotcontext/codec_test.go`

**Step 1: Write failing tests for primitive codecs**

In `dotcontext/codec_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestStringCodec|TestUint64Codec|TestInt64Codec' -v`
Expected: compilation error (types not defined)

**Step 3: Implement Codec interface and primitive codecs**

In `dotcontext/codec.go`:

```go
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
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestStringCodec|TestUint64Codec|TestInt64Codec' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dotcontext/codec.go dotcontext/codec_test.go
git commit -m "Add Codec interface and primitive codecs (string, uint64, int64)"
```

---

### Task 2: DotCodec + CausalContextCodec

**Files:**
- Modify: `dotcontext/codec.go`
- Modify: `dotcontext/codec_test.go`

**Step 1: Write failing tests**

Append to `dotcontext/codec_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestDotCodec|TestCausalContextCodec' -v`
Expected: compilation error

**Step 3: Implement DotCodec and CausalContextCodec**

Append to `dotcontext/codec.go`:

```go
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
```

**Step 4: Run tests**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestDotCodec|TestCausalContextCodec' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dotcontext/codec.go dotcontext/codec_test.go
git commit -m "Add DotCodec and CausalContextCodec"
```

---

### Task 3: DotSetCodec

**Files:**
- Modify: `dotcontext/codec.go`
- Modify: `dotcontext/codec_test.go`

**Step 1: Write failing test**

```go
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
```

Note: `dotSetEqual` already exists in `join_test.go`. Since both are in `package dotcontext`, the test helper is accessible.

**Step 2: Run to verify failure**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestDotSetCodec' -v`

**Step 3: Implement**

```go
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
```

**Step 4: Run tests**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestDotSetCodec' -v`

**Step 5: Commit**

```bash
git add dotcontext/codec.go dotcontext/codec_test.go
git commit -m "Add DotSetCodec"
```

---

### Task 4: DotFunCodec and DotMapCodec

**Files:**
- Modify: `dotcontext/codec.go`
- Modify: `dotcontext/codec_test.go`

**Step 1: Write failing tests**

```go
// maxInt is already defined in dotfun_test.go — reuse it.
// If it's not visible (same package, it is), define a test lattice:
// type testVal struct{ n int64 }
// func (v testVal) Join(other testVal) testVal { ... }

func TestDotFunCodecRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	// Use Int64Codec wrapped as a Codec[maxInt] — need a small adapter.
	// For test purposes, encode maxInt as int64.
	vc := maxIntCodec{}
	c := DotFunCodec[maxInt]{ValueCodec: vc}

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
```

Also need a test codec for maxInt in the test file:

```go
type maxIntCodec struct{}

func (maxIntCodec) Encode(w io.Writer, v maxInt) error {
	return (Int64Codec{}).Encode(w, int64(v))
}

func (maxIntCodec) Decode(r io.Reader) (maxInt, error) {
	n, err := (Int64Codec{}).Decode(r)
	return maxInt(n), err
}
```

**Step 2: Run to verify failure**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestDotFunCodec|TestDotMapCodec' -v`

**Step 3: Implement**

```go
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
```

**Step 4: Run tests**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestDotFunCodec|TestDotMapCodec' -v`

**Step 5: Commit**

```bash
git add dotcontext/codec.go dotcontext/codec_test.go
git commit -m "Add DotFunCodec and DotMapCodec"
```

---

### Task 5: CausalCodec

**Files:**
- Modify: `dotcontext/codec.go`
- Modify: `dotcontext/codec_test.go`

**Step 1: Write failing test**

```go
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
```

**Step 2: Run to verify failure**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestCausalCodec' -v`

**Step 3: Implement**

```go
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
```

**Step 4: Run tests**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -run 'TestCausalCodec' -v`

**Step 5: Run full test suite**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test -race ./...`
Expected: all tests pass including existing ones

**Step 6: Commit**

```bash
git add dotcontext/codec.go dotcontext/codec_test.go
git commit -m "Add CausalCodec — completes delta serialization codecs"
```

---

### Task 6: Fuzz Decoders

**Files:**
- Modify: `dotcontext/codec_test.go` (or create `dotcontext/codec_fuzz_test.go`)

**Step 1: Write fuzz targets**

```go
func FuzzDecodeDotSet(f *testing.F) {
	// Seed: a valid encoded empty DotSet
	var buf bytes.Buffer
	(DotSetCodec{}).Encode(&buf, NewDotSet())
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		// Must not panic. Errors are fine.
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
```

**Step 2: Run fuzz briefly to verify they work**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -fuzz=FuzzDecodeDotSet -fuzztime=5s`
Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -fuzz=FuzzDecodeCausalContext -fuzztime=5s`
Run: `cd /Users/aalpar/projects/crdt-projects/crdt && go test ./dotcontext/ -fuzz=FuzzDecodeCausalDotSet -fuzztime=5s`

If any panics are found, fix the decoder (likely missing bounds checks on length prefixes) and re-run.

**Step 3: Commit**

```bash
git add dotcontext/codec_test.go  # or codec_fuzz_test.go
git commit -m "Add fuzz targets for binary decoders"
```

---

### Task 7: Tessera Integration — BlockRef Codec

**Files:**
- Create: `tessera/codec.go` (in `/Users/aalpar/projects/crdt-projects/tessera/`)
- Create: `tessera/codec_test.go`

**Step 1: Write failing integration test**

In `tessera/codec_test.go`:

```go
package tessera

import (
	"bytes"
	"testing"

	"github.com/aalpar/crdt/dotcontext"
)

func TestBlockRefDeltaRoundTrip(t *testing.T) {
	// Create a worker, add a reference, capture the delta
	cc := dotcontext.New()
	br := NewBlockRef(cc)
	delta := br.AddRef("w1", "hash-abc", "file-1")

	// Encode
	var buf bytes.Buffer
	if err := EncodeBlockRefDelta(&buf, delta); err != nil {
		t.Fatal(err)
	}

	// Decode
	got, err := DecodeBlockRefDelta(&buf)
	if err != nil {
		t.Fatal(err)
	}

	// Merge into fresh replica and verify
	cc2 := dotcontext.New()
	br2 := NewBlockRef(cc2)
	br2.Merge(got)

	refs := br2.Refs("hash-abc")
	if len(refs) != 1 || refs[0] != "file-1" {
		t.Errorf("expected [file-1], got %v", refs)
	}
}
```

Note: This test depends on `BlockRef.AddRef` returning a delta and `BlockRef.Refs` returning file IDs. Verify the actual API signatures in `blockref.go` before implementing — the method names and signatures may differ. Read `blockref.go` first.

**Step 2: Run to verify failure**

Run: `cd /Users/aalpar/projects/crdt-projects/tessera && go test -run TestBlockRefDeltaRoundTrip -v`

**Step 3: Implement**

In `tessera/codec.go`:

```go
package tessera

import (
	"io"

	"github.com/aalpar/crdt/dotcontext"
)

var blockRefDeltaCodec = dotcontext.CausalCodec[*dotcontext.DotMap[string, *dotcontext.DotMap[string, *dotcontext.DotSet]]]{
	StoreCodec: &dotcontext.DotMapCodec[string, *dotcontext.DotMap[string, *dotcontext.DotSet]]{
		KeyCodec: dotcontext.StringCodec{},
		ValueCodec: &dotcontext.DotMapCodec[string, *dotcontext.DotSet]{
			KeyCodec:   dotcontext.StringCodec{},
			ValueCodec: dotcontext.DotSetCodec{},
		},
	},
}

// EncodeBlockRefDelta encodes a BlockRef delta for wire transport.
func EncodeBlockRefDelta(w io.Writer, delta *BlockRef) error {
	// Extract the Causal value from the BlockRef wrapper.
	// Implementation depends on how BlockRef exposes its inner ORMap/Causal.
	// Read blockref.go to determine the exact accessor.
	return blockRefDeltaCodec.Encode(w, /* delta's inner Causal value */)
}

// DecodeBlockRefDelta decodes a BlockRef delta from the wire.
func DecodeBlockRefDelta(r io.Reader) (*BlockRef, error) {
	causal, err := blockRefDeltaCodec.Decode(r)
	if err != nil {
		return nil, err
	}
	// Wrap the decoded Causal value back into a BlockRef.
	// Implementation depends on BlockRef's constructor.
	_ = causal
	return nil, nil // placeholder
}
```

**Important:** The exact implementation of `EncodeBlockRefDelta` and `DecodeBlockRefDelta` depends on how `BlockRef` wraps its inner `ORMap`. Read `blockref.go` to determine:
1. How to extract the `Causal[*DotMap[...]]` from a `BlockRef`
2. How to reconstruct a `BlockRef` from a decoded `Causal[*DotMap[...]]`

The codec composition (`blockRefDeltaCodec`) is correct regardless.

**Step 4: Run tests**

Run: `cd /Users/aalpar/projects/crdt-projects/tessera && go test -run TestBlockRefDeltaRoundTrip -v`

**Step 5: Run full tessera test suite**

Run: `cd /Users/aalpar/projects/crdt-projects/tessera && go test -race ./...`

**Step 6: Commit**

```bash
git add codec.go codec_test.go
git commit -m "Add BlockRef delta encode/decode using crdt codec"
```

---

### Task 8: Final Validation

**Step 1: Run all tests across both projects**

Run: `cd /Users/aalpar/projects/crdt-projects && go test -race ./crdt/... ./tessera/...`

**Step 2: Run linter on crdt**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && make lint`

**Step 3: Verify no regressions**

Run: `cd /Users/aalpar/projects/crdt-projects/crdt && make test`

**Step 4: Update TODO.md in tessera**

Mark delta serialization as done in `/Users/aalpar/projects/crdt-projects/tessera/TODO.md`:

```diff
-  - [ ] Delta serialization — wire-encodable deltas for real network transport
+  - [x] Delta serialization — wire-encodable deltas for real network transport
```

**Step 5: Commit**

```bash
cd /Users/aalpar/projects/crdt-projects/tessera
git add TODO.md
git commit -m "Mark delta serialization as done"
```
