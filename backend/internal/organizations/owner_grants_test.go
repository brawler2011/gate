package organizations

import (
	"context"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOwnerGrantStore struct {
	OrganizationTxStore
	org         organizationsmodel.Organization
	members     map[uuid.UUID]organizationsmodel.OrganizationRole
	users       map[uuid.UUID]accessmodel.User
	invitations map[uuid.UUID]*organizationsmodel.OrganizationInvitation
	requests    map[uuid.UUID]*organizationsmodel.OrganizationJoinRequest
	addedRoles  map[uuid.UUID]organizationsmodel.OrganizationRole
	updatedRole map[uuid.UUID]organizationsmodel.OrganizationRole
}

func newMockOwnerGrantStore(org organizationsmodel.Organization) *mockOwnerGrantStore {
	return &mockOwnerGrantStore{
		org:         org,
		members:     make(map[uuid.UUID]organizationsmodel.OrganizationRole),
		users:       make(map[uuid.UUID]accessmodel.User),
		invitations: make(map[uuid.UUID]*organizationsmodel.OrganizationInvitation),
		requests:    make(map[uuid.UUID]*organizationsmodel.OrganizationJoinRequest),
		addedRoles:  make(map[uuid.UUID]organizationsmodel.OrganizationRole),
		updatedRole: make(map[uuid.UUID]organizationsmodel.OrganizationRole),
	}
}

func (s *mockOwnerGrantStore) HasOrganizationPermission(_ context.Context, _ uuid.UUID, userID uuid.UUID, action accessmodel.OrgAction) (bool, error) {
	user, ok := s.users[userID]
	if ok && user.Role == accessmodel.UserRoleAdmin {
		return true, nil
	}
	role, ok := s.members[userID]
	if !ok {
		return false, nil
	}
	switch action {
	case accessmodel.ActionViewOrganization:
		return true, nil
	case accessmodel.ActionManageOrganization:
		return role == organizationsmodel.OrgRoleOwner || role == organizationsmodel.OrgRoleAdmin, nil
	case accessmodel.ActionDeleteOrganization:
		return role == organizationsmodel.OrgRoleOwner, nil
	default:
		return false, nil
	}
}

func (s *mockOwnerGrantStore) GetUserById(_ context.Context, id uuid.UUID) (accessmodel.User, error) {
	u, ok := s.users[id]
	if !ok {
		return accessmodel.User{}, pkg.ErrNotFound
	}
	return u, nil
}

func (s *mockOwnerGrantStore) GetMember(_ context.Context, _ uuid.UUID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error) {
	role, ok := s.members[userID]
	if !ok {
		return nil, pkg.ErrNotFound
	}
	return &organizationsmodel.OrganizationMember{
		OrganizationID: s.org.ID,
		UserID:         userID,
		Role:           role,
	}, nil
}

func (s *mockOwnerGrantStore) GetPendingInvitation(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*organizationsmodel.OrganizationInvitation, error) {
	return nil, nil
}

func (s *mockOwnerGrantStore) CreateInvitation(_ context.Context, input *organizationsmodel.CreateOrganizationInvitationInput) (*organizationsmodel.OrganizationInvitation, error) {
	inv := &organizationsmodel.OrganizationInvitation{
		ID:             uuid.New(),
		OrganizationID: input.OrganizationID,
		UserID:         input.UserID,
		InviterID:      input.InviterID,
		Role:           input.Role,
		Status:         organizationsmodel.RequestStatusPending,
	}
	s.invitations[inv.ID] = inv
	return inv, nil
}

func (s *mockOwnerGrantStore) CreateNotification(_ context.Context, _ *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error) {
	return &notificationsmodel.Notification{ID: uuid.New()}, nil
}

func (s *mockOwnerGrantStore) AddMember(_ context.Context, _ uuid.UUID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error {
	s.members[userID] = role
	s.addedRoles[userID] = role
	return nil
}

func (s *mockOwnerGrantStore) UpdateMemberRole(_ context.Context, _ uuid.UUID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error {
	s.members[userID] = role
	s.updatedRole[userID] = role
	return nil
}

func (s *mockOwnerGrantStore) GetJoinRequestByID(_ context.Context, id uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error) {
	req, ok := s.requests[id]
	if !ok {
		return nil, pkg.ErrNotFound
	}
	return req, nil
}

func (s *mockOwnerGrantStore) UpdateJoinRequestStatus(_ context.Context, id uuid.UUID, status organizationsmodel.RequestStatus, _ *uuid.UUID) error {
	if req, ok := s.requests[id]; ok {
		req.Status = status
	}
	return nil
}

type mockOwnerGrantTransactor struct {
	store *mockOwnerGrantStore
}

func (t mockOwnerGrantTransactor) Within(_ context.Context, fn func(OrganizationTxStore) error) error {
	return fn(t.store)
}

type mockOwnerGrantRepo struct {
	OrganizationsRepo
	org organizationsmodel.Organization
}

func (r mockOwnerGrantRepo) GetOrganizationByLogin(context.Context, string) (*organizationsmodel.Organization, error) {
	return &r.org, nil
}

func setupOwnerGrantFixture() (*OrganizationsUseCase, *mockOwnerGrantStore, map[string]accessmodel.User) {
	orgID := uuid.New()
	org := organizationsmodel.Organization{
		ID:    orgID,
		Login: "test-org",
		Name:  "Test Organization",
	}

	owner := accessmodel.User{Id: uuid.New(), Username: "org_owner", Role: accessmodel.UserRoleUser}
	admin := accessmodel.User{Id: uuid.New(), Username: "org_admin", Role: accessmodel.UserRoleUser}
	globalAdmin := accessmodel.User{Id: uuid.New(), Username: "global_admin", Role: accessmodel.UserRoleAdmin}
	targetUser := accessmodel.User{Id: uuid.New(), Username: "target_user", Role: accessmodel.UserRoleUser}

	store := newMockOwnerGrantStore(org)
	store.users[owner.Id] = owner
	store.users[admin.Id] = admin
	store.users[globalAdmin.Id] = globalAdmin
	store.users[targetUser.Id] = targetUser

	store.members[owner.Id] = organizationsmodel.OrgRoleOwner
	store.members[admin.Id] = organizationsmodel.OrgRoleAdmin

	repo := mockOwnerGrantRepo{org: org}
	transactor := mockOwnerGrantTransactor{store: store}
	uc := NewOrganizationsUseCase(repo, nil, nil, transactor, nil)

	actors := map[string]accessmodel.User{
		"owner":        owner,
		"admin":        admin,
		"global_admin": globalAdmin,
		"target":       targetUser,
	}

	return uc, store, actors
}

func TestDirectMembershipOwnerGrant(t *testing.T) {
	t.Run("admin cannot add owner", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		admin := actors["admin"]
		target := actors["target"]

		ctx := accessmodel.WithUser(context.Background(), admin)
		err := uc.AddMember(ctx, &organizationsmodel.AddOrganizationMemberInput{
			OrganizationID: store.org.ID,
			UserID:         target.Id,
			Role:           organizationsmodel.OrgRoleOwner,
		}, admin.Id)

		require.ErrorIs(t, err, pkg.NoPermission)
		assert.NotContains(t, store.addedRoles, target.Id)
	})

	t.Run("admin can add member and admin", func(t *testing.T) {
		for _, role := range []organizationsmodel.OrganizationRole{organizationsmodel.OrgRoleAdmin, organizationsmodel.OrgRoleMember} {
			uc, store, actors := setupOwnerGrantFixture()
			admin := actors["admin"]
			target := actors["target"]

			ctx := accessmodel.WithUser(context.Background(), admin)
			err := uc.AddMember(ctx, &organizationsmodel.AddOrganizationMemberInput{
				OrganizationID: store.org.ID,
				UserID:         target.Id,
				Role:           role,
			}, admin.Id)

			require.NoError(t, err)
			assert.Equal(t, role, store.addedRoles[target.Id])
		}
	})

	t.Run("owner can add owner", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		owner := actors["owner"]
		target := actors["target"]

		ctx := accessmodel.WithUser(context.Background(), owner)
		err := uc.AddMember(ctx, &organizationsmodel.AddOrganizationMemberInput{
			OrganizationID: store.org.ID,
			UserID:         target.Id,
			Role:           organizationsmodel.OrgRoleOwner,
		}, owner.Id)

		require.NoError(t, err)
		assert.Equal(t, organizationsmodel.OrgRoleOwner, store.addedRoles[target.Id])
	})

	t.Run("global admin can add owner", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		globalAdmin := actors["global_admin"]
		target := actors["target"]

		ctx := accessmodel.WithUser(context.Background(), globalAdmin)
		err := uc.AddMember(ctx, &organizationsmodel.AddOrganizationMemberInput{
			OrganizationID: store.org.ID,
			UserID:         target.Id,
			Role:           organizationsmodel.OrgRoleOwner,
		}, globalAdmin.Id)

		require.NoError(t, err)
		assert.Equal(t, organizationsmodel.OrgRoleOwner, store.addedRoles[target.Id])
	})
}

func TestInvitationOwnerGrant(t *testing.T) {
	t.Run("admin cannot invite with owner role", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		admin := actors["admin"]
		target := actors["target"]

		ctx := accessmodel.WithUser(context.Background(), admin)
		inv, err := uc.InviteMember(ctx, store.org.Login, target.Id, organizationsmodel.OrgRoleOwner, admin.Id)

		require.ErrorIs(t, err, pkg.NoPermission)
		assert.Nil(t, inv)
		assert.Empty(t, store.invitations)
	})

	t.Run("admin can invite with member and admin role", func(t *testing.T) {
		for _, role := range []organizationsmodel.OrganizationRole{organizationsmodel.OrgRoleAdmin, organizationsmodel.OrgRoleMember} {
			uc, store, actors := setupOwnerGrantFixture()
			admin := actors["admin"]
			target := actors["target"]

			ctx := accessmodel.WithUser(context.Background(), admin)
			inv, err := uc.InviteMember(ctx, store.org.Login, target.Id, role, admin.Id)

			require.NoError(t, err)
			require.NotNil(t, inv)
			assert.Equal(t, role, inv.Role)
		}
	})

	t.Run("owner can invite with owner role", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		owner := actors["owner"]
		target := actors["target"]

		ctx := accessmodel.WithUser(context.Background(), owner)
		inv, err := uc.InviteMember(ctx, store.org.Login, target.Id, organizationsmodel.OrgRoleOwner, owner.Id)

		require.NoError(t, err)
		require.NotNil(t, inv)
		assert.Equal(t, organizationsmodel.OrgRoleOwner, inv.Role)
	})

	t.Run("global admin can invite with owner role", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		globalAdmin := actors["global_admin"]
		target := actors["target"]

		ctx := accessmodel.WithUser(context.Background(), globalAdmin)
		inv, err := uc.InviteMember(ctx, store.org.Login, target.Id, organizationsmodel.OrgRoleOwner, globalAdmin.Id)

		require.NoError(t, err)
		require.NotNil(t, inv)
		assert.Equal(t, organizationsmodel.OrgRoleOwner, inv.Role)
	})
}

func TestApproveJoinRequestOwnerGrant(t *testing.T) {
	setupRequest := func(store *mockOwnerGrantStore, targetID uuid.UUID) uuid.UUID {
		reqID := uuid.New()
		store.requests[reqID] = &organizationsmodel.OrganizationJoinRequest{
			ID:             reqID,
			OrganizationID: store.org.ID,
			UserID:         targetID,
			Status:         organizationsmodel.RequestStatusPending,
		}
		return reqID
	}

	t.Run("admin cannot approve request with owner role", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		admin := actors["admin"]
		target := actors["target"]
		reqID := setupRequest(store, target.Id)

		ctx := accessmodel.WithUser(context.Background(), admin)
		err := uc.ApproveJoinRequest(ctx, store.org.Login, reqID, admin.Id, organizationsmodel.OrgRoleOwner)

		require.ErrorIs(t, err, pkg.NoPermission)
		assert.NotContains(t, store.addedRoles, target.Id)
		assert.Equal(t, organizationsmodel.RequestStatusPending, store.requests[reqID].Status)
	})

	t.Run("admin can approve request with member and admin role", func(t *testing.T) {
		for _, role := range []organizationsmodel.OrganizationRole{organizationsmodel.OrgRoleAdmin, organizationsmodel.OrgRoleMember} {
			uc, store, actors := setupOwnerGrantFixture()
			admin := actors["admin"]
			target := actors["target"]
			reqID := setupRequest(store, target.Id)

			ctx := accessmodel.WithUser(context.Background(), admin)
			err := uc.ApproveJoinRequest(ctx, store.org.Login, reqID, admin.Id, role)

			require.NoError(t, err)
			assert.Equal(t, role, store.addedRoles[target.Id])
			assert.Equal(t, organizationsmodel.RequestStatusApproved, store.requests[reqID].Status)
		}
	})

	t.Run("owner can approve request with owner role", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		owner := actors["owner"]
		target := actors["target"]
		reqID := setupRequest(store, target.Id)

		ctx := accessmodel.WithUser(context.Background(), owner)
		err := uc.ApproveJoinRequest(ctx, store.org.Login, reqID, owner.Id, organizationsmodel.OrgRoleOwner)

		require.NoError(t, err)
		assert.Equal(t, organizationsmodel.OrgRoleOwner, store.addedRoles[target.Id])
		assert.Equal(t, organizationsmodel.RequestStatusApproved, store.requests[reqID].Status)
	})

	t.Run("global admin can approve request with owner role", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		globalAdmin := actors["global_admin"]
		target := actors["target"]
		reqID := setupRequest(store, target.Id)

		ctx := accessmodel.WithUser(context.Background(), globalAdmin)
		err := uc.ApproveJoinRequest(ctx, store.org.Login, reqID, globalAdmin.Id, organizationsmodel.OrgRoleOwner)

		require.NoError(t, err)
		assert.Equal(t, organizationsmodel.OrgRoleOwner, store.addedRoles[target.Id])
		assert.Equal(t, organizationsmodel.RequestStatusApproved, store.requests[reqID].Status)
	})
}

func TestUpdateMemberRoleOwnerGrant(t *testing.T) {
	t.Run("admin cannot promote to owner", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		admin := actors["admin"]
		target := actors["target"]
		store.members[target.Id] = organizationsmodel.OrgRoleMember

		ctx := accessmodel.WithUser(context.Background(), admin)
		err := uc.UpdateMemberRole(ctx, store.org.ID, target.Id, organizationsmodel.OrgRoleOwner, admin.Id)

		require.ErrorIs(t, err, pkg.NoPermission)
		assert.NotContains(t, store.updatedRole, target.Id)
	})

	t.Run("admin can promote to admin", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		admin := actors["admin"]
		target := actors["target"]
		store.members[target.Id] = organizationsmodel.OrgRoleMember

		ctx := accessmodel.WithUser(context.Background(), admin)
		err := uc.UpdateMemberRole(ctx, store.org.ID, target.Id, organizationsmodel.OrgRoleAdmin, admin.Id)

		require.NoError(t, err)
		assert.Equal(t, organizationsmodel.OrgRoleAdmin, store.updatedRole[target.Id])
	})

	t.Run("owner can promote to owner", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		owner := actors["owner"]
		target := actors["target"]
		store.members[target.Id] = organizationsmodel.OrgRoleMember

		ctx := accessmodel.WithUser(context.Background(), owner)
		err := uc.UpdateMemberRole(ctx, store.org.ID, target.Id, organizationsmodel.OrgRoleOwner, owner.Id)

		require.NoError(t, err)
		assert.Equal(t, organizationsmodel.OrgRoleOwner, store.updatedRole[target.Id])
	})

	t.Run("global admin can promote to owner", func(t *testing.T) {
		uc, store, actors := setupOwnerGrantFixture()
		globalAdmin := actors["global_admin"]
		target := actors["target"]
		store.members[target.Id] = organizationsmodel.OrgRoleMember

		ctx := accessmodel.WithUser(context.Background(), globalAdmin)
		err := uc.UpdateMemberRole(ctx, store.org.ID, target.Id, organizationsmodel.OrgRoleOwner, globalAdmin.Id)

		require.NoError(t, err)
		assert.Equal(t, organizationsmodel.OrgRoleOwner, store.updatedRole[target.Id])
	})
}

func (s *mockOwnerGrantStore) GetOrganizationByID(context.Context, uuid.UUID) (*organizationsmodel.Organization, error) {
	return &s.org, nil
}
