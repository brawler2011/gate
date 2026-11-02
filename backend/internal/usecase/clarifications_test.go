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

type mockClarificationsRepo struct {
	clarifications map[uuid.UUID]models.ContestClarification
}

func newMockClarificationsRepo() *mockClarificationsRepo {
	return &mockClarificationsRepo{
		clarifications: make(map[uuid.UUID]models.ContestClarification),
	}
}

func (m *mockClarificationsRepo) WithTx(tx pgx.Tx) interfaces.ClarificationsRepo {
	return m
}

func (m *mockClarificationsRepo) CreateClarification(ctx context.Context, input *models.CreateContestClarificationInput) (*models.ContestClarification, error) {
	id := uuid.New()
	letter := "B"
	c := models.ContestClarification{
		ID:            id,
		ContestID:     input.ContestID,
		ProblemID:     input.ProblemID,
		ProblemLetter: &letter,
		UserID:        input.UserID,
		Username:      "participant1",
		Question:      input.Question,
		Status:        models.ClarificationStatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	m.clarifications[id] = c
	return &c, nil
}

func (m *mockClarificationsRepo) GetClarificationByID(ctx context.Context, id uuid.UUID) (*models.ContestClarification, error) {
	if c, ok := m.clarifications[id]; ok {
		return &c, nil
	}
	return nil, nil
}

func (m *mockClarificationsRepo) AnswerClarification(ctx context.Context, id, contestID, answeredBy uuid.UUID, answer string) (*models.ContestClarification, error) {
	c, ok := m.clarifications[id]
	if !ok {
		return nil, nil
	}
	juryUser := "jury_admin"
	now := time.Now()
	c.Answer = &answer
	c.AnsweredBy = &answeredBy
	c.AnsweredByUsername = &juryUser
	c.Status = models.ClarificationStatusAnswered
	c.AnsweredAt = &now
	c.UpdatedAt = now
	m.clarifications[id] = c
	return &c, nil
}

type mockContestsUCForClarifications struct {
	interfaces.ContestsUC
	contest models.Contest
}

func (m *mockContestsUCForClarifications) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	return m.contest, nil
}

func TestClarificationsUseCase_Flow(t *testing.T) {
	ctx := context.Background()
	clarRepo := newMockClarificationsRepo()
	annRepo := newMockAnnouncementsRepo()
	outboxRepo := &mockOutboxRepo{}
	transactor := &mockTransactor{}

	mockContests := &mockContestsUCForClarifications{
		contest: models.Contest{
			ID: uuid.New(),
			Settings: map[string]interface{}{
				"allow_clarifications": true,
			},
		},
	}

	uc := NewClarificationsUseCase(clarRepo, annRepo, mockContests, outboxRepo, transactor)

	contestID := mockContests.contest.ID
	participantID := uuid.New()
	juryID := uuid.New()
	probID := uuid.New()

	// 1. Participant asks a question
	clar, err := uc.CreateClarification(ctx, &models.CreateContestClarificationInput{
		ContestID: contestID,
		ProblemID: &probID,
		UserID:    participantID,
		Question:  "Are arrays 1-indexed in problem B?",
	})
	require.NoError(t, err)
	assert.Equal(t, models.ClarificationStatusPending, clar.Status)
	assert.Equal(t, "Are arrays 1-indexed in problem B?", clar.Question)
	assert.Len(t, outboxRepo.events, 1)
	assert.Equal(t, models.OutboxEventContestClarificationCreated, outboxRepo.events[0].EventType)

	// 4. Jury answers and publishes as announcement
	answered, err := uc.AnswerClarification(ctx, &models.AnswerContestClarificationInput{
		ClarificationID:       clar.ID,
		ContestID:             contestID,
		Answer:                "Yes, arrays are 1-indexed.",
		AnsweredBy:            juryID,
		PublishAsAnnouncement: true,
		AnnouncementTitle:     "Уточнение по задаче B",
	})
	require.NoError(t, err)
	assert.Equal(t, models.ClarificationStatusAnswered, answered.Status)
	assert.NotNil(t, answered.Answer)
	assert.Equal(t, "Yes, arrays are 1-indexed.", *answered.Answer)

	// Verify events: 1 (created) + 1 (answered) + 1 (announcement created) = 3 events
	assert.Len(t, outboxRepo.events, 3)
	assert.Equal(t, models.OutboxEventContestClarificationAnswered, outboxRepo.events[1].EventType)
	assert.Equal(t, models.OutboxEventContestAnnouncementCreated, outboxRepo.events[2].EventType)

	// Verify announcement was created in annRepo
	require.Len(t, annRepo.announcements, 1)
	for _, announcement := range annRepo.announcements {
		assert.Equal(t, contestID, announcement.ContestID)
		assert.Equal(t, "Уточнение по задаче B", announcement.Title)
	}
}
