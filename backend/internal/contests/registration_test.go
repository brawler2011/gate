package contests

import (
	"context"
	"net/http"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
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
}

func (r *registrationStore) GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error) {
	return r.contest, nil
}
func (r *registrationStore) GetContestMember(context.Context, *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error) {
	if r.member {
		return contestsmodel.ContestMember{}, nil
	}
	return contestsmodel.ContestMember{}, pkg.ErrNotFound
}
func (*registrationStore) GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error) {
	return &organizationsmodel.OrganizationMember{Role: organizationsmodel.OrgRoleMember}, nil
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

func TestContestRegistrationPreservesParticipationModes(t *testing.T) {
	for _, mode := range []string{contestsmodel.ParticipationModeOpen, contestsmodel.ParticipationModeByRequest, contestsmodel.ParticipationModeInviteOnly} {
		t.Run(mode, func(t *testing.T) {
			contest := contestsmodel.Contest{ID: uuid.New(), OrganizationID: uuid.New(), Settings: map[string]interface{}{"participation_mode": mode}}
			store := &registrationStore{contest: contest}
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
			case contestsmodel.ParticipationModeInviteOnly:
				require.ErrorIs(t, err, pkg.ErrBadInput)
				require.False(t, store.joined)
				require.False(t, store.requested)
			}
		})
	}
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
			contest := contestsmodel.Contest{ID: uuid.New(), OrganizationID: uuid.New(), Settings: map[string]interface{}{"participation_mode": contestsmodel.ParticipationModeByRequest}}
			store := &registrationStore{contest: contest, member: tc.member, pending: tc.pending}
			uc := NewContestsUseCase(registrationLookup{contest: contest}, nil, testTransactions[CommandTxStore]{store}, nil)
			caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			request, joined, err := uc.CreateJoinRequest(accessmodel.WithUser(context.Background(), caller), "org", "contest", nil)
			require.ErrorIs(t, err, pkg.ErrConflict)
			require.Equal(t, http.StatusConflict, pkg.ToREST(err))
			require.Nil(t, request)
			require.False(t, joined)
			require.False(t, store.joined)
			require.False(t, store.requested)
		})
	}
}
