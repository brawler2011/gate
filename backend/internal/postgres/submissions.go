package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SubmissionsRepo struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewSubmissionsRepo(db *pgxpool.Pool) *SubmissionsRepo {
	return &SubmissionsRepo{
		queries: sqlc.New(db), pool: db,
	}
}

func (r *SubmissionsRepo) WithTx(tx pgx.Tx) *SubmissionsRepo {
	return &SubmissionsRepo{
		queries: sqlc.New(tx),
	}
}

func (r *SubmissionsRepo) GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error) {
	row, err := r.queries.GetSubmission(ctx, id)
	if err != nil {
		return submissionsmodel.Submission{
			Revision: row.Revision}, HandlePgErr(err)
	}
	return mapGetSubmissionRow(row), nil
}

func (r *SubmissionsRepo) CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error) {
	id, err := r.queries.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		ContestID: creation.ContestId,
		ProblemID: creation.ProblemId,
		OwnerID:   creation.UserId,
		Source:    creation.Solution,
		Language:  creation.Language,
		Penalty:   int32(creation.Penalty),
		State:     creation.State,
		BanReason: creation.BanReason,
	})
	if err != nil {
		return uuid.Nil, HandlePgErr(err)
	}
	return id, nil
}

func (r *SubmissionsRepo) UpdateSubmission(ctx context.Context, id uuid.UUID, update *submissionsmodel.SubmissionUpdate) (submissionsmodel.SubmissionChange, error) {
	var detailsBytes []byte

	testDetails := update.TestDetails
	if testDetails == nil {
		testDetails = &submissionsmodel.SubmissionTestDetails{}
	}
	if update.TimeStat > 0 {
		testDetails.TimeStat = &update.TimeStat
	}
	if update.MemoryStat > 0 {
		testDetails.MemoryStat = &update.MemoryStat
	}
	if update.FailedTest != nil {
		testDetails.FailedTest = update.FailedTest
	}

	var err error
	detailsBytes, err = json.Marshal(testDetails)
	if err != nil {
		return submissionsmodel.SubmissionChange{}, fmt.Errorf("marshal test details: %w", err)
	}

	change, err := r.queries.UpdateSubmission(ctx, sqlc.UpdateSubmissionParams{
		State:       update.State,
		Score:       int32(update.Score),
		TestDetails: detailsBytes,
		ID:          id,
	})
	if err != nil {
		return submissionsmodel.SubmissionChange{}, HandlePgErr(err)
	}
	return submissionsmodel.SubmissionChange{Revision: change.Revision, PreviousState: change.PreviousState}, nil
}

func (r *SubmissionsRepo) BlockSubmission(ctx context.Context, id uuid.UUID, reason *string) error {
	err := r.queries.BlockSubmission(ctx, sqlc.BlockSubmissionParams{
		ID:        id,
		BanReason: reason,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *SubmissionsRepo) BlockUserProblemSubmissions(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string) ([]submissionsmodel.SubmissionReset, error) {
	rows, err := r.queries.BlockUserProblemSubmissions(ctx, sqlc.BlockUserProblemSubmissionsParams{
		ContestID: contestID,
		OwnerID:   userID,
		ProblemID: problemID,
		BanReason: reason,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}
	result := make([]submissionsmodel.SubmissionReset, len(rows))
	for i, row := range rows {
		result[i] = submissionsmodel.SubmissionReset{ID: row.ID, PreviousState: row.PreviousState}
	}
	return result, nil
}

func (r *SubmissionsRepo) ListSubmissions(ctx context.Context, filter submissionsmodel.SubmissionsFilter) ([]submissionsmodel.Submission, int32, int64, error) {
	if r.pool != nil {
		tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if err != nil {
			return nil, 0, 0, HandlePgErr(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		rows, count, revision, err := r.WithTx(tx).ListSubmissions(ctx, filter)
		if err != nil {
			return nil, 0, 0, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, 0, 0, HandlePgErr(err)
		}
		return rows, count, revision, nil
	}
	revision, err := r.queries.GetSubmissionsRevision(ctx, filter.ContestId)
	if err != nil {
		return nil, 0, 0, HandlePgErr(err)
	}
	var sortOrder *int32
	if filter.Order != nil {
		val := int32(*filter.Order)
		sortOrder = &val
	}

	totalCount, err := r.queries.CountSubmissions(ctx, sqlc.CountSubmissionsParams{
		ContestID: filter.ContestId,
		OwnerID:   filter.UserId,
		ProblemID: filter.ProblemId,
		Language:  languagePtrToInt32Ptr(filter.Language),
		State:     statePtrToInt32Ptr(filter.State),
	})
	if err != nil {
		return nil, 0, 0, HandlePgErr(err)
	}

	rows, err := r.queries.ListSubmissions(ctx, sqlc.ListSubmissionsParams{
		Limit:     int32(filter.PageSize),
		Offset:    int32((filter.Page - 1) * filter.PageSize),
		ContestID: filter.ContestId,
		OwnerID:   filter.UserId,
		ProblemID: filter.ProblemId,
		Language:  languagePtrToInt32Ptr(filter.Language),
		State:     statePtrToInt32Ptr(filter.State),
		SortOrder: sortOrder,
	})
	if err != nil {
		return nil, 0, 0, HandlePgErr(err)
	}

	submissions := make([]submissionsmodel.Submission, len(rows))
	for i, row := range rows {
		submissions[i] = mapListSubmissionsRow(row)
	}

	return submissions, safeInt32(totalCount), revision, nil
}

func languagePtrToInt32Ptr(l *submissionsmodel.LanguageName) *int32 {
	if l == nil {
		return nil
	}
	val := int32(*l)
	return &val
}

func statePtrToInt32Ptr(s *submissionsmodel.State) *int32 {
	if s == nil {
		return nil
	}
	val := int32(*s)
	return &val
}

func mapGetSubmissionRow(row sqlc.GetSubmissionRow) submissionsmodel.Submission {
	createdBy := row.OwnerID

	problemID := row.ProblemID

	contestID := row.ContestID

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
	var contestLogin string
	if row.ContestTitle != nil {
		contestTitle = *row.ContestTitle
	} else if row.ContestLogin != nil {
		contestTitle = *row.ContestLogin
	}
	if row.ContestLogin != nil {
		contestLogin = *row.ContestLogin
	}

	var testDetails *submissionsmodel.SubmissionTestDetails
	if len(row.TestDetails) > 0 {
		var td submissionsmodel.SubmissionTestDetails
		if err := json.Unmarshal(row.TestDetails, &td); err == nil {
			testDetails = &td
		}
	}

	var failedTest *int32
	if row.FailedTest > 0 {
		ft := row.FailedTest
		failedTest = &ft
	}

	return submissionsmodel.Submission{
		Revision:          row.Revision,
		Generation:        row.Generation,
		ID:                row.ID,
		CreatedBy:         createdBy,
		Username:          username,
		Submission:        row.Source,
		State:             row.State,
		Score:             row.Score,
		Penalty:           row.Penalty,
		TimeStat:          row.TimeStat,
		MemoryStat:        row.MemoryStat,
		Language:          row.Language,
		FailedTest:        failedTest,
		TestDetails:       testDetails,
		BanReason:         row.BanReason,
		ProblemID:         problemID,
		ProblemTitle:      problemTitle,
		Position:          position,
		ContestID:         contestID,
		ContestLogin:      contestLogin,
		ContestTitle:      contestTitle,
		OrganizationLogin: row.OrganizationLogin,
		UpdatedAt:         row.UpdatedAt,
		CreatedAt:         row.CreatedAt,
	}
}

func mapListSubmissionsRow(row sqlc.ListSubmissionsRow) submissionsmodel.Submission {
	createdBy := row.OwnerID

	problemID := row.ProblemID

	contestID := row.ContestID

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
	var contestLogin string
	if row.ContestTitle != nil {
		contestTitle = *row.ContestTitle
	} else if row.ContestLogin != nil {
		contestTitle = *row.ContestLogin
	}
	if row.ContestLogin != nil {
		contestLogin = *row.ContestLogin
	}

	var failedTest *int32
	if row.FailedTest > 0 {
		ft := row.FailedTest
		failedTest = &ft
	}

	return submissionsmodel.Submission{
		Revision:          row.Revision,
		Generation:        row.Generation,
		ID:                row.ID,
		CreatedBy:         createdBy,
		Username:          username,
		Submission:        "",
		State:             row.State,
		Score:             row.Score,
		Penalty:           row.Penalty,
		TimeStat:          row.TimeStat,
		MemoryStat:        row.MemoryStat,
		Language:          row.Language,
		FailedTest:        failedTest,
		BanReason:         row.BanReason,
		ProblemID:         problemID,
		ProblemTitle:      problemTitle,
		Position:          position,
		ContestID:         contestID,
		ContestLogin:      contestLogin,
		ContestTitle:      contestTitle,
		OrganizationLogin: row.OrganizationLogin,
		UpdatedAt:         row.UpdatedAt,
		CreatedAt:         row.CreatedAt,
	}
}

func (r *SubmissionsRepo) ResetSubmissionsState(ctx context.Context, filter submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error) {
	rows, err := r.queries.ResetSubmissionsState(ctx, sqlc.ResetSubmissionsStateParams{
		ContestID:    filter.ContestID,
		ProblemID:    filter.ProblemID,
		SubmissionID: filter.SubmissionID,
		OwnerID:      filter.UserID,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}
	result := make([]submissionsmodel.SubmissionReset, len(rows))
	for i, row := range rows {
		result[i] = submissionsmodel.SubmissionReset{ID: row.ID, PreviousState: row.PreviousState}
	}
	return result, nil
}
