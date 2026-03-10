package transport

import (
	"errors"
	"net"
	"sync"
)

var errPeerConnected = errors.New("peer already connected")

// Handler receives events from a Transport. Callbacks run on the
// connection's read goroutine — a slow handler backpressures that
// peer's reads. Decouple with channels if async processing is needed.
type Handler interface {
	OnPeerConnect(peerID string)
	OnPeerDisconnect(peerID string, err error)
	OnDeltaBatch(peerID string, payload []byte)
	OnAck(peerID string, payload []byte)
}

// Transport manages a pool of peer connections and dispatches incoming
// messages to a Handler. Safe for concurrent use.
type Transport struct {
	localID  string
	handler  Handler
	connOpts []ConnOption

	mu    sync.Mutex
	ln    net.Listener
	peers map[string]*Conn

	wg       sync.WaitGroup
	done     chan struct{}
	closeOnce sync.Once
}

// New creates a Transport with the given local replica ID and handler.
// ConnOptions are forwarded to each Conn created by Connect or Listen.
func New(localID string, handler Handler, opts ...ConnOption) *Transport {
	return &Transport{
		localID:  localID,
		handler:  handler,
		connOpts: opts,
		peers:    make(map[string]*Conn),
		done:     make(chan struct{}),
	}
}

// Listen accepts connections on ln until Close is called or ln fails.
// Blocks like http.Serve — run in a goroutine.
func (t *Transport) Listen(ln net.Listener) error {
	t.mu.Lock()
	t.ln = ln
	t.mu.Unlock()

	for {
		raw, err := ln.Accept()
		if err != nil {
			select {
			case <-t.done:
				return nil
			default:
				return err
			}
		}
		t.wg.Add(1)
		go t.handleAccept(raw)
	}
}

func (t *Transport) handleAccept(raw net.Conn) {
	defer t.wg.Done()

	c, err := NewConn(raw, t.localID, t.connOpts...)
	if err != nil {
		return
	}
	if !t.addPeer(c) {
		c.Close()
		return
	}
	t.handler.OnPeerConnect(c.PeerID())
	t.readLoop(c)
}

// Connect dials addr, performs a handshake, and starts receiving
// messages from the peer. Returns a PeerError if the peer is already
// connected.
func (t *Transport) Connect(addr string) error {
	raw, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	c, err := NewConn(raw, t.localID, t.connOpts...)
	if err != nil {
		return err
	}
	if !t.addPeer(c) {
		c.Close()
		return &PeerError{PeerID: c.PeerID(), Op: "connect", Err: errPeerConnected}
	}
	t.handler.OnPeerConnect(c.PeerID())
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.readLoop(c)
	}()
	return nil
}

func (t *Transport) addPeer(c *Conn) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.peers[c.PeerID()]; exists {
		return false
	}
	t.peers[c.PeerID()] = c
	return true
}

func (t *Transport) readLoop(c *Conn) {
	peerID := c.PeerID()
	for {
		typ, payload, err := c.Receive()
		if err != nil {
			t.mu.Lock()
			delete(t.peers, peerID)
			t.mu.Unlock()
			c.Close()
			t.handler.OnPeerDisconnect(peerID, err)
			return
		}
		switch typ {
		case MsgDeltaBatch:
			t.handler.OnDeltaBatch(peerID, payload)
		case MsgAck:
			t.handler.OnAck(peerID, payload)
		}
	}
}

// SendDeltaBatch sends a DeltaBatch message to the named peer.
func (t *Transport) SendDeltaBatch(peerID string, payload []byte) error {
	c := t.getPeer(peerID)
	if c == nil {
		return &PeerError{PeerID: peerID, Op: "send", Err: errors.New("peer not connected")}
	}
	return c.SendDeltaBatch(payload)
}

// SendAck sends an Ack message to the named peer.
func (t *Transport) SendAck(peerID string, payload []byte) error {
	c := t.getPeer(peerID)
	if c == nil {
		return &PeerError{PeerID: peerID, Op: "send", Err: errors.New("peer not connected")}
	}
	return c.SendAck(payload)
}

func (t *Transport) getPeer(peerID string) *Conn {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.peers[peerID]
}

// Peers returns a snapshot of connected peer IDs.
func (t *Transport) Peers() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	ids := make([]string, 0, len(t.peers))
	for id := range t.peers {
		ids = append(ids, id)
	}
	return ids
}

// Close shuts down the transport: closes the listener (if any), closes
// all peer connections, and waits for all goroutines to exit.
// Safe to call multiple times.
func (t *Transport) Close() error {
	t.closeOnce.Do(func() {
		close(t.done)
		t.mu.Lock()
		if t.ln != nil {
			t.ln.Close()
		}
		for _, c := range t.peers {
			c.Close()
		}
		t.mu.Unlock()
	})
	t.wg.Wait()
	return nil
}
