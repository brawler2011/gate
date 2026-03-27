package pg

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SubmissionsRepo struct {
	queries *sqlc.Queries
}

func NewSubmissionsRepo(db *pgxpool.Pool) *SubmissionsRepo {
	return &SubmissionsRepo{
		queries: sqlc.New(db),
	}
}

func (r *SubmissionsRepo) WithTx(tx pgx.Tx) interfaces.SubmissionsRepo {
	return &SubmissionsRepo{
		queries: sqlc.New(tx),
	}
}

func (r *SubmissionsRepo) GetSubmission(ctx context.Context, id uuid.UUID) (models.Submission, error) {
	row, err := r.queries.GetSubmission(ctx, id)
	if err != nil {
		return models.Submission{}, HandlePgErr(err)
	}
	return mapGetSubmissionRow(row), nil
}

func (r *SubmissionsRepo) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	id, err := r.queries.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		ContestID: creation.ContestId,
		ProblemID: creation.ProblemId,
		OwnerID:   creation.UserId,
		Source:    creation.Solution,
		Language:  creation.Language,
		Penalty:   int32(creation.Penalty),
	})
	if err != nil {
		return uuid.Nil, HandlePgErr(err)
	}
	return id, nil
}

func (r *SubmissionsRepo) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	err := r.queries.UpdateSubmission(ctx, sqlc.UpdateSubmissionParams{
		State:      update.State,
		Score:      int32(update.Score),
		TimeStat:   int32(update.TimeStat),
		MemoryStat: int32(update.MemoryStat),
		ID:         id,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *SubmissionsRepo) ListSubmissions(ctx context.Context, filter models.SubmissionsFilter) ([]models.Submission, int32, error) {
	var sortOrder *int32
	if filter.Order != nil {
		val := int32(*filter.Order)
		sortOrder = &val
	}

	totalCount, err := r.queries.CountSubmissions(ctx, sqlc.CountSubmissionsParams{
		ContestID: nullableUUIDToPgtype(filter.ContestId),
		OwnerID:   nullableUUIDToPgtype(filter.UserId),
		ProblemID: nullableUUIDToPgtype(filter.ProblemId),
		Language:  languagePtrToInt32Ptr(filter.Language),
		State:     statePtrToInt32Ptr(filter.State),
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	rows, err := r.queries.ListSubmissions(ctx, sqlc.ListSubmissionsParams{
		Limit:     int32(filter.PageSize),
		Offset:    int32((filter.Page - 1) * filter.PageSize),
		ContestID: nullableUUIDToPgtype(filter.ContestId),
		OwnerID:   nullableUUIDToPgtype(filter.UserId),
		ProblemID: nullableUUIDToPgtype(filter.ProblemId),
		Language:  languagePtrToInt32Ptr(filter.Language),
		State:     statePtrToInt32Ptr(filter.State),
		SortOrder: sortOrder,
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	submissions := make([]models.Submission, len(rows))
	for i, row := range rows {
		submissions[i] = mapListSubmissionsRow(row)
	}

	return submissions, int32(totalCount), nil
}

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

func mapGetSubmissionRow(row sqlc.GetSubmissionRow) models.Submission {
	var createdBy *uuid.UUID
	if row.OwnerID.Valid {
		cb := uuid.UUID(row.OwnerID.Bytes)
		createdBy = &cb
	}

	var problemID *uuid.UUID
	if row.ProblemID.Valid {
		pid := uuid.UUID(row.ProblemID.Bytes)
		problemID = &pid
	}

	var contestID *uuid.UUID
	if row.ContestID.Valid {
		cid := uuid.UUID(row.ContestID.Bytes)
		contestID = &cid
	}

	var position *int32
	if row.ProblemOrdinal != nil {
		position = row.ProblemOrdinal
	}

	var username string
	if row.Username != nil {
		username = *row.Username
	}

	// Prefer title from linked entities, fallback to short_name.
	var problemTitle string
	if row.ProblemTitle != nil {
		problemTitle = *row.ProblemTitle
	} else if row.ProblemShortName != nil {
		problemTitle = *row.ProblemShortName
	}

	var contestTitle string
	if row.ContestTitle != nil {
		contestTitle = *row.ContestTitle
	} else if row.ContestShortName != nil {
		contestTitle = *row.ContestShortName
	}

	return models.Submission{
		ID:           row.ID,
		CreatedBy:    createdBy,
		Username:     username,
		Submission:   row.Source,
		State:        row.State,
		Score:        row.Score,
		Penalty:      row.Penalty,
		TimeStat:     row.TimeStat,
		MemoryStat:   row.MemoryStat,
		Language:     row.Language,
		ProblemID:    problemID,
		ProblemTitle: problemTitle,
		Position:     position,
		ContestID:    contestID,
		ContestTitle: contestTitle,
		UpdatedAt:    row.UpdatedAt,
		CreatedAt:    row.CreatedAt,
	}
}

func mapListSubmissionsRow(row sqlc.ListSubmissionsRow) models.Submission {
	var createdBy *uuid.UUID
	if row.OwnerID.Valid {
		cb := uuid.UUID(row.OwnerID.Bytes)
		createdBy = &cb
	}

	var problemID *uuid.UUID
	if row.ProblemID.Valid {
		pid := uuid.UUID(row.ProblemID.Bytes)
		problemID = &pid
	}

	var contestID *uuid.UUID
	if row.ContestID.Valid {
		cid := uuid.UUID(row.ContestID.Bytes)
		contestID = &cid
	}

	var position *int32
	if row.ProblemOrdinal != nil {
		position = row.ProblemOrdinal
	}

	var username string
	if row.Username != nil {
		username = *row.Username
	}

	// Prefer title from linked entities, fallback to short_name.
	var problemTitle string
	if row.ProblemTitle != nil {
		problemTitle = *row.ProblemTitle
	} else if row.ProblemShortName != nil {
		problemTitle = *row.ProblemShortName
	}

	var contestTitle string
	if row.ContestTitle != nil {
		contestTitle = *row.ContestTitle
	} else if row.ContestShortName != nil {
		contestTitle = *row.ContestShortName
	}

	return models.Submission{
		ID:           row.ID,
		CreatedBy:    createdBy,
		Username:     username,
		Submission:   "",
		State:        row.State,
		Score:        row.Score,
		Penalty:      row.Penalty,
		TimeStat:     row.TimeStat,
		MemoryStat:   row.MemoryStat,
		Language:     row.Language,
		ProblemID:    problemID,
		ProblemTitle: problemTitle,
		Position:     position,
		ContestID:    contestID,
		ContestTitle: contestTitle,
		UpdatedAt:    row.UpdatedAt,
		CreatedAt:    row.CreatedAt,
	}
}
