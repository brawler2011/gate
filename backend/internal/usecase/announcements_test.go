package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAnnouncementsRepo struct {
	announcements map[uuid.UUID]models.ContestAnnouncement
}

func newMockAnnouncementsRepo() *mockAnnouncementsRepo {
	return &mockAnnouncementsRepo{
		announcements: make(map[uuid.UUID]models.ContestAnnouncement),
	}
}

func (m *mockAnnouncementsRepo) WithTx(tx pgx.Tx) interfaces.AnnouncementsRepo {
	return m
}

func (m *mockAnnouncementsRepo) CreateAnnouncement(ctx context.Context, input *models.CreateContestAnnouncementInput) (*models.ContestAnnouncement, error) {
	id := uuid.New()
	letter := "A"
	a := models.ContestAnnouncement{
		ID:             id,
		ContestID:      input.ContestID,
		ProblemID:      input.ProblemID,
		ProblemTitle:   nil,
		ProblemLetter:  &letter,
		AuthorID:       input.AuthorID,
		AuthorUsername: "jury_admin",
		Title:          input.Title,
		Body:           input.Body,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	m.announcements[id] = a
	return &a, nil
}

func (m *mockAnnouncementsRepo) GetAnnouncementByID(ctx context.Context, id uuid.UUID) (*models.ContestAnnouncement, error) {
	if a, ok := m.announcements[id]; ok {
		return &a, nil
	}
	return nil, nil
}

func (m *mockAnnouncementsRepo) ListAnnouncements(ctx context.Context, contestID uuid.UUID, page, pageSize int32) (*models.ContestAnnouncementsList, error) {
	var list []models.ContestAnnouncement
	for _, a := range m.announcements {
		if a.ContestID == contestID {
			list = append(list, a)
		}
	}
	return &models.ContestAnnouncementsList{
		Announcements: list,
		Pagination:    models.NewPagination(page, pageSize, int32(len(list))), //nolint:gosec // test slice length
	}, nil
}

func (m *mockAnnouncementsRepo) DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error {
	delete(m.announcements, id)
	return nil
}

type mockTransactor struct{}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return fn(ctx, nil)
}

type mockOutboxRepo struct {
	events []*models.CreateOutboxEventParams
}

func (m *mockOutboxRepo) WithTx(tx pgx.Tx) interfaces.OutboxRepo {
	return m
}

func (m *mockOutboxRepo) CreateEvent(ctx context.Context, params *models.CreateOutboxEventParams) error {
	m.events = append(m.events, params)
	return nil
}

func (m *mockOutboxRepo) DeleteOldEvents(ctx context.Context, retentionDays int32) error {
	return nil
}

func (m *mockOutboxRepo) PickEvents(ctx context.Context, limit int32, timeoutSec int32) ([]models.OutboxEvent, error) {
	return nil, nil
}

func (m *mockOutboxRepo) MarkAsCompleted(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockOutboxRepo) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	return nil
}

func (m *mockOutboxRepo) ResetFailedToPending(ctx context.Context, maxRetries int32, retryDelaySec int32) error {
	return nil
}

func TestAnnouncementsUseCase_CreateAndList(t *testing.T) {
	ctx := context.Background()
	repo := newMockAnnouncementsRepo()
	outboxRepo := &mockOutboxRepo{}
	transactor := &mockTransactor{}

	uc := NewAnnouncementsUseCase(repo, nil, outboxRepo, transactor)

	contestID := uuid.New()
	authorID := uuid.New()

	// 1. Validation error on empty title
	_, err := uc.CreateAnnouncement(ctx, &models.CreateContestAnnouncementInput{
		ContestID: contestID,
		AuthorID:  authorID,
		Title:     "",
		Body:      "Some body",
	})
	require.Error(t, err)

	// 2. Successful creation
	ann, err := uc.CreateAnnouncement(ctx, &models.CreateContestAnnouncementInput{
		ContestID: contestID,
		AuthorID:  authorID,
		Title:     "Test Announcement",
		Body:      "Test announcement body with Markdown",
	})
	require.NoError(t, err)
	assert.Equal(t, "Test Announcement", ann.Title)
	assert.Equal(t, contestID, ann.ContestID)
	assert.Len(t, outboxRepo.events, 1)
	assert.Equal(t, models.OutboxEventContestAnnouncementCreated, outboxRepo.events[0].EventType)

	// 3. List announcements
	list, err := uc.ListAnnouncements(ctx, contestID, 1, 50)
	require.NoError(t, err)
	assert.Len(t, list.Announcements, 1)

	// 4. Delete announcement
	err = uc.DeleteAnnouncement(ctx, ann.ID, contestID)
	require.NoError(t, err)
	assert.Len(t, outboxRepo.events, 2)
	assert.Equal(t, models.OutboxEventContestAnnouncementDeleted, outboxRepo.events[1].EventType)

	listAfter, err := uc.ListAnnouncements(ctx, contestID, 1, 50)
	require.NoError(t, err)
	assert.Empty(t, listAfter.Announcements)
}
