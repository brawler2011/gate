package problems

import (
	"context"

	"github.com/gate149/core/internal/models"
	problemssqlc "github.com/gate149/core/internal/problems/sqlc"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *problemssqlc.Queries
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		pool:    db,
		queries: problemssqlc.New(db),
	}
}

func (r *Repository) CreateProblem(ctx context.Context, params *models.CreateProblemParams) error {
	_, err := r.queries.CreateProblem(ctx, problemssqlc.CreateProblemParams{
		ID:        params.Id,
		Title:     params.Title,
		CreatedBy: params.UserId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetProblemById(ctx context.Context, id uuid.UUID) (problemssqlc.Problem, error) {
	problem, err := r.queries.GetProblemById(ctx, id)
	if err != nil {
		return problemssqlc.Problem{}, pkg.HandlePgErr(err)
	}
	return problem, nil
}

func (r *Repository) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteProblem(ctx, id)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) ListProblems(ctx context.Context, filter *models.ProblemsFilter) ([]problemssqlc.ListProblemsRow, int64, error) {
	sortOrder := int32(1)
	if filter.Descending {
		sortOrder = -1
	}

	total, err := r.queries.CountProblems(ctx, problemssqlc.CountProblemsParams{
		UserID: nullableUUIDToPgtype(filter.OwnerId),
		Search: filter.Search,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	rows, err := r.queries.ListProblems(ctx, problemssqlc.ListProblemsParams{
		Limit:     int32(filter.PageSize),
		Offset:    int32(filter.Offset()),
		UserID:    nullableUUIDToPgtype(filter.OwnerId),
		Search:    filter.Search,
		SortOrder: &sortOrder,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return rows, total, nil
}

func (r *Repository) UpdateProblem(ctx context.Context, id uuid.UUID, problem *models.ProblemUpdate) error {
	var timeLimit *int32
	if problem.TimeLimit != nil {
		tl := int32(*problem.TimeLimit)
		timeLimit = &tl
	}

	var memoryLimit *int32
	if problem.MemoryLimit != nil {
		ml := int32(*problem.MemoryLimit)
		memoryLimit = &ml
	}

	err := r.queries.UpdateProblem(ctx, problemssqlc.UpdateProblemParams{
		ID:               id,
		Title:            problem.Title,
		TimeLimit:        timeLimit,
		MemoryLimit:      memoryLimit,
		Visibility:       stringToNullProblemVisibility(problem.Visibility),
		Legend:           problem.Legend,
		InputFormat:      problem.InputFormat,
		OutputFormat:     problem.OutputFormat,
		Notes:            problem.Notes,
		Scoring:          problem.Scoring,
		LegendHtml:       problem.LegendHtml,
		InputFormatHtml:  problem.InputFormatHtml,
		OutputFormatHtml: problem.OutputFormatHtml,
		NotesHtml:        problem.NotesHtml,
		ScoringHtml:      problem.ScoringHtml,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) CreateProblemMember(ctx context.Context, params *models.CreateProblemMemberParams) error {
	err := r.queries.CreateProblemMember(ctx, problemssqlc.CreateProblemMemberParams{
		ProblemID: params.ProblemId,
		UserID:    params.UserId,
		Role:      problemssqlc.ProblemRole(params.Role),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemssqlc.ProblemMember, error) {
	row, err := r.queries.GetProblemMember(ctx, problemssqlc.GetProblemMemberParams{
		ProblemID: problemId,
		UserID:    userId,
	})
	if err != nil {
		return problemssqlc.ProblemMember{}, pkg.HandlePgErr(err)
	}
	return row, nil
}

func (r *Repository) DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error {
	err := r.queries.DeleteProblemTests(ctx, problemId)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) CreateProblemTests(ctx context.Context, tests models.ProblemTests) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)
	for _, test := range tests {
		err := qtx.CreateProblemTest(ctx, problemssqlc.CreateProblemTestParams{
			ProblemID: test.ProblemId,
			Ordinal:   int32(test.Ordinal),
			Input:     test.Input,
			Output:    test.Output,
		})
		if err != nil {
			return pkg.HandlePgErr(err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]problemssqlc.ProblemTest, error) {
	rows, err := r.queries.GetProblemTests(ctx, problemId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}
	return rows, nil
}

// Helper functions

func stringToNullProblemVisibility(s *string) problemssqlc.NullProblemVisibility {
	if s == nil {
		return problemssqlc.NullProblemVisibility{Valid: false}
	}
	return problemssqlc.NullProblemVisibility{
		ProblemVisibility: problemssqlc.ProblemVisibility(*s),
		Valid:             true,
	}
}

func nullableUUIDToPgtype(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}
