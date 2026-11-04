package accessmodel

import (
	"errors"
	"time"

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
