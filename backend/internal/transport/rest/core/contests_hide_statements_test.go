package core_test

import (
	"context"
	"testing"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockProblemsUC struct {
	mock.Mock
	interfaces.ProblemsUC
}

func (m *MockProblemsUC) GetProblemById(ctx context.Context, id uuid.UUID) (models.Problem, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Problem), args.Error(1) //nolint:wrapcheck
}

func (m *MockContestsUC) GetContestProblem(ctx context.Context, input models.ContestProblemGet) (models.ContestProblem, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(models.ContestProblem), args.Error(1) //nolint:wrapcheck
}

func (m *MockContestsUC) GetContestProblems(ctx context.Context, contestID uuid.UUID) ([]models.ContestProblem, error) {
	args := m.Called(ctx, contestID)
	return args.Get(0).([]models.ContestProblem), args.Error(1) //nolint:wrapcheck
}

func TestHideStatementsAndPDFBooklet(t *testing.T) {
	t.Parallel()

	contestID := uuid.New()
	problemID := uuid.New()
	participantID := uuid.New()
	managerID := uuid.New()

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)

	hiddenContest := models.Contest{
		ID:                contestID,
		Login:             "onsite-contest",
		OrganizationLogin: "test-org",
		StartTime:         &startTime,
		EndTime:           &endTime,
		Settings: map[string]interface{}{
			"hide_statements": true,
		},
	}

	contestProblem := models.ContestProblem{
		ContestID: contestID,
		ProblemID: problemID,
		Ordinal:   1,
		Title:     "A + B",
	}

	problem := models.Problem{
		ID:            problemID,
		Title:         "A + B",
		TimeLimitMs:   1000,
		MemoryLimitMb: 256,
	}

	t.Run("Participant gets problem with stripped statement when hide_statements=true", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		mockProblems := new(MockProblemsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, mockProblems, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		user := models.User{
			Id:   participantID,
			Role: models.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "onsite-contest").Return(hiddenContest, nil)
		mockContests.On("GetContestProblem", mock.Anything, models.ContestProblemGet{
			ContestId: contestID,
			ProblemId: problemID,
		}).Return(contestProblem, nil)
		mockProblems.On("GetProblemById", mock.Anything, problemID).Return(problem, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, models.ActionManageContest).Return(false, nil)

		resp, err := server.GetContestProblem(ctx, corev1.GetContestProblemRequestObject{
			OrgLogin:     "test-org",
			ContestLogin: "onsite-contest",
			ProblemId:    problemID,
		})
		require.NoError(t, err)
		jsonResp, ok := resp.(corev1.GetContestProblem200JSONResponse)
		require.True(t, ok)

		assert.Equal(t, "A + B", jsonResp.Problem.Title)
		assert.Equal(t, int32(1000), jsonResp.Problem.TimeLimit)
		assert.Equal(t, int32(256), jsonResp.Problem.MemoryLimit)
		assert.Empty(t, jsonResp.Problem.LegendHtml)
		assert.Empty(t, jsonResp.Problem.InputFormatHtml)
		assert.Empty(t, jsonResp.Problem.OutputFormatHtml)
		assert.Empty(t, jsonResp.Problem.Samples)
	})

	t.Run("Participant cannot download PDF when hide_statements=true", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		mockProblems := new(MockProblemsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, mockProblems, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		user := models.User{
			Id:   participantID,
			Role: models.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "onsite-contest").Return(hiddenContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, models.ActionManageContest).Return(false, nil)

		_, err := server.DownloadContestStatementsPdf(ctx, corev1.DownloadContestStatementsPdfRequestObject{
			OrgLogin:     "test-org",
			ContestLogin: "onsite-contest",
		})
		require.Error(t, err)
	})

	t.Run("Manager CAN download PDF when hide_statements=true", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		mockProblems := new(MockProblemsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, mockProblems, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		user := models.User{
			Id:   managerID,
			Role: models.UserRoleUser,
		}
		ctx := middleware.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "onsite-contest").Return(hiddenContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, managerID, models.ActionManageContest).Return(true, nil)
		mockContests.On("GetContestProblems", mock.Anything, contestID).Return([]models.ContestProblem{contestProblem}, nil)
		mockProblems.On("GetProblemById", mock.Anything, problemID).Return(problem, nil)

		resp, err := server.DownloadContestStatementsPdf(ctx, corev1.DownloadContestStatementsPdfRequestObject{
			OrgLogin:     "test-org",
			ContestLogin: "onsite-contest",
		})
		require.NoError(t, err)
		pdfResp, ok := resp.(corev1.DownloadContestStatementsPdf200ApplicationpdfResponse)
		require.True(t, ok)
		assert.NotNil(t, pdfResp.Body)
		assert.Positive(t, pdfResp.ContentLength)
	})
}
