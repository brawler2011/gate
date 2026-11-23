package contests

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

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

func (m *mockAnnouncementsRepo) CreateAnnouncement(ctx context.Context, authorID uuid.UUID, input *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error) {
	id := uuid.New()
	letter := "A"
	a := contestsmodel.ContestAnnouncement{
		ID:             id,
		ContestID:      input.ContestID,
		ProblemID:      input.ProblemID,
		ProblemTitle:   nil,
		ProblemLetter:  &letter,
		AuthorID:       authorID,
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

func TestAnnouncementsUseCase_CreateAndDelete(t *testing.T) {
	ctx := context.Background()
	repo := newMockAnnouncementsRepo()
	notices := &noticeLog{}

	contestID := uuid.New()
	authorID := uuid.New()
	loader := &accesstest.Loader{Contests: map[uuid.UUID]access.ContestFacts{contestID: {Contest: contestsmodel.Contest{ID: contestID}, Now: time.Now()}}}
	loader.Grant(contestID, authorID, string(accessmodel.ContestModerator))
	uc := NewAnnouncementsUseCase(testTransactions[ContestEventsTxStore]{contestEventsTestStore{Loader: loader, AnnouncementsRepo: repo, Writer: notices}}, nil, nil)

	ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: authorID, Role: accessmodel.UserRoleUser})

	_, err := uc.CreateAnnouncement(ctx, &contestsmodel.CreateContestAnnouncementInput{
		ContestID: contestID,
		Title:     "",
		Body:      "Some body",
	})
	require.Error(t, err)

	ann, err := uc.CreateAnnouncement(ctx, &contestsmodel.CreateContestAnnouncementInput{
		ContestID: contestID,
		Title:     "Test Announcement",
		Body:      "Test announcement body with Markdown",
	})
	require.NoError(t, err)
	assert.Equal(t, "Test Announcement", ann.Title)
	assert.Equal(t, contestID, ann.ContestID)
	assert.Equal(t, []eventdata.Notice{{Type: contestsmodel.ContestEventAnnouncementCreated, ID: ann.ID}}, notices.notices)

	require.Len(t, repo.announcements, 1)
	assert.Equal(t, *ann, repo.announcements[ann.ID])

	err = uc.DeleteAnnouncement(ctx, ann.ID, contestID)
	require.NoError(t, err)
	assert.Equal(t, eventdata.Notice{Type: contestsmodel.ContestEventAnnouncementDeleted, ID: ann.ID, ContestID: &contestID}, notices.notices[1])

	assert.Empty(t, repo.announcements)
}
