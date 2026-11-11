//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/postgres"

	"github.com/brawler2011/gate/backend/tests/dbfixtures"

	"github.com/brawler2011/gate/backend/internal/access"
	m "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/organizations"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// scopeRow is the shape shared by the generated scope query rows.
type scopeRow struct {
	TotalItems int64
	Resource   []byte
}

// emptyScopeRow reports the single null resource SQL emits for an empty page,
// including when the offset exceeds the total.
func emptyScopeRow(data []byte) bool { return len(data) == 0 || string(data) == "null" }

// scopedIDs decodes a scope page read straight from SQL into resource IDs and the total.
func scopedIDs(rows []scopeRow) ([]uuid.UUID, int64, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	var total int64
	for _, row := range rows {
		total = row.TotalItems
		if emptyScopeRow(row.Resource) {
			continue
		}
		var resource struct {
			ID uuid.UUID `json:"id"`
		}
		if err := json.Unmarshal(row.Resource, &resource); err != nil {
			return nil, 0, fmt.Errorf("decode scoped resource: %w", err)
		}
		ids = append(ids, resource.ID)
	}
	return ids, total, nil
}

func scopeProblems(ctx context.Context, q *sqlc.Queries, params sqlc.ScopeProblemsParams) ([]uuid.UUID, int64, error) {
	rows, err := q.ScopeProblems(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("scope problems: %w", err)
	}
	converted := make([]scopeRow, len(rows))
	for i, row := range rows {
		converted[i] = scopeRow(row)
	}
	return scopedIDs(converted)
}

func scopeContests(ctx context.Context, q *sqlc.Queries, params sqlc.ScopeContestsParams) ([]uuid.UUID, int64, error) {
	rows, err := q.ScopeContests(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("scope contests: %w", err)
	}
	converted := make([]scopeRow, len(rows))
	for i, row := range rows {
		converted[i] = scopeRow(row)
	}
	return scopedIDs(converted)
}

// All scopes, details and future capability projections use the same persisted
// facts. Explicit expectations below also prevent matching bugs in both policies
// and SQL from making this parity test vacuous.
func TestMetadataPolicySQLParity(t *testing.T) {
	pool := getSharedTestPool(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	ctx := m.WithReadTime(context.Background(), now)
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, query, args...)
		require.NoError(t, err)
	}
	suffix := uuid.NewString()[:8]
	org := helperCreateOrg(t, ctx, pool, "parity_"+suffix, "Parity")
	foreign := helperCreateOrg(t, ctx, pool, "parity_foreign_"+suffix, "Foreign")
	users := map[string]m.User{"guest": m.Guest}
	for _, name := range []string{"outsider", "expired", "expiry equality", "primary owner", "global admin", "org owner", "org admin", "org member", "direct viewer", "direct moderator", "direct owner", "team read", "team write", "team admin", "foreign team", "multiple teams"} {
		u := m.User{Id: uuid.New(), Role: m.UserRoleUser}
		if name == "global admin" {
			u.Role = m.UserRoleAdmin
		}
		if name == "expired" {
			past := now.Add(-time.Second)
			u.ExpiresAt = &past
		}
		if name == "expiry equality" {
			u.ExpiresAt = &now
		}
		exec(`INSERT INTO users(id, username, password_hash, role, expires_at) VALUES($1,$2,'hash',$3,$4)`, u.Id, u.Id.String(), u.Role, u.ExpiresAt)
		users[name] = u
	}
	t.Cleanup(func() {
		exec(`DELETE FROM organizations WHERE id IN ($1,$2)`, org, foreign)
		for _, u := range users {
			if !u.IsGuest() {
				exec(`DELETE FROM users WHERE id=$1`, u.Id)
			}
		}
	})
	for name, role := range map[string]string{"org owner": "owner", "org admin": "admin", "org member": "member"} {
		exec(`INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,$3)`, org, users[name].Id, role)
	}
	closed, secret, wrong := uuid.New(), uuid.New(), uuid.New()
	for id, privacy := range map[uuid.UUID]string{closed: "closed", secret: "secret", wrong: "closed"} {
		teamOrg := org
		if id == wrong {
			teamOrg = foreign
		}
		exec(`INSERT INTO teams(id,organization_id,name,slug,privacy) VALUES($1,$2,'Parity',$3,$4)`, id, teamOrg, id.String(), privacy)
	}
	teamIDs := []uuid.UUID{closed, secret}
	problemIDs, contestIDs := []uuid.UUID{}, []uuid.UUID{}
	for _, visibility := range []string{"public", "private", "unlisted"} {
		p, c := uuid.New(), uuid.New()
		problemIDs = append(problemIDs, p)
		contestIDs = append(contestIDs, c)
		owner := users["primary owner"].Id
		require.NoError(t, dbfixtures.InsertProblem(ctx, pool, dbfixtures.ProblemSpec{ID: p, OrganizationID: org, OwnerID: &owner, Visibility: visibility, Title: "Parity", ShortName: p.String()}))
		exec(`INSERT INTO contests(id,organization_id,owner_id,title,login,visibility,settings) VALUES($1,$2,$3,'Parity',$4,$5,'{"participation_mode":"open"}')`, c, org, users["primary owner"].Id, c.String(), visibility)
		for _, team := range []uuid.UUID{closed, secret, wrong} {
			exec(`INSERT INTO problem_teams(problem_id,team_id,permission) VALUES($1,$2,'read')`, p, team)
			exec(`INSERT INTO contest_teams(contest_id,team_id,role) VALUES($1,$2,'participant')`, c, team)
		}
		for name, role := range map[string]string{"direct viewer": "viewer", "direct moderator": "moderator", "direct owner": "owner", "expired": "owner", "expiry equality": "owner"} {
			exec(`INSERT INTO problem_members(problem_id,user_id,role) VALUES($1,$2,$3)`, p, users[name].Id, role)
			contestRole := role
			if role == "viewer" {
				contestRole = "participant"
			}
			exec(`INSERT INTO contest_members(contest_id,user_id,role) VALUES($1,$2,$3)`, c, users[name].Id, contestRole)
		}
	}
	// Independent team grants cover each problem permission and both contest roles.
	for name, permission := range map[string]string{"team read": "read", "team write": "write", "team admin": "admin", "foreign team": "admin"} {
		team := uuid.New()
		teamOrg := org
		if name == "foreign team" {
			teamOrg = foreign
		}
		exec(`INSERT INTO teams(id,organization_id,name,slug,privacy) VALUES($1,$2,'Grant',$3,'secret')`, team, teamOrg, team.String())
		exec(`INSERT INTO team_members(team_id,user_id,role) VALUES($1,$2,'maintainer')`, team, users[name].Id)
		for i, p := range problemIDs {
			exec(`INSERT INTO problem_teams(problem_id,team_id,permission) VALUES($1,$2,$3)`, p, team, permission)
			role := "participant"
			if permission != "read" {
				role = "moderator"
			}
			exec(`INSERT INTO contest_teams(contest_id,team_id,role) VALUES($1,$2,$3)`, contestIDs[i], team, role)
		}
	}
	for _, team := range []uuid.UUID{closed, secret} {
		exec(`INSERT INTO team_members(team_id,user_id,role) VALUES($1,$2,'member')`, team, users["multiple teams"].Id)
	}
	recordingPool, recorder := newRecordingPool(t)
	teamsRepo := postgres.NewTeamsRepo(recordingPool)
	for name, u := range users {
		t.Run(name, func(t *testing.T) {
			actorCtx := m.WithUser(ctx, u)
			in := m.PolicyInput{Actor: u, Now: now}
			recorder.reset()
			q := sqlc.New(recordingPool)
			factsRepo := postgres.NewPolicyFactsRepo(recordingPool)
			pf, err := factsRepo.LoadProblemFacts(actorCtx, u.Id, problemIDs)
			require.NoError(t, err)
			cf, err := factsRepo.LoadContestFacts(actorCtx, u.Id, contestIDs)
			require.NoError(t, err)
			tf, err := factsRepo.LoadTeamFacts(actorCtx, u.Id, teamIDs)
			require.NoError(t, err)
			require.Len(t, recorder.recorded(), 3, "one query per fact type, independent of page length")
			empty, err := factsRepo.LoadProblemFacts(actorCtx, u.Id, nil)
			require.NoError(t, err)
			require.Empty(t, empty)
			require.Len(t, recorder.recorded(), 3)
			metadataLegacy := access.NewPermissionsUseCase(nil, postgres.NewUsersRepo(recordingPool), nil, nil, nil)
			permissions := access.NewMetadataPermissions(metadataLegacy, factsRepo)
			wantP, wantC, wantT := []uuid.UUID{}, []uuid.UUID{}, []uuid.UUID{}
			for i, p := range problemIDs {
				allow := access.RequireProblem(in, pf[p], m.ActionViewProblem) == nil
				allowed, err := permissions.HasProblemPermission(actorCtx, p, u.Id, m.ActionViewProblem)
				if name == "expired" || name == "expiry equality" {
					require.ErrorIs(t, err, pkg.ErrUnauthenticated)
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, allow, allowed)
				if allow {
					wantP = append(wantP, p)
				}
				c := contestIDs[i]
				allow = access.RequireContest(in, cf[c], m.ActionViewContest) == nil
				allowed, err = permissions.HasContestPermission(actorCtx, c, u.Id, m.ActionViewContest)
				if name == "expired" || name == "expiry equality" {
					require.ErrorIs(t, err, pkg.ErrUnauthenticated)
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, allow, allowed)
				if allow {
					wantC = append(wantC, c)
				}
			}
			for _, team := range teamIDs {
				if access.RequireTeam(in, tf[team], m.ActionViewTeam) == nil {
					wantT = append(wantT, team)
				}
			}
			count := 2
			if name == "guest" || name == "outsider" || name == "org member" || name == "foreign team" {
				count = 1
			}
			if name == "expired" || name == "expiry equality" {
				count = 0
			}
			require.Len(t, wantP, count)
			require.Len(t, wantC, count)
			// Use SQL scopes directly for expired identities: public grants must not turn
			// an expired authenticated actor into a guest. Use cases return 401 instead.
			ps, pt, err := scopeProblems(actorCtx, q, sqlc.ScopeProblemsParams{ViewerID: &u.Id, Now: now, OrganizationID: &org, Descending: true, PageSize: 100})
			require.NoError(t, err)
			cs, ct, err := scopeContests(actorCtx, q, sqlc.ScopeContestsParams{ViewerID: &u.Id, Now: now, OrganizationID: &org, PageSize: 100})
			require.NoError(t, err)
			require.ElementsMatch(t, wantP, ps)
			require.ElementsMatch(t, wantC, cs)
			require.EqualValues(t, len(wantP), pt)
			require.EqualValues(t, len(wantC), ct)
			ts, tt, err := teamsRepo.ListTeams(actorCtx, &organizationsmodel.TeamsFilter{OrganizationID: &org, Page: 1, PageSize: 100}, u.Id)
			require.NoError(t, err)
			gotT := []uuid.UUID{}
			for _, team := range ts {
				if team.ID == closed || team.ID == secret {
					gotT = append(gotT, team.ID)
				}
			}
			require.ElementsMatch(t, wantT, gotT)
			require.GreaterOrEqual(t, int(tt), len(wantT))

			allTeams, _, err := teamsRepo.ListTeams(actorCtx, &organizationsmodel.TeamsFilter{Page: 1, PageSize: 100}, u.Id)
			require.NoError(t, err)
			allIDs := []uuid.UUID{}
			for _, team := range allTeams {
				if team.ID == closed || team.ID == secret {
					allIDs = append(allIDs, team.ID)
				}
			}
			require.ElementsMatch(t, wantT, allIDs)
			et, etTotal, err := teamsRepo.ListTeams(actorCtx, &organizationsmodel.TeamsFilter{OrganizationID: &org, Page: 100, PageSize: 1}, u.Id)
			require.NoError(t, err)
			require.Empty(t, et)
			require.Equal(t, tt, etTotal)
			for _, team := range teamIDs {
				allowed, err := permissions.HasTeamPermission(actorCtx, team, u.Id, m.ActionViewTeam)
				if name == "expired" || name == "expiry equality" {
					require.ErrorIs(t, err, pkg.ErrUnauthenticated)
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, access.RequireTeam(in, tf[team], m.ActionViewTeam) == nil, allowed)
			}
			// Dashboard short lists use identical scopes before their LIMIT as well.
			dp, err := postgres.NewProblemsRepo(recordingPool).ListDashboardProblems(actorCtx, u.Id, 100)
			require.NoError(t, err)
			dc, err := postgres.NewContestsRepo(recordingPool).ListDashboardContests(actorCtx, u.Id, 100)
			require.NoError(t, err)
			dashboardP, dashboardC := []uuid.UUID{}, []uuid.UUID{}
			for _, p := range dp {
				if p.OrganizationID == org {
					dashboardP = append(dashboardP, p.ID)
				}
			}
			for _, c := range dc {
				if c.OrganizationID == org {
					dashboardC = append(dashboardC, c.ID)
				}
			}
			require.ElementsMatch(t, wantP, dashboardP)
			require.ElementsMatch(t, wantC, dashboardC)

			// Team visibility is not resource visibility (#145), including direct and
			// other-team grants, duplicate grants, and foreign organization rejection.
			tp, total, err := teamsRepo.GetTeamProblems(actorCtx, closed, u.Id, paging.Request{})
			require.NoError(t, err)
			require.Len(t, tp, count)
			require.EqualValues(t, count, total)
			tc, total, err := teamsRepo.GetTeamContests(actorCtx, closed, u.Id, paging.Request{})
			require.NoError(t, err)
			require.Len(t, tc, count)
			require.EqualValues(t, count, total)
			// One statement must preserve totals on pages beyond the end.
			ep, scopeTotal, err := scopeProblems(actorCtx, q, sqlc.ScopeProblemsParams{ViewerID: &u.Id, Now: now, OrganizationID: &org, PageSize: 1, PageOffset: 100})
			require.NoError(t, err)
			require.Empty(t, ep)
			require.EqualValues(t, count, scopeTotal)
			ec, scopeTotal, err := scopeContests(actorCtx, q, sqlc.ScopeContestsParams{ViewerID: &u.Id, Now: now, OrganizationID: &org, PageSize: 1, PageOffset: 100})
			require.NoError(t, err)
			require.Empty(t, ec)
			require.EqualValues(t, count, scopeTotal)
			// The repositories must drop the null row of a page beyond the end
			// rather than decode it into an empty resource.
			beyondEnd := paging.Request{Page: 100, PageSize: 1}
			tp, total, err = teamsRepo.GetTeamProblems(actorCtx, closed, u.Id, beyondEnd)
			require.NoError(t, err)
			require.Empty(t, tp)
			require.EqualValues(t, count, total)
			tc, total, err = teamsRepo.GetTeamContests(actorCtx, closed, u.Id, beyondEnd)
			require.NoError(t, err)
			require.Empty(t, tc)
			require.EqualValues(t, count, total)
			// Authorization queries must contain neither transactions nor row locks.
			for _, query := range recorder.recorded() {
				upper := strings.ToUpper(query)
				require.NotContains(t, upper, "FOR UPDATE")
				require.NotContains(t, upper, "FOR SHARE")
				require.NotContains(t, upper, "BEGIN")
			}
		})
	}
	// Public wrappers and ordinary filters must only narrow visibility.
	outsider := users["outsider"]
	actorCtx := m.WithUser(ctx, outsider)
	list, err := postgres.NewProblemsRepo(pool).ListProblems(actorCtx, &problemsmodel.ProblemsFilter{OrganizationID: &org, ViewerID: &outsider.Id, Page: 1, PageSize: 100})
	require.NoError(t, err)
	require.Len(t, list.Problems, 1)
	contests, err := postgres.NewContestsRepo(pool).ListOrganizationContests(actorCtx, "parity_"+suffix, "", 1, 100)
	require.NoError(t, err)
	require.Len(t, contests.Contests, 1)
	_, err = postgres.NewContestsRepo(pool).ListPublicContests(actorCtx, contestsmodel.PublicContestsFilter{})
	require.NoError(t, err)

	t.Run("NULL viewer never disables visibility", func(t *testing.T) {
		ps, total, err := scopeProblems(ctx, sqlc.New(pool), sqlc.ScopeProblemsParams{Now: now, OrganizationID: &org, PageSize: 100})
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, ps, 1)
		require.Equal(t, problemIDs[0], ps[0])
		cs, total, err := scopeContests(ctx, sqlc.New(pool), sqlc.ScopeContestsParams{Now: now, OrganizationID: &org, PageSize: 100})
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, cs, 1)
		require.Equal(t, contestIDs[0], cs[0])
		teams, err := sqlc.New(pool).ScopeTeams(ctx, sqlc.ScopeTeamsParams{Now: now, OrganizationID: &org, PageSize: 100})
		require.NoError(t, err)
		require.Len(t, teams, 1)
		require.Zero(t, teams[0].TotalItems)
		require.True(t, emptyScopeRow(teams[0].Resource))
	})

	t.Run("authorization does not wait for row locks", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { require.NoError(t, tx.Rollback(ctx)) }()
		lock := func(query string, args ...any) {
			t.Helper()
			rows, err := tx.Query(ctx, query, args...)
			require.NoError(t, err)
			rows.Close()
			require.NoError(t, rows.Err())
		}
		actor := users["multiple teams"]
		lock(`SELECT id FROM users WHERE id=$1 FOR UPDATE`, actor.Id)
		lock(`SELECT id FROM organizations WHERE id=$1 FOR UPDATE`, org)
		lock(`SELECT id FROM problems WHERE organization_id=$1 FOR UPDATE`, org)
		lock(`SELECT id FROM contests WHERE organization_id=$1 FOR UPDATE`, org)
		lock(`SELECT id FROM teams WHERE organization_id=$1 FOR UPDATE`, org)
		lock(`SELECT team_id FROM team_members WHERE user_id=$1 FOR UPDATE`, actor.Id)
		readCtx, cancel := context.WithTimeout(m.WithUser(ctx, actor), 2*time.Second)
		defer cancel()
		legacy := access.NewPermissionsUseCase(nil, postgres.NewUsersRepo(pool), nil, nil, nil)
		permissions := access.NewMetadataPermissions(legacy, postgres.NewPolicyFactsRepo(pool))
		allowed, err := permissions.HasProblemPermission(readCtx, problemIDs[1], actor.Id, m.ActionViewProblem)
		require.NoError(t, err)
		require.True(t, allowed)
		allowed, err = permissions.HasContestPermission(readCtx, contestIDs[1], actor.Id, m.ActionViewContest)
		require.NoError(t, err)
		require.True(t, allowed)
		teamUC := organizations.NewTeamsUseCase(postgres.NewTeamsRepo(pool), permissions, nil, permissions)
		_, err = teamUC.GetTeam(readCtx, secret, actor.Id)
		require.NoError(t, err)
		_, err = postgres.NewProblemsRepo(pool).ListProblems(readCtx, &problemsmodel.ProblemsFilter{OrganizationID: &org, ViewerID: &actor.Id, Page: 1, PageSize: 100})
		require.NoError(t, err)
		_, err = postgres.NewContestsRepo(pool).ListOrganizationContests(readCtx, "parity_"+suffix, "", 1, 100)
		require.NoError(t, err)
	})

	// Simulate future enum values in the disposable test database. Unknown grants
	// must deny even public metadata, rather than silently becoming absent grants.
	t.Run("unknown roles fail closed", func(t *testing.T) {
		for _, enum := range []string{"problem_role", "contest_role", "team_role", "organization_role", "problem_permission", "user_role"} {
			exec("ALTER TYPE " + enum + " ADD VALUE IF NOT EXISTS '__policy_unknown'")
		}
		actor := users["outsider"]
		actorCtx := m.WithUser(ctx, actor)
		facts := postgres.NewPolicyFactsRepo(pool)
		permissions := access.NewMetadataPermissions(access.NewPermissionsUseCase(nil, postgres.NewUsersRepo(pool), nil, nil, nil), facts)
		verify := func() {
			t.Helper()
			pf, err := facts.LoadProblemFacts(actorCtx, actor.Id, problemIDs)
			require.NoError(t, err)
			cf, err := facts.LoadContestFacts(actorCtx, actor.Id, contestIDs)
			require.NoError(t, err)
			ps, _, err := scopeProblems(actorCtx, sqlc.New(pool), sqlc.ScopeProblemsParams{ViewerID: &actor.Id, Now: now, OrganizationID: &org, PageSize: 100})
			require.NoError(t, err)
			cs, _, err := scopeContests(actorCtx, sqlc.New(pool), sqlc.ScopeContestsParams{ViewerID: &actor.Id, Now: now, OrganizationID: &org, PageSize: 100})
			require.NoError(t, err)
			gotP, gotC := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
			for _, p := range ps {
				gotP[p] = true
			}
			for _, c := range cs {
				gotC[c] = true
			}
			in := m.PolicyInput{Actor: actor, Now: now}
			for i, p := range problemIDs {
				require.Equal(t, access.RequireProblem(in, pf[p], m.ActionViewProblem) == nil, gotP[p])
				require.Equal(t, access.RequireContest(in, cf[contestIDs[i]], m.ActionViewContest) == nil, gotC[contestIDs[i]])
			}
			require.NotContains(t, gotP, problemIDs[0])
			require.NotContains(t, gotC, contestIDs[0])
		}
		exec(`INSERT INTO problem_members(problem_id,user_id,role) VALUES($1,$2,'__policy_unknown')`, problemIDs[0], actor.Id)
		exec(`INSERT INTO contest_members(contest_id,user_id,role) VALUES($1,$2,'__policy_unknown')`, contestIDs[0], actor.Id)
		verify()
		exec(`DELETE FROM problem_members WHERE user_id=$1`, actor.Id)
		exec(`DELETE FROM contest_members WHERE user_id=$1`, actor.Id)
		exec(`INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,'__policy_unknown')`, org, actor.Id)
		verify()
		exec(`DELETE FROM organization_members WHERE user_id=$1`, actor.Id)
		exec(`INSERT INTO team_members(team_id,user_id,role) VALUES($1,$2,'__policy_unknown')`, wrong, actor.Id)
		verify() // Even a malformed grant from a foreign team fails closed.
		exec(`DELETE FROM team_members WHERE user_id=$1`, actor.Id)
		exec(`INSERT INTO team_members(team_id,user_id,role) VALUES($1,$2,'member')`, closed, actor.Id)
		exec(`UPDATE problem_teams SET permission='__policy_unknown' WHERE team_id=$1`, closed)
		exec(`UPDATE contest_teams SET role='__policy_unknown' WHERE team_id=$1`, closed)
		verify()
		exec(`UPDATE problem_teams SET permission='read' WHERE team_id=$1`, closed)
		exec(`UPDATE contest_teams SET role='participant' WHERE team_id=$1`, closed)
		exec(`UPDATE team_members SET role='__policy_unknown' WHERE team_id=$1 AND user_id=$2`, closed, actor.Id)
		allowed, err := permissions.HasTeamPermission(actorCtx, closed, actor.Id, m.ActionViewTeam)
		require.NoError(t, err)
		require.False(t, allowed)
		ts, _, err := postgres.NewTeamsRepo(pool).ListTeams(actorCtx, &organizationsmodel.TeamsFilter{OrganizationID: &org, Page: 1, PageSize: 100}, actor.Id)
		require.NoError(t, err)
		for _, team := range ts {
			require.NotEqual(t, closed, team.ID)
		}
		exec(`DELETE FROM team_members WHERE user_id=$1`, actor.Id)
		exec(`UPDATE users SET role='__policy_unknown' WHERE id=$1`, actor.Id)
		actor.Role = "__policy_unknown"
		verify()
		allowed, err = permissions.HasProblemPermission(actorCtx, problemIDs[0], actor.Id, m.ActionViewProblem)
		require.False(t, allowed)
		require.ErrorIs(t, err, pkg.ErrUnauthenticated)
		exec(`UPDATE users SET role='user' WHERE id=$1`, actor.Id)
	})

}
