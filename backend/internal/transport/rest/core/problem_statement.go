package core

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func (h *CoreServer) loadProblemStatement(ctx context.Context, problemID uuid.UUID) *models.Statement {
	if h.workshopUC == nil || !h.workshopUC.IsInitialized(ctx, problemID) {
		return nil
	}

	manifestData, err := h.workshopUC.ReadProblemFile(ctx, problemID, "manifest.json")
	if err != nil {
		return nil
	}

	var manifest models.ProblemManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil
	}

	if hasStatementContent(manifest.Statement) {
		statement := manifest.Statement
		return &statement
	}

	return nil
}

func hasStatementContent(statement models.Statement) bool {
	return strings.TrimSpace(statement.Title) != "" ||
		strings.TrimSpace(statement.Legend) != "" ||
		strings.TrimSpace(statement.InputFormat) != "" ||
		strings.TrimSpace(statement.OutputFormat) != "" ||
		strings.TrimSpace(statement.Notes) != "" ||
		strings.TrimSpace(statement.Scoring) != ""
}
