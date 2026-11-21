package core

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

var (
	contestLoginRegex     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	reservedContestLogins = map[string]struct{}{
		"problems":      {},
		"teams":         {},
		"members":       {},
		"settings":      {},
		"submit":        {},
		"mysubmissions": {},
		"submissions":   {},
		"monitor":       {},
	}
)

func isReservedContestLogin(login string) bool {
	_, isReserved := reservedContestLogins[login]
	return isReserved
}

func validateContestLogin(login string) error {
	if len(login) < 3 || len(login) > 64 {
		return pkg.AtField("/login", pkg.Invalid(pkg.ErrLength, "contest login must be between 3 and 64 characters", map[string]any{"min": 3, "max": 64}))
	}
	if !contestLoginRegex.MatchString(login) {
		return pkg.AtField("/login", pkg.Invalid(pkg.ErrFormat, "contest login must contain only lowercase alphanumeric characters and hyphens, and cannot start or end with a hyphen", nil))
	}
	if isReservedContestLogin(login) {
		return pkg.AtField("/login", pkg.Invalid(pkg.ErrReserved, fmt.Sprintf("contest login '%s' is reserved", login), nil))
	}
	return nil
}

func lengthError(pointer string, min, max int) error {
	field := pointer[1:]
	if min == 0 {
		return pkg.AtField(pointer, pkg.Invalid(pkg.ErrLength, fmt.Sprintf("%s must be at most %d characters", field, max), map[string]any{"max": max}))
	}
	return pkg.AtField(pointer, pkg.Invalid(pkg.ErrLength, fmt.Sprintf("%s must be between %d and %d characters", field, min, max), map[string]any{"min": min, "max": max}))
}

func invalidValue(pointer string) error {
	return pkg.AtField(pointer, pkg.Invalid(pkg.ErrInvalid, "invalid "+pointer[1:]+" value", nil))
}

func validateCreateContestParams(title string, login corev1.OptString) error {
	if title == "" {
		return pkg.AtField("/title", pkg.Invalid(pkg.ErrRequired, "title is required", nil))
	}
	if !checkLength(title, 3, 64) {
		return lengthError("/title", 3, 64)
	}

	if login.IsSet() && login.Value != "" {
		if err := validateContestLogin(login.Value); err != nil {
			return err
		}
	}

	return nil
}

func publicOrPrivate(s string) bool {
	return s == "private" || s == "public"
}

func checkLength(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return length >= min && length <= max
}

func validateUpdateContestRequest(params *corev1.UpdateContestRequestModel) error {
	if params == nil {
		return nil
	}

	if params.Login.IsSet() {
		if err := validateContestLogin(params.Login.Value); err != nil {
			return err
		}
	}

	if params.Title.IsSet() && !checkLength(params.Title.Value, 3, 64) {
		return lengthError("/title", 3, 64)
	}

	if params.Description.IsSet() && !checkLength(params.Description.Value, 0, 2048) {
		return lengthError("/description", 0, 2048)
	}

	if params.Visibility.IsSet() && !publicOrPrivate(params.Visibility.Value) {
		return invalidValue("/visibility")
	}

	if params.StartTime.IsSet() && params.EndTime.IsSet() && !params.EndTime.Null && !params.StartTime.Null {
		if !params.EndTime.Value.After(params.StartTime.Value) {
			return pkg.AtField("/end_time", pkg.Invalid(pkg.ErrEndBeforeStart, "end_time must be after start_time", nil))
		}
	}

	return nil
}

const (
	maxSolutionSize int64 = 10 * 1024 * 1024
)

var (
	orgLoginRegex     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)
	reservedOrgLogins = map[string]struct{}{
		"admin":       {},
		"auth":        {},
		"blog":        {},
		"contests":    {},
		"orgs":        {},
		"problems":    {},
		"submissions": {},
		"api":         {},
		"users":       {},
		"settings":    {},
		"profile":     {},
		"login":       {},
		"register":    {},
		"dashboard":   {},
		"workshop":    {},
		"static":      {},
		"_next":       {},
		"favicon.ico": {},
		"robots.txt":  {},
		"sitemap.xml": {},
	}
)

func validateOrgLogin(login string) error {
	normalized := strings.ToLower(strings.TrimSpace(login))
	if len(normalized) < 3 || len(normalized) > 32 {
		return lengthError("/login", 3, 32)
	}
	if !orgLoginRegex.MatchString(normalized) {
		return pkg.AtField("/login", pkg.Invalid(pkg.ErrFormat, "organization login must contain only lowercase latin letters, numbers, and hyphens, and cannot start or end with a hyphen", nil))
	}
	if _, isReserved := reservedOrgLogins[normalized]; isReserved {
		return pkg.AtField("/login", pkg.Invalid(pkg.ErrReserved, fmt.Sprintf("organization login '%s' is reserved", normalized), nil))
	}
	return nil
}

func validateCreateOrganizationParams(name string) error {
	if !checkLength(name, 3, 64) {
		return lengthError("/name", 3, 64)
	}

	return nil
}

func validateUpdateOrganizationRequest(params *corev1.UpdateOrganizationRequestModel) error {
	if params == nil {
		return nil
	}

	if params.Login.IsSet() {
		if err := validateOrgLogin(params.Login.Value); err != nil {
			return err
		}
	}

	if params.Name.IsSet() && !checkLength(params.Name.Value, 3, 64) {
		return lengthError("/name", 3, 64)
	}

	if params.Description.IsSet() && !checkLength(params.Description.Value, 0, 2048) {
		return lengthError("/description", 0, 2048)
	}

	return nil
}

func validateOrganizationRole(role string) bool {
	return role == string(organizationsmodel.OrgRoleOwner) ||
		role == string(organizationsmodel.OrgRoleAdmin) ||
		role == string(organizationsmodel.OrgRoleMember)
}

func validateCreateTeamRequest(name string, organizationID uuid.UUID) error {
	if !checkLength(name, 3, 64) {
		return lengthError("/name", 3, 64)
	}

	if organizationID == uuid.Nil {
		return pkg.AtField("/organization_id", pkg.Invalid(pkg.ErrRequired, "organization_id is required", nil))
	}

	return nil
}

func validateUpdateTeamRequest(params *corev1.UpdateTeamRequestModel) error {
	if params == nil {
		return nil
	}

	if params.Name.IsSet() && !checkLength(params.Name.Value, 3, 64) {
		return lengthError("/name", 3, 64)
	}

	if params.Description.IsSet() && !checkLength(params.Description.Value, 0, 2048) {
		return lengthError("/description", 0, 2048)
	}

	return nil
}
