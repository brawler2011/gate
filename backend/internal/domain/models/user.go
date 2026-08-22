package models

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type UserRole = string

const (
	UserRoleGuest UserRole = "guest"
	UserRoleAdmin UserRole = "admin"
	UserRoleUser  UserRole = "user"
)

func UserRoleValidate(r UserRole) error {
	if r == UserRoleAdmin || r == UserRoleUser {
		return nil
	}

	return errors.New("role must be one of 'admin' or 'user'")
}

type UsersList struct {
	Users      []User
	Pagination Pagination
}

func (f UsersListFilter) Validate() error {
	errs := make([]error, 0)

	if f.Page < 1 {
		errs = append(errs, errors.New("page must be >= 1"))
	}
	if !pkg.IsBetween(f.PageSize, 1, 20) {
		errs = append(errs, errors.New("page size must be between 1 and 20"))
	}
	if !pkg.IsLengthBetween(f.Search, 0, 70) {
		errs = append(errs, errors.New("search must be at most 70 characters"))
	}
	if f.Role != "" {
		if err := UserRoleValidate(f.Role); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

type CreateUserParams struct {
	Id              uuid.UUID
	Username        string
	Role            UserRole
	PasswordHash    string
	Email           *string
	AvatarUrl       *string
	ExpiresAt       *time.Time
	IsEmailVerified bool
}

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)

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
	if !usernameRegex.MatchString(username) {
		return errors.New("username must be between 3 and 30 characters and contain only letters, numbers, and underscores")
	}
	if _, reserved := reservedUsernames[strings.ToLower(username)]; reserved {
		return errors.New("username is reserved")
	}
	return nil
}

func EmailValidate(email string) error {
	if !pkg.IsEmail(email) {
		return errors.New("email must be a valid email address")
	}
	return nil
}

func PasswordValidate(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

func (p CreateUserParams) Validate() error {
	errs := []error{
		UsernameValidate(p.Username),
		UserRoleValidate(p.Role),
	}
	if p.Email != nil && *p.Email != "" {
		if err := EmailValidate(*p.Email); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
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

type UpdateUserParams struct {
	Id              uuid.UUID
	Username        *string
	Role            *UserRole
	Email           *string
	AvatarUrl       *string
	ExpiresAt       *time.Time
	IsEmailVerified *bool
}

func (p UpdateUserParams) Validate() error {
	errs := make([]error, 0)

	if p.Username != nil {
		errs = append(errs, UsernameValidate(*p.Username))
	}
	if p.Role != nil {
		errs = append(errs, UserRoleValidate(*p.Role))
	}
	if p.Email != nil {
		errs = append(errs, EmailValidate(*p.Email))
	}

	return errors.Join(errs...)
}

type UpdateUserInput struct {
	Id              uuid.UUID
	Username        *string
	Role            *string
	Email           *string
	AvatarUrl       *string
	ExpiresAt       *time.Time
	IsEmailVerified *bool
}

type User struct {
	Id              uuid.UUID
	Username        string
	Role            UserRole
	PasswordHash    string
	Email           *string
	AvatarUrl       *string
	ExpiresAt       *time.Time
	IsEmailVerified bool
	ClaimedByUserID *uuid.UUID
	ClaimedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
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


var Guest = User{
	Id:   uuid.Nil,
	Role: UserRoleGuest,
}

func (u User) IsGuest() bool {
	return u.Role == UserRoleGuest
}

func (u User) IsUser() bool {
	return u.Role == UserRoleUser
}

func (u User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

func (u User) IsExpired() bool {
	return u.ExpiresAt != nil && u.ExpiresAt.Before(time.Now())
}

func (u User) IsTemporary() bool {
	return u.ExpiresAt != nil
}

func (u User) IsClaimed() bool {
	return u.ClaimedByUserID != nil
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

type ClaimedAccountItem struct {
	ID        uuid.UUID
	Username  string
	ClaimedAt time.Time
	ExpiresAt *time.Time
}

type BatchCreateOrganizationUsersInput struct {
	OrgLogin string
	Prefix   string
	Count    int32
	TTLDays  *int32
}

type BatchCreatedUserItem struct {
	ID        uuid.UUID
	Username  string
	Password  string
	ExpiresAt *time.Time
}

type BatchCreateOrganizationUsersResult struct {
	Users []BatchCreatedUserItem
}

