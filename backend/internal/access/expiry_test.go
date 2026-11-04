package access

import (
	"context"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestExpiredAccountCannotResolvePermissions(t *testing.T) {
	users := newMockUsersReader()
	actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleAdmin}
	ctx := accessmodel.WithUser(context.Background(), actor)
	expired := time.Now().Add(-time.Hour)
	actor.ExpiresAt = &expired
	users.users[actor.Id] = actor
	uc := NewPermissionsUseCase(nil, users, nil, nil, nil)
	for name, check := range map[string]func() error{
		"contest action": func() error {
			_, err := uc.HasContestPermission(ctx, uuid.New(), actor.Id, accessmodel.ActionManageContest)
			return err
		},
		"problem action": func() error {
			_, err := uc.HasProblemPermission(ctx, uuid.New(), actor.Id, accessmodel.ActionEditProblem)
			return err
		},
		"organization action": func() error {
			_, err := uc.HasOrganizationPermission(ctx, uuid.New(), actor.Id, accessmodel.ActionManageOrganization)
			return err
		},
		"contest permissions": func() error { _, err := uc.GetContestPermissions(ctx, uuid.New(), actor.Id); return err },
		"problem permissions": func() error { _, err := uc.GetProblemPermissions(ctx, uuid.New(), actor.Id); return err },
	} {
		t.Run(name, func(t *testing.T) { require.ErrorIs(t, check(), pkg.ErrUnauthenticated) })
	}
}
