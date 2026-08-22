package core_test

import (
	"context"
	"fmt"
	"testing"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (m *MockContestsUC) ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []models.ContestProblemReorderItem) error {
	args := m.Called(ctx, contestId, problems)
	if err := args.Error(0); err != nil {
		return fmt.Errorf("mock error: %w", err)
	}
	return nil
}

func TestReorderContestProblems(t *testing.T) {
	t.Parallel()

	contestID := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()

	baseContest := models.Contest{
		ID:                contestID,
		Login:             "round-1",
		OrganizationLogin: "org-1",
	}

	t.Run("Successfully reorders contest problems", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		server := core.NewCoreServer(nil, mockContests, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		ctx := context.Background()

		mockContests.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "org-1", "round-1").Return(baseContest, nil)
		mockContests.On("ReorderContestProblems", mock.Anything, contestID, []models.ContestProblemReorderItem{
			{ProblemID: p2, Position: 1},
			{ProblemID: p1, Position: 2},
		}).Return(nil)

		body := corev1.ReorderContestProblemsJSONRequestBody{
			Problems: []corev1.ContestProblemReorderItemModel{
				{ProblemId: p2, Position: 1},
				{ProblemId: p1, Position: 2},
			},
		}

		resp, err := server.ReorderContestProblems(ctx, corev1.ReorderContestProblemsRequestObject{
			OrgLogin:     "org-1",
			ContestLogin: "round-1",
			Body:         &body,
		})

		require.NoError(t, err)
		assert.IsType(t, corev1.ReorderContestProblems200Response{}, resp)
		mockContests.AssertExpectations(t)
	})

	t.Run("Returns error when request body is nil", func(t *testing.T) {
		t.Parallel()
		mockContests := new(MockContestsUC)
		server := core.NewCoreServer(nil, mockContests, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		ctx := context.Background()

		resp, err := server.ReorderContestProblems(ctx, corev1.ReorderContestProblemsRequestObject{
			OrgLogin:     "org-1",
			ContestLogin: "round-1",
			Body:         nil,
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})
}
