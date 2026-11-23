package organizations

import (
	"context"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type TeamsUseCase struct {
	access     access.Loader
	repo       TeamsRepo
	transactor TeamTransactions
}

func NewTeamsUseCase(repo TeamsRepo, loader access.Loader, transactor TeamTransactions) *TeamsUseCase {
	return &TeamsUseCase{repo: repo, access: loader, transactor: transactor}
}

func (uc *TeamsUseCase) CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput) (*organizationsmodel.Team, error) {
	var team *organizationsmodel.Team
	err := access.WithinOrganization(ctx, uc.transactor, input.OrganizationID, accessmodel.OrganizationCreate, func(tx TeamTxStore, org access.OrganizationAccess) error {
		var err error
		team, err = tx.CreateTeam(ctx, input)
		if err != nil {
			return fmt.Errorf("create team: %w", err)
		}
		if err := tx.AddTeamMember(ctx, team.ID, org.Actor.Id, accessmodel.TeamMaintainer); err != nil {
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
	if _, err := access.Team(ctx, uc.access, teamID, accessmodel.TeamView); err != nil {
		return nil, err
	}
	return uc.repo.GetTeamByID(ctx, teamID)
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
	return access.WithinTeam(ctx, uc.transactor, teamID, accessmodel.TeamManage, func(tx TeamTxStore, _ access.TeamAccess) error {
		return tx.UpdateTeam(ctx, teamID, input)
	})
}

func (uc *TeamsUseCase) DeleteTeam(ctx context.Context, teamID uuid.UUID) error {
	return access.WithinTeam(ctx, uc.transactor, teamID, accessmodel.TeamDelete, func(tx TeamTxStore, _ access.TeamAccess) error {
		return tx.DeleteTeam(ctx, teamID)
	})
}

func (uc *TeamsUseCase) AddTeamMember(ctx context.Context, input *organizationsmodel.AddTeamMemberInput) error {
	return access.WithinTeam(ctx, uc.transactor, input.TeamID, accessmodel.TeamManage, func(tx TeamTxStore, a access.TeamAccess) error {
		if err := a.ChangeMember(input.UserID, "", input.Role); err != nil {
			return err
		}
		if _, err := tx.GetMember(ctx, a.Facts.OrganizationID, input.UserID); err != nil {
			return pkg.OnNotFound(err, pkg.ErrNotOrgMember, "user must be a member of the organization")
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

func (uc *TeamsUseCase) UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role accessmodel.TeamRole) error {
	return uc.changeMember(ctx, teamID, userID, role, func(tx TeamTxStore) error { return tx.UpdateTeamMemberRole(ctx, teamID, userID, role) })
}

func (uc *TeamsUseCase) RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error {
	return uc.changeMember(ctx, teamID, userID, "", func(tx TeamTxStore) error { return tx.RemoveTeamMember(ctx, teamID, userID) })
}

// Members may lower or drop their own role without managing the team.
func (uc *TeamsUseCase) changeMember(ctx context.Context, teamID, userID uuid.UUID, role accessmodel.TeamRole, write func(TeamTxStore) error) error {
	return access.WithinTeam(ctx, uc.transactor, teamID, accessmodel.TeamView, func(tx TeamTxStore, a access.TeamAccess) error {
		member, err := tx.GetTeamMember(ctx, teamID, userID)
		if err != nil {
			return err
		}
		if err := a.ChangeMember(userID, member.Role, role); err != nil {
			return err
		}
		return write(tx)
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
