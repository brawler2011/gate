package postgres

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestPostgresErrorCategories(t *testing.T) {
	for _, tc := range []struct {
		name          string
		err, category error
	}{
		{name: "nil"},
		{name: "missing", err: sql.ErrNoRows, category: pkg.ErrNotFound},
		{name: "unique", err: &pgconn.PgError{Code: pgerrcode.UniqueViolation}, category: pkg.ErrConflict},
		{name: "exclusion", err: &pgconn.PgError{Code: pgerrcode.ExclusionViolation}, category: pkg.ErrConflict},
		{name: "foreign key", err: &pgconn.PgError{Code: pgerrcode.ForeignKeyViolation}, category: pkg.ErrBadInput},
		{name: "check", err: &pgconn.PgError{Code: pgerrcode.CheckViolation}, category: pkg.ErrBadInput},
		{name: "database failure", err: &pgconn.PgError{Code: pgerrcode.InternalError}, category: pkg.ErrInternal},
		{name: "connection failure", err: errors.New("connection failed"), category: pkg.ErrInternal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := HandlePgErr(tc.err)
			require.ErrorIs(t, err, tc.category)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			}
		})
	}
}
