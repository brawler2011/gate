package identity

import (
	"context"
	"runtime"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestLoginBcryptSemaphoreCancellationAndRelease(t *testing.T) {
	ctx := context.Background()
	users, sessions := newFullMockUsersRepo(), newFullMockAuthRepo()
	hash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, users.CreateUser(ctx, identitymodel.CreateUserParams{Id: uuid.New(), Username: "limited", Role: accessmodel.UserRoleUser, PasswordHash: string(hash), IsEmailVerified: true}))
	store := identityTestStore{identityUsersStore: users, identityAuthStore: sessions}
	uc := NewAuthUseCase(users, sessions, testTransactions[IdentityTxStore]{store}, nil, ratelimit.New(ratelimit.NewMemory()))
	require.Equal(t, runtime.GOMAXPROCS(0)*2, cap(uc.bcryptSlots))
	for range cap(uc.bcryptSlots) {
		uc.bcryptSlots <- struct{}{}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = uc.Login(cancelled, "limited", "Password123")
	require.ErrorIs(t, err, context.Canceled)
	for range cap(uc.bcryptSlots) {
		<-uc.bcryptSlots
	}
	_, _, err = uc.Login(ctx, "limited", "wrong-password")
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
	require.Empty(t, uc.bcryptSlots)
	_, _, err = uc.Login(ctx, "limited", "Password123")
	require.NoError(t, err)
	require.Empty(t, uc.bcryptSlots)
}
