package core_test

import (
	"context"
	"testing"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockProblemsUC struct {
	mock.Mock
	ProblemsUC
}

func (m *MockProblemsUC) GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(problemsmodel.Problem), args.Error(1) //nolint:wrapcheck
}

func (m *MockContestsUC) GetContestProblem(ctx context.Context, input contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(contestsmodel.ContestProblem), args.Error(1) //nolint:wrapcheck
}

func (m *MockContestsUC) GetContestProblems(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.ContestProblem, error) {
	args := m.Called(ctx, contestID)
	return args.Get(0).([]contestsmodel.ContestProblem), args.Error(1) //nolint:wrapcheck
}

type stubBookletCompiler struct{}

func (stubBookletCompiler) CompilePDF(ctx context.Context, texSource string) ([]byte, error) {
	return []byte("%PDF-1.4 mock PDF content"), nil
}

func TestHideStatementsAndPDFBooklet(t *testing.T) {
	t.Parallel()

	contestID := uuid.New()
	problemID := uuid.New()
	participantID := uuid.New()
	managerID := uuid.New()

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)

	hiddenContest := contestsmodel.Contest{
		ID:                contestID,
		Login:             "onsite-contest",
		OrganizationLogin: "test-org",
		StartTime:         &startTime,
		EndTime:           &endTime,
		Settings: map[string]interface{}{
			"hide_statements": true,
		},
	}

	contestProblem := contestsmodel.ContestProblem{
		ContestID: contestID,
		ProblemID: problemID,
		Ordinal:   1,
		Title:     "A + B",
	}

	problem := problemsmodel.Problem{
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
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, mockProblems, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, stubBookletCompiler{},
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
		ctx := accessmodel.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "onsite-contest").Return(hiddenContest, nil)
		mockContests.On("GetContestProblem", mock.Anything, contestsmodel.ContestProblemGet{
			ContestId: contestID,
			ProblemId: problemID,
		}).Return(contestProblem, nil)
		mockProblems.On("GetProblemById", mock.Anything, problemID).Return(problem, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, accessmodel.ActionManageContest).Return(false, nil)

		resp, err := server.GetContestProblem(ctx, corev1.GetContestProblemParams{
			OrgLogin:     "test-org",
			ContestLogin: "onsite-contest",
			ProblemID:    problemID,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)

		assert.Equal(t, "A + B", resp.Problem.Title)
		assert.Equal(t, int32(1000), resp.Problem.TimeLimit)
		assert.Equal(t, int32(256), resp.Problem.MemoryLimit)
		assert.Empty(t, resp.Problem.LegendHTML)
		assert.Empty(t, resp.Problem.InputFormatHTML)
		assert.Empty(t, resp.Problem.OutputFormatHTML)
		assert.Empty(t, resp.Problem.Samples)
	})

	t.Run("Participant cannot download PDF when hide_statements=true", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		mockPerms := new(MockPermissionsUC)
		mockProblems := new(MockProblemsUC)
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, mockProblems, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, stubBookletCompiler{},
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
		ctx := accessmodel.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "onsite-contest").Return(hiddenContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, participantID, accessmodel.ActionManageContest).Return(false, nil)

		_, err := server.DownloadContestStatementsPdf(ctx, corev1.DownloadContestStatementsPdfParams{
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
		server := core.NewCoreServer(nil, mockContests, mockPerms, nil, nil, mockProblems, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, stubBookletCompiler{},
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
			Id:   managerID,
			Role: accessmodel.UserRoleUser,
		}
		ctx := accessmodel.WithUser(context.Background(), user)

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "test-org", "onsite-contest").Return(hiddenContest, nil)
		mockPerms.On("HasContestPermission", mock.Anything, contestID, managerID, accessmodel.ActionManageContest).Return(true, nil)
		mockContests.On("GetContestProblems", mock.Anything, contestID).Return([]contestsmodel.ContestProblem{contestProblem}, nil)
		mockProblems.On("GetProblemById", mock.Anything, problemID).Return(problem, nil)

		resp, err := server.DownloadContestStatementsPdf(ctx, corev1.DownloadContestStatementsPdfParams{
			OrgLogin:     "test-org",
			ContestLogin: "onsite-contest",
		})
		require.NoError(t, err)
		assert.NotNil(t, resp.Data)
	})
}
