package identity

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/email"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UsersUseCase struct {
	usersRepo UsersRepo
	authRepo  AuthRepo

	txManager    IdentityTransactions
	emailService email.EmailService
}

func NewUsersUseCase(
	repo UsersRepo,
	txManager IdentityTransactions,
	authRepo AuthRepo,
	emailService email.EmailService,
) *UsersUseCase {
	if emailService == nil {
		emailService = email.NewEmailService("local", "", "", "")
	}

	return &UsersUseCase{
		usersRepo: repo,
		authRepo:  authRepo,

		txManager:    txManager,
		emailService: emailService,
	}
}

func (u *UsersUseCase) CreateUser(ctx context.Context, input identitymodel.CreateUserInput) (uuid.UUID, error) {
	id := uuid.New()

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}

	params := identitymodel.CreateUserParams{
		Id:              id,
		Username:        input.Username,
		Role:            accessmodel.UserRole(input.Role),
		PasswordHash:    string(hashed),
		Email:           input.Email,
		AvatarUrl:       input.AvatarUrl,
		ExpiresAt:       input.ExpiresAt,
		IsEmailVerified: true, // Created by admin or system, verified by default
	}

	err = u.usersRepo.CreateUser(ctx, params)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (u *UsersUseCase) GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	return u.usersRepo.GetUserById(ctx, id)
}

func (u *UsersUseCase) GetUserByUsername(ctx context.Context, username string) (accessmodel.User, error) {
	username = strings.TrimPrefix(username, "@")
	return u.usersRepo.GetUserByUsername(ctx, username)
}

func (u *UsersUseCase) UpdateUser(ctx context.Context, input identitymodel.UpdateUserInput) error {
	return u.usersRepo.UpdateUser(ctx, input)
}

func (u *UsersUseCase) ClaimTemporaryUser(ctx context.Context, caller accessmodel.User, input identitymodel.ClaimTemporaryUserInput) (identitymodel.ClaimTemporaryUserResult, error) {
	if caller.IsGuest() {
		return identitymodel.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}
	if caller.IsExpired() {
		return identitymodel.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.NoPermission, nil, "permanent account has expired")
	}

	tempUser, err := u.usersRepo.GetUserByUsername(ctx, input.Username)
	if err != nil {
		return identitymodel.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrBadInput, err, "temporary user not found")
	}

	// Must be a temporary user
	if !tempUser.IsTemporary() {
		return identitymodel.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrBadInput, nil, "target account is not a temporary account")
	}

	// Must not be claimed already
	if tempUser.IsClaimed() {
		return identitymodel.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrConflict, nil, "temporary account has already been claimed")
	}

	// Cannot claim oneself
	if tempUser.Id == caller.Id {
		return identitymodel.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrBadInput, nil, "cannot claim own account")
	}

	// Verify temporary user password
	err = bcrypt.CompareHashAndPassword([]byte(tempUser.PasswordHash), []byte(input.Password))
	if err != nil {
		return identitymodel.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrUnauthenticated, err, "invalid credentials for temporary account")
	}

	var grantedContests []uuid.UUID

	err = u.txManager.Within(ctx, func(tx IdentityTxStore) error {
		txCtx := ctx
		txUsersRepo := tx
		txContestsRepo := tx

		now := time.Now()
		err := txUsersRepo.ClaimTemporaryUser(txCtx, tempUser.Id, caller.Id, now)
		if err != nil {
			return err
		}

		// Find contests of tempUser and grant access to caller
		memberships, err := txContestsRepo.ListUserContestMemberships(txCtx, tempUser.Id)
		if err != nil {
			return err
		}

		for _, m := range memberships {
			err = txContestsRepo.AddContestMemberIfNotExists(txCtx, m.ContestID, caller.Id, string(contestsmodel.ContestRoleParticipant))
			if err != nil {
				return err
			}
			grantedContests = append(grantedContests, m.ContestID)
		}

		return nil
	})

	if err != nil {
		return identitymodel.ClaimTemporaryUserResult{}, err
	}

	return identitymodel.ClaimTemporaryUserResult{
		ClaimedUserID:   tempUser.Id,
		ClaimedUsername: tempUser.Username,
		ContestsGranted: grantedContests,
	}, nil
}

func (u *UsersUseCase) AdminSetPassword(ctx context.Context, username, newPassword string) error {
	if err := identitymodel.PasswordValidate(newPassword); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid password")
	}

	user, err := u.GetUserByUsername(ctx, username)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user not found")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}

	err = u.usersRepo.UpdateUserPassword(ctx, user.Id, string(hashed))
	if err != nil {
		return err
	}

	if u.authRepo != nil {
		_ = u.authRepo.DeleteSessionsByUserID(ctx, user.Id)
	}

	return nil
}

func (u *UsersUseCase) AdminSendPasswordReset(ctx context.Context, username string) error {
	user, err := u.GetUserByUsername(ctx, username)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user not found")
	}

	if user.Email == nil || *user.Email == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "user has no email address")
	}

	if u.authRepo != nil {
		_ = u.authRepo.DeleteAuthTokensByUserIdAndType(ctx, user.Id, identitymodel.AuthTokenTypePasswordReset)
	}

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	if u.authRepo != nil {
		err = u.authRepo.CreateAuthToken(ctx, identitymodel.CreateAuthTokenParams{
			ID:        uuid.New(),
			UserID:    user.Id,
			TokenType: identitymodel.AuthTokenTypePasswordReset,
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(passwordResetTokenTTL),
		})
		if err != nil {
			return err
		}
	}

	return u.emailService.SendPasswordResetEmail(ctx, *user.Email, user.Username, token)
}

func (u *UsersUseCase) AdminChangeEmail(ctx context.Context, username, newEmail string, withConfirmation bool) error {
	if err := identitymodel.EmailValidate(newEmail); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid email")
	}

	user, err := u.GetUserByUsername(ctx, username)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user not found")
	}

	if existing, err := u.usersRepo.GetUserByUsernameOrEmail(ctx, newEmail); err == nil && existing.Id != user.Id {
		return pkg.Wrap(pkg.ErrConflict, nil, "email already in use by another user")
	}

	if !withConfirmation {
		return u.usersRepo.UpdateUserEmail(ctx, user.Id, newEmail, true)
	}

	// With confirmation
	if u.authRepo != nil {
		_ = u.authRepo.DeleteAuthTokensByUserIdAndType(ctx, user.Id, identitymodel.AuthTokenTypeEmailChange)
	}

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	payloadBytes, err := json.Marshal(emailChangePayload{NewEmail: newEmail})
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to marshal payload")
	}

	if u.authRepo != nil {
		err = u.authRepo.CreateAuthToken(ctx, identitymodel.CreateAuthTokenParams{
			ID:        uuid.New(),
			UserID:    user.Id,
			TokenType: identitymodel.AuthTokenTypeEmailChange,
			TokenHash: tokenHash,
			Payload:   payloadBytes,
			ExpiresAt: time.Now().Add(emailChangeTokenTTL),
		})
		if err != nil {
			return err
		}
	}

	if err := u.emailService.SendEmailChangeVerification(ctx, newEmail, user.Username, token); err != nil {
		slog.Error("failed to send email change verification", "error", err, "email", newEmail)
	}

	if user.Email != nil && *user.Email != "" {
		if err := u.emailService.SendEmailChangeAlert(ctx, *user.Email, user.Username, newEmail); err != nil {
			slog.Error("failed to send email change alert", "error", err, "old_email", *user.Email)
		}
	}

	return nil
}

func (u *UsersUseCase) AdminResendVerification(ctx context.Context, username string) error {
	user, err := u.GetUserByUsername(ctx, username)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user not found")
	}

	if user.IsEmailVerified {
		return nil
	}

	if user.Email == nil || *user.Email == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "user has no email address")
	}

	if u.authRepo != nil {
		_ = u.authRepo.DeleteAuthTokensByUserIdAndType(ctx, user.Id, identitymodel.AuthTokenTypeEmailVerification)
	}

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	if u.authRepo != nil {
		err = u.authRepo.CreateAuthToken(ctx, identitymodel.CreateAuthTokenParams{
			ID:        uuid.New(),
			UserID:    user.Id,
			TokenType: identitymodel.AuthTokenTypeEmailVerification,
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(verificationTokenTTL),
		})
		if err != nil {
			return err
		}
	}

	return u.emailService.SendVerificationEmail(ctx, *user.Email, user.Username, token)
}
