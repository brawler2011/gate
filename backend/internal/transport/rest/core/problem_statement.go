package core

import (
	"context"
	"strings"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func (h *CoreServer) loadProblemStatement(ctx context.Context, problemID uuid.UUID) *models.Statement {
	if h.workshopUC == nil || !h.workshopUC.IsInitialized(ctx, problemID) {
		return nil
	}

	manifest, err := h.workshopUC.GetManifest(ctx, problemID)
	if err != nil {
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
