package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TeamsUseCase struct {
	repo          interfaces.TeamsRepo
	orgsRepo      interfaces.OrganizationsRepo
	usersRepo     interfaces.UsersRepo
	permissionsUC *PermissionsUseCase
	transactor    interfaces.Transactor
}

func NewTeamsUseCase(
	repo interfaces.TeamsRepo,
	orgsRepo interfaces.OrganizationsRepo,
	usersRepo interfaces.UsersRepo,
	permissionsUC *PermissionsUseCase,
	transactor interfaces.Transactor,
) *TeamsUseCase {
	return &TeamsUseCase{
		repo:          repo,
		orgsRepo:      orgsRepo,
		usersRepo:     usersRepo,
		permissionsUC: permissionsUC,
		transactor:    transactor,
	}
}

func (uc *TeamsUseCase) CreateTeam(ctx context.Context, input *models.CreateTeamInput, creatorID uuid.UUID) (*models.Team, error) {
	// Check if user has permission to create teams in this organization
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, input.OrganizationID, creatorID, models.ActionManageOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, errors.New("access denied: need org admin/owner role to create teams")
	}

	var team *models.Team
	err = uc.transactor.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		// Create team
		team, err = uc.repo.CreateTeam(txCtx, input)
		if err != nil {
			return fmt.Errorf("create team: %w", err)
		}

		// Add creator as maintainer
		err = uc.repo.AddTeamMember(txCtx, team.ID, creatorID, models.TeamRoleMaintainer)
		if err != nil {
			return fmt.Errorf("add creator as maintainer: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return team, nil
}

func (uc *TeamsUseCase) GetTeam(ctx context.Context, teamID, userID uuid.UUID) (*models.Team, error) {
	team, err := uc.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Check if user can view this team
	// User can view if:
	// 1. They are org admin/owner
	// 2. They are team member
	// 3. Team is not secret
	hasOrgAccess, _ := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, userID, models.ActionViewOrganization)
	if !hasOrgAccess {
		return nil, errors.New("access denied: not a member of organization")
	}

	if team.Privacy == models.TeamPrivacySecret {
		// For secret teams, must be a member
		_, err := uc.repo.GetTeamMember(ctx, teamID, userID)
		if err != nil {
			// Not a team member, check if org admin
			hasManageAccess, _ := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, userID, models.ActionManageOrganization)
			if !hasManageAccess {
				return nil, errors.New("access denied: team is secret")
			}
		}
	}

	return team, nil
}

func (uc *TeamsUseCase) ListOrganizationTeams(ctx context.Context, orgID, userID uuid.UUID) ([]models.Team, error) {
	// Check org access
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, orgID, userID, models.ActionViewOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, errors.New("access denied")
	}

	teams, err := uc.repo.ListOrganizationTeams(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// Filter secret teams if user is not a member
	hasManageAccess, _ := uc.permissionsUC.HasOrganizationPermission(ctx, orgID, userID, models.ActionManageOrganization)
	
	visibleTeams := make([]models.Team, 0, len(teams))
	for _, team := range teams {
		if team.Privacy == models.TeamPrivacySecret && !hasManageAccess {
			// Check if user is team member
			_, err := uc.repo.GetTeamMember(ctx, team.ID, userID)
			if err != nil {
				continue // Skip secret team
			}
		}
		visibleTeams = append(visibleTeams, team)
	}

	return visibleTeams, nil
}

func (uc *TeamsUseCase) UpdateTeam(ctx context.Context, teamID, userID uuid.UUID, input *models.UpdateTeamInput) error {
	team, err := uc.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Check if user can manage team (org admin or team maintainer)
	hasOrgAccess, _ := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, userID, models.ActionManageOrganization)
	
	teamMember, err := uc.repo.GetTeamMember(ctx, teamID, userID)
	isTeamMaintainer := err == nil && teamMember.Role == models.TeamRoleMaintainer

	if !hasOrgAccess && !isTeamMaintainer {
		return errors.New("access denied: need org admin or team maintainer role")
	}

	return uc.repo.UpdateTeam(ctx, teamID, input)
}

func (uc *TeamsUseCase) DeleteTeam(ctx context.Context, teamID, userID uuid.UUID) error {
	team, err := uc.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Only org admins can delete teams
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, userID, models.ActionManageOrganization)
	if err != nil {
		return fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return errors.New("access denied: only org admins can delete teams")
	}

	return uc.repo.DeleteTeam(ctx, teamID)
}

// Member management

func (uc *TeamsUseCase) AddTeamMember(ctx context.Context, input *models.AddTeamMemberInput, requestUserID uuid.UUID) error {
	team, err := uc.repo.GetTeamByID(ctx, input.TeamID)
	if err != nil {
		return err
	}

	// Check permissions
	hasOrgAccess, _ := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, requestUserID, models.ActionManageOrganization)
	
	teamMember, err := uc.repo.GetTeamMember(ctx, input.TeamID, requestUserID)
	isTeamMaintainer := err == nil && teamMember.Role == models.TeamRoleMaintainer

	if !hasOrgAccess && !isTeamMaintainer {
		return errors.New("access denied: need org admin or team maintainer role")
	}

	// Verify user exists
	_, err = uc.usersRepo.GetUserById(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Verify user is org member
	_, err = uc.orgsRepo.GetMember(ctx, team.OrganizationID, input.UserID)
	if err != nil {
		return errors.New("user must be organization member first")
	}

	return uc.repo.AddTeamMember(ctx, input.TeamID, input.UserID, input.Role)
}

func (uc *TeamsUseCase) ListTeamMembers(ctx context.Context, teamID, requestUserID uuid.UUID) ([]models.TeamMember, error) {
	team, err := uc.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Check if user can view team members
	hasOrgAccess, _ := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, requestUserID, models.ActionViewOrganization)
	
	_, err = uc.repo.GetTeamMember(ctx, teamID, requestUserID)
	isTeamMember := err == nil

	if !hasOrgAccess && !isTeamMember {
		return nil, errors.New("access denied")
	}

	return uc.repo.ListTeamMembers(ctx, teamID)
}

func (uc *TeamsUseCase) UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role models.TeamRole, requestUserID uuid.UUID) error {
	team, err := uc.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Check permissions
	hasOrgAccess, _ := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, requestUserID, models.ActionManageOrganization)
	
	teamMember, err := uc.repo.GetTeamMember(ctx, teamID, requestUserID)
	isTeamMaintainer := err == nil && teamMember.Role == models.TeamRoleMaintainer

	if !hasOrgAccess && !isTeamMaintainer {
		return errors.New("access denied: need org admin or team maintainer role")
	}

	return uc.repo.UpdateTeamMemberRole(ctx, teamID, userID, role)
}

func (uc *TeamsUseCase) RemoveTeamMember(ctx context.Context, teamID, userID, requestUserID uuid.UUID) error {
	team, err := uc.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Check permissions
	hasOrgAccess, _ := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, requestUserID, models.ActionManageOrganization)
	
	teamMember, err := uc.repo.GetTeamMember(ctx, teamID, requestUserID)
	isTeamMaintainer := err == nil && teamMember.Role == models.TeamRoleMaintainer

	// Users can remove themselves
	isSelf := userID == requestUserID

	if !hasOrgAccess && !isTeamMaintainer && !isSelf {
		return errors.New("access denied")
	}

	return uc.repo.RemoveTeamMember(ctx, teamID, userID)
}

func (uc *TeamsUseCase) GetUserTeams(ctx context.Context, userID uuid.UUID) ([]models.Team, error) {
	return uc.repo.GetUserTeams(ctx, userID)
}
