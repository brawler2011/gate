package ws

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now, or configure based on env
	},
}

// Handler handles WebSocket connections for submission progress.
//
// WebSocket Events (JSON):
// - SubmissionWebSocketEventModel: Sent when a submission is created or updated
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

// GorillaWebSocketConn wraps a gorilla websocket connection to implement WebSocketConn
type GorillaWebSocketConn struct {
	*websocket.Conn
}

func (c *GorillaWebSocketConn) WriteMessage(messageType int, data []byte) error {
	return c.Conn.WriteMessage(messageType, data)
}

func (c *GorillaWebSocketConn) ReadMessage() (int, []byte, error) {
	return c.Conn.ReadMessage()
}

func (c *GorillaWebSocketConn) Close() error {
	return c.Conn.Close()
}

// HandleSubmissions handles WebSocket connections for submission list updates
// GET /ws/submissions?contestId=uuid&userId=uuid&problemId=uuid&state=int&language=int&sortOrder=desc&ids=uuid1,uuid2
//
// Parameters:
// - contestId (optional): Filter by contest ID
// - userId (optional): Filter by user ID
// - problemId (optional): Filter by problem ID
// - state (optional): Filter by submission state
// - language (optional): Filter by programming language
// - sortOrder (required): Must be "desc" for real-time updates (page=1, sortOrder=desc)
// - ids (optional): Comma-separated submission IDs to get cached last test events on connect
func (h *Handler) HandleSubmissions(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("failed to upgrade to websocket", slog.Any("error", err))
		return
	}

	// Parse sortOrder - must be "desc" for real-time updates
	sortOrder := r.URL.Query().Get("sortOrder")
	if sortOrder != "desc" {
		h.logger.Debug("invalid sortOrder for WebSocket", slog.String("sortOrder", sortOrder))
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData,
				"sortOrder must be 'desc' for real-time updates (page=1, sortOrder=desc)"))
		conn.Close()
		return
	}

	// Build filter from query parameters
	filter := models.WebSocketFilter{}

	// Parse contestId
	if contestIdStr := r.URL.Query().Get("contestId"); contestIdStr != "" {
		contestId, err := uuid.Parse(contestIdStr)
		if err != nil {
			h.logger.Debug("invalid contestId", slog.String("contestId", contestIdStr), slog.Any("error", err))
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid contestId: "+contestIdStr))
			conn.Close()
			return
		}
		filter.ContestID = &contestId
	}

	// Parse userId
	if userIdStr := r.URL.Query().Get("userId"); userIdStr != "" {
		userId, err := uuid.Parse(userIdStr)
		if err != nil {
			h.logger.Debug("invalid userId", slog.String("userId", userIdStr), slog.Any("error", err))
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid userId: "+userIdStr))
			conn.Close()
			return
		}
		filter.UserID = &userId
	}

	// Parse problemId
	if problemIdStr := r.URL.Query().Get("problemId"); problemIdStr != "" {
		problemId, err := uuid.Parse(problemIdStr)
		if err != nil {
			h.logger.Debug("invalid problemId", slog.String("problemId", problemIdStr), slog.Any("error", err))
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid problemId: "+problemIdStr))
			conn.Close()
			return
		}
		filter.ProblemID = &problemId
	}

	// Parse state
	if stateStr := r.URL.Query().Get("state"); stateStr != "" {
		stateInt, err := strconv.ParseInt(stateStr, 10, 64)
		if err != nil {
			h.logger.Debug("invalid state", slog.String("state", stateStr), slog.Any("error", err))
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid state: "+stateStr))
			conn.Close()
			return
		}
		state := models.State(stateInt)
		filter.State = &state
	}

	// Parse language
	if languageStr := r.URL.Query().Get("language"); languageStr != "" {
		languageInt, err := strconv.ParseInt(languageStr, 10, 64)
		if err != nil {
			h.logger.Debug("invalid language", slog.String("language", languageStr), slog.Any("error", err))
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid language: "+languageStr))
			conn.Close()
			return
		}
		language := models.LanguageName(languageInt)
		filter.Language = &language
	}

	// Parse ids (comma-separated submission IDs to get cached last tests)
	var submissionIDs []uuid.UUID
	if idsStr := r.URL.Query().Get("ids"); idsStr != "" {
		idParts := strings.Split(idsStr, ",")
		for _, idStr := range idParts {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			id, err := uuid.Parse(idStr)
			if err != nil {
				h.logger.Debug("invalid id in ids parameter", slog.String("id", idStr), slog.Any("error", err))
				conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid id in ids: "+idStr))
				conn.Close()
				return
			}
			submissionIDs = append(submissionIDs, id)
		}
	}

	// Create client
	clientID := uuid.New().String()
	wsConn := &GorillaWebSocketConn{Conn: conn}
	client := NewClient(clientID, wsConn, h.hub, &filter)

	h.logger.Info("WebSocket client connected for submission list",
		slog.String("client_id", clientID),
		slog.Any("filter", filter))

	// Register client with hub
	h.hub.RegisterClient(client)

	// Subscribe to submission list events (new submissions and updates)
	if err := h.hub.SubscribeToSubmissionList(client); err != nil {
		h.logger.Error("failed to subscribe to submission list",
			slog.String("client_id", clientID),
			slog.Any("error", err))
		h.hub.UnregisterClient(client)
		return
	}

	// Send cached last tests for requested submission IDs
	if len(submissionIDs) > 0 {
		cachedTests := h.hub.GetLastTests(submissionIDs)
		for id, data := range cachedTests {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				h.logger.Error("failed to send cached test",
					slog.String("client_id", clientID),
					slog.String("submission_id", id.String()),
					slog.Any("error", err))
			} else {
				h.logger.Debug("sent cached test to client",
					slog.String("client_id", clientID),
					slog.String("submission_id", id.String()))
			}
		}
	}

	// Start write pump in a goroutine
	go client.WritePump()

	// Read pump blocks until client disconnects
	client.ReadPump()

	h.logger.Info("WebSocket client disconnected", slog.String("client_id", clientID))
}
