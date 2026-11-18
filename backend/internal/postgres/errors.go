package postgres

import (
	"database/sql"
	"errors"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// isForeignKeyViolation reports a reference to a missing row, including an
// object of another organization (23503), or the deletion of a row that an
// ON DELETE RESTRICT reference still holds (23001).
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == pgerrcode.ForeignKeyViolation || pgErr.Code == pgerrcode.RestrictViolation)
}

func HandlePgErr(err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := pkg.HandleContextErr(err); ctxErr != nil {
		return ctxErr
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == pgerrcode.UniqueViolation || pgErr.Code == pgerrcode.ExclusionViolation || pgErr.Code == pgerrcode.SerializationFailure || pgErr.Code == pgerrcode.DeadlockDetected {
			return pkg.Wrap(pkg.ErrConflict, err, "conflicting data")
		}
		if pgerrcode.IsIntegrityConstraintViolation(pgErr.Code) {
			return pkg.Wrap(pkg.ErrBadInput, err, "integrity constraint violation")
		}

		return pkg.Wrap(pkg.ErrInternal, err, "unexpected postgres error")
	}

	if errors.Is(err, sql.ErrNoRows) {
		return pkg.Wrap(pkg.ErrNotFound, err, "no rows found")
	}

	return pkg.Wrap(pkg.ErrInternal, err, "unexpected error")
}
