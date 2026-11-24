package submissions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"

	"github.com/brawler2011/gate/backend/pkg/outputs"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

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
	outputs         OutputsStore
}

// OutputsStore keeps the outputs archives of output-only submissions.
type OutputsStore struct {
	Store interface {
		UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error
		DownloadFile(ctx context.Context, bucket, key string, ifNoneMatch *string) (io.ReadCloser, string, error)
	}
	Bucket string
}

func NewSubmissionsUseCase(submissionsRepo SubmissionsRepo, transactor SubmissionTransactions, loader access.Loader, outputs OutputsStore) *SubmissionsUseCase {
	return &SubmissionsUseCase{submissionsRepo: submissionsRepo, transactor: transactor, access: loader, outputs: outputs}
}

// OpenOutputs opens an output-only submission's outputs archive where its source would be visible.
func (uc *SubmissionsUseCase) OpenOutputs(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	sub, err := uc.GetSubmission(ctx, id)
	if err != nil {
		return nil, err
	}
	if sub.SourceHidden {
		return nil, pkg.ErrForbidden
	}
	if sub.Outputs == nil {
		return nil, pkg.ErrNotFound
	}
	r, _, err := uc.outputs.Store.DownloadFile(ctx, uc.outputs.Bucket, outputs.Key(*sub.Outputs), nil)
	return r, err //nolint:wrapcheck // storage errors carry their kind
}

// CreateOutputsSubmission submits outputs packed by outputs.Pack to an output-only problem.
func (uc *SubmissionsUseCase) CreateOutputsSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation, archive []byte, sha string) (uuid.UUID, error) {
	input := *creation
	input.Solution, input.Language, input.Outputs = "", submissionsmodel.OutputsLanguage, &sha
	return uc.create(ctx, &input, func(ctx context.Context) error {
		return uc.outputs.Store.UploadFile(ctx, uc.outputs.Bucket, outputs.Key(sha), bytes.NewReader(archive), "application/zip") //nolint:wrapcheck // storage errors carry their kind
	})
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
	if requireRead && !a.Submission(author, sub.Kind == submissionsmodel.KindVirtual).Open {
		return submissionsmodel.Submission{}, pkg.ErrForbidden
	}
	return SubmissionView(a, sub.CreatedBy, sub.CreatedAt, sub.Kind).Apply(sub), nil
}

func (uc *SubmissionsUseCase) CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error) {
	return uc.create(ctx, creation, nil)
}

// create runs store, if given, once the submission is allowed and before it is recorded.
func (uc *SubmissionsUseCase) create(ctx context.Context, creation *submissionsmodel.SubmissionCreation, store func(context.Context) error) (uuid.UUID, error) {
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

		problem, err := tx.GetContestProblem(ctx, contestsmodel.ContestProblemGet{ContestId: creation.ContestId, ProblemId: creation.ProblemId})
		if err != nil {
			return pkg.OnNotFound(err, pkg.ErrNotFound, "problem does not belong to contest")
		}
		if outputOnly := problem.Type == "output-only"; outputOnly != (creation.Outputs != nil) {
			if outputOnly {
				return pkg.Invalid(pkg.ErrInconsistent, "an output-only problem takes outputs, not a program", nil)
			}
			return pkg.Invalid(pkg.ErrInconsistent, "this problem takes a program, not outputs", nil)
		}
		if len(problem.Languages) > 0 && !slices.Contains(problem.Languages, creation.Language) {
			return pkg.Invalid(pkg.ErrLanguageNotAllowed, "the problem's graders do not compile this language", map[string]any{"id": creation.Language})
		}
		if store != nil {
			if err := store(ctx); err != nil {
				return err
			}
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

// SetSubmissionBlocked blocks a contest submission, or unblocks it by rejudging.
func (uc *SubmissionsUseCase) SetSubmissionBlocked(ctx context.Context, submissionID uuid.UUID, blocked bool, reason *string) error {
	if _, err := accessmodel.RequireUser(ctx); err != nil {
		return err
	}
	sub, err := uc.submissionsRepo.GetSubmission(ctx, submissionID)
	if err != nil {
		return err
	}
	if sub.ContestID == nil {
		return pkg.Wrap(pkg.ErrInvalid, nil, "only contest submissions can be blocked")
	}
	contestID := *sub.ContestID
	if !blocked {
		_, err := uc.RejudgeSubmissions(ctx, submissionsmodel.RejudgeFilter{ContestID: contestID, SubmissionID: &submissionID})
		return err
	}
	return access.WithinContest(ctx, uc.transactor, contestID, accessmodel.ContestManage, func(tx SubmissionTxStore, _ access.ContestAccess) error {
		if err := tx.BlockSubmission(ctx, submissionID, reason); err != nil {
			return err
		}
		return tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCompleted, ID: submissionID})
	})
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
		if filter.State != nil && !a.Can(accessmodel.ContestManage) {
			if !ownOnly && a.Facts.Contest.IsFrozenAt(a.Facts.Now) {
				return nil, pkg.Wrap(pkg.ErrScoreboardFrozen, nil, "state filter is unavailable during freeze")
			}
			author := uuid.New()
			if ownOnly {
				author = a.Actor.Id
			}
			if a.Submission(author, false).Results == contestsmodel.FeedbackHidden || a.Submission(author, true).Results == contestsmodel.FeedbackHidden {
				return nil, pkg.Wrap(pkg.ErrForbidden, nil, "state filter is unavailable while results are hidden")
			}
		}
		views.access = map[uuid.UUID]access.ContestAccess{*filter.ContestId: a}
	} else if !user.IsAdmin() && (!ownOnly || filter.State != nil) {
		// A state filter across contests would reveal results that a contest hides.
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
