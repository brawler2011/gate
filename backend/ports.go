package main

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/google/uuid"
)

type AuthUC interface {
	Register(ctx context.Context, username, email, password string) (accessmodel.User, error)
	Login(ctx context.Context, identifier, password string) (accessmodel.User, uuid.UUID, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	Authenticate(ctx context.Context, sessionID uuid.UUID) (accessmodel.User, error)
	CleanupExpiredSessions(ctx context.Context) error

	VerifyEmail(ctx context.Context, token string) (accessmodel.User, uuid.UUID, error)
	ResendVerificationEmail(ctx context.Context, identifier string) error
	ForgotPassword(ctx context.Context, identifier string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) (uuid.UUID, error)
	RequestEmailChange(ctx context.Context, userID uuid.UUID, password, newEmail string) error
	ConfirmEmailChange(ctx context.Context, token string) error
}

type UsersUC interface {
	CreateUser(ctx context.Context, input identitymodel.CreateUserInput) (uuid.UUID, error)
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
	GetUserByUsername(ctx context.Context, username string) (accessmodel.User, error)
	UpdateUser(ctx context.Context, input identitymodel.UpdateUserInput) error
	ClaimTemporaryUser(ctx context.Context, caller accessmodel.User, input identitymodel.ClaimTemporaryUserInput) (identitymodel.ClaimTemporaryUserResult, error)
	AdminSetPassword(ctx context.Context, username, newPassword string) error
	AdminSendPasswordReset(ctx context.Context, username string) error
	AdminChangeEmail(ctx context.Context, username, newEmail string, withConfirmation bool) error
	AdminResendVerification(ctx context.Context, username string) error
}
