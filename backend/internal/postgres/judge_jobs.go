package postgres

import (
	"context"
	"errors"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"

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
		return judgingmodel.Job{}, dbError("schedule job", err)
	}
	return mapJudgeJob(row), r.wakeWorkers(ctx)
}

func (r *JudgeJobs) ClaimNext(ctx context.Context) (judgingmodel.Job, bool, error) {
	row, err := r.queries.ClaimNextJudgeJob(ctx, uuid.New())
	if errors.Is(err, pgx.ErrNoRows) {
		return judgingmodel.Job{}, false, nil
	}
	if err != nil {
		return judgingmodel.Job{}, false, dbError("claim", err)
	}
	return mapJudgeJob(row), true, nil
}

func (r *JudgeJobs) wakeWorkers(ctx context.Context) error {
	if err := r.queries.WakeJudgeWorkers(ctx); err != nil {
		return dbError("wake judge workers", err)
	}
	return nil
}

func (r *JudgeJobs) DeleteObsolete(ctx context.Context) error {
	if _, err := r.queries.DeleteObsoleteJudgeJobs(ctx); err != nil {
		return dbError("delete obsolete judge jobs", err)
	}
	return nil
}

func (r *JudgeJobs) Renew(ctx context.Context, job judgingmodel.Job) error {
	count, err := r.queries.RenewJudgeJob(ctx, sqlc.RenewJudgeJobParams{ID: job.ID, LeaseToken: job.LeaseToken})
	if err != nil {
		return dbError("renew", err)
	}
	if count == 0 {
		return judging.ErrLeaseLost
	}
	return nil
}

func (r *JudgeJobs) Release(ctx context.Context, job judgingmodel.Job) error {
	count, err := r.queries.ReleaseJudgeJob(ctx, sqlc.ReleaseJudgeJobParams{ID: job.ID, LeaseToken: job.LeaseToken})
	if err != nil {
		return dbError("release", err)
	}
	if count == 0 {
		return nil
	}
	return r.wakeWorkers(ctx)
}

func (r *JudgeJobs) CompleteJob(ctx context.Context, job judgingmodel.Job) (bool, error) {
	sub, err := r.queries.LockSubmission(ctx, job.SubmissionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, dbError("complete job", err)
	}
	count, err := r.queries.CompleteJudgeJob(ctx, sqlc.CompleteJudgeJobParams{ID: job.ID, SubmissionID: job.SubmissionID, Revision: job.Revision, LeaseToken: job.LeaseToken})
	if err != nil {
		return false, dbError("complete job", err)
	}
	if count > 0 {
		return true, nil
	}
	if sub.Revision != job.Revision || sub.State == submissionsmodel.Disqualified {
		return false, nil
	}
	return false, judging.ErrLeaseLost
}

func (r *JudgeJobs) PickStalledJobs(ctx context.Context, limit int32) ([]judgingmodel.Job, error) {
	rows, err := r.queries.PickStalledJudgeJobs(ctx, limit)
	if err != nil {
		return nil, dbError("pick stalled jobs", err)
	}
	jobs := make([]judgingmodel.Job, len(rows))
	for i, row := range rows {
		jobs[i] = judgingmodel.Job{
			ID:           row.ID,
			SubmissionID: row.SubmissionID,
			Revision:     row.Revision,
			Attempt:      row.Attempt,
		}
	}
	return jobs, nil
}

func (r *JudgeJobs) FailStalledJob(ctx context.Context, reference judgingmodel.JobRef) (bool, error) {
	sub, err := r.queries.LockSubmission(ctx, reference.SubmissionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, r.completeStalled(ctx, reference)
		}
		return false, dbError("fail stalled job", err)
	}
	if sub.Revision != reference.Revision || sub.State == submissionsmodel.Disqualified {
		return false, r.completeStalled(ctx, reference)
	}
	count, err := r.queries.DeleteStalledJudgeJob(ctx, sqlc.DeleteStalledJudgeJobParams{ID: reference.ID, Revision: reference.Revision})
	if err != nil {
		return false, dbError("fail stalled job", err)
	}
	return count > 0, nil
}

func (r *JudgeJobs) RequeueStalledJob(ctx context.Context, reference judgingmodel.JobRef) (bool, error) {
	sub, err := r.queries.LockSubmission(ctx, reference.SubmissionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, r.completeStalled(ctx, reference)
		}
		return false, dbError("requeue stalled job", err)
	}
	if sub.Revision != reference.Revision || sub.State == submissionsmodel.Disqualified {
		return false, r.completeStalled(ctx, reference)
	}
	count, err := r.queries.RequeueStalledJudgeJob(ctx, sqlc.RequeueStalledJudgeJobParams{ID: reference.ID, Revision: reference.Revision})
	if err != nil {
		return false, dbError("requeue stalled job", err)
	}
	if count == 0 {
		return false, nil
	}
	return true, r.wakeWorkers(ctx)
}

func mapJudgeJob(row sqlc.JudgeJob) judgingmodel.Job {
	job := judgingmodel.Job{ID: row.ID, SubmissionID: row.SubmissionID, Revision: row.Revision,
		PackageID: row.PackageID, PackageHash: row.PackageHash, Attempt: row.Attempt, CreatedAt: row.CreatedAt}
	if row.LeaseToken != nil {
		job.LeaseToken = *row.LeaseToken
	}
	if row.DeadlineAt != nil {
		job.Deadline = *row.DeadlineAt
	}
	return job
}

// completeStalled retires a job whose submission is gone or superseded.
func (r *JudgeJobs) completeStalled(ctx context.Context, reference judgingmodel.JobRef) error {
	if _, err := r.queries.DeleteStalledJudgeJob(ctx, sqlc.DeleteStalledJudgeJobParams{ID: reference.ID, Revision: reference.Revision}); err != nil {
		return dbError("complete stalled judge job", err)
	}
	return nil
}
