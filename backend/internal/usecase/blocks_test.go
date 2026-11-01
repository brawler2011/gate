package usecase_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func (r *scoreboardRepo) CreateContestUserProblemBlock(_ context.Context, p *models.CreateContestUserProblemBlockParams) error {
	r.blocks[scoreboardKey{p.ContestID, p.UserID, p.ProblemID}] = models.ContestUserProblemBlock{ContestID: p.ContestID, UserID: p.UserID, ProblemID: p.ProblemID, Reason: p.Reason, CreatedBy: p.CreatedBy}
	return nil
}
func (r *scoreboardRepo) DeleteContestUserProblemBlock(_ context.Context, contest, user, problem uuid.UUID) error {
	delete(r.blocks, scoreboardKey{contest, user, problem})
	return nil
}
func (r *scoreboardRepo) GetContestUserProblemBlock(_ context.Context, contest, user, problem uuid.UUID) (*models.ContestUserProblemBlock, error) {
	block, ok := r.blocks[scoreboardKey{contest, user, problem}]
	if !ok {
		return nil, pkg.ErrNotFound
	}
	return &block, nil
}

type blockingSubmissionsRepo struct {
	interfaces.SubmissionsRepo
	repo *scoreboardRepo
}

func (r *blockingSubmissionsRepo) WithTx(pgx.Tx) interfaces.SubmissionsRepo { return r }
func (r *blockingSubmissionsRepo) GetSubmission(_ context.Context, id uuid.UUID) (models.Submission, error) {
	sub, ok := r.repo.submissions[id]
	if !ok {
		return sub, pkg.ErrNotFound
	}
	return sub, nil
}
func (r *blockingSubmissionsRepo) CreateSubmission(_ context.Context, p *models.SubmissionCreation) (uuid.UUID, error) {
	id := uuid.New()
	state := models.Saved
	if p.State != nil {
		state = *p.State
	}
	r.repo.submissions[id] = models.Submission{ID: id, ContestID: &p.ContestId, CreatedBy: &p.UserId, ProblemID: &p.ProblemId, Submission: p.Solution, State: state, BanReason: p.BanReason, CreatedAt: r.repo.contest.StartTime.Add(time.Minute)}
	return id, nil
}
func (r *blockingSubmissionsRepo) BlockSubmission(_ context.Context, id uuid.UUID, reason *string) error {
	sub, ok := r.repo.submissions[id]
	if !ok {
		return pkg.ErrNotFound
	}
	sub.State = models.Disqualified
	sub.BanReason = reason
	r.repo.submissions[id] = sub
	return nil
}
func (r *blockingSubmissionsRepo) BlockUserProblemSubmissions(ctx context.Context, contest, user, problem uuid.UUID, reason *string) error {
	for id, sub := range r.repo.submissions {
		if sub.ContestID != nil && *sub.ContestID == contest && sub.CreatedBy != nil && *sub.CreatedBy == user && sub.ProblemID != nil && *sub.ProblemID == problem {
			if err := r.BlockSubmission(ctx, id, reason); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *blockingSubmissionsRepo) ResetSubmissionsState(_ context.Context, filter models.RejudgeFilter) ([]models.SubmissionReset, error) {
	var ids []models.SubmissionReset
	for id, sub := range r.repo.submissions {
		if sub.ContestID == nil || *sub.ContestID != filter.ContestID || filter.SubmissionID != nil && id != *filter.SubmissionID || filter.UserID != nil && (sub.CreatedBy == nil || *sub.CreatedBy != *filter.UserID) || filter.ProblemID != nil && (sub.ProblemID == nil || *sub.ProblemID != *filter.ProblemID) {
			continue
		}
		previous := sub.State
		sub.State = models.Saved
		sub.BanReason = nil
		r.repo.submissions[id] = sub
		ids = append(ids, models.SubmissionReset{ID: id, PreviousState: previous})
	}
	return ids, nil
}

type blockingOutbox struct {
	interfaces.OutboxRepo
	events []models.CreateOutboxEventParams
}

func (r *blockingOutbox) WithTx(pgx.Tx) interfaces.OutboxRepo { return r }
func (r *blockingOutbox) CreateEvent(_ context.Context, p *models.CreateOutboxEventParams) error {
	r.events = append(r.events, *p)
	return nil
}

type blockingTransactor struct{}

func (blockingTransactor) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	return fn(ctx, nil)
}

type blockingProblems struct{ interfaces.ProblemsUC }

func (blockingProblems) GetProblemById(context.Context, uuid.UUID) (models.Problem, error) {
	return models.Problem{}, nil
}

func TestBlockSubmissionRemovesSolveAndUnblockQueuesRejudge(t *testing.T) {
	f := newScoreboardFixture("alice")
	id := f.submit(t, "alice", 0, models.Accepted, 10*time.Minute)
	other := f.submit(t, "alice", 1, models.Accepted, 20*time.Minute)
	repo, outbox := &blockingSubmissionsRepo{repo: f.repo}, &blockingOutbox{}
	uc := usecase.NewSubmissionsUseCase(repo, f.uc, blockingProblems{}, outbox, blockingTransactor{})
	ctx := context.Background()
	reason := "Cheating"
	require.NoError(t, uc.BlockSubmission(ctx, f.repo.contest.ID, id, &reason))
	sub, err := uc.GetSubmission(ctx, id)
	require.NoError(t, err)
	require.Equal(t, models.Disqualified, sub.State)
	require.Equal(t, &reason, sub.BanReason)
	sb := f.scoreboard(t, false)
	require.EqualValues(t, 1, sb.Items[0].ProblemsSolved)
	require.EqualValues(t, 20, sb.Items[0].TotalPenalty)
	require.NoError(t, uc.UnblockSubmission(ctx, f.repo.contest.ID, id))
	sub, err = uc.GetSubmission(ctx, id)
	require.NoError(t, err)
	require.Equal(t, models.Saved, sub.State)
	require.Nil(t, sub.BanReason)
	require.Len(t, outbox.events, 1)
	var event models.SubmissionCreatedEvent
	require.NoError(t, json.Unmarshal(outbox.events[0].Payload, &event))
	require.Equal(t, id, event.Id)
	require.Equal(t, models.Saved, event.State)
	untouched, err := uc.GetSubmission(ctx, other)
	require.NoError(t, err)
	require.Equal(t, models.Accepted, untouched.State)
}

func TestProblemBlockDisqualifiesExistingAndFutureSubmissions(t *testing.T) {
	f := newScoreboardFixture("alice", "bob")
	alice := f.submit(t, "alice", 0, models.Accepted, 10*time.Minute)
	bob := f.submit(t, "bob", 0, models.Accepted, 20*time.Minute)
	other := f.submit(t, "alice", 1, models.Accepted, 30*time.Minute)
	repo, outbox := &blockingSubmissionsRepo{repo: f.repo}, &blockingOutbox{}
	f.uc = usecase.NewContestsUseCase(f.repo, nil, repo)
	uc := usecase.NewSubmissionsUseCase(repo, f.uc, blockingProblems{}, outbox, blockingTransactor{})
	ctx := context.Background()
	reason := "Banned from problem A"
	contest, user, problem := f.repo.contest.ID, f.users["alice"], f.repo.problems[0].ProblemID
	require.NoError(t, f.uc.BlockProblemForUser(ctx, contest, user, problem, &reason, uuid.New()))
	require.Equal(t, models.Disqualified, f.repo.submissions[alice].State)
	require.Equal(t, models.Accepted, f.repo.submissions[bob].State)
	require.Equal(t, models.Accepted, f.repo.submissions[other].State)
	sb := f.scoreboard(t, false)
	for _, item := range sb.Items {
		if item.Username == "alice" {
			require.EqualValues(t, 1, item.ProblemsSolved)
			require.EqualValues(t, 30, item.TotalPenalty)
		}
	}
	creation := &models.SubmissionCreation{ContestId: contest, UserId: user, ProblemId: problem, Solution: "int main(){}", Language: models.Cpp}
	blockedID, err := uc.CreateSubmission(ctx, creation)
	require.NoError(t, err)
	blocked, err := uc.GetSubmission(ctx, blockedID)
	require.NoError(t, err)
	require.Equal(t, models.Disqualified, blocked.State)
	require.Equal(t, &reason, blocked.BanReason)
	require.Empty(t, outbox.events)
	require.NoError(t, f.uc.UnblockProblemForUser(ctx, contest, user, problem, false))
	block, err := f.uc.GetProblemBlockStatusForUser(ctx, contest, user, problem)
	require.NoError(t, err)
	require.Nil(t, block)
	creation.State, creation.BanReason = nil, nil
	allowedID, err := uc.CreateSubmission(ctx, creation)
	require.NoError(t, err)
	allowed, err := uc.GetSubmission(ctx, allowedID)
	require.NoError(t, err)
	require.Equal(t, models.Saved, allowed.State)
	require.Len(t, outbox.events, 1)
	require.Equal(t, allowedID, outbox.events[0].AggregateID)
}
