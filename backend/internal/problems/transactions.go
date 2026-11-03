package problems

import (
	"context"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
)

type ProblemTxStore interface {
	CreateProblem(ctx context.Context, params *problemsmodel.CreateProblemParams) error
	CreateProblemMember(ctx context.Context, params *problemsmodel.CreateProblemMemberParams) error
}

type ProblemTransactions interface {
	Within(context.Context, func(ProblemTxStore) error) error
}
