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
		if operationID == "NewError" {
			continue
		}
		if _, ok := endpointPolicies[operationID]; !ok {
			t.Errorf("missing authz policy for operation %q", operationID)
		}
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
		"AddOrganizationMember", "ListOrganizationInvitations", "ResolveOrganizationJoinRequest",
		"InviteContestParticipant", "UpdateContestMember", "DeleteContestMember",
		"CreateContestTeam", "UpdateContestTeam", "DeleteContestTeam", "ListContestJoinRequests",
		"UpdateProblem", "DeleteProblem", "ImportProblem", "BuildProblemPackage", "SaveProblemDraft",
		"DownloadProblemPackage", "CreateProblemMember",
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
