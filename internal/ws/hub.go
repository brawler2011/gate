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
	ID     string
	conn   WebSocketConn
	filter *models.WebSocketFilter
	send   chan []byte
	hub    *Hub
	mu     sync.RWMutex
	// submissionIDs is used for backward compatibility with individual submission progress tracking
	submissionIDs map[uuid.UUID]bool
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
// - SubmissionWebSocketEventModel (for submission list updates)
// - TestingStartedEventModel
// - TestCompletedEventModel
// - TestingCompletedEventModel
type Hub struct {
	logger        *slog.Logger
	natsConn      *nats.Conn
	clients       map[*Client]bool
	subscriptions map[string]*nats.Subscription // Changed to string key for flexibility
	register      chan *Client
	unregister    chan *Client
	mu            sync.RWMutex
	
	// listSubscription is the global subscription for submission list events
	listSubscription *nats.Subscription
	// listClients are clients subscribed to the submission list
	listClients map[*Client]bool
	// lastTestCache stores the last test_completed event for each submission
	lastTestCache map[uuid.UUID][]byte
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
		subscriptions: make(map[string]*nats.Subscription),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		listClients:   make(map[*Client]bool),
		lastTestCache: make(map[uuid.UUID][]byte),
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
				delete(h.listClients, client)
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
	for _, sub := range h.subscriptions {
		sub.Unsubscribe()
	}

	// Close list subscription if exists
	if h.listSubscription != nil {
		h.listSubscription.Unsubscribe()
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

// SubscribeToSubmissionList subscribes a client to submission list events
// Events are filtered based on the client's filter parameters
func (h *Hub) SubscribeToSubmissionList(client *Client) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Add client to list clients
	h.listClients[client] = true

	// Create list subscription if it doesn't exist
	if h.listSubscription == nil {
		sub, err := h.natsConn.Subscribe("submissions.list", func(msg *nats.Msg) {
			h.handleSubmissionListEvent(msg.Data)
		})
		if err != nil {
			return fmt.Errorf("failed to subscribe to submissions.list: %w", err)
		}
		h.listSubscription = sub
		h.logger.Debug("subscribed to NATS subject", slog.String("subject", "submissions.list"))
	}

	return nil
}

// handleSubmissionListEvent handles incoming submission list events from NATS
// This handles both submission events (created/updated) and test progress events
func (h *Hub) handleSubmissionListEvent(data []byte) {
	h.logger.Debug("received nats message on submissions.list", slog.Int("size", len(data)))

	var event models.SubmissionWebSocketEvent
	if err := json.Unmarshal(data, &event); err != nil {
		h.logger.Error("failed to unmarshal submission list event",
			slog.Any("error", err))
		return
	}

	// Cache test_completed events for later retrieval
	if event.MessageType == models.MessageTypeTestCompleted && event.SubmissionID != nil {
		h.mu.Lock()
		h.lastTestCache[*event.SubmissionID] = data
		h.mu.Unlock()
		h.logger.Debug("cached test_completed event",
			slog.String("submission_id", event.SubmissionID.String()),
			slog.Int("test_number", event.TestNumber))
	}

	// Clear cached test when testing is completed
	if event.MessageType == models.MessageTypeTestingCompleted && event.SubmissionID != nil {
		h.mu.Lock()
		delete(h.lastTestCache, *event.SubmissionID)
		h.mu.Unlock()
		h.logger.Debug("cleared cached test for submission",
			slog.String("submission_id", event.SubmissionID.String()))
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// Send to all clients that match the filter
	for client := range h.listClients {
		// Check if the event matches client's filter
		if client.filter != nil && !client.filter.MatchesEvent(&event) {
			// Log mismatch for debugging (only at debug level usually, but helpful here)
			filterUserID := "nil"
			if client.filter.UserID != nil {
				filterUserID = client.filter.UserID.String()
			}
			eventUserID := "nil"
			if event.UserID != nil {
				eventUserID = event.UserID.String()
			}
			h.logger.Debug("event dropped by filter",
				slog.String("client_id", client.ID),
				slog.String("message_type", string(event.MessageType)),
				slog.String("filter_user_id", filterUserID),
				slog.String("event_user_id", eventUserID))
			continue
		}

		// Send to client
		select {
		case client.send <- data:
			h.logger.Debug("sent event to client",
				slog.String("client_id", client.ID),
				slog.String("message_type", string(event.MessageType)))
		default:
			// Client's send buffer is full, skip this message
			h.logger.Warn("client send buffer full, skipping message",
				slog.String("client_id", client.ID))
		}
	}
}

// SubscribeToSubmissions subscribes a client to individual submission progress events
// This is kept for backward compatibility and for tracking specific submission test progress
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
		subKey := subject

		// Check if subscription already exists
		if _, exists := h.subscriptions[subKey]; exists {
			continue
		}

		sub, err := h.natsConn.Subscribe(subject, func(msg *nats.Msg) {
			var event models.TestProgressEvent
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				h.logger.Error("failed to unmarshal NATS message",
					slog.String("subject", subject),
					slog.Any("error", err))
				return
			}

			// Send to all clients subscribed to this submission
			h.broadcastToSubmissionSubscribers(event.SubmissionId, msg.Data)
		})

		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
		}

		h.subscriptions[subKey] = sub
		h.logger.Debug("subscribed to NATS subject",
			slog.String("subject", subject),
			slog.String("client_id", client.ID))
	}

	return nil
}

// broadcastToSubmissionSubscribers sends a message to all clients subscribed to a specific submission
func (h *Hub) broadcastToSubmissionSubscribers(submissionID uuid.UUID, data []byte) {
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
			subKey := fmt.Sprintf("submissions.%s.progress", submissionID.String())
			if sub, ok := h.subscriptions[subKey]; ok {
				sub.Unsubscribe()
				delete(h.subscriptions, subKey)
				h.logger.Debug("unsubscribed from NATS subject",
					slog.String("submission_id", submissionID.String()))
			}
		}
	}

	// Check if we should unsubscribe from list events
	hasOtherListClients := false
	for c := range h.listClients {
		if c != client {
			hasOtherListClients = true
			break
		}
	}

	if !hasOtherListClients && h.listSubscription != nil {
		h.listSubscription.Unsubscribe()
		h.listSubscription = nil
		h.logger.Debug("unsubscribed from submissions.list")
	}
}

// GetLastTests returns the cached last test_completed events for the given submission IDs
func (h *Hub) GetLastTests(submissionIDs []uuid.UUID) map[uuid.UUID][]byte {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make(map[uuid.UUID][]byte)
	for _, id := range submissionIDs {
		if data, ok := h.lastTestCache[id]; ok {
			result[id] = data
		}
	}
	return result
}

// NewClient creates a new Client
func NewClient(id string, conn WebSocketConn, hub *Hub, filter *models.WebSocketFilter) *Client {
	return &Client{
		ID:            id,
		conn:          conn,
		filter:        filter,
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
