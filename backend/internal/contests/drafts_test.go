package contests

import (
	"context"
	"strings"
	"testing"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type mockDraftsRepo struct {
	drafts map[uuid.UUID]contestsmodel.ContestDraft
}

func newMockDraftsRepo() *mockDraftsRepo {
	return &mockDraftsRepo{
		drafts: make(map[uuid.UUID]contestsmodel.ContestDraft),
	}
}

func (m *mockDraftsRepo) CreateDraft(ctx context.Context, creation *contestsmodel.ContestDraftCreation) (uuid.UUID, error) {
	id := uuid.New()
	m.drafts[id] = contestsmodel.ContestDraft{
		ID:        id,
		ContestID: creation.ContestID,
		UserID:    creation.UserID,
		Code:      creation.Code,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return id, nil
}

func (m *mockDraftsRepo) GetDraft(ctx context.Context, id uuid.UUID) (contestsmodel.ContestDraft, error) {
	draft, ok := m.drafts[id]
	if !ok {
		return contestsmodel.ContestDraft{}, pkg.ErrNotFound
	}
	return draft, nil
}

func (m *mockDraftsRepo) GetDraftsCount(ctx context.Context, contestID, userID uuid.UUID) (int64, error) {
	var count int64
	for _, d := range m.drafts {
		if d.ContestID == contestID && d.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *mockDraftsRepo) DeleteDraft(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.drafts[id]; !ok {
		return pkg.ErrNotFound
	}
	delete(m.drafts, id)
	return nil
}

func (m *mockDraftsRepo) WithTx(tx pgx.Tx) DraftsRepo {
	return m
}

func TestDraftsUseCase_Validation(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(repo, nil)

	contestID := uuid.New()
	userID := uuid.New()

	ctx := context.Background()

	// 1. Empty code
	_, err := uc.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		UserID:    userID,
		Code:      "",
	})
	if err == nil {
		t.Fatal("expected error on empty code, got nil")
	}

	// 2. Code too large (> 64KB)
	largeCode := strings.Repeat("a", 65*1024)
	_, err = uc.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		UserID:    userID,
		Code:      largeCode,
	})
	if err == nil {
		t.Fatal("expected error on code size > 64KB, got nil")
	}
}

func TestDraftsUseCase_LimitPerUser(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(repo, nil)

	contestID := uuid.New()
	userID := uuid.New()
	ctx := context.Background()

	// Fill 50 drafts
	for i := 0; i < 50; i++ {
		_, err := repo.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{
			ContestID: contestID,
			UserID:    userID,
			Code:      "code",
		})
		if err != nil {
			t.Fatalf("failed to seed draft %d: %v", i, err)
		}
	}

	// Attempt 51st draft
	_, err := uc.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		UserID:    userID,
		Code:      "code 51",
	})
	if err == nil {
		t.Fatal("expected limit error for 51st draft, got nil")
	}
}

func TestDraftsUseCase_DeletePermissions(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(repo, nil)

	authorID := uuid.New()
	otherUserID := uuid.New()
	contestID := uuid.New()
	ctx := context.Background()

	draftID, err := repo.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		UserID:    authorID,
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
	draftID2, _ := repo.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		UserID:    authorID,
		Code:      "package main",
	})
	err = uc.DeleteDraft(ctx, draftID2, authorID, false)
	if err != nil {
		t.Fatalf("author should be able to delete draft, got: %v", err)
	}
}
