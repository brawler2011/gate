package pg

import (
	"database/sql"
	"errors"

	"github.com/gate149/gate/backend/pkg"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func HandlePgErr(err error) error {
	if ctxErr := pkg.HandleContextErr(err); ctxErr != nil {
		return ctxErr
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgerrcode.IsIntegrityConstraintViolation(pgErr.Code) {
			return pkg.Wrap(pkg.ErrBadInput, err, "integrity constraint violation")
		}

		return pkg.Wrap(pkg.ErrUnhandled, err, "unexpected postgres error")
	}

	if errors.Is(err, sql.ErrNoRows) {
		return pkg.Wrap(pkg.ErrNotFound, err, "no rows found")
	}

	return pkg.Wrap(pkg.ErrUnhandled, err, "unexpected error")
}
