package problems

import (
	"context"

	"github.com/gate149/core/pkg"
	"github.com/google/uuid"

	"github.com/gate149/core/internal/models"
	"github.com/jmoiron/sqlx"

	_ "embed"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		db: db,
	}
}

//go:embed sql/create_problem.sql
var CreateProblemQuery string

func (r *Repository) CreateProblem(ctx context.Context, params *models.CreateProblemParams) error {
	_, err := r.db.QueryxContext(ctx, CreateProblemQuery,
		params.Id,
		params.Title,
		params.UserId,
	)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

//go:embed sql/get_problem_by_id.sql
var GetProblemByIdQuery string

func (r *Repository) GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error) {
	var problem models.Problem
	err := r.db.GetContext(ctx, &problem, GetProblemByIdQuery, id)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return &problem, nil
}

//go:embed sql/delete_problem.sql
var DeleteProblemQuery string

func (r *Repository) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, DeleteProblemQuery, id)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

//go:embed sql/list_problems.sql
var ListProblemsQuery string

//go:embed sql/count_problems.sql
var CountProblemsQuery string

func (r *Repository) ListProblems(ctx context.Context, filter *models.ProblemsFilter) (*models.ProblemsList, error) {
	var total int64
	err := r.db.GetContext(ctx, &total, CountProblemsQuery, filter.OwnerId, filter.Search)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	list := make([]*models.ProblemsListItem, 0)

	sortOrder := 1
	if filter.Descending {
		sortOrder = -1
	}

	err = r.db.SelectContext(ctx, &list, ListProblemsQuery,
		filter.OwnerId,
		filter.Search,
		sortOrder,
		filter.PageSize,
		filter.Offset(),
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return &models.ProblemsList{
		Problems: list,
		Pagination: models.Pagination{
			Total: models.Total(total, filter.PageSize),
			Page:  filter.Page,
		},
	}, nil
}

//go:embed sql/update_problem.sql
var UpdateProblemQuery string

func (r *Repository) UpdateProblem(ctx context.Context, id uuid.UUID, problem *models.ProblemUpdate) error {
	_, err := r.db.ExecContext(ctx, UpdateProblemQuery,
		id,
		problem.Title,
		problem.TimeLimit,
		problem.MemoryLimit,
		problem.Visibility,
		problem.Legend,
		problem.InputFormat,
		problem.OutputFormat,
		problem.Notes,
		problem.Scoring,
		problem.LegendHtml,
		problem.InputFormatHtml,
		problem.OutputFormatHtml,
		problem.NotesHtml,
		problem.ScoringHtml,
	)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

//go:embed sql/create_problem_member.sql
var CreateProblemMemberQuery string

func (r *Repository) CreateProblemMember(ctx context.Context, params *models.CreateProblemMemberParams) error {
	_, err := r.db.ExecContext(ctx, CreateProblemMemberQuery,
		params.ProblemId,
		params.UserId,
		params.Role,
	)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

//go:embed sql/get_problem_member.sql
var GetProblemMemberQuery string

func (r *Repository) GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (*models.ProblemMember, error) {
	var member models.ProblemMember
	err := r.db.GetContext(ctx, &member, GetProblemMemberQuery, problemId, userId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}
	return &member, nil
}

//go:embed sql/delete_problem_tests.sql
var DeleteProblemTestsQuery string

func (r *Repository) DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, DeleteProblemTestsQuery, problemId)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

//go:embed sql/create_problem_tests.sql
var CreateProblemTestsQuery string

func (r *Repository) CreateProblemTests(ctx context.Context, tests models.ProblemTests) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	defer tx.Rollback()

	for _, test := range tests {
		_, err := tx.ExecContext(ctx, CreateProblemTestsQuery,
			test.ProblemId,
			test.Ordinal,
			test.Input,
			test.Output,
		)
		if err != nil {
			return pkg.HandlePgErr(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

//go:embed sql/get_problem_tests.sql
var GetProblemTestsQuery string

func (r *Repository) GetProblemTests(ctx context.Context, problemId uuid.UUID) (models.ProblemTests, error) {
	tests := make(models.ProblemTests, 0)
	err := r.db.SelectContext(ctx, &tests, GetProblemTestsQuery, problemId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return tests, nil
}