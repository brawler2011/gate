package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type ContestsRepoMock struct {
	mock.Mock
	interfaces.ContestsRepo
}

func (m *ContestsRepoMock) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error) {
	args := m.Called(ctx, c)
	return args.Get(0).(models.ContestMember), args.Error(1)
}

func (m *ContestsRepoMock) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Contest), args.Error(1)
}

func (m *ContestsRepoMock) GetContestProblemResult(ctx context.Context, contestID, userID, problemID uuid.UUID) (*models.ContestProblemResult, error) {
	args := m.Called(ctx, contestID, userID, problemID)
	if res := args.Get(0); res != nil {
		return res.(*models.ContestProblemResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *ContestsRepoMock) UpsertContestProblemResult(ctx context.Context, params *models.UpsertContestProblemResultParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}

func (m *ContestsRepoMock) GetContestProblems(ctx context.Context, contestID uuid.UUID) ([]models.ContestProblem, error) {
	args := m.Called(ctx, contestID)
	return args.Get(0).([]models.ContestProblem), args.Error(1)
}

func (m *ContestsRepoMock) GetContestScoreboardFromStandings(ctx context.Context, contestID uuid.UUID) ([]models.ContestProblemResult, map[uuid.UUID]string, error) {
	args := m.Called(ctx, contestID)
	return args.Get(0).([]models.ContestProblemResult), args.Get(1).(map[uuid.UUID]string), args.Error(2)
}

func (m *ContestsRepoMock) GetSubmissionsForScoreboard(ctx context.Context, contestID, userID, problemID uuid.UUID) ([]models.SubmissionForScoreboard, error) {
	args := m.Called(ctx, contestID, userID, problemID)
	return args.Get(0).([]models.SubmissionForScoreboard), args.Error(1)
}

func TestProcessSubmissionResult_Rules(t *testing.T) {
	ctx := context.Background()

	contestID := uuid.New()
	userID := uuid.New()
	problemID := uuid.New()
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(2 * time.Hour)

	mockRepo := new(ContestsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo)

	mockRepo.On("GetContestMember", mock.Anything, &models.ContestPermissionGet{
		ContestId: contestID,
		UserId:    userID,
	}).Return(models.ContestMember{
		ContestID:   contestID,
		UserID:      userID,
		ContestRole: models.ContestRoleParticipant,
	}, nil)

	mockRepo.On("GetContest", mock.Anything, contestID).Return(models.Contest{
		ID:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"penalty_per_attempt": 20,
		},
	}, nil)

	// 1. First submission: WA at startTime + 10m
	subWA := &models.Submission{
		ID:        uuid.New(),
		ContestID: &contestID,
		CreatedBy: &userID,
		ProblemID: &problemID,
		State:     models.GotWA,
		CreatedAt: startTime.Add(10 * time.Minute),
	}

	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userID, problemID).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(10 * time.Minute)},
		}, nil).Once()

	mockRepo.On("UpsertContestProblemResult", mock.Anything, mock.MatchedBy(func(p *models.UpsertContestProblemResultParams) bool {
		return p.ContestID == contestID && p.UserID == userID && p.ProblemID == problemID &&
			!p.Solved && p.FailedAttempts == 1
	})).Return(nil).Once()

	err := uc.ProcessSubmissionResult(ctx, subWA)
	assert.NoError(t, err)

	// 2. Second submission: Accepted at startTime + 25m
	subAC := &models.Submission{
		ID:        uuid.New(),
		ContestID: &contestID,
		CreatedBy: &userID,
		ProblemID: &problemID,
		State:     models.Accepted,
		CreatedAt: startTime.Add(25 * time.Minute),
	}

	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userID, problemID).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(10 * time.Minute)},
			{State: models.Accepted, CreatedAt: startTime.Add(25 * time.Minute)},
		}, nil).Once()

	mockRepo.On("UpsertContestProblemResult", mock.Anything, mock.MatchedBy(func(p *models.UpsertContestProblemResultParams) bool {
		return p.ContestID == contestID && p.UserID == userID && p.ProblemID == problemID &&
			p.Solved && p.FailedAttempts == 1 && p.TimeMinutes != nil && *p.TimeMinutes == 25
	})).Return(nil).Once()

	err = uc.ProcessSubmissionResult(ctx, subAC)
	assert.NoError(t, err)

	// 3. Third submission: WA after AC -> should be IGNORED
	subPostAC := &models.Submission{
		ID:        uuid.New(),
		ContestID: &contestID,
		CreatedBy: &userID,
		ProblemID: &problemID,
		State:     models.GotWA,
		CreatedAt: startTime.Add(30 * time.Minute),
	}

	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userID, problemID).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(10 * time.Minute)},
			{State: models.Accepted, CreatedAt: startTime.Add(25 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(30 * time.Minute)},
		}, nil).Once()

	mockRepo.On("UpsertContestProblemResult", mock.Anything, mock.MatchedBy(func(p *models.UpsertContestProblemResultParams) bool {
		return p.ContestID == contestID && p.UserID == userID && p.ProblemID == problemID &&
			p.Solved && p.FailedAttempts == 1 && p.TimeMinutes != nil && *p.TimeMinutes == 25
	})).Return(nil).Once()

	err = uc.ProcessSubmissionResult(ctx, subPostAC)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestGetContestScoreboard_TieBreakerAndCustomPenalty(t *testing.T) {
	ctx := context.Background()

	contestID := uuid.New()
	userID1 := uuid.New()
	userID2 := uuid.New()
	probID := uuid.New()
	startTime := time.Now().Add(-1 * time.Hour)

	mockRepo := new(ContestsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo)

	mockRepo.On("GetContest", mock.Anything, contestID).Return(models.Contest{
		ID:        contestID,
		StartTime: &startTime,
		Settings: map[string]interface{}{
			"penalty_per_attempt": 15, // Custom 15 min penalty per attempt
		},
	}, nil)

	mockRepo.On("GetContestProblems", mock.Anything, contestID).Return([]models.ContestProblem{
		{
			ContestID: contestID,
			ProblemID: probID,
			Title:     "A+B",
			ShortName: "A",
			Ordinal:   1,
		},
	}, nil)

	acTime1 := startTime.Add(10 * time.Minute)
	mins1 := int32(10)

	acTime2 := startTime.Add(20 * time.Minute)
	mins2 := int32(20)

	results := []models.ContestProblemResult{
		{
			ContestID:      contestID,
			UserID:         userID1,
			ProblemID:      probID,
			Solved:         true,
			FailedAttempts: 2, // penalty = 2 * 15 = 30 min. Total = 10 + 30 = 40 min
			FirstACTime:    &acTime1,
			TimeMinutes:    &mins1,
		},
		{
			ContestID:      contestID,
			UserID:         userID2,
			ProblemID:      probID,
			Solved:         true,
			FailedAttempts: 0, // penalty = 0. Total = 20 + 0 = 20 min
			FirstACTime:    &acTime2,
			TimeMinutes:    &mins2,
		},
	}

	userMap := map[uuid.UUID]string{
		userID1: "alice",
		userID2: "bob",
	}

	mockRepo.On("GetContestScoreboardFromStandings", mock.Anything, contestID).
		Return(results, userMap, nil)

	sb, err := uc.GetContestScoreboard(ctx, contestID, uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, int32(15), sb.PenaltyPerAttempt)
	assert.Len(t, sb.Items, 2)

	// Bob solved 1 problem with total penalty 20
	// Alice solved 1 problem with total penalty 40
	// Bob should be Rank 1, Alice Rank 2
	assert.Equal(t, "bob", sb.Items[0].Username)
	assert.Equal(t, int32(20), sb.Items[0].TotalPenalty)
	assert.Equal(t, "alice", sb.Items[1].Username)
	assert.Equal(t, int32(40), sb.Items[1].TotalPenalty)

	mockRepo.AssertExpectations(t)
}
