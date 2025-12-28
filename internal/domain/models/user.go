package models

import (
	"errors"
	"time"

	"github.com/gate149/core/pkg"
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
	Id       uuid.UUID
	Username string
	Role     UserRole
	KratosId uuid.UUID
	Email    string
	Name     string
	Surname  string
	Bio      string
	ImgId    *uuid.UUID
}

func UsernameValidate(username string) error {
	if !pkg.IsLengthBetween(username, 3, 70) {
		return errors.New("username must be between 3 and 70 characters")
	}
	return nil
}

func EmailValidate(email string) error {
	if !pkg.IsEmail(email) {
		return errors.New("email must be a valid email address")
	}
	return nil
}

func NameValidate(name string) error {
	if !pkg.IsLengthBetween(name, 0, 70) {
		return errors.New("name must be at most 70 characters")
	}
	return nil
}

func SurnameValidate(surname string) error {
	if !pkg.IsLengthBetween(surname, 0, 70) {
		return errors.New("surname must be at most 70 characters")
	}
	return nil
}

func BioValidate(bio string) error {
	if !pkg.IsLengthBetween(bio, 0, 500) {
		return errors.New("bio must be at most 500 characters")
	}
	return nil
}

func (p CreateUserParams) Validate() error {
	errs := []error{
		UsernameValidate(p.Username),
		UserRoleValidate(p.Role),
		EmailValidate(p.Email),
		NameValidate(p.Name),
		SurnameValidate(p.Surname),
		BioValidate(p.Bio),
	}

	return errors.Join(errs...)
}

type CreateUserInput struct {
	Username string
	Role     string
	KratosId uuid.UUID
	Email    string
	Name     string
	Surname  string
	Bio      string
	Image    *string
}

type UsersListFilter struct {
	Page     int32
	PageSize int32
	Search   string
	Role     string
}

type UpdateUserParams struct {
	Id       uuid.UUID
	Username *string
	Role     *UserRole
	Email    *string
	Name     *string
	Surname  *string
	Bio      *string
	ImgId    *uuid.UUID
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
	if p.Name != nil {
		errs = append(errs, NameValidate(*p.Name))
	}
	if p.Surname != nil {
		errs = append(errs, SurnameValidate(*p.Surname))
	}
	if p.Bio != nil {
		errs = append(errs, BioValidate(*p.Bio))
	}

	return errors.Join(errs...)
}

type UpdateUserInput struct {
	Id       uuid.UUID
	Username *string
	Role     *string
	Email    *string
	Name     *string
	Surname  *string
	Bio      *string
	ImgId    *uuid.UUID
}

type User struct {
	Id        uuid.UUID
	Username  string
	Role      UserRole
	KratosID  uuid.UUID
	Email     string
	Name      string
	Surname   string
	Bio       string
	ImgId     *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

var Guest = User{
	Id:       uuid.Nil,
	Role:     UserRoleGuest,
	KratosID: uuid.Nil,
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
