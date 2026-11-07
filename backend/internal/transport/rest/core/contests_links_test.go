package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/coremodules"
	"github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (m *MockContestsUC) CreateContestProblem(ctx context.Context, input contestsmodel.ContestProblemCreation) error {
	return m.Called(ctx, input).Error(0) //nolint:wrapcheck
}

func (m *MockContestsUC) CreateParticipant(ctx context.Context, input contestsmodel.ParticipantCreation) error {
	return m.Called(ctx, input).Error(0) //nolint:wrapcheck
}

type contestLinksSecurity struct{}

func (contestLinksSecurity) HandleCookieAuth(ctx context.Context, _ corev1.OperationName, _ corev1.CookieAuth) (context.Context, error) {
	return ctx, nil
}

func TestCreateContestLinksHTTP(t *testing.T) {
	t.Parallel()

	contestID, problemID, userID, packageID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, operation := range []struct {
		name   string
		path   string
		method string
		input  any
	}{
		{
			name:   "problem",
			path:   "/problems?problem_id=" + problemID.String(),
			method: "CreateContestProblem",
			input:  contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: problemID},
		},
		{
			name:   "problem with package",
			path:   "/problems?problem_id=" + problemID.String() + "&package_id=" + packageID.String(),
			method: "CreateContestProblem",
			input:  contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: problemID, PackageId: packageID},
		},
		{
			name:   "member",
			path:   "/members?user_id=" + userID.String(),
			method: "CreateParticipant",
			input:  contestsmodel.ParticipantCreation{ContestId: contestID, UserId: userID},
		},
	} {
		for _, result := range []struct {
			name       string
			lookupErr  error
			createErr  error
			statusCode int
		}{
			{name: "success", statusCode: http.StatusNoContent},
			{name: "contest missing", lookupErr: pkg.ErrNotFound, statusCode: http.StatusNotFound},
			{name: "creation conflict", createErr: pkg.ErrConflict, statusCode: http.StatusConflict},
		} {
			t.Run(operation.name+"/"+result.name, func(t *testing.T) {
				t.Parallel()
				uc := new(MockContestsUC)
				uc.On("GetContestByOrgLoginAndContestLogin", mock.Anything, "org", "round").
					Return(contestsmodel.Contest{ID: contestID}, result.lookupErr).Once()
				if result.lookupErr == nil {
					uc.On(operation.method, mock.Anything, operation.input).Return(result.createErr).Once()
				}
				handler := core.NewCoreServer(nil, nil, coremodules.Contests{ContestsUC: uc, ContestsReader: uc}, nil, nil, nil, nil, nil, false)
				server, err := corev1.NewServer(handler, contestLinksSecurity{}, corev1.WithErrorHandler(func(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
					w.WriteHeader(pkg.ToREST(err))
				}))
				require.NoError(t, err)

				req := httptest.NewRequest(http.MethodPost, "/organizations/org/contests/round"+operation.path, nil)
				req.AddCookie(&http.Cookie{Name: "session_id", Value: "test"})
				response := httptest.NewRecorder()
				server.ServeHTTP(response, req)

				require.Equal(t, result.statusCode, response.Code, response.Body.String())
				if result.statusCode == http.StatusNoContent {
					require.Empty(t, response.Body.Bytes())
					require.Empty(t, response.Header().Get("Content-Type"))
				}
				uc.AssertExpectations(t)
			})
		}
	}
}
