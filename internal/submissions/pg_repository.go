package submissions

import (
	"context"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	_ "embed"
)

type PgRepository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *PgRepository {
	return &PgRepository{
		db: db,
	}
}

//go:embed sql/get_solution.sql
var GetSolutionQuery string

func (r *PgRepository) GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error) {
	const op = "Repository.GetSubmissions"

	var solution models.Submission
	err := r.db.GetContext(ctx, &solution, GetSolutionQuery, id)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &solution, nil
}

//go:embed sql/create_solution.sql
var CreateSolutionQuery string

func (r *PgRepository) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	const op = "Repository.CreateSubmission"

	rows, err := r.db.QueryxContext(ctx,
		CreateSolutionQuery,
		creation.ContestId,
		creation.ProblemId,
		creation.UserId,
		creation.Solution,
		creation.Language,
		creation.Penalty,
	)
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

//go:embed sql/update_solution.sql
var UpdateSolutionQuery string

func (r *PgRepository) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	const op = "Repository.UpdateSubmission"

	_, err := r.db.ExecContext(ctx, UpdateSolutionQuery, update.State, update.Score, update.TimeStat, update.MemoryStat, id)
	if err != nil {
		return pkg.HandlePgErr(err, op)
	}

	return nil
}

//go:embed sql/list_solutions.sql
var ListSolutionsQuery string

//go:embed sql/count_solutions.sql
var CountSolutionsQuery string

func (r *PgRepository) ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error) {
	const op = "ContestRepository.ListSolutions"

	var order int = 0
	if filter.Order != nil {
		order = int(*filter.Order)
	}

	// Get count
	var totalCount int64
	err := r.db.GetContext(ctx, &totalCount, CountSolutionsQuery,
		filter.ContestId,
		filter.UserId,
		filter.ProblemId,
		filter.Language,
		filter.State,
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	// Get submissions list
	solutions := make([]*models.SubmissionListItem, 0)
	err = r.db.SelectContext(ctx, &solutions, ListSolutionsQuery,
		filter.ContestId,
		filter.UserId,
		filter.ProblemId,
		filter.Language,
		filter.State,
		order,
		filter.PageSize,
		filter.Offset(),
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &models.SolutionsList{
		Solutions: solutions,
		Pagination: models.Pagination{
			Total: models.Total(totalCount, filter.PageSize),
			Page:  filter.Page,
		},
	}, nil
}
