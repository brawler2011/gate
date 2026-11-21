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
	CanTeam(context.Context, uuid.UUID, accessmodel.TeamAction) (bool, error)
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

func (uc *TeamsUseCase) CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput) (*organizationsmodel.Team, error) {
	creatorID := accessmodel.UserFromContext(ctx).Id
	var team *organizationsmodel.Team
	err := uc.transactor.Within(ctx, func(tx TeamTxStore) error {
		if _, err := tx.GetOrganizationByID(ctx, input.OrganizationID); err != nil {
			return err
		}
		if err := access.Organization(ctx, tx, input.OrganizationID, accessmodel.ActionManageOrganization); err != nil {
			return err
		}

		var err error
		team, err = tx.CreateTeam(ctx, input)
		if err != nil {
			return fmt.Errorf("create team: %w", err)
		}

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

func (uc *TeamsUseCase) GetTeam(ctx context.Context, teamID uuid.UUID) (*organizationsmodel.Team, error) {
	team, err := uc.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if err := access.Team(ctx, uc.visibility, teamID, accessmodel.ActionViewTeam); err != nil {
		return nil, err
	}

	return team, nil
}

func (uc *TeamsUseCase) ListTeams(ctx context.Context, filter *organizationsmodel.TeamsFilter) (*organizationsmodel.TeamsList, error) {
	user := accessmodel.UserFromContext(ctx)
	if filter == nil {
		return nil, pkg.ErrInvalid
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

func (uc *TeamsUseCase) UpdateTeam(ctx context.Context, teamID uuid.UUID, input *organizationsmodel.UpdateTeamInput) error {
	return uc.withinTeam(ctx, teamID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		if err := requireTeamManager(ctx, tx, team); err != nil {
			return err
		}
		return tx.UpdateTeam(ctx, teamID, input)
	})
}

func (uc *TeamsUseCase) DeleteTeam(ctx context.Context, teamID uuid.UUID) error {
	return uc.withinTeam(ctx, teamID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		if err := access.Organization(ctx, tx, team.OrganizationID, accessmodel.ActionManageOrganization); err != nil {
			return err
		}
		return tx.DeleteTeam(ctx, teamID)
	})
}

func (uc *TeamsUseCase) AddTeamMember(ctx context.Context, input *organizationsmodel.AddTeamMemberInput) error {
	return uc.withinTeam(ctx, input.TeamID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		if err := requireTeamManager(ctx, tx, team); err != nil {
			return err
		}
		if _, err := tx.GetMember(ctx, team.OrganizationID, input.UserID); err != nil {
			return err
		}
		return tx.AddTeamMember(ctx, input.TeamID, input.UserID, input.Role)
	})
}

func (uc *TeamsUseCase) ListTeamMembers(ctx context.Context, teamID uuid.UUID, request paging.Request) (*paging.List[organizationsmodel.TeamMember], error) {
	if _, err := uc.GetTeam(ctx, teamID); err != nil {
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

func (uc *TeamsUseCase) UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole) error {
	return uc.withinTeam(ctx, teamID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		if err := requireTeamManager(ctx, tx, team); err != nil {
			return err
		}
		return tx.UpdateTeamMemberRole(ctx, teamID, userID, role)
	})
}

func (uc *TeamsUseCase) RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error {
	return uc.withinTeam(ctx, teamID, func(tx TeamTxStore, team *organizationsmodel.Team) error {
		if userID != accessmodel.UserFromContext(ctx).Id {
			if err := requireTeamManager(ctx, tx, team); err != nil {
				return err
			}
		}
		return tx.RemoveTeamMember(ctx, teamID, userID)
	})
}

func (uc *TeamsUseCase) GetTeamContests(ctx context.Context, teamID uuid.UUID, request paging.Request) (*paging.List[contestsmodel.Contest], error) {
	if _, err := uc.GetTeam(ctx, teamID); err != nil {
		return nil, err
	}
	request, err := request.Normalize()
	if err != nil {
		return nil, err
	}
	items, total, err := uc.repo.GetTeamContests(ctx, teamID, accessmodel.UserFromContext(ctx).Id, request)
	if err != nil {
		return nil, err
	}
	return &paging.List[contestsmodel.Contest]{Items: items, Pagination: paging.NewPagination(request.Page, request.PageSize, total)}, nil
}

func (uc *TeamsUseCase) GetTeamProblems(ctx context.Context, teamID uuid.UUID, request paging.Request) (*paging.List[problemsmodel.Problem], error) {
	if _, err := uc.GetTeam(ctx, teamID); err != nil {
		return nil, err
	}
	request, err := request.Normalize()
	if err != nil {
		return nil, err
	}
	items, total, err := uc.repo.GetTeamProblems(ctx, teamID, accessmodel.UserFromContext(ctx).Id, request)
	if err != nil {
		return nil, err
	}
	return &paging.List[problemsmodel.Problem]{Items: items, Pagination: paging.NewPagination(request.Page, request.PageSize, total)}, nil
}

func (uc *TeamsUseCase) withinTeam(ctx context.Context, id uuid.UUID, fn func(TeamTxStore, *organizationsmodel.Team) error) error {
	return uc.transactor.Within(ctx, func(tx TeamTxStore) error {
		current, err := tx.GetTeamByID(ctx, id)
		if err != nil {
			return err
		}
		return fn(tx, current)
	})
}

// Organization managers and team maintainers manage a team.
func requireTeamManager(ctx context.Context, tx TeamTxStore, team *organizationsmodel.Team) error {
	allowed, err := tx.CanOrganization(ctx, team.OrganizationID, accessmodel.ActionManageOrganization)
	if err != nil || allowed {
		return err
	}
	member, err := tx.GetTeamMember(ctx, team.ID, accessmodel.UserFromContext(ctx).Id)
	if err != nil && !errors.Is(err, pkg.ErrNotFound) {
		return err
	}
	if err == nil && member.Role == organizationsmodel.TeamRoleMaintainer {
		return nil
	}
	return pkg.ErrForbidden
}
