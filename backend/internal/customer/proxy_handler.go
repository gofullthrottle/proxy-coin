// Package customer handles customer-facing REST API operations.
package customer

// This file previously contained separate HandleProxyRequest and HandleBatchProxy
// handler methods. They are now implemented directly on Handler in handler.go to
// keep all route logic in one place.
//
// Sticky session support is provided by the proxy.Handler internally via
// node.RedisStore.SetSessionBinding / GetSessionBinding. Customers that want
// a sticky session should include a "session_id" field in the request body:
//
//	POST /v1/proxy
//	{"url":"https://example.com","session_id":"my-session-abc123"}
//
// Subsequent requests with the same session_id will be routed to the same
// node if the session binding is still active (5-minute TTL in Redis).
