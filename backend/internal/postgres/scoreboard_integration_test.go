//go:build integration

package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// Count actual history queries, including any accidental per-problem reads.
type scoreboardQueryCounter struct {
	sqlc.DBTX
	history int
}

func (c *scoreboardQueryCounter) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	if strings.Contains(query, "FROM submissions") {
		c.history++
	}
	return c.DBTX.Query(ctx, query, args...)
}

func TestScoreboardBatchedHistoryPostgres(t *testing.T) {
	pool := getSharedTestPool(t)
	ctx := context.Background()
	for _, size := range []int{1, 8} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			exec := func(query string, args ...any) {
				_, err := pool.Exec(ctx, query, args...)
				require.NoError(t, err)
			}
			suffix := uuid.NewString()
			org := helperCreateOrg(t, ctx, pool, "scoreboard_"+suffix, "Scoreboard")
			repo := NewContestsRepo(pool)
			start := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
			end := start.Add(2 * time.Hour)
			contest, otherContest := uuid.New(), uuid.New()
			for _, id := range []uuid.UUID{contest, otherContest} {
				require.NoError(t, repo.CreateContest(ctx, &contestsmodel.CreateContestParams{ID: id, OrganizationID: org, Title: "Scoreboard", Login: id.String(), Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &start, EndTime: &end, Settings: map[string]interface{}{"freeze_status": contestsmodel.FreezeStatusFrozen, "freeze_duration_minutes": 60}}))
			}
			var problems []uuid.UUID
			for i := 0; i < size+1; i++ {
				problem := uuid.New()
				exec(`INSERT INTO problems(id,organization_id,title,short_name) VALUES ($1,$2,'Problem',$3)`, problem, org, fmt.Sprint(i))
				problems = append(problems, problem)
				// The final problem is deliberately absent from the contest.
				if i < size {
					pack := uuid.New()
					hash := strings.ReplaceAll(pack.String(), "-", "")
					exec(`INSERT INTO problem_packages(id,problem_id,organization_id,package_hash,status,version) VALUES ($1,$2,$3,$4,'ready',1)`, pack, problem, org, hash+hash)
					require.NoError(t, repo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: contest, ProblemId: problem, PackageId: pack}))
				}
			}
			var users []uuid.UUID
			for i := 0; i < size+2; i++ {
				user := uuid.New()
				users = append(users, user)
				exec(`INSERT INTO users(id,username,password_hash) VALUES ($1,$2,'unused')`, user, fmt.Sprintf("scoreboard_%s_%d", suffix, i))
				role := contestsmodel.ContestRoleParticipant
				if i == size+1 {
					role = contestsmodel.ContestRoleModerator
				}
				require.NoError(t, repo.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: contest, UserId: user, Role: role}))
			}
			// Insert out of chronological order to exercise SQL ordering. The accepted
			// submission must not erase the earlier failed attempt.
			add := func(c, u, p uuid.UUID, state submissionsmodel.State, minute int) {
				exec(`INSERT INTO submissions(contest_id,owner_id,problem_id,source,language,state,created_at) VALUES ($1,$2,$3,'print(1)',30,$4,$5)`, c, u, p, state, start.Add(time.Duration(minute)*time.Minute))
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

			// Populate live standings through the same path used after verdicts.
			for _, user := range users[:size] {
				for _, problem := range problems[:size] {
					require.NoError(t, contests.ApplySubmissionResult(ctx, repo, &submissionsmodel.Submission{ContestID: &contest, CreatedBy: &user, ProblemID: &problem}))
				}
			}
			counter := &scoreboardQueryCounter{DBTX: pool}
			repo.queries = sqlc.New(counter)
			uc := contests.NewContestsUseCase(repo, nil, nil, nil)
			for _, unfrozen := range []bool{false, true} {
				counter.history = 0
				snapshot, err := uc.ProjectScoreboard(ctx, contest, unfrozen)
				require.NoError(t, err)
				expectedHistory, pending := 1, 2
				if unfrozen {
					expectedHistory, pending = 0, 0
				}
				require.Equal(t, expectedHistory, counter.history)
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
					counter.history = 0
					row, err := uc.GetContestScoreboardUser(ctx, contest, item.UserID, unfrozen)
					require.NoError(t, err)
					require.Equal(t, 1, counter.history)
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
