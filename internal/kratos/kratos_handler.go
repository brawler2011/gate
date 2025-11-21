package kratos

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gate149/core/internal/models"
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

type UsersUC interface {
	GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error)
	CreateUser(ctx context.Context, user *models.CreateUserInput) (uuid.UUID, error)
}

type KratosHandler struct {
	usersUC     UsersUC
	identityAPI ory.IdentityAPI
}

func NewKratosHandler(usersUC UsersUC, identityAPI ory.IdentityAPI) *KratosHandler {
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

	defaultRole := models.RoleUser

	userCreation := &models.CreateUserInput{
		Username: req.Username,
		Role:     defaultRole,
		KratosId: req.UserId,
	}

	// FIXME:
	userId, err := h.usersUC.CreateUser(ctx, userCreation)
	if err != nil {
		existingUser, fetchErr := h.usersUC.GetUserByKratosId(ctx, req.UserId)
		if fetchErr == nil && existingUser != nil {
			slog.Info("User already exists", "kratos_id", req.UserId)
			return c.Status(http.StatusOK).JSON(KratosWebhookResponse{
				Identity: &IdentityModification{
					MetadataPublic: map[string]any{
						"user_id": existingUser.Id.String(),
						"role":    existingUser.Role,
					},
				},
			})
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
		"user_id", userId.String(),
		"username", req.Username,
		"role", defaultRole,
	)

	// Update Kratos identity with metadata
	_, _, err = h.identityAPI.PatchIdentity(ctx, req.UserId).JsonPatch([]ory.JsonPatch{
		{
			Op:    "add",
			Path:  "/metadata_public/user_id",
			Value: userId.String(),
		},
		{
			Op:    "add",
			Path:  "/metadata_public/role",
			Value: defaultRole,
		},
	}).Execute()

	if err != nil {
		slog.Error("Failed to update Kratos identity metadata",
			"error", err,
			"kratos_id", req.UserId,
		)
		// We don't fail the request here because the user is already created in our DB
		// and the session is active. The metadata will be missing until next update/login
		// or we could try to retry.
	} else {
		slog.Info("Successfully updated Kratos identity metadata", "kratos_id", req.UserId)
	}

	return c.Status(http.StatusOK).JSON(KratosWebhookResponse{
		Identity: &IdentityModification{
			MetadataPublic: map[string]any{
				"user_id": userId.String(),
				"role":    defaultRole,
			},
		},
	})
}
