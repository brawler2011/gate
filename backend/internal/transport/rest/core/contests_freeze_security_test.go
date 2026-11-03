package core_test

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
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
