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

func TestAuthorizationUUIDParameters(t *testing.T) {
	id := uuid.New()
	otherID := uuid.New()
	for _, tc := range []struct {
		name   string
		params ogenMiddleware.Parameters
		read   func(ogenMiddleware.Request) (uuid.UUID, error)
		wantID uuid.UUID
	}{
		{name: "problem path", params: ogenMiddleware.Parameters{{Name: "problemId", In: "path"}: id, {Name: "id", In: "path"}: otherID}, read: func(req ogenMiddleware.Request) (uuid.UUID, error) { return pathUUID(req, "problemId", "id") }, wantID: id},
		{name: "problem id path", params: ogenMiddleware.Parameters{{Name: "id", In: "path"}: id}, read: func(req ogenMiddleware.Request) (uuid.UUID, error) { return pathUUID(req, "problemId", "id") }, wantID: id},
		{name: "submission path", params: ogenMiddleware.Parameters{{Name: "submission_id", In: "path"}: id}, read: func(req ogenMiddleware.Request) (uuid.UUID, error) { return pathUUID(req, "submission_id") }, wantID: id},
		{name: "query cannot replace path", params: ogenMiddleware.Parameters{{Name: "user_id", In: "query"}: corev1.NewOptUUID(id)}, read: func(req ogenMiddleware.Request) (uuid.UUID, error) { return pathUUID(req, "user_id") }},
		{name: "contest query", params: ogenMiddleware.Parameters{{Name: "contestId", In: "query"}: corev1.NewOptUUID(id)}, read: func(req ogenMiddleware.Request) (uuid.UUID, error) { return queryUUID(req, "contestId") }, wantID: id},
		{name: "user query", params: ogenMiddleware.Parameters{{Name: "userId", In: "query"}: corev1.NewOptUUID(id)}, read: func(req ogenMiddleware.Request) (uuid.UUID, error) { return queryUUID(req, "userId") }, wantID: id},
		{name: "unset query", params: ogenMiddleware.Parameters{{Name: "userId", In: "query"}: corev1.OptUUID{}}, read: func(req ogenMiddleware.Request) (uuid.UUID, error) { return queryUUID(req, "userId") }},
		{name: "string path rejected", params: ogenMiddleware.Parameters{{Name: "id", In: "path"}: id.String()}, read: func(req ogenMiddleware.Request) (uuid.UUID, error) { return pathUUID(req, "id") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.read(ogenMiddleware.Request{Params: tc.params})
			if tc.wantID == uuid.Nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantID, got)
		})
	}
}

func TestContestAuthorizationLogins(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  ogenMiddleware.Request
	}{
		{name: "contest path", req: ogenMiddleware.Request{OperationID: "GetContest", Params: ogenMiddleware.Parameters{
			{Name: "org_login", In: "path"}:     "org",
			{Name: "contest_login", In: "path"}: "contest",
		}}},
		{name: "submission query", req: ogenMiddleware.Request{OperationID: "CreateSubmission", Params: ogenMiddleware.Parameters{
			{Name: "organization_login", In: "query"}: "org",
			{Name: "contest_login", In: "query"}:      "contest",
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orgLogin, contestLogin, err := contestLogins(tc.req)
			require.NoError(t, err)
			require.Equal(t, "org", orgLogin)
			require.Equal(t, "contest", contestLogin)
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
