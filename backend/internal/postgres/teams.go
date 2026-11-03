package postgres

import (
	"context"
	"fmt"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TeamsRepo struct {
	db *pgxpool.Pool
	q  *sqlc.Queries
}

func NewTeamsRepo(db *pgxpool.Pool) *TeamsRepo {
	return &TeamsRepo{
		db: db,
		q:  sqlc.New(db),
	}
}

func (r *TeamsRepo) WithTx(tx pgx.Tx) *TeamsRepo {
	return &TeamsRepo{
		db: r.db,
		q:  r.q.WithTx(tx),
	}
}

func (r *TeamsRepo) CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput) (*organizationsmodel.Team, error) {
	team, err := r.q.CreateTeam(ctx, sqlc.CreateTeamParams{
		ID:             uuid.New(),
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Slug:           input.Slug,
		Description:    input.Description,
		Privacy:        string(input.Privacy),
		ParentTeamID:   nullableUUIDToPgtype(input.ParentTeamID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	return &organizationsmodel.Team{
		ID:             team.ID,
		OrganizationID: team.OrganizationID,
		Name:           team.Name,
		Slug:           team.Slug,
		Description:    team.Description,
		Privacy:        organizationsmodel.TeamPrivacy(team.Privacy),
		ParentTeamID:   pgtypeToNullableUUID(team.ParentTeamID),
		CreatedAt:      team.CreatedAt,
		UpdatedAt:      team.UpdatedAt,
	}, nil
}

func (r *TeamsRepo) GetTeamByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.Team, error) {
	team, err := r.q.GetTeamByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get team by ID: %w", err)
	}

	return &organizationsmodel.Team{
		ID:             team.ID,
		OrganizationID: team.OrganizationID,
		Name:           team.Name,
		Slug:           team.Slug,
		Description:    team.Description,
		Privacy:        organizationsmodel.TeamPrivacy(team.Privacy),
		ParentTeamID:   pgtypeToNullableUUID(team.ParentTeamID),
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
		return nil, fmt.Errorf("failed to get team by slug: %w", err)
	}

	return &organizationsmodel.Team{
		ID:             team.ID,
		OrganizationID: team.OrganizationID,
		Name:           team.Name,
		Slug:           team.Slug,
		Description:    team.Description,
		Privacy:        organizationsmodel.TeamPrivacy(team.Privacy),
		ParentTeamID:   pgtypeToNullableUUID(team.ParentTeamID),
		CreatedAt:      team.CreatedAt,
		UpdatedAt:      team.UpdatedAt,
	}, nil
}

func (r *TeamsRepo) ListOrganizationTeams(ctx context.Context, orgID uuid.UUID) ([]organizationsmodel.Team, error) {
	teams, err := r.q.ListOrganizationTeams(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization teams: %w", err)
	}

	result := make([]organizationsmodel.Team, len(teams))
	for i, t := range teams {
		result[i] = organizationsmodel.Team{
			ID:             t.ID,
			OrganizationID: t.OrganizationID,
			Name:           t.Name,
			Slug:           t.Slug,
			Description:    t.Description,
			Privacy:        organizationsmodel.TeamPrivacy(t.Privacy),
			ParentTeamID:   pgtypeToNullableUUID(t.ParentTeamID),
			CreatedAt:      t.CreatedAt,
			UpdatedAt:      t.UpdatedAt,
		}
	}

	return result, nil
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
	if input.Privacy != nil {
		privacy := string(*input.Privacy)
		params.Privacy = &privacy
	}

	if err := r.q.UpdateTeam(ctx, params); err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}

	return nil
}

func (r *TeamsRepo) DeleteTeam(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteTeam(ctx, id); err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	return nil
}

func (r *TeamsRepo) AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole) error {
	if err := r.q.AddTeamMember(ctx, sqlc.AddTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
		Role:   sqlc.TeamRole(role),
	}); err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}
	return nil
}

func (r *TeamsRepo) GetTeamMember(ctx context.Context, teamID, userID uuid.UUID) (*organizationsmodel.TeamMember, error) {
	member, err := r.q.GetTeamMember(ctx, sqlc.GetTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get team member: %w", err)
	}

	return &organizationsmodel.TeamMember{
		TeamID:    member.TeamID,
		UserID:    member.UserID,
		Role:      organizationsmodel.TeamRole(member.Role),
		CreatedAt: member.CreatedAt,
	}, nil
}

func (r *TeamsRepo) ListTeamMembers(ctx context.Context, teamID uuid.UUID) ([]organizationsmodel.TeamMember, error) {
	members, err := r.q.ListTeamMembers(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list team members: %w", err)
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
			Role:      organizationsmodel.TeamRole(m.Role),
			Username:  m.Username,
			Email:     email,
			CreatedAt: m.CreatedAt,
		}
	}

	return result, nil
}

func (r *TeamsRepo) UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole) error {
	if err := r.q.UpdateTeamMemberRole(ctx, sqlc.UpdateTeamMemberRoleParams{
		TeamID: teamID,
		UserID: userID,
		Role:   sqlc.TeamRole(role),
	}); err != nil {
		return fmt.Errorf("failed to update team member role: %w", err)
	}
	return nil
}

func (r *TeamsRepo) RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error {
	if err := r.q.RemoveTeamMember(ctx, sqlc.RemoveTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("failed to remove team member: %w", err)
	}
	return nil
}

func (r *TeamsRepo) GetUserTeams(ctx context.Context, userID uuid.UUID) ([]organizationsmodel.Team, error) {
	teams, err := r.q.GetUserTeams(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user teams: %w", err)
	}

	result := make([]organizationsmodel.Team, len(teams))
	for i, t := range teams {
		result[i] = organizationsmodel.Team{
			ID:             t.ID,
			OrganizationID: t.OrganizationID,
			Name:           t.Name,
			Slug:           t.Slug,
			Description:    t.Description,
			Privacy:        organizationsmodel.TeamPrivacy(t.Privacy),
			ParentTeamID:   pgtypeToNullableUUID(t.ParentTeamID),
			CreatedAt:      t.CreatedAt,
			UpdatedAt:      t.UpdatedAt,
		}
	}

	return result, nil
}

func (r *TeamsRepo) GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]organizationsmodel.Team, error) {
	teams, err := r.q.GetUserTeamsByOrganization(ctx, sqlc.GetUserTeamsByOrganizationParams{
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user teams by organization: %w", err)
	}

	result := make([]organizationsmodel.Team, len(teams))
	for i, t := range teams {
		result[i] = organizationsmodel.Team{
			ID:             t.ID,
			OrganizationID: t.OrganizationID,
			Name:           t.Name,
			Slug:           t.Slug,
			Description:    t.Description,
			Privacy:        organizationsmodel.TeamPrivacy(t.Privacy),
			ParentTeamID:   pgtypeToNullableUUID(t.ParentTeamID),
			CreatedAt:      t.CreatedAt,
			UpdatedAt:      t.UpdatedAt,
		}
	}

	return result, nil
}

func (r *TeamsRepo) GetTeamContests(ctx context.Context, teamID uuid.UUID) ([]contestsmodel.Contest, error) {
	contests, err := r.q.GetTeamContests(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team contests: %w", err)
	}

	result := make([]contestsmodel.Contest, len(contests))
	for i, c := range contests {
		result[i] = contestsmodel.Contest{
			ID:             c.ID,
			OrganizationID: c.OrganizationID,
			OwnerID:        pgtypeToNullableUUID(c.OwnerID),
			Visibility:     string(c.Visibility),
			Title:          c.Title,
			Description:    c.Description,
			StartTime:      pgTimestamptzToTimePtr(c.StartTime),
			EndTime:        pgTimestamptzToTimePtr(c.EndTime),
			CreatedAt:      c.CreatedAt,
			UpdatedAt:      c.UpdatedAt,
		}
	}

	return result, nil
}

func (r *TeamsRepo) GetTeamProblems(ctx context.Context, teamID uuid.UUID) ([]problemsmodel.Problem, error) {
	problems, err := r.q.GetTeamProblems(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team problems: %w", err)
	}

	result := make([]problemsmodel.Problem, len(problems))
	for i, p := range problems {
		result[i] = problemsmodel.Problem{
			ID:             p.ID,
			OrganizationID: p.OrganizationID,
			OwnerID:        pgtypeToNullableUUID(p.OwnerID),
			Visibility:     string(p.Visibility),
			Title:          p.Title,
			ShortName:      p.ShortName,
			TimeLimitMs:    int(p.TimeLimitMs),
			MemoryLimitMb:  int(p.MemoryLimitMb),
			IsTemplate:     p.IsTemplate,
			CreatedAt:      p.CreatedAt,
			UpdatedAt:      p.UpdatedAt,
		}
	}

	return result, nil
}

// Helper functions
func pgtypeToNullableUUID(pgu pgtype.UUID) *uuid.UUID {
	if !pgu.Valid {
		return nil
	}
	u := uuid.UUID(pgu.Bytes)
	return &u
}

func (r *TeamsRepo) LockTeam(ctx context.Context, id uuid.UUID) error {
	_, err := r.q.LockTeam(ctx, id)
	return HandlePgErr(err)
}
