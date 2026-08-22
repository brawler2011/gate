package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/email"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionHardLifetime = 7 * 24 * time.Hour // 7 days
	sessionSoftLifetime = 40 * time.Minute   // 40 minutes

	verificationTokenTTL = 24 * time.Hour
	passwordResetTokenTTL = 1 * time.Hour
	emailChangeTokenTTL   = 24 * time.Hour
)

type AuthUseCase struct {
	usersRepo    interfaces.UsersRepo
	authRepo     interfaces.AuthRepo
	txManager    interfaces.Transactor
	emailService email.EmailService
}

func NewAuthUseCase(
	usersRepo interfaces.UsersRepo,
	authRepo interfaces.AuthRepo,
	txManager interfaces.Transactor,
	emailService ...email.EmailService,
) *AuthUseCase {
	var es email.EmailService
	if len(emailService) > 0 && emailService[0] != nil {
		es = emailService[0]
	} else {
		es = email.NewEmailService("", "", "")
	}

	return &AuthUseCase{
		usersRepo:    usersRepo,
		authRepo:     authRepo,
		txManager:    txManager,
		emailService: es,
	}
}

func generateSecureToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
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

func (uc *AuthUseCase) Register(ctx context.Context, username, emailAddr, password string) (models.User, error) {
	if err := models.UsernameValidate(username); err != nil {
		return models.User{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid username")
	}
	if err := models.EmailValidate(emailAddr); err != nil {
		return models.User{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid email")
	}
	if err := models.PasswordValidate(password); err != nil {
		return models.User{}, pkg.Wrap(pkg.ErrBadInput, err, "invalid password")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}

	userID := uuid.New()

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return models.User{}, pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	err = uc.usersRepo.CreateUser(ctx, models.CreateUserParams{
		Id:              userID,
		Username:        username,
		Role:            models.UserRoleUser,
		PasswordHash:    string(hashed),
		Email:           &emailAddr,
		IsEmailVerified: false,
	})
	if err != nil {
		return models.User{}, err
	}

	err = uc.authRepo.CreateAuthToken(ctx, models.CreateAuthTokenParams{
		ID:        uuid.New(),
		UserID:    userID,
		TokenType: models.AuthTokenTypeEmailVerification,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(verificationTokenTTL),
	})
	if err != nil {
		slog.Error("failed to create email verification token", "error", err, "user_id", userID)
	}

	// Send verification email
	if err := uc.emailService.SendVerificationEmail(ctx, emailAddr, username, token); err != nil {
		slog.Error("failed to send verification email", "error", err, "email", emailAddr)
	}

	user, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return models.User{}, err
	}

	return user, nil
}

func (uc *AuthUseCase) Login(ctx context.Context, identifier, password string) (models.User, uuid.UUID, error) {
	user, err := uc.usersRepo.GetUserByUsernameOrEmail(ctx, identifier)
	if err != nil {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrUnauthenticated, err, "invalid credentials")
	}

	if user.IsExpired() {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "account has expired")
	}

	if !user.IsEmailVerified {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrBadInput, errors.New("email not verified"), "email not verified")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrUnauthenticated, err, "invalid credentials")
	}

	sessionID := uuid.New()
	expiresAt := time.Now().Add(sessionSoftLifetime)

	err = uc.authRepo.CreateSession(ctx, sessionID, user.Id, expiresAt)
	if err != nil {
		return models.User{}, uuid.Nil, err
	}

	return user, sessionID, nil
}

func (uc *AuthUseCase) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return uc.authRepo.DeleteSession(ctx, sessionID)
}

func (uc *AuthUseCase) Authenticate(ctx context.Context, sessionID uuid.UUID) (models.User, error) {
	session, err := uc.authRepo.GetSession(ctx, sessionID)
	if err != nil {
		return models.User{}, pkg.Wrap(pkg.ErrUnauthenticated, err, "invalid session")
	}

	// 1. Hard lifetime check: expires 7 days after creation
	hardExpiry := session.CreatedAt.Add(sessionHardLifetime)
	if time.Now().After(hardExpiry) {
		_ = uc.authRepo.DeleteSession(ctx, sessionID)
		return models.User{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "session expired (hard limit)")
	}

	// 2. Soft lifetime check: expires if user was inactive for 40 minutes (session.ExpiresAt is the soft expiry)
	if session.IsExpired() {
		_ = uc.authRepo.DeleteSession(ctx, sessionID)
		return models.User{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "session expired (soft limit)")
	}

	// Update session expiry in database (sliding session)
	newExpiry := time.Now().Add(sessionSoftLifetime)
	if newExpiry.After(hardExpiry) {
		newExpiry = hardExpiry
	}
	err = uc.authRepo.UpdateSessionExpiry(ctx, sessionID, newExpiry)
	if err != nil {
		slog.Debug("failed to update session expiry", "error", err)
	}

	user, err := uc.usersRepo.GetUserById(ctx, session.UserID)
	if err != nil {
		return models.User{}, pkg.Wrap(pkg.ErrUnauthenticated, err, "user not found")
	}

	if user.IsExpired() {
		_ = uc.authRepo.DeleteSession(ctx, sessionID)
		return models.User{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "account has expired")
	}

	return user, nil
}

func (uc *AuthUseCase) CleanupExpiredSessions(ctx context.Context) error {
	cutoff := time.Now().Add(-sessionHardLifetime)
	return uc.authRepo.CleanupExpiredSessions(ctx, cutoff)
}

func (uc *AuthUseCase) VerifyEmail(ctx context.Context, token string) (models.User, uuid.UUID, error) {
	tokenHash := hashToken(token)

	tok, err := uc.authRepo.GetAuthTokenByHash(ctx, tokenHash, models.AuthTokenTypeEmailVerification)
	if err != nil {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid or expired verification link")
	}

	if time.Now().After(tok.ExpiresAt) {
		_ = uc.authRepo.DeleteAuthToken(ctx, tok.ID)
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "verification link has expired")
	}

	err = uc.usersRepo.SetUserEmailVerified(ctx, tok.UserID, true)
	if err != nil {
		return models.User{}, uuid.Nil, err
	}

	_ = uc.authRepo.DeleteAuthToken(ctx, tok.ID)

	user, err := uc.usersRepo.GetUserById(ctx, tok.UserID)
	if err != nil {
		return models.User{}, uuid.Nil, err
	}

	sessionID := uuid.New()
	expiresAt := time.Now().Add(sessionSoftLifetime)
	err = uc.authRepo.CreateSession(ctx, sessionID, user.Id, expiresAt)
	if err != nil {
		return user, uuid.Nil, nil
	}

	return user, sessionID, nil
}

func (uc *AuthUseCase) ResendVerificationEmail(ctx context.Context, identifier string) error {
	user, err := uc.usersRepo.GetUserByUsernameOrEmail(ctx, identifier)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user not found")
	}

	if user.IsEmailVerified {
		return nil
	}

	if user.Email == nil || *user.Email == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "user has no email associated")
	}

	_ = uc.authRepo.DeleteAuthTokensByUserIdAndType(ctx, user.Id, models.AuthTokenTypeEmailVerification)

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	err = uc.authRepo.CreateAuthToken(ctx, models.CreateAuthTokenParams{
		ID:        uuid.New(),
		UserID:    user.Id,
		TokenType: models.AuthTokenTypeEmailVerification,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(verificationTokenTTL),
	})
	if err != nil {
		return err
	}

	return uc.emailService.SendVerificationEmail(ctx, *user.Email, user.Username, token)
}

func (uc *AuthUseCase) ForgotPassword(ctx context.Context, identifier string) error {
	user, err := uc.usersRepo.GetUserByUsernameOrEmail(ctx, identifier)
	if err != nil {
		// Do not reveal if user does not exist
		return nil
	}

	if user.Email == nil || *user.Email == "" {
		return nil
	}

	_ = uc.authRepo.DeleteAuthTokensByUserIdAndType(ctx, user.Id, models.AuthTokenTypePasswordReset)

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	err = uc.authRepo.CreateAuthToken(ctx, models.CreateAuthTokenParams{
		ID:        uuid.New(),
		UserID:    user.Id,
		TokenType: models.AuthTokenTypePasswordReset,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(passwordResetTokenTTL),
	})
	if err != nil {
		return err
	}

	return uc.emailService.SendPasswordResetEmail(ctx, *user.Email, user.Username, token)
}

func (uc *AuthUseCase) ResetPassword(ctx context.Context, token, newPassword string) error {
	if err := models.PasswordValidate(newPassword); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid password")
	}

	tokenHash := hashToken(token)

	tok, err := uc.authRepo.GetAuthTokenByHash(ctx, tokenHash, models.AuthTokenTypePasswordReset)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid or expired reset token")
	}

	if time.Now().After(tok.ExpiresAt) {
		_ = uc.authRepo.DeleteAuthToken(ctx, tok.ID)
		return pkg.Wrap(pkg.ErrBadInput, nil, "reset token has expired")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}

	err = uc.usersRepo.UpdateUserPassword(ctx, tok.UserID, string(hashed))
	if err != nil {
		return err
	}

	_ = uc.authRepo.DeleteSessionsByUserID(ctx, tok.UserID)
	_ = uc.authRepo.DeleteAuthToken(ctx, tok.ID)

	return nil
}

func (uc *AuthUseCase) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) (uuid.UUID, error) {
	if err := models.PasswordValidate(newPassword); err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid password")
	}

	user, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrUnauthenticated, err, "user not found")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword))
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "incorrect current password")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}

	err = uc.usersRepo.UpdateUserPassword(ctx, userID, string(hashed))
	if err != nil {
		return uuid.Nil, err
	}

	_ = uc.authRepo.DeleteSessionsByUserID(ctx, userID)

	newSessionID := uuid.New()
	expiresAt := time.Now().Add(sessionSoftLifetime)
	err = uc.authRepo.CreateSession(ctx, newSessionID, userID, expiresAt)
	if err != nil {
		return uuid.Nil, err
	}

	return newSessionID, nil
}

type emailChangePayload struct {
	NewEmail string `json:"new_email"`
}

func (uc *AuthUseCase) RequestEmailChange(ctx context.Context, userID uuid.UUID, password, newEmail string) error {
	if err := models.EmailValidate(newEmail); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid email")
	}

	user, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return pkg.Wrap(pkg.ErrUnauthenticated, err, "user not found")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "incorrect password")
	}

	if existing, err := uc.usersRepo.GetUserByUsernameOrEmail(ctx, newEmail); err == nil && existing.Id != userID {
		return pkg.Wrap(pkg.ErrConflict, nil, "email already in use")
	}

	_ = uc.authRepo.DeleteAuthTokensByUserIdAndType(ctx, userID, models.AuthTokenTypeEmailChange)

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	payloadBytes, err := json.Marshal(emailChangePayload{NewEmail: newEmail})
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to marshal payload")
	}

	err = uc.authRepo.CreateAuthToken(ctx, models.CreateAuthTokenParams{
		ID:        uuid.New(),
		UserID:    userID,
		TokenType: models.AuthTokenTypeEmailChange,
		TokenHash: tokenHash,
		Payload:   payloadBytes,
		ExpiresAt: time.Now().Add(emailChangeTokenTTL),
	})
	if err != nil {
		return err
	}

	if err := uc.emailService.SendEmailChangeVerification(ctx, newEmail, user.Username, token); err != nil {
		slog.Error("failed to send email change verification", "error", err, "email", newEmail)
	}

	if user.Email != nil && *user.Email != "" {
		if err := uc.emailService.SendEmailChangeAlert(ctx, *user.Email, user.Username, newEmail); err != nil {
			slog.Error("failed to send email change alert", "error", err, "old_email", *user.Email)
		}
	}

	return nil
}

func (uc *AuthUseCase) ConfirmEmailChange(ctx context.Context, token string) error {
	tokenHash := hashToken(token)

	tok, err := uc.authRepo.GetAuthTokenByHash(ctx, tokenHash, models.AuthTokenTypeEmailChange)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid or expired email change token")
	}

	if time.Now().After(tok.ExpiresAt) {
		_ = uc.authRepo.DeleteAuthToken(ctx, tok.ID)
		return pkg.Wrap(pkg.ErrBadInput, nil, "email change token has expired")
	}

	var payload emailChangePayload
	if err := json.Unmarshal(tok.Payload, &payload); err != nil || payload.NewEmail == "" {
		return pkg.Wrap(pkg.ErrInternal, err, "corrupted token payload")
	}

	err = uc.usersRepo.UpdateUserEmail(ctx, tok.UserID, payload.NewEmail, true)
	if err != nil {
		return err
	}

	_ = uc.authRepo.DeleteAuthToken(ctx, tok.ID)

	return nil
}
