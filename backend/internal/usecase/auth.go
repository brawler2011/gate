package usecase

import (
	"context"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionHardLifetime = 7 * 24 * time.Hour // 7 days
	sessionSoftLifetime = 40 * time.Minute   // 40 minutes
)

type AuthUseCase struct {
	usersRepo interfaces.UsersRepo
	authRepo  interfaces.AuthRepo
	txManager interfaces.Transactor
}

func NewAuthUseCase(
	usersRepo interfaces.UsersRepo,
	authRepo interfaces.AuthRepo,
	txManager interfaces.Transactor,
) *AuthUseCase {
	return &AuthUseCase{
		usersRepo: usersRepo,
		authRepo:  authRepo,
		txManager: txManager,
	}
}

func (uc *AuthUseCase) Register(ctx context.Context, username, email, password string) (models.User, uuid.UUID, error) {
	if err := models.UsernameValidate(username); err != nil {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid username")
	}
	if err := models.EmailValidate(email); err != nil {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid email")
	}
	if err := models.PasswordValidate(password); err != nil {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid password")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}

	userID := uuid.New()
	sessionID := uuid.New()
	expiresAt := time.Now().Add(sessionSoftLifetime)

	err = uc.usersRepo.CreateUser(ctx, models.CreateUserParams{
		Id:           userID,
		Username:     username,
		Role:         models.UserRoleUser,
		PasswordHash: string(hashed),
		Email:        email,
	})
	if err != nil {
		return models.User{}, uuid.Nil, err
	}

	err = uc.authRepo.CreateSession(ctx, sessionID, userID, expiresAt)
	if err != nil {
		return models.User{}, uuid.Nil, err
	}

	user, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return models.User{}, uuid.Nil, err
	}

	return user, sessionID, nil
}

func (uc *AuthUseCase) Login(ctx context.Context, identifier, password string) (models.User, uuid.UUID, error) {
	user, err := uc.usersRepo.GetUserByUsernameOrEmail(ctx, identifier)
	if err != nil {
		return models.User{}, uuid.Nil, pkg.Wrap(pkg.ErrUnauthenticated, err, "invalid credentials")
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
		// Log error but don't fail authentication
		// (optional: could fail, but usually we just proceed)
	}

	user, err := uc.usersRepo.GetUserById(ctx, session.UserID)
	if err != nil {
		return models.User{}, pkg.Wrap(pkg.ErrUnauthenticated, err, "user not found")
	}

	return user, nil
}

func (uc *AuthUseCase) CleanupExpiredSessions(ctx context.Context) error {
	cutoff := time.Now().Add(-sessionHardLifetime)
	return uc.authRepo.CleanupExpiredSessions(ctx, cutoff)
}
