package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type mockDraftsRepo struct {
	drafts map[uuid.UUID]models.ContestDraft
}

func newMockDraftsRepo() *mockDraftsRepo {
	return &mockDraftsRepo{
		drafts: make(map[uuid.UUID]models.ContestDraft),
	}
}

func (m *mockDraftsRepo) CreateDraft(ctx context.Context, creation *models.ContestDraftCreation) (uuid.UUID, error) {
	id := uuid.New()
	m.drafts[id] = models.ContestDraft{
		ID:        id,
		ContestID: creation.ContestID,
		UserID:    creation.UserID,
		ProblemID: creation.ProblemID,
		Language:  creation.Language,
		Code:      creation.Code,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return id, nil
}

func (m *mockDraftsRepo) GetDraft(ctx context.Context, id uuid.UUID) (models.ContestDraft, error) {
	draft, ok := m.drafts[id]
	if !ok {
		return models.ContestDraft{}, pkg.ErrNotFound
	}
	return draft, nil
}

func (m *mockDraftsRepo) GetDraftsCountByProblem(ctx context.Context, contestID, userID, problemID uuid.UUID) (int64, error) {
	var count int64
	for _, d := range m.drafts {
		if d.ContestID == contestID && d.UserID == userID && d.ProblemID == problemID {
			count++
		}
	}
	return count, nil
}

func (m *mockDraftsRepo) ListDrafts(ctx context.Context, filter models.ContestDraftsFilter) ([]models.ContestDraft, int32, error) {
	var result []models.ContestDraft
	for _, d := range m.drafts {
		if d.ContestID != filter.ContestID {
			continue
		}
		if filter.UserID != nil && d.UserID != *filter.UserID {
			continue
		}
		if filter.ProblemID != nil && d.ProblemID != *filter.ProblemID {
			continue
		}
		result = append(result, d)
	}
	//nolint:gosec
	return result, int32(len(result)), nil
}

func (m *mockDraftsRepo) DeleteDraft(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.drafts[id]; !ok {
		return pkg.ErrNotFound
	}
	delete(m.drafts, id)
	return nil
}

func (m *mockDraftsRepo) WithTx(tx pgx.Tx) interfaces.DraftsRepo {
	return m
}

func TestDraftsUseCase_Validation(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(repo, nil, nil, nil)

	contestID := uuid.New()
	userID := uuid.New()
	problemID := uuid.New()

	ctx := context.Background()

	// 1. Empty code
	_, err := uc.CreateDraft(ctx, &models.ContestDraftCreation{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
		Language:  models.Cpp,
		Code:      "",
	})
	if err == nil {
		t.Fatal("expected error on empty code, got nil")
	}

	// 2. Code too large (> 64KB)
	largeCode := strings.Repeat("a", 65*1024)
	_, err = uc.CreateDraft(ctx, &models.ContestDraftCreation{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
		Language:  models.Cpp,
		Code:      largeCode,
	})
	if err == nil {
		t.Fatal("expected error on code size > 64KB, got nil")
	}

	// 3. Invalid language
	_, err = uc.CreateDraft(ctx, &models.ContestDraftCreation{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
		Language:  999,
		Code:      "print(1)",
	})
	if err == nil {
		t.Fatal("expected error on invalid language, got nil")
	}
}

func TestDraftsUseCase_LimitPerProblem(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(repo, nil, nil, nil)

	contestID := uuid.New()
	userID := uuid.New()
	problemID := uuid.New()
	ctx := context.Background()

	// Fill 50 drafts
	for i := 0; i < 50; i++ {
		_, err := repo.CreateDraft(ctx, &models.ContestDraftCreation{
			ContestID: contestID,
			UserID:    userID,
			ProblemID: problemID,
			Language:  models.Python,
			Code:      "code",
		})
		if err != nil {
			t.Fatalf("failed to seed draft %d: %v", i, err)
		}
	}

	// Attempt 51st draft
	_, err := uc.CreateDraft(ctx, &models.ContestDraftCreation{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
		Language:  models.Python,
		Code:      "code 51",
	})
	if err == nil {
		t.Fatal("expected limit error for 51st draft, got nil")
	}
}

func TestDraftsUseCase_DeletePermissions(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(repo, nil, nil, nil)

	authorID := uuid.New()
	otherUserID := uuid.New()
	contestID := uuid.New()
	problemID := uuid.New()
	ctx := context.Background()

	draftID, err := repo.CreateDraft(ctx, &models.ContestDraftCreation{
		ContestID: contestID,
		UserID:    authorID,
		ProblemID: problemID,
		Language:  models.Golang,
		Code:      "package main",
	})
	if err != nil {
		t.Fatalf("failed to create draft: %v", err)
	}

	// Other user cannot delete
	err = uc.DeleteDraft(ctx, draftID, otherUserID, false)
	if err == nil {
		t.Fatal("expected permission error when deleting other user's draft as non-manager, got nil")
	}

	// Manager can delete
	err = uc.DeleteDraft(ctx, draftID, otherUserID, true)
	if err != nil {
		t.Fatalf("manager should be able to delete draft, got: %v", err)
	}

	// Author can delete
	draftID2, _ := repo.CreateDraft(ctx, &models.ContestDraftCreation{
		ContestID: contestID,
		UserID:    authorID,
		ProblemID: problemID,
		Language:  models.Golang,
		Code:      "package main",
	})
	err = uc.DeleteDraft(ctx, draftID2, authorID, false)
	if err != nil {
		t.Fatalf("author should be able to delete draft, got: %v", err)
	}
}
