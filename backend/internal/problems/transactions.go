package problems

import (
	"context"

	"github.com/google/uuid"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
)

type ProblemTxStore interface {
	CommitWorkspace(context.Context, uuid.UUID, string, string, *problemsmodel.WorkspaceMetadata) error
	CreateProblem(ctx context.Context, params *problemsmodel.CreateProblemParams) error
	CreateProblemMember(ctx context.Context, params *problemsmodel.CreateProblemMemberParams) error
}

type ProblemTransactions interface {
	Within(context.Context, func(ProblemTxStore) error) error
}
