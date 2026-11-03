package submissions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	paging "github.com/brawler2011/gate/backend/internal/page"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
)

type SubmissionsUseCase struct {
	submissionsRepo SubmissionsRepo
	contestsUC      ContestsUC
	problemsUC      ProblemsUC

	transactor SubmissionTransactions
}

func NewSubmissionsUseCase(
	submissionsRepo SubmissionsRepo,
	contestsUC ContestsUC,
	problemsUC ProblemsUC,
	transactor SubmissionTransactions,
) *SubmissionsUseCase {
	return &SubmissionsUseCase{
		submissionsRepo: submissionsRepo,
		contestsUC:      contestsUC,
		problemsUC:      problemsUC,

		transactor: transactor,
	}
}

func (uc *SubmissionsUseCase) GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error) {
	return uc.submissionsRepo.GetSubmission(ctx, id)
}

func (uc *SubmissionsUseCase) CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error) {
	_, err := uc.contestsUC.GetContest(ctx, creation.ContestId)
	if err != nil {
		return uuid.Nil, err
	}

	_, err = uc.problemsUC.GetProblemById(ctx, creation.ProblemId)
	if err != nil {
		return uuid.Nil, err
	}

	if creation.ContestId != uuid.Nil {
		_, err = uc.contestsUC.GetContestProblem(ctx, contestsmodel.ContestProblemGet{
			ContestId: creation.ContestId,
			ProblemId: creation.ProblemId,
		})
		if err != nil {
			return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "problem does not belong to contest")
		}

		block, err := uc.contestsUC.GetProblemBlockStatusForUser(ctx, creation.ContestId, creation.UserId, creation.ProblemId)
		if err == nil && block != nil {
			dqState := submissionsmodel.Disqualified
			creation.State = &dqState
			creation.BanReason = block.Reason

			id, err := uc.submissionsRepo.CreateSubmission(ctx, creation)
			if err != nil {
				return uuid.Nil, err
			}

			submission, err := uc.submissionsRepo.GetSubmission(ctx, id)
			if err == nil && uc.contestsUC != nil {
				_ = uc.contestsUC.ProcessSubmissionResult(ctx, &submission)
			}
			slog.InfoContext(ctx, "submission_accepted", "event", "submission_accepted", "submission_id", id, "request_id", requestid.Get(ctx), "state", submissionsmodel.Disqualified)
			return id, nil
		}
	}

	var id uuid.UUID

	err = uc.transactor.Within(ctx, func(tx SubmissionTxStore) error {
		id, err = tx.CreateSubmission(ctx, creation)
		if err != nil {
			return err
		}

		submission, err := tx.GetSubmission(ctx, id)
		if err != nil {
			return err
		}

		job, err := tx.ScheduleJob(ctx, id)
		if err != nil {
			return err
		}
		eventParams := newOutboxEventParams(submission, job, nil)

		if err := tx.Record(ctx, eventParams); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	slog.InfoContext(ctx, "submission_accepted", "event", "submission_accepted", "submission_id", id, "request_id", requestid.Get(ctx))
	return id, nil
}

func (uc *SubmissionsUseCase) BlockSubmission(ctx context.Context, contestID, submissionID uuid.UUID, reason *string) error {
	return uc.transactor.Within(ctx, func(tx SubmissionTxStore) error {
		sub, err := tx.GetSubmission(ctx, submissionID)
		if err != nil {
			return err
		}
		if contestID != uuid.Nil && (sub.ContestID == nil || *sub.ContestID != contestID) {
			return pkg.Wrap(pkg.ErrBadInput, nil, "submission does not belong to specified contest")
		}
		if sub.ContestID != nil {
			if err := tx.LockContest(ctx, *sub.ContestID); err != nil {
				return err
			}
		}
		// Read again after acquiring the shared scoring lock.
		sub, err = tx.GetSubmission(ctx, submissionID)
		if err != nil {
			return err
		}
		previous := sub.State
		if err := tx.BlockSubmission(ctx, submissionID, reason); err != nil {
			return err
		}
		sub, err = tx.GetSubmission(ctx, submissionID)
		if err != nil {
			return err
		}
		if err := contests.ApplySubmissionResult(ctx, tx, &sub); err != nil {
			return err
		}
		event := submissionsmodel.SubmissionCompletedEvent{Id: sub.ID, PreviousState: previous, State: sub.State, Score: sub.Score, Penalty: sub.Penalty, TimeStat: sub.TimeStat, MemoryStat: sub.MemoryStat, FailedTest: sub.FailedTest,
			SubmissionEventMeta: submissionsmodel.SubmissionEventMeta{Revision: sub.Revision, UpdatedAt: sub.UpdatedAt, UserId: sub.CreatedBy, Username: sub.Username, ContestId: sub.ContestID, ContestTitle: sub.ContestTitle, ProblemId: sub.ProblemID, ProblemTitle: sub.ProblemTitle, Position: sub.Position, Language: sub.Language, CreatedAt: sub.CreatedAt}}
		return tx.Record(ctx, eventdata.Event{AggregateID: sub.ID, Type: eventdata.OutboxEventSubmissionCompleted, Payload: event})
	})
}

func (uc *SubmissionsUseCase) UnblockSubmission(ctx context.Context, contestID, submissionID uuid.UUID) error {
	filter := submissionsmodel.RejudgeFilter{
		ContestID:    contestID,
		SubmissionID: &submissionID,
	}
	_, err := uc.RejudgeSubmissions(ctx, filter)
	return err
}

func newOutboxEventParams(submission submissionsmodel.Submission, job judging.Job, previousState *submissionsmodel.State) eventdata.Event {
	submissionCreatedEvent := submissionsmodel.SubmissionCreatedEvent{JobID: job.ID, Generation: job.Generation,
		IsNew: previousState == nil, PreviousState: previousState,
		SubmissionEventMeta: submissionsmodel.SubmissionEventMeta{
			Revision:     submission.Revision,
			UserId:       submission.CreatedBy,
			Username:     submission.Username,
			ContestId:    submission.ContestID,
			ContestTitle: submission.ContestTitle,
			ProblemId:    submission.ProblemID,
			ProblemTitle: submission.ProblemTitle,
			Position:     submission.Position,
			Language:     submission.Language,
			CreatedAt:    submission.CreatedAt,
			UpdatedAt:    submission.UpdatedAt,
		},
		Id:     submission.ID,
		State:  submission.State,
		Source: submission.Submission,
	}

	return eventdata.Event{AggregateID: submission.ID, Type: eventdata.OutboxEventSubmissionCreated, Payload: submissionCreatedEvent}
}

func (uc *SubmissionsUseCase) UpdateSubmission(ctx context.Context, id uuid.UUID, update *submissionsmodel.SubmissionUpdate) error {
	_, err := uc.submissionsRepo.UpdateSubmission(ctx, id, update)
	return err
}

func (uc *SubmissionsUseCase) ListSubmissions(ctx context.Context, filter submissionsmodel.SubmissionsFilter) (*submissionsmodel.SubmissionsList, error) {
	submissions, total, revision, err := uc.submissionsRepo.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &submissionsmodel.SubmissionsList{
		Submissions: submissions, TotalCount: total, Revision: revision,
		Pagination: paging.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *SubmissionsUseCase) RejudgeSubmissions(ctx context.Context, filter submissionsmodel.RejudgeFilter) (int, error) {
	if filter.ContestID == uuid.Nil {
		return 0, pkg.Wrap(pkg.ErrBadInput, nil, "contest id is required")
	}

	_, err := uc.contestsUC.GetContest(ctx, filter.ContestID)
	if err != nil {
		return 0, err
	}

	if filter.ProblemID != nil && *filter.ProblemID != uuid.Nil {
		_, err = uc.problemsUC.GetProblemById(ctx, *filter.ProblemID)
		if err != nil {
			return 0, err
		}
	}

	if filter.SubmissionID != nil && *filter.SubmissionID != uuid.Nil {
		sub, err := uc.submissionsRepo.GetSubmission(ctx, *filter.SubmissionID)
		if err != nil {
			return 0, err
		}
		if sub.ContestID != nil && *sub.ContestID != filter.ContestID {
			return 0, pkg.Wrap(pkg.ErrBadInput, nil, "submission does not belong to specified contest")
		}
	}

	var rejudgedCount int
	var rejudgedIDs []uuid.UUID

	err = uc.transactor.Within(ctx, func(tx SubmissionTxStore) error {
		if err := tx.LockContest(ctx, filter.ContestID); err != nil {
			return err
		}
		txSubRepo := tx
		txOutboxRepo := tx

		resetIDs, err := txSubRepo.ResetSubmissionsState(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to reset submissions state: %w", err)
		}

		for _, reset := range resetIDs {
			id := reset.ID
			submission, err := txSubRepo.GetSubmission(ctx, id)
			if err != nil {
				return fmt.Errorf("failed to get submission %s for rejudge: %w", id, err)
			}

			if err := contests.ApplySubmissionResult(ctx, tx, &submission); err != nil {
				return err
			}
			job, err := tx.ScheduleJob(ctx, id)
			if err != nil {
				return err
			}
			eventParams := newOutboxEventParams(submission, job, &reset.PreviousState)

			if err := txOutboxRepo.Record(ctx, eventParams); err != nil {
				return fmt.Errorf("failed to create outbox event for submission %s: %w", id, err)
			}
		}

		rejudgedCount = len(resetIDs)
		for _, reset := range resetIDs {
			rejudgedIDs = append(rejudgedIDs, reset.ID)
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	for _, id := range rejudgedIDs {
		slog.InfoContext(ctx, "submission_rejudge_requested", "event", "submission_rejudge_requested", "submission_id", id, "request_id", requestid.Get(ctx))
	}
	return rejudgedCount, nil
}
