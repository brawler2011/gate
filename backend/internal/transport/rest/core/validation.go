package core

import (
	"fmt"
	"unicode/utf8"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

func isLengthBetween(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return length >= min && length <= max
}

const (
	minPage         = 1
	minPageSize     = 1
	maxPageSize     = 20
	maxSearchLength = 50
	maxArchiveSize  = 10 * 1024 * 1024 // 10 MB
)

func validateCreateContestParams(params corev1.CreateContestParams) error {
	if params.Title == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "empty title")
	}

	titleLength := utf8.RuneCountInString(params.Title)
	if titleLength < 3 || titleLength > 64 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "title must be between 3 and 64 characters")
	}

	return nil
}

func publicOrPrivate(s string) bool {
	return s == "private" || s == "public"
}

func checkScope(scope string) bool {
	return scope == "participant" || scope == "moderator" || scope == "owner"
}

func checkLength(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return length >= min && length <= max
}

func validateUpdateContestRequest(params corev1.UpdateContestRequestModel) error {
	if params.Title != nil && !checkLength(*params.Title, 3, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "title must be between 3 and 64 characters")
	}

	if params.Description != nil && !checkLength(*params.Description, 0, 2048) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "description length must be less than 2048 characters")
	}

	if params.Visibility != nil && !publicOrPrivate(*params.Visibility) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid visibility value")
	}

	if params.MonitorScope != nil && !checkScope(*params.MonitorScope) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid monitor scope value")
	}

	if params.SubmissionsListScope != nil && !checkScope(*params.SubmissionsListScope) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid submissions list scope value")
	}

	if params.SubmissionsReviewScope != nil && !checkScope(*params.SubmissionsReviewScope) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid submissions review scope value")
	}

	return nil
}

func validateListContestsParams(page, pageSize int32, search *string) error {
	if page < 1 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page must be greater than 0")
	}

	if pageSize < 1 || pageSize > 100 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page size must be between 1 and 100")
	}

	if search != nil && !checkLength(*search, 0, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "search length must be less than 64 characters")
	}

	return nil
}

const (
	maxSolutionSize int64 = 10 * 1024 * 1024 // 10 MB
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

// Organizations validation

func validateListOrganizationsParams(page, pageSize int32, search *string) error {
	if page < 1 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page must be greater than 0")
	}

	if pageSize < 1 || pageSize > 100 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page size must be between 1 and 100")
	}

	if search != nil && !checkLength(*search, 0, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "search length must be less than 64 characters")
	}

	return nil
}

func validateCreateOrganizationParams(name string) error {
	if !checkLength(name, 3, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "name must be between 3 and 64 characters")
	}

	return nil
}

func validateUpdateOrganizationRequest(params corev1.UpdateOrganizationRequestModel) error {
	if params.Name != nil && !checkLength(*params.Name, 3, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "name must be between 3 and 64 characters")
	}

	if params.Description != nil && !checkLength(*params.Description, 0, 2048) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "description length must be less than 2048 characters")
	}

	return nil
}

func validateOrganizationRole(role string) bool {
	return role == string(models.OrgRoleOwner) ||
		role == string(models.OrgRoleAdmin) ||
		role == string(models.OrgRoleMember)
}

// Teams validation

func validateListTeamsParams(page, pageSize int32, search *string) error {
	if page < 1 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page must be greater than 0")
	}

	if pageSize < 1 || pageSize > 100 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page size must be between 1 and 100")
	}

	if search != nil && !checkLength(*search, 0, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "search length must be less than 64 characters")
	}

	return nil
}

func validateCreateTeamRequest(name string, organizationID uuid.UUID) error {
	if !checkLength(name, 3, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "name must be between 3 and 64 characters")
	}

	if organizationID == uuid.Nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "organization_id is required")
	}

	return nil
}

func validateUpdateTeamRequest(params corev1.UpdateTeamRequestModel) error {
	if params.Name != nil && !checkLength(*params.Name, 3, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "name must be between 3 and 64 characters")
	}

	if params.Description != nil && !checkLength(*params.Description, 0, 2048) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "description length must be less than 2048 characters")
	}

	return nil
}