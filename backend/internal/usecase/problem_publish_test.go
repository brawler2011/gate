package usecase

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPublishProblem_Validation(t *testing.T) {
	ctx := context.Background()
	problemID := uuid.New()
	orgID := uuid.New()

	store := storage.NewLocalStorage(t.TempDir())
	wsStorage := NewWorkspaceStorage(store, "test-workshop")

	mockRepo := newProblemImportMockProblemsRepo()
	mockRepo.problems[problemID] = models.Problem{
		ID:             problemID,
		OrganizationID: orgID,
		Title:          "Test Problem",
	}

	publishUC := NewProblemPublishUseCase(mockRepo, nil, wsStorage, nil, "packages")

	// Case 1: pass-fail without checker fails
	yamlNoChecker := `format_version: "1.0"
title: "No Checker"
type: "pass-fail"
limits:
  time_ms: 1000
  memory_mb: 256
`
	require.NoError(t, wsStorage.WriteFile(ctx, problemID, "problem.yaml", []byte(yamlNoChecker)))
	_, err := publishUC.PublishProblem(ctx, problemID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "problem must have a checker")

	// Case 2: pass-fail with missing checker file fails
	yamlWithMissingChecker := `format_version: "1.0"
title: "Missing Checker"
type: "pass-fail"
checker: checkers/checker.cpp
limits:
  time_ms: 1000
  memory_mb: 256
`
	require.NoError(t, wsStorage.WriteFile(ctx, problemID, "problem.yaml", []byte(yamlWithMissingChecker)))
	_, err = publishUC.PublishProblem(ctx, problemID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "checker file")

	// Case 3: interactive without interactor fails
	yamlInteractiveNoInteractor := `format_version: "1.0"
title: "Interactive"
type: "interactive"
limits:
  time_ms: 1000
  memory_mb: 256
`
	require.NoError(t, wsStorage.WriteFile(ctx, problemID, "problem.yaml", []byte(yamlInteractiveNoInteractor)))
	_, err = publishUC.PublishProblem(ctx, problemID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "interactive problem must have an interactor")
}
