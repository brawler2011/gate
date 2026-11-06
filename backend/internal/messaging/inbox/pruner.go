package inbox

import (
	"context"
	"log/slog"
	"time"
)

const DefaultPruneInterval = 1 * time.Hour

type Pruner struct {
	repo            Repository
	retentionWindow time.Duration
	interval        time.Duration
}

func NewPruner(repo Repository, retentionWindow time.Duration, interval time.Duration) *Pruner {
	return &Pruner{
		repo:            repo,
		retentionWindow: retentionWindow,
		interval:        interval,
	}
}

func (p *Pruner) Prune(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-p.retentionWindow)
	deleted, err := p.repo.PruneReceipts(ctx, cutoff)
	if err != nil {
		slog.ErrorContext(ctx, "failed to prune expired inbox receipts", "operation", "inbox_prune", "error", err)
		return 0, err
	}
	if deleted > 0 {
		slog.InfoContext(ctx, "pruned expired inbox receipts", "operation", "inbox_prune", "count", deleted)
	}
	return deleted, nil
}

func (p *Pruner) Start(ctx context.Context) {
	_, _ = p.Prune(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Inbox pruner stopping...")
			return
		case <-ticker.C:
			_, _ = p.Prune(ctx)
		}
	}
}
