package kratos

import (
	"log/slog"
	"net/http"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	ory "github.com/ory/client-go"
)

type KratosWebhookRequest struct {
	UserId   string `json:"userId"`
	Username string `json:"username"`
}

type KratosWebhookResponse struct {
	Identity *IdentityModification `json:"identity,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type IdentityModification struct {
	MetadataPublic map[string]any `json:"metadata_public"`
}

type KratosHandler struct {
	usersUC     interfaces.UsersUC
	identityAPI ory.IdentityAPI
}

func NewKratosHandler(usersUC interfaces.UsersUC, identityAPI ory.IdentityAPI) *KratosHandler {
	return &KratosHandler{
		usersUC:     usersUC,
		identityAPI: identityAPI,
	}
}

func (h *KratosHandler) HandleKratosWebhook(c *fiber.Ctx) error {
	ctx := c.Context()

	slog.Info("Received webhook from Kratos",
		"method", c.Method(),
		"path", c.Path(),
		"content_type", c.Get("Content-Type"),
	)

	var req KratosWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		slog.Error("Failed to parse webhook body", "error", err)
		return c.Status(http.StatusBadRequest).JSON(&ErrorResponse{
			Error: "Invalid request body",
		})
	}

	slog.Info("Processing webhook", "user_id", req.UserId, "username", req.Username)

	if req.UserId == "" || req.Username == "" {
		slog.Error("Missing required fields in webhook")

		return c.Status(http.StatusBadRequest).JSON(&ErrorResponse{
			Error: "Missing required fields: userId and username",
		})
	}

	defaultRole := models.UserRoleUser

	kratosId, err := uuid.Parse(req.UserId)
	if err != nil {
		slog.Error("Invalid userId format", "error", err, "user_id", req.UserId)
		return c.Status(http.StatusBadRequest).JSON(&ErrorResponse{
			Error: "Invalid userId format",
		})
	}

	userCreation := models.CreateUserInput{
		Username: req.Username,
		Role:     defaultRole,
		KratosId: kratosId,
	}

	kratosId, err = h.usersUC.CreateUser(ctx, userCreation)
	if err != nil {
		existingUser, fetchErr := h.usersUC.GetUserByKratosId(ctx, kratosId)
		if fetchErr == nil && existingUser.Id != uuid.Nil {
			slog.Info("User already exists", "kratos_id", req.UserId)
			return c.Status(http.StatusOK).JSON(KratosWebhookResponse{})
		}

		slog.Error("Failed to create user",
			"error", err,
			"kratos_id", req.UserId,
			"username", req.Username,
		)

		return c.Status(http.StatusInternalServerError).JSON(&ErrorResponse{
			Error: "Failed to create user in database",
		})
	}

	slog.Info("Successfully created user",
		"kratos_id", req.UserId,
		"user_id", kratosId.String(),
		"username", req.Username,
		"role", defaultRole,
	)

	return c.Status(http.StatusOK).JSON(KratosWebhookResponse{})
}
