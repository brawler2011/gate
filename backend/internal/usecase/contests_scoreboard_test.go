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
	"github.com/stretchr/testify/require"
)

type ContestsRepoMock struct {
	mock.Mock
	interfaces.ContestsRepo
}

func (m *ContestsRepoMock) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error) {
	args := m.Called(ctx, c)
	return args.Get(0).(models.ContestMember), args.Error(1) //nolint:wrapcheck
}

func (m *ContestsRepoMock) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Contest), args.Error(1) //nolint:wrapcheck
}

func (m *ContestsRepoMock) GetContestProblemResult(ctx context.Context, contestID, userID, problemID uuid.UUID) (*models.ContestProblemResult, error) {
	args := m.Called(ctx, contestID, userID, problemID)
	if res := args.Get(0); res != nil {
		return res.(*models.ContestProblemResult), args.Error(1) //nolint:wrapcheck
	}
	return nil, args.Error(1) //nolint:wrapcheck
}

func (m *ContestsRepoMock) UpsertContestProblemResult(ctx context.Context, params *models.UpsertContestProblemResultParams) error {
	args := m.Called(ctx, params)
	return args.Error(0) //nolint:wrapcheck
}

func (m *ContestsRepoMock) GetContestProblems(ctx context.Context, contestID uuid.UUID) ([]models.ContestProblem, error) {
	args := m.Called(ctx, contestID)
	return args.Get(0).([]models.ContestProblem), args.Error(1) //nolint:wrapcheck
}

func (m *ContestsRepoMock) GetContestScoreboardFromStandings(ctx context.Context, contestID uuid.UUID) ([]models.ContestProblemResult, map[uuid.UUID]string, error) {
	args := m.Called(ctx, contestID)
	return args.Get(0).([]models.ContestProblemResult), args.Get(1).(map[uuid.UUID]string), args.Error(2) //nolint:wrapcheck
}

func (m *ContestsRepoMock) GetSubmissionsForScoreboard(ctx context.Context, contestID, userID, problemID uuid.UUID) ([]models.SubmissionForScoreboard, error) {
	args := m.Called(ctx, contestID, userID, problemID)
	return args.Get(0).([]models.SubmissionForScoreboard), args.Error(1) //nolint:wrapcheck
}

func (m *ContestsRepoMock) CreateContestUserProblemBlock(ctx context.Context, params *models.CreateContestUserProblemBlockParams) error {
	args := m.Called(ctx, params)
	return args.Error(0) //nolint:wrapcheck
}

func (m *ContestsRepoMock) DeleteContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) error {
	args := m.Called(ctx, contestID, userID, problemID)
	return args.Error(0) //nolint:wrapcheck
}

func (m *ContestsRepoMock) GetContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) (*models.ContestUserProblemBlock, error) {
	args := m.Called(ctx, contestID, userID, problemID)
	if res := args.Get(0); res != nil {
		return res.(*models.ContestUserProblemBlock), args.Error(1) //nolint:wrapcheck
	}
	return nil, args.Error(1) //nolint:wrapcheck
}

func (m *ContestsRepoMock) ListContestUserProblemBlocks(ctx context.Context, contestID uuid.UUID, userID *uuid.UUID) ([]models.ContestUserProblemBlock, error) {
	args := m.Called(ctx, contestID, userID)
	return args.Get(0).([]models.ContestUserProblemBlock), args.Error(1) //nolint:wrapcheck
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
	require.NoError(t, err)

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
	require.NoError(t, err)

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
	require.NoError(t, err)

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

	sb, err := uc.GetContestScoreboard(ctx, contestID, uuid.New(), false)
	require.NoError(t, err)
	assert.Equal(t, int32(15), sb.PenaltyPerAttempt)
	assert.False(t, sb.IsFrozen)
	assert.Nil(t, sb.FreezeTime)
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

func TestGetContestScoreboard_Freeze_ParticipantView(t *testing.T) {
	ctx := context.Background()

	contestID := uuid.New()
	userAlice := uuid.New()
	userBob := uuid.New()
	userCharlie := uuid.New()
	userDave := uuid.New()

	probA := uuid.New()
	probB := uuid.New()

	// Contest started 2 hours ago, ends in 30 minutes.
	// Freeze duration = 60 minutes -> Freeze began 30 minutes ago.
	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(30 * time.Minute)
	freezeTime := endTime.Add(-60 * time.Minute) // 30 minutes ago

	mockRepo := new(ContestsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo)

	contest := models.Contest{
		ID:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           "auto",
			"freeze_duration_minutes": 60,
			"penalty_per_attempt":     20,
		},
	}

	mockRepo.On("GetContest", mock.Anything, contestID).Return(contest, nil)

	problems := []models.ContestProblem{
		{ContestID: contestID, ProblemID: probA, Title: "Problem A", ShortName: "A", Ordinal: 1},
		{ContestID: contestID, ProblemID: probB, Title: "Problem B", ShortName: "B", Ordinal: 2},
	}
	mockRepo.On("GetContestProblems", mock.Anything, contestID).Return(problems, nil)

	userMap := map[uuid.UUID]string{
		userAlice:   "alice",
		userBob:     "bob",
		userCharlie: "charlie",
		userDave:    "dave",
	}
	mockRepo.On("GetContestScoreboardFromStandings", mock.Anything, contestID).
		Return([]models.ContestProblemResult{}, userMap, nil)

	// --- Alice Submissions ---
	// Problem A: WA at T-90m (pre-freeze), AC at T-70m (pre-freeze).
	// During freeze: WA at T-20m, AC at T-10m.
	// Expected Prob A: Solved=true, FailedAttempts=1, PendingAttempts=2, TimeMinutes=50, Penalty=20
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userAlice, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(30 * time.Minute)}, // T-90m
			{State: models.Accepted, CreatedAt: startTime.Add(50 * time.Minute)}, // T-70m (AC at min 50)
			{State: models.GotWA, CreatedAt: freezeTime.Add(10 * time.Minute)},  // During freeze
			{State: models.Accepted, CreatedAt: freezeTime.Add(20 * time.Minute)}, // During freeze
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userAlice, probB).
		Return([]models.SubmissionForScoreboard{}, nil)

	// --- Bob Submissions ---
	// Problem A: 2 WA before freeze (T-80m, T-60m). AC during freeze (T-15m).
	// Expected Prob A: Solved=false, FailedAttempts=2, PendingAttempts=1, Penalty=0
	// Problem B: 1 WA during freeze (T-10m).
	// Expected Prob B: Solved=false, FailedAttempts=0, PendingAttempts=1, Penalty=0
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userBob, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(40 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(60 * time.Minute)},
			{State: models.Accepted, CreatedAt: freezeTime.Add(15 * time.Minute)}, // During freeze
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userBob, probB).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: freezeTime.Add(20 * time.Minute)}, // During freeze
		}, nil)

	// --- Charlie Submissions ---
	// Problem A: AC at T-80m (40m into contest). No freeze submissions.
	// Expected Prob A: Solved=true, FailedAttempts=0, PendingAttempts=0, TimeMinutes=40, Penalty=0
	// Problem B: 2 WA during freeze (first attempts to B).
	// Expected Prob B: Solved=false, FailedAttempts=0, PendingAttempts=2, Penalty=0
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userCharlie, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.Accepted, CreatedAt: startTime.Add(40 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userCharlie, probB).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: freezeTime.Add(5 * time.Minute)},
			{State: models.GotWA, CreatedAt: freezeTime.Add(10 * time.Minute)},
		}, nil)

	// --- Dave Submissions ---
	// Problem A: 1 AC during freeze (first attempt).
	// Expected Prob A: Solved=false, FailedAttempts=0, PendingAttempts=1, Penalty=0
	// Problem B: None.
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userDave, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.Accepted, CreatedAt: freezeTime.Add(5 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userDave, probB).
		Return([]models.SubmissionForScoreboard{}, nil)

	// Request scoreboard as participant (unfrozen=false)
	sb, err := uc.GetContestScoreboard(ctx, contestID, userAlice, false)
	require.NoError(t, err)
	assert.True(t, sb.IsFrozen)
	require.NotNil(t, sb.FreezeTime)
	assert.Equal(t, int32(20), sb.PenaltyPerAttempt)
	assert.Len(t, sb.Items, 4)

	// Check Rankings:
	// Charlie: 1 solved, 40 penalty
	// Alice: 1 solved, 70 penalty (50 mins + 20 penalty)
	// Bob: 0 solved, 0 penalty
	// Dave: 0 solved, 0 penalty
	assert.Equal(t, "charlie", sb.Items[0].Username)
	assert.Equal(t, int32(1), sb.Items[0].ProblemsSolved)
	assert.Equal(t, int32(40), sb.Items[0].TotalPenalty)

	assert.Equal(t, "alice", sb.Items[1].Username)
	assert.Equal(t, int32(1), sb.Items[1].ProblemsSolved)
	assert.Equal(t, int32(70), sb.Items[1].TotalPenalty)

	assert.Equal(t, "bob", sb.Items[2].Username)
	assert.Equal(t, int32(0), sb.Items[2].ProblemsSolved)
	assert.Equal(t, int32(0), sb.Items[2].TotalPenalty)

	assert.Equal(t, "dave", sb.Items[3].Username)
	assert.Equal(t, int32(0), sb.Items[3].ProblemsSolved)
	assert.Equal(t, int32(0), sb.Items[3].TotalPenalty)

	// Check Alice's problem results
	aliceResults := make(map[uuid.UUID]models.ContestProblemResult)
	for _, r := range sb.Items[1].ProblemResults {
		aliceResults[r.ProblemID] = r
	}
	resAliceA, ok := aliceResults[probA]
	require.True(t, ok)
	assert.True(t, resAliceA.Solved)
	assert.Equal(t, int32(1), resAliceA.FailedAttempts)
	assert.Equal(t, int32(2), resAliceA.PendingAttempts) // +1 2?
	assert.Equal(t, int32(20), resAliceA.Penalty)
	assert.Equal(t, int32(50), *resAliceA.TimeMinutes)

	// Check Bob's problem results
	bobResults := make(map[uuid.UUID]models.ContestProblemResult)
	for _, r := range sb.Items[2].ProblemResults {
		bobResults[r.ProblemID] = r
	}
	resBobA, ok := bobResults[probA]
	require.True(t, ok)
	assert.False(t, resBobA.Solved)
	assert.Equal(t, int32(2), resBobA.FailedAttempts)
	assert.Equal(t, int32(1), resBobA.PendingAttempts) // -2 1?

	resBobB, ok := bobResults[probB]
	require.True(t, ok)
	assert.False(t, resBobB.Solved)
	assert.Equal(t, int32(0), resBobB.FailedAttempts)
	assert.Equal(t, int32(1), resBobB.PendingAttempts) // ?1

	// Check Dave's problem results
	daveResults := make(map[uuid.UUID]models.ContestProblemResult)
	for _, r := range sb.Items[3].ProblemResults {
		daveResults[r.ProblemID] = r
	}
	resDaveA, ok := daveResults[probA]
	require.True(t, ok)
	assert.False(t, resDaveA.Solved)
	assert.Equal(t, int32(0), resDaveA.FailedAttempts)
	assert.Equal(t, int32(1), resDaveA.PendingAttempts) // ?1

	mockRepo.AssertExpectations(t)
}

func TestGetContestScoreboard_Freeze_ManagerUnfrozenView(t *testing.T) {
	ctx := context.Background()

	contestID := uuid.New()
	userAlice := uuid.New()
	userBob := uuid.New()
	probA := uuid.New()

	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(30 * time.Minute)

	mockRepo := new(ContestsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo)

	contest := models.Contest{
		ID:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           "auto",
			"freeze_duration_minutes": 60,
			"penalty_per_attempt":     20,
		},
	}

	mockRepo.On("GetContest", mock.Anything, contestID).Return(contest, nil)

	problems := []models.ContestProblem{
		{ContestID: contestID, ProblemID: probA, Title: "Problem A", ShortName: "A", Ordinal: 1},
	}
	mockRepo.On("GetContestProblems", mock.Anything, contestID).Return(problems, nil)

	minsAlice := int32(50)
	minsBob := int32(105)

	// Live standings in database
	liveResults := []models.ContestProblemResult{
		{
			ContestID:      contestID,
			UserID:         userAlice,
			ProblemID:      probA,
			Solved:         true,
			FailedAttempts: 1,
			TimeMinutes:    &minsAlice,
		},
		{
			ContestID:      contestID,
			UserID:         userBob,
			ProblemID:      probA,
			Solved:         true,
			FailedAttempts: 2,
			TimeMinutes:    &minsBob,
		},
	}

	userMap := map[uuid.UUID]string{
		userAlice: "alice",
		userBob:   "bob",
	}

	mockRepo.On("GetContestScoreboardFromStandings", mock.Anything, contestID).
		Return(liveResults, userMap, nil)

	// Manager requests live scoreboard with unfrozen=true
	sb, err := uc.GetContestScoreboard(ctx, contestID, uuid.New(), true)
	require.NoError(t, err)

	// Response preserves freeze metadata
	assert.True(t, sb.IsFrozen)
	require.NotNil(t, sb.FreezeTime)

	// Scoreboard returns real live standings
	assert.Len(t, sb.Items, 2)
	assert.Equal(t, "alice", sb.Items[0].Username)
	assert.Equal(t, int32(1), sb.Items[0].ProblemsSolved)
	assert.Equal(t, int32(70), sb.Items[0].TotalPenalty) // 50 + 20

	assert.Equal(t, "bob", sb.Items[1].Username)
	assert.Equal(t, int32(1), sb.Items[1].ProblemsSolved)
	assert.Equal(t, int32(145), sb.Items[1].TotalPenalty) // 105 + 40

	// PendingAttempts should be 0 in unfrozen view
	assert.Equal(t, int32(0), sb.Items[0].ProblemResults[0].PendingAttempts)
	assert.Equal(t, int32(0), sb.Items[1].ProblemResults[0].PendingAttempts)

	mockRepo.AssertExpectations(t)
}

func TestGetContestScoreboard_ManualUnfrozenOverride(t *testing.T) {
	ctx := context.Background()

	contestID := uuid.New()
	userID := uuid.New()
	probA := uuid.New()

	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(30 * time.Minute)

	mockRepo := new(ContestsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo)

	// freeze_status is manually "unfrozen"
	contest := models.Contest{
		ID:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           "unfrozen",
			"freeze_duration_minutes": 60,
			"penalty_per_attempt":     20,
		},
	}

	mockRepo.On("GetContest", mock.Anything, contestID).Return(contest, nil)

	problems := []models.ContestProblem{
		{ContestID: contestID, ProblemID: probA, Title: "Problem A", ShortName: "A", Ordinal: 1},
	}
	mockRepo.On("GetContestProblems", mock.Anything, contestID).Return(problems, nil)

	userMap := map[uuid.UUID]string{userID: "alice"}
	mockRepo.On("GetContestScoreboardFromStandings", mock.Anything, contestID).
		Return([]models.ContestProblemResult{}, userMap, nil)

	sb, err := uc.GetContestScoreboard(ctx, contestID, userID, false)
	require.NoError(t, err)

	assert.False(t, sb.IsFrozen)
	mockRepo.AssertExpectations(t)
}

func TestProcessSubmissionResult_DisqualifiedCountsAsFailedAttempt(t *testing.T) {
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

	// Submissions: 1 Disqualified submission, followed by 1 Accepted submission
	subDQ := models.SubmissionForScoreboard{
		State:     models.Disqualified,
		CreatedAt: startTime.Add(10 * time.Minute),
	}
	subAC := models.SubmissionForScoreboard{
		State:     models.Accepted,
		CreatedAt: startTime.Add(25 * time.Minute),
	}

	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userID, problemID).
		Return([]models.SubmissionForScoreboard{subDQ, subAC}, nil)

	timeMins := int32(25)
	firstAC := subAC.CreatedAt
	mockRepo.On("UpsertContestProblemResult", mock.Anything, &models.UpsertContestProblemResultParams{
		ContestID:      contestID,
		UserID:         userID,
		ProblemID:      problemID,
		Solved:         true,
		FailedAttempts: 1, // 1 Disqualified counts as 1 failed attempt
		FirstACTime:    &firstAC,
		TimeMinutes:    &timeMins,
	}).Return(nil)

	err := uc.ProcessSubmissionResult(ctx, &models.Submission{
		ContestID: &contestID,
		CreatedBy: &userID,
		ProblemID: &problemID,
		State:     models.Accepted,
		CreatedAt: subAC.CreatedAt,
	})
	require.NoError(t, err)

	mockRepo.AssertExpectations(t)
}
