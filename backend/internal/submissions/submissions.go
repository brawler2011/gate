package submissions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

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
	transactor      SubmissionTransactions
	access          access.Loader
}

func NewSubmissionsUseCase(submissionsRepo SubmissionsRepo, transactor SubmissionTransactions, loader access.Loader) *SubmissionsUseCase {
	return &SubmissionsUseCase{submissionsRepo: submissionsRepo, transactor: transactor, access: loader}
}

func (uc *SubmissionsUseCase) GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error) {
	if _, err := accessmodel.RequireUser(ctx); err != nil {
		return submissionsmodel.Submission{}, err
	}
	sub, err := uc.submissionsRepo.GetSubmission(ctx, id)
	if err != nil {
		return sub, err
	}
	views := contestViews{uc: uc}
	return views.apply(ctx, sub, true)
}

// contestViews loads each contest's access once while projecting a page of submissions.
type contestViews struct {
	uc     *SubmissionsUseCase
	access map[uuid.UUID]access.ContestAccess
}

func (v *contestViews) apply(ctx context.Context, sub submissionsmodel.Submission, requireRead bool) (submissionsmodel.Submission, error) {
	if sub.ContestID == nil {
		actor, err := access.Actor(ctx, v.uc.access)
		if err != nil {
			return submissionsmodel.Submission{}, err
		}
		if (sub.CreatedBy == nil || *sub.CreatedBy != actor.Id) && !actor.IsAdmin() {
			return submissionsmodel.Submission{}, pkg.ErrForbidden
		}
		return sub, nil
	}
	a, ok := v.access[*sub.ContestID]
	if !ok {
		var err error
		if a, err = access.Contest(ctx, v.uc.access, *sub.ContestID, accessmodel.ContestView); err != nil {
			return submissionsmodel.Submission{}, err
		}
		if v.access == nil {
			v.access = map[uuid.UUID]access.ContestAccess{}
		}
		v.access[*sub.ContestID] = a
	}
	var author uuid.UUID
	if sub.CreatedBy != nil {
		author = *sub.CreatedBy
	}
	if requireRead && !a.CanSubmission(author, accessmodel.SubmissionView) {
		return submissionsmodel.Submission{}, pkg.ErrForbidden
	}
	return SubmissionView(a, sub.CreatedBy, sub.CreatedAt).Apply(sub), nil
}

func (uc *SubmissionsUseCase) CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error) {
	var id uuid.UUID
	err := access.WithinContest(ctx, uc.transactor, creation.ContestId, accessmodel.ContestView, func(tx SubmissionTxStore, a access.ContestAccess) error {
		kind, err := a.Submit()
		if err != nil {
			return err
		}
		if creation.UserId != a.Actor.Id {
			return pkg.ErrForbidden
		}
		if err := tx.LockSubmissionTuple(ctx, creation.ContestId, creation.UserId, creation.ProblemId); err != nil {
			return err
		}

		if _, err := tx.GetContestProblem(ctx, contestsmodel.ContestProblemGet{ContestId: creation.ContestId, ProblemId: creation.ProblemId}); err != nil {
			return pkg.OnNotFound(err, pkg.ErrNotFound, "problem does not belong to contest")
		}
		input := *creation
		input.Kind = kind
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
			return tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCompleted, ID: id})
		}

		if _, err := tx.ScheduleJob(ctx, id); err != nil {
			return err
		}
		return tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCreated, ID: id})
	})
	if err != nil {
		return uuid.Nil, err
	}

	slog.InfoContext(ctx, "submission_accepted", "event", "submission_accepted", "submission_id", id, "request_id", requestid.Get(ctx))
	return id, nil
}

func (uc *SubmissionsUseCase) BlockSubmission(ctx context.Context, contestID, submissionID uuid.UUID, reason *string) error {
	return access.WithinContest(ctx, uc.transactor, contestID, accessmodel.ContestManage, func(tx SubmissionTxStore, _ access.ContestAccess) error {
		sub, err := tx.GetSubmission(ctx, submissionID)
		if err != nil {
			return err
		}
		if sub.ContestID == nil || *sub.ContestID != contestID {
			return pkg.Wrap(pkg.ErrNotFound, nil, "submission does not belong to specified contest")
		}
		if sub.CreatedBy != nil && sub.ProblemID != nil {
			if err := tx.LockSubmissionTuple(ctx, contestID, *sub.CreatedBy, *sub.ProblemID); err != nil {
				return err
			}
		}
		sub, err = tx.GetSubmission(ctx, submissionID)
		if err != nil {
			return err
		}
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
		return tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCompleted, ID: sub.ID})
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

func (uc *SubmissionsUseCase) ListSubmissions(ctx context.Context, filter submissionsmodel.SubmissionsFilter) (*submissionsmodel.SubmissionsList, error) {
	user, err := accessmodel.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	ownOnly := filter.UserId != nil && *filter.UserId == user.Id
	views := contestViews{uc: uc}
	if filter.ContestId != nil {
		a, err := access.Contest(ctx, uc.access, *filter.ContestId, accessmodel.ContestView)
		if err != nil {
			return nil, err
		}
		action := accessmodel.ContestListSubmissions
		if ownOnly {
			action = accessmodel.ContestListOwnSubmissions
		}
		if !a.Can(action) {
			return nil, pkg.ErrForbidden
		}
		if filter.State != nil && a.Facts.Contest.IsFrozenAt(a.Facts.Now) && !a.Can(accessmodel.ContestManage) && !ownOnly {
			return nil, pkg.Wrap(pkg.ErrScoreboardFrozen, nil, "state filter is unavailable during freeze")
		}
		views.access = map[uuid.UUID]access.ContestAccess{*filter.ContestId: a}
	} else if !ownOnly && !user.IsAdmin() {
		return nil, pkg.ErrForbidden
	}

	submissions, total, err := uc.submissionsRepo.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i, sub := range submissions {
		visible, err := views.apply(ctx, sub, false)
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
		return 0, pkg.AtField("/contest_id", pkg.Invalid(pkg.ErrRequired, "contest id is required", nil))
	}

	var rejudgedCount int
	var rejudgedIDs []uuid.UUID

	err := access.WithinContest(ctx, uc.transactor, filter.ContestID, accessmodel.ContestManage, func(tx SubmissionTxStore, _ access.ContestAccess) error {
		if filter.SubmissionID != nil && *filter.SubmissionID != uuid.Nil {
			sub, err := tx.GetSubmission(ctx, *filter.SubmissionID)
			if err != nil {
				return err
			}
			if sub.ContestID == nil || *sub.ContestID != filter.ContestID {
				return pkg.Wrap(pkg.ErrNotFound, nil, "submission does not belong to specified contest")
			}
		}

		resetIDs, err := tx.ResetSubmissionsState(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to reset submissions state: %w", err)
		}

		for _, reset := range resetIDs {
			id := reset.ID
			submission, err := tx.GetSubmission(ctx, id)
			if err != nil {
				return fmt.Errorf("failed to get submission %s for rejudge: %w", id, err)
			}

			if err := contests.ApplySubmissionResult(ctx, tx, &submission); err != nil {
				return err
			}
			if _, err := tx.ScheduleJob(ctx, id); err != nil {
				return err
			}
			if err := tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCreated, ID: id}); err != nil {
				return fmt.Errorf("notify rejudge of submission %s: %w", id, err)
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
