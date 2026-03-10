package transport

import (
	"bytes"
	"encoding/binary"
	"errors"
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
