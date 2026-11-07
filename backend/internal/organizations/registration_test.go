package organizations

import (
	"context"
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
	org    organizationsmodel.Organization
	member bool
	added  bool
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
	}{
		{name: "open", policy: organizationsmodel.OrgJoinPolicyOpen, joined: true},
		{name: "policy changed", policy: organizationsmodel.OrgJoinPolicyInviteOnly},
		{name: "concurrent membership", policy: organizationsmodel.OrgJoinPolicyOpen, member: true},
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
				require.ErrorIs(t, err, pkg.ErrBadInput)
			}
			require.Equal(t, tc.joined, joined)
			require.Equal(t, tc.joined, store.added)
		})
	}
}
