package contests_test

import (
	"context"
	"sort"
	"testing"
	"time"

	contests "github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type scoreboardKey struct{ contest, user, problem uuid.UUID }

type scoreboardRepo struct {
	ContestsRepo
	contest     contestsmodel.Contest
	problems    []contestsmodel.ContestProblem
	users       map[uuid.UUID]string
	roles       map[uuid.UUID]contestsmodel.ContestRole
	submissions map[uuid.UUID]submissionsmodel.Submission
	results     map[scoreboardKey]contestsmodel.ContestProblemResult
	blocks      map[scoreboardKey]contestsmodel.ContestUserProblemBlock
}

func (r *scoreboardRepo) GetContest(_ context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	if id != r.contest.ID {
		return contestsmodel.Contest{}, pkg.ErrNotFound
	}
	return r.contest, nil
}

func (r *scoreboardRepo) GetContestMember(_ context.Context, input *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error) {
	role, ok := r.roles[input.UserId]
	if !ok || input.ContestId != r.contest.ID {
		return contestsmodel.ContestMember{}, pkg.ErrNotFound
	}
	return contestsmodel.ContestMember{ContestID: input.ContestId, UserID: input.UserId, ContestRole: role, Username: r.users[input.UserId]}, nil
}

func (r *scoreboardRepo) GetContestProblems(context.Context, uuid.UUID) ([]contestsmodel.ContestProblem, error) {
	return r.problems, nil
}

func (r *scoreboardRepo) GetContestProblem(_ context.Context, input contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error) {
	for _, p := range r.problems {
		if p.ContestID == input.ContestId && p.ProblemID == input.ProblemId {
			return p, nil
		}
	}
	return contestsmodel.ContestProblem{}, pkg.ErrNotFound
}

func (r *scoreboardRepo) GetSubmissionsForScoreboard(_ context.Context, contest, user, problem uuid.UUID) ([]contestsmodel.SubmissionForScoreboard, error) {
	var result []contestsmodel.SubmissionForScoreboard
	for _, s := range r.submissions {
		if s.ContestID != nil && *s.ContestID == contest && s.CreatedBy != nil && *s.CreatedBy == user && s.ProblemID != nil && *s.ProblemID == problem {
			result = append(result, contestsmodel.SubmissionForScoreboard{State: s.State, CreatedAt: s.CreatedAt})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (r *scoreboardRepo) UpsertContestProblemResult(_ context.Context, p *contestsmodel.UpsertContestProblemResultParams) error {
	r.results[scoreboardKey{p.ContestID, p.UserID, p.ProblemID}] = contestsmodel.ContestProblemResult{
		ContestID: p.ContestID, UserID: p.UserID, ProblemID: p.ProblemID, Solved: p.Solved,
		FailedAttempts: p.FailedAttempts, FirstACTime: p.FirstACTime, TimeMinutes: p.TimeMinutes,
	}
	return nil
}

func (r *scoreboardRepo) GetContestScoreboardFromStandings(_ context.Context, contest uuid.UUID) ([]contestsmodel.ContestProblemResult, map[uuid.UUID]string, error) {
	var results []contestsmodel.ContestProblemResult
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
	uc    *contests.ContestsUseCase
	users map[string]uuid.UUID
}

func newScoreboardFixture(names ...string) *scoreboardFixture {
	start := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	r := &scoreboardRepo{
		contest: contestsmodel.Contest{ID: uuid.New(), StartTime: &start, EndTime: &end, Settings: map[string]interface{}{"freeze_status": contestsmodel.FreezeStatusUnfrozen, "freeze_duration_minutes": 60, "penalty_per_attempt": 20}},
		users:   map[uuid.UUID]string{}, roles: map[uuid.UUID]contestsmodel.ContestRole{}, submissions: map[uuid.UUID]submissionsmodel.Submission{},
		results: map[scoreboardKey]contestsmodel.ContestProblemResult{}, blocks: map[scoreboardKey]contestsmodel.ContestUserProblemBlock{},
	}
	for i, name := range []string{"A", "B"} {
		r.problems = append(r.problems, contestsmodel.ContestProblem{ContestID: r.contest.ID, ProblemID: uuid.New(), Title: name, ShortName: name, Ordinal: i + 1})
	}
	f := &scoreboardFixture{repo: r, uc: contests.NewContestsUseCase(r, nil), users: map[string]uuid.UUID{}}
	for _, name := range names {
		id := uuid.New()
		f.users[name] = id
		r.users[id] = name
		r.roles[id] = contestsmodel.ContestRoleParticipant
	}
	return f
}

func (f *scoreboardFixture) submit(t *testing.T, user string, problem int, state submissionsmodel.State, offset time.Duration) uuid.UUID {
	t.Helper()
	id, contest, author, task := uuid.New(), f.repo.contest.ID, f.users[user], f.repo.problems[problem].ProblemID
	sub := submissionsmodel.Submission{ID: id, ContestID: &contest, CreatedBy: &author, ProblemID: &task, State: state, CreatedAt: f.repo.contest.StartTime.Add(offset)}
	f.repo.submissions[id] = sub
	require.NoError(t, f.uc.ProcessSubmissionResult(context.Background(), &sub))
	return id
}

func (f *scoreboardFixture) scoreboard(t *testing.T, unfrozen bool) *contestsmodel.ScoreboardResponse {
	t.Helper()
	result, err := f.uc.GetContestScoreboard(context.Background(), f.repo.contest.ID, uuid.Nil, unfrozen)
	require.NoError(t, err)
	return result
}

func TestScoreboardRecomputesAfterEveryVerdict(t *testing.T) {
	f := newScoreboardFixture("alice")
	for _, tc := range []struct {
		state           submissionsmodel.State
		minute          int
		solved          bool
		failed, penalty int32
	}{
		{submissionsmodel.GotCE, 5, false, 0, 0}, {submissionsmodel.Saved, 6, false, 0, 0},
		{submissionsmodel.GotWA, 10, false, 1, 0}, {submissionsmodel.Disqualified, 15, false, 2, 0},
		{submissionsmodel.Accepted, 25, true, 2, 65}, {submissionsmodel.GotWA, 30, true, 2, 65},
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
		state   submissionsmodel.State
		minute  int
	}{
		{"alice", 0, submissionsmodel.GotWA, 10}, {"alice", 0, submissionsmodel.Accepted, 15}, {"alice", 0, submissionsmodel.GotWA, 70},
		{"alice", 1, submissionsmodel.Accepted, 20},
		{"bob", 0, submissionsmodel.Accepted, 15}, {"bob", 1, submissionsmodel.GotWA, 20}, {"bob", 1, submissionsmodel.Accepted, 65},
		{"charlie", 0, submissionsmodel.GotWA, 10}, {"charlie", 0, submissionsmodel.GotWA, 70},
		{"dave", 0, submissionsmodel.GotWA, 70}, {"dave", 0, submissionsmodel.Accepted, 80},
		{"eve", 0, submissionsmodel.Accepted, 5}, {"eve", 0, submissionsmodel.GotWA, 12}, {"eve", 0, submissionsmodel.Accepted, 70}, {"eve", 0, submissionsmodel.GotWA, 75},
	} {
		f.submit(t, s.user, s.problem, s.state, time.Duration(s.minute)*time.Minute)
	}
	f.repo.contest.Settings["freeze_status"] = contestsmodel.FreezeStatusAuto
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
	f.repo.contest.Settings["freeze_status"] = contestsmodel.FreezeStatusUnfrozen
	unfrozen := f.scoreboard(t, false)
	require.False(t, unfrozen.IsFrozen)
	require.Equal(t, live.Items, unfrozen.Items)
}

func TestScoreboardContestAndFreezeBoundaries(t *testing.T) {
	f := newScoreboardFixture("alice")
	f.repo.contest.Settings["freeze_status"] = contestsmodel.FreezeStatusAuto
	for _, s := range []struct {
		state  submissionsmodel.State
		offset time.Duration
	}{
		{submissionsmodel.Accepted, -time.Nanosecond}, {submissionsmodel.GotWA, 0}, {submissionsmodel.Accepted, time.Hour - time.Nanosecond},
		{submissionsmodel.GotWA, time.Hour}, {submissionsmodel.GotWA, time.Hour + time.Nanosecond},
		{submissionsmodel.GotCE, 70 * time.Minute}, {submissionsmodel.Saved, 75 * time.Minute},
		{submissionsmodel.GotWA, 2 * time.Hour}, {submissionsmodel.Accepted, 2*time.Hour + time.Nanosecond},
	} {
		f.submit(t, "alice", 0, s.state, s.offset)
	}
	for i, state := range []submissionsmodel.State{submissionsmodel.GotWA, submissionsmodel.GotTL, submissionsmodel.GotML, submissionsmodel.GotRE, submissionsmodel.GotPE} {
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
	f.submit(t, "alice", 0, submissionsmodel.GotWA, 10*time.Minute)
	f.submit(t, "alice", 0, submissionsmodel.Accepted, 20*time.Minute)
	f.repo.contest.Settings["freeze_status"] = contestsmodel.FreezeStatusFrozen
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
	f.submit(t, "alice", 0, submissionsmodel.GotWA, 5*time.Minute)
	f.submit(t, "alice", 0, submissionsmodel.Accepted, 10*time.Minute)
	f.submit(t, "bob", 0, submissionsmodel.Accepted, 25*time.Minute)
	sb := f.scoreboard(t, false)
	require.Len(t, sb.Items, 4)
	for i, name := range []string{"alice", "bob", "charlie", "dave"} {
		require.Equal(t, name, sb.Items[i].Username)
	}
	require.EqualValues(t, 25, sb.Items[0].TotalPenalty)
	require.EqualValues(t, 25, sb.Items[1].TotalPenalty)
	require.EqualValues(t, 15, sb.PenaltyPerAttempt)
	f.submit(t, "bob", 1, submissionsmodel.Accepted, 30*time.Minute)
	sb = f.scoreboard(t, false)
	require.Equal(t, "bob", sb.Items[0].Username)
	require.EqualValues(t, 2, sb.Items[0].ProblemsSolved)
	require.EqualValues(t, 55, sb.Items[0].TotalPenalty)
}

func TestScoreboardUserUpdateMatchesSnapshot(t *testing.T) {
	f := newScoreboardFixture("alice", "bob", "charlie", "idle")
	f.submit(t, "alice", 0, submissionsmodel.GotWA, 10*time.Minute)
	accepted := f.submit(t, "alice", 0, submissionsmodel.Accepted, 80*time.Minute)
	f.submit(t, "alice", 1, submissionsmodel.GotCE, 20*time.Minute)
	f.submit(t, "bob", 0, submissionsmodel.Saved, 70*time.Minute)
	f.submit(t, "charlie", 0, submissionsmodel.Accepted, -time.Minute)
	check := func() {
		t.Helper()
		for _, frozen := range []string{contestsmodel.FreezeStatusUnfrozen, contestsmodel.FreezeStatusAuto, contestsmodel.FreezeStatusFrozen} {
			f.repo.contest.Settings["freeze_status"] = frozen
			for _, unfrozen := range []bool{false, true} {
				snapshot := f.scoreboard(t, unfrozen)
				for _, item := range snapshot.Items {
					update, err := f.uc.GetContestScoreboardUser(context.Background(), f.repo.contest.ID, item.UserID, unfrozen)
					require.NoError(t, err)
					require.Equal(t, snapshot.Problems, update.Problems)
					require.Equal(t, snapshot.IsFrozen, update.IsFrozen)
					require.Equal(t, snapshot.FreezeTime, update.FreezeTime)
					require.Equal(t, snapshot.PenaltyPerAttempt, update.PenaltyPerAttempt)
					require.Len(t, update.Items, 1)
					actual := update.Items[0]
					require.ElementsMatch(t, item.ProblemResults, actual.ProblemResults)
					actual.ProblemResults = item.ProblemResults
					require.Equal(t, item, actual)
				}
			}
		}
	}
	check()
	sub := f.repo.submissions[accepted]
	sub.State = submissionsmodel.Saved
	f.repo.submissions[accepted] = sub
	require.NoError(t, f.uc.ProcessSubmissionResult(context.Background(), &sub))
	check()
	delete(f.repo.roles, f.users["alice"])
	update, err := f.uc.GetContestScoreboardUser(context.Background(), f.repo.contest.ID, f.users["alice"], false)
	require.NoError(t, err)
	require.Empty(t, update.Items)
}
