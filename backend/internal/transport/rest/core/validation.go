package core

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
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
		return pkg.Wrap(pkg.ErrBadInput, nil, "contest login must be between 3 and 64 characters")
	}
	if !contestLoginRegex.MatchString(login) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "contest login must contain only lowercase alphanumeric characters and hyphens, and cannot start or end with a hyphen")
	}
	if isReservedContestLogin(login) {
		return pkg.Wrap(pkg.ErrBadInput, nil, fmt.Sprintf("contest login '%s' is reserved", login))
	}
	return nil
}

func validateCreateContestParams(title string, login corev1.OptString) error {
	if title == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "empty title")
	}

	titleLength := utf8.RuneCountInString(title)
	if titleLength < 3 || titleLength > 64 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "title must be between 3 and 64 characters")
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

func checkScope(scope string) bool {
	return scope == "participant" || scope == "moderator" || scope == "owner"
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
		return pkg.Wrap(pkg.ErrBadInput, nil, "title must be between 3 and 64 characters")
	}

	if params.Description.IsSet() && !checkLength(params.Description.Value, 0, 2048) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "description length must be less than 2048 characters")
	}

	if params.Visibility.IsSet() && !publicOrPrivate(params.Visibility.Value) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid visibility value")
	}

	if params.MonitorScope.IsSet() && !checkScope(params.MonitorScope.Value) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid monitor scope value")
	}

	if params.SubmissionsListScope.IsSet() && !checkScope(params.SubmissionsListScope.Value) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid submissions list scope value")
	}

	if params.SubmissionsReviewScope.IsSet() && !checkScope(params.SubmissionsReviewScope.Value) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid submissions review scope value")
	}

	if params.StartTime.IsSet() && params.EndTime.IsSet() && !params.EndTime.Null && !params.StartTime.Null {
		if !params.EndTime.Value.After(params.StartTime.Value) {
			return pkg.Wrap(pkg.ErrBadInput, nil, "end_time must be after start_time")
		}
	}

	if params.FreezeDurationMinutes.IsSet() && !params.FreezeDurationMinutes.Null && params.FreezeDurationMinutes.Value < 0 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "freeze_duration_minutes must be non-negative")
	}

	if params.FreezeStatus.IsSet() {
		status := string(params.FreezeStatus.Value)
		if status != models.FreezeStatusAuto && status != models.FreezeStatusFrozen && status != models.FreezeStatusUnfrozen {
			return pkg.Wrap(pkg.ErrBadInput, nil, "invalid freeze_status value")
		}
	}

	return nil
}

const (
	maxSolutionSize int64 = 10 * 1024 * 1024 // 10 MB
)

// Organizations validation

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
	if strings.HasPrefix(normalized, "@") {
		return pkg.Wrap(pkg.ErrBadInput, nil, "organization login cannot start with '@'")
	}
	if len(normalized) < 3 || len(normalized) > 32 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "organization login must be between 3 and 32 characters")
	}
	if !orgLoginRegex.MatchString(normalized) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "organization login must contain only lowercase latin letters, numbers, and hyphens, and cannot start or end with a hyphen")
	}
	if _, isReserved := reservedOrgLogins[normalized]; isReserved {
		return pkg.Wrap(pkg.ErrBadInput, nil, fmt.Sprintf("organization login '%s' is reserved", normalized))
	}
	return nil
}

func validateCreateOrganizationParams(name string) error {
	if !checkLength(name, 3, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "name must be between 3 and 64 characters")
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
		return pkg.Wrap(pkg.ErrBadInput, nil, "name must be between 3 and 64 characters")
	}

	if params.Description.IsSet() && !checkLength(params.Description.Value, 0, 2048) {
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

func validateCreateTeamRequest(name string, organizationID uuid.UUID) error {
	if !checkLength(name, 3, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "name must be between 3 and 64 characters")
	}

	if organizationID == uuid.Nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "organization_id is required")
	}

	return nil
}

func validateUpdateTeamRequest(params *corev1.UpdateTeamRequestModel) error {
	if params == nil {
		return nil
	}

	if params.Name.IsSet() && !checkLength(params.Name.Value, 3, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "name must be between 3 and 64 characters")
	}

	if params.Description.IsSet() && !checkLength(params.Description.Value, 0, 2048) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "description length must be less than 2048 characters")
	}

	return nil
}
