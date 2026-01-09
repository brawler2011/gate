package pg

import (
	"context"

	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProblemsRepo struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewProblemsRepo(db *pgxpool.Pool) *ProblemsRepo {
	return &ProblemsRepo{
		pool:    db,
		queries: sqlc.New(db),
	}
}

func (r *ProblemsRepo) CreateProblem(ctx context.Context, params *models.CreateProblemParams) error {
	_, err := r.queries.CreateProblem(ctx, sqlc.CreateProblemParams{
		ID:        params.Id,
		Title:     params.Title,
		CreatedBy: params.UserId,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) GetProblemById(ctx context.Context, id uuid.UUID) (models.Problem, error) {
	problem, err := r.queries.GetProblemById(ctx, id)
	if err != nil {
		return models.Problem{}, HandlePgErr(err)
	}
	return mapProblem(problem), nil
}

func (r *ProblemsRepo) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteProblem(ctx, id)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) ListProblems(ctx context.Context, filter *models.ProblemsFilter) ([]models.Problem, int32, error) {
	sortOrder := int32(1)
	if filter.Descending {
		sortOrder = -1
	}

	total, err := r.queries.CountProblems(ctx, sqlc.CountProblemsParams{
		UserID: nullableUUIDToPgtype(filter.OwnerId),
		Search: filter.Search,
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	rows, err := r.queries.ListProblems(ctx, sqlc.ListProblemsParams{
		Limit:     int32(filter.PageSize),
		Offset:    int32(filter.Offset()),
		UserID:    nullableUUIDToPgtype(filter.OwnerId),
		Search:    filter.Search,
		SortOrder: &sortOrder,
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	problems := make([]models.Problem, len(rows))
	for i, row := range rows {
		problems[i] = mapListProblemsRow(row)
	}

	return problems, int32(total), nil
}

func (r *ProblemsRepo) UpdateProblem(ctx context.Context, id uuid.UUID, problem *models.ProblemUpdate) error {
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

	err := r.queries.UpdateProblem(ctx, sqlc.UpdateProblemParams{
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
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) CreateProblemMember(ctx context.Context, params *models.CreateProblemMemberParams) error {
	err := r.queries.CreateProblemMember(ctx, sqlc.CreateProblemMemberParams{
		ProblemID: params.ProblemId,
		UserID:    params.UserId,
		Role:      sqlc.ProblemRole(params.Role),
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (models.ProblemMember, error) {
	row, err := r.queries.GetProblemMember(ctx, sqlc.GetProblemMemberParams{
		ProblemID: problemId,
		UserID:    userId,
	})
	if err != nil {
		return models.ProblemMember{}, HandlePgErr(err)
	}
	return mapProblemMember(row), nil
}

func (r *ProblemsRepo) DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error {
	err := r.queries.DeleteProblemTests(ctx, problemId)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

// FIXME: multi insert
func (r *ProblemsRepo) CreateProblemTests(ctx context.Context, tests models.ProblemTests) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return HandlePgErr(err)
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)
	for _, test := range tests {
		err := qtx.CreateProblemTest(ctx, sqlc.CreateProblemTestParams{
			ProblemID: test.ProblemID,
			Ordinal:   int32(test.Ordinal),
			Input:     test.Input,
			Output:    test.Output,
		})
		if err != nil {
			return HandlePgErr(err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]models.ProblemTest, error) {
	rows, err := r.queries.GetProblemTests(ctx, problemId)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	tests := make([]models.ProblemTest, len(rows))
	for i, row := range rows {
		tests[i] = mapProblemTest(row)
	}
	return tests, nil
}

func stringToNullProblemVisibility(s *string) sqlc.NullProblemVisibility {
	if s == nil {
		return sqlc.NullProblemVisibility{Valid: false}
	}
	return sqlc.NullProblemVisibility{
		ProblemVisibility: sqlc.ProblemVisibility(*s),
		Valid:             true,
	}
}

func mapProblem(p sqlc.Problem) models.Problem {
	return models.Problem{
		ID:               p.ID,
		CreatedBy:        pgUUIDToUUID(p.CreatedBy),
		Visibility:       string(p.Visibility),
		Title:            p.Title,
		TimeLimit:        p.TimeLimit,
		MemoryLimit:      p.MemoryLimit,
		Legend:           p.Legend,
		InputFormat:      p.InputFormat,
		OutputFormat:     p.OutputFormat,
		Notes:            p.Notes,
		Scoring:          p.Scoring,
		LegendHtml:       p.LegendHtml,
		InputFormatHtml:  p.InputFormatHtml,
		OutputFormatHtml: p.OutputFormatHtml,
		NotesHtml:        p.NotesHtml,
		ScoringHtml:      p.ScoringHtml,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

func mapListProblemsRow(row sqlc.ListProblemsRow) models.Problem {
	return models.Problem{
		ID:          row.ID,
		Title:       row.Title,
		TimeLimit:   row.TimeLimit,
		MemoryLimit: row.MemoryLimit,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func mapProblemMember(pm sqlc.ProblemMember) models.ProblemMember {
	return models.ProblemMember{
		ProblemId: pgUUIDToUUID(pm.ProblemID),
		UserId:    pm.UserID,
		Role:      string(pm.Role),
	}
}

func mapProblemTest(row sqlc.ProblemTest) models.ProblemTest {
	return models.ProblemTest{
		ID:        row.ID,
		ProblemID: row.ProblemID,
		Ordinal:   row.Ordinal,
		Input:     row.Input,
		Output:    row.Output,
		CreatedAt: row.CreatedAt,
	}
}
