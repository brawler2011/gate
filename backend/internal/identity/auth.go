package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/email"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionHardLifetime = 7 * 24 * time.Hour
	sessionSoftLifetime = 40 * time.Minute

	verificationTokenTTL  = 24 * time.Hour
	passwordResetTokenTTL = 1 * time.Hour
	emailChangeTokenTTL   = 24 * time.Hour
)

type AuthUseCase struct {
	usersRepo    UsersRepo
	authRepo     AuthRepo
	txManager    IdentityTransactions
	emailService email.EmailService
	limiter      *ratelimit.Limiter
	bcryptSlots  chan struct{}
}

func NewAuthUseCase(
	usersRepo UsersRepo,
	authRepo AuthRepo,
	txManager IdentityTransactions,
	emailService email.EmailService,
	limiter *ratelimit.Limiter,
) *AuthUseCase {
	return &AuthUseCase{
		usersRepo: usersRepo, authRepo: authRepo, txManager: txManager,
		emailService: emailService, limiter: limiter,
		bcryptSlots: make(chan struct{}, runtime.GOMAXPROCS(0)*2),
	}
}

func generateSecureToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("read random bytes: %w", err)
	}
	token := hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])
	return token, tokenHash, nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (uc *AuthUseCase) Register(ctx context.Context, username, emailAddr, password string) (accessmodel.User, error) {
	if err := identitymodel.UsernameValidate(username); err != nil {
		return accessmodel.User{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid username")
	}
	if err := identitymodel.EmailValidate(emailAddr); err != nil {
		return accessmodel.User{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid email")
	}
	if err := identitymodel.PasswordValidate(password); err != nil {
		return accessmodel.User{}, pkg.Wrap(pkg.ErrInvalid, err, "invalid password")
	}

	if err := uc.limiter.Email(ctx, emailAddr); err != nil {
		return accessmodel.User{}, err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return accessmodel.User{}, pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return accessmodel.User{}, pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	var targetUserID uuid.UUID
	err = uc.txManager.Within(ctx, func(store IdentityTxStore) error {
		existingByName, errByName := store.GetUserByUsername(ctx, username)
		if errByName != nil && !errors.Is(errByName, pkg.ErrNotFound) {
			return errByName
		}

		existingByEmail, errByEmail := store.GetUserByUsernameOrEmail(ctx, emailAddr)
		if errByEmail != nil && !errors.Is(errByEmail, pkg.ErrNotFound) {
			return errByEmail
		}

		if errByName == nil {
			if err := store.LockUser(ctx, existingByName.Id); err != nil {
				return err
			}
			existingByName, errByName = store.GetUserById(ctx, existingByName.Id)
			if errByName != nil {
				return errByName
			}
		}
		if errByEmail == nil {
			if err := store.LockUser(ctx, existingByEmail.Id); err != nil {
				return err
			}
			existingByEmail, errByEmail = store.GetUserById(ctx, existingByEmail.Id)
			if errByEmail != nil {
				return errByEmail
			}
		}

		if errByName == nil && existingByName.IsEmailVerified {
			return pkg.Wrap(pkg.ErrUsernameTaken, nil, "username is taken")
		}
		if errByEmail == nil && existingByEmail.IsEmailVerified {
			return pkg.Wrap(pkg.ErrEmailTaken, nil, "email is taken")
		}

		if errByName == nil && errByEmail == nil && existingByName.Id != existingByEmail.Id {
			return pkg.Wrap(pkg.ErrConflict, nil, "username or email already in use")
		}

		switch {
		case errByName == nil:
			targetUserID = existingByName.Id
			if err := store.UpdateUserPassword(ctx, targetUserID, string(hashed)); err != nil {
				return err
			}
			if existingByName.Username != username {
				if err := store.UpdateUser(ctx, identitymodel.UpdateUserParams{
					Id:       targetUserID,
					Username: &username,
				}); err != nil {
					return err
				}
			}
			if existingByName.Email == nil || *existingByName.Email != emailAddr {
				if err := store.UpdateUserEmail(ctx, targetUserID, emailAddr, false); err != nil {
					return err
				}
			}
			if err := store.DeleteAuthTokensByUserIdAndType(ctx, targetUserID, identitymodel.AuthTokenTypeEmailVerification); err != nil {
				return err
			}
		case errByEmail == nil:
			targetUserID = existingByEmail.Id
			if err := store.UpdateUserPassword(ctx, targetUserID, string(hashed)); err != nil {
				return err
			}
			if err := store.UpdateUser(ctx, identitymodel.UpdateUserParams{
				Id:       targetUserID,
				Username: &username,
			}); err != nil {
				return err
			}
			if err := store.DeleteAuthTokensByUserIdAndType(ctx, targetUserID, identitymodel.AuthTokenTypeEmailVerification); err != nil {
				return err
			}
		default:
			targetUserID = uuid.New()
			err = store.CreateUser(ctx, identitymodel.CreateUserParams{
				Id:              targetUserID,
				Username:        username,
				Role:            accessmodel.UserRoleUser,
				PasswordHash:    string(hashed),
				Email:           &emailAddr,
				IsEmailVerified: false,
			})
			if err != nil {
				return err
			}
		}

		err = store.CreateAuthToken(ctx, identitymodel.CreateAuthTokenParams{
			ID:        uuid.New(),
			UserID:    targetUserID,
			TokenType: identitymodel.AuthTokenTypeEmailVerification,
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(verificationTokenTTL),
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return accessmodel.User{}, err
	}

	if err := uc.emailService.SendVerificationEmail(ctx, emailAddr, username, token); err != nil {
		slog.ErrorContext(ctx, "failed to send verification email", "error", err, "email", emailAddr)
	}

	user, err := uc.usersRepo.GetUserById(ctx, targetUserID)
	if err != nil {
		return accessmodel.User{}, err
	}

	return user, nil
}

func (uc *AuthUseCase) Login(ctx context.Context, identifier, password string) (result accessmodel.User, session uuid.UUID, loginErr error) {
	defer func() {
		if errors.Is(loginErr, pkg.ErrUnauthenticated) {
			if err := uc.limiter.LoginFailure(ctx, identifier); err != nil {
				loginErr = err
			}
		} else if loginErr == nil {
			uc.limiter.LoginSuccess(ctx, identifier)
		}
	}()
	user, err := uc.usersRepo.GetUserByUsernameOrEmail(ctx, identifier)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return accessmodel.User{}, uuid.Nil, pkg.Wrap(pkg.ErrUnauthenticated, err, "invalid credentials")
		}
		return accessmodel.User{}, uuid.Nil, err
	}

	// Account state is revealed only after the password matches.
	// Valid credentials clear cooldown; bound bcrypt work independently of Redis availability.
	select {
	case uc.bcryptSlots <- struct{}{}:
	case <-ctx.Done():
		return accessmodel.User{}, uuid.Nil, ctx.Err()
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	<-uc.bcryptSlots
	if err != nil {
		return accessmodel.User{}, uuid.Nil, pkg.Wrap(pkg.ErrUnauthenticated, err, "invalid credentials")
	}
	if user.IsExpired() {
		return accessmodel.User{}, uuid.Nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "account has expired")
	}
	if !user.IsEmailVerified {
		return accessmodel.User{}, uuid.Nil, pkg.Wrap(pkg.ErrEmailNotVerified, nil, "email not verified")
	}

	sessionID := uuid.New()
	expiresAt := time.Now().Add(sessionSoftLifetime)

	err = uc.txManager.Within(ctx, func(store IdentityTxStore) error {
		if err := store.LockUser(ctx, user.Id); err != nil {
			return err
		}
		current, err := store.GetUserById(ctx, user.Id)
		if err != nil {
			return err
		}
		if current.PasswordHash != user.PasswordHash || current.IsExpired() {
			return pkg.Wrap(pkg.ErrUnauthenticated, nil, "credentials changed or account expired")
		}
		if !current.IsEmailVerified {
			return pkg.Wrap(pkg.ErrEmailNotVerified, nil, "email not verified")
		}
		user = current
		return store.CreateSession(ctx, sessionID, user.Id, expiresAt)
	})
	if err != nil {
		return accessmodel.User{}, uuid.Nil, err
	}

	return user, sessionID, nil
}

func (uc *AuthUseCase) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return uc.authRepo.DeleteSession(ctx, sessionID)
}

func (uc *AuthUseCase) Authenticate(ctx context.Context, sessionID uuid.UUID) (accessmodel.User, error) {
	return uc.authenticate(ctx, sessionID, true)
}

// Observer lease validation must not extend an idle session.
func (uc *AuthUseCase) ValidateSession(ctx context.Context, sessionID uuid.UUID) (accessmodel.User, error) {
	return uc.authenticate(ctx, sessionID, false)
}
func (uc *AuthUseCase) authenticate(ctx context.Context, sessionID uuid.UUID, touch bool) (accessmodel.User, error) {
	session, err := uc.authRepo.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return accessmodel.User{}, pkg.Wrap(pkg.ErrUnauthenticated, err, "invalid session")
		}
		return accessmodel.User{}, fmt.Errorf("get session: %w", err)
	}

	hardExpiry := session.CreatedAt.Add(sessionHardLifetime)
	if time.Now().After(hardExpiry) {
		if touch {
			uc.deleteExpiredSession(ctx, sessionID)
		}
		return accessmodel.User{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "session expired (hard limit)")
	}

	if session.IsExpired() {
		if touch {
			uc.deleteExpiredSession(ctx, sessionID)
		}
		return accessmodel.User{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "session expired (soft limit)")
	}

	if touch {
		newExpiry := time.Now().Add(sessionSoftLifetime)
		if newExpiry.After(hardExpiry) {
			newExpiry = hardExpiry
		}
		err = uc.authRepo.UpdateSessionExpiry(ctx, sessionID, newExpiry)
		if err != nil {
			slog.WarnContext(ctx, "extend session", "error", err)
		}
	}

	user, err := uc.usersRepo.GetUserById(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return accessmodel.User{}, pkg.Wrap(pkg.ErrUnauthenticated, err, "user not found")
		}
		return accessmodel.User{}, fmt.Errorf("get session user: %w", err)
	}

	if user.IsExpired() {
		if touch {
			_ = uc.authRepo.DeleteSession(ctx, sessionID)
		}
		return accessmodel.User{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "account has expired")
	}

	return user, nil
}

func (uc *AuthUseCase) CleanupExpiredSessions(ctx context.Context) error {
	cutoff := time.Now().Add(-sessionHardLifetime)
	return uc.authRepo.CleanupExpiredSessions(ctx, cutoff)
}

func (uc *AuthUseCase) VerifyEmail(ctx context.Context, token string) (accessmodel.User, uuid.UUID, error) {
	var user accessmodel.User
	var sessionID uuid.UUID
	err := uc.txManager.Within(ctx, func(store IdentityTxStore) error {
		var err error
		user, sessionID, err = uc.verifyEmailWithin(ctx, store, token)
		return err
	})
	return user, sessionID, err
}

func (uc *AuthUseCase) verifyEmailWithin(ctx context.Context, store IdentityTxStore, token string) (accessmodel.User, uuid.UUID, error) {
	tokenHash := hashToken(token)

	tok, err := store.GetAuthTokenByHash(ctx, tokenHash, identitymodel.AuthTokenTypeEmailVerification)
	if err != nil {
		return accessmodel.User{}, uuid.Nil, pkg.OnNotFound(err, pkg.ErrInvalid, "invalid or expired verification link")
	}

	if time.Now().After(tok.ExpiresAt) {
		if err := store.DeleteAuthToken(ctx, tok.ID); err != nil {
			return accessmodel.User{}, uuid.Nil, err
		}
		return accessmodel.User{}, uuid.Nil, pkg.Wrap(pkg.ErrInvalid, nil, "verification link has expired")
	}

	err = store.SetUserEmailVerified(ctx, tok.UserID, true)
	if err != nil {
		return accessmodel.User{}, uuid.Nil, err
	}

	if err := store.DeleteAuthToken(ctx, tok.ID); err != nil {
		return accessmodel.User{}, uuid.Nil, err
	}

	user, err := store.GetUserById(ctx, tok.UserID)
	if err != nil {
		return accessmodel.User{}, uuid.Nil, err
	}

	sessionID := uuid.New()
	expiresAt := time.Now().Add(sessionSoftLifetime)
	err = store.CreateSession(ctx, sessionID, user.Id, expiresAt)
	if err != nil {
		return accessmodel.User{}, uuid.Nil, err
	}

	return user, sessionID, nil
}

// Like ForgotPassword, the answer does not reveal whether the account exists or has an email.
func (uc *AuthUseCase) ResendVerificationEmail(ctx context.Context, identifier string) error {
	user, err := uc.usersRepo.GetUserByUsernameOrEmail(ctx, identifier)
	if errors.Is(err, pkg.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.IsEmailVerified || user.Email == nil || *user.Email == "" {
		return nil
	}

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	err = replaceAuthToken(ctx, uc.txManager, identitymodel.CreateAuthTokenParams{
		ID:        uuid.New(),
		UserID:    user.Id,
		TokenType: identitymodel.AuthTokenTypeEmailVerification,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(verificationTokenTTL),
	})
	if err != nil {
		return err
	}

	return uc.emailService.SendVerificationEmail(ctx, *user.Email, user.Username, token)
}

//nolint:nilerr // Do not reveal whether the account exists.
func (uc *AuthUseCase) ForgotPassword(ctx context.Context, identifier string) error {
	user, err := uc.usersRepo.GetUserByUsernameOrEmail(ctx, identifier)
	if err != nil {
		return nil
	}

	if user.Email == nil || *user.Email == "" {
		return nil
	}

	if err := uc.limiter.Email(ctx, *user.Email); err != nil {
		return err
	}
	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	err = replaceAuthToken(ctx, uc.txManager, identitymodel.CreateAuthTokenParams{
		ID:        uuid.New(),
		UserID:    user.Id,
		TokenType: identitymodel.AuthTokenTypePasswordReset,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(passwordResetTokenTTL),
	})
	if err != nil {
		return err
	}

	return uc.emailService.SendPasswordResetEmail(ctx, *user.Email, user.Username, token)
}

func (uc *AuthUseCase) ResetPassword(ctx context.Context, token, newPassword string) error {
	err := uc.txManager.Within(ctx, func(store IdentityTxStore) error {
		return uc.resetPasswordWithin(ctx, store, token, newPassword)
	})
	return err
}

func (uc *AuthUseCase) resetPasswordWithin(ctx context.Context, store IdentityTxStore, token, newPassword string) error {
	if err := identitymodel.PasswordValidate(newPassword); err != nil {
		return pkg.Wrap(pkg.ErrInvalid, err, "invalid password")
	}

	tokenHash := hashToken(token)

	tok, err := store.GetAuthTokenByHash(ctx, tokenHash, identitymodel.AuthTokenTypePasswordReset)
	if err != nil {
		return pkg.OnNotFound(err, pkg.ErrInvalid, "invalid or expired reset token")
	}

	if time.Now().After(tok.ExpiresAt) {
		if err := store.DeleteAuthToken(ctx, tok.ID); err != nil {
			return err
		}
		return pkg.Wrap(pkg.ErrInvalid, nil, "reset token has expired")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}

	err = store.UpdateUserPassword(ctx, tok.UserID, string(hashed))
	if err != nil {
		return err
	}

	if err := store.DeleteSessionsByUserID(ctx, tok.UserID); err != nil {
		return err
	}
	if err := store.DeleteAuthToken(ctx, tok.ID); err != nil {
		return err
	}

	return nil
}

func (uc *AuthUseCase) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) (uuid.UUID, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, userID); err != nil {
		return uuid.Nil, err
	}
	if err := identitymodel.PasswordValidate(newPassword); err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInvalid, err, "invalid password")
	}
	user, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInvalid, err, "incorrect current password")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}
	sessionID := uuid.New()
	err = withinAccount(ctx, uc.txManager, userID, false, func(store IdentityTxStore) error {
		current, err := store.GetUserById(ctx, userID)
		if err != nil {
			return err
		}
		if current.PasswordHash != user.PasswordHash {
			return pkg.Wrap(pkg.ErrConflict, nil, "password changed concurrently")
		}
		if err := store.UpdateUserPassword(ctx, userID, string(hashed)); err != nil {
			return err
		}
		if err := store.DeleteSessionsByUserID(ctx, userID); err != nil {
			return err
		}
		return store.CreateSession(ctx, sessionID, userID, time.Now().Add(sessionSoftLifetime))
	})
	if err != nil {
		return uuid.Nil, err
	}
	return sessionID, nil
}

type emailChangePayload struct {
	NewEmail string `json:"new_email"`
}

func (uc *AuthUseCase) RequestEmailChange(ctx context.Context, userID uuid.UUID, password, newEmail string) error {
	if err := accessmodel.RequireSelfOrAdmin(ctx, userID); err != nil {
		return err
	}
	if err := identitymodel.EmailValidate(newEmail); err != nil {
		return pkg.Wrap(pkg.ErrInvalid, err, "invalid email")
	}

	user, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return pkg.OnNotFound(err, pkg.ErrUnauthenticated, "user not found")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return pkg.Wrap(pkg.ErrInvalid, err, "incorrect password")
	}

	existing, err := uc.usersRepo.GetUserByUsernameOrEmail(ctx, newEmail)
	switch {
	case err == nil && existing.Id != userID:
		return pkg.Wrap(pkg.ErrEmailTaken, nil, "email is taken")
	case err != nil && !errors.Is(err, pkg.ErrNotFound):
		return err
	}

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	payloadBytes, err := json.Marshal(emailChangePayload{NewEmail: newEmail})
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to marshal payload")
	}

	err = withinAccount(ctx, uc.txManager, userID, false, func(store IdentityTxStore) error {
		current, err := store.GetUserById(ctx, userID)
		if err != nil {
			return err
		}
		if current.PasswordHash != user.PasswordHash {
			return pkg.Wrap(pkg.ErrConflict, nil, "password changed concurrently")
		}
		user = current
		return replaceAuthTokenWithin(ctx, store, identitymodel.CreateAuthTokenParams{
			ID:        uuid.New(),
			UserID:    userID,
			TokenType: identitymodel.AuthTokenTypeEmailChange,
			TokenHash: tokenHash,
			Payload:   payloadBytes,
			ExpiresAt: time.Now().Add(emailChangeTokenTTL),
		})
	})
	if err != nil {
		return err
	}

	if err := uc.emailService.SendEmailChangeVerification(ctx, newEmail, user.Username, token); err != nil {
		slog.ErrorContext(ctx, "failed to send email change verification", "error", err, "email", newEmail)
	}

	if user.Email != nil && *user.Email != "" {
		if err := uc.emailService.SendEmailChangeAlert(ctx, *user.Email, user.Username, newEmail); err != nil {
			slog.ErrorContext(ctx, "failed to send email change alert", "error", err, "old_email", *user.Email)
		}
	}

	return nil
}

func (uc *AuthUseCase) ConfirmEmailChange(ctx context.Context, token string) error {
	err := uc.txManager.Within(ctx, func(store IdentityTxStore) error {
		return uc.confirmEmailChangeWithin(ctx, store, token)
	})
	return err
}

func (uc *AuthUseCase) confirmEmailChangeWithin(ctx context.Context, store IdentityTxStore, token string) error {
	tokenHash := hashToken(token)

	tok, err := store.GetAuthTokenByHash(ctx, tokenHash, identitymodel.AuthTokenTypeEmailChange)
	if err != nil {
		return pkg.OnNotFound(err, pkg.ErrInvalid, "invalid or expired email change token")
	}

	if time.Now().After(tok.ExpiresAt) {
		if err := store.DeleteAuthToken(ctx, tok.ID); err != nil {
			return err
		}
		return pkg.Wrap(pkg.ErrInvalid, nil, "email change token has expired")
	}

	var payload emailChangePayload
	if err := json.Unmarshal(tok.Payload, &payload); err != nil || payload.NewEmail == "" {
		return pkg.Wrap(pkg.ErrInternal, err, "corrupted token payload")
	}

	err = store.UpdateUserEmail(ctx, tok.UserID, payload.NewEmail, true)
	if err != nil {
		return err
	}

	if err := store.DeleteAuthToken(ctx, tok.ID); err != nil {
		return err
	}

	return nil
}

func replaceAuthToken(ctx context.Context, transactions IdentityTransactions, params identitymodel.CreateAuthTokenParams) error {
	return transactions.Within(ctx, func(store IdentityTxStore) error {
		if err := store.LockUser(ctx, params.UserID); err != nil {
			return err
		}
		return replaceAuthTokenWithin(ctx, store, params)
	})
}
func replaceAuthTokenWithin(ctx context.Context, store IdentityTxStore, params identitymodel.CreateAuthTokenParams) error {
	if err := store.DeleteAuthTokensByUserIdAndType(ctx, params.UserID, params.TokenType); err != nil {
		return err
	}
	return store.CreateAuthToken(ctx, params)
}

// An expired session is rejected either way; a failed delete only leaves a dead row behind.
func (uc *AuthUseCase) deleteExpiredSession(ctx context.Context, sessionID uuid.UUID) {
	if err := uc.authRepo.DeleteSession(ctx, sessionID); err != nil {
		slog.WarnContext(ctx, "delete expired session", "error", err)
	}
}
