package interfaces

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Transactor interface {
	WithTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error
}
