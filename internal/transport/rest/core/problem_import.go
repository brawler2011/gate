package core

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/gate149/core/internal/transport/middleware"
	"github.com/gate149/core/internal/usecase"
	"github.com/google/uuid"
)

type ProblemImportHandler struct {
	importUC *usecase.ProblemImportUseCase
}

func NewProblemImportHandler(importUC *usecase.ProblemImportUseCase) *ProblemImportHandler {
	return &ProblemImportHandler{importUC: importUC}
}

// ImportProblem handles POST /api/problems/import
func (h *ProblemImportHandler) ImportProblem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	// Check if user is authenticated and has permission
	if user.IsGuest() {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	// Only admins can import problems (for now)
	if !user.IsAdmin() {
		http.Error(w, "forbidden: only admins can import problems", http.StatusForbidden)
		return
	}

	// Parse multipart form (max 100MB)
	err := r.ParseMultipartForm(100 << 20)
	if err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	// Get file from form
	file, header, err := r.FormFile("package")
	if err != nil {
		http.Error(w, "package file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file into memory (for ReaderAt interface)
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	// Generate problem ID
	problemID := uuid.New()

	// Import problem
	format, err := h.importUC.ImportProblemPackage(ctx, bytes.NewReader(fileBytes), int64(len(fileBytes)), problemID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to import problem: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"problem_id": "%s", "format": "%s", "filename": "%s"}`, problemID, format, header.Filename)
}
