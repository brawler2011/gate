package pg

import (
	"context"
	"fmt"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TeamsRepo struct {
	db *pgxpool.Pool
	q  *sqlc.Queries
}

func NewTeamsRepo(db *pgxpool.Pool) interfaces.TeamsRepo {
	return &TeamsRepo{
		db: db,
		q:  sqlc.New(db),
	}
}

func (r *TeamsRepo) WithTx(tx pgx.Tx) interfaces.TeamsRepo {
	return &TeamsRepo{
		db: r.db,
		q:  r.q.WithTx(tx),
	}
}

func (r *TeamsRepo) CreateTeam(ctx context.Context, input *models.CreateTeamInput) (*models.Team, error) {
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

	return &models.Team{
		ID:             team.ID,
		OrganizationID: team.OrganizationID,
		Name:           team.Name,
		Slug:           team.Slug,
		Description:    team.Description,
		Privacy:        models.TeamPrivacy(team.Privacy),
		ParentTeamID:   pgtypeToNullableUUID(team.ParentTeamID),
		CreatedAt:      team.CreatedAt,
		UpdatedAt:      team.UpdatedAt,
	}, nil
}

func (r *TeamsRepo) GetTeamByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	team, err := r.q.GetTeamByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get team by ID: %w", err)
	}

	return &models.Team{
		ID:             team.ID,
		OrganizationID: team.OrganizationID,
		Name:           team.Name,
		Slug:           team.Slug,
		Description:    team.Description,
		Privacy:        models.TeamPrivacy(team.Privacy),
		ParentTeamID:   pgtypeToNullableUUID(team.ParentTeamID),
		CreatedAt:      team.CreatedAt,
		UpdatedAt:      team.UpdatedAt,
	}, nil
}

func (r *TeamsRepo) GetTeamBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Team, error) {
	team, err := r.q.GetTeamBySlug(ctx, sqlc.GetTeamBySlugParams{
		OrganizationID: orgID,
		Slug:           slug,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get team by slug: %w", err)
	}

	return &models.Team{
		ID:             team.ID,
		OrganizationID: team.OrganizationID,
		Name:           team.Name,
		Slug:           team.Slug,
		Description:    team.Description,
		Privacy:        models.TeamPrivacy(team.Privacy),
		ParentTeamID:   pgtypeToNullableUUID(team.ParentTeamID),
		CreatedAt:      team.CreatedAt,
		UpdatedAt:      team.UpdatedAt,
	}, nil
}

func (r *TeamsRepo) ListOrganizationTeams(ctx context.Context, orgID uuid.UUID) ([]models.Team, error) {
	teams, err := r.q.ListOrganizationTeams(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization teams: %w", err)
	}

	result := make([]models.Team, len(teams))
	for i, t := range teams {
		result[i] = models.Team{
			ID:             t.ID,
			OrganizationID: t.OrganizationID,
			Name:           t.Name,
			Slug:           t.Slug,
			Description:    t.Description,
			Privacy:        models.TeamPrivacy(t.Privacy),
			ParentTeamID:   pgtypeToNullableUUID(t.ParentTeamID),
			CreatedAt:      t.CreatedAt,
			UpdatedAt:      t.UpdatedAt,
		}
	}

	return result, nil
}

func (r *TeamsRepo) UpdateTeam(ctx context.Context, id uuid.UUID, input *models.UpdateTeamInput) error {
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

func (r *TeamsRepo) AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role models.TeamRole) error {
	if err := r.q.AddTeamMember(ctx, sqlc.AddTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
		Role:   sqlc.TeamRole(role),
	}); err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}
	return nil
}

func (r *TeamsRepo) GetTeamMember(ctx context.Context, teamID, userID uuid.UUID) (*models.TeamMember, error) {
	member, err := r.q.GetTeamMember(ctx, sqlc.GetTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get team member: %w", err)
	}

	return &models.TeamMember{
		TeamID:    member.TeamID,
		UserID:    member.UserID,
		Role:      models.TeamRole(member.Role),
		CreatedAt: member.CreatedAt,
	}, nil
}

func (r *TeamsRepo) ListTeamMembers(ctx context.Context, teamID uuid.UUID) ([]models.TeamMember, error) {
	members, err := r.q.ListTeamMembers(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list team members: %w", err)
	}

	result := make([]models.TeamMember, len(members))
	for i, m := range members {
		result[i] = models.TeamMember{
			TeamID:    m.TeamID,
			UserID:    m.UserID,
			Role:      models.TeamRole(m.Role),
			Username:  m.Username,
			Email:     m.Email,
			CreatedAt: m.CreatedAt,
		}
	}

	return result, nil
}

func (r *TeamsRepo) UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role models.TeamRole) error {
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

func (r *TeamsRepo) GetUserTeams(ctx context.Context, userID uuid.UUID) ([]models.Team, error) {
	teams, err := r.q.GetUserTeams(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user teams: %w", err)
	}

	result := make([]models.Team, len(teams))
	for i, t := range teams {
		result[i] = models.Team{
			ID:             t.ID,
			OrganizationID: t.OrganizationID,
			Name:           t.Name,
			Slug:           t.Slug,
			Description:    t.Description,
			Privacy:        models.TeamPrivacy(t.Privacy),
			ParentTeamID:   pgtypeToNullableUUID(t.ParentTeamID),
			CreatedAt:      t.CreatedAt,
			UpdatedAt:      t.UpdatedAt,
		}
	}

	return result, nil
}

func (r *TeamsRepo) GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]models.Team, error) {
	teams, err := r.q.GetUserTeamsByOrganization(ctx, sqlc.GetUserTeamsByOrganizationParams{
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user teams by organization: %w", err)
	}

	result := make([]models.Team, len(teams))
	for i, t := range teams {
		result[i] = models.Team{
			ID:             t.ID,
			OrganizationID: t.OrganizationID,
			Name:           t.Name,
			Slug:           t.Slug,
			Description:    t.Description,
			Privacy:        models.TeamPrivacy(t.Privacy),
			ParentTeamID:   pgtypeToNullableUUID(t.ParentTeamID),
			CreatedAt:      t.CreatedAt,
			UpdatedAt:      t.UpdatedAt,
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
