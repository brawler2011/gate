package contests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	contests "github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissions "github.com/brawler2011/gate/backend/internal/submissions"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (r *scoreboardRepo) CreateContestUserProblemBlock(_ context.Context, p *contestsmodel.CreateContestUserProblemBlockParams) error {
	r.blocks[scoreboardKey{p.ContestID, p.UserID, p.ProblemID}] = contestsmodel.ContestUserProblemBlock{ContestID: p.ContestID, UserID: p.UserID, ProblemID: p.ProblemID, Reason: p.Reason, CreatedBy: p.CreatedBy}
	return nil
}

func (r *scoreboardRepo) DeleteContestUserProblemBlock(_ context.Context, contest, user, problem uuid.UUID) error {
	delete(r.blocks, scoreboardKey{contest, user, problem})
	return nil
}

func (r *scoreboardRepo) GetContestUserProblemBlock(_ context.Context, contest, user, problem uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error) {
	block, ok := r.blocks[scoreboardKey{contest, user, problem}]
	if !ok {
		return nil, pkg.ErrNotFound
	}
	return &block, nil
}

type blockingSubmissionsRepo struct {
	SubmissionsRepo
	repo *scoreboardRepo
}

func (r *blockingSubmissionsRepo) GetSubmission(_ context.Context, id uuid.UUID) (submissionsmodel.Submission, error) {
	sub, ok := r.repo.submissions[id]
	if !ok {
		return sub, pkg.ErrNotFound
	}
	return sub, nil
}

func (r *blockingSubmissionsRepo) CreateSubmission(_ context.Context, p *submissionsmodel.SubmissionCreation) (uuid.UUID, error) {
	id := uuid.New()
	state := submissionsmodel.Saved
	if p.State != nil {
		state = *p.State
	}
	r.repo.submissions[id] = submissionsmodel.Submission{ID: id, ContestID: &p.ContestId, CreatedBy: &p.UserId, ProblemID: &p.ProblemId, Submission: p.Solution, State: state, BanReason: p.BanReason, CreatedAt: r.repo.contest.StartTime.Add(time.Minute)}
	return id, nil
}

func (r *blockingSubmissionsRepo) BlockSubmission(_ context.Context, id uuid.UUID, reason *string) error {
	sub, ok := r.repo.submissions[id]
	if !ok {
		return pkg.ErrNotFound
	}
	sub.State = submissionsmodel.Disqualified
	sub.BanReason = reason
	r.repo.submissions[id] = sub
	return nil
}

func (r *blockingSubmissionsRepo) BlockUserProblemSubmissions(ctx context.Context, contest, user, problem uuid.UUID, reason *string) ([]submissionsmodel.SubmissionReset, error) {
	var changed []submissionsmodel.SubmissionReset
	for id, sub := range r.repo.submissions {
		if sub.ContestID != nil && *sub.ContestID == contest && sub.CreatedBy != nil && *sub.CreatedBy == user && sub.ProblemID != nil && *sub.ProblemID == problem {
			changed = append(changed, submissionsmodel.SubmissionReset{ID: id, PreviousState: sub.State})
			if err := r.BlockSubmission(ctx, id, reason); err != nil {
				return nil, err
			}
		}
	}
	return changed, nil
}

func (r *blockingSubmissionsRepo) ResetSubmissionsState(_ context.Context, filter submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error) {
	var ids []submissionsmodel.SubmissionReset
	for id, sub := range r.repo.submissions {
		if sub.ContestID == nil || *sub.ContestID != filter.ContestID || filter.SubmissionID != nil && id != *filter.SubmissionID || filter.UserID != nil && (sub.CreatedBy == nil || *sub.CreatedBy != *filter.UserID) || filter.ProblemID != nil && (sub.ProblemID == nil || *sub.ProblemID != *filter.ProblemID) {
			continue
		}
		previous := sub.State
		sub.State = submissionsmodel.Saved
		sub.BanReason = nil
		r.repo.submissions[id] = sub
		ids = append(ids, submissionsmodel.SubmissionReset{ID: id, PreviousState: previous})
	}
	return ids, nil
}

type blockingOutbox struct {
	OutboxRepo
	events []eventdata.CreateOutboxEventParams
}

func (r *blockingOutbox) CreateEvent(_ context.Context, p *eventdata.CreateOutboxEventParams) error {
	r.events = append(r.events, *p)
	return nil
}

type blockingTransactor struct{ submissions.SubmissionTxStore }

func (t blockingTransactor) Within(_ context.Context, fn func(submissions.SubmissionTxStore) error) error {
	return fn(t.SubmissionTxStore)
}

type blockingProblems struct{ ProblemsUC }

func (blockingProblems) GetProblemById(context.Context, uuid.UUID) (problemsmodel.Problem, error) {
	return problemsmodel.Problem{}, nil
}

func TestBlockSubmissionRemovesSolveAndUnblockQueuesRejudge(t *testing.T) {
	f := newScoreboardFixture("alice")
	id := f.submit(t, "alice", 0, submissionsmodel.Accepted, 10*time.Minute)
	other := f.submit(t, "alice", 1, submissionsmodel.Accepted, 20*time.Minute)
	repo, outbox := &blockingSubmissionsRepo{repo: f.repo}, &blockingOutbox{}
	uc := submissions.NewSubmissionsUseCase(repo, f.uc, blockingProblems{}, blockingTransactor{struct {
		*blockingSubmissionsRepo
		OutboxRepo
		*scoreboardRepo
	}{repo, outbox, f.repo}}, allowSubmissionPermissions{})
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: f.users["alice"], Role: accessmodel.UserRoleUser})
	reason := "Cheating"
	require.NoError(t, uc.BlockSubmission(ctx, f.repo.contest.ID, id, &reason))
	sub, err := uc.GetSubmission(ctx, id)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Disqualified, sub.State)
	require.Equal(t, &reason, sub.BanReason)
	sb := f.scoreboard(t, false)
	require.EqualValues(t, 1, sb.Items[0].ProblemsSolved)
	require.EqualValues(t, 20, sb.Items[0].TotalPenalty)
	require.NoError(t, uc.UnblockSubmission(ctx, f.repo.contest.ID, id))
	sub, err = uc.GetSubmission(ctx, id)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Saved, sub.State)
	require.Nil(t, sub.BanReason)
	require.Len(t, outbox.events, 2)
	var event submissionsmodel.SubmissionCreatedEvent
	require.NoError(t, json.Unmarshal(outbox.events[1].Payload, &event))
	require.Equal(t, id, event.Id)
	require.Equal(t, submissionsmodel.Saved, event.State)
	untouched, err := uc.GetSubmission(ctx, other)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Accepted, untouched.State)
}

func TestProblemBlockDisqualifiesExistingAndFutureSubmissions(t *testing.T) {
	f := newScoreboardFixture("alice", "bob")
	alice := f.submit(t, "alice", 0, submissionsmodel.Accepted, 10*time.Minute)
	bob := f.submit(t, "bob", 0, submissionsmodel.Accepted, 20*time.Minute)
	other := f.submit(t, "alice", 1, submissionsmodel.Accepted, 30*time.Minute)
	repo, outbox := &blockingSubmissionsRepo{repo: f.repo}, &blockingOutbox{}
	f.uc = contests.NewContestsUseCase(f.repo, nil, contestCommandTransactions{struct {
		*scoreboardRepo
		*blockingSubmissionsRepo
		OutboxRepo
	}{f.repo, repo, outbox}}, nil)
	uc := submissions.NewSubmissionsUseCase(repo, f.uc, blockingProblems{}, blockingTransactor{struct {
		*blockingSubmissionsRepo
		OutboxRepo
		*scoreboardRepo
	}{repo, outbox, f.repo}}, allowSubmissionPermissions{})
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: f.users["alice"], Role: accessmodel.UserRoleUser})
	reason := "Banned from problem A"
	contest, user, problem := f.repo.contest.ID, f.users["alice"], f.repo.problems[0].ProblemID
	require.NoError(t, f.uc.BlockProblemForUser(ctx, contest, user, problem, &reason, uuid.New()))
	require.Equal(t, submissionsmodel.Disqualified, f.repo.submissions[alice].State)
	require.Equal(t, submissionsmodel.Accepted, f.repo.submissions[bob].State)
	require.Equal(t, submissionsmodel.Accepted, f.repo.submissions[other].State)
	sb := f.scoreboard(t, false)
	for _, item := range sb.Items {
		if item.Username == "alice" {
			require.EqualValues(t, 1, item.ProblemsSolved)
			require.EqualValues(t, 30, item.TotalPenalty)
		}
	}
	creation := &submissionsmodel.SubmissionCreation{ContestId: contest, UserId: user, ProblemId: problem, Solution: "int main(){}", Language: submissionsmodel.Cpp}
	blockedID, err := uc.CreateSubmission(ctx, creation)
	require.NoError(t, err)
	blocked, err := uc.GetSubmission(ctx, blockedID)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Disqualified, blocked.State)
	require.Equal(t, &reason, blocked.BanReason)
	require.Len(t, outbox.events, 3)
	require.Equal(t, eventdata.OutboxEventSubmissionCompleted, outbox.events[0].EventType)
	require.Equal(t, eventdata.OutboxEventSubmissionCreated, outbox.events[1].EventType)
	require.Equal(t, blockedID, outbox.events[1].AggregateID)
	require.Equal(t, eventdata.OutboxEventSubmissionCompleted, outbox.events[2].EventType)
	require.Equal(t, blockedID, outbox.events[2].AggregateID)
	require.NoError(t, f.uc.UnblockProblemForUser(ctx, contest, user, problem, false))
	block, err := f.uc.GetProblemBlockStatusForUser(ctx, contest, user, problem)
	require.NoError(t, err)
	require.Nil(t, block)
	creation.State, creation.BanReason = nil, nil
	allowedID, err := uc.CreateSubmission(ctx, creation)
	require.NoError(t, err)
	allowed, err := uc.GetSubmission(ctx, allowedID)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Saved, allowed.State)
	require.Len(t, outbox.events, 4)
	require.Equal(t, allowedID, outbox.events[3].AggregateID)
}

func (r *blockingOutbox) Record(ctx context.Context, event eventdata.Event) error {
	data, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("encode test event: %w", err)
	}
	return r.CreateEvent(ctx, &eventdata.CreateOutboxEventParams{Id: uuid.New(), AggregateID: event.AggregateID, EventType: event.Type, Payload: data})
}

func (r *blockingSubmissionsRepo) ScheduleJob(_ context.Context, id uuid.UUID) (judgingmodel.Job, error) {
	return judgingmodel.Job{ID: uuid.New(), SubmissionID: id, Generation: 1}, nil
}

func (*blockingSubmissionsRepo) LockContest(context.Context, uuid.UUID) error { return nil }

type contestCommandTransactions struct{ store contests.CommandTxStore }

func (t contestCommandTransactions) Within(_ context.Context, fn func(contests.CommandTxStore) error) error {
	return fn(t.store)
}

type allowSubmissionPermissions struct{}

func (allowSubmissionPermissions) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error) {
	return true, nil
}
func (allowSubmissionPermissions) GetContestPermissions(context.Context, uuid.UUID, uuid.UUID) (*accessmodel.ContestPermissions, error) {
	return &accessmodel.ContestPermissions{GetOtherUserSubmission: true, GetSubmissionDetails: true, ManageContest: true}, nil
}
