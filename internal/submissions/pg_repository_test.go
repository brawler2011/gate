package submissions_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/submissions"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// setupTestDB creates a mocked sqlx.DB and sqlmock instance for testing.
func setupTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	return sqlxDB, mock
}

func TestRepository_CreateSolution(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := submissions.NewRepository(db)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()

		solutionID := uuid.New()
		contestID := uuid.New()
		problemID := uuid.New()
		userID := uuid.New()

		creation := &models.SubmissionCreation{
			ContestId: contestID,
			ProblemId: problemID,
			UserId:    userID,
			Solution:  "#include <iostream>\nint main() { return 0; }",
			Language:  models.Cpp,
			Penalty:   20,
		}

		mock.ExpectQuery(submissions.CreateSolutionQuery).
			WithArgs(
				creation.ContestId,
				creation.ProblemId,
				creation.UserId,
				creation.Solution,
				creation.Language,
				creation.Penalty,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(solutionID))

		id, err := repo.CreateSubmission(ctx, creation)
		assert.NoError(t, err)
		assert.Equal(t, solutionID, id)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query error", func(t *testing.T) {
		ctx := context.Background()

		creation := &models.SubmissionCreation{
			ContestId: uuid.New(),
			ProblemId: uuid.New(),
			UserId:    uuid.New(),
			Solution:  "test",
			Language:  models.Python,
			Penalty:   0,
		}

		mock.ExpectQuery(submissions.CreateSolutionQuery).
			WithArgs(
				creation.ContestId,
				creation.ProblemId,
				creation.UserId,
				creation.Solution,
				creation.Language,
				creation.Penalty,
			).
			WillReturnError(sql.ErrConnDone)

		id, err := repo.CreateSubmission(ctx, creation)
		assert.Error(t, err)
		assert.Equal(t, uuid.Nil, id)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("scan error", func(t *testing.T) {
		ctx := context.Background()

		creation := &models.SubmissionCreation{
			ContestId: uuid.New(),
			ProblemId: uuid.New(),
			UserId:    uuid.New(),
			Solution:  "test",
			Language:  models.Golang,
			Penalty:   0,
		}

		mock.ExpectQuery(submissions.CreateSolutionQuery).
			WithArgs(
				creation.ContestId,
				creation.ProblemId,
				creation.UserId,
				creation.Solution,
				creation.Language,
				creation.Penalty,
			).
			WillReturnRows(sqlmock.NewRows([]string{"wrong_column"}))

		id, err := repo.CreateSubmission(ctx, creation)
		assert.Error(t, err)
		assert.Equal(t, uuid.Nil, id)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRepository_GetSolution(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := submissions.NewRepository(db)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()

		solutionID := uuid.New()
		contestID := uuid.New()
		problemID := uuid.New()
		userID := uuid.New()
		now := time.Now()

		expected := &models.Submission{
			Id:           solutionID,
			UserId:       userID,
			Username:     "testuser",
			Solution:     "#include <iostream>\nint main() { return 0; }",
			State:        models.Accepted,
			Score:        100,
			Penalty:      20,
			TimeStat:     150,
			MemoryStat:   2048,
			Language:     models.Cpp,
			ProblemId:    problemID,
			ProblemTitle: "Test Problem",
			Position:     1,
			ContestId:    contestID,
			ContestTitle: "Test Contest",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		columns := []string{
			"id",
			"user_id",
			"username",
			"solution",
			"state",
			"score",
			"penalty",
			"time_stat",
			"memory_stat",
			"language",
			"problem_id",
			"problem_title",
			"position",
			"contest_id",
			"contest_title",
			"updated_at",
			"created_at",
		}

		mock.ExpectQuery(submissions.GetSolutionQuery).
			WithArgs(solutionID).
			WillReturnRows(sqlmock.NewRows(columns).
				AddRow(
					expected.Id,
					expected.UserId,
					expected.Username,
					expected.Solution,
					expected.State,
					expected.Score,
					expected.Penalty,
					expected.TimeStat,
					expected.MemoryStat,
					expected.Language,
					expected.ProblemId,
					expected.ProblemTitle,
					expected.Position,
					expected.ContestId,
					expected.ContestTitle,
					expected.UpdatedAt,
					expected.CreatedAt,
				))

		solution, err := repo.GetSubmissions(ctx, solutionID)
		assert.NoError(t, err)
		assert.EqualExportedValues(t, expected, solution)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		ctx := context.Background()

		solutionID := uuid.New()

		mock.ExpectQuery(submissions.GetSolutionQuery).
			WithArgs(solutionID).
			WillReturnError(sql.ErrNoRows)

		solution, err := repo.GetSubmissions(ctx, solutionID)
		assert.Error(t, err)
		assert.Nil(t, solution)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query error", func(t *testing.T) {
		ctx := context.Background()

		solutionID := uuid.New()

		mock.ExpectQuery(submissions.GetSolutionQuery).
			WithArgs(solutionID).
			WillReturnError(sql.ErrConnDone)

		solution, err := repo.GetSubmissions(ctx, solutionID)
		assert.Error(t, err)
		assert.Nil(t, solution)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRepository_UpdateSolution(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := submissions.NewRepository(db)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()

		solutionID := uuid.New()
		update := &models.SubmissionUpdate{
			State:      models.Accepted,
			Score:      100,
			TimeStat:   150,
			MemoryStat: 2048,
		}

		mock.ExpectExec(submissions.UpdateSolutionQuery).
			WithArgs(
				update.State,
				update.Score,
				update.TimeStat,
				update.MemoryStat,
				solutionID,
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.UpdateSubmission(ctx, solutionID, update)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		ctx := context.Background()

		solutionID := uuid.New()
		update := &models.SubmissionUpdate{
			State:      models.GotWA,
			Score:      0,
			TimeStat:   0,
			MemoryStat: 0,
		}

		mock.ExpectExec(submissions.UpdateSolutionQuery).
			WithArgs(
				update.State,
				update.Score,
				update.TimeStat,
				update.MemoryStat,
				solutionID,
			).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.UpdateSubmission(ctx, solutionID, update)
		assert.NoError(t, err) // Update with 0 rows affected is not an error
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query error", func(t *testing.T) {
		ctx := context.Background()

		solutionID := uuid.New()
		update := &models.SubmissionUpdate{
			State:      models.GotCE,
			Score:      0,
			TimeStat:   0,
			MemoryStat: 0,
		}

		mock.ExpectExec(submissions.UpdateSolutionQuery).
			WithArgs(
				update.State,
				update.Score,
				update.TimeStat,
				update.MemoryStat,
				solutionID,
			).
			WillReturnError(sql.ErrConnDone)

		err := repo.UpdateSubmission(ctx, solutionID, update)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRepository_ListSolutions(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := submissions.NewRepository(db)

	t.Run("success with filters", func(t *testing.T) {
		ctx := context.Background()

		contestID := uuid.New()
		userID := uuid.New()
		problemID := uuid.New()
		language := models.Cpp
		state := models.Accepted
		order := int32(1)

		filter := models.SolutionsFilter{
			Page:      1,
			PageSize:  10,
			ContestId: &contestID,
			UserId:    &userID,
			ProblemId: &problemID,
			Language:  &language,
			State:     &state,
			Order:     &order,
		}

		now := time.Now()
		solution1 := &models.SubmissionListItem{
			Id:           uuid.New(),
			UserId:       userID,
			Username:     "user1",
			State:        models.Accepted,
			Score:        100,
			Penalty:      20,
			TimeStat:     150,
			MemoryStat:   2048,
			Language:     models.Cpp,
			ProblemId:    problemID,
			ProblemTitle: "Problem 1",
			Position:     1,
			ContestId:    contestID,
			ContestTitle: "Contest 1",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		solution2 := &models.SubmissionListItem{
			Id:           uuid.New(),
			UserId:       userID,
			Username:     "user1",
			State:        models.Accepted,
			Score:        100,
			Penalty:      40,
			TimeStat:     200,
			MemoryStat:   1024,
			Language:     models.Cpp,
			ProblemId:    problemID,
			ProblemTitle: "Problem 1",
			Position:     1,
			ContestId:    contestID,
			ContestTitle: "Contest 1",
			CreatedAt:    now.Add(time.Minute),
			UpdatedAt:    now.Add(time.Minute),
		}

		var totalCount int32 = 2

		// Mock count query
		mock.ExpectQuery(submissions.CountSolutionsQuery).
			WithArgs(
				filter.ContestId,
				filter.UserId,
				filter.ProblemId,
				filter.Language,
				filter.State,
			).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(totalCount))

		// Mock list query
		columns := []string{
			"id",
			"user_id",
			"username",
			"state",
			"score",
			"penalty",
			"time_stat",
			"memory_stat",
			"language",
			"problem_id",
			"problem_title",
			"position",
			"contest_id",
			"contest_title",
			"updated_at",
			"created_at",
		}

		mock.ExpectQuery(submissions.ListSolutionsQuery).
			WithArgs(
				filter.ContestId,
				filter.UserId,
				filter.ProblemId,
				filter.Language,
				filter.State,
				int(order),
				filter.PageSize,
				filter.Offset(),
			).
			WillReturnRows(sqlmock.NewRows(columns).
				AddRow(
					solution1.Id,
					solution1.UserId,
					solution1.Username,
					solution1.State,
					solution1.Score,
					solution1.Penalty,
					solution1.TimeStat,
					solution1.MemoryStat,
					solution1.Language,
					solution1.ProblemId,
					solution1.ProblemTitle,
					solution1.Position,
					solution1.ContestId,
					solution1.ContestTitle,
					solution1.UpdatedAt,
					solution1.CreatedAt,
				).
				AddRow(
					solution2.Id,
					solution2.UserId,
					solution2.Username,
					solution2.State,
					solution2.Score,
					solution2.Penalty,
					solution2.TimeStat,
					solution2.MemoryStat,
					solution2.Language,
					solution2.ProblemId,
					solution2.ProblemTitle,
					solution2.Position,
					solution2.ContestId,
					solution2.ContestTitle,
					solution2.UpdatedAt,
					solution2.CreatedAt,
				))

		list, err := repo.ListSolutions(ctx, filter)
		assert.NoError(t, err)
		assert.NotNil(t, list)
		assert.Equal(t, 2, len(list.Solutions))
		assert.Equal(t, int32(1), list.Pagination.Page)
		assert.Equal(t, int32(1), list.Pagination.Total) // Total pages = ceil(2/10) = 1
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success without filters", func(t *testing.T) {
		ctx := context.Background()

		filter := models.SolutionsFilter{
			Page:     1,
			PageSize: 10,
		}

		var totalCount int32 = 0

		// Mock count query
		mock.ExpectQuery(submissions.CountSolutionsQuery).
			WithArgs(
				filter.ContestId,
				filter.UserId,
				filter.ProblemId,
				filter.Language,
				filter.State,
			).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(totalCount))

		// Mock list query
		columns := []string{
			"id",
			"user_id",
			"username",
			"state",
			"score",
			"penalty",
			"time_stat",
			"memory_stat",
			"language",
			"problem_id",
			"problem_title",
			"position",
			"contest_id",
			"contest_title",
			"updated_at",
			"created_at",
		}

		mock.ExpectQuery(submissions.ListSolutionsQuery).
			WithArgs(
				filter.ContestId,
				filter.UserId,
				filter.ProblemId,
				filter.Language,
				filter.State,
				0, // order = 0 when Order is nil
				filter.PageSize,
				filter.Offset(),
			).
			WillReturnRows(sqlmock.NewRows(columns))

		list, err := repo.ListSolutions(ctx, filter)
		assert.NoError(t, err)
		assert.NotNil(t, list)
		assert.Equal(t, 0, len(list.Solutions))
		assert.Equal(t, int32(1), list.Pagination.Page)
		assert.Equal(t, int32(0), list.Pagination.Total)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("count query error", func(t *testing.T) {
		ctx := context.Background()

		filter := models.SolutionsFilter{
			Page:     1,
			PageSize: 10,
		}

		mock.ExpectQuery(submissions.CountSolutionsQuery).
			WithArgs(
				filter.ContestId,
				filter.UserId,
				filter.ProblemId,
				filter.Language,
				filter.State,
			).
			WillReturnError(sql.ErrConnDone)

		list, err := repo.ListSolutions(ctx, filter)
		assert.Error(t, err)
		assert.Nil(t, list)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("list query error", func(t *testing.T) {
		ctx := context.Background()

		filter := models.SolutionsFilter{
			Page:     1,
			PageSize: 10,
		}

		var totalCount int32 = 5

		// Mock count query
		mock.ExpectQuery(submissions.CountSolutionsQuery).
			WithArgs(
				filter.ContestId,
				filter.UserId,
				filter.ProblemId,
				filter.Language,
				filter.State,
			).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(totalCount))

		// Mock list query error
		mock.ExpectQuery(submissions.ListSolutionsQuery).
			WithArgs(
				filter.ContestId,
				filter.UserId,
				filter.ProblemId,
				filter.Language,
				filter.State,
				0,
				filter.PageSize,
				filter.Offset(),
			).
			WillReturnError(sql.ErrConnDone)

		list, err := repo.ListSolutions(ctx, filter)
		assert.Error(t, err)
		assert.Nil(t, list)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("pagination", func(t *testing.T) {
		ctx := context.Background()

		filter := models.SolutionsFilter{
			Page:     2,
			PageSize: 5,
		}

		var totalCount int32 = 12

		// Mock count query
		mock.ExpectQuery(submissions.CountSolutionsQuery).
			WithArgs(
				filter.ContestId,
				filter.UserId,
				filter.ProblemId,
				filter.Language,
				filter.State,
			).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(totalCount))

		// Mock list query
		columns := []string{
			"id",
			"user_id",
			"username",
			"state",
			"score",
			"penalty",
			"time_stat",
			"memory_stat",
			"language",
			"problem_id",
			"problem_title",
			"position",
			"contest_id",
			"contest_title",
			"updated_at",
			"created_at",
		}

		mock.ExpectQuery(submissions.ListSolutionsQuery).
			WithArgs(
				filter.ContestId,
				filter.UserId,
				filter.ProblemId,
				filter.Language,
				filter.State,
				0,
				filter.PageSize,
				filter.Offset(), // Should be 5 for page 2 with pageSize 5
			).
			WillReturnRows(sqlmock.NewRows(columns))

		list, err := repo.ListSolutions(ctx, filter)
		assert.NoError(t, err)
		assert.NotNil(t, list)
		assert.Equal(t, int32(2), list.Pagination.Page)
		assert.Equal(t, int32(3), list.Pagination.Total) // Total pages = ceil(12/5) = 3
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
