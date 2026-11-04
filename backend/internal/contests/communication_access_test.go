package contests

import (
	"context"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type communicationPermissions struct{ view, manage bool }

func (p communicationPermissions) HasContestPermission(_ context.Context, _, _ uuid.UUID, action accessmodel.ContestAction) (bool, error) {
	if action == accessmodel.ActionManageContest {
		return p.manage, nil
	}
	return p.view, nil
}

type communicationReads struct {
	user      uuid.UUID
	moderator bool
	called    bool
}

func (r *communicationReads) ListAnnouncements(context.Context, uuid.UUID, int32, int32) (*contestsmodel.ContestAnnouncementsList, error) {
	r.called = true
	return &contestsmodel.ContestAnnouncementsList{}, nil
}
func (r *communicationReads) ListClarificationsForUser(_ context.Context, _ uuid.UUID, user uuid.UUID, _, _ int32) (*contestsmodel.ContestClarificationsList, error) {
	r.called = true
	r.user = user
	return &contestsmodel.ContestClarificationsList{}, nil
}
func (r *communicationReads) ListClarificationsForModerator(context.Context, uuid.UUID, *contestsmodel.ContestClarificationsFilter) (*contestsmodel.ContestClarificationsList, error) {
	r.called = true
	r.moderator = true
	return &contestsmodel.ContestClarificationsList{}, nil
}

func TestCommunicationReadsEnforceVisibility(t *testing.T) {
	for _, tc := range []struct {
		name                string
		guest, view, manage bool
		err                 error
	}{
		{name: "participant", view: true},
		{name: "moderator", view: true, manage: true},
		{name: "outsider", err: pkg.NoPermission},
		{name: "guest", guest: true, view: true, err: pkg.ErrUnauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := &communicationReads{}
			reader := NewCommunicationReader(reads, reads, communicationPermissions{tc.view, tc.manage})
			user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			ctx := context.Background()
			if !tc.guest {
				ctx = accessmodel.WithUser(ctx, user)
			}
			_, err := reader.ListClarifications(ctx, uuid.New(), &contestsmodel.ContestClarificationsFilter{Page: 1, PageSize: 50})
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, tc.err == nil, reads.called)
			if tc.err == nil {
				require.Equal(t, tc.manage, reads.moderator)
				if !tc.manage {
					require.Equal(t, user.Id, reads.user)
				}
			}
			reads.called = false
			_, err = reader.ListAnnouncements(ctx, uuid.New(), 1, 50)
			if tc.view {
				require.NoError(t, err)
				require.True(t, reads.called)
			} else {
				require.ErrorIs(t, err, pkg.NoPermission)
				require.False(t, reads.called)
			}
		})
	}
}

type deniedCommunicationStore struct {
	contestEventsTestStore
	locked bool
}

func (s *deniedCommunicationStore) LockContest(context.Context, uuid.UUID) error {
	s.locked = true
	return nil
}
func (s *deniedCommunicationStore) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error) {
	if !s.locked {
		panic("permission check before lock")
	}
	return false, nil
}

func TestCommunicationCommandsRequireTransactionBoundPermission(t *testing.T) {
	store := &deniedCommunicationStore{}
	transactions := testTransactions[ContestEventsTxStore]{store}
	actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	ctx := accessmodel.WithUser(context.Background(), actor)
	announcements := NewAnnouncementsUseCase(transactions)
	_, err := announcements.CreateAnnouncement(ctx, &contestsmodel.CreateContestAnnouncementInput{ContestID: uuid.New(), AuthorID: actor.Id, Title: "Title", Body: "Body"})
	require.ErrorIs(t, err, pkg.NoPermission)
	require.ErrorIs(t, announcements.DeleteAnnouncement(ctx, uuid.New(), uuid.New()), pkg.NoPermission)
	clarifications := NewClarificationsUseCase(transactions)
	_, err = clarifications.CreateClarification(ctx, &contestsmodel.CreateContestClarificationInput{ContestID: uuid.New(), UserID: actor.Id, Question: "Question"})
	require.ErrorIs(t, err, pkg.NoPermission)
	_, err = clarifications.AnswerClarification(ctx, &contestsmodel.AnswerContestClarificationInput{ContestID: uuid.New(), AnsweredBy: actor.Id, Answer: "Answer"})
	require.ErrorIs(t, err, pkg.NoPermission)
}

func TestClarificationsSettingCheckedInsideTransaction(t *testing.T) {
	store := contestEventsTestStore{Contest: contestsmodel.Contest{Settings: map[string]interface{}{"allow_clarifications": false}}}
	user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	uc := NewClarificationsUseCase(testTransactions[ContestEventsTxStore]{store})
	_, err := uc.CreateClarification(accessmodel.WithUser(context.Background(), user), &contestsmodel.CreateContestClarificationInput{ContestID: uuid.New(), UserID: user.Id, Question: "Question"})
	require.ErrorIs(t, err, pkg.NoPermission)
}
