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

	txManager    IdentityTransactions
	emailService email.EmailService
}

func NewUsersUseCase(
	repo UsersRepo,
	txManager IdentityTransactions,
	emailService email.EmailService,
) *UsersUseCase {
	return &UsersUseCase{
		usersRepo: repo,

		txManager:    txManager,
		emailService: emailService,
	}
}

func (u *UsersUseCase) CreateUser(ctx context.Context, input identitymodel.CreateUserInput) (uuid.UUID, error) {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return uuid.Nil, err
	}
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

	err = withinAccount(ctx, u.txManager, uuid.Nil, true, func(store IdentityTxStore) error {
		return store.CreateUser(ctx, params)
	})
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
	if err := accessmodel.RequireSelfOrAdmin(ctx, input.Id); err != nil {
		return err
	}
	if err := input.Validate(); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid user update")
	}
	return withinAccount(ctx, u.txManager, input.Id, input.Role != nil || input.ExpiresAt != nil || input.IsEmailVerified != nil, func(store IdentityTxStore) error {
		return store.UpdateUser(ctx, input)
	})
}

func (u *UsersUseCase) ClaimTemporaryUser(ctx context.Context, caller accessmodel.User, input identitymodel.ClaimTemporaryUserInput) (identitymodel.ClaimTemporaryUserResult, error) {
	if actor := accessmodel.UserFromContext(ctx); actor.IsGuest() || actor.Id != caller.Id {
		return identitymodel.ClaimTemporaryUserResult{}, pkg.ErrUnauthenticated
	}
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
		if err := lockAccounts(ctx, tx, caller.Id, tempUser.Id); err != nil {
			return err
		}
		currentCaller, err := tx.GetUserById(ctx, caller.Id)
		if err != nil {
			return err
		}
		if currentCaller.IsExpired() {
			return pkg.NoPermission
		}
		currentTemp, err := tx.GetUserById(ctx, tempUser.Id)
		if err != nil {
			return err
		}
		if !currentTemp.IsTemporary() || currentTemp.PasswordHash != tempUser.PasswordHash {
			return pkg.ErrUnauthenticated
		}
		if currentTemp.IsClaimed() {
			return pkg.Wrap(pkg.ErrConflict, nil, "temporary account has already been claimed")
		}

		now := time.Now()
		err = tx.ClaimTemporaryUser(ctx, tempUser.Id, caller.Id, now)
		if err != nil {
			return err
		}

		// Find contests of tempUser and grant access to caller
		memberships, err := tx.ListUserContestMemberships(ctx, tempUser.Id)
		if err != nil {
			return err
		}

		for _, m := range memberships {
			err = tx.AddContestMemberIfNotExists(ctx, m.ContestID, caller.Id, string(contestsmodel.ContestRoleParticipant))
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
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return err
	}
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

	return withinAccount(ctx, u.txManager, user.Id, true, func(tx IdentityTxStore) error {
		if err := tx.UpdateUserPassword(ctx, user.Id, string(hashed)); err != nil {
			return err
		}
		return tx.DeleteSessionsByUserID(ctx, user.Id)
	})
}

func (u *UsersUseCase) AdminSendPasswordReset(ctx context.Context, username string) error {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return err
	}
	user, err := u.GetUserByUsername(ctx, username)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user not found")
	}

	if user.Email == nil || *user.Email == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "user has no email address")
	}

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	err = u.replaceAdminAuthToken(ctx, identitymodel.CreateAuthTokenParams{
		ID:        uuid.New(),
		UserID:    user.Id,
		TokenType: identitymodel.AuthTokenTypePasswordReset,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(passwordResetTokenTTL),
	})
	if err != nil {
		return err
	}

	return u.emailService.SendPasswordResetEmail(ctx, *user.Email, user.Username, token)
}

func (u *UsersUseCase) AdminChangeEmail(ctx context.Context, username, newEmail string, withConfirmation bool) error {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return err
	}
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
		return withinAccount(ctx, u.txManager, user.Id, true, func(store IdentityTxStore) error {
			return store.UpdateUserEmail(ctx, user.Id, newEmail, true)
		})
	}

	// With confirmation

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	payloadBytes, err := json.Marshal(emailChangePayload{NewEmail: newEmail})
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to marshal payload")
	}

	err = u.replaceAdminAuthToken(ctx, identitymodel.CreateAuthTokenParams{
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
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return err
	}
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

	token, tokenHash, err := generateSecureToken()
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to generate token")
	}

	err = u.replaceAdminAuthToken(ctx, identitymodel.CreateAuthTokenParams{
		ID:        uuid.New(),
		UserID:    user.Id,
		TokenType: identitymodel.AuthTokenTypeEmailVerification,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(verificationTokenTTL),
	})
	if err != nil {
		return err
	}

	return u.emailService.SendVerificationEmail(ctx, *user.Email, user.Username, token)
}

func (u *UsersUseCase) replaceAdminAuthToken(ctx context.Context, params identitymodel.CreateAuthTokenParams) error {
	return withinAccount(ctx, u.txManager, params.UserID, true, func(store IdentityTxStore) error {
		return replaceAuthTokenWithin(ctx, store, params)
	})
}
