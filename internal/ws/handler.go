package ws

import (
	"log/slog"
	"strings"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	MaxSubmissionIDs = 20
)

// Handler handles WebSocket connections for submission progress.
//
// WebSocket Events (JSON):
// - TestingStartedEventModel: Sent when testing of a submission begins
// - TestCompletedEventModel: Sent after each individual test case completes
// - TestingCompletedEventModel: Sent when all testing for a submission is finished
//
// See contracts/core/v1/openapi.yaml for complete schema definitions.
type Handler struct {
	logger *slog.Logger
	hub    *Hub
}

// NewHandler creates a new WebSocket handler
func NewHandler(logger *slog.Logger, hub *Hub) *Handler {
	return &Handler{
		logger: logger,
		hub:    hub,
	}
}

// FiberWebSocketConn wraps a fiber websocket connection to implement WebSocketConn
type FiberWebSocketConn struct {
	*websocket.Conn
}

func (c *FiberWebSocketConn) WriteMessage(messageType int, data []byte) error {
	return c.Conn.WriteMessage(messageType, data)
}

func (c *FiberWebSocketConn) ReadMessage() (int, []byte, error) {
	return c.Conn.ReadMessage()
}

func (c *FiberWebSocketConn) Close() error {
	return c.Conn.Close()
}

// UpgradeMiddleware is a middleware that upgrades HTTP connections to WebSocket
func (h *Handler) UpgradeMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}

// HandleSubmissions handles WebSocket connections for submission progress
// GET /ws/submissions?ids=id1,id2,...
func (h *Handler) HandleSubmissions() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Parse submission IDs from query params
		idsParam := c.Query("ids")
		if idsParam == "" {
			h.logger.Debug("no submission IDs provided")
			c.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "missing ids parameter"))
			return
		}

		idStrings := strings.Split(idsParam, ",")
		if len(idStrings) > MaxSubmissionIDs {
			h.logger.Debug("too many submission IDs", slog.Int("count", len(idStrings)))
			c.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "maximum 20 submission IDs allowed"))
			return
		}

		submissionIDs := make([]uuid.UUID, 0, len(idStrings))
		for _, idStr := range idStrings {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			id, err := uuid.Parse(idStr)
			if err != nil {
				h.logger.Debug("invalid submission ID", slog.String("id", idStr), slog.Any("error", err))
				c.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid submission ID: "+idStr))
				return
			}
			submissionIDs = append(submissionIDs, id)
		}

		if len(submissionIDs) == 0 {
			h.logger.Debug("no valid submission IDs provided")
			c.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "no valid submission IDs provided"))
			return
		}

		// Create client
		clientID := uuid.New().String()
		wsConn := &FiberWebSocketConn{Conn: c}
		client := NewClient(clientID, wsConn, h.hub)

		h.logger.Info("WebSocket client connected",
			slog.String("client_id", clientID),
			slog.Int("subscription_count", len(submissionIDs)))

		// Register client with hub
		h.hub.RegisterClient(client)

		// Subscribe to submission progress events
		if err := h.hub.SubscribeToSubmissions(client, submissionIDs); err != nil {
			h.logger.Error("failed to subscribe to submissions",
				slog.String("client_id", clientID),
				slog.Any("error", err))
			h.hub.UnregisterClient(client)
			return
		}

		// Start write pump in a goroutine
		go client.WritePump()

		// Read pump blocks until client disconnects
		client.ReadPump()

		h.logger.Info("WebSocket client disconnected", slog.String("client_id", clientID))
	})
}

// RegisterRoutes registers WebSocket routes with the Fiber app
func (h *Handler) RegisterRoutes(app *fiber.App) {
	wsGroup := app.Group("/ws")
	wsGroup.Use(h.UpgradeMiddleware())
	wsGroup.Get("/submissions", h.HandleSubmissions())
}

