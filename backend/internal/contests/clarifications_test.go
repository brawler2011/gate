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

type mockClarificationsRepo struct {
	clarifications map[uuid.UUID]contestsmodel.ContestClarification
}

func newMockClarificationsRepo() *mockClarificationsRepo {
	return &mockClarificationsRepo{
		clarifications: make(map[uuid.UUID]contestsmodel.ContestClarification),
	}
}

func (m *mockClarificationsRepo) CreateClarification(ctx context.Context, userID uuid.UUID, input *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error) {
	id := uuid.New()
	letter := "B"
	c := contestsmodel.ContestClarification{
		ID:            id,
		ContestID:     input.ContestID,
		ProblemID:     input.ProblemID,
		ProblemLetter: &letter,
		UserID:        userID,
		Username:      "participant1",
		Question:      input.Question,
		Status:        contestsmodel.ClarificationStatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	m.clarifications[id] = c
	return &c, nil
}

func (m *mockClarificationsRepo) GetClarificationByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestClarification, error) {
	if c, ok := m.clarifications[id]; ok {
		return &c, nil
	}
	return nil, nil
}

func (m *mockClarificationsRepo) AnswerClarification(ctx context.Context, id, contestID, answeredBy uuid.UUID, answer string) (*contestsmodel.ContestClarification, error) {
	c, ok := m.clarifications[id]
	if !ok {
		return nil, nil
	}
	juryUser := "jury_admin"
	now := time.Now()
	c.Answer = &answer
	c.AnsweredBy = &answeredBy
	c.AnsweredByUsername = &juryUser
	c.Status = contestsmodel.ClarificationStatusAnswered
	c.AnsweredAt = &now
	c.UpdatedAt = now
	m.clarifications[id] = c
	return &c, nil
}

func TestClarificationsUseCase_Flow(t *testing.T) {
	ctx := context.Background()
	clarRepo := newMockClarificationsRepo()
	annRepo := newMockAnnouncementsRepo()
	notices := &noticeLog{}

	contestID := uuid.New()
	participantID := uuid.New()
	juryID := uuid.New()
	contest := contestsmodel.Contest{ID: contestID, Visibility: contestsmodel.ContestVisibilityPrivate, Settings: contestsmodel.ContestSettings{AllowClarifications: true}}
	loader := &accesstest.Loader{Contests: map[uuid.UUID]access.ContestFacts{contestID: {Contest: contest, Now: time.Now()}}}
	loader.Grant(contestID, participantID, string(accessmodel.ContestParticipant))
	loader.Grant(contestID, juryID, string(accessmodel.ContestModerator))
	uc := NewClarificationsUseCase(testTransactions[ContestEventsTxStore]{contestEventsTestStore{Loader: loader, AnnouncementsRepo: annRepo, ClarificationsRepo: clarRepo, Writer: notices}}, nil, nil)

	probID := uuid.New()

	clar, err := uc.CreateClarification(accessmodel.WithUser(ctx, accessmodel.User{Id: participantID, Role: accessmodel.UserRoleUser}), &contestsmodel.CreateContestClarificationInput{
		ContestID: contestID,
		ProblemID: &probID,
		Question:  "Are arrays 1-indexed in problem B?",
	})
	require.NoError(t, err)
	assert.Equal(t, contestsmodel.ClarificationStatusPending, clar.Status)
	assert.Equal(t, participantID, clar.UserID, "the asker comes from the caller context")
	assert.Equal(t, "Are arrays 1-indexed in problem B?", clar.Question)
	assert.Equal(t, []eventdata.Notice{{Type: contestsmodel.ContestEventClarificationCreated, ID: clar.ID}}, notices.notices)

	answered, err := uc.AnswerClarification(accessmodel.WithUser(ctx, accessmodel.User{Id: juryID, Role: accessmodel.UserRoleUser}), &contestsmodel.AnswerContestClarificationInput{
		ClarificationID:       clar.ID,
		ContestID:             contestID,
		Answer:                "Yes, arrays are 1-indexed.",
		PublishAsAnnouncement: true,
		AnnouncementTitle:     "Уточнение по задаче B",
	})
	require.NoError(t, err)
	assert.Equal(t, contestsmodel.ClarificationStatusAnswered, answered.Status)
	assert.NotNil(t, answered.Answer)
	assert.Equal(t, "Yes, arrays are 1-indexed.", *answered.Answer)
	require.NotNil(t, answered.AnsweredBy)
	assert.Equal(t, juryID, *answered.AnsweredBy, "the answerer comes from the caller context")

	require.Len(t, notices.notices, 3)
	assert.Equal(t, eventdata.Notice{Type: contestsmodel.ContestEventClarificationAnswered, ID: clar.ID}, notices.notices[1])
	assert.Equal(t, contestsmodel.ContestEventAnnouncementCreated, notices.notices[2].Type)

	require.Len(t, annRepo.announcements, 1)
	for _, announcement := range annRepo.announcements {
		assert.Equal(t, contestID, announcement.ContestID)
		assert.Equal(t, "Уточнение по задаче B", announcement.Title)
		assert.Equal(t, juryID, announcement.AuthorID, "the published announcement is authored by the answerer")
	}
}
