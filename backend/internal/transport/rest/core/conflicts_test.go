package core

import (
	"context"
	"net/http"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type takenLogins struct {
	ContestsUC
	taken map[string]bool
	tried []string
}

func (c *takenLogins) CreateContest(_ context.Context, input *contestsmodel.CreateContestInput) (uuid.UUID, error) {
	c.tried = append(c.tried, input.Login)
	if c.taken[input.Login] {
		return uuid.Nil, pkg.Wrap(pkg.ErrContestLoginTaken, nil, "contest login is taken")
	}
	return uuid.New(), nil
}

func TestCreateContestLogin(t *testing.T) {
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
	t.Run("generated login skips taken ones", func(t *testing.T) {
		contests := &takenLogins{taken: map[string]bool{"spring-cup": true, "spring-cup-2": true}}
		server := &CoreServer{Services: Services{Contests: contests}}
		response, err := server.CreateContest(ctx, &corev1.CreateContestRequestModel{OrganizationID: uuid.New(), Title: "Spring Cup"})
		require.NoError(t, err)
		require.Equal(t, "spring-cup-3", response.Login.Value)
		require.Equal(t, []string{"spring-cup", "spring-cup-2", "spring-cup-3"}, contests.tried)
	})
	t.Run("explicit login is not replaced", func(t *testing.T) {
		contests := &takenLogins{taken: map[string]bool{"taken": true}}
		server := &CoreServer{Services: Services{Contests: contests}}
		response, err := server.CreateContest(ctx, &corev1.CreateContestRequestModel{OrganizationID: uuid.New(), Title: "Contest", Login: corev1.NewOptString("taken")})
		require.ErrorIs(t, err, pkg.ErrConflict)
		require.Equal(t, http.StatusConflict, pkg.HTTPStatus(pkg.CodeOf(err)))
		require.Nil(t, response)
		require.Equal(t, []string{"taken"}, contests.tried)
	})
}
