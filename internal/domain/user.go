package domain

import (
	"time"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	KratosID  string    `json:"kratos_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u User) IsAdmin() bool {
	return u.Role == models.RoleAdmin
}

type UsersList struct {
	Users      []User     `json:"users"`
	Pagination Pagination `json:"pagination"`
}
