package handlers

import (
	"fmt"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/pkg"
)

func badInput(msg string) error {
	return pkg.Wrap(pkg.ErrBadInput, nil, msg)
}

var (
	badPageSize = badInput(
		fmt.Sprintf("page_size parameter must be between %d and %d", minPageSize, maxPageSize),
	)
	badPage = badInput(
		fmt.Sprintf("page parameter must be >= %d", minPage),
	)
	badSearch = badInput(
		fmt.Sprintf("search parameter length must be between 0 and %d characters", maxSearchLength),
	)
	badRole = badInput(
		"role parameter must be either 'user' or 'admin'",
	)
)

func validateGetUsersParams(params corev1.ListUsersParams) (*models.UsersListFilter, error) {
	filter := &models.UsersListFilter{
		Page:     params.Page,
		PageSize: params.PageSize,
		Search:   "",
		Role:     "",
	}

	if params.PageSize < minPageSize || params.PageSize > maxPageSize {
		return nil, badPageSize
	}

	if params.Page < minPage {
		return nil, badPage
	}

	if params.Search != nil {
		if !pkg.IsLengthBetween(*params.Search, 0, maxSearchLength) {
			return nil, badSearch
		}
		filter.Search = *params.Search
	}

	if params.Role != nil {
		if !isUserOrAdmin(*params.Role) {
			return nil, badRole
		}
		filter.Role = *params.Role
	}

	return filter, nil
}

func isUserOrAdmin(role string) bool {
	return role == string(models.UserRoleUser) || role == string(models.UserRoleAdmin)
}
