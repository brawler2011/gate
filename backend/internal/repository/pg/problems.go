package pg

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg/sqlc"
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
		ID:             params.ID,
		OrganizationID: params.OrganizationID,
		OwnerID:        uuidPtrToNullUUID(params.OwnerID),
		Visibility:     sqlc.ProblemVisibility(params.Visibility),
		Title:          params.Title,
		ShortName:      params.ShortName,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) GetProblemById(ctx context.Context, id uuid.UUID) (models.Problem, error) {
	problem, err := r.queries.GetProblemByID(ctx, id)
	if err != nil {
		return models.Problem{}, HandlePgErr(err)
	}
	return mapGetProblemByIDRow(problem), nil
}

func (r *ProblemsRepo) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteProblem(ctx, id)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) ListProblems(ctx context.Context, filter *models.ProblemsFilter) ([]models.Problem, int32, error) {
	// Calculate offset
	offset := (filter.Page - 1) * filter.PageSize

	// If no organization filter, list all problems
	if filter.OrganizationID == nil {
		total, err := r.queries.CountAllProblems(ctx, sqlc.CountAllProblemsParams{
			Column1: filter.Search,
			Column2: filter.Visibility,
		})
		if err != nil {
			return nil, 0, HandlePgErr(err)
		}

		rows, err := r.queries.ListAllProblems(ctx, sqlc.ListAllProblemsParams{
			Column1: filter.Search,
			Column2: filter.Visibility,
			Limit:   filter.PageSize,
			Offset:  offset,
		})
		if err != nil {
			return nil, 0, HandlePgErr(err)
		}

		problems := make([]models.Problem, len(rows))
		for i, row := range rows {
			problems[i] = mapListAllProblemsRow(row)
		}

		return problems, int32(total), nil
	}

	// Filter by organization
	total, err := r.queries.CountProblems(ctx, sqlc.CountProblemsParams{
		OrganizationID: *filter.OrganizationID,
		Column2:        filter.Search,
		Column3:        filter.Visibility,
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	rows, err := r.queries.ListProblems(ctx, sqlc.ListProblemsParams{
		OrganizationID: *filter.OrganizationID,
		Column2:        filter.Search,
		Column3:        filter.Visibility,
		Limit:          filter.PageSize,
		Offset:         offset,
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
	err := r.queries.UpdateProblem(ctx, sqlc.UpdateProblemParams{
		ID:            id,
		Title:         problem.Title,
		Visibility:    stringToNullProblemVisibility(problem.Visibility),
		OwnerID:       uuidPtrToNullUUID(problem.OwnerID),
		GitCommitHash: problem.GitCommitHash,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) CreateProblemMember(ctx context.Context, params *models.CreateProblemMemberParams) error {
	err := r.queries.AddProblemMember(ctx, sqlc.AddProblemMemberParams{
		ProblemID: params.ProblemID,
		UserID:    params.UserID,
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

// Tests are now stored in git repos, not in database
// These methods are kept for interface compatibility but return errors or empty results

func (r *ProblemsRepo) DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error {
	// Tests are now in git repos, not in database
	return nil
}

func (r *ProblemsRepo) CreateProblemTests(ctx context.Context, tests models.ProblemTests) error {
	// Tests are now in git repos, not in database
	return nil
}

func (r *ProblemsRepo) GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]models.ProblemTest, error) {
	// Tests are now in git repos, not in database
	return []models.ProblemTest{}, nil
}

func (r *ProblemsRepo) GetProblemTeams(ctx context.Context, problemId uuid.UUID) ([]models.ProblemTeam, error) {
	// TODO: Implement when problem_teams table is fully integrated
	// For now, return empty slice as teams feature is not fully implemented
	return []models.ProblemTeam{}, nil
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

func mapGetProblemByIDRow(p sqlc.GetProblemByIDRow) models.Problem {
	return models.Problem{
		ID:             p.ID,
		OrganizationID: p.OrganizationID,
		OwnerID:        pgUUIDToUUIDPtr(p.OwnerID),
		Visibility:     string(p.Visibility),
		Title:          p.Title,
		ShortName:      p.ShortName,
		GitCommitHash:  p.GitCommitHash,
		TimeLimitMs:    int(p.TimeLimitMs),
		MemoryLimitMb:  int(p.MemoryLimitMb),
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func mapListAllProblemsRow(p sqlc.ListAllProblemsRow) models.Problem {
	return models.Problem{
		ID:             p.ID,
		OrganizationID: p.OrganizationID,
		OwnerID:        pgUUIDToUUIDPtr(p.OwnerID),
		Visibility:     string(p.Visibility),
		Title:          p.Title,
		ShortName:      p.ShortName,
		GitCommitHash:  p.GitCommitHash,
		TimeLimitMs:    int(p.TimeLimitMs),
		MemoryLimitMb:  int(p.MemoryLimitMb),
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func mapListProblemsRow(p sqlc.ListProblemsRow) models.Problem {
	return models.Problem{
		ID:             p.ID,
		OrganizationID: p.OrganizationID,
		OwnerID:        pgUUIDToUUIDPtr(p.OwnerID),
		Visibility:     string(p.Visibility),
		Title:          p.Title,
		ShortName:      p.ShortName,
		GitCommitHash:  p.GitCommitHash,
		TimeLimitMs:    int(p.TimeLimitMs),
		MemoryLimitMb:  int(p.MemoryLimitMb),
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func (r *ProblemsRepo) UpdateProblemLimits(ctx context.Context, id uuid.UUID, timeLimitMs, memoryLimitMb int) error {
	err := r.queries.UpdateProblemLimits(ctx, sqlc.UpdateProblemLimitsParams{
		ID:            id,
		TimeLimitMs:   int32(timeLimitMs),
		MemoryLimitMb: int32(memoryLimitMb),
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

// mapListProblemsRow is no longer used - removed as ListProblemsRow type doesn't exist
// func mapListProblemsRow(row sqlc.ListProblemsRow) models.Problem {
// 	return models.Problem{
// 		ID:          row.ID,
// 		Title:       row.Title,
// 		TimeLimit:   row.TimeLimit,
// 		MemoryLimit: row.MemoryLimit,
// 		CreatedAt:   row.CreatedAt,
// 		UpdatedAt:   row.UpdatedAt,
// 	}
// }

func mapProblemMember(pm sqlc.ProblemMember) models.ProblemMember {
	return models.ProblemMember{
		ProblemID: pm.ProblemID,
		UserID:    pm.UserID,
		Role:      models.ProblemRole(pm.Role),
		// Username and Email would need to be joined from users table
		// For now, leave empty
		Username:  "",
		Email:     "",
		CreatedAt: pm.CreatedAt,
	}
}

// mapProblemTest is no longer used - removed as ProblemTest type doesn't exist in current schema
// func mapProblemTest(row sqlc.ProblemTest) models.ProblemTest {
// 	return models.ProblemTest{
// 		ID:        row.ID,
// 		ProblemID: row.ProblemID,
// 		Ordinal:   row.Ordinal,
// 		Input:     row.Input,
// 		Output:    row.Output,
// 		CreatedAt: row.CreatedAt,
// 	}
// }
