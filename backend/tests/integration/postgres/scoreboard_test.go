//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"

	"github.com/brawler2011/gate/backend/pkg"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestScoreboardBatchedHistoryPostgres(t *testing.T) {
	t.Parallel()
	pool := getTestPool(t)
	ctx := context.Background()
	for _, size := range []int{1, 8} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			exec := func(query string, args ...any) {
				_, err := pool.Exec(ctx, query, args...)
				require.NoError(t, err)
			}
			suffix := uuid.NewString()
			org := helperCreateOrg(t, ctx, pool, "scoreboard_"+suffix, "Scoreboard")
			owner := helperOrgOwner(t, ctx, pool, org)
			repo := postgres.NewContestsRepo(pool)
			start := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
			end := start.Add(2 * time.Hour)
			contest, otherContest := uuid.New(), uuid.New()
			for _, id := range []uuid.UUID{contest, otherContest} {
				require.NoError(t, dbfixtures.CreateContest(ctx, pool, contestsmodel.CreateContestParams{ID: id, OrganizationID: org, Title: "Scoreboard", Login: id.String(), Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &start, EndTime: &end}, owner))
				require.NoError(t, repo.UpdateContest(ctx, contestsmodel.ContestUpdateInput{ID: id, Settings: contestsmodel.ContestSettingsPatch{FreezeDurationMinutes: new(int32(60)), FreezeDurationSet: true}}))
			}
			var problems []uuid.UUID
			for i := 0; i < size+1; i++ {
				problem := uuid.New()
				require.NoError(t, dbfixtures.InsertProblem(ctx, pool, dbfixtures.ProblemSpec{ID: problem, OrganizationID: org, Owner: owner, Title: "Problem", ShortName: fmt.Sprint(i)}))
				problems = append(problems, problem)
				// The final problem is deliberately absent from the contest.
				if i < size {
					pack, err := dbfixtures.InsertPackage(ctx, pool, problem, org, time.Time{})
					require.NoError(t, err)
					require.NoError(t, repo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: contest, ProblemId: problem, PackageId: pack}))
				}
			}
			var users []uuid.UUID
			for i := 0; i < size+2; i++ {
				user := uuid.New()
				users = append(users, user)
				exec(`INSERT INTO users(id,username,password_hash) VALUES ($1,$2,'unused')`, user, fmt.Sprintf("scoreboard_%s_%d", suffix, i))
				role := accessmodel.ContestParticipant
				if i == size+1 {
					role = accessmodel.ContestModerator
				}
				require.NoError(t, repo.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: contest, UserId: user, Role: role}))
			}
			// Out-of-order inserts must not let acceptance erase earlier failed attempts.
			add := func(c, u, p uuid.UUID, state submissionsmodel.State, minute int) {
				exec(`INSERT INTO submissions(contest_id,owner_id,problem_id,kind,source,language,state,created_at) VALUES ($1,$2,$3,'contest','print(1)',30,$4,$5)`, c, u, p, state, start.Add(time.Duration(minute)*time.Minute))
			}
			for _, user := range users[:size] {
				for _, problem := range problems[:size] {
					add(contest, user, problem, submissionsmodel.Accepted, 80)
					add(contest, user, problem, submissionsmodel.GotCE, 70)
					add(contest, user, problem, submissionsmodel.Saved, 65)
					add(contest, user, problem, submissionsmodel.GotWA, 10)
				}
			}
			add(otherContest, users[0], problems[0], submissionsmodel.Accepted, 5)
			add(contest, users[0], problems[size], submissionsmodel.Accepted, 5)
			add(contest, users[size+1], problems[0], submissionsmodel.Accepted, 5)

			ctx = accessmodel.WithReadTime(ctx, end.Add(-time.Nanosecond))
			// Count actual history queries, including any accidental per-problem reads.
			recordingPool, recorder := newRecordingPool(t)
			history := func() int { return recorder.count("FROM submissions") }
			uc := contests.NewContestsUseCase(postgres.NewContestsRepo(recordingPool), nil, nil, nil)
			for _, unfrozen := range []bool{false, true} {
				recorder.reset()
				snapshot, err := uc.ProjectScoreboard(ctx, contest, unfrozen)
				require.NoError(t, err)
				pending := 3
				if unfrozen {
					pending = 0
				}
				require.Equal(t, 1, history())
				require.Len(t, snapshot.Items, size+1)
				require.Len(t, snapshot.Problems, size)
				for _, item := range snapshot.Items {
					if item.UserID == users[size] {
						require.Empty(t, item.ProblemResults)
					} else {
						require.Len(t, item.ProblemResults, size)
						for _, result := range item.ProblemResults {
							require.EqualValues(t, 1, result.FailedAttempts)
							require.Equal(t, unfrozen, result.Solved)
							require.EqualValues(t, pending, result.PendingAttempts)
						}
						if unfrozen {
							require.EqualValues(t, size*100, item.TotalPenalty)
						} else {
							require.Zero(t, item.TotalPenalty)
						}
					}
					recorder.reset()
					row, err := uc.GetContestScoreboardUser(ctx, contest, item.UserID, unfrozen)
					require.NoError(t, err)
					require.Equal(t, 1, history())
					require.Len(t, row.Items, 1)
					actual := row.Items[0]
					require.ElementsMatch(t, item.ProblemResults, actual.ProblemResults)
					actual.ProblemResults = item.ProblemResults
					require.Equal(t, item, actual)
				}
			}
		})
	}
}

// ICPC freeze: only submissions made after the freeze stay hidden; earlier ones show whenever they are judged or rejudged.
func TestScoreboardFreezeHidesOnlyLaterSubmissions(t *testing.T) {
	t.Parallel()
	f := newContestProblemFixture(t)
	f.grantRead(t)
	require.NoError(t, f.create(f.original))
	now := time.Now().UTC().Truncate(time.Microsecond)
	start, end := now.Add(-time.Hour), now.Add(time.Hour)
	freeze := now.Add(-10 * time.Minute)
	_, err := f.pool.Exec(f.ctx, `UPDATE contests SET start_time=$2,end_time=$3,scoreboard_during='public',scoreboard_after='public',freeze_duration_minutes=70 WHERE id=$1`, f.contest, start, end)
	require.NoError(t, err)
	users := []uuid.UUID{uuid.New(), uuid.New()}
	for _, user := range users {
		_, err = f.pool.Exec(f.ctx, `INSERT INTO users(id,username,password_hash) VALUES($1,$2,'unused')`, user, user.String())
		require.NoError(t, err)
		require.NoError(t, f.repo.CreateContestMember(f.ctx, &contestsmodel.CreateContestMemberParams{ContestId: f.contest, UserId: user, Role: accessmodel.ContestParticipant}))
	}
	add := func(user uuid.UUID, created time.Time, state submissionsmodel.State) {
		t.Helper()
		_, err = f.pool.Exec(f.ctx, `INSERT INTO submissions(id,contest_id,problem_id,owner_id,kind,source,language,state,created_at) VALUES($1,$2,$3,$4,'contest','code',30,$5,$6)`, uuid.New(), f.contest, f.problem, user, state, created)
		require.NoError(t, err)
	}
	add(users[0], freeze.Add(-time.Minute), submissionsmodel.GotWA)
	add(users[0], freeze, submissionsmodel.Accepted)
	add(users[0], freeze.Add(time.Minute), submissionsmodel.Saved)
	add(users[1], freeze.Add(-2*time.Minute), submissionsmodel.Accepted)
	read := func(actor accessmodel.User, at time.Time, live bool) *contestsmodel.ScoreboardResponse {
		t.Helper()
		board, err := f.uc.GetContestScoreboard(accessmodel.WithReadTime(accessmodel.WithUser(f.ctx, actor), at), f.contest, live, false)
		require.NoError(t, err)
		return board
	}
	participant := accessmodel.User{Id: users[0], Role: accessmodel.UserRoleUser}
	masked := read(participant, now, false)
	require.True(t, masked.IsFrozen)
	public := read(accessmodel.Guest, now, false)
	require.Equal(t, masked.Items, public.Items)
	results := map[uuid.UUID]contestsmodel.ScoreboardItem{}
	for _, item := range masked.Items {
		results[item.UserID] = item
	}
	require.Zero(t, results[users[0]].ProblemsSolved)
	require.EqualValues(t, 1, results[users[0]].ProblemResults[0].FailedAttempts)
	require.EqualValues(t, 2, results[users[0]].ProblemResults[0].PendingAttempts)
	require.EqualValues(t, 1, results[users[1]].ProblemsSolved)
	// A rejudge of a pre-freeze submission during the freeze is visible at once.
	_, err = f.pool.Exec(f.ctx, `UPDATE submissions SET state=$3 WHERE owner_id=$1 AND contest_id=$2`, users[1], f.contest, submissionsmodel.GotWA)
	require.NoError(t, err)
	for _, item := range read(participant, now, false).Items {
		if item.UserID == users[1] {
			require.Zero(t, item.ProblemsSolved)
		}
	}
	_, err = f.uc.GetContestScoreboard(accessmodel.WithReadTime(accessmodel.WithUser(f.ctx, participant), now), f.contest, true, false)
	require.ErrorIs(t, err, pkg.ErrForbidden)
	live := read(accessmodel.UserFromContext(f.ctx), now, true)
	require.NotEqual(t, masked.Items, live.Items)
	final := read(participant, end, false)
	require.False(t, final.IsFrozen)
	require.Equal(t, live.Items, final.Items)
}
