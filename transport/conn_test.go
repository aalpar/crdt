package transport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestPeerErrorFormat(t *testing.T) {
	c := qt.New(t)
	inner := errors.New("connection refused")
	err := &PeerError{PeerID: "alice", Op: "connect", Err: inner}
	c.Assert(err.Error(), qt.Equals, "peer alice: connect: connection refused")
	c.Assert(errors.Is(err, inner), qt.IsTrue)

	var pe *PeerError
	c.Assert(errors.As(err, &pe), qt.IsTrue)
	c.Assert(pe.PeerID, qt.Equals, "alice")
	c.Assert(pe.Op, qt.Equals, "connect")
}

func TestFrameRoundTrip(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer

	payload := []byte("hello world")
	c.Assert(writeFrame(&buf, tagDeltaBatch, payload), qt.IsNil)

	// Verify wire format: [4-byte length] [1-byte tag] [payload]
	wire := buf.Bytes()
	c.Assert(len(wire), qt.Equals, 5+len(payload))
	c.Assert(binary.BigEndian.Uint32(wire[:4]), qt.Equals, uint32(len(payload)))
	c.Assert(wire[4], qt.Equals, tagDeltaBatch)
	c.Assert(wire[5:], qt.DeepEquals, payload)

	// Read it back
	tag, got, err := readFrame(&buf, defaultMaxMsg)
	c.Assert(err, qt.IsNil)
	c.Assert(tag, qt.Equals, tagDeltaBatch)
	c.Assert(got, qt.DeepEquals, payload)
}

func TestFrameEmptyPayload(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	c.Assert(writeFrame(&buf, tagAck, nil), qt.IsNil)

	tag, got, err := readFrame(&buf, defaultMaxMsg)
	c.Assert(err, qt.IsNil)
	c.Assert(tag, qt.Equals, tagAck)
	c.Assert(len(got), qt.Equals, 0)
}

func TestFrameOversizedRead(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	c.Assert(writeFrame(&buf, tagDeltaBatch, make([]byte, 100)), qt.IsNil)

	_, _, err := readFrame(&buf, 50) // maxMsg < payload
	c.Assert(errors.Is(err, errFrameTooLarge), qt.IsTrue)
}

// newConnPair creates two connected Conns over a TCP loopback.
// TCP provides kernel buffering, so writes don't block until the peer reads.
func newConnPair(t *testing.T, id1, id2 string, opts ...ConnOption) (*Conn, *Conn) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	type result struct {
		c   *Conn
		err error
	}
	ch := make(chan result, 1)
	go func() {
		raw, err := ln.Accept()
		if err != nil {
			ch <- result{err: err}
			return
		}
		c, err := NewConn(raw, id2, opts...)
		ch <- result{c, err}
	}()

	raw1, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c1, err := NewConn(raw1, id1, opts...)
	if err != nil {
		t.Fatal(err)
	}
	r := <-ch
	if r.err != nil {
		t.Fatal(r.err)
	}

	t.Cleanup(func() {
		c1.Close()
		r.c.Close()
	})
	return c1, r.c
}

func TestConnHandshake(t *testing.T) {
	c := qt.New(t)
	a, b := newConnPair(t, "alice", "bob")
	c.Assert(a.PeerID(), qt.Equals, "bob")
	c.Assert(b.PeerID(), qt.Equals, "alice")
}

func TestConnDeltaBatchRoundTrip(t *testing.T) {
	c := qt.New(t)
	a, b := newConnPair(t, "alice", "bob")

	c.Assert(a.SendDeltaBatch([]byte("delta-payload")), qt.IsNil)

	typ, payload, err := b.Receive()
	c.Assert(err, qt.IsNil)
	c.Assert(typ, qt.Equals, MsgDeltaBatch)
	c.Assert(payload, qt.DeepEquals, []byte("delta-payload"))
}

func TestConnAckRoundTrip(t *testing.T) {
	c := qt.New(t)
	a, b := newConnPair(t, "alice", "bob")

	c.Assert(a.SendAck([]byte("ack-payload")), qt.IsNil)

	typ, payload, err := b.Receive()
	c.Assert(err, qt.IsNil)
	c.Assert(typ, qt.Equals, MsgAck)
	c.Assert(payload, qt.DeepEquals, []byte("ack-payload"))
}

func TestConnInterleavedMessages(t *testing.T) {
	c := qt.New(t)
	a, b := newConnPair(t, "alice", "bob")

	c.Assert(a.SendDeltaBatch([]byte("d1")), qt.IsNil)
	c.Assert(a.SendAck([]byte("a1")), qt.IsNil)
	c.Assert(a.SendDeltaBatch([]byte("d2")), qt.IsNil)

	typ, payload, _ := b.Receive()
	c.Assert(typ, qt.Equals, MsgDeltaBatch)
	c.Assert(payload, qt.DeepEquals, []byte("d1"))

	typ, payload, _ = b.Receive()
	c.Assert(typ, qt.Equals, MsgAck)
	c.Assert(payload, qt.DeepEquals, []byte("a1"))

	typ, payload, _ = b.Receive()
	c.Assert(typ, qt.Equals, MsgDeltaBatch)
	c.Assert(payload, qt.DeepEquals, []byte("d2"))
}

func TestConnHelloAfterHandshake(t *testing.T) {
	c := qt.New(t)
	a, b := newConnPair(t, "alice", "bob")

	// Inject a Hello frame directly — bypass SendDeltaBatch/SendAck.
	a.wmu.Lock()
	err := writeFrame(a.raw, tagHello, []byte("sneaky"))
	a.wmu.Unlock()
	c.Assert(err, qt.IsNil)

	_, _, err = b.Receive()
	c.Assert(errors.Is(err, errUnexpectedHello), qt.IsTrue)
}

func TestConnOversizedSend(t *testing.T) {
	c := qt.New(t)
	a, _ := newConnPair(t, "alice", "bob", WithMaxMessageSize(64))

	err := a.SendDeltaBatch(make([]byte, 65))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Matches, `.*exceeds max.*`)

	// Connection still works after rejected send.
	c.Assert(a.SendDeltaBatch([]byte("ok")), qt.IsNil)
}

func TestConnOversizedReceive(t *testing.T) {
	c := qt.New(t)
	// alice has large limit, bob has small limit.
	// Use newConnPair approach but with asymmetric options.
	// Since newConnPair applies the same opts to both sides,
	// we need to create the pair manually.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, qt.IsNil)
	defer ln.Close()

	ch := make(chan *Conn, 1)
	go func() {
		raw, _ := ln.Accept()
		conn, _ := NewConn(raw, "bob", WithMaxMessageSize(32))
		ch <- conn
	}()
	raw, err := net.Dial("tcp", ln.Addr().String())
	c.Assert(err, qt.IsNil)
	a, err := NewConn(raw, "alice", WithMaxMessageSize(1024))
	c.Assert(err, qt.IsNil)
	b := <-ch

	t.Cleanup(func() { a.Close(); b.Close() })

	// alice sends a payload that exceeds bob's limit.
	c.Assert(a.SendDeltaBatch(make([]byte, 64)), qt.IsNil)
	_, _, err = b.Receive()
	c.Assert(errors.Is(err, errFrameTooLarge), qt.IsTrue)
}

func TestConnConcurrentSends(t *testing.T) {
	c := qt.New(t)
	a, b := newConnPair(t, "alice", "bob")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		a.SendDeltaBatch([]byte("delta"))
	}()
	go func() {
		defer wg.Done()
		a.SendAck([]byte("ack"))
	}()
	wg.Wait()

	msgs := make(map[MessageType][]byte)
	for range 2 {
		typ, payload, err := b.Receive()
		c.Assert(err, qt.IsNil)
		msgs[typ] = payload
	}
	c.Assert(msgs[MsgDeltaBatch], qt.DeepEquals, []byte("delta"))
	c.Assert(msgs[MsgAck], qt.DeepEquals, []byte("ack"))
}

func TestConnCloseDuringReceive(t *testing.T) {
	c := qt.New(t)
	a, _ := newConnPair(t, "alice", "bob")

	errCh := make(chan error, 1)
	go func() {
		_, _, err := a.Receive()
		errCh <- err
	}()

	a.Close()
	c.Assert(<-errCh, qt.IsNotNil)
}
