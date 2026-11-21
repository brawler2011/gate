package contests

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type registrationLookup struct {
	ContestsRepo
	contest contestsmodel.Contest
}

func (r registrationLookup) GetContestByOrgLoginAndContestLogin(context.Context, string, string) (contestsmodel.Contest, error) {
	return r.contest, nil
}

type registrationStore struct {
	CommandTxStore
	contest           contestsmodel.Contest
	joined, requested bool
	member, pending   bool
	owners, notified  []uuid.UUID
}

func (r *registrationStore) Actor(ctx context.Context, _ uuid.UUID) (accessmodel.User, error) {
	return accessmodel.UserFromContext(ctx), nil
}
func (r *registrationStore) ContestFacts(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]access.ContestFacts, error) {
	return map[uuid.UUID]access.ContestFacts{r.contest.ID: {Contest: r.contest, Now: time.Now()}}, nil
}
func (r *registrationStore) GetContestMember(context.Context, *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error) {
	if r.member {
		return contestsmodel.ContestMember{}, nil
	}
	return contestsmodel.ContestMember{}, pkg.ErrNotFound
}
func (*registrationStore) GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error) {
	return &organizationsmodel.OrganizationMember{Role: accessmodel.OrganizationMember}, nil
}
func (r *registrationStore) CreateContestMember(context.Context, *contestsmodel.CreateContestMemberParams) error {
	r.joined = true
	return nil
}
func (r *registrationStore) GetPendingContestJoinRequest(context.Context, uuid.UUID, uuid.UUID) (*contestsmodel.ContestJoinRequest, error) {
	if r.pending {
		return &contestsmodel.ContestJoinRequest{}, nil
	}
	return nil, nil
}
func (r *registrationStore) CreateContestJoinRequest(_ context.Context, input *contestsmodel.CreateContestJoinRequestInput) (*contestsmodel.ContestJoinRequest, error) {
	r.requested = true
	return &contestsmodel.ContestJoinRequest{ID: uuid.New(), ContestID: input.ContestID, UserID: input.UserID}, nil
}

func (r *registrationStore) ListContestOwners(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return r.owners, nil
}
func (r *registrationStore) CreateNotification(_ context.Context, input *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error) {
	r.notified = append(r.notified, input.UserID)
	return &notificationsmodel.Notification{UserID: input.UserID}, nil
}

func TestContestRegistrationPreservesParticipationModes(t *testing.T) {
	for _, mode := range []contestsmodel.ParticipationMode{contestsmodel.ParticipationModeOpen, contestsmodel.ParticipationModeByRequest, contestsmodel.ParticipationModeInviteOnly} {
		t.Run(string(mode), func(t *testing.T) {
			contest := contestsmodel.Contest{ID: uuid.New(), OrganizationID: uuid.New(), Visibility: contestsmodel.ContestVisibilityPublic, Settings: contestsmodel.ContestSettings{ParticipationMode: mode}}
			store := &registrationStore{contest: contest, owners: []uuid.UUID{uuid.New(), uuid.New()}}
			uc := NewContestsUseCase(registrationLookup{contest: contest}, nil, testTransactions[CommandTxStore]{store}, nil)
			caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			request, joined, err := uc.CreateJoinRequest(accessmodel.WithUser(context.Background(), caller), "org", "contest", nil)
			switch mode {
			case contestsmodel.ParticipationModeOpen:
				require.NoError(t, err)
				require.True(t, joined)
				require.True(t, store.joined)
				require.Nil(t, request)
			case contestsmodel.ParticipationModeByRequest:
				require.NoError(t, err)
				require.False(t, joined)
				require.True(t, store.requested)
				require.NotNil(t, request)
				require.Equal(t, store.owners, store.notified, "every owner hears about the request")
			case contestsmodel.ParticipationModeInviteOnly:
				require.ErrorIs(t, err, pkg.ErrInviteOnly)
				require.False(t, store.joined)
				require.False(t, store.requested)
			}
		})
	}
}

func TestContestRegistrationRechecksVisibility(t *testing.T) {
	contest := contestsmodel.Contest{ID: uuid.New(), OrganizationID: uuid.New(), Visibility: contestsmodel.ContestVisibilityPublic, Settings: contestsmodel.ContestSettings{ParticipationMode: contestsmodel.ParticipationModeOpen}}
	hidden := contest
	hidden.Visibility = contestsmodel.ContestVisibilityPrivate
	// The pre-transaction lookup still sees the public contest; the transaction sees it hidden.
	store := &registrationStore{contest: hidden}
	uc := NewContestsUseCase(registrationLookup{contest: contest}, nil, testTransactions[CommandTxStore]{store}, nil)
	caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	_, _, err := uc.CreateJoinRequest(accessmodel.WithUser(context.Background(), caller), "org", "contest", nil)
	require.ErrorIs(t, err, pkg.ErrNotFound)
	require.False(t, store.joined)
}

func TestContestRegistrationRejectsDuplicates(t *testing.T) {
	for _, tc := range []struct {
		name            string
		member, pending bool
	}{
		{name: "existing member", member: true},
		{name: "pending request", pending: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contest := contestsmodel.Contest{ID: uuid.New(), OrganizationID: uuid.New(), Visibility: contestsmodel.ContestVisibilityPublic, Settings: contestsmodel.ContestSettings{ParticipationMode: contestsmodel.ParticipationModeByRequest}}
			store := &registrationStore{contest: contest, member: tc.member, pending: tc.pending}
			uc := NewContestsUseCase(registrationLookup{contest: contest}, nil, testTransactions[CommandTxStore]{store}, nil)
			caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			request, joined, err := uc.CreateJoinRequest(accessmodel.WithUser(context.Background(), caller), "org", "contest", nil)
			require.ErrorIs(t, err, pkg.ErrConflict)
			require.Equal(t, http.StatusConflict, pkg.HTTPStatus(pkg.CodeOf(err)))
			require.Nil(t, request)
			require.False(t, joined)
			require.False(t, store.joined)
			require.False(t, store.requested)
		})
	}
}
