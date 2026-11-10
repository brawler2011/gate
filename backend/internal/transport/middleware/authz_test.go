package middleware

import (
	"context"
	"reflect"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	ogenMiddleware "github.com/ogen-go/ogen/middleware"
	"github.com/stretchr/testify/require"
)

func TestStrictAuthzPoliciesCoverStrictOperations(t *testing.T) {
	handlerInterfaceType := reflect.TypeOf((*corev1.Handler)(nil)).Elem()

	for i := 0; i < handlerInterfaceType.NumMethod(); i++ {
		operationID := handlerInterfaceType.Method(i).Name
		evaluators, ok := endpointPolicies[operationID]
		if !ok || len(evaluators.evaluators) == 0 {
			t.Errorf("missing authz policy for operation %q", operationID)
		}
	}
}

func TestSelfOrAdminUsesUsernamePathParameter(t *testing.T) {
	user := accessmodel.User{Id: uuid.New(), Username: "owner", Role: accessmodel.UserRoleUser}
	for _, tc := range []struct {
		name    string
		params  ogenMiddleware.Parameters
		wantErr error
	}{
		{name: "owner", params: ogenMiddleware.Parameters{{Name: "username", In: "path"}: "OWNER"}},
		{name: "other user", params: ogenMiddleware.Parameters{{Name: "username", In: "path"}: "other"}, wantErr: pkg.NoPermission},
		{name: "query cannot replace path", params: ogenMiddleware.Parameters{{Name: "username", In: "query"}: "owner"}, wantErr: pkg.ErrBadInput},
		{name: "missing path", wantErr: pkg.ErrBadInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireSelfOrAdmin()(context.Background(), &EvalContext{
				User: user,
				Request: ogenMiddleware.Request{
					Params: tc.params,
					Body:   &corev1.UpdateUserRequestModel{},
				},
			})
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}
		})
	}
}

func TestStrictAuthzPoliciesContainOnlyStrictOperations(t *testing.T) {
	handlerInterfaceType := reflect.TypeOf((*corev1.Handler)(nil)).Elem()

	for operationID := range endpointPolicies {
		if _, ok := handlerInterfaceType.MethodByName(operationID); !ok {
			t.Errorf("unknown operation in authz policy map %q", operationID)
		}
	}
}

func TestResourceAuthorizationIsDelegatedToHandlers(t *testing.T) {
	for _, operation := range []string{
		"CreateContest", "GetOrganization", "UpdateOrganization", "DeleteOrganization",
		"AddOrganizationMember", "ListOrganizationInvitations", "ApproveOrganizationJoinRequest",
		"CreateContestMember", "UpdateContestMember", "DeleteContestMember",
		"CreateContestTeam", "UpdateContestTeam", "DeleteContestTeam", "ListContestJoinRequests",
		"UpdateProblem", "DeleteProblem", "ImportProblem", "BuildProblemPackage", "SaveProblemDraft",
		"DownloadProblemPackage", "CreateProblemMember", "CreateProblemTeam",
		"ListUserSubmissions",
	} {
		t.Run(operation, func(t *testing.T) {
			for _, user := range []accessmodel.User{accessmodel.Guest, {Id: uuid.New(), Role: accessmodel.UserRoleUser}} {
				ctx := accessmodel.WithUser(context.Background(), user)
				called := false
				_, err := AuthzMiddleware()(ogenMiddleware.Request{Context: ctx, OperationID: operation}, func(req ogenMiddleware.Request) (ogenMiddleware.Response, error) {
					called = true
					return ogenMiddleware.Response{}, nil
				})
				if user.IsGuest() {
					require.ErrorIs(t, err, pkg.ErrUnauthenticated)
					require.False(t, called)
				} else {
					require.NoError(t, err)
					require.True(t, called)
				}
			}
		})
	}
}
