package models

import (
	"github.com/google/uuid"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type UsersList struct {
	// This struct is likely obsolete if we return []User directly, but kept if needed for pagination wrapper
	// But repo returns *models.UsersList. I should change repo to return ([]User, int64).
	// So I'll remove this.
}

type UsersFilter struct {
	Page     int64
	PageSize int64
	Search   string
	Role     string
}

func (f UsersFilter) Offset() int64 {
	return (f.Page - 1) * f.PageSize
}

type CreateUserInput struct {
	Username string
	Role     string
	KratosId string
}

type CreateUserParams struct {
	Id       uuid.UUID
	Username string
	Role     string
	KratosId string
}
