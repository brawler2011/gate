package problems

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type ProblemsUseCase struct {
	permissions  ProblemPermissions
	repo         ProblemsRepo
	orgsRepo     OrganizationsRepo
	transactions ProblemTransactions
	drafts       *DraftUseCase
	packages     PackagesRepo
	content      *PackageContent
}

func NewProblemsUseCase(repo ProblemsRepo, transactions ProblemTransactions, orgs OrganizationsRepo, permissions ProblemPermissions, drafts *DraftUseCase, packages PackagesRepo, content *PackageContent) *ProblemsUseCase {
	return &ProblemsUseCase{repo: repo, transactions: transactions, orgsRepo: orgs, permissions: permissions, drafts: drafts, packages: packages, content: content}
}
func (uc *ProblemsUseCase) UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error {
	return withinProblem(ctx, uc.transactions, id, accessmodel.ActionEditProblem, func(tx ProblemTxStore) error {
		if problem.IsTemplate != nil && *problem.IsTemplate {
			if _, err := tx.GetReadyPackage(ctx, id); err != nil {
				return pkg.Wrap(pkg.ErrBadInput, err, "template requires a successful package")
			}
		}
		return tx.UpdateProblem(ctx, id, problem)
	})
}
func (uc *ProblemsUseCase) CreateProblem(ctx context.Context, input *problemsmodel.CreateProblemInput) (uuid.UUID, error) {
	return uc.drafts.CreateProblem(ctx, input)
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
