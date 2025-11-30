package ws

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Client represents a WebSocket client connection
type Client struct {
	ID            string
	conn          WebSocketConn
	submissionIDs map[uuid.UUID]bool
	send          chan []byte
	hub           *Hub
	mu            sync.RWMutex
}

// WebSocketConn is an interface for WebSocket connections (allows testing)
type WebSocketConn interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
}

// Hub manages WebSocket clients and NATS subscriptions.
//
// Messages are JSON structures defined in contracts/core/v1/openapi.yaml:
// - TestingStartedEventModel
// - TestCompletedEventModel
// - TestingCompletedEventModel
type Hub struct {
	logger        *slog.Logger
	natsConn      *nats.Conn
	clients       map[*Client]bool
	subscriptions map[uuid.UUID][]*nats.Subscription
	register      chan *Client
	unregister    chan *Client
	mu            sync.RWMutex
}

// NewHub creates a new Hub
func NewHub(logger *slog.Logger, natsURL string) (*Hub, error) {
	conn, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	hub := &Hub{
		logger:        logger,
		natsConn:      conn,
		clients:       make(map[*Client]bool),
		subscriptions: make(map[uuid.UUID][]*nats.Subscription),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
	}

	return hub, nil
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Debug("client registered", slog.String("client_id", client.ID))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.cleanupClientSubscriptions(client)
			}
			h.mu.Unlock()
			h.logger.Debug("client unregistered", slog.String("client_id", client.ID))
		}
	}
}

// Close closes the hub and all connections
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all NATS subscriptions
	for _, subs := range h.subscriptions {
		for _, sub := range subs {
			sub.Unsubscribe()
		}
	}

	// Close NATS connection
	if h.natsConn != nil {
		h.natsConn.Close()
	}
}

// RegisterClient registers a new client with the hub
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient unregisters a client from the hub
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// SubscribeToSubmissions subscribes a client to submission progress events
func (h *Hub) SubscribeToSubmissions(client *Client, submissionIDs []uuid.UUID) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, submissionID := range submissionIDs {
		// Check if already subscribed
		client.mu.Lock()
		if client.submissionIDs[submissionID] {
			client.mu.Unlock()
			continue
		}
		client.submissionIDs[submissionID] = true
		client.mu.Unlock()

		subject := fmt.Sprintf("submissions.%s.progress", submissionID.String())

		sub, err := h.natsConn.Subscribe(subject, func(msg *nats.Msg) {
			var event models.TestProgressEvent
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				h.logger.Error("failed to unmarshal NATS message",
					slog.String("subject", subject),
					slog.Any("error", err))
				return
			}

			// Send to all clients subscribed to this submission
			h.broadcastToSubscribers(event.SubmissionId, msg.Data)
		})

		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
		}

		h.subscriptions[submissionID] = append(h.subscriptions[submissionID], sub)
		h.logger.Debug("subscribed to NATS subject",
			slog.String("subject", subject),
			slog.String("client_id", client.ID))
	}

	return nil
}

// broadcastToSubscribers sends a message to all clients subscribed to a submission
func (h *Hub) broadcastToSubscribers(submissionID uuid.UUID, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		client.mu.RLock()
		subscribed := client.submissionIDs[submissionID]
		client.mu.RUnlock()

		if subscribed {
			select {
			case client.send <- data:
			default:
				// Client's send buffer is full, skip this message
				h.logger.Warn("client send buffer full, skipping message",
					slog.String("client_id", client.ID))
			}
		}
	}
}

// cleanupClientSubscriptions removes subscriptions that no longer have any clients
func (h *Hub) cleanupClientSubscriptions(client *Client) {
	client.mu.RLock()
	clientSubs := make(map[uuid.UUID]bool)
	for id := range client.submissionIDs {
		clientSubs[id] = true
	}
	client.mu.RUnlock()

	for submissionID := range clientSubs {
		// Check if any other client is subscribed to this submission
		hasOtherSubscribers := false
		for c := range h.clients {
			if c == client {
				continue
			}
			c.mu.RLock()
			if c.submissionIDs[submissionID] {
				hasOtherSubscribers = true
			}
			c.mu.RUnlock()
			if hasOtherSubscribers {
				break
			}
		}

		// If no other subscribers, unsubscribe from NATS
		if !hasOtherSubscribers {
			if subs, ok := h.subscriptions[submissionID]; ok {
				for _, sub := range subs {
					sub.Unsubscribe()
				}
				delete(h.subscriptions, submissionID)
				h.logger.Debug("unsubscribed from NATS subject",
					slog.String("submission_id", submissionID.String()))
			}
		}
	}
}

// NewClient creates a new Client
func NewClient(id string, conn WebSocketConn, hub *Hub) *Client {
	return &Client{
		ID:            id,
		conn:          conn,
		submissionIDs: make(map[uuid.UUID]bool),
		send:          make(chan []byte, 256),
		hub:           hub,
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	defer func() {
		c.conn.Close()
	}()

	for message := range c.send {
		if err := c.conn.WriteMessage(1, message); err != nil { // 1 = TextMessage
			c.hub.logger.Debug("failed to write message to client",
				slog.String("client_id", c.ID),
				slog.Any("error", err))
			return
		}
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
// This keeps the connection alive by reading messages (ping/pong)
func (c *Client) ReadPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			c.hub.logger.Debug("client disconnected",
				slog.String("client_id", c.ID),
				slog.Any("error", err))
			break
		}
	}
}

