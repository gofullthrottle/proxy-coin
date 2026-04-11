// Package websocket manages WebSocket connections from Android proxy nodes.
package websocket

import (
	"errors"
	"sync"
)

var (
	// ErrConnectionNotFound is returned when a node ID has no active connection.
	ErrConnectionNotFound = errors.New("connection not found")

	// ErrConnectionClosed is returned when an operation targets a closed connection.
	ErrConnectionClosed = errors.New("connection is closed")

	// ErrMaxConcurrent is returned when a node has reached its concurrent-request limit.
	ErrMaxConcurrent = errors.New("node at max concurrent requests")
)

// Pool manages the set of active WebSocket connections indexed by node ID.
// All methods are safe for concurrent use.
type Pool struct {
	mu          sync.RWMutex
	connections map[string]*Connection // nodeID → *Connection
}

// NewPool creates an empty Pool.
func NewPool() *Pool {
	return &Pool{
		connections: make(map[string]*Connection),
	}
}

// Add registers conn under nodeID, replacing any prior connection for that node.
func (p *Pool) Add(nodeID string, conn *Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connections[nodeID] = conn
}

// Remove removes the connection for nodeID from the pool.
func (p *Pool) Remove(nodeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.connections, nodeID)
}

// Get retrieves the active connection for nodeID without removing it.
// Returns false if no connection exists.
func (p *Pool) Get(nodeID string) (*Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	conn, ok := p.connections[nodeID]
	return conn, ok
}

// Count returns the number of active connections in the pool.
func (p *Pool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections)
}

// GetAll returns a snapshot of all active connections.
func (p *Pool) GetAll() []*Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		out = append(out, conn)
	}
	return out
}

// CloseAll closes every connection in the pool and empties it.
func (p *Pool) CloseAll() {
	p.mu.Lock()
	conns := make([]*Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		conns = append(conns, conn)
	}
	p.connections = make(map[string]*Connection)
	p.mu.Unlock()

	for _, conn := range conns {
		conn.Close()
	}
}
