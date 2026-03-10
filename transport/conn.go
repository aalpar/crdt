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
func writeFrame(w io.Writer, tag uint8, payload []byte) error {
	var hdr [5]byte
	binary.BigEndian.PutUint32(hdr[:4], uint32(len(payload)))
	hdr[4] = tag
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := w.Write(payload)
		return err
	}
	return nil
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
