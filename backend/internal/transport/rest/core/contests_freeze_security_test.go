package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
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
	interfaces.ContestsUC
}

func (m *MockContestsUC) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Contest), args.Error(1) //nolint:wrapcheck
}

func (m *MockContestsUC) GetContestScoreboard(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*models.ScoreboardResponse, error) {
	args := m.Called(ctx, contestID, userID, unfrozen)
	if res := args.Get(0); res != nil {
		return res.(*models.ScoreboardResponse), args.Error(1) //nolint:wrapcheck
	}
	return nil, args.Error(1) //nolint:wrapcheck
}

type MockPermissionsUC struct {
	mock.Mock
	interfaces.PermissionsUC
}

func (m *MockPermissionsUC) HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action models.ContestAction) (bool, error) {
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

	baseContest := models.Contest{
		ID:        contestID,
		OwnerID:   &ownerID,
		StartTime: &startTime,
		EndTime:   &endTime,
		Settings: map[string]interface{}{
			"freeze_status":           models.FreezeStatusAuto,
			"freeze_duration_minutes": 30,
		},
	}

	boolTrue := true
	boolFalse := false

	t.Run("Participant Requesting unfrozen=true Is Rejected (403 Forbidden)", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		user := models.User{
			Id:   participantID,
			Role: models.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContest", mock.Anything, contestID).Return(baseContest, nil)
		// Participant has permission to view monitor
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, models.ActionGetMonitor).Return(true, nil)
		// But DOES NOT have permission to manage contest (cannot view unfrozen scoreboard)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, models.ActionManageContest).Return(false, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardRequestObject{
			ContestId: contestID,
			Params: corev1.GetContestScoreboardParams{
				Unfrozen: &boolTrue,
			},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, errors.Is(err, pkg.NoPermission))
		assert.Equal(t, 403, pkg.ToREST(err))
		mockContests.AssertNotCalled(t, "GetContestScoreboard", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Participant Requesting Default unfrozen=false Receives Frozen Scoreboard", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		user := models.User{
			Id:   participantID,
			Role: models.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContest", mock.Anything, contestID).Return(baseContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, models.ActionGetMonitor).Return(true, nil)

		expectedScoreboard := &models.ScoreboardResponse{
			ContestID:  contestID,
			IsFrozen:   true,
			FreezeTime: &freezeTime,
			Problems:   []models.ScoreboardProblemHeader{},
			Items:      []models.ScoreboardItem{},
		}
		mockContests.On("GetContestScoreboard", mock.Anything, contestID, participantID, false).Return(expectedScoreboard, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardRequestObject{
			ContestId: contestID,
			Params: corev1.GetContestScoreboardParams{
				Unfrozen: &boolFalse,
			},
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		jsonResp, ok := resp.(corev1.GetContestScoreboard200JSONResponse)
		require.True(t, ok)
		assert.True(t, jsonResp.IsFrozen)
		assert.Equal(t, &freezeTime, jsonResp.FreezeTime)
	})

	t.Run("Moderator Requesting unfrozen=true Receives Live Scoreboard with is_frozen=true", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		user := models.User{
			Id:   moderatorID,
			Role: models.UserRoleUser, // User role in system, but moderator in contest
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContest", mock.Anything, contestID).Return(baseContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, moderatorID, models.ActionGetMonitor).Return(true, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, moderatorID, models.ActionManageContest).Return(true, nil)

		expectedScoreboard := &models.ScoreboardResponse{
			ContestID:  contestID,
			IsFrozen:   true,
			FreezeTime: &freezeTime,
			Problems:   []models.ScoreboardProblemHeader{},
			Items:      []models.ScoreboardItem{},
		}
		mockContests.On("GetContestScoreboard", mock.Anything, contestID, moderatorID, true).Return(expectedScoreboard, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardRequestObject{
			ContestId: contestID,
			Params: corev1.GetContestScoreboardParams{
				Unfrozen: &boolTrue,
			},
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		jsonResp, ok := resp.(corev1.GetContestScoreboard200JSONResponse)
		require.True(t, ok)
		assert.True(t, jsonResp.IsFrozen) // Preserves is_frozen: true
		assert.Equal(t, &freezeTime, jsonResp.FreezeTime)
	})

	t.Run("Admin Requesting unfrozen=true Receives Live Scoreboard", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		user := models.User{
			Id:   adminID,
			Role: models.UserRoleAdmin,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContest", mock.Anything, contestID).Return(baseContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, adminID, models.ActionGetMonitor).Return(true, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, adminID, models.ActionManageContest).Return(true, nil)

		expectedScoreboard := &models.ScoreboardResponse{
			ContestID:  contestID,
			IsFrozen:   true,
			FreezeTime: &freezeTime,
			Problems:   []models.ScoreboardProblemHeader{},
			Items:      []models.ScoreboardItem{},
		}
		mockContests.On("GetContestScoreboard", mock.Anything, contestID, adminID, true).Return(expectedScoreboard, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardRequestObject{
			ContestId: contestID,
			Params: corev1.GetContestScoreboardParams{
				Unfrozen: &boolTrue,
			},
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Contest Not Started - Non-Manager Blocked (403)", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		futureStart := time.Now().Add(1 * time.Hour)
		futureContest := models.Contest{
			ID:        contestID,
			OwnerID:   &ownerID,
			StartTime: &futureStart,
			Settings:  map[string]interface{}{},
		}

		user := models.User{
			Id:   participantID,
			Role: models.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContest", mock.Anything, contestID).Return(futureContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, models.ActionGetMonitor).Return(true, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardRequestObject{
			ContestId: contestID,
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, errors.Is(err, pkg.NoPermission))
		assert.Equal(t, 403, pkg.ToREST(err))
	})

	t.Run("Contest Not Started - Contest Owner Allowed", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		futureStart := time.Now().Add(1 * time.Hour)
		futureContest := models.Contest{
			ID:        contestID,
			OwnerID:   &ownerID,
			StartTime: &futureStart,
			Settings:  map[string]interface{}{},
		}

		user := models.User{
			Id:   ownerID,
			Role: models.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContest", mock.Anything, contestID).Return(futureContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, ownerID, models.ActionGetMonitor).Return(true, nil)

		mockContests.On("GetContestScoreboard", mock.Anything, contestID, ownerID, false).
			Return(&models.ScoreboardResponse{ContestID: contestID}, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardRequestObject{
			ContestId: contestID,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("User Lacking ActionGetMonitor Blocked (403)", func(t *testing.T) {
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		user := models.User{
			Id:   participantID,
			Role: models.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContest", mock.Anything, contestID).Return(baseContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, models.ActionGetMonitor).Return(false, nil)

		resp, err := server.GetContestScoreboard(ctx, corev1.GetContestScoreboardRequestObject{
			ContestId: contestID,
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, errors.Is(err, pkg.NoPermission))
		assert.Equal(t, 403, pkg.ToREST(err))
	})
}
