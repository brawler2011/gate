package contests

import (
	"context"
	"errors"
	"testing"

	"github.com/brawler2011/gate/backend/internal/access"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type problemCommandStore struct {
	CommandTxStore
	manage        bool
	permissionErr error
	writes        int
}

func (*problemCommandStore) LockContestProblemOrder(context.Context, uuid.UUID) error { return nil }
func (s *problemCommandStore) HasContestPermission(_ context.Context, _, _ uuid.UUID, action accessmodel.ContestAction) (bool, error) {
	if action != accessmodel.ActionManageContest {
		panic("unexpected contest action")
	}
	return s.manage, s.permissionErr
}
func (s *problemCommandStore) CreateContestProblem(context.Context, contestsmodel.ContestProblemCreation) error {
	s.writes++
	return nil
}
func (s *problemCommandStore) UpdateContestProblemPackage(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	s.writes++
	return nil
}

type problemCommandPermissions struct {
	Permissions
	manage                  bool
	contestErr              error
	t                       *testing.T
	actor, contest, problem uuid.UUID
	allowed                 bool
	err                     error
	calls                   int
}

func (p *problemCommandPermissions) HasContestPermission(_ context.Context, contest, user uuid.UUID, action accessmodel.ContestAction) (bool, error) {
	require.Equal(p.t, p.actor, user)
	require.Equal(p.t, p.contest, contest)
	require.Equal(p.t, accessmodel.ActionManageContest, action)
	return p.manage, p.contestErr
}

func (p *problemCommandPermissions) HasProblemPermission(_ context.Context, problem, user uuid.UUID, action accessmodel.ProblemAction) (bool, error) {
	p.calls++
	require.Equal(p.t, p.actor, user)
	require.Equal(p.t, p.problem, problem)
	require.Equal(p.t, accessmodel.ActionUseProblem, action)
	return p.allowed, p.err
}

func TestContestProblemCommandsRequireSourceAccess(t *testing.T) {
	failure := errors.New("authorization unavailable")
	for _, update := range []bool{false, true} {
		command := "create"
		if update {
			command = "update"
		}
		t.Run(command, func(t *testing.T) {
			for _, tc := range []struct {
				name                         string
				manage, use, guest, txDenied bool
				contestErr, sourceErr, want  error
			}{
				{name: "allowed", manage: true, use: true},
				{name: "destination denied", use: true, want: pkg.NoPermission},
				{name: "source denied", manage: true, want: pkg.NoPermission},
				{name: "destination error", manage: true, use: true, contestErr: failure, want: failure},
				{name: "source error", manage: true, use: true, sourceErr: failure, want: failure},
				{name: "guest", guest: true, manage: true, use: true, want: pkg.ErrUnauthenticated},
			} {
				t.Run(tc.name, func(t *testing.T) {
					actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
					if tc.guest {
						actor = accessmodel.Guest
					}
					contest, problem, pack := uuid.New(), uuid.New(), uuid.New()
					store := &problemCommandStore{manage: tc.manage && !tc.txDenied, permissionErr: tc.contestErr}
					permissions := &problemCommandPermissions{t: t, actor: actor.Id, contest: contest, problem: problem, manage: tc.manage, contestErr: tc.contestErr, allowed: tc.use, err: tc.sourceErr}
					uc := NewContestsUseCase(&problemPackageReader{}, nil, testTransactions[CommandTxStore]{store}, permissions)
					ctx := accessmodel.WithUser(context.Background(), actor)
					var err error
					if update {
						err = uc.UpdateContestProblemPackage(ctx, contest, problem, pack)
					} else {
						err = uc.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: contest, ProblemId: problem, PackageId: pack})
					}
					if tc.want != nil {
						require.ErrorIs(t, err, tc.want)
						require.Zero(t, store.writes)
					} else {
						require.NoError(t, err)
						require.Equal(t, 1, store.writes)
					}
					if tc.guest || !tc.manage || tc.contestErr != nil {
						require.Zero(t, permissions.calls)
					} else {
						require.Equal(t, 1, permissions.calls)
					}
				})
			}
		})
	}
}

type problemPackageReader struct{ ContestsRepo }

func (*problemPackageReader) ResolveContestProblemPackage(_ context.Context, problem, pack uuid.UUID, _ bool) (access.PackageContext, error) {
	return access.PackageContext{ID: pack, ProblemID: problem, Status: "ready"}, nil
}
