package identitymodel

import (
	"errors"
	"regexp"
	"strings"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (f UsersListFilter) Validate() error {
	errs := make([]error, 0)

	if f.Page < 1 {
		errs = append(errs, errors.New("page must be >= 1"))
	}
	if !pkg.IsBetween(f.PageSize, 1, 100) {
		errs = append(errs, errors.New("page size must be between 1 and 100"))
	}
	if !pkg.IsLengthBetween(f.Search, 0, 70) {
		errs = append(errs, errors.New("search must be at most 70 characters"))
	}
	if f.Role != "" {
		if err := accessmodel.UserRoleValidate(f.Role); err != nil {
			errs = append(errs, err)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return pkg.Wrap(pkg.ErrInvalid, err, "invalid users filter")
	}
	return nil
}

type CreateUserParams struct {
	Id              uuid.UUID
	Username        string
	Role            accessmodel.UserRole
	PasswordHash    string
	Email           *string
	AvatarUrl       *string
	ExpiresAt       *time.Time
	IsEmailVerified bool
}

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

var reservedUsernames = map[string]struct{}{
	"admin":       {},
	"api":         {},
	"auth":        {},
	"blog":        {},
	"contests":    {},
	"problems":    {},
	"submissions": {},
	"users":       {},
	"orgs":        {},
	"settings":    {},
	"static":      {},
	"login":       {},
	"register":    {},
	"dashboard":   {},
	"me":          {},
	"health":      {},
	"leaderboard": {},
	"docs":        {},
	"help":        {},
	"about":       {},
	"support":     {},
}

func UsernameValidate(username string) error {
	username = strings.TrimPrefix(username, "@")
	if !pkg.IsLengthBetween(username, 3, 30) {
		return pkg.Invalid(pkg.ErrLength, "username must be between 3 and 30 characters", map[string]any{"min": 3, "max": 30})
	}
	if !usernameRegex.MatchString(username) {
		return pkg.Invalid(pkg.ErrFormat, "username may contain only latin letters, numbers, and underscores", nil)
	}
	if _, reserved := reservedUsernames[strings.ToLower(username)]; reserved {
		return pkg.Invalid(pkg.ErrReserved, "username is reserved", nil)
	}
	return nil
}

func EmailValidate(email string) error {
	if !pkg.IsEmail(email) {
		return pkg.Invalid(pkg.ErrFormat, "email must be a valid email address", nil)
	}
	return nil
}

func PasswordValidate(password string) error {
	if len(password) < 8 {
		return pkg.Invalid(pkg.ErrLength, "password must be at least 8 characters", map[string]any{"min": 8})
	}
	return nil
}

func (p CreateUserParams) Validate() error {
	errs := []error{
		pkg.AtField("/username", UsernameValidate(p.Username)),
		pkg.AtField("/role", accessmodel.UserRoleValidate(p.Role)),
	}
	if p.Email != nil && *p.Email != "" {
		errs = append(errs, pkg.AtField("/email", EmailValidate(*p.Email)))
	}
	return pkg.Fields(errs...)
}

type CreateUserInput struct {
	Username  string
	Role      string
	Password  string
	Email     *string
	AvatarUrl *string
	ExpiresAt *time.Time
}

type UsersListFilter struct {
	Page     int32
	PageSize int32
	Search   string
	Role     string
}

type UpdateUserInput struct {
	Id              uuid.UUID
	Username        *string
	Role            *accessmodel.UserRole
	Email           *string
	AvatarUrl       *string
	ExpiresAt       *time.Time
	IsEmailVerified *bool
}

type UpdateUserParams = UpdateUserInput

func (p UpdateUserInput) Validate() error {
	var errs []error
	if p.Username != nil {
		errs = append(errs, pkg.AtField("/username", UsernameValidate(*p.Username)))
	}
	if p.Role != nil {
		errs = append(errs, pkg.AtField("/role", accessmodel.UserRoleValidate(*p.Role)))
	}
	if p.Email != nil {
		errs = append(errs, pkg.AtField("/email", EmailValidate(*p.Email)))
	}
	return pkg.Fields(errs...)
}

type AuthTokenType = string

//nolint:gosec // token type identifier, not hardcoded credentials
const (
	AuthTokenTypeEmailVerification AuthTokenType = "email_verification"
	AuthTokenTypePasswordReset     AuthTokenType = "password_reset"
	AuthTokenTypeEmailChange       AuthTokenType = "email_change"
)

type AuthToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenType AuthTokenType
	TokenHash string
	Payload   []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

type CreateAuthTokenParams struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenType AuthTokenType
	TokenHash string
	Payload   []byte
	ExpiresAt time.Time
}

type ClaimTemporaryUserInput struct {
	Username string
	Password string
}

type ClaimTemporaryUserResult struct {
	ClaimedUserID   uuid.UUID
	ClaimedUsername string
	ContestsGranted []uuid.UUID
}

type UsersList struct {
	Users      []UserProfile
	Pagination paging.Pagination
}

type ClaimedAccountItem struct {
	ID        uuid.UUID
	Username  string
	ClaimedAt time.Time
	ExpiresAt *time.Time
}

type UserProfile struct {
	Id              uuid.UUID
	Username        string
	Role            accessmodel.UserRole
	Email           *string
	AvatarUrl       *string
	ExpiresAt       *time.Time
	IsEmailVerified bool
	ClaimedByUserID *uuid.UUID
	ClaimedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
