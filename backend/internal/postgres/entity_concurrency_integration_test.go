//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	"github.com/brawler2011/gate/backend/internal/organizations"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func (f *concurrencyFixture) organization(t *testing.T) *organizationsmodel.Organization {
	t.Helper()
	c, err := NewContestsRepo(f.pool).GetContest(f.ctx, f.contest)
	require.NoError(t, err)
	org, err := NewOrganizationsRepo(f.pool).GetOrganizationByID(f.ctx, c.OrganizationID)
	require.NoError(t, err)
	return org
}

func (f *concurrencyFixture) contestUC() *contests.ContestsUseCase {
	return contests.NewContestsUseCase(NewContestsRepo(f.pool), NewOrganizationsRepo(f.pool), NewContestCommandTransactions(f.pool), f.permissions)
}

func (f *concurrencyFixture) organizationUC(tx organizations.OrganizationTransactions) *organizations.OrganizationsUseCase {
	return organizations.NewOrganizationsUseCase(NewOrganizationsRepo(f.pool), NewUsersRepo(f.pool), f.permissions, tx, nil)
}

// PostgreSQL FK checks take KEY SHARE locks. NO KEY UPDATE models an unrelated
// parent edit without making those normal checks block the operation under test.
func TestIndependentEntityCommandsIgnoreParentEdits(t *testing.T) {
	f := newConcurrencyFixture(t)
	org := f.organization(t)
	orgRepo, teamRepo := NewOrganizationsRepo(f.pool), NewTeamsRepo(f.pool)
	for _, user := range f.users {
		require.NoError(t, orgRepo.AddMember(f.ctx, org.ID, user, organizationsmodel.OrgRoleMember))
	}
	team, err := teamRepo.CreateTeam(f.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "Existing", Slug: uuid.NewString(), Privacy: organizationsmodel.TeamPrivacyClosed})
	require.NoError(t, err)
	require.NoError(t, teamRepo.AddTeamMember(f.ctx, team.ID, f.users[1], organizationsmodel.TeamRoleMember))
	c, err := NewContestsRepo(f.pool).GetContest(f.ctx, f.contest)
	require.NoError(t, err)
	_, err = f.pool.Exec(f.ctx, `UPDATE contests SET settings='{"participation_mode":"open"}' WHERE id=$1`, f.contest)
	require.NoError(t, err)
	require.NoError(t, NewContestsRepo(f.pool).DeleteContestMember(f.ctx, f.users[2], f.contest))
	applicant := uuid.New()
	_, err = f.pool.Exec(f.ctx, "INSERT INTO users(id,username,password_hash) VALUES ($1,$2,'unused')", applicant, applicant.String())
	require.NoError(t, err)
	blocker := beginConcurrencyTx(t, f)
	for _, held := range []struct {
		table string
		id    uuid.UUID
	}{{"organizations", org.ID}, {"contests", f.contest}, {"problems", f.problem}, {"teams", team.ID}} {
		_, err := blocker.Exec(f.ctx, "SELECT id FROM "+held.table+" WHERE id=$1 FOR NO KEY UPDATE", held.id)
		require.NoError(t, err)
	}
	communications := contests.NewClarificationsUseCase(NewContestEventsTransactions(f.pool))
	// Both participant requests must finish before the parent transaction ends.
	questions := make([]<-chan error, 2)
	for i := range questions {
		questions[i] = asyncConcurrent(func() error {
			_, err := communications.CreateClarification(f.userContext(1), &contestsmodel.CreateContestClarificationInput{ContestID: f.contest, UserID: f.users[1], Question: "Question"})
			return err
		})
	}
	for _, result := range questions {
		awaitConcurrent(t, result)
	}
	teams := organizations.NewTeamsUseCase(teamRepo, f.permissions, NewTeamTransactions(f.pool))
	orgs := f.organizationUC(NewOrganizationTransactions(f.pool))
	commands := map[string]func() error{
		"announcement": func() error {
			_, err := contests.NewAnnouncementsUseCase(NewContestEventsTransactions(f.pool)).CreateAnnouncement(f.userContext(0), &contestsmodel.CreateContestAnnouncementInput{ContestID: f.contest, AuthorID: f.users[0], Title: "Title", Body: "Body"})
			return err
		},
		"registration": func() error {
			_, joined, err := f.contestUC().CreateJoinRequest(f.userContext(2), org.Login, c.Login, nil)
			if err == nil && !joined {
				return errors.New("open registration did not create membership")
			}
			return err
		},
		"problem attachment": func() error {
			return f.contestUC().CreateContestProblem(f.userContext(0), contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: f.problem})
		},
		"team attachment": func() error {
			return f.contestUC().CreateContestTeam(f.userContext(0), f.contest, team.ID, f.users[0], contestsmodel.ContestRoleParticipant)
		},
		"contest member role": func() error {
			return f.contestUC().UpdateContestMember(f.userContext(0), f.contest, f.users[1], contestsmodel.ContestRoleModerator)
		},
		"organization member role": func() error {
			return orgs.UpdateMemberRole(f.userContext(0), org.ID, f.users[1], organizationsmodel.OrgRoleAdmin, f.users[0])
		},
		"team creation": func() error {
			_, err := teams.CreateTeam(f.userContext(0), &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "New", Slug: uuid.NewString(), Privacy: organizationsmodel.TeamPrivacyClosed}, f.users[0])
			return err
		},
		"team member role": func() error {
			return teams.UpdateTeamMemberRole(f.userContext(0), team.ID, f.users[1], organizationsmodel.TeamRoleMaintainer, f.users[0])
		},
		"team member addition": func() error {
			return teams.AddTeamMember(f.userContext(0), &organizationsmodel.AddTeamMemberInput{TeamID: team.ID, UserID: f.users[2], Role: organizationsmodel.TeamRoleMember}, f.users[0])
		},
		"problem member addition": func() error {
			return problems.NewProblemsUseCase(nil, nil, NewProblemTransactions(f.pool), nil, nil, f.permissions).CreateProblemMember(f.userContext(0), f.problem, f.users[1], f.users[0], problemsmodel.ProblemRoleModerator)
		},
		"invitation": func() error {
			_, err := orgs.InviteMember(f.userContext(0), org.Login, applicant, organizationsmodel.OrgRoleMember, f.users[0])
			return err
		},
		"organization join request": func() error {
			ctx := accessmodel.WithUser(f.ctx, accessmodel.User{Id: applicant, Role: accessmodel.UserRoleUser})
			_, _, err := orgs.CreateJoinRequest(ctx, org.Login, nil)
			return err
		},
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) { awaitConcurrent(t, asyncConcurrent(command)) })
	}
	awaitConcurrent(t, asyncConcurrent(func() error { return teams.RemoveTeamMember(f.userContext(0), team.ID, f.users[2], f.users[0]) }))
	awaitConcurrent(t, asyncConcurrent(func() error { return orgs.RemoveMember(f.userContext(0), org.ID, f.users[1], f.users[0]) }))
	_, err = orgRepo.GetMember(f.ctx, org.ID, f.users[1])
	require.ErrorIs(t, err, pkg.ErrNotFound)
	_, err = teamRepo.GetTeamMember(f.ctx, team.ID, f.users[1])
	require.ErrorIs(t, err, pkg.ErrNotFound)
	var events int
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1", f.contest).Scan(&events))
	require.Equal(t, 3, events, "communication writes retain their delivery events")
}

type readQueryTrace struct {
	mu      sync.Mutex
	queries []string
}

func (tr *readQueryTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.queries = append(tr.queries, strings.ToUpper(data.SQL))
	return ctx
}
func (*readQueryTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestEntityReadsUsePoolWithoutRowLocks(t *testing.T) {
	f := newConcurrencyFixture(t)
	org := f.organization(t)
	_, err := f.pool.Exec(f.ctx, "UPDATE problems SET owner_id=$2 WHERE id=$1", f.problem, f.users[0])
	require.NoError(t, err)
	require.NoError(t, NewOrganizationsRepo(f.pool).AddMember(f.ctx, org.ID, f.users[1], organizationsmodel.OrgRoleMember))
	require.NoError(t, NewProblemsRepo(f.pool).CreateProblemMember(f.ctx, &problemsmodel.CreateProblemMemberParams{ProblemID: f.problem, UserID: f.users[1], Role: problemsmodel.ProblemRoleModerator}))
	team, err := NewTeamsRepo(f.pool).CreateTeam(f.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "Readers", Slug: uuid.NewString(), Privacy: organizationsmodel.TeamPrivacySecret})
	require.NoError(t, err)
	require.NoError(t, NewTeamsRepo(f.pool).AddTeamMember(f.ctx, team.ID, f.users[2], organizationsmodel.TeamRoleMember))
	require.NoError(t, NewProblemsRepo(f.pool).AddProblemTeam(f.ctx, f.problem, team.ID, problemsmodel.ProblemPermissionRead))
	trace := &readQueryTrace{}
	config := f.pool.Config()
	config.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(f.ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	permissions := access.NewPermissionsUseCase(NewContestsRepo(pool), NewUsersRepo(pool), NewProblemsRepo(pool), NewTeamsRepo(pool), NewOrganizationsRepo(pool))
	repo := NewProblemsRepo(pool)
	objects := storage.NewLocalStorage(t.TempDir())
	workspace := problems.NewWorkspaceStorage(objects, "workspace", problems.NewWorkspaceCatalog(repo, NewProblemTransactions(pool)))
	reader := problems.NewProblemsUseCase(repo, workspace, nil, nil, nil, permissions)
	publisher := problems.NewProblemPublishUseCase(repo, NewPackagesRepo(pool), workspace, objects, "packages", nil, permissions)
	drafts := contests.NewDraftsUseCase(nil, NewDraftsRepo(pool), permissions)
	blocker := beginConcurrencyTx(t, f)
	// Even exclusive parent and membership edits cannot block pool readers.
	for _, query := range []string{
		"SELECT id FROM organizations WHERE id=$1 FOR UPDATE",
		"SELECT user_id FROM organization_members WHERE organization_id=$1 FOR UPDATE",
		"SELECT id FROM teams WHERE organization_id=$1 FOR UPDATE",
		"SELECT user_id FROM team_members WHERE team_id IN (SELECT id FROM teams WHERE organization_id=$1) FOR UPDATE",
	} {
		_, err := blocker.Exec(f.ctx, query, org.ID)
		require.NoError(t, err)
	}
	for _, query := range []string{
		"SELECT id FROM problems WHERE id=$1 FOR UPDATE",
		"SELECT user_id FROM problem_members WHERE problem_id=$1 FOR UPDATE",
	} {
		_, err := blocker.Exec(f.ctx, query, f.problem)
		require.NoError(t, err)
	}
	_, err = blocker.Exec(f.ctx, "SELECT id FROM contests WHERE id=$1 FOR UPDATE", f.contest)
	require.NoError(t, err)
	for _, user := range []int{1, 2} {
		awaitConcurrent(t, asyncConcurrent(func() error {
			_, err := reader.GetProblemDetails(f.userContext(user), f.problem)
			return err
		}))
	}
	awaitConcurrent(t, asyncConcurrent(func() error { _, err := publisher.ListPackages(f.userContext(1), f.problem); return err }))
	awaitConcurrent(t, asyncConcurrent(func() error {
		_, err := publisher.GetPublishedPackageURL(f.userContext(1), f.problem, "v1")
		return err
	}))
	awaitConcurrent(t, asyncConcurrent(func() error { _, err := publisher.GetReadyPackage(f.userContext(1), f.problem); return err }))
	awaitConcurrent(t, asyncConcurrent(func() error {
		_, err := drafts.ListDrafts(f.userContext(1), contestsmodel.ContestDraftsFilter{ContestID: f.contest, UserID: f.users[1]})
		return err
	}))
	_, err = reader.GetProblemDetails(f.userContext(1), uuid.New())
	require.ErrorIs(t, err, pkg.ErrNotFound)
	_, err = reader.GetProblemDetails(context.Background(), f.problem)
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)

	outsider := uuid.New()
	_, err = f.pool.Exec(f.ctx, "INSERT INTO users(id,username,password_hash) VALUES ($1,$2,'unused')", outsider, outsider.String())
	require.NoError(t, err)
	_, err = reader.GetProblemDetails(accessmodel.WithUser(f.ctx, accessmodel.User{Id: outsider, Role: accessmodel.UserRoleUser}), f.problem)
	require.ErrorIs(t, err, pkg.NoPermission)

	trace.mu.Lock()
	defer trace.mu.Unlock()
	require.NotEmpty(t, trace.queries)
	for _, query := range trace.queries {
		for _, forbidden := range []string{"BEGIN", "COMMIT", "FOR UPDATE", "FOR NO KEY UPDATE", "FOR SHARE", "FOR KEY SHARE", "PG_ADVISORY"} {
			require.NotContains(t, query, forbidden)
		}
	}
}

func waitForBlockedCommands(t *testing.T, f *concurrencyFixture, table string, count int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var n int
		err := f.pool.QueryRow(f.ctx, "SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE $1", "%UPDATE "+table+"%").Scan(&n)
		return err == nil && n == count
	}, 3*time.Second, 5*time.Millisecond)
}

func TestConcurrentPendingDecisionsHaveOneWinner(t *testing.T) {
	for _, kind := range []string{"organization", "contest", "invitation"} {
		for _, second := range []string{"approve", "reject"} {
			t.Run(kind+"/"+second, func(t *testing.T) {
				f := newConcurrencyFixture(t)
				org := f.organization(t)
				orgUC := f.organizationUC(NewOrganizationTransactions(f.pool))
				contest, err := NewContestsRepo(f.pool).GetContest(f.ctx, f.contest)
				require.NoError(t, err)
				var id uuid.UUID
				var table, members, parent string
				var parentID uuid.UUID
				var approve, reject func() error
				switch kind {
				case "organization":
					req, err := NewOrganizationsRepo(f.pool).CreateJoinRequest(f.ctx, &organizationsmodel.CreateOrganizationJoinRequestInput{OrganizationID: org.ID, UserID: f.users[1]})
					require.NoError(t, err)
					id, table, members, parent, parentID = req.ID, "organization_join_requests", "organization_members", "organization_id", org.ID
					approve = func() error {
						return orgUC.ApproveJoinRequest(f.userContext(0), org.Login, id, f.users[0], organizationsmodel.OrgRoleMember)
					}
					reject = func() error { return orgUC.RejectJoinRequest(f.userContext(0), org.Login, id, f.users[0]) }
				case "contest":
					require.NoError(t, NewContestsRepo(f.pool).DeleteContestMember(f.ctx, f.users[1], f.contest))
					req, err := NewContestsRepo(f.pool).CreateContestJoinRequest(f.ctx, &contestsmodel.CreateContestJoinRequestInput{ContestID: f.contest, UserID: f.users[1]})
					require.NoError(t, err)
					id, table, members, parent, parentID = req.ID, "contest_join_requests", "contest_members", "contest_id", f.contest
					approve = func() error {
						return f.contestUC().ApproveJoinRequest(f.userContext(0), org.Login, contest.Login, id, f.users[0])
					}
					reject = func() error {
						return f.contestUC().RejectJoinRequest(f.userContext(0), org.Login, contest.Login, id, f.users[0])
					}
				default:
					inv, err := NewOrganizationsRepo(f.pool).CreateInvitation(f.ctx, &organizationsmodel.CreateOrganizationInvitationInput{OrganizationID: org.ID, UserID: f.users[1], InviterID: f.users[0], Role: organizationsmodel.OrgRoleMember})
					require.NoError(t, err)
					id, table, members, parent, parentID = inv.ID, "organization_invitations", "organization_members", "organization_id", org.ID
					approve = func() error { return orgUC.AcceptInvitation(f.userContext(1), id) }
					reject = func() error { return orgUC.DeclineInvitation(f.userContext(1), id) }
				}
				blocker := beginConcurrencyTx(t, f)
				_, err = blocker.Exec(f.ctx, "SELECT id FROM "+table+" WHERE id=$1 FOR UPDATE", id)
				require.NoError(t, err)
				other := reject
				if second == "approve" {
					other = approve
				}
				results := []<-chan error{asyncConcurrent(approve), asyncConcurrent(other)}
				// Both commands read pending and reach the conditional UPDATE before
				// either can perform a dependent write.
				waitForBlockedCommands(t, f, table, 2)
				require.NoError(t, blocker.Commit(f.ctx))
				successes := 0
				for _, result := range results {
					err := <-result
					if err == nil {
						successes++
					} else {
						require.ErrorIs(t, err, pkg.ErrConflict)
					}
				}
				require.Equal(t, 1, successes)
				var status string
				require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT status FROM "+table+" WHERE id=$1", id).Scan(&status))
				var member bool
				require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT EXISTS(SELECT 1 FROM "+members+" WHERE "+parent+"=$1 AND user_id=$2)", parentID, f.users[1]).Scan(&member))
				require.Equal(t, status == "approved" || status == "accepted", member)
				var count int
				require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM notifications WHERE user_id=$1", f.users[1]).Scan(&count))
				if kind == "invitation" {
					require.Zero(t, count)
				} else {
					require.Equal(t, 1, count)
				}
			})
		}
	}
}

func TestConcurrentContestProblemOrder(t *testing.T) {
	f := newConcurrencyFixture(t)
	org := f.organization(t)
	uc := f.contestUC()
	newProblem := func() (uuid.UUID, uuid.UUID) {
		id := uuid.New()
		_, err := f.pool.Exec(f.ctx, "INSERT INTO problems(id,organization_id,title,short_name) VALUES ($1,$2,'Problem',$3)", id, org.ID, id.String())
		require.NoError(t, err)

		pack := uuid.New()
		_, err = NewPackagesRepo(f.pool).CreatePackage(f.ctx, &problemsmodel.CreatePackageParams{ID: pack, ProblemID: id, OrganizationID: org.ID, PackageHash: strings.Repeat(strings.ReplaceAll(pack.String(), "-", ""), 2), Status: "ready"})
		require.NoError(t, err)
		return id, pack
	}
	second, pack := newProblem()
	third, thirdPack := newProblem()
	// A gap must not cause the next attachment to reuse an occupied ordinal.
	_, err := f.pool.Exec(f.ctx, "UPDATE contest_problems SET ordinal=3 WHERE contest_id=$1", f.contest)
	require.NoError(t, err)
	blocker := beginConcurrencyTx(t, f)
	require.NoError(t, (&ContestsRepo{queries: sqlc.New(blocker)}).LockContestProblemOrder(f.ctx, f.contest))
	results := []<-chan error{
		asyncConcurrent(func() error {
			return uc.CreateContestProblem(f.userContext(0), contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: second, PackageId: pack})
		}),
		asyncConcurrent(func() error {
			return uc.CreateContestProblem(f.userContext(0), contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: third, PackageId: thirdPack})
		}),
	}
	waitForAdvisoryWaiters(t, f, advisoryKey("gate:contest-problem-order:", f.contest), 2)
	require.NoError(t, blocker.Commit(f.ctx))
	for _, result := range results {
		awaitConcurrent(t, result)
	}
	rows, err := NewContestsRepo(f.pool).GetContestProblems(f.ctx, f.contest)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	for i, row := range rows {
		require.EqualValues(t, i+3, row.Ordinal)
	}
	// Reattaching updates the package and preserves the position in one statement.
	replacement, err := NewPackagesRepo(f.pool).CreatePackage(f.ctx, &problemsmodel.CreatePackageParams{ID: uuid.New(), ProblemID: second, OrganizationID: org.ID, PackageHash: strings.Repeat(strings.ReplaceAll(uuid.NewString(), "-", ""), 2), Status: "ready"})
	require.NoError(t, err)
	require.NoError(t, uc.CreateContestProblem(f.userContext(0), contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: second, PackageId: replacement.ID}))
	attached, err := NewContestsRepo(f.pool).GetContestProblem(f.ctx, contestsmodel.ContestProblemGet{ContestId: f.contest, ProblemId: second})
	require.NoError(t, err)
	require.Equal(t, replacement.ID, attached.PackageID)
	blocker = beginConcurrencyTx(t, f)
	require.NoError(t, (&ContestsRepo{queries: sqlc.New(blocker)}).LockContestProblemOrder(f.ctx, f.contest))
	fourth, fourthPack := newProblem()
	results = []<-chan error{
		asyncConcurrent(func() error {
			return uc.ReorderContestProblems(f.userContext(0), f.contest, []contestsmodel.ContestProblemReorderItem{{ProblemID: second, Position: 1}, {ProblemID: f.problem, Position: 2}})
		}),
		asyncConcurrent(func() error {
			return uc.DeleteContestProblem(f.userContext(0), contestsmodel.ContestProblemDeletion{ContestId: f.contest, ProblemId: third})
		}),
		asyncConcurrent(func() error {
			return uc.CreateContestProblem(f.userContext(0), contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: fourth, PackageId: fourthPack})
		}),
	}
	waitForAdvisoryWaiters(t, f, advisoryKey("gate:contest-problem-order:", f.contest), 3)
	require.NoError(t, blocker.Commit(f.ctx))
	for _, result := range results {
		awaitConcurrent(t, result)
	}
	rows, err = NewContestsRepo(f.pool).GetContestProblems(f.ctx, f.contest)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.Equal(t, second, rows[0].ProblemID)
	require.Equal(t, f.problem, rows[1].ProblemID)
	require.Equal(t, fourth, rows[2].ProblemID)
	before := rows
	require.Error(t, uc.ReorderContestProblems(f.userContext(0), f.contest, []contestsmodel.ContestProblemReorderItem{{ProblemID: second, Position: 1}, {ProblemID: f.problem, Position: 1}}))
	rows, err = NewContestsRepo(f.pool).GetContestProblems(f.ctx, f.contest)
	require.NoError(t, err)
	require.Equal(t, before, rows, "an invalid reorder rolls back every temporary and final position")
	// Positions above the old fixed temporary range can still be swapped.
	require.NoError(t, uc.ReorderContestProblems(f.userContext(0), f.contest, []contestsmodel.ContestProblemReorderItem{{ProblemID: second, Position: 10000}, {ProblemID: f.problem, Position: 10001}}))
	require.NoError(t, uc.ReorderContestProblems(f.userContext(0), f.contest, []contestsmodel.ContestProblemReorderItem{{ProblemID: second, Position: 10001}, {ProblemID: f.problem, Position: 10000}}))

}

func TestConcurrentPackageVersions(t *testing.T) {
	f := newConcurrencyFixture(t)

	objects := storage.NewLocalStorage(t.TempDir())
	workspace := problems.NewWorkspaceStorage(objects, "workspace", problems.NewWorkspaceCatalog(NewProblemsRepo(f.pool), NewProblemTransactions(f.pool)))
	editor := problems.NewWorkshopUseCase(workspace, nil, f.permissions)
	require.NoError(t, editor.InitProblemWorkshop(f.userContext(0), f.problem, "Publication"))
	require.NoError(t, editor.UpdateProblemFile(f.userContext(0), problemsmodel.UpdateFileRequest{ProblemID: f.problem, Path: "checkers/check.cpp", Content: []byte("checker")}))
	blocker := beginConcurrencyTx(t, f)
	require.NoError(t, (&PackagesRepo{queries: sqlc.New(blocker)}).LockPackageVersions(f.ctx, f.problem))
	results := make([]<-chan error, 6)

	for i := range results {
		require.NoError(t, editor.UpdateProblemFile(f.userContext(0), problemsmodel.UpdateFileRequest{ProblemID: f.problem, Path: "checkers/check.cpp", Content: []byte(fmt.Sprintf("checker %d", i))}))
		// Independent workspace instances model publishers in different server processes.
		ws := problems.NewWorkspaceStorage(objects, "workspace", problems.NewWorkspaceCatalog(NewProblemsRepo(f.pool), NewProblemTransactions(f.pool)))
		publisher := problems.NewProblemPublishUseCase(NewProblemsRepo(f.pool), NewPackagesRepo(f.pool), ws, objects, "packages", NewProblemTransactions(f.pool), f.permissions)
		results[i] = asyncConcurrent(func() error { _, err := publisher.PublishProblem(f.userContext(0), f.problem); return err })
		// Capture this revision before preparing a different hash for the next
		// publisher; package hashes have an existing global unique constraint.
		waitForAdvisoryWaiters(t, f, advisoryKey("gate:package-versions:", f.problem), i+1)
	}

	waitForAdvisoryWaiters(t, f, advisoryKey("gate:package-versions:", f.problem), len(results))
	require.NoError(t, blocker.Commit(f.ctx))
	for _, result := range results {
		awaitConcurrent(t, result)
	}
	var versions []int32
	rows, err := f.pool.Query(f.ctx, "SELECT version FROM problem_packages WHERE problem_id=$1 ORDER BY version", f.problem)
	require.NoError(t, err)
	versions, err = pgx.CollectRows(rows, pgx.RowTo[int32])
	require.NoError(t, err)
	require.Equal(t, []int32{1, 2, 3, 4, 5, 6, 7}, versions)
}

type failedOrganizationNotification struct {
	organizations.OrganizationTxStore
}

func (failedOrganizationNotification) CreateNotification(context.Context, *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error) {
	return nil, pkg.ErrInternal
}

type failedOrganizationTransaction struct {
	organizations.OrganizationTransactions
}

func (tr failedOrganizationTransaction) Within(ctx context.Context, fn func(organizations.OrganizationTxStore) error) error {
	return tr.OrganizationTransactions.Within(ctx, func(tx organizations.OrganizationTxStore) error { return fn(failedOrganizationNotification{tx}) })
}

func TestPendingDecisionRollsBackMembershipAndStatus(t *testing.T) {
	f := newConcurrencyFixture(t)
	org := f.organization(t)
	repo := NewOrganizationsRepo(f.pool)
	req, err := repo.CreateJoinRequest(f.ctx, &organizationsmodel.CreateOrganizationJoinRequestInput{OrganizationID: org.ID, UserID: f.users[1]})
	require.NoError(t, err)
	uc := f.organizationUC(failedOrganizationTransaction{NewOrganizationTransactions(f.pool)})
	require.ErrorIs(t, uc.ApproveJoinRequest(f.userContext(0), org.Login, req.ID, f.users[0], organizationsmodel.OrgRoleMember), pkg.ErrInternal)
	current, err := repo.GetJoinRequestByID(f.ctx, req.ID)
	require.NoError(t, err)
	require.Equal(t, organizationsmodel.RequestStatusPending, current.Status)
	_, err = repo.GetMember(f.ctx, org.ID, f.users[1])
	require.ErrorIs(t, err, pkg.ErrNotFound)
}

type failedTeamMembership struct{ organizations.TeamTxStore }

func (failedTeamMembership) AddTeamMember(context.Context, uuid.UUID, uuid.UUID, organizationsmodel.TeamRole) error {
	return pkg.ErrInternal
}

type failedTeamTransaction struct{ organizations.TeamTransactions }

func (tr failedTeamTransaction) Within(ctx context.Context, fn func(organizations.TeamTxStore) error) error {
	return tr.TeamTransactions.Within(ctx, func(tx organizations.TeamTxStore) error { return fn(failedTeamMembership{tx}) })
}

func TestTeamCreationRollsBackWithoutCreatorMembership(t *testing.T) {
	f := newConcurrencyFixture(t)
	org := f.organization(t)
	teams := organizations.NewTeamsUseCase(NewTeamsRepo(f.pool), f.permissions, failedTeamTransaction{NewTeamTransactions(f.pool)})
	slug := uuid.NewString()
	_, err := teams.CreateTeam(f.userContext(0), &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "Atomic", Slug: slug, Privacy: organizationsmodel.TeamPrivacyClosed}, f.users[0])
	require.ErrorIs(t, err, pkg.ErrInternal)
	var count int
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM teams WHERE organization_id=$1 AND slug=$2", org.ID, slug).Scan(&count))
	require.Zero(t, count)
}

func TestOrganizationRemovalWaitsOnlyForTargetMembership(t *testing.T) {
	f := newConcurrencyFixture(t)
	org := f.organization(t)
	orgRepo := NewOrganizationsRepo(f.pool)
	for _, user := range f.users[1:] {
		require.NoError(t, orgRepo.AddMember(f.ctx, org.ID, user, organizationsmodel.OrgRoleMember))
	}
	team, err := NewTeamsRepo(f.pool).CreateTeam(f.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "Cleanup", Slug: uuid.NewString(), Privacy: organizationsmodel.TeamPrivacyClosed})
	require.NoError(t, err)
	attachment := beginConcurrencyTx(t, f)
	bound := NewTeamTransactions(f.pool).(transactions[organizations.TeamTxStore]).bind(sqlc.New(attachment))
	_, err = bound.GetMember(f.ctx, org.ID, f.users[1])
	require.NoError(t, err)
	require.NoError(t, bound.AddTeamMember(f.ctx, team.ID, f.users[1], organizationsmodel.TeamRoleMember))
	uc := f.organizationUC(NewOrganizationTransactions(f.pool))
	remove := asyncConcurrent(func() error { return uc.RemoveMember(f.userContext(0), org.ID, f.users[1], f.users[0]) })
	require.Eventually(t, func() bool {
		var count int
		err := f.pool.QueryRow(f.ctx, "SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%-- name: LockOrganizationMember%'").Scan(&count)
		return err == nil && count == 1
	}, 3*time.Second, 5*time.Millisecond)
	awaitConcurrent(t, asyncConcurrent(func() error { return uc.RemoveMember(f.userContext(0), org.ID, f.users[2], f.users[0]) }))
	require.NoError(t, attachment.Commit(f.ctx))
	awaitConcurrent(t, remove)
	_, err = NewTeamsRepo(f.pool).GetTeamMember(f.ctx, team.ID, f.users[1])
	require.ErrorIs(t, err, pkg.ErrNotFound, "cleanup must see and remove the attachment committed during its wait")
}

func TestOrganizationCreationsShareCreatorPrivileges(t *testing.T) {
	f := newConcurrencyFixture(t)
	blocker := beginConcurrencyTx(t, f)
	_, err := blocker.Exec(f.ctx, "SELECT id FROM users WHERE id=$1 FOR SHARE", f.users[0])
	require.NoError(t, err)
	uc := f.organizationUC(NewOrganizationTransactions(f.pool))
	results := make([]<-chan error, 2)
	for i := range results {
		results[i] = asyncConcurrent(func() error {
			org, err := uc.CreateOrganization(f.userContext(0), &organizationsmodel.CreateOrganizationInput{CreatorID: f.users[0], Login: "org-" + uuid.NewString(), Name: "Independent"})
			if err != nil {
				return err
			}
			member, err := NewOrganizationsRepo(f.pool).GetMember(f.ctx, org.ID, f.users[0])
			if err == nil && member.Role != organizationsmodel.OrgRoleOwner {
				return errors.New("creator was not added as owner")
			}
			return err
		})
	}
	for _, result := range results {
		awaitConcurrent(t, result)
	}
}
