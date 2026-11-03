package problems

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

type problemTestTransactions struct {
	ProblemTxStore
	repo      *problemImportMockProblemsRepo
	packages  packageCreation
	workspace WorkspaceRepo
}

func newProblemTestTransactions(repo *problemImportMockProblemsRepo, packages packageCreation) *problemTestTransactions {
	return &problemTestTransactions{repo: repo, packages: packages, workspace: repo}
}
func (s *problemTestTransactions) Within(_ context.Context, fn func(ProblemTxStore) error) error {
	return fn(s)
}
func (s *problemTestTransactions) GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error) {
	return s.repo.GetProblemById(ctx, id)
}
func (*problemTestTransactions) LockOrganization(context.Context, uuid.UUID) error { return nil }
func (*problemTestTransactions) LockProblem(context.Context, uuid.UUID) error      { return nil }
func (*problemTestTransactions) HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error) {
	return true, nil
}
func (s *problemTestTransactions) CommitWorkspace(ctx context.Context, id uuid.UUID, previous, hash string, metadata *problemsmodel.WorkspaceMetadata) error {
	return s.workspace.CommitWorkspace(ctx, id, previous, hash, metadata)
}
func (s *problemTestTransactions) CreatePackage(ctx context.Context, params *problemsmodel.CreatePackageParams) (problemsmodel.ProblemPackage, error) {
	return s.packages.CreatePackage(ctx, params)
}
func problemActorContext() context.Context {
	return accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
}

type packageCreation interface {
	CreatePackage(context.Context, *problemsmodel.CreatePackageParams) (problemsmodel.ProblemPackage, error)
}
