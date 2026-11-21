package organizations

import (
	"context"
	"net/http"
	"testing"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type invitationOrgs struct {
	OrganizationsRepo
	org organizationsmodel.Organization
}

func (r invitationOrgs) GetOrganizationByLogin(context.Context, string) (*organizationsmodel.Organization, error) {
	return &r.org, nil
}

// unimplementedOrganizationTx sits deeper than the loader so the loader's methods win.
type unimplementedOrganizationTx struct{ organizationTx }
type organizationTx struct{ OrganizationTxStore }

type invitationStore struct {
	unimplementedOrganizationTx
	accesstest.Loader
	member, pending bool
	invitations     int
}

func (s *invitationStore) GetUserById(_ context.Context, id uuid.UUID) (accessmodel.User, error) {
	return accessmodel.User{Id: id, Role: accessmodel.UserRoleUser}, nil
}
func (s *invitationStore) GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error) {
	if s.member {
		return &organizationsmodel.OrganizationMember{}, nil
	}
	return nil, pkg.ErrNotFound
}
func (s *invitationStore) GetPendingInvitation(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationInvitation, error) {
	if s.pending {
		return &organizationsmodel.OrganizationInvitation{}, nil
	}
	return nil, nil
}
func (s *invitationStore) CreateInvitation(context.Context, *organizationsmodel.CreateOrganizationInvitationInput) (*organizationsmodel.OrganizationInvitation, error) {
	s.invitations++
	return &organizationsmodel.OrganizationInvitation{}, nil
}
func (s *invitationStore) CreateNotification(context.Context, *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error) {
	return &notificationsmodel.Notification{}, nil
}

type invitationTransactions struct{ store OrganizationTxStore }

func (t invitationTransactions) Within(_ context.Context, fn func(OrganizationTxStore) error) error {
	return fn(t.store)
}

func TestInvitationRejectsDuplicates(t *testing.T) {
	for _, existingMember := range []bool{true, false} {
		name := "pending invitation"
		if existingMember {
			name = "existing member"
		}
		t.Run(name, func(t *testing.T) {
			org := organizationsmodel.Organization{ID: uuid.New(), Login: "org"}
			owner := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			store := &invitationStore{member: existingMember, pending: !existingMember}
			store.Organizations = map[uuid.UUID]access.OrganizationFacts{org.ID: {ID: org.ID, JoinPolicy: accessmodel.JoinByRequest}}
			store.Grant(org.ID, owner.Id, string(accessmodel.OrganizationOwner))
			uc := NewOrganizationsUseCase(invitationOrgs{org: org}, nil, store.Loader, invitationTransactions{store}, nil)
			invitation, err := uc.InviteMember(accessmodel.WithUser(context.Background(), owner), org.Login, uuid.New(), accessmodel.OrganizationMember)
			require.ErrorIs(t, err, pkg.ErrConflict)
			require.Equal(t, http.StatusConflict, pkg.HTTPStatus(pkg.CodeOf(err)))
			require.Nil(t, invitation)
			require.Zero(t, store.invitations)
		})
	}
}
