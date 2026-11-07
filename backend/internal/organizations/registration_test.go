package organizations

import (
	"context"
	"net/http"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type registrationReader struct {
	OrganizationsRepo
	org organizationsmodel.Organization
}

func (r registrationReader) GetOrganizationByLogin(context.Context, string) (*organizationsmodel.Organization, error) {
	return &r.org, nil
}

type registrationStore struct {
	OrganizationTxStore
	org     organizationsmodel.Organization
	member  bool
	pending bool
	added   bool
}

func (s *registrationStore) GetOrganizationByID(context.Context, uuid.UUID) (*organizationsmodel.Organization, error) {
	return &s.org, nil
}
func (s *registrationStore) GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error) {
	if s.member {
		return &organizationsmodel.OrganizationMember{}, nil
	}
	return nil, pkg.ErrNotFound
}
func (s *registrationStore) GetUserById(_ context.Context, id uuid.UUID) (accessmodel.User, error) {
	return accessmodel.User{Id: id, Role: accessmodel.UserRoleUser}, nil
}
func (s *registrationStore) AddMember(context.Context, uuid.UUID, uuid.UUID, organizationsmodel.OrganizationRole) error {
	s.added = true
	return nil
}

type registrationTransactions struct{ store *registrationStore }

func (t registrationTransactions) Within(ctx context.Context, fn func(OrganizationTxStore) error) error {
	return fn(t.store)
}

func TestRegistrationUsesCurrentOrganizationState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy organizationsmodel.OrganizationJoinPolicy
		member bool
		joined bool
		err    error
	}{
		{name: "open", policy: organizationsmodel.OrgJoinPolicyOpen, joined: true},
		{name: "policy changed", policy: organizationsmodel.OrgJoinPolicyInviteOnly, err: pkg.ErrBadInput},
		{name: "concurrent membership", policy: organizationsmodel.OrgJoinPolicyOpen, member: true, err: pkg.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			store := &registrationStore{org: organizationsmodel.Organization{ID: id, JoinPolicy: tc.policy}, member: tc.member}
			reader := registrationReader{org: organizationsmodel.Organization{ID: id, Login: "org", JoinPolicy: organizationsmodel.OrgJoinPolicyOpen}}
			uc := NewOrganizationsUseCase(reader, nil, nil, registrationTransactions{store}, nil)
			_, joined, err := uc.CreateJoinRequest(accessmodel.WithUser(context.Background(), user), "org", nil)
			if tc.joined {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.err)
			}
			require.Equal(t, tc.joined, joined)
			require.Equal(t, tc.joined, store.added)
		})
	}
}
func (*registrationStore) LockUser(context.Context, uuid.UUID) error { return nil }

func (s *registrationStore) GetPendingJoinRequest(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error) {
	if s.pending {
		return &organizationsmodel.OrganizationJoinRequest{}, nil
	}
	return nil, nil
}

func TestDuplicateOrganizationJoinRequest(t *testing.T) {
	org := organizationsmodel.Organization{ID: uuid.New(), Login: "org", JoinPolicy: organizationsmodel.OrgJoinPolicyByRequest}
	store := &registrationStore{org: org, pending: true}
	uc := NewOrganizationsUseCase(registrationReader{org: org}, nil, nil, registrationTransactions{store}, nil)
	caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	request, joined, err := uc.CreateJoinRequest(accessmodel.WithUser(context.Background(), caller), org.Login, nil)
	require.ErrorIs(t, err, pkg.ErrConflict)
	require.Equal(t, http.StatusConflict, pkg.ToREST(err))
	require.Nil(t, request)
	require.False(t, joined)
	require.False(t, store.added)
}
