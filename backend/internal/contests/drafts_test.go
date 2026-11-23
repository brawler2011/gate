package contests

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mockDraftsRepo struct {
	drafts     map[uuid.UUID]contestsmodel.ContestDraft
	listFilter *contestsmodel.ContestDraftsFilter
}

func newMockDraftsRepo() *mockDraftsRepo {
	return &mockDraftsRepo{
		drafts: make(map[uuid.UUID]contestsmodel.ContestDraft),
	}
}

func (m *mockDraftsRepo) CreateDraft(ctx context.Context, userID uuid.UUID, creation *contestsmodel.ContestDraftCreation) (uuid.UUID, error) {
	id := uuid.New()
	m.drafts[id] = contestsmodel.ContestDraft{
		ID:        id,
		ContestID: creation.ContestID,
		UserID:    userID,
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

func TestDraftsUseCase_Validation(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(testTransactions[DraftTxStore]{draftTestStore{mockDraftsRepo: repo}}, repo, draftTestStore{mockDraftsRepo: repo})

	contestID := uuid.New()
	userID := uuid.New()

	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser})

	_, err := uc.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		Code:      "",
	})
	if err == nil {
		t.Fatal("expected error on empty code, got nil")
	}

	largeCode := strings.Repeat("a", 65*1024)
	_, err = uc.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		Code:      largeCode,
	})
	if err == nil {
		t.Fatal("expected error on code size > 64KB, got nil")
	}
}

func TestDraftsUseCase_LimitPerUser(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(testTransactions[DraftTxStore]{draftTestStore{mockDraftsRepo: repo}}, repo, draftTestStore{mockDraftsRepo: repo})

	contestID := uuid.New()
	userID := uuid.New()
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser})

	for i := 0; i < 50; i++ {
		_, err := repo.CreateDraft(ctx, userID, &contestsmodel.ContestDraftCreation{
			ContestID: contestID,
			Code:      "code",
		})
		if err != nil {
			t.Fatalf("failed to seed draft %d: %v", i, err)
		}
	}

	_, err := uc.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		Code:      "code 51",
	})
	if err == nil {
		t.Fatal("expected limit error for 51st draft, got nil")
	}
}

func TestDraftsUseCase_DeletePermissions(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(testTransactions[DraftTxStore]{draftTestStore{mockDraftsRepo: repo}}, repo, draftTestStore{mockDraftsRepo: repo})

	authorID := uuid.New()
	otherUserID := uuid.New()
	contestID := uuid.New()
	ctx := context.Background()

	draftID, err := repo.CreateDraft(ctx, authorID, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		Code:      "package main",
	})
	if err != nil {
		t.Fatalf("failed to create draft: %v", err)
	}

	// Other users cannot delete the draft.
	err = uc.DeleteDraft(accessmodel.WithUser(ctx, accessmodel.User{Id: otherUserID, Role: accessmodel.UserRoleUser}), contestID, draftID)
	if err == nil {
		t.Fatal("expected permission error when deleting other user's draft as non-manager, got nil")
	}

	// Managers can delete drafts.
	err = uc.DeleteDraft(accessmodel.WithUser(ctx, accessmodel.User{Id: otherUserID, Role: accessmodel.UserRoleAdmin}), contestID, draftID)
	if err != nil {
		t.Fatalf("manager should be able to delete draft, got: %v", err)
	}

	// Authors can delete their own drafts.
	draftID2, _ := repo.CreateDraft(ctx, authorID, &contestsmodel.ContestDraftCreation{
		ContestID: contestID,
		Code:      "package main",
	})
	err = uc.DeleteDraft(accessmodel.WithUser(ctx, accessmodel.User{Id: authorID, Role: accessmodel.UserRoleUser}), contestID, draftID2)
	if err != nil {
		t.Fatalf("author should be able to delete draft, got: %v", err)
	}
}

type draftTestStore struct {
	*mockDraftsRepo
	runningContests
}

// runningContests treats every id as a public, running contest anyone may enter.
type runningContests struct {
	accesstest.Loader
	noDrafts bool
}

func (c runningContests) ContestFacts(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.ContestFacts, error) {
	facts := map[uuid.UUID]access.ContestFacts{}
	for _, id := range ids {
		settings := contestsmodel.ContestSettings{ParticipationMode: contestsmodel.ParticipationModeOpen, EnableDrafts: !c.noDrafts}
		contest := contestsmodel.Contest{ID: id, Visibility: contestsmodel.ContestVisibilityPublic, Settings: settings}
		facts[id] = access.ContestFacts{Contest: contest, Now: time.Now()}
	}
	return facts, nil
}

func (m *mockDraftsRepo) ListDrafts(_ context.Context, userID uuid.UUID, filter contestsmodel.ContestDraftsFilter) (*contestsmodel.ContestDraftsList, error) {
	m.listFilter = &filter
	result := &contestsmodel.ContestDraftsList{}
	for _, draft := range m.drafts {
		if draft.ContestID == filter.ContestID && draft.UserID == userID {
			result.Drafts = append(result.Drafts, draft)
		}
	}
	return result, nil
}

func TestDraftListReturnsOnlyCallerDrafts(t *testing.T) {
	repo := newMockDraftsRepo()
	uc := NewDraftsUseCase(testTransactions[DraftTxStore]{draftTestStore{mockDraftsRepo: repo}}, repo, draftTestStore{mockDraftsRepo: repo})
	owner, other, contest := uuid.New(), uuid.New(), uuid.New()
	id, err := repo.CreateDraft(context.Background(), owner, &contestsmodel.ContestDraftCreation{ContestID: contest, Code: "private"})
	if err != nil {
		t.Fatal(err)
	}
	filter := contestsmodel.ContestDraftsFilter{ContestID: contest}
	if _, err := uc.ListDrafts(context.Background(), filter); !errors.Is(err, pkg.ErrUnauthenticated) {
		t.Fatalf("guest: %v", err)
	}
	otherCtx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: other, Role: accessmodel.UserRoleUser})
	if list, err := uc.ListDrafts(otherCtx, filter); err != nil || len(list.Drafts) != 0 {
		t.Fatalf("other user: %+v, %v", list, err)
	}
	ownerCtx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: owner, Role: accessmodel.UserRoleUser})
	list, err := uc.ListDrafts(ownerCtx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Drafts) != 1 || list.Drafts[0].ID != id {
		t.Fatalf("unexpected drafts: %+v", list)
	}
}

func TestDisabledDraftsRejectEveryoneAndStayStored(t *testing.T) {
	repo := newMockDraftsRepo()
	author, contest := uuid.New(), uuid.New()
	id, err := repo.CreateDraft(context.Background(), author, &contestsmodel.ContestDraftCreation{ContestID: contest, Code: "kept"})
	require.NoError(t, err)
	useCase := func(disabled bool) *DraftsUseCase {
		store := draftTestStore{mockDraftsRepo: repo, runningContests: runningContests{noDrafts: disabled}}
		return NewDraftsUseCase(testTransactions[DraftTxStore]{store}, repo, store)
	}
	off := useCase(true)
	for name, user := range map[string]accessmodel.User{
		"author": {Id: author, Role: accessmodel.UserRoleUser},
		"admin":  {Id: uuid.New(), Role: accessmodel.UserRoleAdmin},
	} {
		ctx := accessmodel.WithUser(context.Background(), user)
		_, err := off.ListDrafts(ctx, contestsmodel.ContestDraftsFilter{ContestID: contest})
		require.ErrorIs(t, err, pkg.ErrDraftsDisabled, name)
		_, err = off.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{ContestID: contest, Code: "new"})
		require.ErrorIs(t, err, pkg.ErrDraftsDisabled, name)
		require.ErrorIs(t, off.DeleteDraft(ctx, contest, id), pkg.ErrDraftsDisabled, name)
	}
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: author, Role: accessmodel.UserRoleUser})
	list, err := useCase(false).ListDrafts(ctx, contestsmodel.ContestDraftsFilter{ContestID: contest})
	require.NoError(t, err)
	require.Len(t, list.Drafts, 1)
	require.Equal(t, id, list.Drafts[0].ID)
}

func (draftTestStore) LockDraftOwner(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func TestDraftListPagination(t *testing.T) {
	user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	ctx := accessmodel.WithUser(context.Background(), user)
	for _, tc := range []struct {
		name                           string
		page, size, wantPage, wantSize int32
		err                            error
	}{
		{name: "defaults", wantPage: 1, wantSize: 20},
		{name: "default page", size: 1, wantPage: 1, wantSize: 1},
		{name: "default size", page: 2, wantPage: 2, wantSize: 20},
		{name: "minimum", page: 1, size: 1, wantPage: 1, wantSize: 1},
		{name: "maximum size", page: 2, size: 100, wantPage: 2, wantSize: 100},
		{name: "negative page", page: -1, size: 20, err: pkg.ErrInvalid},
		{name: "negative size", page: 1, size: -1, err: pkg.ErrInvalid},
		{name: "oversized", page: 1, size: 101, err: pkg.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockDraftsRepo()
			uc := NewDraftsUseCase(testTransactions[DraftTxStore]{draftTestStore{mockDraftsRepo: repo}}, repo, draftTestStore{mockDraftsRepo: repo})
			if tc.err != nil {
				// Invalid pagination must return before accessing a transaction.
				uc = NewDraftsUseCase(nil, nil, nil)
			}
			filter := contestsmodel.ContestDraftsFilter{ContestID: uuid.New(), Page: tc.page, PageSize: tc.size}
			_, err := uc.ListDrafts(ctx, filter)
			require.ErrorIs(t, err, tc.err)
			if tc.err != nil {
				require.Nil(t, repo.listFilter)
				return
			}
			want := filter
			want.Page, want.PageSize = tc.wantPage, tc.wantSize
			require.Equal(t, &want, repo.listFilter)
		})
	}
}

func TestDraftListAuthorizesBeforePagination(t *testing.T) {
	uc := NewDraftsUseCase(nil, nil, nil)
	_, err := uc.ListDrafts(context.Background(), contestsmodel.ContestDraftsFilter{Page: -1})
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
}
