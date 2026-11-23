package postgres

import (
	"errors"
	"fmt"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Constraints whose violation is a client mistake rather than a bug.
var constraintErrors = map[string]struct {
	code    *pkg.Error
	message string
}{
	"users_username_lower_idx":                      {pkg.ErrUsernameTaken, "username is taken"},
	"users_email_idx":                               {pkg.ErrEmailTaken, "email is taken"},
	"organizations_login_lower_idx":                 {pkg.ErrOrganizationLoginTaken, "organization login is taken"},
	"contests_org_id_login_lower_idx":               {pkg.ErrContestLoginTaken, "contest login is taken"},
	"teams_organization_id_slug_key":                {pkg.ErrTeamSlugTaken, "team slug is taken"},
	"problem_drafts_organization_id_short_name_key": {pkg.ErrProblemShortNameTaken, "problem short name is taken"},
	"organization_members_pkey":                     {pkg.ErrAlreadyMember, "user is already a member"},
	"team_members_pkey":                             {pkg.ErrAlreadyMember, "user is already a member"},
	"contest_members_pkey":                          {pkg.ErrAlreadyMember, "user is already a member"},
	"problem_members_pkey":                          {pkg.ErrAlreadyMember, "user is already a member"},
	"contest_teams_pkey":                            {pkg.ErrAlreadyMember, "team is already a member"},
	"org_invitations_pending_unique_idx":            {pkg.ErrRequestPending, "an invitation is already pending"},
	"org_join_requests_pending_unique_idx":          {pkg.ErrRequestPending, "a join request is already pending"},
	"contest_join_requests_pending_unique_idx":      {pkg.ErrRequestPending, "a join request is already pending"},
	"contest_problems_pkey":                         {pkg.ErrProblemInContest, "problem is already in the contest"},
	"contest_virtual_runs_pkey":                     {pkg.ErrConflict, "a virtual run was already started"},
	"contests_check":                                {pkg.ErrEndBeforeStart, "end time must be after start time"},
	"contests_login_check":                          {pkg.ErrLength, "contest login must be 3 to 64 characters"},
	"contest_problems_ordinal_check":                {pkg.ErrRange, "problem positions must be non-negative"},
	"contest_problems_contest_id_ordinal_key":       {pkg.ErrDuplicate, "problem positions must be distinct"},
	"contest_teams_team_id_fkey":                    {pkg.ErrNotFound, "team not found"},
	"owner_required":                                {pkg.ErrOwnerRequired, "keep at least one permanent owner"},
	"contest_user_problem_blocks_user_id_fkey":      {pkg.ErrNotFound, "user not found"},
	"contest_user_problem_blocks_problem_id_fkey":   {pkg.ErrNotFound, "problem not found"},
}

// isRetryable reports a lost race that a fresh transaction may win.
func isRetryable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == pgerrcode.SerializationFailure || pgErr.Code == pgerrcode.DeadlockDetected)
}

// dbError names the failed operation; only a missing row, a known constraint or a blocked delete get a client code.
func dbError(op string, err error) error {
	if err == nil {
		return nil
	}
	wrapped := fmt.Errorf("%s: %w", op, err)
	var appErr *pkg.Error
	if errors.As(err, &appErr) {
		return wrapped
	}
	// Only single-row queries return ErrNoRows.
	if errors.Is(err, pgx.ErrNoRows) {
		return pkg.Wrap(pkg.ErrNotFound, wrapped, "")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return wrapped
	}
	if known, ok := constraintErrors[pgErr.ConstraintName]; ok {
		return pkg.Wrap(known.code, wrapped, known.message)
	}
	// 23503 is also raised when deleting a row a NO ACTION key still references.
	if pgErr.Code == pgerrcode.RestrictViolation || (pgErr.Code == pgerrcode.ForeignKeyViolation && strings.HasPrefix(pgErr.Message, "update or delete")) {
		return pkg.Wrap(pkg.ErrInUse, wrapped, "still in use")
	}
	return wrapped
}
