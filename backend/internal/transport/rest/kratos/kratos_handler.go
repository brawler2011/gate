package kratos

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	ory "github.com/ory/client-go"
)

type KratosWebhookRequest struct {
	UserId   string `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email"`
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

func (h *KratosHandler) HandleKratosWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	slog.Info("Received webhook from Kratos",
		"method", r.Method,
		"path", r.URL.Path,
		"content_type", r.Header.Get("Content-Type"),
	)

	var req KratosWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to parse webhook body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&ErrorResponse{
			Error: "Invalid request body",
		})
		return
	}

	slog.Info("Processing webhook", "user_id", req.UserId, "username", req.Username)

	if req.UserId == "" || req.Username == "" || req.Email == "" {
		slog.Error("Missing required fields in webhook")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&ErrorResponse{
			Error: "Missing required fields: userId, username and email",
		})
		return
	}

	defaultRole := models.UserRoleUser

	kratosId, err := uuid.Parse(req.UserId)
	if err != nil {
		slog.Error("Invalid userId format", "error", err, "user_id", req.UserId)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&ErrorResponse{
			Error: "Invalid userId format",
		})
		return
	}

	userCreation := models.CreateUserInput{
		Username: req.Username,
		Role:     defaultRole,
		KratosId: kratosId,
		Email:    req.Email,
	}

	kratosId, err = h.usersUC.CreateUser(ctx, userCreation)
	if err != nil {
		existingUser, fetchErr := h.usersUC.GetUserByKratosId(ctx, kratosId)
		if fetchErr == nil && existingUser.Id != uuid.Nil {
			slog.Info("User already exists", "kratos_id", req.UserId)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(KratosWebhookResponse{})
			return
		}

		slog.Error("Failed to create user",
			"error", err,
			"kratos_id", req.UserId,
			"username", req.Username,
		)

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&ErrorResponse{
			Error: "Failed to create user in database",
		})
		return
	}

	slog.Info("Successfully created user",
		"kratos_id", req.UserId,
		"user_id", kratosId.String(),
		"username", req.Username,
		"role", defaultRole,
	)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(KratosWebhookResponse{})
}
