//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/organizations"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type teamResourcesCountingDB struct {
	*pgxpool.Pool
	queries int
}

func (db *teamResourcesCountingDB) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	db.queries++
	return db.Pool.QueryRow(ctx, query, args...)
}

func (db *teamResourcesCountingDB) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	db.queries++
	return db.Pool.Query(ctx, query, args...)
}

func TestTeamResourcesViewerAccess(t *testing.T) {
	pool := getSharedTestPool(t)
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, query, args...)
		require.NoError(t, err)
	}
	suffix := uuid.NewString()[:8]
	orgID := helperCreateOrg(t, ctx, pool, "teamres_"+suffix, "Resource organization")
	foreignOrgID := helperCreateOrg(t, ctx, pool, "foreign_"+suffix, "Foreign organization")
	t.Cleanup(func() {
		_, err := pool.Exec(ctx, `DELETE FROM organizations WHERE id IN ($1, $2)`, orgID, foreignOrgID)
		require.NoError(t, err)
	})
	users := map[string]uuid.UUID{}
	for _, name := range []string{"outsider", "org member", "team member", "other team member", "direct member", "owner", "org admin", "org owner", "global admin", "foreign team member"} {
		id := uuid.New()
		users[name] = id
		role := "user"
		if name == "global admin" {
			role = "admin"
		}
		exec(`INSERT INTO users (id, username, password_hash, role) VALUES ($1, $2, 'hash', $3)`, id, id.String(), role)
		t.Cleanup(func() { _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id); require.NoError(t, err) })
	}
	for name, role := range map[string]string{"org member": "member", "org admin": "admin", "org owner": "owner"} {
		exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, orgID, users[name], role)
	}
	teamID, otherTeamID, foreignTeamID := uuid.New(), uuid.New(), uuid.New()
	for id, org := range map[uuid.UUID]uuid.UUID{teamID: orgID, otherTeamID: orgID, foreignTeamID: foreignOrgID} {
		exec(`INSERT INTO teams (id, organization_id, name, slug) VALUES ($1, $2, 'Team', $3)`, id, org, id.String())
	}
	for name, id := range map[string]uuid.UUID{"team member": teamID, "other team member": otherTeamID, "foreign team member": foreignTeamID} {
		exec(`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'member')`, id, users[name])
	}
	contestIDs, problemIDs := []uuid.UUID{}, []uuid.UUID{}
	for i, visibility := range []string{"public", "private", "unlisted"} {
		contestID, problemID := uuid.New(), uuid.New()
		contestIDs = append(contestIDs, contestID)
		problemIDs = append(problemIDs, problemID)
		// Distinct timestamps make preservation of the existing descending order verifiable.
		exec(`INSERT INTO contests (id, organization_id, owner_id, visibility, title, login, settings, created_at)
        VALUES ($1, $2, $3, $4, 'Contest', $5, '{}', '2026-01-01'::timestamptz + $6 * interval '1 day')`, contestID, orgID, users["owner"], visibility, contestID.String(), i)
		exec(`INSERT INTO problems (id, organization_id, owner_id, visibility, title, short_name, created_at)
        VALUES ($1, $2, $3, $4, 'Problem', $5, '2026-01-01'::timestamptz + $6 * interval '1 day')`, problemID, orgID, users["owner"], visibility, problemID.String(), i)
		for _, id := range []uuid.UUID{teamID, otherTeamID, foreignTeamID} {
			exec(`INSERT INTO contest_teams (contest_id, team_id, role) VALUES ($1, $2, 'participant')`, contestID, id)
			exec(`INSERT INTO problem_teams (problem_id, team_id, permission) VALUES ($1, $2, 'read')`, problemID, id)
		}
		exec(`INSERT INTO contest_members (contest_id, user_id, role) VALUES ($1, $2, 'participant')`, contestID, users["direct member"])
		exec(`INSERT INTO problem_members (problem_id, user_id, role) VALUES ($1, $2, 'viewer')`, problemID, users["direct member"])
	}
	teams := NewTeamsRepo(pool)
	permissions := access.NewPermissionsUseCase(NewContestsRepo(pool), NewUsersRepo(pool), NewProblemsRepo(pool), teams, NewOrganizationsRepo(pool))
	uc := organizations.NewTeamsUseCase(teams, permissions, nil)
	for name, viewerID := range users {
		t.Run(name, func(t *testing.T) {
			db := &teamResourcesCountingDB{Pool: pool}
			repo := &TeamsRepo{q: sqlc.New(db)}
			contests, err := repo.GetTeamContests(ctx, teamID, viewerID)
			require.NoError(t, err)
			require.Equal(t, 1, db.queries)
			problems, err := repo.GetTeamProblems(ctx, teamID, viewerID)
			require.NoError(t, err)
			require.Equal(t, 2, db.queries)
			viewer, err := NewUsersRepo(pool).GetUserById(ctx, viewerID)
			require.NoError(t, err)
			actorCtx := accessmodel.WithUser(ctx, viewer)
			visibleContests, err := uc.GetTeamContests(actorCtx, teamID, viewerID)
			require.NoError(t, err)
			require.Equal(t, contests, visibleContests)
			visibleProblems, err := uc.GetTeamProblems(actorCtx, teamID, viewerID)
			require.NoError(t, err)
			require.Equal(t, problems, visibleProblems)
			gotContests, gotProblems := []uuid.UUID{}, []uuid.UUID{}
			for _, c := range contests {
				gotContests = append(gotContests, c.ID)
			}
			for _, p := range problems {
				gotProblems = append(gotProblems, p.ID)
			}
			wantContests, wantProblems := []uuid.UUID{}, []uuid.UUID{}
			for i := len(contestIDs) - 1; i >= 0; i-- {
				allowed, err := permissions.HasContestPermission(ctx, contestIDs[i], viewerID, accessmodel.ActionGetContest)
				require.NoError(t, err)
				if allowed {
					wantContests = append(wantContests, contestIDs[i])
				}
				allowed, err = permissions.HasProblemPermission(ctx, problemIDs[i], viewerID, accessmodel.ActionViewProblem)
				require.NoError(t, err)
				if allowed {
					wantProblems = append(wantProblems, problemIDs[i])
				}
			}
			require.Equal(t, wantContests, gotContests)
			require.Equal(t, wantProblems, gotProblems)
			expected := 3
			if name == "outsider" || name == "org member" || name == "foreign team member" {
				expected = 1
			}
			require.Len(t, contests, expected)
			require.Len(t, problems, expected)
		})
	}
	// Preserve an independently granted private item while excluding unlisted items.
	exec(`INSERT INTO contest_members (contest_id, user_id, role) VALUES ($1, $2, 'participant')`, contestIDs[1], users["outsider"])
	exec(`INSERT INTO problem_members (problem_id, user_id, role) VALUES ($1, $2, 'viewer')`, problemIDs[1], users["outsider"])
	mixedContests, err := teams.GetTeamContests(ctx, teamID, users["outsider"])
	require.NoError(t, err)
	require.Len(t, mixedContests, 2)
	require.Equal(t, contestIDs[1], mixedContests[0].ID)
	require.Equal(t, contestIDs[0], mixedContests[1].ID)
	mixedProblems, err := teams.GetTeamProblems(ctx, teamID, users["outsider"])
	require.NoError(t, err)
	require.Len(t, mixedProblems, 2)
	require.Equal(t, problemIDs[1], mixedProblems[0].ID)
	require.Equal(t, problemIDs[0], mixedProblems[1].ID)
	exec(`DELETE FROM contest_members WHERE user_id = $1`, users["outsider"])
	exec(`DELETE FROM problem_members WHERE user_id = $1`, users["outsider"])
	// A team linked only to inaccessible resources returns an empty collection.
	exec(`DELETE FROM contest_teams WHERE team_id = $1 AND contest_id = $2`, teamID, contestIDs[0])
	exec(`DELETE FROM problem_teams WHERE team_id = $1 AND problem_id = $2`, teamID, problemIDs[0])
	contests, err := teams.GetTeamContests(ctx, teamID, users["outsider"])
	require.NoError(t, err)
	require.Empty(t, contests)
	problems, err := teams.GetTeamProblems(ctx, teamID, users["outsider"])
	require.NoError(t, err)
	require.Empty(t, problems)
}
