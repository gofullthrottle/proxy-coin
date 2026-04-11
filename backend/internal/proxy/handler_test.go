package proxy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/gofullthrottle/proxy-coin/backend/internal/node"
	"github.com/gofullthrottle/proxy-coin/backend/internal/proxy"
	ws "github.com/gofullthrottle/proxy-coin/backend/internal/websocket"
	pb "github.com/gofullthrottle/proxy-coin/backend/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------------------------
// Mock selector: always returns a fixed node.
// ---------------------------------------------------------------------------

type mockSelector struct {
	fixedNode *node.Node
}

func (m *mockSelector) SelectNode(_ context.Context, _ node.NodeFilter) (*node.Node, error) {
	return m.fixedNode, nil
}

// ---------------------------------------------------------------------------
// Mock registry: returns the same node for any GetByID call.
// ---------------------------------------------------------------------------

type mockRegistry struct {
	fixedNode *node.Node
}

func (m *mockRegistry) GetByID(_ context.Context, _ string) (*node.Node, error) {
	return m.fixedNode, nil
}

// ---------------------------------------------------------------------------
// Mock SessionStore: in-memory stubs for the calls Handler makes.
// ---------------------------------------------------------------------------

type mockSessionStore struct {
	mu       sync.Mutex
	sessions map[string]string
	counters map[string]int64
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]string),
		counters: make(map[string]int64),
	}
}

func (m *mockSessionStore) GetSessionBinding(_ context.Context, sessionID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[sessionID], nil
}

func (m *mockSessionStore) SetSessionBinding(_ context.Context, sessionID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = nodeID
	return nil
}

func (m *mockSessionStore) IncrActiveRequests(_ context.Context, nodeID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[nodeID]++
	return m.counters[nodeID], nil
}

func (m *mockSessionStore) DecrActiveRequests(_ context.Context, nodeID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters[nodeID] > 0 {
		m.counters[nodeID]--
	}
	return m.counters[nodeID], nil
}

// ---------------------------------------------------------------------------
// Helpers to build mock WebSocket pairs.
// ---------------------------------------------------------------------------

// upgrader is the server-side WebSocket upgrader used in tests.
var upgrader = gws.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

// newConnPair returns a pair of connected gorilla WebSocket connections.
//
// srvConn is the "Android node" side.
// clientConn is the backend ws.Connection (used by Handler).
func newConnPair(t *testing.T) (srvConn *gws.Conn, clientConn *ws.Connection) {
	t.Helper()

	ready := make(chan *gws.Conn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("ws upgrade: %v", err)
			return
		}
		ready <- c
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	rawClient, _, err := gws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}

	conn := ws.NewConnection(rawClient)
	conn.SetNodeID("test-node-id")
	go conn.WritePump()

	srvConn = <-ready
	return srvConn, conn
}

// ---------------------------------------------------------------------------
// Integration test: full request/response round-trip through Handler.
// ---------------------------------------------------------------------------

// TestHandler_HandleRequest_Success simulates an Android node that receives a
// ProxyRequest and streams ResponseStart + ResponseChunk + ResponseEnd back.
// The Handler must return a correctly populated Response.
func TestHandler_HandleRequest_Success(t *testing.T) {
	const testNodeID = "test-node-id"
	const testBody = "Hello from the proxied origin!"

	srvConn, clientConn := newConnPair(t)

	fixedNode := &node.Node{
		ID:      testNodeID,
		Country: "US",
		Region:  "CA",
		IP:      "1.2.3.4",
		Status:  "active",
	}

	pool := ws.NewPool()
	pool.Add(testNodeID, clientConn)

	handler := proxy.NewHandlerWithInterfaces(
		&mockSelector{fixedNode: fixedNode},
		&mockRegistry{fixedNode: fixedNode},
		pool,
		newMockSessionStore(),
		3*time.Second,
	)

	// Simulated Android node: read the request and send streaming response.
	go func() {
		_, data, err := srvConn.ReadMessage()
		if err != nil {
			return
		}
		var reqMsg pb.WebSocketMessage
		if err := proto.Unmarshal(data, &reqMsg); err != nil {
			return
		}
		proxyReq := reqMsg.GetProxyRequest()
		if proxyReq == nil {
			return
		}
		requestID := proxyReq.GetRequestId()

		send := func(msg *pb.WebSocketMessage) {
			d, _ := proto.Marshal(msg)
			_ = srvConn.WriteMessage(gws.BinaryMessage, d)
		}

		send(&pb.WebSocketMessage{Payload: &pb.WebSocketMessage_ProxyResponseStart{
			ProxyResponseStart: &pb.ProxyResponseStart{
				RequestId:  requestID,
				StatusCode: 200,
				Headers:    []*pb.Header{{Key: "Content-Type", Value: "text/plain"}},
			},
		}})
		send(&pb.WebSocketMessage{Payload: &pb.WebSocketMessage_ProxyResponseChunk{
			ProxyResponseChunk: &pb.ProxyResponseChunk{
				RequestId:  requestID,
				ChunkIndex: 0,
				Data:       []byte(testBody),
			},
		}})
		send(&pb.WebSocketMessage{Payload: &pb.WebSocketMessage_ProxyResponseEnd{
			ProxyResponseEnd: &pb.ProxyResponseEnd{
				RequestId:  requestID,
				TotalBytes: int64(len(testBody)),
				LatencyMs:  42,
				Success:    true,
			},
		}})
	}()

	// Client-side read pump: route inbound frames to the pending request channel.
	// In production this is done by the WebSocket server's message dispatcher.
	go func() {
		for {
			_, data, err := clientConn.Conn.ReadMessage()
			if err != nil {
				return
			}
			var msg pb.WebSocketMessage
			if err := proto.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch p := msg.Payload.(type) {
			case *pb.WebSocketMessage_ProxyResponseEnd:
				clientConn.CompletePendingRequest(p.ProxyResponseEnd.GetRequestId(), &msg)
			}
		}
	}()

	resp, err := handler.HandleRequest(context.Background(), proxy.Request{
		URL:        "http://example.com/",
		Method:     "GET",
		CustomerID: "cust-1",
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil Response")
	}
	if resp.NodeIP != fixedNode.IP {
		t.Errorf("NodeIP = %q, want %q", resp.NodeIP, fixedNode.IP)
	}
	if resp.BytesTransferred != int64(len(testBody)) {
		t.Errorf("BytesTransferred = %d, want %d", resp.BytesTransferred, len(testBody))
	}
	if resp.ProxyMs != 42 {
		t.Errorf("ProxyMs = %d, want 42", resp.ProxyMs)
	}
}

// TestHandler_HandleRequest_Timeout verifies that HandleRequest returns
// ErrRequestTimeout when the node never sends a response.
func TestHandler_HandleRequest_Timeout(t *testing.T) {
	const testNodeID = "timeout-node"

	_, clientConn := newConnPair(t)

	fixedNode := &node.Node{ID: testNodeID, Status: "active"}
	pool := ws.NewPool()
	pool.Add(testNodeID, clientConn)

	handler := proxy.NewHandlerWithInterfaces(
		&mockSelector{fixedNode: fixedNode},
		&mockRegistry{fixedNode: fixedNode},
		pool,
		newMockSessionStore(),
		50*time.Millisecond, // very short timeout to trigger quickly
	)

	// Start a reader so the WritePump does not stall on the server side.
	go func() {
		for {
			if _, _, err := clientConn.Conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	_, err := handler.HandleRequest(context.Background(), proxy.Request{
		URL:    "http://example.com/",
		Method: "GET",
	})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") && err != proxy.ErrRequestTimeout {
		t.Errorf("unexpected error type: %v", err)
	}
}
