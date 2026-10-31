package usecase_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type scoreboardKey struct{ contest, user, problem uuid.UUID }

type scoreboardRepo struct {
	interfaces.ContestsRepo
	contest     models.Contest
	problems    []models.ContestProblem
	users       map[uuid.UUID]string
	roles       map[uuid.UUID]models.ContestRole
	submissions map[uuid.UUID]models.Submission
	results     map[scoreboardKey]models.ContestProblemResult
	blocks      map[scoreboardKey]models.ContestUserProblemBlock
}

func (r *scoreboardRepo) GetContest(_ context.Context, id uuid.UUID) (models.Contest, error) {
	if id != r.contest.ID {
		return models.Contest{}, pkg.ErrNotFound
	}
	return r.contest, nil
}
func (r *scoreboardRepo) GetContestMember(_ context.Context, input *models.ContestPermissionGet) (models.ContestMember, error) {
	role, ok := r.roles[input.UserId]
	if !ok || input.ContestId != r.contest.ID {
		return models.ContestMember{}, pkg.ErrNotFound
	}
	return models.ContestMember{ContestID: input.ContestId, UserID: input.UserId, ContestRole: role}, nil
}
func (r *scoreboardRepo) GetContestProblems(context.Context, uuid.UUID) ([]models.ContestProblem, error) {
	return r.problems, nil
}
func (r *scoreboardRepo) GetContestProblem(_ context.Context, input models.ContestProblemGet) (models.ContestProblem, error) {
	for _, p := range r.problems {
		if p.ContestID == input.ContestId && p.ProblemID == input.ProblemId {
			return p, nil
		}
	}
	return models.ContestProblem{}, pkg.ErrNotFound
}
func (r *scoreboardRepo) GetSubmissionsForScoreboard(_ context.Context, contest, user, problem uuid.UUID) ([]models.SubmissionForScoreboard, error) {
	var result []models.SubmissionForScoreboard
	for _, s := range r.submissions {
		if s.ContestID != nil && *s.ContestID == contest && s.CreatedBy != nil && *s.CreatedBy == user && s.ProblemID != nil && *s.ProblemID == problem {
			result = append(result, models.SubmissionForScoreboard{State: s.State, CreatedAt: s.CreatedAt})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}
func (r *scoreboardRepo) UpsertContestProblemResult(_ context.Context, p *models.UpsertContestProblemResultParams) error {
	r.results[scoreboardKey{p.ContestID, p.UserID, p.ProblemID}] = models.ContestProblemResult{
		ContestID: p.ContestID, UserID: p.UserID, ProblemID: p.ProblemID, Solved: p.Solved,
		FailedAttempts: p.FailedAttempts, FirstACTime: p.FirstACTime, TimeMinutes: p.TimeMinutes,
	}
	return nil
}
func (r *scoreboardRepo) GetContestScoreboardFromStandings(_ context.Context, contest uuid.UUID) ([]models.ContestProblemResult, map[uuid.UUID]string, error) {
	var results []models.ContestProblemResult
	for key, result := range r.results {
		if key.contest == contest {
			results = append(results, result)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ProblemID.String() < results[j].ProblemID.String() })
	return results, r.users, nil
}

type scoreboardFixture struct {
	repo  *scoreboardRepo
	uc    *usecase.ContestsUseCase
	users map[string]uuid.UUID
}

func newScoreboardFixture(names ...string) *scoreboardFixture {
	start := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	r := &scoreboardRepo{
		contest: models.Contest{ID: uuid.New(), StartTime: &start, EndTime: &end, Settings: map[string]interface{}{"freeze_status": models.FreezeStatusUnfrozen, "freeze_duration_minutes": 60, "penalty_per_attempt": 20}},
		users:   map[uuid.UUID]string{}, roles: map[uuid.UUID]models.ContestRole{}, submissions: map[uuid.UUID]models.Submission{},
		results: map[scoreboardKey]models.ContestProblemResult{}, blocks: map[scoreboardKey]models.ContestUserProblemBlock{},
	}
	for i, name := range []string{"A", "B"} {
		r.problems = append(r.problems, models.ContestProblem{ContestID: r.contest.ID, ProblemID: uuid.New(), Title: name, ShortName: name, Ordinal: i + 1})
	}
	f := &scoreboardFixture{repo: r, uc: usecase.NewContestsUseCase(r, nil), users: map[string]uuid.UUID{}}
	for _, name := range names {
		id := uuid.New()
		f.users[name] = id
		r.users[id] = name
		r.roles[id] = models.ContestRoleParticipant
	}
	return f
}
func (f *scoreboardFixture) submit(t *testing.T, user string, problem int, state models.State, offset time.Duration) uuid.UUID {
	t.Helper()
	id, contest, author, task := uuid.New(), f.repo.contest.ID, f.users[user], f.repo.problems[problem].ProblemID
	sub := models.Submission{ID: id, ContestID: &contest, CreatedBy: &author, ProblemID: &task, State: state, CreatedAt: f.repo.contest.StartTime.Add(offset)}
	f.repo.submissions[id] = sub
	require.NoError(t, f.uc.ProcessSubmissionResult(context.Background(), &sub))
	return id
}
func (f *scoreboardFixture) scoreboard(t *testing.T, unfrozen bool) *models.ScoreboardResponse {
	t.Helper()
	result, err := f.uc.GetContestScoreboard(context.Background(), f.repo.contest.ID, uuid.Nil, unfrozen)
	require.NoError(t, err)
	return result
}

func TestScoreboardRecomputesAfterEveryVerdict(t *testing.T) {
	f := newScoreboardFixture("alice")
	for _, tc := range []struct {
		state           models.State
		minute          int
		solved          bool
		failed, penalty int32
	}{
		{models.GotCE, 5, false, 0, 0}, {models.Saved, 6, false, 0, 0},
		{models.GotWA, 10, false, 1, 0}, {models.Disqualified, 15, false, 2, 0},
		{models.Accepted, 25, true, 2, 65}, {models.GotWA, 30, true, 2, 65},
	} {
		f.submit(t, "alice", 0, tc.state, time.Duration(tc.minute)*time.Minute)
		sb := f.scoreboard(t, false)
		require.Len(t, sb.Items, 1)
		require.Len(t, sb.Items[0].ProblemResults, 1)
		result := sb.Items[0].ProblemResults[0]
		require.Equal(t, tc.solved, result.Solved)
		require.Equal(t, tc.failed, result.FailedAttempts)
		require.Equal(t, tc.penalty, sb.Items[0].TotalPenalty)
		if tc.solved {
			require.NotNil(t, result.TimeMinutes)
			require.EqualValues(t, 25, *result.TimeMinutes)
		}
	}
}

func TestScoreboardFrozenAndLiveViews(t *testing.T) {
	f := newScoreboardFixture("alice", "bob", "charlie", "dave", "eve", "frank")
	for _, s := range []struct {
		user    string
		problem int
		state   models.State
		minute  int
	}{
		{"alice", 0, models.GotWA, 10}, {"alice", 0, models.Accepted, 15}, {"alice", 0, models.GotWA, 70},
		{"alice", 1, models.Accepted, 20},
		{"bob", 0, models.Accepted, 15}, {"bob", 1, models.GotWA, 20}, {"bob", 1, models.Accepted, 65},
		{"charlie", 0, models.GotWA, 10}, {"charlie", 0, models.GotWA, 70},
		{"dave", 0, models.GotWA, 70}, {"dave", 0, models.Accepted, 80},
		{"eve", 0, models.Accepted, 5}, {"eve", 0, models.GotWA, 12}, {"eve", 0, models.Accepted, 70}, {"eve", 0, models.GotWA, 75},
	} {
		f.submit(t, s.user, s.problem, s.state, time.Duration(s.minute)*time.Minute)
	}
	f.repo.contest.Settings["freeze_status"] = models.FreezeStatusAuto
	frozen := f.scoreboard(t, false)
	require.True(t, frozen.IsFrozen)
	require.Equal(t, f.repo.contest.EndTime.Add(-time.Hour), *frozen.FreezeTime)
	require.Len(t, frozen.Items, 6)
	for i, expected := range []struct {
		name            string
		solved, penalty int32
	}{
		{"alice", 2, 55}, {"eve", 1, 5}, {"bob", 1, 15}, {"charlie", 0, 0}, {"dave", 0, 0}, {"frank", 0, 0},
	} {
		item := frozen.Items[i]
		require.Equal(t, expected.name, item.Username)
		require.Equal(t, expected.solved, item.ProblemsSolved)
		require.Equal(t, expected.penalty, item.TotalPenalty)
	}
	for _, tc := range []struct {
		user            string
		solved          bool
		failed, pending int32
	}{
		{"alice", true, 1, 1}, {"bob", true, 0, 0}, {"charlie", false, 1, 1}, {"dave", false, 0, 2}, {"eve", true, 0, 2},
	} {
		for _, item := range frozen.Items {
			if item.Username == tc.user {
				require.NotEmpty(t, item.ProblemResults)
				result := item.ProblemResults[0]
				require.Equal(t, tc.solved, result.Solved)
				require.Equal(t, tc.failed, result.FailedAttempts)
				require.Equal(t, tc.pending, result.PendingAttempts)
			}
		}
	}
	require.Empty(t, frozen.Items[5].ProblemResults)
	live := f.scoreboard(t, true)
	require.True(t, live.IsFrozen)
	require.Equal(t, "alice", live.Items[0].Username)
	require.Equal(t, "bob", live.Items[1].Username)
	require.EqualValues(t, 2, live.Items[1].ProblemsSolved)
	require.EqualValues(t, 100, live.Items[1].TotalPenalty)
	for _, item := range live.Items {
		for _, result := range item.ProblemResults {
			require.Zero(t, result.PendingAttempts)
		}
	}
	f.repo.contest.Settings["freeze_status"] = models.FreezeStatusUnfrozen
	unfrozen := f.scoreboard(t, false)
	require.False(t, unfrozen.IsFrozen)
	require.Equal(t, live.Items, unfrozen.Items)
}

func TestScoreboardContestAndFreezeBoundaries(t *testing.T) {
	f := newScoreboardFixture("alice")
	f.repo.contest.Settings["freeze_status"] = models.FreezeStatusAuto
	for _, s := range []struct {
		state  models.State
		offset time.Duration
	}{
		{models.Accepted, -time.Nanosecond}, {models.GotWA, 0}, {models.Accepted, time.Hour - time.Nanosecond},
		{models.GotWA, time.Hour}, {models.GotWA, time.Hour + time.Nanosecond},
		{models.GotCE, 70 * time.Minute}, {models.Saved, 75 * time.Minute},
		{models.GotWA, 2 * time.Hour}, {models.Accepted, 2*time.Hour + time.Nanosecond},
	} {
		f.submit(t, "alice", 0, s.state, s.offset)
	}
	for i, state := range []models.State{models.GotWA, models.GotTL, models.GotML, models.GotRE, models.GotPE} {
		f.submit(t, "alice", 1, state, time.Duration(i+1)*5*time.Minute)
	}
	sb := f.scoreboard(t, false)
	require.Len(t, sb.Items, 1)
	require.EqualValues(t, 1, sb.Items[0].ProblemsSolved)
	require.EqualValues(t, 79, sb.Items[0].TotalPenalty)
	require.Len(t, sb.Items[0].ProblemResults, 2)
	a, b := sb.Items[0].ProblemResults[0], sb.Items[0].ProblemResults[1]
	require.True(t, a.Solved)
	require.EqualValues(t, 1, a.FailedAttempts)
	require.EqualValues(t, 5, a.PendingAttempts)
	require.EqualValues(t, 59, *a.TimeMinutes)
	require.False(t, b.Solved)
	require.EqualValues(t, 5, b.FailedAttempts)
	require.Zero(t, b.Penalty)
}

func TestScoreboardManualFreezeWithoutCutoff(t *testing.T) {
	f := newScoreboardFixture("alice")
	f.submit(t, "alice", 0, models.GotWA, 10*time.Minute)
	f.submit(t, "alice", 0, models.Accepted, 20*time.Minute)
	f.repo.contest.Settings["freeze_status"] = models.FreezeStatusFrozen
	f.repo.contest.Settings["freeze_duration_minutes"] = 0
	sb := f.scoreboard(t, false)
	require.True(t, sb.IsFrozen)
	require.Nil(t, sb.FreezeTime)
	require.Len(t, sb.Items, 1)
	require.Zero(t, sb.Items[0].ProblemsSolved)
	require.Zero(t, sb.Items[0].TotalPenalty)
	require.Len(t, sb.Items[0].ProblemResults, 1)
	require.EqualValues(t, 2, sb.Items[0].ProblemResults[0].PendingAttempts)
	require.Zero(t, sb.Items[0].ProblemResults[0].FailedAttempts)
}

func TestScoreboardCustomPenaltyAndTieBreaks(t *testing.T) {
	f := newScoreboardFixture("alice", "bob", "charlie", "dave")
	f.repo.contest.Settings["penalty_per_attempt"] = 15
	f.submit(t, "alice", 0, models.GotWA, 5*time.Minute)
	f.submit(t, "alice", 0, models.Accepted, 10*time.Minute)
	f.submit(t, "bob", 0, models.Accepted, 25*time.Minute)
	sb := f.scoreboard(t, false)
	require.Len(t, sb.Items, 4)
	for i, name := range []string{"alice", "bob", "charlie", "dave"} {
		require.Equal(t, name, sb.Items[i].Username)
	}
	require.EqualValues(t, 25, sb.Items[0].TotalPenalty)
	require.EqualValues(t, 25, sb.Items[1].TotalPenalty)
	require.EqualValues(t, 15, sb.PenaltyPerAttempt)
	f.submit(t, "bob", 1, models.Accepted, 30*time.Minute)
	sb = f.scoreboard(t, false)
	require.Equal(t, "bob", sb.Items[0].Username)
	require.EqualValues(t, 2, sb.Items[0].ProblemsSolved)
	require.EqualValues(t, 55, sb.Items[0].TotalPenalty)
}
