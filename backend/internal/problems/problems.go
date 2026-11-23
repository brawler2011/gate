package problems

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type ProblemsUseCase struct {
	access        access.Loader
	repo          ProblemsRepo
	organizations OrganizationsRepo
	templates     TemplatePackages
	transactions  ProblemTransactions
}

func NewProblemsUseCase(repo ProblemsRepo, transactions ProblemTransactions, organizations OrganizationsRepo, loader access.Loader, templates TemplatePackages) *ProblemsUseCase {
	return &ProblemsUseCase{repo: repo, transactions: transactions, organizations: organizations, access: loader, templates: templates}
}

func (uc *ProblemsUseCase) UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error {
	return access.WithinProblem(ctx, uc.transactions, id, accessmodel.ProblemEdit, func(tx ProblemTxStore, _ access.ProblemAccess) error {
		if problem.IsTemplate != nil && *problem.IsTemplate {
			if _, err := tx.GetReadyPackage(ctx, id); err != nil {
				return pkg.OnNotFound(err, pkg.ErrPackageNotReady, "template requires a successful package")
			}
		}
		return tx.UpdateProblem(ctx, id, problem)
	})
}

func (uc *ProblemsUseCase) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	return access.WithinProblem(ctx, uc.transactions, id, accessmodel.ProblemDelete, func(tx ProblemTxStore, _ access.ProblemAccess) error {
		return tx.DeleteProblem(ctx, id)
	})
}

func (uc *ProblemsUseCase) CreateProblemMember(ctx context.Context, problemID, userID uuid.UUID, role accessmodel.ProblemRole) error {
	return access.WithinProblem(ctx, uc.transactions, problemID, accessmodel.ProblemManage, func(tx ProblemTxStore, a access.ProblemAccess) error {
		if err := a.ChangeMember(userID, "", role); err != nil {
			return err
		}
		if _, err := tx.GetMember(ctx, a.Facts.OrganizationID, userID); err != nil {
			return pkg.OnNotFound(err, pkg.ErrNotOrgMember, "user must be a member of the organization")
		}
		return tx.CreateProblemMember(ctx, &problemsmodel.CreateProblemMemberParams{ProblemID: problemID, UserID: userID, Role: role})
	})
}

func (uc *ProblemsUseCase) UpdateProblemMemberRole(ctx context.Context, problemID, userID uuid.UUID, role accessmodel.ProblemRole) error {
	return uc.changeMember(ctx, problemID, userID, role, func(tx ProblemTxStore) error { return tx.UpdateProblemMemberRole(ctx, problemID, userID, role) })
}

func (uc *ProblemsUseCase) RemoveProblemMember(ctx context.Context, problemID, userID uuid.UUID) error {
	return uc.changeMember(ctx, problemID, userID, "", func(tx ProblemTxStore) error { return tx.RemoveProblemMember(ctx, problemID, userID) })
}

// Members may lower or drop their own role without managing the problem.
func (uc *ProblemsUseCase) changeMember(ctx context.Context, problemID, userID uuid.UUID, role accessmodel.ProblemRole, write func(ProblemTxStore) error) error {
	return access.WithinProblem(ctx, uc.transactions, problemID, accessmodel.ProblemView, func(tx ProblemTxStore, a access.ProblemAccess) error {
		member, err := tx.GetProblemMember(ctx, problemID, userID)
		if err != nil {
			return err
		}
		if err := a.ChangeMember(userID, member.Role, role); err != nil {
			return err
		}
		return write(tx)
	})
}
