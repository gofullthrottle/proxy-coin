// Package proxy handles HTTP/HTTPS proxy request routing and forwarding.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gofullthrottle/proxy-coin/backend/internal/node"
	ws "github.com/gofullthrottle/proxy-coin/backend/internal/websocket"
	pb "github.com/gofullthrottle/proxy-coin/backend/pkg/protocol"
)

// ErrNoAvailableNodes is returned when no node can be selected for a request.
var ErrNoAvailableNodes = errors.New("proxy: no available nodes matching request criteria")

// ErrRequestTimeout is returned when the node does not respond within the deadline.
var ErrRequestTimeout = errors.New("proxy: request timed out waiting for node response")

// ErrNodeError is returned when the node reports a non-success proxy result.
var ErrNodeError = errors.New("proxy: node reported an error for the request")

// Request represents a customer's proxy request.
type Request struct {
	URL        string
	Method     string
	Headers    map[string]string
	Body       []byte
	GeoCountry string
	GeoRegion  string
	SessionID  string
	TimeoutMs  int
	CustomerID string
}

// Response represents the proxy response returned to the customer.
type Response struct {
	RequestID        string
	StatusCode       int
	Headers          map[string]string
	Body             []byte
	NodeCountry      string
	NodeRegion       string
	NodeIP           string
	TotalMs          int
	ProxyMs          int
	BytesTransferred int64
}

// ---------------------------------------------------------------------------
// Dependency interfaces (allow injection of real or mock implementations)
// ---------------------------------------------------------------------------

// NodeSelector picks an appropriate node for an incoming request.
type NodeSelector interface {
	SelectNode(ctx context.Context, filter node.NodeFilter) (*node.Node, error)
}

// NodeLookup retrieves node details by ID.
type NodeLookup interface {
	GetByID(ctx context.Context, id string) (*node.Node, error)
}

// SessionStore manages sticky session bindings and active-request counters.
type SessionStore interface {
	GetSessionBinding(ctx context.Context, sessionID string) (string, error)
	SetSessionBinding(ctx context.Context, sessionID, nodeID string) error
	IncrActiveRequests(ctx context.Context, nodeID string) (int64, error)
	DecrActiveRequests(ctx context.Context, nodeID string) (int64, error)
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// Handler orchestrates proxy requests from customers to Android nodes.
type Handler struct {
	selector   NodeSelector
	registry   NodeLookup
	pool       *ws.Pool
	redisStore SessionStore
	timeout    time.Duration
}

// NewHandler creates a Handler backed by the real node.Selector, node.Registry,
// and node.RedisStore implementations.
func NewHandler(
	selector *node.Selector,
	registry *node.Registry,
	pool *ws.Pool,
	redisStore *node.RedisStore,
	timeout time.Duration,
) *Handler {
	return &Handler{
		selector:   selector,
		registry:   registry,
		pool:       pool,
		redisStore: redisStore,
		timeout:    timeout,
	}
}

// NewHandlerWithInterfaces creates a Handler from interface values — useful in
// tests where mock implementations are injected.
func NewHandlerWithInterfaces(
	selector NodeSelector,
	registry NodeLookup,
	pool *ws.Pool,
	redisStore SessionStore,
	timeout time.Duration,
) *Handler {
	return &Handler{
		selector:   selector,
		registry:   registry,
		pool:       pool,
		redisStore: redisStore,
		timeout:    timeout,
	}
}

// HandleRequest routes a proxy request to an appropriate Android node and
// assembles the response. It handles sticky sessions, node selection, and
// request/response lifecycle over the WebSocket connection.
func (h *Handler) HandleRequest(ctx context.Context, req Request) (*Response, error) {
	requestID := uuid.New().String()
	start := time.Now()

	// Determine effective timeout: use request's timeout if set, else handler default.
	timeout := h.timeout
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	// Create a context with the timeout deadline.
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Attempt sticky session binding: look up a previously assigned node.
	var selectedNode *node.Node
	if req.SessionID != "" {
		boundNodeID, err := h.redisStore.GetSessionBinding(reqCtx, req.SessionID)
		if err == nil && boundNodeID != "" {
			// Try to use the bound node if it still has an active connection.
			if conn, ok := h.pool.Get(boundNodeID); ok && !conn.IsClosed() {
				// Fetch node details for response metadata.
				if n, err := h.registry.GetByID(reqCtx, boundNodeID); err == nil {
					selectedNode = n
				}
			}
		}
		// If session binding lookup fails or the connection is gone,
		// fall through to fresh selection below.
	}

	// No sticky node resolved — select a fresh one.
	if selectedNode == nil {
		filter := node.NodeFilter{
			Country: req.GeoCountry,
			Region:  req.GeoRegion,
			Status:  "active",
			Limit:   20,
		}
		n, err := h.selector.SelectNode(reqCtx, filter)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNoAvailableNodes, err)
		}
		selectedNode = n

		// Persist new session binding for subsequent requests in the same session.
		if req.SessionID != "" {
			// Best-effort; do not fail the request if this write fails.
			_ = h.redisStore.SetSessionBinding(reqCtx, req.SessionID, selectedNode.ID)
		}
	}

	// Fetch the active WebSocket connection for the selected node.
	conn, ok := h.pool.Get(selectedNode.ID)
	if !ok || conn.IsClosed() {
		return nil, fmt.Errorf("proxy: no active WebSocket connection for node %s", selectedNode.ID)
	}

	// Track active requests for load-awareness in the selector.
	if _, err := h.redisStore.IncrActiveRequests(reqCtx, selectedNode.ID); err != nil {
		// Non-fatal: continue without failing the request.
		_ = err
	}
	defer func() {
		// Decrement even if the request failed; use a fresh background context so
		// this runs even if reqCtx has already been cancelled.
		_, _ = h.redisStore.DecrActiveRequests(context.Background(), selectedNode.ID)
	}()

	// Register a pending request slot on the connection so the response router
	// can deliver the reply to us via CompletePendingRequest.
	pending := conn.AddPendingRequest(requestID, timeout)
	defer conn.RemovePendingRequest(requestID)

	// Build and send the protobuf ProxyRequest to the node.
	pbReq := buildProtoRequest(requestID, req)
	wsMsg := &pb.WebSocketMessage{
		Payload: &pb.WebSocketMessage_ProxyRequest{
			ProxyRequest: pbReq,
		},
	}
	if err := conn.Send(wsMsg); err != nil {
		return nil, fmt.Errorf("proxy: send request to node %s: %w", selectedNode.ID, err)
	}

	// Wait for the assembled response, delivered via the pending request channel.
	select {
	case <-reqCtx.Done():
		return nil, ErrRequestTimeout

	case msg, ok := <-pending.ResponseCh:
		if !ok {
			return nil, fmt.Errorf("proxy: connection closed for node %s before response", selectedNode.ID)
		}

		// The response router signals completion by delivering a ProxyResponseEnd.
		endMsg := msg.GetProxyResponseEnd()
		if endMsg == nil {
			return nil, fmt.Errorf("proxy: unexpected message type in response channel for request %s", requestID)
		}
		if !endMsg.GetSuccess() {
			return nil, fmt.Errorf("%w: %s", ErrNodeError, endMsg.GetErrorMessage())
		}

		totalMs := int(time.Since(start).Milliseconds())
		resp := &Response{
			RequestID:        requestID,
			NodeCountry:      selectedNode.Country,
			NodeRegion:       selectedNode.Region,
			NodeIP:           selectedNode.IP,
			TotalMs:          totalMs,
			ProxyMs:          int(endMsg.GetLatencyMs()),
			BytesTransferred: endMsg.GetTotalBytes(),
		}
		return resp, nil
	}
}

// ---------------------------------------------------------------------------
// CONNECT tunnel
// ---------------------------------------------------------------------------

// TunnelRequest represents an HTTPS CONNECT tunnel request.
type TunnelRequest struct {
	// TargetHost is the destination in "host:port" format (e.g. "api.example.com:443").
	TargetHost string
	// RequestID uniquely identifies this tunnel session.
	RequestID string
	// TimeoutMs is the per-tunnel inactivity timeout; 0 means use handler default.
	TimeoutMs int
	// NodeID pins the tunnel to a specific node. Empty means auto-select.
	NodeID string
}

// HandleConnectRequest establishes an opaque HTTPS CONNECT tunnel through an
// Android node. No TLS termination occurs on the backend — the encrypted
// stream is relayed bidirectionally between the caller and the node. The
// function returns when the tunnel closes or the context is cancelled.
//
// dataToNode is a channel the caller uses to push bytes destined for the
// remote host (written by the customer-facing goroutine); dataFromNode is
// a channel this function writes to when bytes arrive from the node
// (read by the customer-facing goroutine). Close dataToNode to half-close
// the upstream direction and signal the node to finish.
func (h *Handler) HandleConnectRequest(
	ctx context.Context,
	req TunnelRequest,
	dataToNode <-chan []byte,
	dataFromNode chan<- []byte,
) error {
	timeout := h.timeout
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	tunnelCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Select a node for the tunnel.
	var conn *ws.Connection
	if req.NodeID != "" {
		// Caller has pinned a specific node.
		c, ok := h.pool.Get(req.NodeID)
		if !ok {
			return fmt.Errorf("proxy: node %s has no active connection for CONNECT tunnel", req.NodeID)
		}
		conn = c
	} else {
		filter := node.NodeFilter{Status: "active", Limit: 20}
		n, err := h.selector.SelectNode(tunnelCtx, filter)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrNoAvailableNodes, err)
		}
		c, ok := h.pool.Get(n.ID)
		if !ok {
			return fmt.Errorf("proxy: node %s has no active connection for CONNECT tunnel", n.ID)
		}
		conn = c
	}

	// Build a CONNECT ProxyRequest. We reuse the ProxyRequest message with
	// Method=CONNECT; the URL field carries the "host:port" target so the node
	// knows where to open the raw TCP connection.
	requestID := req.RequestID
	if requestID == "" {
		requestID = uuid.New().String()
	}
	pbReq := &pb.ProxyRequest{
		RequestId: requestID,
		Method:    pb.ProxyMethod_CONNECT,
		Url:       req.TargetHost,
		TimeoutMs: int32(req.TimeoutMs),
	}
	wsMsg := &pb.WebSocketMessage{
		Payload: &pb.WebSocketMessage_ProxyRequest{ProxyRequest: pbReq},
	}
	if err := conn.Send(wsMsg); err != nil {
		return fmt.Errorf("proxy: send CONNECT request to node: %w", err)
	}

	// Register pending slot to receive tunnel data frames on the same channel
	// mechanism used by regular proxy requests.
	pending := conn.AddPendingRequest(requestID, timeout)
	defer conn.RemovePendingRequest(requestID)

	// relay loop: pump data from caller → node and node → caller concurrently.
	errCh := make(chan error, 2)

	// upstream: caller data → node
	go func() {
		for {
			select {
			case <-tunnelCtx.Done():
				errCh <- tunnelCtx.Err()
				return
			case chunk, ok := <-dataToNode:
				if !ok {
					// caller closed upstream — signal end to node via a zero-byte chunk.
					endMsg := &pb.WebSocketMessage{
						Payload: &pb.WebSocketMessage_ProxyResponseChunk{
							ProxyResponseChunk: &pb.ProxyResponseChunk{
								RequestId:  requestID,
								ChunkIndex: -1,
								Data:       nil,
							},
						},
					}
					_ = conn.Send(endMsg)
					errCh <- nil
					return
				}
				chunkMsg := &pb.WebSocketMessage{
					Payload: &pb.WebSocketMessage_ProxyResponseChunk{
						ProxyResponseChunk: &pb.ProxyResponseChunk{
							RequestId: requestID,
							Data:      chunk,
						},
					},
				}
				if err := conn.Send(chunkMsg); err != nil {
					errCh <- fmt.Errorf("proxy: CONNECT upstream relay error: %w", err)
					return
				}
			}
		}
	}()

	// downstream: node data → caller
	go func() {
		for {
			select {
			case <-tunnelCtx.Done():
				errCh <- tunnelCtx.Err()
				return
			case msg, ok := <-pending.ResponseCh:
				if !ok {
					errCh <- nil
					return
				}
				switch p := msg.Payload.(type) {
				case *pb.WebSocketMessage_ProxyResponseChunk:
					// Forward raw encrypted bytes to the caller without inspection.
					select {
					case dataFromNode <- p.ProxyResponseChunk.GetData():
					case <-tunnelCtx.Done():
						errCh <- tunnelCtx.Err()
						return
					}
				case *pb.WebSocketMessage_ProxyResponseEnd:
					// Node signalled tunnel closure.
					if !p.ProxyResponseEnd.GetSuccess() {
						errCh <- fmt.Errorf("%w: %s", ErrNodeError, p.ProxyResponseEnd.GetErrorMessage())
					} else {
						errCh <- nil
					}
					return
				}
			}
		}
	}()

	// Wait for the first goroutine to finish; the second will then be cancelled
	// via the tunnelCtx deadline or the connection closure.
	return <-errCh
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildProtoRequest converts a Request into a protobuf ProxyRequest.
func buildProtoRequest(requestID string, req Request) *pb.ProxyRequest {
	method := methodToProto(req.Method)
	headers := make([]*pb.Header, 0, len(req.Headers))
	for k, v := range req.Headers {
		headers = append(headers, &pb.Header{Key: k, Value: v})
	}
	return &pb.ProxyRequest{
		RequestId: requestID,
		Method:    method,
		Url:       req.URL,
		Headers:   headers,
		Body:      req.Body,
		TimeoutMs: int32(req.TimeoutMs),
	}
}

// methodToProto maps an HTTP method string to the protobuf enum value.
func methodToProto(method string) pb.ProxyMethod {
	switch method {
	case "GET":
		return pb.ProxyMethod_GET
	case "POST":
		return pb.ProxyMethod_POST
	case "PUT":
		return pb.ProxyMethod_PUT
	case "DELETE":
		return pb.ProxyMethod_DELETE
	case "PATCH":
		return pb.ProxyMethod_PATCH
	case "HEAD":
		return pb.ProxyMethod_HEAD
	case "OPTIONS":
		return pb.ProxyMethod_OPTIONS
	case "CONNECT":
		return pb.ProxyMethod_CONNECT
	default:
		return pb.ProxyMethod_PROXY_METHOD_UNSPECIFIED
	}
}
