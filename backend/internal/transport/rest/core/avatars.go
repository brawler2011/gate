package core

import (
	"fmt"
	"net/http"

	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/google/uuid"
)

type AvatarsHandler struct {
	avatarsUC *usecase.AvatarsUseCase
}

func NewAvatarsHandler(avatarsUC *usecase.AvatarsUseCase) *AvatarsHandler {
	return &AvatarsHandler{avatarsUC: avatarsUC}
}

// UploadAvatar handles POST /users/{id}/avatar
func (h *AvatarsHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	// Parse user ID from URL
	userIDStr := r.PathValue("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	// Check permissions: only the user themselves or admin can upload avatar
	if user.Id != userID && !user.IsAdmin() {
		http.Error(w, "forbidden: can only upload your own avatar", http.StatusForbidden)
		return
	}

	// Parse multipart form (max 10MB)
	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	// Get file from form
	file, header, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "avatar file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload avatar
	avatarURL, err := h.avatarsUC.UploadAvatar(ctx, userID, file, header.Filename, contentType)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to upload avatar: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"avatar_url": "%s"}`, avatarURL)
}

// DeleteAvatar handles DELETE /users/{id}/avatar
func (h *AvatarsHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	// Parse user ID from URL
	userIDStr := r.PathValue("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	// Check permissions: only the user themselves or admin can delete avatar
	if user.Id != userID && !user.IsAdmin() {
		http.Error(w, "forbidden: can only delete your own avatar", http.StatusForbidden)
		return
	}

	// Delete avatar
	err = h.avatarsUC.DeleteAvatar(ctx, userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to delete avatar: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.WriteHeader(http.StatusNoContent)
}

// GetAvatar handles GET /users/{id}/avatar (proxy to S3)
func (h *AvatarsHandler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	// For now, we'll just return the avatar URL from the user record
	// In production, you might want to proxy the actual image
	http.Error(w, "not implemented: use avatar_url from user object", http.StatusNotImplemented)
}
