package core

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gate149/core/internal/transport/middleware"
	"github.com/gate149/core/internal/usecase"
	"github.com/google/uuid"
)

type ProblemPublishHandler struct {
	publishUC *usecase.ProblemPublishUseCase
}

func NewProblemPublishHandler(publishUC *usecase.ProblemPublishUseCase) *ProblemPublishHandler {
	return &ProblemPublishHandler{publishUC: publishUC}
}

type PublishProblemRequest struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commit_sha"`
}

type PublishProblemResponse struct {
	ProblemID  string `json:"problem_id"`
	Version    string `json:"version"`
	PackageURL string `json:"package_url"`
}

// PublishProblem handles POST /api/problems/{id}/publish
func (h *ProblemPublishHandler) PublishProblem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	// Check if user is authenticated
	if user.IsGuest() {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	// Only admins can publish problems (for now)
	// TODO: Check problem-specific permissions
	if !user.IsAdmin() {
		http.Error(w, "forbidden: only admins can publish problems", http.StatusForbidden)
		return
	}

	// Parse problem ID from URL
	problemIDStr := r.PathValue("id")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		http.Error(w, "invalid problem ID", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req PublishProblemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate version
	if req.Version == "" {
		http.Error(w, "version is required", http.StatusBadRequest)
		return
	}

	// Publish problem
	err = h.publishUC.PublishProblem(ctx, problemID, req.Version, req.CommitSHA)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to publish problem: %v", err), http.StatusInternalServerError)
		return
	}

	// Get package URL
	packageURL, err := h.publishUC.GetPublishedPackageURL(ctx, problemID, req.Version)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get package URL: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	resp := PublishProblemResponse{
		ProblemID:  problemID.String(),
		Version:    req.Version,
		PackageURL: packageURL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GetPublishedPackage handles GET /api/problems/{id}/package/{version}
func (h *ProblemPublishHandler) GetPublishedPackage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse problem ID from URL
	problemIDStr := r.PathValue("id")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		http.Error(w, "invalid problem ID", http.StatusBadRequest)
		return
	}

	// Parse version from URL
	version := r.PathValue("version")
	if version == "" {
		http.Error(w, "version is required", http.StatusBadRequest)
		return
	}

	// Get package URL
	packageURL, err := h.publishUC.GetPublishedPackageURL(ctx, problemID, version)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get package URL: %v", err), http.StatusInternalServerError)
		return
	}

	// Redirect to S3 presigned URL
	http.Redirect(w, r, packageURL, http.StatusFound)
}
