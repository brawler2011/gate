package inbox

import (
	"context"
	"time"
)

type Repository interface {
	PruneReceipts(ctx context.Context, cutoff time.Time) (int64, error)
}
