# Delta Serialization Design

Custom binary codec for wire-encoding CRDT deltas.

## Decisions

- **Location:** crdt library (`dotcontext/codec.go`), not tessera
- **Format:** Custom binary, little-endian, fixed-width integers
- **Type awareness:** Receiver knows type at decode time (no self-describing tags)
- **Dependencies:** Zero — stdlib `io` and `encoding/binary` only
- **Versioning:** No version byte yet (zero consumers, format can break freely)

## Codec Interface

```go
type Codec[T any] interface {
    Encode(w io.Writer, v T) error
    Decode(r io.Reader) (T, error)
}
```

Codecs compose: callers supply codecs for generic type parameters.

## Wire Format

Primitives:

| Type | Encoding |
|------|----------|
| uint64 | 8 bytes LE fixed |
| int64 | 8 bytes LE fixed |
| string | uint64 length + raw bytes |

Composite layouts:

```
Dot:            [string: ID] [uint64: Seq]

CausalContext:  [uint64: vv_len]
                  [string: replicaID] [uint64: seq]    x vv_len
                [uint64: outliers_len]
                  [Dot]                                x outliers_len

DotSet:         [uint64: len]
                  [Dot]                                x len

DotFun[V]:      [uint64: len]
                  [Dot] [V: via value codec]           x len

DotMap[K,V]:    [uint64: len]
                  [K: via key codec] [V: via value codec]  x len

Causal[T]:      [T: via store codec]
                [CausalContext]
```

## Provided Codecs

In `dotcontext/codec.go`:

| Codec | Type | Notes |
|-------|------|-------|
| StringCodec | string | Primitive |
| Uint64Codec | uint64 | Primitive |
| Int64Codec | int64 | Primitive |
| DotCodec | Dot | Uses StringCodec + Uint64Codec |
| CausalContextCodec | *CausalContext | Same-package access to vv/outliers |
| DotSetCodec | *DotSet | Same-package access to dots map |
| DotFunCodec[V] | *DotFun[V] | Takes ValueCodec Codec[V] |
| DotMapCodec[K,V] | *DotMap[K,V] | Takes KeyCodec + ValueCodec |
| CausalCodec[T] | Causal[T] | Takes StoreCodec Codec[T] |

Tessera composes these for its concrete types:

```go
var blockRefCodec = CausalCodec[*DotMap[string, *DotMap[string, *DotSet]]]{
    StoreCodec: &DotMapCodec[string, *DotMap[string, *DotSet]]{
        KeyCodec: StringCodec{},
        ValueCodec: &DotMapCodec[string, *DotSet]{
            KeyCodec:   StringCodec{},
            ValueCodec: DotSetCodec{},
        },
    },
}
```

## Testing

1. **Round-trip property tests** — Encode then decode each type, verify equality
2. **Fuzz decoders** — Random bytes must return errors, never panic
3. **Integration in tessera** — Encode a real BlockRef delta, decode, merge into fresh replica, verify semantics preserved

Errors are bare io errors; tessera's transport wraps with message context.
