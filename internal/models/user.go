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
	Id        uuid.UUID `db:"id"`
	Username  string    `db:"username"`
	Role      string    `db:"role"`
	KratosId  *string   `db:"kratos_id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (u User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u User) IsUser() bool {
	return u.Role == RoleUser
}

type UserCreation struct {
	Id       uuid.UUID
	Username string
	Role     string
	KratosId *string
}

type UsersList struct {
	Users      []*User
	Pagination Pagination
}

type UsersSearch struct {
	Page     int64
	PageSize int64
	Search   string
	Role     string
}
