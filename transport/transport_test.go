package transport

import (
	"errors"
	"net"
	"testing"

	qt "github.com/frankban/quicktest"
)

type disconnectEvent struct {
	PeerID string
	Err    error
}

type peerMsg struct {
	PeerID  string
	Payload []byte
}

type testHandler struct {
	connect    chan string
	disconnect chan disconnectEvent
	delta      chan peerMsg
	ack        chan peerMsg
}

func newTestHandler() *testHandler {
	return &testHandler{
		connect:    make(chan string, 10),
		disconnect: make(chan disconnectEvent, 10),
		delta:      make(chan peerMsg, 10),
		ack:        make(chan peerMsg, 10),
	}
}

func (h *testHandler) OnPeerConnect(peerID string)                { h.connect <- peerID }
func (h *testHandler) OnPeerDisconnect(peerID string, err error)  { h.disconnect <- disconnectEvent{peerID, err} }
func (h *testHandler) OnDeltaBatch(peerID string, payload []byte) { h.delta <- peerMsg{peerID, append([]byte(nil), payload...)} }
func (h *testHandler) OnAck(peerID string, payload []byte)        { h.ack <- peerMsg{peerID, append([]byte(nil), payload...)} }

// newTransportPair creates two transports connected over localhost TCP.
// t1 listens, t2 dials. Waits for both sides to confirm the connection.
func newTransportPair(t *testing.T, id1, id2 string) (*Transport, *testHandler, *Transport, *testHandler) {
	t.Helper()
	h1 := newTestHandler()
	h2 := newTestHandler()
	t1 := New(id1, h1)
	t2 := New(id2, h2)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go t1.Listen(ln)

	if err := t2.Connect(ln.Addr().String()); err != nil {
		t.Fatal(err)
	}

	// Wait for both sides to register the peer.
	<-h1.connect
	<-h2.connect

	t.Cleanup(func() {
		t1.Close()
		t2.Close()
	})
	return t1, h1, t2, h2
}

func TestTransportConnectListen(t *testing.T) {
	c := qt.New(t)
	t1, _, t2, _ := newTransportPair(t, "alice", "bob")

	c.Assert(t1.Peers(), qt.HasLen, 1)
	c.Assert(t1.Peers()[0], qt.Equals, "bob")
	c.Assert(t2.Peers(), qt.HasLen, 1)
	c.Assert(t2.Peers()[0], qt.Equals, "alice")
}

func TestTransportDisconnect(t *testing.T) {
	c := qt.New(t)
	_, h1, t2, _ := newTransportPair(t, "alice", "bob")

	t2.Close()
	ev := <-h1.disconnect
	c.Assert(ev.PeerID, qt.Equals, "bob")
	c.Assert(ev.Err, qt.IsNotNil)
}

func TestTransportCloseShutdown(t *testing.T) {
	c := qt.New(t)
	t1, _, _, h2 := newTransportPair(t, "alice", "bob")

	t1.Close()
	ev := <-h2.disconnect
	c.Assert(ev.PeerID, qt.Equals, "alice")
	c.Assert(t1.Peers(), qt.HasLen, 0)
}

func TestTransportDeltaBatchDelivery(t *testing.T) {
	c := qt.New(t)
	t1, _, _, h2 := newTransportPair(t, "alice", "bob")

	c.Assert(t1.SendDeltaBatch("bob", []byte("delta-1")), qt.IsNil)
	msg := <-h2.delta
	c.Assert(msg.PeerID, qt.Equals, "alice")
	c.Assert(msg.Payload, qt.DeepEquals, []byte("delta-1"))
}

func TestTransportAckDelivery(t *testing.T) {
	c := qt.New(t)
	t1, _, _, h2 := newTransportPair(t, "alice", "bob")

	c.Assert(t1.SendAck("bob", []byte("ack-1")), qt.IsNil)
	msg := <-h2.ack
	c.Assert(msg.PeerID, qt.Equals, "alice")
	c.Assert(msg.Payload, qt.DeepEquals, []byte("ack-1"))
}

func TestTransportBidirectional(t *testing.T) {
	c := qt.New(t)
	t1, h1, t2, h2 := newTransportPair(t, "alice", "bob")

	c.Assert(t1.SendDeltaBatch("bob", []byte("from-alice")), qt.IsNil)
	c.Assert(t2.SendDeltaBatch("alice", []byte("from-bob")), qt.IsNil)

	msg := <-h2.delta
	c.Assert(msg.Payload, qt.DeepEquals, []byte("from-alice"))

	msg = <-h1.delta
	c.Assert(msg.Payload, qt.DeepEquals, []byte("from-bob"))
}

func TestTransportSendToUnknownPeer(t *testing.T) {
	c := qt.New(t)
	t1, _, _, _ := newTransportPair(t, "alice", "bob")

	err := t1.SendDeltaBatch("charlie", []byte("hello"))
	c.Assert(err, qt.IsNotNil)
	var pe *PeerError
	c.Assert(errors.As(err, &pe), qt.IsTrue)
	c.Assert(pe.PeerID, qt.Equals, "charlie")
}

func TestTransportDuplicatePeer(t *testing.T) {
	c := qt.New(t)
	h1 := newTestHandler()
	t1 := New("alice", h1)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, qt.IsNil)
	go t1.Listen(ln)

	h2 := newTestHandler()
	t2 := New("bob", h2)
	c.Assert(t2.Connect(ln.Addr().String()), qt.IsNil)
	<-h1.connect
	<-h2.connect

	// Second "bob" tries to connect — should be rejected.
	h3 := newTestHandler()
	t3 := New("bob", h3)
	_ = t3.Connect(ln.Addr().String())
	// The listener side (t1) sees the duplicate peerID and closes the
	// new connection. t1 should still have exactly one "bob".
	c.Assert(t1.Peers(), qt.HasLen, 1)

	t.Cleanup(func() { t1.Close(); t2.Close(); t3.Close() })
}

func TestTransportMultiplePeers(t *testing.T) {
	c := qt.New(t)
	h1 := newTestHandler()
	t1 := New("alice", h1)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, qt.IsNil)
	go t1.Listen(ln)

	t2 := New("bob", newTestHandler())
	c.Assert(t2.Connect(ln.Addr().String()), qt.IsNil)
	<-h1.connect

	t3 := New("charlie", newTestHandler())
	c.Assert(t3.Connect(ln.Addr().String()), qt.IsNil)
	<-h1.connect

	c.Assert(t1.Peers(), qt.HasLen, 2)

	// Send to each peer independently.
	c.Assert(t1.SendDeltaBatch("bob", []byte("for-bob")), qt.IsNil)
	c.Assert(t1.SendDeltaBatch("charlie", []byte("for-charlie")), qt.IsNil)

	t.Cleanup(func() { t1.Close(); t2.Close(); t3.Close() })
}
