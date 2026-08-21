package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetContestScoreboard_StressAndBoundaryRules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	contestID := uuid.New()
	startTime := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC) // 120 minutes contest
	freezeTime := endTime.Add(-60 * time.Minute)             // Freeze at 11:00:00 (60m into contest)

	probA := uuid.New()
	probB := uuid.New()
	probC := uuid.New()
	probD := uuid.New()
	probE := uuid.New()

	problems := []models.ContestProblem{
		{ContestID: contestID, ProblemID: probA, Title: "Problem A", ShortName: "A", Ordinal: 1},
		{ContestID: contestID, ProblemID: probB, Title: "Problem B", ShortName: "B", Ordinal: 2},
		{ContestID: contestID, ProblemID: probC, Title: "Problem C", ShortName: "C", Ordinal: 3},
		{ContestID: contestID, ProblemID: probD, Title: "Problem D", ShortName: "D", Ordinal: 4},
		{ContestID: contestID, ProblemID: probE, Title: "Problem E", ShortName: "E", Ordinal: 5},
	}

	userAlice := uuid.New()
	userBob := uuid.New()
	userCharlie := uuid.New()
	userDave := uuid.New()
	userEve := uuid.New()
	userFrank := uuid.New()

	userMap := map[uuid.UUID]string{
		userAlice:   "alice",
		userBob:     "bob",
		userCharlie: "charlie",
		userDave:    "dave",
		userEve:     "eve",
		userFrank:   "frank",
	}

	contestAuto := models.Contest{
		ID:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           models.FreezeStatusAuto,
			"freeze_duration_minutes": 60,
			"penalty_per_attempt":     20,
		},
	}

	mockRepo := new(ContestsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo, nil)

	mockRepo.On("GetContest", mock.Anything, contestID).Return(contestAuto, nil)
	mockRepo.On("GetContestProblems", mock.Anything, contestID).Return(problems, nil)
	mockRepo.On("GetContestScoreboardFromStandings", mock.Anything, contestID).
		Return([]models.ContestProblemResult{}, userMap, nil)

	// ==========================================
	// Alice:
	// Prob A: WA at 10m, AC at 15m. During freeze: WA at 70m.
	// Prob B: AC at 20m.
	// Prob C: WA at 30m, WA at 40m, AC at 45m.
	// Prob D: 3 WA during freeze (75m, 80m, 85m), 1 AC during freeze (90m).
	// Prob E: 1 WA during freeze (95m).
	// ==========================================
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userAlice, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(10 * time.Minute)},
			{State: models.Accepted, CreatedAt: startTime.Add(15 * time.Minute)},
			{State: models.GotWA, CreatedAt: freezeTime.Add(10 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userAlice, probB).
		Return([]models.SubmissionForScoreboard{
			{State: models.Accepted, CreatedAt: startTime.Add(20 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userAlice, probC).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(30 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(40 * time.Minute)},
			{State: models.Accepted, CreatedAt: startTime.Add(45 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userAlice, probD).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: freezeTime.Add(15 * time.Minute)},
			{State: models.GotWA, CreatedAt: freezeTime.Add(20 * time.Minute)},
			{State: models.GotWA, CreatedAt: freezeTime.Add(25 * time.Minute)},
			{State: models.Accepted, CreatedAt: freezeTime.Add(30 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userAlice, probE).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: freezeTime.Add(35 * time.Minute)},
		}, nil)

	// ==========================================
	// Bob (Freeze Comeback - solves all 4 other problems in freeze):
	// Prob A: AC at 15m.
	// Prob B: 3 WA before freeze (20m, 30m, 40m). AC during freeze (65m).
	// Prob C: AC during freeze (75m).
	// Prob D: AC during freeze (85m).
	// Prob E: AC during freeze (95m).
	// ==========================================
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userBob, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.Accepted, CreatedAt: startTime.Add(15 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userBob, probB).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(20 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(30 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(40 * time.Minute)},
			{State: models.Accepted, CreatedAt: freezeTime.Add(5 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userBob, probC).
		Return([]models.SubmissionForScoreboard{
			{State: models.Accepted, CreatedAt: freezeTime.Add(15 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userBob, probD).
		Return([]models.SubmissionForScoreboard{
			{State: models.Accepted, CreatedAt: freezeTime.Add(25 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userBob, probE).
		Return([]models.SubmissionForScoreboard{
			{State: models.Accepted, CreatedAt: freezeTime.Add(35 * time.Minute)},
		}, nil)

	// ==========================================
	// Charlie (No solved problems):
	// Prob A: 4 WA before freeze (10m, 20m, 30m, 40m), 2 WA during freeze (65m, 70m).
	// Prob B: 1 WA before freeze (50m).
	// Probs C, D, E: None.
	// ==========================================
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userCharlie, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(10 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(20 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(30 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(40 * time.Minute)},
			{State: models.GotWA, CreatedAt: freezeTime.Add(5 * time.Minute)},
			{State: models.GotWA, CreatedAt: freezeTime.Add(10 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userCharlie, probB).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(50 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userCharlie, probC).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userCharlie, probD).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userCharlie, probE).
		Return([]models.SubmissionForScoreboard{}, nil)

	// ==========================================
	// Dave (Submissions only during freeze):
	// Prob A: 1 WA at 70m, 1 AC at 80m.
	// Prob B: 2 WA at 85m, 90m.
	// Probs C, D, E: None.
	// ==========================================
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userDave, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: freezeTime.Add(10 * time.Minute)},
			{State: models.Accepted, CreatedAt: freezeTime.Add(20 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userDave, probB).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: freezeTime.Add(25 * time.Minute)},
			{State: models.GotWA, CreatedAt: freezeTime.Add(30 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userDave, probC).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userDave, probD).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userDave, probE).
		Return([]models.SubmissionForScoreboard{}, nil)

	// ==========================================
	// Eve (Pre-freeze AC + post-AC submissions):
	// Prob A: AC at 5m. Then at 12m WA, at 18m WA (pre-freeze). Then at 70m AC, at 75m WA (post-freeze).
	// Probs B, C, D, E: None.
	// ==========================================
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userEve, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.Accepted, CreatedAt: startTime.Add(5 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(12 * time.Minute)},
			{State: models.GotWA, CreatedAt: startTime.Add(18 * time.Minute)},
			{State: models.Accepted, CreatedAt: freezeTime.Add(10 * time.Minute)},
			{State: models.GotWA, CreatedAt: freezeTime.Add(15 * time.Minute)},
		}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userEve, probB).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userEve, probC).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userEve, probD).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userEve, probE).
		Return([]models.SubmissionForScoreboard{}, nil)

	// ==========================================
	// Frank (Zero submissions to anything):
	// ==========================================
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userFrank, probA).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userFrank, probB).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userFrank, probC).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userFrank, probD).
		Return([]models.SubmissionForScoreboard{}, nil)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userFrank, probE).
		Return([]models.SubmissionForScoreboard{}, nil)

	// Execute scoreboard call for participant
	sb, err := uc.GetContestScoreboard(ctx, contestID, userAlice, false)
	require.NoError(t, err)
	assert.True(t, sb.IsFrozen)
	require.NotNil(t, sb.FreezeTime)
	assert.Equal(t, freezeTime, *sb.FreezeTime)
	assert.Len(t, sb.Items, 6)

	// -------------------------------------------------------------
	// Verify Alice:
	// Solved: 3 (Prob A: 15m + 20pen = 35; Prob B: 20m + 0 = 20; Prob C: 45m + 40pen = 85)
	// Total Penalty: 35 + 20 + 85 = 140 min.
	// -------------------------------------------------------------
	itemMap := make(map[string]models.ScoreboardItem)
	for _, it := range sb.Items {
		itemMap[it.Username] = it
	}

	alice := itemMap["alice"]
	assert.Equal(t, int32(3), alice.ProblemsSolved)
	assert.Equal(t, int32(140), alice.TotalPenalty)

	aliceResults := make(map[uuid.UUID]models.ContestProblemResult)
	for _, r := range alice.ProblemResults {
		aliceResults[r.ProblemID] = r
	}
	assert.True(t, aliceResults[probA].Solved)
	assert.Equal(t, int32(1), aliceResults[probA].FailedAttempts)
	assert.Equal(t, int32(1), aliceResults[probA].PendingAttempts) // WA during freeze
	assert.Equal(t, int32(20), aliceResults[probA].Penalty)

	assert.True(t, aliceResults[probB].Solved)
	assert.Equal(t, int32(0), aliceResults[probB].FailedAttempts)
	assert.Equal(t, int32(0), aliceResults[probB].PendingAttempts)

	assert.True(t, aliceResults[probC].Solved)
	assert.Equal(t, int32(2), aliceResults[probC].FailedAttempts)
	assert.Equal(t, int32(0), aliceResults[probC].PendingAttempts)
	assert.Equal(t, int32(40), aliceResults[probC].Penalty)

	assert.False(t, aliceResults[probD].Solved)
	assert.Equal(t, int32(0), aliceResults[probD].FailedAttempts)
	assert.Equal(t, int32(4), aliceResults[probD].PendingAttempts) // 3 WA + 1 AC during freeze

	assert.False(t, aliceResults[probE].Solved)
	assert.Equal(t, int32(0), aliceResults[probE].FailedAttempts)
	assert.Equal(t, int32(1), aliceResults[probE].PendingAttempts) // 1 WA during freeze

	// -------------------------------------------------------------
	// Verify Bob:
	// Solved: 1 (Prob A: 15m + 0 = 15).
	// Total Penalty: 15 min. (Freeze comeback hidden from scoreboard!)
	// -------------------------------------------------------------
	bob := itemMap["bob"]
	assert.Equal(t, int32(1), bob.ProblemsSolved)
	assert.Equal(t, int32(15), bob.TotalPenalty)

	bobResults := make(map[uuid.UUID]models.ContestProblemResult)
	for _, r := range bob.ProblemResults {
		bobResults[r.ProblemID] = r
	}
	assert.True(t, bobResults[probA].Solved)
	assert.Equal(t, int32(0), bobResults[probA].FailedAttempts)
	assert.Equal(t, int32(0), bobResults[probA].PendingAttempts)

	assert.False(t, bobResults[probB].Solved)
	assert.Equal(t, int32(3), bobResults[probB].FailedAttempts)
	assert.Equal(t, int32(1), bobResults[probB].PendingAttempts) // AC during freeze

	assert.False(t, bobResults[probC].Solved)
	assert.Equal(t, int32(0), bobResults[probC].FailedAttempts)
	assert.Equal(t, int32(1), bobResults[probC].PendingAttempts)

	assert.False(t, bobResults[probD].Solved)
	assert.Equal(t, int32(0), bobResults[probD].FailedAttempts)
	assert.Equal(t, int32(1), bobResults[probD].PendingAttempts)

	assert.False(t, bobResults[probE].Solved)
	assert.Equal(t, int32(0), bobResults[probE].FailedAttempts)
	assert.Equal(t, int32(1), bobResults[probE].PendingAttempts)

	// -------------------------------------------------------------
	// Verify Eve:
	// Prob A: Solved at 5m with 0 fails before AC. Post-AC pre-freeze WA are ignored.
	// 2 freeze attempts become PendingAttempts=2.
	// Total Solved: 1, Total Penalty: 5.
	// -------------------------------------------------------------
	eve := itemMap["eve"]
	assert.Equal(t, int32(1), eve.ProblemsSolved)
	assert.Equal(t, int32(5), eve.TotalPenalty)
	eveResults := make(map[uuid.UUID]models.ContestProblemResult)
	for _, r := range eve.ProblemResults {
		eveResults[r.ProblemID] = r
	}
	assert.True(t, eveResults[probA].Solved)
	assert.Equal(t, int32(0), eveResults[probA].FailedAttempts)
	assert.Equal(t, int32(2), eveResults[probA].PendingAttempts)

	// -------------------------------------------------------------
	// Verify Charlie:
	// Solved: 0, Total Penalty: 0.
	// Prob A: FailedAttempts=4, PendingAttempts=2.
	// Prob B: FailedAttempts=1, PendingAttempts=0.
	// -------------------------------------------------------------
	charlie := itemMap["charlie"]
	assert.Equal(t, int32(0), charlie.ProblemsSolved)
	assert.Equal(t, int32(0), charlie.TotalPenalty)

	// -------------------------------------------------------------
	// Verify Dave:
	// Solved: 0, Total Penalty: 0.
	// Prob A: FailedAttempts=0, PendingAttempts=2.
	// Prob B: FailedAttempts=0, PendingAttempts=2.
	// -------------------------------------------------------------
	dave := itemMap["dave"]
	assert.Equal(t, int32(0), dave.ProblemsSolved)
	assert.Equal(t, int32(0), dave.TotalPenalty)

	// -------------------------------------------------------------
	// Verify Frank:
	// Solved: 0, Total Penalty: 0, ProblemResults empty.
	// -------------------------------------------------------------
	frank := itemMap["frank"]
	assert.Equal(t, int32(0), frank.ProblemsSolved)
	assert.Equal(t, int32(0), frank.TotalPenalty)
	assert.Empty(t, frank.ProblemResults)

	// -------------------------------------------------------------
	// Verify Strict Rank Ordering in Frozen View:
	// Rank 1: Alice (3 solved, penalty 140)
	// Rank 2: Eve   (1 solved, penalty 5)
	// Rank 3: Bob   (1 solved, penalty 15)
	// Rank 4: Charlie (0 solved, penalty 0, alphabetical)
	// Rank 5: Dave    (0 solved, penalty 0, alphabetical)
	// Rank 6: Frank   (0 solved, penalty 0, alphabetical)
	// -------------------------------------------------------------
	assert.Equal(t, "alice", sb.Items[0].Username)
	assert.Equal(t, "eve", sb.Items[1].Username)
	assert.Equal(t, "bob", sb.Items[2].Username)
	assert.Equal(t, "charlie", sb.Items[3].Username)
	assert.Equal(t, "dave", sb.Items[4].Username)
	assert.Equal(t, "frank", sb.Items[5].Username)
}

func TestGetContestScoreboard_BoundarySubmissionsAndFiltering(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	contestID := uuid.New()
	startTime := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	freezeTime := endTime.Add(-60 * time.Minute) // 11:00:00

	probA := uuid.New()
	probB := uuid.New()

	problems := []models.ContestProblem{
		{ContestID: contestID, ProblemID: probA, Title: "A", ShortName: "A", Ordinal: 1},
		{ContestID: contestID, ProblemID: probB, Title: "B", ShortName: "B", Ordinal: 2},
	}

	userTest := uuid.New()
	userMap := map[uuid.UUID]string{userTest: "testuser"}

	contest := models.Contest{
		ID:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           models.FreezeStatusAuto,
			"freeze_duration_minutes": 60,
			"penalty_per_attempt":     20,
		},
	}

	mockRepo := new(ContestsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo, nil)

	mockRepo.On("GetContest", mock.Anything, contestID).Return(contest, nil)
	mockRepo.On("GetContestProblems", mock.Anything, contestID).Return(problems, nil)
	mockRepo.On("GetContestScoreboardFromStandings", mock.Anything, contestID).
		Return([]models.ContestProblemResult{}, userMap, nil)

	// Problem A:
	// - 1ns before StartTime -> IGNORED (out of contest)
	// - Exactly at StartTime -> WA (valid pre-freeze attempt)
	// - 1ns before FreezeTime -> AC (valid pre-freeze solve at min 59)
	// - Exactly at FreezeTime -> WA (FROZEN pending attempt)
	// - 1ns after FreezeTime -> WA (FROZEN pending attempt)
	// - CE during freeze -> IGNORED
	// - Saved during freeze -> IGNORED
	// - Exactly at EndTime -> WA (FROZEN pending attempt)
	// - 1ns after EndTime -> IGNORED (out of contest)
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userTest, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.Accepted, CreatedAt: startTime.Add(-1 * time.Nanosecond)}, // Ignored
			{State: models.GotWA, CreatedAt: startTime},                              // Failed 1
			{State: models.Accepted, CreatedAt: freezeTime.Add(-1 * time.Nanosecond)},// Solved at min 59
			{State: models.GotWA, CreatedAt: freezeTime},                             // Pending 1
			{State: models.GotWA, CreatedAt: freezeTime.Add(1 * time.Nanosecond)},    // Pending 2
			{State: models.GotCE, CreatedAt: freezeTime.Add(10 * time.Minute)},       // CE Ignored
			{State: models.Saved, CreatedAt: freezeTime.Add(15 * time.Minute)},       // Saved Ignored
			{State: models.GotWA, CreatedAt: endTime},                                // Pending 3
			{State: models.Accepted, CreatedAt: endTime.Add(1 * time.Nanosecond)},    // Ignored
		}, nil)

	// Problem B: All error verdict types before freeze (WA, TL, ML, RE, PE) -> all count towards failedAttempts
	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userTest, probB).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(5 * time.Minute)},
			{State: models.GotTL, CreatedAt: startTime.Add(10 * time.Minute)},
			{State: models.GotML, CreatedAt: startTime.Add(15 * time.Minute)},
			{State: models.GotRE, CreatedAt: startTime.Add(20 * time.Minute)},
			{State: models.GotPE, CreatedAt: startTime.Add(25 * time.Minute)},
		}, nil)

	sb, err := uc.GetContestScoreboard(ctx, contestID, userTest, false)
	require.NoError(t, err)
	require.Len(t, sb.Items, 1)

	item := sb.Items[0]
	assert.Equal(t, int32(1), item.ProblemsSolved)
	// Problem A: 59 min + (1 * 20) penalty = 79 penalty
	assert.Equal(t, int32(79), item.TotalPenalty)

	resMap := make(map[uuid.UUID]models.ContestProblemResult)
	for _, r := range item.ProblemResults {
		resMap[r.ProblemID] = r
	}

	resA := resMap[probA]
	assert.True(t, resA.Solved)
	assert.Equal(t, int32(1), resA.FailedAttempts)
	assert.Equal(t, int32(3), resA.PendingAttempts) // Exactly at freezeTime, 1ns after, exactly at endTime
	assert.Equal(t, int32(59), *resA.TimeMinutes)
	assert.Equal(t, int32(20), resA.Penalty)

	resB := resMap[probB]
	assert.False(t, resB.Solved)
	assert.Equal(t, int32(5), resB.FailedAttempts) // WA, TL, ML, RE, PE
	assert.Equal(t, int32(0), resB.PendingAttempts)
	assert.Equal(t, int32(0), resB.Penalty)
}

func TestGetContestScoreboard_ManualFrozenWithZeroDuration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	contestID := uuid.New()
	startTime := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	probA := uuid.New()

	problems := []models.ContestProblem{
		{ContestID: contestID, ProblemID: probA, Title: "A", ShortName: "A", Ordinal: 1},
	}

	userTest := uuid.New()
	userMap := map[uuid.UUID]string{userTest: "testuser"}

	// Manual frozen with 0 duration -> all submissions become pending attempts
	contest := models.Contest{
		ID:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           models.FreezeStatusFrozen,
			"freeze_duration_minutes": 0,
			"penalty_per_attempt":     20,
		},
	}

	mockRepo := new(ContestsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo, nil)

	mockRepo.On("GetContest", mock.Anything, contestID).Return(contest, nil)
	mockRepo.On("GetContestProblems", mock.Anything, contestID).Return(problems, nil)
	mockRepo.On("GetContestScoreboardFromStandings", mock.Anything, contestID).
		Return([]models.ContestProblemResult{}, userMap, nil)

	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userTest, probA).
		Return([]models.SubmissionForScoreboard{
			{State: models.GotWA, CreatedAt: startTime.Add(10 * time.Minute)},
			{State: models.Accepted, CreatedAt: startTime.Add(20 * time.Minute)},
		}, nil)

	sb, err := uc.GetContestScoreboard(ctx, contestID, userTest, false)
	require.NoError(t, err)
	assert.True(t, sb.IsFrozen)
	assert.Nil(t, sb.FreezeTime)

	item := sb.Items[0]
	assert.Equal(t, int32(0), item.ProblemsSolved)
	assert.Equal(t, int32(0), item.TotalPenalty)
	require.Len(t, item.ProblemResults, 1)
	assert.False(t, item.ProblemResults[0].Solved)
	assert.Equal(t, int32(0), item.ProblemResults[0].FailedAttempts)
	assert.Equal(t, int32(2), item.ProblemResults[0].PendingAttempts)
}
