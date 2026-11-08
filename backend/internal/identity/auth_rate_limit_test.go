package identity

import (
	"context"
	"runtime"
	"testing"

	"github.com/alicebob/miniredis/v2"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestLoginBcryptSemaphoreCancellationAndRelease(t *testing.T) {
	ctx := context.Background()
	users, sessions := newFullMockUsersRepo(), newFullMockAuthRepo()
	hash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, users.CreateUser(ctx, identitymodel.CreateUserParams{Id: uuid.New(), Username: "limited", Role: accessmodel.UserRoleUser, PasswordHash: string(hash), IsEmailVerified: true}))
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	store := identityTestStore{identityUsersStore: users, identityAuthStore: sessions}
	uc := NewAuthUseCase(users, sessions, testTransactions[IdentityTxStore]{store}, nil, ratelimit.New(client))
	require.Equal(t, runtime.GOMAXPROCS(0)*2, cap(uc.bcryptSlots))
	for range cap(uc.bcryptSlots) {
		uc.bcryptSlots <- struct{}{}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = uc.Login(cancelled, "limited", "Password123")
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, server.Exists("rl:fail:login:limited"))
	for range cap(uc.bcryptSlots) {
		<-uc.bcryptSlots
	}
	_, _, err = uc.Login(ctx, "limited", "wrong-password")
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
	require.Empty(t, uc.bcryptSlots)
	_, _, err = uc.Login(ctx, "limited", "Password123")
	require.NoError(t, err)
	require.Empty(t, uc.bcryptSlots)
	require.False(t, server.Exists("rl:fail:login:limited"))
}
