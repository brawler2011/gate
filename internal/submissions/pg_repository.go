package submissions

import (
	"context"

	"github.com/gate149/core/internal/models"
	submissionssqlc "github.com/gate149/core/internal/submissions/sqlc"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgRepository struct {
	queries *submissionssqlc.Queries
}

func NewRepository(db *pgxpool.Pool) *PgRepository {
	return &PgRepository{
		queries: submissionssqlc.New(db),
	}
}

func (r *PgRepository) GetSubmission(ctx context.Context, id uuid.UUID) (submissionssqlc.GetSubmissionRow, error) {
	row, err := r.queries.GetSubmission(ctx, id)
	if err != nil {
		return submissionssqlc.GetSubmissionRow{}, pkg.HandlePgErr(err)
	}
	return row, nil
}

func (r *PgRepository) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	id, err := r.queries.CreateSubmission(ctx, submissionssqlc.CreateSubmissionParams{
		ContestID:  creation.ContestId,
		ProblemID:  creation.ProblemId,
		CreatedBy:  creation.UserId,
		Submission: creation.Solution,
		Language:   creation.Language,
		Penalty:    int32(creation.Penalty),
	})
	if err != nil {
		return uuid.Nil, pkg.HandlePgErr(err)
	}
	return id, nil
}

func (r *PgRepository) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	var failedTest *int32
	if update.FailedTest != nil {
		val := int32(*update.FailedTest)
		failedTest = &val
	}
	
	err := r.queries.UpdateSubmission(ctx, submissionssqlc.UpdateSubmissionParams{
		State:      update.State,
		Score:      int32(update.Score),
		TimeStat:   int32(update.TimeStat),
		MemoryStat: int32(update.MemoryStat),
		FailedTest: failedTest,
		ID:         id,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *PgRepository) ListSolutions(ctx context.Context, filter models.SolutionsFilter) ([]submissionssqlc.ListSubmissionsRow, int64, error) {
	var sortOrder *int32
	if filter.Order != nil {
		val := int32(*filter.Order)
		sortOrder = &val
	}

	// Get count
	totalCount, err := r.queries.CountSubmissions(ctx, submissionssqlc.CountSubmissionsParams{
		ContestID: nullableUUIDToPgtype(filter.ContestId),
		CreatedBy: nullableUUIDToPgtype(filter.UserId),
		ProblemID: nullableUUIDToPgtype(filter.ProblemId),
		Language:  languagePtrToInt32Ptr(filter.Language),
		State:     statePtrToInt32Ptr(filter.State),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	// Get submissions list
	rows, err := r.queries.ListSubmissions(ctx, submissionssqlc.ListSubmissionsParams{
		Limit:     int32(filter.PageSize),
		Offset:    int32(filter.Offset()),
		ContestID: nullableUUIDToPgtype(filter.ContestId),
		CreatedBy: nullableUUIDToPgtype(filter.UserId),
		ProblemID: nullableUUIDToPgtype(filter.ProblemId),
		Language:  languagePtrToInt32Ptr(filter.Language),
		State:     statePtrToInt32Ptr(filter.State),
		SortOrder: sortOrder,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return rows, totalCount, nil
}

func (r *PgRepository) GetUntestedSubmissions(ctx context.Context, limit int32) ([]submissionssqlc.GetUntestedSubmissionsRow, error) {
	rows, err := r.queries.GetUntestedSubmissions(ctx, limit)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}
	return rows, nil
}

// Helper functions

func nullableUUIDToPgtype(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func languagePtrToInt32Ptr(l *models.LanguageName) *int32 {
	if l == nil {
		return nil
	}
	val := int32(*l)
	return &val
}

func statePtrToInt32Ptr(s *models.State) *int32 {
	if s == nil {
		return nil
	}
	val := int32(*s)
	return &val
}
