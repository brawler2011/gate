package organizations

import (
	"context"
	"errors"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type TeamVisibility interface {
	HasTeamPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.TeamAction) (bool, error)
}

type TeamsUseCase struct {
	visibility    TeamVisibility
	repo          TeamsRepo
	permissionsUC PermissionsUC
	transactor    TeamTransactions
}

func NewTeamsUseCase(
	repo TeamsRepo,
	permissionsUC PermissionsUC,
	transactor TeamTransactions,
	visibility TeamVisibility,
) *TeamsUseCase {
	return &TeamsUseCase{
		repo:          repo,
		visibility:    visibility,
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
	if err := access.CheckReadActor(ctx); err != nil {
		return nil, err
	}
	if err := accessmodel.RequireCaller(ctx, userID); err != nil {
		return nil, err
	}
	team, err := uc.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	allowed, err := uc.visibility.HasTeamPermission(ctx, teamID, userID, accessmodel.ActionViewTeam)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, pkg.NoPermission
	}

	return team, nil
}

func (uc *TeamsUseCase) ListTeams(ctx context.Context, filter *organizationsmodel.TeamsFilter) (*organizationsmodel.TeamsList, error) {
	user := accessmodel.UserFromContext(ctx)
	if err := access.CheckReadActor(ctx); err != nil {
		return nil, err
	}
	if user.IsGuest() {
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
	teams, total, err := uc.repo.ListTeams(ctx, &normalized, user.Id)
	if err != nil {
		return nil, err
	}
	return &organizationsmodel.TeamsList{
		Teams:      teams,
		Pagination: paging.NewPagination(normalized.Page, normalized.PageSize, total),
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

func (uc *TeamsUseCase) ListTeamMembers(ctx context.Context, teamID, requestUserID uuid.UUID, request paging.Request) (*paging.List[organizationsmodel.TeamMember], error) {
	if _, err := uc.GetTeam(ctx, teamID, requestUserID); err != nil {
		return nil, err
	}
	request, err := request.Normalize()
	if err != nil {
		return nil, err
	}
	items, total, err := uc.repo.ListTeamMembers(ctx, teamID, request)
	if err != nil {
		return nil, err
	}
	return &paging.List[organizationsmodel.TeamMember]{Items: items, Pagination: paging.NewPagination(request.Page, request.PageSize, total)}, nil
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

func (uc *TeamsUseCase) GetTeamContests(ctx context.Context, teamID, requestUserID uuid.UUID, request paging.Request) (*paging.List[contestsmodel.Contest], error) {
	if _, err := uc.GetTeam(ctx, teamID, requestUserID); err != nil {
		return nil, err
	}
	request, err := request.Normalize()
	if err != nil {
		return nil, err
	}
	items, total, err := uc.repo.GetTeamContests(ctx, teamID, requestUserID, request)
	if err != nil {
		return nil, err
	}
	return &paging.List[contestsmodel.Contest]{Items: items, Pagination: paging.NewPagination(request.Page, request.PageSize, total)}, nil
}

func (uc *TeamsUseCase) GetTeamProblems(ctx context.Context, teamID, requestUserID uuid.UUID, request paging.Request) (*paging.List[problemsmodel.Problem], error) {
	if _, err := uc.GetTeam(ctx, teamID, requestUserID); err != nil {
		return nil, err
	}
	request, err := request.Normalize()
	if err != nil {
		return nil, err
	}
	items, total, err := uc.repo.GetTeamProblems(ctx, teamID, requestUserID, request)
	if err != nil {
		return nil, err
	}
	return &paging.List[problemsmodel.Problem]{Items: items, Pagination: paging.NewPagination(request.Page, request.PageSize, total)}, nil
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
