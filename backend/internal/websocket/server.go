// Package websocket manages WebSocket connections from Android proxy nodes.
package websocket

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	pb "github.com/gofullthrottle/proxy-coin/backend/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development; restrict to known origins in production.
		return true
	},
}

// MessageHandler processes a single incoming WebSocket message from a node.
type MessageHandler func(conn *Connection, msg *pb.WebSocketMessage)

// Server manages WebSocket connections from Android proxy nodes.
// Connections are stored in a sync.Map keyed by node ID; a temporary
// pre-registration slot uses the remote address as key until the REGISTER
// message has been handled.
type Server struct {
	// connections maps nodeID → *Connection after registration.
	connections sync.Map

	handler MessageHandler

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
}

// NewServer creates a Server that dispatches incoming messages to handler.
// heartbeatInterval controls how often the server expects a heartbeat from
// a node; heartbeatTimeout is the read deadline applied to each read call.
func NewServer(handler MessageHandler, heartbeatInterval, heartbeatTimeout time.Duration) *Server {
	return &Server{
		handler:           handler,
		heartbeatInterval: heartbeatInterval,
		heartbeatTimeout:  heartbeatTimeout,
	}
}

// ServeHTTP upgrades the HTTP request to a WebSocket, creates a Connection,
// and begins the read/write pumps. The connection is stored in the sync.Map
// temporarily by remote address until the REGISTER message arrives and
// RegisterConnection is called with the real node ID.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket: upgrade failed from %s: %v", r.RemoteAddr, err)
		return
	}

	conn := NewConnection(wsConn)
	log.Printf("websocket: new connection from %s", r.RemoteAddr)

	// Store temporarily by remote address so the handler can call
	// RegisterConnection once the nodeID is known from REGISTER.
	s.connections.Store(r.RemoteAddr, conn)

	// Start the write pump in a separate goroutine.
	go conn.WritePump()

	// Block in the read loop until the connection is closed.
	s.HandleConnection(conn)

	// Clean up: remove by remote address (may already be replaced by nodeID).
	s.connections.Delete(r.RemoteAddr)
	if conn.NodeID != "" {
		s.connections.Delete(conn.NodeID)
	}
	log.Printf("websocket: connection closed for node=%q addr=%s", conn.NodeID, r.RemoteAddr)
}

// HandleConnection runs the read loop for conn. It reads binary protobuf
// frames, deserialises them as WebSocketMessage, and dispatches each to the
// registered MessageHandler. It returns when the connection is closed or a
// read error occurs.
func (s *Server) HandleConnection(conn *Connection) {
	for {
		// Extend the read deadline on every iteration so the connection is
		// considered alive as long as messages keep arriving.
		if err := conn.Conn.SetReadDeadline(time.Now().Add(s.heartbeatTimeout)); err != nil {
			log.Printf("websocket: SetReadDeadline: %v", err)
			break
		}

		msgType, data, err := conn.Conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived) {
				log.Printf("websocket: read error for node=%q: %v", conn.NodeID, err)
			}
			break
		}

		if msgType != websocket.BinaryMessage {
			log.Printf("websocket: unexpected message type %d from node=%q", msgType, conn.NodeID)
			continue
		}

		msg := &pb.WebSocketMessage{}
		if err := proto.Unmarshal(data, msg); err != nil {
			log.Printf("websocket: unmarshal error from node=%q: %v", conn.NodeID, err)
			continue
		}

		s.handler(conn, msg)
	}

	conn.Close()
}

// GetConnection looks up an active connection by node ID.
func (s *Server) GetConnection(nodeID string) (*Connection, bool) {
	val, ok := s.connections.Load(nodeID)
	if !ok {
		return nil, false
	}
	return val.(*Connection), true
}

// RegisterConnection stores conn under nodeID. Call this from the message
// handler after a successful REGISTER exchange.
func (s *Server) RegisterConnection(nodeID string, conn *Connection) {
	conn.SetNodeID(nodeID)
	s.connections.Store(nodeID, conn)
}

// RemoveConnection removes the connection registered under nodeID.
func (s *Server) RemoveConnection(nodeID string) {
	s.connections.Delete(nodeID)
}

// Broadcast sends msg to every currently registered connection. Errors for
// individual connections are logged but do not interrupt delivery to others.
func (s *Server) Broadcast(msg *pb.WebSocketMessage) {
	s.connections.Range(func(_, val any) bool {
		conn, ok := val.(*Connection)
		if !ok || conn.IsClosed() {
			return true
		}
		if err := conn.Send(msg); err != nil {
			log.Printf("websocket: broadcast send error for node=%q: %v", conn.NodeID, err)
		}
		return true
	})
}

// ConnectionCount returns the number of entries currently in the connection map.
// This includes the temporary pre-registration slots, so it may be slightly
// higher than the number of fully registered nodes.
func (s *Server) ConnectionCount() int {
	count := 0
	s.connections.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// Shutdown closes all active connections gracefully. It waits until ctx is
// done or all connections have been closed, whichever comes first.
func (s *Server) Shutdown(ctx context.Context) {
	// Collect connections to close.
	var conns []*Connection
	s.connections.Range(func(_, val any) bool {
		if conn, ok := val.(*Connection); ok {
			conns = append(conns, conn)
		}
		return true
	})

	var wg sync.WaitGroup
	for _, conn := range conns {
		wg.Add(1)
		go func(c *Connection) {
			defer wg.Done()
			c.Close()
		}(conn)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("websocket: shutdown complete (%d connections closed)", len(conns))
	case <-ctx.Done():
		log.Printf("websocket: shutdown timed out, forcing close")
	}
}
