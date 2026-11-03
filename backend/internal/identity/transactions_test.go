package identity

import (
	"context"
)

type testTransactions[T any] struct{ store T }

func (t testTransactions[T]) Within(_ context.Context, fn func(T) error) error { return fn(t.store) }

type identityTestStore struct {
	UsersRepo
	AuthRepo
	ContestsRepo
}
