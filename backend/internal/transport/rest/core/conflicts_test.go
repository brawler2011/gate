package core

import (
	"context"
	"net/http"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type conflictOrganizations struct{ OrganizationsUC }

func (conflictOrganizations) GetOrganizationByLogin(context.Context, string) (*organizationsmodel.Organization, error) {
	return &organizationsmodel.Organization{ID: uuid.New()}, nil
}

type conflictContests struct{ ContestsUC }

func (conflictContests) GetContestByOrgLoginAndContestLogin(_ context.Context, _, login string) (contestsmodel.Contest, error) {
	return contestsmodel.Contest{ID: uuid.New(), Login: login}, nil
}

func TestDuplicateContestLogin(t *testing.T) {
	server := &CoreServer{Services: Services{Organizations: conflictOrganizations{}, Contests: conflictContests{}}}
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
	t.Run("create", func(t *testing.T) {
		response, err := server.CreateContest(ctx, corev1.CreateContestParams{OrgLogin: "org", Title: "Contest", Login: corev1.NewOptString("taken")})
		require.ErrorIs(t, err, pkg.ErrConflict)
		require.Equal(t, http.StatusConflict, pkg.ToREST(err))
		require.Nil(t, response)
	})
	t.Run("rename", func(t *testing.T) {
		err := server.UpdateContest(ctx, &corev1.UpdateContestRequestModel{Login: corev1.NewOptString("taken")}, corev1.UpdateContestParams{OrgLogin: "org", ContestLogin: "original"})
		require.ErrorIs(t, err, pkg.ErrConflict)
		require.Equal(t, http.StatusConflict, pkg.ToREST(err))
	})
}
