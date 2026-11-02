package usecase

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
)

type testTransactions[T any] struct{ store T }

func (t testTransactions[T]) Within(_ context.Context, fn func(T) error) error { return fn(t.store) }

type identityTestStore struct {
	interfaces.UsersRepo
	interfaces.AuthRepo
	interfaces.ContestsRepo
}
type organizationTestStore struct {
	interfaces.OrganizationsRepo
	interfaces.UsersRepo
}
type contestEventsTestStore struct {
	interfaces.AnnouncementsRepo
	interfaces.ClarificationsRepo
	interfaces.OutboxRepo
}
type submissionTestStore struct {
	interfaces.SubmissionsRepo
	interfaces.OutboxRepo
}
