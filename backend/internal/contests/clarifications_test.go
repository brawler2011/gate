package contests

import (
	"context"
	"testing"
	"time"

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

func (m *mockClarificationsRepo) CreateClarification(ctx context.Context, input *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error) {
	id := uuid.New()
	letter := "B"
	c := contestsmodel.ContestClarification{
		ID:            id,
		ContestID:     input.ContestID,
		ProblemID:     input.ProblemID,
		ProblemLetter: &letter,
		UserID:        input.UserID,
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

type mockContestsUCForClarifications struct {
	ContestsUC
	contest contestsmodel.Contest
}

func (m *mockContestsUCForClarifications) GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	return m.contest, nil
}

func TestClarificationsUseCase_Flow(t *testing.T) {
	ctx := context.Background()
	clarRepo := newMockClarificationsRepo()
	annRepo := newMockAnnouncementsRepo()
	outboxRepo := &mockOutboxRepo{}

	mockContests := &mockContestsUCForClarifications{
		contest: contestsmodel.Contest{
			ID: uuid.New(),
			Settings: map[string]interface{}{
				"allow_clarifications": true,
			},
		},
	}

	uc := NewClarificationsUseCase(mockContests, testTransactions[ContestEventsTxStore]{contestEventsTestStore{AnnouncementsRepo: annRepo, ClarificationsRepo: clarRepo, OutboxRepo: outboxRepo}})

	contestID := mockContests.contest.ID
	participantID := uuid.New()
	juryID := uuid.New()
	probID := uuid.New()

	// 1. Participant asks a question
	clar, err := uc.CreateClarification(ctx, &contestsmodel.CreateContestClarificationInput{
		ContestID: contestID,
		ProblemID: &probID,
		UserID:    participantID,
		Question:  "Are arrays 1-indexed in problem B?",
	})
	require.NoError(t, err)
	assert.Equal(t, contestsmodel.ClarificationStatusPending, clar.Status)
	assert.Equal(t, "Are arrays 1-indexed in problem B?", clar.Question)
	assert.Len(t, outboxRepo.events, 1)
	assert.Equal(t, eventdata.OutboxEventContestClarificationCreated, outboxRepo.events[0].EventType)

	// 4. Jury answers and publishes as announcement
	answered, err := uc.AnswerClarification(ctx, &contestsmodel.AnswerContestClarificationInput{
		ClarificationID:       clar.ID,
		ContestID:             contestID,
		Answer:                "Yes, arrays are 1-indexed.",
		AnsweredBy:            juryID,
		PublishAsAnnouncement: true,
		AnnouncementTitle:     "Уточнение по задаче B",
	})
	require.NoError(t, err)
	assert.Equal(t, contestsmodel.ClarificationStatusAnswered, answered.Status)
	assert.NotNil(t, answered.Answer)
	assert.Equal(t, "Yes, arrays are 1-indexed.", *answered.Answer)

	// Verify events: 1 (created) + 1 (answered) + 1 (announcement created) = 3 events
	assert.Len(t, outboxRepo.events, 3)
	assert.Equal(t, eventdata.OutboxEventContestClarificationAnswered, outboxRepo.events[1].EventType)
	assert.Equal(t, eventdata.OutboxEventContestAnnouncementCreated, outboxRepo.events[2].EventType)

	// Verify announcement was created in annRepo
	require.Len(t, annRepo.announcements, 1)
	for _, announcement := range annRepo.announcements {
		assert.Equal(t, contestID, announcement.ContestID)
		assert.Equal(t, "Уточнение по задаче B", announcement.Title)
	}
}
