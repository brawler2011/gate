package problems

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

type WorkspaceCatalog struct {
	lookup       WorkspaceRepo
	transactions ProblemTransactions
}

func NewWorkspaceCatalog(lookup WorkspaceRepo, transactions ProblemTransactions) *WorkspaceCatalog {
	return &WorkspaceCatalog{lookup: lookup, transactions: transactions}
}
func (c *WorkspaceCatalog) GetWorkspaceHash(ctx context.Context, id uuid.UUID) (string, error) {
	return c.lookup.GetWorkspaceHash(ctx, id)
}
func (c *WorkspaceCatalog) CommitWorkspace(ctx context.Context, id uuid.UUID, previous, hash string, metadata *problemsmodel.WorkspaceMetadata) error {
	return withinProblem(ctx, c.transactions, id, accessmodel.ActionEditProblem, func(tx ProblemTxStore) error { return tx.CommitWorkspace(ctx, id, previous, hash, metadata) })
}
