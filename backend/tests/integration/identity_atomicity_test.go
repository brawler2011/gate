//go:build integration

package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/identity"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestRegistrationRollsBackTokenFailure() {
	_, err := s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_auth_token() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected token failure'; END $$;
CREATE TRIGGER reject_auth_token BEFORE INSERT ON auth_tokens FOR EACH ROW EXECUTE FUNCTION reject_auth_token()`)
	s.Require().NoError(err)
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_auth_token ON auth_tokens; DROP FUNCTION reject_auth_token()`)
		s.Require().NoError(err)
	}()
	auth := identity.NewAuthUseCase(s.usersRepo, pgstore.NewAuthRepo(s.dbPool), pgstore.NewIdentityTransactions(s.dbPool), nil)
	_, err = auth.Register(s.ctx, "atomic_registration", "atomic@example.com", "Password123!")
	s.Require().Error(err)
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM users WHERE username='atomic_registration'`).Scan(&count))
	s.Zero(count)
}

func (s *IntegrationTestSuite) TestVerificationRollsBackSessionFailureAndConsumesTokenOnce() {
	user := s.createUser("atomic_verification", accessmodel.UserRoleUser)
	s.Require().NoError(s.usersRepo.SetUserEmailVerified(s.ctx, user.Id, false))
	repo := pgstore.NewAuthRepo(s.dbPool)
	token := uuid.NewString()
	digest := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(digest[:])
	s.Require().NoError(repo.CreateAuthToken(s.ctx, identitymodel.CreateAuthTokenParams{ID: uuid.New(), UserID: user.Id, TokenType: identitymodel.AuthTokenTypeEmailVerification, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}))
	_, err := s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_auth_session() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected session failure'; END $$;
CREATE TRIGGER reject_auth_session BEFORE INSERT ON sessions FOR EACH ROW EXECUTE FUNCTION reject_auth_session()`)
	s.Require().NoError(err)
	auth := identity.NewAuthUseCase(s.usersRepo, repo, pgstore.NewIdentityTransactions(s.dbPool), nil)
	_, _, err = auth.VerifyEmail(s.ctx, token)
	_, cleanupErr := s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_auth_session ON sessions; DROP FUNCTION reject_auth_session()`)
	s.Require().NoError(cleanupErr)
	s.Require().Error(err)
	persisted, err := s.usersRepo.GetUserById(s.ctx, user.Id)
	s.Require().NoError(err)
	s.False(persisted.IsEmailVerified)
	_, err = repo.GetAuthTokenByHash(s.ctx, hash, identitymodel.AuthTokenTypeEmailVerification)
	s.Require().NoError(err, "failed verification must retain the token")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, _, err := auth.VerifyEmail(s.ctx, token); results <- err })
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	s.Equal(1, successes, "a verification token can create only one session")
	var sessions int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM sessions WHERE user_id=$1`, user.Id).Scan(&sessions))
	s.Equal(1, sessions)
}
