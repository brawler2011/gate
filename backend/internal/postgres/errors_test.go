package postgres

import (
	"errors"
	"os"
	"testing"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestDBErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code pkg.Code
	}{
		{"missing row", pgx.ErrNoRows, pkg.CodeNotFound},
		{"known constraint", &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "organizations_login_lower_idx"}, pkg.ErrOrganizationLoginTaken.Code},
		{"unknown unique violation is a bug", &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "outbox_aggregate_version"}, pkg.CodeInternal},
		{"insert with a missing parent is a bug", &pgconn.PgError{Code: pgerrcode.ForeignKeyViolation, Message: "insert or update on table \"x\" violates foreign key constraint"}, pkg.CodeInternal},
		{"blocked delete", &pgconn.PgError{Code: pgerrcode.ForeignKeyViolation, Message: "update or delete on table \"x\" violates foreign key constraint"}, pkg.ErrInUse.Code},
		{"restrict", &pgconn.PgError{Code: pgerrcode.RestrictViolation}, pkg.ErrInUse.Code},
		{"check violation is a bug", &pgconn.PgError{Code: pgerrcode.CheckViolation}, pkg.CodeInternal},
		{"connection failure", errors.New("connection failed"), pkg.CodeInternal},
		{"already coded", pkg.Wrap(pkg.ErrForbidden, nil, "no"), pkg.CodeForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := dbError("op", tc.err)
			require.Equal(t, tc.code, pkg.CodeOf(err))
			require.ErrorIs(t, err, tc.err)
			require.Contains(t, err.Error(), "op: ")
		})
	}
	require.NoError(t, dbError("op", nil))
}

func TestRetryable(t *testing.T) {
	err := retryable(dbError("commit", &pgconn.PgError{Code: pgerrcode.SerializationFailure}))
	require.ErrorIs(t, err, pkg.ErrConcurrentUpdate)
	require.NotErrorIs(t, retryable(errors.New("x")), pkg.ErrConcurrentUpdate)
}

func TestConstraintErrorsNameSchemaConstraints(t *testing.T) {
	schema, err := os.ReadFile("../../migrations/20261112000000_init.sql")
	require.NoError(t, err)
	for name := range constraintErrors {
		require.Contains(t, string(schema), name+" ", "constraint %s is not in the schema", name)
	}
}
