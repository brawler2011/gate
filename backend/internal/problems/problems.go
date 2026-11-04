package problems

import (
	"context"
	"errors"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type ProblemsUseCase struct {
	repo         ProblemsRepo
	orgsRepo     OrganizationsRepo
	workspace    *WorkspaceStorage
	transactions ProblemTransactions
	templates    PackageTemplates
}

func NewProblemsUseCase(repo ProblemsRepo, workspace *WorkspaceStorage, transactions ProblemTransactions, templates PackageTemplates, orgRepo OrganizationsRepo) *ProblemsUseCase {
	return &ProblemsUseCase{
		repo:         repo,
		workspace:    workspace,
		transactions: transactions,
		templates:    templates,
		orgsRepo:     orgRepo,
	}
}

func (uc *ProblemsUseCase) UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error {
	if err := withinProblem(ctx, uc.transactions, id, accessmodel.ActionEditProblem, func(ProblemTxStore) error { return nil }); err != nil {
		return err
	}
	var previous string
	var version *workspaceVersion
	if problem.Title != nil {
		var err error
		previous, version, err = uc.workspace.prepareChange(ctx, id, func(ctx context.Context) error {
			workshop := newWorkshop(uc.workspace)
			config, err := workshop.readProblemConfig(ctx, id)
			if errors.Is(err, storage.ErrNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			config.Title = *problem.Title
			return workshop.writeProblemConfig(ctx, id, config)
		})
		if err != nil {
			return err
		}
	}
	return withinProblem(ctx, uc.transactions, id, accessmodel.ActionEditProblem, func(tx ProblemTxStore) error {
		if problem.IsTemplate != nil && *problem.IsTemplate {
			if _, err := tx.GetReadyPackage(ctx, id); err != nil {
				if errors.Is(err, pkg.ErrNotFound) {
					return pkg.Wrap(pkg.ErrBadInput, err, "для перевода задачи в шаблон необходим хотя бы один успешно собранный пакет")
				}
				return err
			}
		}

		if version != nil && version.hash != previous {
			if err := tx.CommitWorkspace(ctx, id, previous, version.hash, version.metadata); err != nil {
				return err
			}
		}
		return tx.UpdateProblem(ctx, id, problem)
	})
}

func (uc *ProblemsUseCase) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	return withinProblem(ctx, uc.transactions, id, accessmodel.ActionAdminProblem, func(tx ProblemTxStore) error { return tx.DeleteProblem(ctx, id) })
}

func (uc *ProblemsUseCase) AddProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission problemsmodel.ProblemPermission) error {
	if accessmodel.UserFromContext(ctx).Id != requestUserID {
		return pkg.NoPermission
	}
	return withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionAdminProblem, func(tx ProblemTxStore) error {
		return tx.AddProblemTeam(ctx, problemID, teamID, permission)
	})
}

func (uc *ProblemsUseCase) UpdateProblemTeamPermission(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission problemsmodel.ProblemPermission) error {
	if accessmodel.UserFromContext(ctx).Id != requestUserID {
		return pkg.NoPermission
	}
	return withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionAdminProblem, func(tx ProblemTxStore) error {
		return tx.UpdateProblemTeamPermission(ctx, problemID, teamID, permission)
	})
}

func (uc *ProblemsUseCase) RemoveProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID) error {
	if accessmodel.UserFromContext(ctx).Id != requestUserID {
		return pkg.NoPermission
	}
	return withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionAdminProblem, func(tx ProblemTxStore) error {
		return tx.RemoveProblemTeam(ctx, problemID, teamID)
	})
}

func (uc *ProblemsUseCase) CreateProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role problemsmodel.ProblemRole) error {
	if accessmodel.UserFromContext(ctx).Id != requestUserID {
		return pkg.NoPermission
	}
	return withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionAdminProblem, func(tx ProblemTxStore) error {
		problem, err := tx.GetProblemById(ctx, problemID)
		if err != nil {
			return err
		}
		if _, err := tx.GetMember(ctx, problem.OrganizationID, userID); err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "user must be a member of the organization")
		}
		return tx.CreateProblemMember(ctx, &problemsmodel.CreateProblemMemberParams{ProblemID: problemID, UserID: userID, Role: role})
	})
}

func (uc *ProblemsUseCase) UpdateProblemMemberRole(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role problemsmodel.ProblemRole) error {
	if accessmodel.UserFromContext(ctx).Id != requestUserID {
		return pkg.NoPermission
	}
	return withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionAdminProblem, func(tx ProblemTxStore) error {
		return tx.UpdateProblemMemberRole(ctx, problemID, userID, role)
	})
}

func (uc *ProblemsUseCase) RemoveProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID) error {
	if accessmodel.UserFromContext(ctx).Id != requestUserID {
		return pkg.NoPermission
	}
	return withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionAdminProblem, func(tx ProblemTxStore) error {
		return tx.RemoveProblemMember(ctx, problemID, userID)
	})
}
