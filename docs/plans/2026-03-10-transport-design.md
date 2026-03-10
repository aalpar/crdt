# Network Transport Design

Delta-state CRDT replication over the network. Sits on top of the existing
replication layer (`PeerTracker`, `DeltaStore`, `GC`, `WriteDeltaBatch`/`ReadDeltaBatch`).

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Topology | Agnostic | Matches replication layer — `PeerTracker` doesn't assume topology. Caller decides who connects to whom |
| Dissemination | Eager push + pull on connect | Push = event-driven (mutation triggers send). Pull = event-driven (connection triggers catch-up via `Missing()` + `Fetch()`). No timers |
| Wire protocol | `net.Conn` interface, TCP default | Zero-dependency. Callers swap in TLS, Unix sockets, QUIC — anything that implements `net.Conn` |
| Message types | 2: DeltaBatch + Ack | Handshake = mutual Ack exchange on connect. Stateless protocol — no phase distinction |
| API level | Transport type (caller owns CRDT) | Caller owns CRDT, PeerTracker, DeltaStore, wires via callbacks. Matches existing composable-primitives pattern |
| Layering | Conn + Transport | `Conn` handles single-connection framing. `Transport` manages connection pool. Both are public |

## Package

`transport/` — alongside `replication/` and `dotcontext/`. Zero imports from either.
Moves tagged byte payloads; the caller owns serialization.

## Wire Protocol

Every message on the wire:

```
[uint32 big-endian: payload length] [uint8: type tag] [payload bytes]
```

Three wire types (two visible to caller):

| Tag | Name | Payload | Visibility |
|-----|------|---------|------------|
| `0x00` | Hello | replica ID (UTF-8 string) | Internal to `Conn` — handshake only |
| `0x01` | DeltaBatch | caller-encoded delta batch | Public |
| `0x02` | Ack | caller-encoded CausalContext | Public |

Max message size: 16 MiB default (configurable). Reject on read if length exceeds
limit. Length prefix provides frame boundaries independent of payload content —
a codec bug loses one message, not the stream.

ID exchange (internal to `NewConn`): both sides simultaneously send Hello with
their replica ID, both read the peer's Hello. No version negotiation for v0.

## Conn Layer

Wraps `net.Conn` with message framing, type tagging, and ID exchange.

```go
type MessageType uint8

const (
    DeltaBatch MessageType = 0x01
    Ack        MessageType = 0x02
)

type Conn struct {
    raw    net.Conn
    peerID string
    maxMsg uint32      // default 16 MiB
    wmu    sync.Mutex  // serializes writes
}

func NewConn(raw net.Conn, localID string, opts ...ConnOption) (*Conn, error)
func (c *Conn) PeerID() string
func (c *Conn) SendDeltaBatch(payload []byte) error
func (c *Conn) SendAck(payload []byte) error
func (c *Conn) Receive() (MessageType, []byte, error)
func (c *Conn) Close() error
```

**Properties:**
- `NewConn` sends and reads Hello concurrently (avoids deadlock)
- Write-safe (mutex), read-single-owner (one goroutine calls Receive)
- Symmetric — works identically for dialer and acceptor
- Payload length checked against `maxMsg` on both send and receive
- Hello after handshake = protocol violation error

## Transport Layer

Manages a pool of `Conn`s, dispatches to `Handler`.

```go
type Handler interface {
    OnPeerConnect(peerID string)
    OnPeerDisconnect(peerID string, err error)
    OnDeltaBatch(peerID string, payload []byte)
    OnAck(peerID string, payload []byte)
}

type Transport struct {
    localID string
    handler Handler
    mu      sync.Mutex
    peers   map[string]*Conn
    wg      sync.WaitGroup
    done    chan struct{}
}

func New(localID string, handler Handler) *Transport
func (t *Transport) Listen(ln net.Listener) error
func (t *Transport) Connect(addr string) error
func (t *Transport) SendDeltaBatch(peerID string, payload []byte) error
func (t *Transport) SendAck(peerID string, payload []byte) error
func (t *Transport) Peers() []string
func (t *Transport) Close() error
```

**Behavior:**

- `Listen` blocks (like `http.Serve`). Caller runs `go t.Listen(ln)`. `Close()`
  closes the listener, causing Listen to return.
- `Connect` dials, handshakes, registers peer, starts read loop.
- Per-connection read loop: `Receive()` → dispatch to Handler → on error, remove
  peer and call `OnPeerDisconnect`.
- Duplicate peerID: first connection wins. `Connect` returns error; `Listen`
  silently closes the new connection.
- Handler callbacks run on the read goroutine. Slow handler = backpressure on
  that peer's reads. Caller decouples with channels if needed.
- No reconnection, no retry, no keepalives.

## Error Handling

One error type:

```go
type PeerError struct {
    PeerID string
    Op     string  // "send", "receive", "connect"
    Err    error
}

func (e *PeerError) Error() string
func (e *PeerError) Unwrap() error
```

**Conn-level:**

| Scenario | Behavior |
|----------|----------|
| Handshake fails | `NewConn` returns error, raw conn closed |
| Payload exceeds maxMsg on send | Return error, conn stays open |
| Frame exceeds maxMsg on receive | Return error |
| Hello after handshake | Return protocol violation error |
| Write/read on closed conn | Return underlying net.Conn error |

**Transport-level:**

| Scenario | Behavior |
|----------|----------|
| Receive error | Remove peer, `OnPeerDisconnect(peerID, err)` |
| Send to unknown peer | Return `PeerError` |
| Send write failure | Return error; read loop independently fires `OnPeerDisconnect` |
| Duplicate peerID | Close new conn; error (Connect) or skip (Listen) |
| Listener failure | `Listen` returns the error |

## Protocol Flow

```
1. Connect     → both sides send Ack (current CausalContext)
2. Catch-up    → each computes Missing(), sends DeltaBatch if non-empty
3. Merge       → receiver joins delta, sends Ack (updated CC)
4. Steady state: mutation → push DeltaBatch → peer merges → Ack
```

Steps 2-4 use the same two message types with no special cases.

## Testing Strategy

**Conn tests** (over `net.Pipe()`, no TCP):
- Handshake round-trip
- DeltaBatch/Ack round-trip
- Interleaved messages
- Concurrent sends
- Oversized send/receive rejected
- Hello after handshake rejected
- Close during Receive

**Transport tests** (localhost TCP):
- Connect + Listen peer lifecycle
- Bidirectional DeltaBatch/Ack delivery
- Disconnect detection (OnPeerDisconnect)
- Duplicate peer rejected
- Close shuts everything down
- Send to unknown peer returns error
- Multiple peers

**Integration test** (Transport + CRDT replication):
- Two replicas with AWSet, DeltaStore, PeerTracker, Transport
- Mutate on one side, verify convergence
- Validates the Transport delivers bytes correctly (CRDT serialization
  already proven by `replication/e2e_test.go`)
