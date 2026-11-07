package core

import (
	"context"
	"net/http"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	paging "github.com/brawler2011/gate/backend/internal/page"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type packageHistoryErrorStub struct {
	ProblemsModule
	err error
}

func (s packageHistoryErrorStub) ListPackages(context.Context, uuid.UUID, paging.Request) (*paging.List[problemsmodel.ProblemPackage], error) {
	return nil, s.err
}

func TestListProblemPackagesPreservesReadErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{pkg.NoPermission, http.StatusForbidden},
		{pkg.ErrUnauthenticated, http.StatusUnauthorized},
		{pkg.ErrNotFound, http.StatusNotFound},
		{pkg.ErrBadInput, http.StatusBadRequest},
		{pkg.ErrInternal, http.StatusInternalServerError},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			server := &CoreServer{problems: packageHistoryErrorStub{err: tc.err}}
			_, err := server.ListProblemPackages(context.Background(), corev1.ListProblemPackagesParams{ID: uuid.New()})
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, tc.status, pkg.ToREST(err))
		})
	}
}
