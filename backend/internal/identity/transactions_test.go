package identity

import (
	"context"

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
