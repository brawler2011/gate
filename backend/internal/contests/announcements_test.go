package contests

import (
	"context"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"testing"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAnnouncementsRepo struct {
	announcements map[uuid.UUID]contestsmodel.ContestAnnouncement
}

func newMockAnnouncementsRepo() *mockAnnouncementsRepo {
	return &mockAnnouncementsRepo{
		announcements: make(map[uuid.UUID]contestsmodel.ContestAnnouncement),
	}
}

func (m *mockAnnouncementsRepo) CreateAnnouncement(ctx context.Context, input *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error) {
	id := uuid.New()
	letter := "A"
	a := contestsmodel.ContestAnnouncement{
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

func (m *mockAnnouncementsRepo) GetAnnouncementByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestAnnouncement, error) {
	if a, ok := m.announcements[id]; ok {
		return &a, nil
	}
	return nil, nil
}

func (m *mockAnnouncementsRepo) DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error {
	delete(m.announcements, id)
	return nil
}

type mockOutboxRepo struct {
	events []*eventdata.CreateOutboxEventParams
}

func (m *mockOutboxRepo) CreateEvent(ctx context.Context, params *eventdata.CreateOutboxEventParams) error {
	m.events = append(m.events, params)
	return nil
}

func (m *mockOutboxRepo) DeleteOldEvents(ctx context.Context, retentionDays int32) error {
	return nil
}

func (m *mockOutboxRepo) PickEvents(ctx context.Context, limit int32, timeoutSec int32) ([]eventdata.OutboxEvent, error) {
	return nil, nil
}

func (m *mockOutboxRepo) MarkAsCompleted(ctx context.Context, id, leaseToken uuid.UUID) error {
	return nil
}

func (m *mockOutboxRepo) MarkAsFailed(ctx context.Context, id, leaseToken uuid.UUID, errorMsg string, permanent bool) error {
	return nil
}

func (m *mockOutboxRepo) ResetFailedToPending(ctx context.Context, maxRetries int32, retryDelaySec int32) error {
	return nil
}

func TestAnnouncementsUseCase_CreateAndDelete(t *testing.T) {
	ctx := context.Background()
	repo := newMockAnnouncementsRepo()
	outboxRepo := &mockOutboxRepo{}

	uc := NewAnnouncementsUseCase(testTransactions[ContestEventsTxStore]{contestEventsTestStore{AnnouncementsRepo: repo, OutboxRepo: outboxRepo}})

	contestID := uuid.New()
	authorID := uuid.New()
	ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: authorID, Role: accessmodel.UserRoleUser})

	// 1. Validation error on empty title
	_, err := uc.CreateAnnouncement(ctx, &contestsmodel.CreateContestAnnouncementInput{
		ContestID: contestID,
		AuthorID:  authorID,
		Title:     "",
		Body:      "Some body",
	})
	require.Error(t, err)

	// 2. Successful creation
	ann, err := uc.CreateAnnouncement(ctx, &contestsmodel.CreateContestAnnouncementInput{
		ContestID: contestID,
		AuthorID:  authorID,
		Title:     "Test Announcement",
		Body:      "Test announcement body with Markdown",
	})
	require.NoError(t, err)
	assert.Equal(t, "Test Announcement", ann.Title)
	assert.Equal(t, contestID, ann.ContestID)
	assert.Len(t, outboxRepo.events, 1)
	assert.Equal(t, eventdata.OutboxEventContestAnnouncementCreated, outboxRepo.events[0].EventType)

	// 3. Verify the created record
	require.Len(t, repo.announcements, 1)
	assert.Equal(t, *ann, repo.announcements[ann.ID])

	// 4. Delete announcement
	err = uc.DeleteAnnouncement(ctx, ann.ID, contestID)
	require.NoError(t, err)
	assert.Len(t, outboxRepo.events, 2)
	assert.Equal(t, eventdata.OutboxEventContestAnnouncementDeleted, outboxRepo.events[1].EventType)

	assert.Empty(t, repo.announcements)
}

func (r *mockOutboxRepo) Record(ctx context.Context, event eventdata.Event) error {
	return recordTestEvent(ctx, event, r.CreateEvent)
}
