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

type pendingInvitationStore struct{ *mockOwnerGrantStore }

func (*pendingInvitationStore) GetPendingInvitation(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationInvitation, error) {
	return &organizationsmodel.OrganizationInvitation{}, nil
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
			uc, store, actors := setupOwnerGrantFixture()
			owner, target := actors["owner"], actors["target"]
			if existingMember {
				store.members[target.Id] = organizationsmodel.OrgRoleMember
			} else {
				uc.transactor = invitationTransactions{store: &pendingInvitationStore{store}}
			}
			invitation, err := uc.InviteMember(accessmodel.WithUser(context.Background(), owner), store.org.Login, target.Id, organizationsmodel.OrgRoleMember, owner.Id)
			require.ErrorIs(t, err, pkg.ErrConflict)
			require.Equal(t, http.StatusConflict, pkg.ToREST(err))
			require.Nil(t, invitation)
			require.Empty(t, store.invitations)
		})
	}
}
