package kratos

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gate149/core/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// KratosWebhookRequest represents the webhook payload from Kratos
type KratosWebhookRequest struct {
	UserId   string `json:"userId"`
	Username string `json:"username"`
}

// KratosWebhookResponse represents the response to Kratos
type KratosWebhookResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type UsersUC interface {
	GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error)
	CreateUser(ctx context.Context, user *models.UserCreation) (uuid.UUID, error)
}

type KratosHandler struct {
	usersUC UsersUC
}

func NewKratosHandler(usersUC UsersUC) *KratosHandler {
	return &KratosHandler{
		usersUC: usersUC,
	}
}

// HandleKratosWebhook handles webhook requests from Kratos
func (h *KratosHandler) HandleKratosWebhook(c *fiber.Ctx) error {
	ctx := c.Context()

	slog.Info("Received webhook from Kratos",
		slog.String("method", c.Method()),
		slog.String("path", c.Path()),
		slog.String("content_type", c.Get("Content-Type")),
	)

	var req KratosWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		slog.Error("Failed to parse webhook body", slog.Any("error", err))
		return c.Status(http.StatusBadRequest).JSON(KratosWebhookResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}

	slog.Info("Processing webhook",
		slog.String("user_id", req.UserId),
		slog.String("username", req.Username),
	)

	// Validate required fields
	if req.UserId == "" || req.Username == "" {
		slog.Error("Missing required fields in webhook")
		return c.Status(http.StatusBadRequest).JSON(KratosWebhookResponse{
			Success: false,
			Error:   "Missing required fields: userId and username",
		})
	}

	// Check if user already exists
	existingUser, err := h.usersUC.GetUserByKratosId(ctx, req.UserId)
	if err == nil && existingUser != nil {
		slog.Info("User already exists", slog.String("kratos_id", req.UserId))
		return c.Status(http.StatusOK).JSON(KratosWebhookResponse{
			Success: true,
			Message: "User already exists",
		})
	}

	// Create new user
	userCreation := &models.UserCreation{
		Id:       uuid.New(),
		Username: req.Username,
		Role:     "user", // Default role for new users
		KratosId: &req.UserId,
	}

	_, err = h.usersUC.CreateUser(ctx, userCreation)
	if err != nil {
		slog.Error("Failed to create user",
			slog.Any("error", err),
			slog.String("kratos_id", req.UserId),
			slog.String("username", req.Username),
		)
		return c.Status(http.StatusInternalServerError).JSON(KratosWebhookResponse{
			Success: false,
			Error:   "Failed to create user in database",
		})
	}

	slog.Info("Successfully created user",
		slog.String("kratos_id", req.UserId),
		slog.String("username", req.Username),
	)

	return c.Status(http.StatusOK).JSON(KratosWebhookResponse{
		Success: true,
		Message: "User created successfully",
	})
}

// HealthCheck provides a simple health check endpoint for the private server
func (h *KratosHandler) HealthCheck(c *fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "tester-private-server",
	})
}
