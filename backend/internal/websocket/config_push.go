// Package websocket manages WebSocket connections from Android proxy nodes.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	pb "github.com/gofullthrottle/proxy-coin/backend/pkg/protocol"
)

// configPubSubChannel is the Redis Pub/Sub channel on which config-update
// events are broadcast across backend instances. Every instance subscribes
// and pushes the new config to its locally-connected nodes.
const configPubSubChannel = "proxy-coin:config-updates"

// NodeConfig mirrors pb.NodeConfig for JSON round-tripping over Redis.
type NodeConfig struct {
	PrxyPerMb           float64 `json:"prxy_per_mb"`
	HeartbeatIntervalMs int32   `json:"heartbeat_interval_ms"`
	MaxConcurrent       int32   `json:"max_concurrent"`
	BlocklistHash       []byte  `json:"blocklist_hash,omitempty"`
	BlocklistVersion    int64   `json:"blocklist_version,omitempty"`
}

// toProto converts the local NodeConfig into the protobuf representation.
func (c NodeConfig) toProto() *pb.NodeConfig {
	return &pb.NodeConfig{
		PrxyPerMb:           c.PrxyPerMb,
		HeartbeatIntervalMs: c.HeartbeatIntervalMs,
		MaxConcurrent:       c.MaxConcurrent,
		BlocklistHash:       c.BlocklistHash,
		BlocklistVersion:    c.BlocklistVersion,
	}
}

// buildConfigUpdateMsg wraps a NodeConfig in the WebSocket envelope.
func buildConfigUpdateMsg(cfg NodeConfig) *pb.WebSocketMessage {
	return &pb.WebSocketMessage{
		Payload: &pb.WebSocketMessage_ConfigUpdate{
			ConfigUpdate: &pb.ConfigUpdate{
				Config: cfg.toProto(),
			},
		},
	}
}

// PushConfigToNode sends a ConfigUpdate message to the single node identified
// by nodeID. It returns ErrConnectionNotFound if the node has no active
// WebSocket connection on this backend instance.
func (s *Server) PushConfigToNode(nodeID string, config NodeConfig) error {
	conn, ok := s.GetConnection(nodeID)
	if !ok {
		return fmt.Errorf("%w: node %s", ErrConnectionNotFound, nodeID)
	}
	if conn.IsClosed() {
		return fmt.Errorf("%w: node %s", ErrConnectionClosed, nodeID)
	}

	msg := buildConfigUpdateMsg(config)
	if err := conn.Send(msg); err != nil {
		return fmt.Errorf("websocket: push config to node %s: %w", nodeID, err)
	}
	log.Printf("websocket: pushed config update to node %s", nodeID)
	return nil
}

// BroadcastConfig sends a ConfigUpdate to every currently-connected node.
// Individual send errors are logged but do not interrupt delivery to other nodes.
func (s *Server) BroadcastConfig(config NodeConfig) error {
	msg := buildConfigUpdateMsg(config)

	var lastErr error
	s.connections.Range(func(key, val any) bool {
		conn, ok := val.(*Connection)
		if !ok || conn.IsClosed() {
			return true
		}
		if err := conn.Send(msg); err != nil {
			log.Printf("websocket: broadcast config send error for node=%q: %v", conn.NodeID, err)
			lastErr = err
		}
		return true
	})
	return lastErr
}

// SubscribeConfigUpdates starts a Redis Pub/Sub subscriber that listens for
// config-update events published by any backend instance. On receipt it calls
// BroadcastConfig to forward the new config to all locally-connected nodes.
//
// The function blocks until ctx is cancelled. Run it in a dedicated goroutine.
func (s *Server) SubscribeConfigUpdates(ctx context.Context, redisClient *redis.Client) {
	pubsub := redisClient.Subscribe(ctx, configPubSubChannel)
	defer func() {
		if err := pubsub.Close(); err != nil {
			log.Printf("websocket: config pubsub close error: %v", err)
		}
	}()

	log.Printf("websocket: subscribed to Redis config-update channel %q", configPubSubChannel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			log.Printf("websocket: config update subscriber shutting down")
			return

		case msg, ok := <-ch:
			if !ok {
				log.Printf("websocket: config pubsub channel closed")
				return
			}

			var cfg NodeConfig
			if err := json.Unmarshal([]byte(msg.Payload), &cfg); err != nil {
				log.Printf("websocket: failed to decode config update payload: %v", err)
				continue
			}

			if err := s.BroadcastConfig(cfg); err != nil {
				log.Printf("websocket: broadcast config error: %v", err)
			}
		}
	}
}

// PublishConfigUpdate serialises cfg as JSON and publishes it to the Redis
// Pub/Sub channel so that all backend instances will forward it to their
// connected nodes. Call this from the admin API when a global config change
// is needed (e.g. rate change, blocklist version bump).
func PublishConfigUpdate(ctx context.Context, redisClient *redis.Client, cfg NodeConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("websocket: marshal config update: %w", err)
	}
	if err := redisClient.Publish(ctx, configPubSubChannel, string(data)).Err(); err != nil {
		return fmt.Errorf("websocket: publish config update to Redis: %w", err)
	}
	return nil
}
