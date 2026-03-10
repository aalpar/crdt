package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	defaultMaxMsg = 16 << 20 // 16 MiB

	tagHello      uint8 = 0x00
	tagDeltaBatch uint8 = 0x01
	tagAck        uint8 = 0x02
)

// MessageType identifies the kind of message received from a peer.
type MessageType uint8

const (
	// MsgDeltaBatch carries a caller-encoded delta batch.
	MsgDeltaBatch MessageType = MessageType(tagDeltaBatch)
	// MsgAck carries a caller-encoded CausalContext acknowledgment.
	MsgAck MessageType = MessageType(tagAck)
)

var (
	errFrameTooLarge   = errors.New("frame exceeds maximum message size")
	errUnexpectedHello = errors.New("received hello after handshake")
)

// PeerError records a failed operation on a peer connection.
type PeerError struct {
	PeerID string
	Op     string // "send", "receive", "connect"
	Err    error
}

func (e *PeerError) Error() string {
	return fmt.Sprintf("peer %s: %s: %v", e.PeerID, e.Op, e.Err)
}

func (e *PeerError) Unwrap() error { return e.Err }

// ConnOption configures a Conn.
type ConnOption func(*Conn)

// WithMaxMessageSize sets the maximum payload size in bytes.
// Messages exceeding this limit are rejected on both send and receive.
func WithMaxMessageSize(n uint32) ConnOption {
	return func(c *Conn) { c.maxMsg = n }
}

// Conn is a framed, type-tagged message connection between two peers.
// Sends are safe for concurrent use. Receive must be called from a
// single goroutine.
type Conn struct {
	raw    net.Conn
	peerID string
	maxMsg uint32
	wmu    sync.Mutex // serializes writes
}

// writeFrame writes a length-prefixed, type-tagged frame.
// Wire format: [uint32 big-endian payload length] [uint8 tag] [payload].
// Header and payload are written in a single call to avoid blocking
// on synchronous transports (e.g. net.Pipe).
func writeFrame(w io.Writer, tag uint8, payload []byte) error {
	buf := make([]byte, 5+len(payload))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(payload)))
	buf[4] = tag
	copy(buf[5:], payload)
	_, err := w.Write(buf)
	return err
}

// readFrame reads one length-prefixed, type-tagged frame.
// Returns an error if the payload length exceeds maxMsg.
func readFrame(r io.Reader, maxMsg uint32) (uint8, []byte, error) {
	var hdr [5]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(hdr[:4])
	tag := hdr[4]
	if length > maxMsg {
		return 0, nil, errFrameTooLarge
	}
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
	}
	return tag, payload, nil
}

// NewConn performs a handshake over raw and returns a framed Conn.
// Both sides send their ID concurrently to avoid deadlock.
func NewConn(raw net.Conn, localID string, opts ...ConnOption) (*Conn, error) {
	c := &Conn{raw: raw, maxMsg: defaultMaxMsg}
	for _, o := range opts {
		o(c)
	}

	// Read peer's Hello in a goroutine while we send ours.
	type helloResult struct {
		peerID string
		err    error
	}
	ch := make(chan helloResult, 1)
	go func() {
		tag, payload, err := readFrame(raw, c.maxMsg)
		if err != nil {
			ch <- helloResult{err: err}
			return
		}
		if tag != tagHello {
			ch <- helloResult{err: fmt.Errorf("handshake: expected hello (tag 0x00), got 0x%02x", tag)}
			return
		}
		ch <- helloResult{peerID: string(payload)}
	}()

	if err := writeFrame(raw, tagHello, []byte(localID)); err != nil {
		raw.Close()
		return nil, err
	}

	r := <-ch
	if r.err != nil {
		raw.Close()
		return nil, r.err
	}
	c.peerID = r.peerID
	return c, nil
}

// PeerID returns the remote peer's replica ID, received during handshake.
func (c *Conn) PeerID() string { return c.peerID }

// SendDeltaBatch sends a DeltaBatch message with the given payload.
// Safe for concurrent use.
func (c *Conn) SendDeltaBatch(payload []byte) error {
	if len(payload) > int(c.maxMsg) {
		return fmt.Errorf("payload length %d exceeds max %d", len(payload), c.maxMsg)
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return writeFrame(c.raw, tagDeltaBatch, payload)
}

// SendAck sends an Ack message with the given payload.
// Safe for concurrent use.
func (c *Conn) SendAck(payload []byte) error {
	if len(payload) > int(c.maxMsg) {
		return fmt.Errorf("payload length %d exceeds max %d", len(payload), c.maxMsg)
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return writeFrame(c.raw, tagAck, payload)
}

// Receive reads the next message from the connection.
// Must be called from a single goroutine (the read loop).
// Returns errUnexpectedHello if a Hello frame arrives after handshake.
func (c *Conn) Receive() (MessageType, []byte, error) {
	tag, payload, err := readFrame(c.raw, c.maxMsg)
	if err != nil {
		return 0, nil, err
	}
	if tag == tagHello {
		return 0, nil, errUnexpectedHello
	}
	return MessageType(tag), payload, nil
}

// Close closes the underlying network connection.
func (c *Conn) Close() error { return c.raw.Close() }
