package postgres

import (
	"context"
	"errors"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	judging "github.com/brawler2011/gate/backend/internal/judging"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type JudgeJobs struct{ queries *sqlc.Queries }

func NewJudgeJobs(pool *pgxpool.Pool) *JudgeJobs { return &JudgeJobs{queries: sqlc.New(pool)} }

func (r *JudgeJobs) ScheduleJob(ctx context.Context, id uuid.UUID) (judgingmodel.Job, error) {
	row, err := r.queries.ScheduleJudgeJob(ctx, sqlc.ScheduleJudgeJobParams{ID: uuid.New(), SubmissionID: id})
	if err != nil {
		return judgingmodel.Job{}, HandlePgErr(err)
	}
	return mapJudgeJob(row), nil
}

func (r *JudgeJobs) Claim(ctx context.Context, reference judgingmodel.JobRef) (judgingmodel.Job, error) {
	row, err := r.queries.ClaimJudgeJob(ctx, sqlc.ClaimJudgeJobParams{ID: reference.ID, Generation: reference.Generation, SubmissionID: reference.SubmissionID, LeaseToken: uuid.New()})
	if errors.Is(err, pgx.ErrNoRows) {
		active, err := r.queries.JudgeJobActive(ctx, sqlc.JudgeJobActiveParams{ID: reference.ID, Generation: reference.Generation, SubmissionID: reference.SubmissionID})
		if err != nil {
			return judgingmodel.Job{}, HandlePgErr(err)
		}
		if active {
			return judgingmodel.Job{}, judging.ErrBusy
		}
		return judgingmodel.Job{}, nil
	}
	if err != nil {
		return judgingmodel.Job{}, HandlePgErr(err)
	}
	return mapJudgeJob(row), nil
}

func (r *JudgeJobs) Renew(ctx context.Context, job judgingmodel.Job) error {
	count, err := r.queries.RenewJudgeJob(ctx, sqlc.RenewJudgeJobParams{ID: job.ID, LeaseToken: job.LeaseToken})
	if err != nil {
		return HandlePgErr(err)
	}
	if count == 0 {
		return judging.ErrLeaseLost
	}
	return nil
}

func (r *JudgeJobs) Release(ctx context.Context, job judgingmodel.Job) error {
	_, err := r.queries.ReleaseJudgeJob(ctx, sqlc.ReleaseJudgeJobParams{ID: job.ID, LeaseToken: job.LeaseToken})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *JudgeJobs) CompleteJob(ctx context.Context, job judgingmodel.Job) (bool, error) {
	identifiers, err := r.queries.GetSubmissionIdentifiers(ctx, job.SubmissionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, HandlePgErr(err)
	}
	if identifiers.ContestID != nil && identifiers.OwnerID != nil && identifiers.ProblemID != nil {
		if err := r.queries.AdvisoryTransactionLock(ctx, submissionTupleKey(*identifiers.ContestID, *identifiers.OwnerID, *identifiers.ProblemID)); err != nil {
			return false, HandlePgErr(err)
		}
	}
	if _, err := r.queries.LockSubmission(ctx, job.SubmissionID); err != nil {
		return false, HandlePgErr(err)
	}
	count, err := r.queries.CompleteJudgeJob(ctx, sqlc.CompleteJudgeJobParams{ID: job.ID, SubmissionID: job.SubmissionID, Generation: job.Generation, LeaseToken: job.LeaseToken})
	if err != nil {
		return false, HandlePgErr(err)
	}
	return count > 0, nil
}

func mapJudgeJob(row sqlc.JudgeJob) judgingmodel.Job {
	job := judgingmodel.Job{ID: row.ID, SubmissionID: row.SubmissionID, Generation: row.Generation,
		PackageID: row.PackageID, PackageHash: row.PackageHash, Attempt: row.Attempt}
	if row.LeaseToken != nil {
		job.LeaseToken = *row.LeaseToken
	}
	if row.DeadlineAt != nil {
		job.Deadline = *row.DeadlineAt
	}
	return job
}
