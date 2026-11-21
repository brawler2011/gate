package access

import (
	"context"
	"errors"
	"testing"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type checkerFunc func(m.ContestAction) (bool, error)

func (f checkerFunc) CanContest(_ context.Context, _ uuid.UUID, a m.ContestAction) (bool, error) {
	return f(a)
}

func TestAuthorizeMapsDenials(t *testing.T) {
	failure := errors.New("facts unavailable")
	user := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	for _, tc := range []struct {
		name    string
		actor   m.User
		allowed map[m.ContestAction]bool
		err     error
		want    error
	}{
		{name: "allowed", actor: user, allowed: map[m.ContestAction]bool{m.ActionManageContest: true}},
		{name: "visible", actor: user, allowed: map[m.ContestAction]bool{m.ActionGetContest: true}, want: pkg.ErrForbidden},
		{name: "hidden", actor: user, want: pkg.ErrNotFound},
		{name: "guest", actor: m.Guest, allowed: map[m.ContestAction]bool{m.ActionGetContest: true}, want: pkg.ErrUnauthenticated},
		{name: "check failure", actor: m.Guest, err: failure, want: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check := checkerFunc(func(a m.ContestAction) (bool, error) { return tc.allowed[a], tc.err })
			err := Contest(m.WithUser(context.Background(), tc.actor), check, uuid.New(), m.ActionManageContest)
			require.ErrorIs(t, err, tc.want)
			if tc.want == nil {
				require.NoError(t, err)
			}
		})
	}
}
