package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type User struct {
	Id        uuid.UUID `db:"id" json:"Id"`
	Username  string    `db:"username" json:"Username"`
	Role      string    `db:"role" json:"Role"`
	KratosId  *string   `db:"kratos_id" json:"KratosId"`
	CreatedAt time.Time `db:"created_at" json:"CreatedAt"`
	UpdatedAt time.Time `db:"updated_at" json:"UpdatedAt"`
}

func (u User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u User) IsUser() bool {
	return u.Role == RoleUser
}

type UsersList struct {
	Users      []*User
	Pagination *Pagination
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
