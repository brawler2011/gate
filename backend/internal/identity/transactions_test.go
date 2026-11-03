package identity

import (
	"context"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"

	"github.com/google/uuid"
)

type testTransactions[T any] struct{ store T }

func (t testTransactions[T]) Within(_ context.Context, fn func(T) error) error { return fn(t.store) }

type identityTestStore struct {
	UsersRepo
	AuthRepo
	ContestsRepo
}

func (identityTestStore) LockUser(context.Context, uuid.UUID) error { return nil }

type ContestsRepo interface {
	ListUserContestMemberships(context.Context, uuid.UUID) ([]contestsmodel.UserContestMembership, error)
	AddContestMemberIfNotExists(context.Context, uuid.UUID, uuid.UUID, string) error
}

type OutboxRepo interface {
	eventdata.Writer
}
