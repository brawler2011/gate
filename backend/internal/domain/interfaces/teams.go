package interfaces

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TeamsRepo interface {
	CreateTeam(ctx context.Context, input *models.CreateTeamInput) (*models.Team, error)
	GetTeamByID(ctx context.Context, id uuid.UUID) (*models.Team, error)
	GetTeamBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Team, error)
	ListOrganizationTeams(ctx context.Context, orgID uuid.UUID) ([]models.Team, error)
	UpdateTeam(ctx context.Context, id uuid.UUID, input *models.UpdateTeamInput) error
	DeleteTeam(ctx context.Context, id uuid.UUID) error

	// Member management
	AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role models.TeamRole) error
	GetTeamMember(ctx context.Context, teamID, userID uuid.UUID) (*models.TeamMember, error)
	ListTeamMembers(ctx context.Context, teamID uuid.UUID) ([]models.TeamMember, error)
	UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role models.TeamRole) error
	RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error

	// User's teams
	GetUserTeams(ctx context.Context, userID uuid.UUID) ([]models.Team, error)
	GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]models.Team, error)

	WithTx(tx pgx.Tx) TeamsRepo
}

type TeamsUC interface {
	CreateTeam(ctx context.Context, input *models.CreateTeamInput, creatorID uuid.UUID) (*models.Team, error)
	GetTeam(ctx context.Context, teamID, userID uuid.UUID) (*models.Team, error)
	ListOrganizationTeams(ctx context.Context, orgID, userID uuid.UUID) ([]models.Team, error)
	UpdateTeam(ctx context.Context, teamID, userID uuid.UUID, input *models.UpdateTeamInput) error
	DeleteTeam(ctx context.Context, teamID, userID uuid.UUID) error

	// Member management
	AddTeamMember(ctx context.Context, input *models.AddTeamMemberInput, requestUserID uuid.UUID) error
	ListTeamMembers(ctx context.Context, teamID, requestUserID uuid.UUID) ([]models.TeamMember, error)
	UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role models.TeamRole, requestUserID uuid.UUID) error
	RemoveTeamMember(ctx context.Context, teamID, userID, requestUserID uuid.UUID) error
	GetUserTeams(ctx context.Context, userID uuid.UUID) ([]models.Team, error)
}
