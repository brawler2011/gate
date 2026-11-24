package contests_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"

	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"

	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	contests "github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
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

type noticeLog struct{ notices []eventdata.Notice }

func (l *noticeLog) Notify(_ context.Context, n eventdata.Notice) error {
	l.notices = append(l.notices, n)
	return nil
}

type blockingTransactor struct{ submissions.SubmissionTxStore }

func (t blockingTransactor) Within(_ context.Context, fn func(submissions.SubmissionTxStore) error) error {
	return fn(t.SubmissionTxStore)
}

// moderatorAccess lets alice moderate the fixture contest half an hour after it starts.
func moderatorAccess(f *scoreboardFixture) *accesstest.Loader {
	contest := f.repo.contest
	l := &accesstest.Loader{Contests: map[uuid.UUID]access.ContestFacts{contest.ID: {Contest: contest, Now: contest.StartTime.Add(30 * time.Minute)}}}
	l.Grant(contest.ID, f.users["alice"], string(accessmodel.ContestModerator))
	return l
}

func TestBlockSubmissionRemovesSolveAndUnblockQueuesRejudge(t *testing.T) {
	f := newScoreboardFixture("alice")
	id := f.submit(t, "alice", 0, submissionsmodel.Accepted, 10*time.Minute)
	other := f.submit(t, "alice", 1, submissionsmodel.Accepted, 20*time.Minute)
	repo, notices, loader := &blockingSubmissionsRepo{repo: f.repo}, &noticeLog{}, moderatorAccess(f)
	uc := submissions.NewSubmissionsUseCase(repo, blockingTransactor{struct {
		*blockingSubmissionsRepo
		eventdata.Writer
		*scoreboardRepo
		*accesstest.Loader
	}{repo, notices, f.repo, loader}}, loader)
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: f.users["alice"], Role: accessmodel.UserRoleUser})
	reason := "Cheating"
	require.NoError(t, uc.SetSubmissionBlocked(ctx, id, true, &reason))
	sub, err := uc.GetSubmission(ctx, id)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Disqualified, sub.State)
	require.Equal(t, &reason, sub.BanReason)
	sb := f.scoreboard(t, false)
	require.EqualValues(t, 1, sb.Items[0].ProblemsSolved)
	require.EqualValues(t, 20, sb.Items[0].TotalPenalty)
	require.NoError(t, uc.SetSubmissionBlocked(ctx, id, false, nil))
	sub, err = uc.GetSubmission(ctx, id)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Saved, sub.State)
	require.Nil(t, sub.BanReason)
	require.Equal(t, []eventdata.Notice{{Type: submissionsmodel.SubmissionEventCompleted, ID: id}, {Type: submissionsmodel.SubmissionEventCreated, ID: id}}, notices.notices)
	untouched, err := uc.GetSubmission(ctx, other)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Accepted, untouched.State)
}

func TestProblemBlockDisqualifiesExistingAndFutureSubmissions(t *testing.T) {
	f := newScoreboardFixture("alice", "bob")
	alice := f.submit(t, "alice", 0, submissionsmodel.Accepted, 10*time.Minute)
	bob := f.submit(t, "bob", 0, submissionsmodel.Accepted, 20*time.Minute)
	other := f.submit(t, "alice", 1, submissionsmodel.Accepted, 30*time.Minute)
	repo, notices, loader := &blockingSubmissionsRepo{repo: f.repo}, &noticeLog{}, moderatorAccess(f)
	f.uc = contests.NewContestsUseCase(f.repo, nil, contestCommandTransactions{struct {
		*scoreboardRepo
		*blockingSubmissionsRepo
		eventdata.Writer
		*accesstest.Loader
	}{f.repo, repo, notices, loader}}, loader)
	uc := submissions.NewSubmissionsUseCase(repo, blockingTransactor{struct {
		*blockingSubmissionsRepo
		eventdata.Writer
		*scoreboardRepo
		*accesstest.Loader
	}{repo, notices, f.repo, loader}}, loader)
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: f.users["alice"], Role: accessmodel.UserRoleUser})
	reason := "Banned from problem A"
	contest, user, problem := f.repo.contest.ID, f.users["alice"], f.repo.problems[0].ProblemID
	require.NoError(t, f.uc.BlockProblemForUser(ctx, contest, user, problem, &reason))
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
	creation := &submissionsmodel.SubmissionCreation{ContestId: contest, UserId: user, ProblemId: problem, Solution: "int main(){}", Language: "cpp17"}
	blockedID, err := uc.CreateSubmission(ctx, creation)
	require.NoError(t, err)
	blocked, err := uc.GetSubmission(ctx, blockedID)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Disqualified, blocked.State)
	require.Equal(t, &reason, blocked.BanReason)
	require.Equal(t, []eventdata.Notice{{Type: submissionsmodel.SubmissionEventCompleted, ID: alice}, {Type: submissionsmodel.SubmissionEventCompleted, ID: blockedID}}, notices.notices)
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
	require.Equal(t, eventdata.Notice{Type: submissionsmodel.SubmissionEventCreated, ID: allowedID}, notices.notices[2])
}

func (r *blockingSubmissionsRepo) ScheduleJob(_ context.Context, id uuid.UUID) (judgingmodel.Job, error) {
	return judgingmodel.Job{ID: uuid.New(), SubmissionID: id, Revision: 1}, nil
}

func (*blockingSubmissionsRepo) LockSubmissionTuple(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}

type contestCommandTransactions struct{ store contests.CommandTxStore }

func (t contestCommandTransactions) Within(_ context.Context, fn func(contests.CommandTxStore) error) error {
	return fn(t.store)
}

func (*blockingSubmissionsRepo) GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error) {
	return nil, pkg.ErrNotFound
}

func (*blockingSubmissionsRepo) CreateNotification(context.Context, *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error) {
	return nil, errors.New("not implemented")
}

func (*blockingSubmissionsRepo) LockContestProblemOrder(context.Context, uuid.UUID) error { return nil }

func (*blockingSubmissionsRepo) ListContestOwners(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, errors.New("not implemented")
}

func (*blockingSubmissionsRepo) GetContestTeam(context.Context, uuid.UUID, uuid.UUID) (contestsmodel.ContestTeam, error) {
	return contestsmodel.ContestTeam{}, errors.New("not implemented")
}

func (*blockingSubmissionsRepo) ResolveContestProblemPackage(context.Context, uuid.UUID, uuid.UUID, bool) (contestsmodel.PackageRef, error) {
	return contestsmodel.PackageRef{}, errors.New("not implemented")
}
