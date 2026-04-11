// Package websocket manages WebSocket connections from Android proxy nodes.
package websocket

import (
	"fmt"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
	pb "github.com/gofullthrottle/proxy-coin/backend/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

const (
	// sendChannelBuffer is the number of outbound messages buffered per connection.
	sendChannelBuffer = 256

	// writeDeadline is the timeout for a single WebSocket write operation.
	writeDeadline = 10 * time.Second
)

// PendingRequest represents a proxy request waiting for a response from the node.
type PendingRequest struct {
	RequestID  string
	ResponseCh chan *pb.WebSocketMessage
	CreatedAt  time.Time
}

// Connection wraps a WebSocket connection to an Android proxy node.
// It is safe for concurrent use; the write path is serialized through SendCh.
type Connection struct {
	// NodeID is set after the REGISTER handshake completes.
	NodeID string

	// Conn is the underlying gorilla WebSocket connection.
	Conn *ws.Conn

	// SendCh accepts serialized protobuf frames for the write pump.
	SendCh chan []byte

	// ActiveReqs maps requestID → *PendingRequest for in-flight proxy requests.
	ActiveReqs sync.Map

	mu       sync.Mutex
	lastPing time.Time
	closed   bool
	closeCh  chan struct{}
}

// NewConnection initialises a Connection for a freshly upgraded WebSocket.
// Call WritePump in a goroutine immediately after creation.
func NewConnection(conn *ws.Conn) *Connection {
	return &Connection{
		Conn:    conn,
		SendCh:  make(chan []byte, sendChannelBuffer),
		closeCh: make(chan struct{}),
	}
}

// SetNodeID assigns the node identity once the REGISTER message has been
// processed.
func (c *Connection) SetNodeID(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.NodeID = nodeID
}

// Send serialises msg as a binary protobuf frame and enqueues it for
// delivery. It returns an error if the connection is closed or the send
// buffer is full.
func (c *Connection) Send(msg *pb.WebSocketMessage) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("websocket: marshal message: %w", err)
	}
	return c.SendRaw(data)
}

// SendRaw enqueues a pre-serialised binary frame for delivery.
func (c *Connection) SendRaw(data []byte) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrConnectionClosed
	}
	c.mu.Unlock()

	select {
	case c.SendCh <- data:
		return nil
	default:
		return fmt.Errorf("websocket: send buffer full for node %s", c.NodeID)
	}
}

// WritePump reads from SendCh and writes binary frames to the WebSocket.
// Run this in a dedicated goroutine. It exits when SendCh is closed or
// a write error occurs.
func (c *Connection) WritePump() {
	defer c.Close()

	for {
		select {
		case data, ok := <-c.SendCh:
			if !ok {
				// Channel closed — send a clean close frame.
				_ = c.Conn.WriteMessage(ws.CloseMessage,
					ws.FormatCloseMessage(ws.CloseNormalClosure, ""))
				return
			}
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
				return
			}
			if err := c.Conn.WriteMessage(ws.BinaryMessage, data); err != nil {
				return
			}
		case <-c.closeCh:
			return
		}
	}
}

// Close tears down the connection. It is idempotent.
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}
	c.closed = true
	close(c.closeCh)
	_ = c.Conn.Close()
}

// IsClosed reports whether the connection has been closed.
func (c *Connection) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// AddPendingRequest registers an in-flight proxy request and returns a handle
// that will receive the response (or time out). The caller must call
// RemovePendingRequest when done.
func (c *Connection) AddPendingRequest(requestID string, timeout time.Duration) *PendingRequest {
	req := &PendingRequest{
		RequestID:  requestID,
		ResponseCh: make(chan *pb.WebSocketMessage, 1),
		CreatedAt:  time.Now(),
	}
	c.ActiveReqs.Store(requestID, req)

	// Auto-expire: remove after timeout so ActiveReqs does not leak.
	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			c.RemovePendingRequest(requestID)
		case <-c.closeCh:
		}
	}()

	return req
}

// CompletePendingRequest delivers msg to the waiting goroutine for requestID.
// It is a no-op if the request has already been removed or timed out.
func (c *Connection) CompletePendingRequest(requestID string, msg *pb.WebSocketMessage) {
	val, ok := c.ActiveReqs.Load(requestID)
	if !ok {
		return
	}
	req := val.(*PendingRequest)
	// Non-blocking send — the channel is buffered with capacity 1.
	select {
	case req.ResponseCh <- msg:
	default:
	}
}

// RemovePendingRequest removes the pending request from ActiveReqs.
func (c *Connection) RemovePendingRequest(requestID string) {
	c.ActiveReqs.Delete(requestID)
}
