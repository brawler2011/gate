package organizations

import (
	"context"
	"errors"
	"fmt"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type TeamsUseCase struct {
	repo          TeamsRepo
	permissionsUC PermissionsUC
	transactor    TeamTransactions
}

func NewTeamsUseCase(
	repo TeamsRepo,
	permissionsUC PermissionsUC,
	transactor TeamTransactions,
) *TeamsUseCase {
	return &TeamsUseCase{
		repo:          repo,
		permissionsUC: permissionsUC,
		transactor:    transactor,
	}
}

func (uc *TeamsUseCase) CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput, creatorID uuid.UUID) (*organizationsmodel.Team, error) {
	if user := accessmodel.UserFromContext(ctx); user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	} else if user.Id != creatorID {
		return nil, pkg.NoPermission
	}

	var team *organizationsmodel.Team
	err := uc.transactor.Within(ctx, func(tx TeamTxStore) error {
		if _, err := tx.GetOrganizationByID(ctx, input.OrganizationID); err != nil {
			return err
		}
		allowed, err := tx.HasOrganizationPermission(ctx, input.OrganizationID, creatorID, accessmodel.ActionManageOrganization)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.NoPermission
		}

		// Create team
		team, err = tx.CreateTeam(ctx, input)
		if err != nil {
			return fmt.Errorf("create team: %w", err)
		}

		// Add creator as maintainer
		err = tx.AddTeamMember(ctx, team.ID, creatorID, organizationsmodel.TeamRoleMaintainer)
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

func (uc *TeamsUseCase) GetTeam(ctx context.Context, teamID, userID uuid.UUID) (*organizationsmodel.Team, error) {
	if err := accessmodel.RequireCaller(ctx, userID); err != nil {
		return nil, err
	}
	team, err := uc.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Check if user can view this team
	// User can view if:
	// 1. They are org admin/owner
	// 2. They are team member
	// 3. Team is not secret
	hasOrgAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, userID, accessmodel.ActionViewOrganization)
	if err != nil {
		return nil, err
	}
	if !hasOrgAccess {
		return nil, pkg.NoPermission
	}

	if team.Privacy == organizationsmodel.TeamPrivacySecret {
		// For secret teams, must be a member
		_, err := uc.repo.GetTeamMember(ctx, teamID, userID)
		if err != nil {
			if !errors.Is(err, pkg.ErrNotFound) {
				return nil, err
			}
			hasManageAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, team.OrganizationID, userID, accessmodel.ActionManageOrganization)
			if err != nil {
				return nil, err
			}
			if !hasManageAccess {
				return nil, pkg.NoPermission
			}
		}
	}

	return team, nil
}

func (uc *TeamsUseCase) ListTeams(ctx context.Context, filter *organizationsmodel.TeamsFilter) (*organizationsmodel.TeamsList, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() || user.IsExpired() {
		return nil, pkg.ErrUnauthenticated
	}
	if filter == nil {
		return nil, pkg.ErrBadInput
	}
	normalized := *filter
	var err error
	normalized.Page, normalized.PageSize, err = paging.Normalize(normalized.Page, normalized.PageSize, 50)
	if err != nil {
		return nil, err
	}
	canManage := false
	if normalized.OrganizationID != nil {
		allowed, err := uc.permissionsUC.HasOrganizationPermission(ctx, *normalized.OrganizationID, user.Id, accessmodel.ActionViewOrganization)
		if err != nil {
			return nil, fmt.Errorf("check organization visibility: %w", err)
		}
		if !allowed {
			return nil, pkg.NoPermission
		}
		canManage, err = uc.permissionsUC.HasOrganizationPermission(ctx, *normalized.OrganizationID, user.Id, accessmodel.ActionManageOrganization)
		if err != nil {
			return nil, fmt.Errorf("check organization management: %w", err)
		}
	}
	teams, total, err := uc.repo.ListTeams(ctx, &normalized, user.Id, canManage)
	if err != nil {
		return nil, err
	}
	return &organizationsmodel.TeamsList{
		Teams: teams,
		Pagination: paging.Pagination{
			Page:  normalized.Page,
			Total: total,
		},
	}, nil
}

func (uc *TeamsUseCase) UpdateTeam(ctx context.Context, teamID, userID uuid.UUID, input *organizationsmodel.UpdateTeamInput) error {
	return uc.withinTeam(ctx, teamID, userID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		allowed, err := tx.HasOrganizationPermission(ctx, team.OrganizationID, userID, accessmodel.ActionManageOrganization)
		if err != nil {
			return err
		}
		if !allowed {
			member, err := tx.GetTeamMember(ctx, teamID, userID)
			if err != nil && !errors.Is(err, pkg.ErrNotFound) {
				return err
			}
			allowed = err == nil && member.Role == organizationsmodel.TeamRoleMaintainer
		}
		if !allowed {
			return pkg.NoPermission
		}
		return tx.UpdateTeam(ctx, teamID, input)
	})
}

func (uc *TeamsUseCase) DeleteTeam(ctx context.Context, teamID, userID uuid.UUID) error {
	return uc.withinTeam(ctx, teamID, userID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		allowed, err := tx.HasOrganizationPermission(ctx, team.OrganizationID, userID, accessmodel.ActionManageOrganization)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.NoPermission
		}
		return tx.DeleteTeam(ctx, teamID)
	})
}

func (uc *TeamsUseCase) AddTeamMember(ctx context.Context, input *organizationsmodel.AddTeamMemberInput, requestUserID uuid.UUID) error {
	return uc.withinTeam(ctx, input.TeamID, requestUserID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		allowed, err := tx.HasOrganizationPermission(ctx, team.OrganizationID, requestUserID, accessmodel.ActionManageOrganization)
		if err != nil {
			return err
		}
		if !allowed {
			member, err := tx.GetTeamMember(ctx, input.TeamID, requestUserID)
			if err != nil && !errors.Is(err, pkg.ErrNotFound) {
				return err
			}
			allowed = err == nil && member.Role == organizationsmodel.TeamRoleMaintainer
		}
		if !allowed {
			return pkg.NoPermission
		}
		if _, err := tx.GetMember(ctx, team.OrganizationID, input.UserID); err != nil {
			return err
		}
		return tx.AddTeamMember(ctx, input.TeamID, input.UserID, input.Role)
	})
}

func (uc *TeamsUseCase) ListTeamMembers(ctx context.Context, teamID, requestUserID uuid.UUID) ([]organizationsmodel.TeamMember, error) {
	if _, err := uc.GetTeam(ctx, teamID, requestUserID); err != nil {
		return nil, err
	}
	return uc.repo.ListTeamMembers(ctx, teamID)
}

func (uc *TeamsUseCase) UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole, requestUserID uuid.UUID) error {
	return uc.withinTeam(ctx, teamID, requestUserID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		allowed, err := tx.HasOrganizationPermission(ctx, team.OrganizationID, requestUserID, accessmodel.ActionManageOrganization)
		if err != nil {
			return err
		}
		if !allowed {
			member, err := tx.GetTeamMember(ctx, teamID, requestUserID)
			if err != nil && !errors.Is(err, pkg.ErrNotFound) {
				return err
			}
			allowed = err == nil && member.Role == organizationsmodel.TeamRoleMaintainer
		}
		if !allowed {
			return pkg.NoPermission
		}
		return tx.UpdateTeamMemberRole(ctx, teamID, userID, role)
	})
}

func (uc *TeamsUseCase) RemoveTeamMember(ctx context.Context, teamID, userID, requestUserID uuid.UUID) error {
	return uc.withinTeam(ctx, teamID, requestUserID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		allowed, err := tx.HasOrganizationPermission(ctx, team.OrganizationID, requestUserID, accessmodel.ActionManageOrganization)
		if err != nil {
			return err
		}
		if !allowed {
			member, err := tx.GetTeamMember(ctx, teamID, requestUserID)
			if err != nil && !errors.Is(err, pkg.ErrNotFound) {
				return err
			}
			allowed = err == nil && member.Role == organizationsmodel.TeamRoleMaintainer
		}
		allowed = allowed || userID == requestUserID
		if !allowed {
			return pkg.NoPermission
		}
		return tx.RemoveTeamMember(ctx, teamID, userID)
	})
}

func (uc *TeamsUseCase) GetTeamContests(ctx context.Context, teamID, requestUserID uuid.UUID) ([]contestsmodel.Contest, error) {
	if _, err := uc.GetTeam(ctx, teamID, requestUserID); err != nil {
		return nil, err
	}
	return uc.repo.GetTeamContests(ctx, teamID)
}

func (uc *TeamsUseCase) GetTeamProblems(ctx context.Context, teamID, requestUserID uuid.UUID) ([]problemsmodel.Problem, error) {
	if _, err := uc.GetTeam(ctx, teamID, requestUserID); err != nil {
		return nil, err
	}
	return uc.repo.GetTeamProblems(ctx, teamID)
}

func (uc *TeamsUseCase) withinTeam(ctx context.Context, id, actor uuid.UUID, fn func(TeamTxStore, *organizationsmodel.Team) error) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	if user.Id != actor {
		return pkg.NoPermission
	}
	return uc.transactor.Within(ctx, func(tx TeamTxStore) error {
		current, err := tx.GetTeamByID(ctx, id)
		if err != nil {
			return err
		}
		return fn(tx, current)
	})
}
