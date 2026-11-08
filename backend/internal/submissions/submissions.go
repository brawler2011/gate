package submissions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
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

	transactor  SubmissionTransactions
	permissions Permissions
}

func NewSubmissionsUseCase(
	submissionsRepo SubmissionsRepo,
	contestsUC ContestsUC,
	problemsUC ProblemsUC,
	transactor SubmissionTransactions,
	permissions Permissions,
) *SubmissionsUseCase {
	return &SubmissionsUseCase{
		submissionsRepo: submissionsRepo,
		contestsUC:      contestsUC,
		problemsUC:      problemsUC,

		transactor:  transactor,
		permissions: permissions,
	}
}

func (uc *SubmissionsUseCase) GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return submissionsmodel.Submission{}, pkg.ErrUnauthenticated
	}
	sub, err := uc.submissionsRepo.GetSubmission(ctx, id)
	if err != nil {
		return sub, err
	}
	return uc.submissionView(ctx, sub, true)
}

func (uc *SubmissionsUseCase) submissionView(ctx context.Context, sub submissionsmodel.Submission, requireReadPermission bool) (submissionsmodel.Submission, error) {
	user := accessmodel.UserFromContext(ctx)
	own := sub.CreatedBy != nil && *sub.CreatedBy == user.Id
	if sub.ContestID == nil {
		if !own && !user.IsAdmin() {
			return submissionsmodel.Submission{}, pkg.NoPermission
		}
		return sub, nil
	}
	permissions, err := uc.permissions.GetContestPermissions(ctx, *sub.ContestID, user.Id)
	if err != nil {
		return submissionsmodel.Submission{}, err
	}
	if requireReadPermission && (own && !permissions.GetOwnSubmission || !own && !permissions.GetOtherUserSubmission) {
		return submissionsmodel.Submission{}, pkg.NoPermission
	}
	contest, err := uc.contestsUC.GetContest(ctx, *sub.ContestID)
	if err != nil {
		return submissionsmodel.Submission{}, err
	}
	return SubmissionView(&contest, sub.CreatedBy, sub.CreatedAt, user.Id, permissions, accessmodel.ReadTime(ctx)).Apply(sub), nil
}

func (uc *SubmissionsUseCase) CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error) {
	var id uuid.UUID
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		id, err = uc.createSubmission(ctx, creation)
		if !errors.Is(err, pkg.ErrConflict) {
			return id, err
		}
	}
	return uuid.Nil, err
}

func (uc *SubmissionsUseCase) createSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error) {
	var err error
	ctx, err = uc.transactor.Prepare(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if err := uc.requireContest(ctx, creation.ContestId, accessmodel.ActionCreateSubmission); err != nil {
		return uuid.Nil, err
	}

	if creation.UserId != accessmodel.UserFromContext(ctx).Id {
		return uuid.Nil, pkg.NoPermission
	}

	contest, err := uc.contestsUC.GetContest(ctx, creation.ContestId)
	if err != nil {
		return uuid.Nil, err
	}

	ctx = accessmodel.WithDecisionDeadline(ctx, contest.EndTime)
	var id uuid.UUID

	err = uc.transactor.Within(ctx, func(tx SubmissionTxStore) error {
		if err := tx.LockSubmissionTuple(ctx, creation.ContestId, creation.UserId, creation.ProblemId); err != nil {
			return err
		}

		if _, err := tx.GetContestProblem(ctx, contestsmodel.ContestProblemGet{ContestId: creation.ContestId, ProblemId: creation.ProblemId}); err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "problem does not belong to contest")
		}
		input := *creation
		block, err := tx.GetContestUserProblemBlock(ctx, input.ContestId, input.UserId, input.ProblemId)
		if err != nil && !errors.Is(err, pkg.ErrNotFound) {
			return err
		}
		if block != nil {
			state := submissionsmodel.Disqualified
			input.State, input.BanReason = &state, block.Reason
		}

		id, err = tx.CreateSubmission(ctx, &input)
		if err != nil {
			return err
		}

		submission, err := tx.GetSubmission(ctx, id)
		if err != nil {
			return err
		}

		if submission.State == submissionsmodel.Disqualified {
			if err := contests.ApplySubmissionResult(ctx, tx, &submission); err != nil {
				return err
			}
			if err := tx.Record(ctx, newOutboxEventParams(submission, judgingmodel.Job{}, nil)); err != nil {
				return err
			}
			return tx.Record(ctx, eventdata.Event{AggregateID: submission.ID, Type: eventdata.OutboxEventSubmissionCompleted, Payload: submission.CompletedEvent(submissionsmodel.Saved)})
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
	if err := uc.requireContest(ctx, contestID, accessmodel.ActionManageContest); err != nil {
		return err
	}

	return uc.transactor.Within(ctx, func(tx SubmissionTxStore) error {
		sub, err := tx.GetSubmission(ctx, submissionID)
		if err != nil {
			return err
		}
		if contestID != uuid.Nil && (sub.ContestID == nil || *sub.ContestID != contestID) {
			return pkg.Wrap(pkg.ErrBadInput, nil, "submission does not belong to specified contest")
		}
		if sub.ContestID != nil && sub.CreatedBy != nil && sub.ProblemID != nil {
			if err := tx.LockSubmissionTuple(ctx, *sub.ContestID, *sub.CreatedBy, *sub.ProblemID); err != nil {
				return err
			}
		}
		if sub.ContestID != nil {
			if err := requireContest(ctx, tx, *sub.ContestID, accessmodel.ActionManageContest); err != nil {
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
		event := sub.CompletedEvent(previous)
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

func newOutboxEventParams(submission submissionsmodel.Submission, job judgingmodel.Job, previousState *submissionsmodel.State) eventdata.Event {
	submissionCreatedEvent := submission.CreatedEvent(job.ID, job.Generation, previousState)

	return eventdata.Event{AggregateID: submission.ID, Type: eventdata.OutboxEventSubmissionCreated, Payload: submissionCreatedEvent}
}

func (uc *SubmissionsUseCase) ListSubmissions(ctx context.Context, filter submissionsmodel.SubmissionsFilter) (*submissionsmodel.SubmissionsList, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}
	ownOnly := filter.UserId != nil && *filter.UserId == user.Id
	if filter.ContestId != nil {
		permissions, err := uc.permissions.GetContestPermissions(ctx, *filter.ContestId, user.Id)
		if err != nil {
			return nil, err
		}
		canList := permissions.ListUsersSubmissions
		if ownOnly {
			canList = permissions.ListOwnSubmissions
		}
		if !canList {
			return nil, pkg.NoPermission
		}
		contest, err := uc.contestsUC.GetContest(ctx, *filter.ContestId)
		if err != nil {
			return nil, err
		}
		if filter.State != nil && contest.IsFrozenAt(accessmodel.ReadTime(ctx)) && !permissions.ManageContest && !ownOnly {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "state filter is unavailable during freeze")
		}
	} else if !ownOnly && !user.IsAdmin() {
		return nil, pkg.NoPermission
	}

	submissions, total, err := uc.submissionsRepo.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i, sub := range submissions {
		visible, err := uc.submissionView(ctx, sub, false)
		if err != nil {
			return nil, err
		}
		submissions[i] = visible
	}

	return &submissionsmodel.SubmissionsList{
		Submissions: submissions, TotalCount: total,
		Pagination: paging.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *SubmissionsUseCase) RejudgeSubmissions(ctx context.Context, filter submissionsmodel.RejudgeFilter) (int, error) {
	if filter.ContestID == uuid.Nil {
		return 0, pkg.Wrap(pkg.ErrBadInput, nil, "contest id is required")
	}

	if err := uc.requireContest(ctx, filter.ContestID, accessmodel.ActionManageContest); err != nil {
		return 0, err
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
		if err := requireContest(ctx, tx, filter.ContestID, accessmodel.ActionManageContest); err != nil {
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
