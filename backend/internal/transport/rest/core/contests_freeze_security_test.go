package core_test

import (
	"context"
	"testing"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockContestsUC struct {
	mock.Mock
	ContestsUC
}

func (m *MockContestsUC) GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(contestsmodel.Contest), args.Error(1) //nolint:wrapcheck
}

func (m *MockContestsUC) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error) {
	args := m.Called(ctx, orgLogin, contestLogin)
	return args.Get(0).(contestsmodel.Contest), args.Error(1) //nolint:wrapcheck
}

func (m *MockContestsUC) GetContestScoreboard(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	args := m.Called(ctx, contestID, userID, unfrozen)
	if res := args.Get(0); res != nil {
		return res.(*contestsmodel.ScoreboardResponse), args.Error(1) //nolint:wrapcheck
	}
	return nil, args.Error(1) //nolint:wrapcheck
}

type MockPermissionsUC struct {
	mock.Mock
	PermissionsUC
}

func (m *MockPermissionsUC) HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action accessmodel.ContestAction) (bool, error) {
	args := m.Called(ctx, contestID, userID, action)
	return args.Bool(0), args.Error(1) //nolint:wrapcheck
}

func TestGetContestScoreboard_SecurityAndPermissions(t *testing.T) {
	t.Parallel()

	contestID := uuid.New()
	ownerID := uuid.New()
	participantID := uuid.New()
	moderatorID := uuid.New()
	adminID := uuid.New()

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)
	freezeTime := endTime.Add(-30 * time.Minute)

	baseContest := contestsmodel.Contest{
		ID:                contestID,
		Login:             "test-contest",
		OrganizationLogin: "test-org",
		OwnerID:           &ownerID,
		StartTime:         &startTime,
		EndTime:           &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           contestsmodel.FreezeStatusAuto,
			"freeze_duration_minutes": 30,
		},
	}

	t.Run("Participant Requesting unfrozen=true Is Rejected (403 Forbidden)", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		user := accessmodel.User{
			Id:   participantID,
			Role: accessmodel.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "test-contest").Return(baseContest, nil)
		// Participant has permission to view monitor
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, accessmodel.ActionGetMonitor).Return(true, nil)
		// But DOES NOT have permission to manage contest (cannot view unfrozen scoreboard)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, accessmodel.ActionManageContest).Return(false, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardParams{
			OrgLogin:     "test-org",
			ContestLogin: "test-contest",
			Unfrozen:     corev1.NewOptBool(true),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		require.ErrorIs(t, err, pkg.NoPermission)
		assert.Equal(t, 403, pkg.ToREST(err))
		mockContests.AssertNotCalled(t, "GetContestScoreboard", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Participant Requesting Default unfrozen=false Receives Frozen Scoreboard", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		user := accessmodel.User{
			Id:   participantID,
			Role: accessmodel.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "test-contest").Return(baseContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, accessmodel.ActionGetMonitor).Return(true, nil)

		expectedScoreboard := &contestsmodel.ScoreboardResponse{
			ContestID:         contestID,
			ContestLogin:      "test-contest",
			OrganizationLogin: "test-org",
			IsFrozen:          true,
			FreezeTime:        &freezeTime,
			Problems:          []contestsmodel.ScoreboardProblemHeader{},
			Items:             []contestsmodel.ScoreboardItem{},
		}
		mockContests.On("GetContestScoreboard", mock.Anything, contestID, participantID, false).Return(expectedScoreboard, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardParams{
			OrgLogin:     "test-org",
			ContestLogin: "test-contest",
			Unfrozen:     corev1.NewOptBool(false),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsFrozen)
		assert.True(t, resp.FreezeTime.IsSet())
		assert.Equal(t, freezeTime.Unix(), resp.FreezeTime.Value.Unix())
	})

	t.Run("Moderator Requesting unfrozen=true Receives Live Scoreboard with is_frozen=true", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		user := accessmodel.User{
			Id:   moderatorID,
			Role: accessmodel.UserRoleUser, // User role in system, but moderator in contest
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "test-contest").Return(baseContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, moderatorID, accessmodel.ActionGetMonitor).Return(true, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, moderatorID, accessmodel.ActionManageContest).Return(true, nil)

		expectedScoreboard := &contestsmodel.ScoreboardResponse{
			ContestID:         contestID,
			ContestLogin:      "test-contest",
			OrganizationLogin: "test-org",
			IsFrozen:          true,
			FreezeTime:        &freezeTime,
			Problems:          []contestsmodel.ScoreboardProblemHeader{},
			Items:             []contestsmodel.ScoreboardItem{},
		}
		mockContests.On("GetContestScoreboard", mock.Anything, contestID, moderatorID, true).Return(expectedScoreboard, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardParams{
			OrgLogin:     "test-org",
			ContestLogin: "test-contest",
			Unfrozen:     corev1.NewOptBool(true),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsFrozen) // Preserves is_frozen: true
		assert.True(t, resp.FreezeTime.IsSet())
		assert.Equal(t, freezeTime.Unix(), resp.FreezeTime.Value.Unix())
	})

	t.Run("Admin Requesting unfrozen=true Receives Live Scoreboard", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		user := accessmodel.User{
			Id:   adminID,
			Role: accessmodel.UserRoleAdmin,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "test-contest").Return(baseContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, adminID, accessmodel.ActionGetMonitor).Return(true, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, adminID, accessmodel.ActionManageContest).Return(true, nil)

		expectedScoreboard := &contestsmodel.ScoreboardResponse{
			ContestID:         contestID,
			ContestLogin:      "test-contest",
			OrganizationLogin: "test-org",
			IsFrozen:          true,
			FreezeTime:        &freezeTime,
			Problems:          []contestsmodel.ScoreboardProblemHeader{},
			Items:             []contestsmodel.ScoreboardItem{},
		}
		mockContests.On("GetContestScoreboard", mock.Anything, contestID, adminID, true).Return(expectedScoreboard, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardParams{
			OrgLogin:     "test-org",
			ContestLogin: "test-contest",
			Unfrozen:     corev1.NewOptBool(true),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Contest Not Started - Non-Manager Blocked (403)", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		futureStart := time.Now().Add(1 * time.Hour)
		futureContest := contestsmodel.Contest{
			ID:                contestID,
			Login:             "test-contest",
			OrganizationLogin: "test-org",
			OwnerID:           &ownerID,
			StartTime:         &futureStart,
			Settings:          map[string]interface{}{},
		}

		user := accessmodel.User{
			Id:   participantID,
			Role: accessmodel.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "test-contest").Return(futureContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, accessmodel.ActionGetMonitor).Return(true, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardParams{
			OrgLogin:     "test-org",
			ContestLogin: "test-contest",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		require.ErrorIs(t, err, pkg.NoPermission)
		assert.Equal(t, 403, pkg.ToREST(err))
	})

	t.Run("Contest Not Started - Contest Owner Allowed", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		futureStart := time.Now().Add(1 * time.Hour)
		futureContest := contestsmodel.Contest{
			ID:                contestID,
			Login:             "test-contest",
			OrganizationLogin: "test-org",
			OwnerID:           &ownerID,
			StartTime:         &futureStart,
			Settings:          map[string]interface{}{},
		}

		user := accessmodel.User{
			Id:   ownerID,
			Role: accessmodel.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "test-contest").Return(futureContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, ownerID, accessmodel.ActionGetMonitor).Return(true, nil)

		mockContests.On("GetContestScoreboard", mock.Anything, contestID, ownerID, false).
			Return(&contestsmodel.ScoreboardResponse{ContestID: contestID, ContestLogin: "test-contest", OrganizationLogin: "test-org"}, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardParams{
			OrgLogin:     "test-org",
			ContestLogin: "test-contest",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("User Lacking ActionGetMonitor Blocked (403)", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		user := accessmodel.User{
			Id:   participantID,
			Role: accessmodel.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "test-contest").Return(baseContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, accessmodel.ActionGetMonitor).Return(false, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardParams{
			OrgLogin:     "test-org",
			ContestLogin: "test-contest",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		require.ErrorIs(t, err, pkg.NoPermission)
		assert.Equal(t, 403, pkg.ToREST(err))
	})
}
