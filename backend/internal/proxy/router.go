// Package proxy handles HTTP/HTTPS proxy request routing and forwarding.
package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofullthrottle/proxy-coin/backend/internal/metering"
	ws "github.com/gofullthrottle/proxy-coin/backend/internal/websocket"
	pb "github.com/gofullthrottle/proxy-coin/backend/pkg/protocol"
)

// ErrResponseNotFound is returned when a request ID has no buffered response.
var ErrResponseNotFound = errors.New("proxy: no buffered response for request ID")

// ErrResponseIncomplete is returned when the response has not been fully received.
var ErrResponseIncomplete = errors.New("proxy: response not yet complete")

// responseBuffer accumulates streaming response chunks for a single proxy request.
type responseBuffer struct {
	requestID  string
	nodeID     string
	customerID string
	statusCode int
	headers    map[string]string
	chunks     [][]byte
	totalBytes int64
	latencyMs  int
	startTime  int64 // Unix milliseconds
	complete   bool
}

// ResponseAssembler collects streaming response chunks into complete responses
// and publishes metering events upon completion.
type ResponseAssembler struct {
	mu       sync.Mutex
	pending  map[string]*responseBuffer // requestID → buffer
	reporter *metering.Reporter
}

// NewResponseAssembler creates a ResponseAssembler that publishes to reporter.
func NewResponseAssembler(reporter *metering.Reporter) *ResponseAssembler {
	return &ResponseAssembler{
		pending:  make(map[string]*responseBuffer),
		reporter: reporter,
	}
}

// HandleResponseStart initialises a new response buffer for a streaming response.
// It is called when the node sends the first ProxyResponseStart message.
func (ra *ResponseAssembler) HandleResponseStart(conn *ws.Connection, msg *pb.ProxyResponseStart) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	headers := make(map[string]string, len(msg.GetHeaders()))
	for _, h := range msg.GetHeaders() {
		headers[h.GetKey()] = h.GetValue()
	}

	buf := &responseBuffer{
		requestID:  msg.GetRequestId(),
		nodeID:     conn.NodeID,
		statusCode: int(msg.GetStatusCode()),
		headers:    headers,
		startTime:  time.Now().UnixMilli(),
	}
	ra.pending[msg.GetRequestId()] = buf
}

// HandleResponseChunk appends a data chunk to the response buffer.
// Chunks may arrive out of order; the assembler stores them in arrival order
// because the node is expected to send them sequentially.
func (ra *ResponseAssembler) HandleResponseChunk(conn *ws.Connection, msg *pb.ProxyResponseChunk) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	buf, ok := ra.pending[msg.GetRequestId()]
	if !ok {
		log.Printf("proxy/assembler: received chunk for unknown request %s from node %s",
			msg.GetRequestId(), conn.NodeID)
		return
	}

	chunk := msg.GetData()
	buf.chunks = append(buf.chunks, chunk)
	buf.totalBytes += int64(len(chunk))
}

// HandleResponseEnd marks the response as complete, assembles the body,
// delivers the result to the waiting Handler via the connection's pending
// request channel, and publishes a metering event.
func (ra *ResponseAssembler) HandleResponseEnd(conn *ws.Connection, msg *pb.ProxyResponseEnd) {
	ra.mu.Lock()
	buf, ok := ra.pending[msg.GetRequestId()]
	if !ok {
		ra.mu.Unlock()
		log.Printf("proxy/assembler: received end for unknown request %s from node %s",
			msg.GetRequestId(), conn.NodeID)
		return
	}

	buf.complete = true
	buf.latencyMs = int(msg.GetLatencyMs())
	// Use node-reported total bytes if non-zero, otherwise use accumulated chunk size.
	if msg.GetTotalBytes() > 0 {
		buf.totalBytes = msg.GetTotalBytes()
	}

	// Snapshot fields needed outside the lock.
	nodeID := buf.nodeID
	customerID := buf.customerID
	statusCode := buf.statusCode
	totalBytes := buf.totalBytes
	latencyMs := buf.latencyMs
	success := msg.GetSuccess()
	requestID := buf.requestID

	ra.mu.Unlock()

	// Deliver the ProxyResponseEnd to the Handler waiting on the pending channel.
	// This unblocks HandleRequest so it can return the Response to the caller.
	conn.CompletePendingRequest(requestID, &pb.WebSocketMessage{
		Payload: &pb.WebSocketMessage_ProxyResponseEnd{
			ProxyResponseEnd: msg,
		},
	})

	// Publish metering event asynchronously so we do not block the read pump.
	go func() {
		event := metering.Event{
			RequestID:  requestID,
			NodeID:     nodeID,
			CustomerID: customerID,
			BytesOut:   totalBytes,
			LatencyMs:  latencyMs,
			StatusCode: statusCode,
			Success:    success,
			Timestamp:  time.Now().UTC(),
		}
		if err := ra.reporter.Publish(context.Background(), event); err != nil {
			log.Printf("proxy/assembler: metering publish error for request %s: %v", requestID, err)
		}
	}()
}

// GetCompleteResponse returns the assembled body, status code, and headers for
// a completed response. It removes the buffer from the assembler.
// Returns ErrResponseNotFound if the request ID is unknown, or
// ErrResponseIncomplete if the response has not been fully received yet.
func (ra *ResponseAssembler) GetCompleteResponse(requestID string) ([]byte, int, map[string]string, error) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	buf, ok := ra.pending[requestID]
	if !ok {
		return nil, 0, nil, fmt.Errorf("%w: %s", ErrResponseNotFound, requestID)
	}
	if !buf.complete {
		return nil, 0, nil, fmt.Errorf("%w: %s", ErrResponseIncomplete, requestID)
	}

	// Assemble the body from collected chunks.
	body := assembleBody(buf.chunks)

	// Remove the buffer now that it has been consumed.
	delete(ra.pending, requestID)

	return body, buf.statusCode, buf.headers, nil
}

// assembleBody concatenates chunk slices into a single byte slice efficiently.
func assembleBody(chunks [][]byte) []byte {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) == 1 {
		return chunks[0]
	}
	var buf bytes.Buffer
	for _, c := range chunks {
		buf.Write(c)
	}
	return buf.Bytes()
}
