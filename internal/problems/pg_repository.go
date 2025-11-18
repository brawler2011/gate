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

func (r *Repository) CreateProblem(ctx context.Context, title string) (uuid.UUID, error) {
	const op = "Repository.CreateProblem"

	rows, err := r.db.QueryxContext(ctx, CreateProblemQuery, title)
	if err != nil {
		return uuid.Nil, pkg.HandlePgErr(err, op)
	}

	defer rows.Close()
	var id uuid.UUID
	rows.Next()
	err = rows.Scan(&id)
	if err != nil {
		return uuid.Nil, pkg.HandlePgErr(err, op)
	}

	return id, nil
}

//go:embed sql/get_problem_by_id.sql
var GetProblemByIdQuery string

func (r *Repository) GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error) {
	const op = "Repository.ReadProblemById"

	var problem models.Problem
	err := r.db.GetContext(ctx, &problem, GetProblemByIdQuery, id)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &problem, nil
}

//go:embed sql/delete_problem.sql
var DeleteProblemQuery string

func (r *Repository) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	const op = "Repository.DeleteProblem"

	_, err := r.db.ExecContext(ctx, DeleteProblemQuery, id)
	if err != nil {
		return pkg.HandlePgErr(err, op)
	}

	return nil
}

//go:embed sql/list_problems.sql
var ListProblemsQuery string

//go:embed sql/count_problems.sql
var CountProblemsQuery string

func (r *Repository) ListProblems(ctx context.Context, filter *models.ProblemsFilter) (*models.ProblemsList, error) {
	const op = "ContestRepository.ListProblems"

	var total int64
	err := r.db.GetContext(ctx, &total, CountProblemsQuery, filter.OwnerId, filter.Search)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	list := make([]*models.ProblemsListItem, 0)
	err = r.db.SelectContext(ctx, &list, ListProblemsQuery,
		filter.OwnerId,
		filter.Search,
		filter.Descending,
		filter.PageSize,
		filter.Offset(),
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
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
	const op = "Repository.UpdateProblem"

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
		return pkg.HandlePgErr(err, op)
	}

	return nil
}

//go:embed sql/get_problem_member.sql
var GetProblemMemberQuery string

func (r *Repository) GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (*models.ProblemMember, error) {
	const op = "Repository.GetProblemMember"

	var member models.ProblemMember
	err := r.db.GetContext(ctx, &member, GetProblemMemberQuery, problemId, userId)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &member, nil
}
