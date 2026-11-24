package postgres

import (
	"context"
	"encoding/json"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	paging "github.com/brawler2011/gate/backend/internal/page"

	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TeamsRepo struct {
	q *sqlc.Queries
}

func NewTeamsRepo(db *pgxpool.Pool) *TeamsRepo {
	return &TeamsRepo{
		q: sqlc.New(db),
	}
}

func (r *TeamsRepo) CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput) (*organizationsmodel.Team, error) {
	team, err := r.q.CreateTeam(ctx, sqlc.CreateTeamParams{
		ID:             uuid.New(),
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Slug:           input.Slug,
		Description:    input.Description,
	})
	if err != nil {
		return nil, dbError("create team", err)
	}

	return &organizationsmodel.Team{
		ID:             team.ID,
		OrganizationID: team.OrganizationID,
		Name:           team.Name,
		Slug:           team.Slug,
		Description:    team.Description,
		CreatedAt:      team.CreatedAt,
		UpdatedAt:      team.UpdatedAt,
	}, nil
}

func (r *TeamsRepo) GetTeamByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.Team, error) {
	team, err := r.q.GetTeamByID(ctx, id)
	if err != nil {
		return nil, dbError("get team by ID", err)
	}

	return &organizationsmodel.Team{
		ID:             team.ID,
		OrganizationID: team.OrganizationID,
		Name:           team.Name,
		Slug:           team.Slug,
		Description:    team.Description,
		CreatedAt:      team.CreatedAt,
		UpdatedAt:      team.UpdatedAt,
	}, nil
}

func (r *TeamsRepo) GetTeamBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*organizationsmodel.Team, error) {
	team, err := r.q.GetTeamBySlug(ctx, sqlc.GetTeamBySlugParams{
		OrganizationID: orgID,
		Slug:           slug,
	})
	if err != nil {
		return nil, dbError("get team by slug", err)
	}

	return &organizationsmodel.Team{
		ID:             team.ID,
		OrganizationID: team.OrganizationID,
		Name:           team.Name,
		Slug:           team.Slug,
		Description:    team.Description,
		CreatedAt:      team.CreatedAt,
		UpdatedAt:      team.UpdatedAt,
	}, nil
}

func (r *TeamsRepo) ListTeams(ctx context.Context, filter *organizationsmodel.TeamsFilter, userID uuid.UUID) ([]organizationsmodel.Team, int32, error) {
	viewer, all, err := (Access{q: r.q}).visibility(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.q.ScopeTeams(ctx, sqlc.ScopeTeamsParams{ViewerID: viewer, All: all, OrganizationID: filter.OrganizationID, Search: filter.Search, PageSize: filter.PageSize, PageOffset: (int64(filter.Page) - 1) * int64(filter.PageSize)})
	if err != nil {
		return nil, 0, dbError("list teams", err)
	}
	teams := make([]sqlc.Team, 0, len(rows))
	var total int32
	for _, row := range rows {
		total = safeInt32(row.TotalItems)
		if emptyScopeRow(row.Resource) {
			continue
		}
		var team sqlc.Team
		if err := json.Unmarshal(row.Resource, &team); err != nil {
			return nil, 0, dbError("decode scoped team", err)
		}
		teams = append(teams, team)
	}
	return mapTeams(teams), total, nil
}

func (r *TeamsRepo) UpdateTeam(ctx context.Context, id uuid.UUID, input *organizationsmodel.UpdateTeamInput) error {
	params := sqlc.UpdateTeamParams{
		ID: id,
	}

	if input.Name != nil {
		params.Name = input.Name
	}
	if input.Description != nil {
		params.Description = input.Description
	}

	if err := r.q.UpdateTeam(ctx, params); err != nil {
		return dbError("update team", err)
	}

	return nil
}

func (r *TeamsRepo) DeleteTeam(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteTeam(ctx, id); err != nil {
		return dbError("delete team", err)
	}
	return nil
}

func (r *TeamsRepo) AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role accessmodel.TeamRole) error {
	if err := r.q.AddTeamMember(ctx, sqlc.AddTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	}); err != nil {
		return dbError("add team member", err)
	}
	return nil
}

func (r *TeamsRepo) GetTeamMember(ctx context.Context, teamID, userID uuid.UUID) (*organizationsmodel.TeamMember, error) {
	member, err := r.q.GetTeamMember(ctx, sqlc.GetTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
	})
	if err != nil {
		return nil, dbError("get team member", err)
	}

	return &organizationsmodel.TeamMember{
		TeamID:    member.TeamID,
		UserID:    member.UserID,
		Role:      accessmodel.TeamRole(member.Role),
		CreatedAt: member.CreatedAt,
	}, nil
}

func (r *TeamsRepo) ListTeamMembers(ctx context.Context, teamID uuid.UUID, request paging.Request) ([]organizationsmodel.TeamMember, int32, error) {
	request, err := request.Normalize()
	if err != nil {
		return nil, 0, err
	}

	members, err := r.q.ListTeamMembers(ctx, sqlc.ListTeamMembersParams{TeamID: teamID, PageSize: request.PageSize, PageOffset: request.Offset()})
	if err != nil {
		return nil, 0, dbError("list team members", err)
	}

	result := make([]organizationsmodel.TeamMember, len(members))
	for i, m := range members {
		var email string
		if m.Email != nil {
			email = *m.Email
		}
		result[i] = organizationsmodel.TeamMember{
			TeamID:    m.TeamID,
			UserID:    m.UserID,
			Role:      accessmodel.TeamRole(m.Role),
			Username:  m.Username,
			Email:     email,
			CreatedAt: m.CreatedAt,
		}
	}

	count, err := r.q.CountTeamMembers(ctx, teamID)
	if err != nil {
		return nil, 0, dbError("list team members", err)
	}
	return result, safeInt32(count), nil
}

func (r *TeamsRepo) UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role accessmodel.TeamRole) error {
	if err := r.q.UpdateTeamMemberRole(ctx, sqlc.UpdateTeamMemberRoleParams{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	}); err != nil {
		return dbError("update team member role", err)
	}
	return nil
}

func (r *TeamsRepo) RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error {
	if err := r.q.RemoveTeamMember(ctx, sqlc.RemoveTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
	}); err != nil {
		return dbError("remove team member", err)
	}
	return nil
}

func (r *TeamsRepo) GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]organizationsmodel.Team, error) {
	teams, err := r.q.GetUserTeamsByOrganization(ctx, sqlc.GetUserTeamsByOrganizationParams{
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err != nil {
		return nil, dbError("get user teams by organization", err)
	}

	return mapTeams(teams), nil
}

func mapTeams(teams []sqlc.Team) []organizationsmodel.Team {
	result := make([]organizationsmodel.Team, len(teams))
	for i, t := range teams {
		result[i] = organizationsmodel.Team{
			ID:             t.ID,
			OrganizationID: t.OrganizationID,
			Name:           t.Name,
			Slug:           t.Slug,
			Description:    t.Description,
			CreatedAt:      t.CreatedAt,
			UpdatedAt:      t.UpdatedAt,
		}
	}

	return result
}
