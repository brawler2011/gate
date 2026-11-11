package builder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type GCRepository interface {
	CloseExpiredUploads(context.Context) error
	RecoverBuilds(context.Context) error
	PrunePackages(context.Context, time.Duration) error
	PruneBuilds(context.Context, time.Duration) error
	ListGCObjects(context.Context, time.Duration) ([]model.StoredObject, error)
	BeginObjectDeletion(context.Context, uuid.UUID, time.Duration) (bool, error)
	CompleteObjectDeletion(context.Context, uuid.UUID) error
	ObjectByKey(context.Context, string) (model.StoredObject, error)
	ActiveBuildAttempt(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}
type GCStore interface {
	DeleteFile(context.Context, string, string) error
	ListObjects(context.Context, string, string) ([]storage.ObjectInfo, error)
}
type GC struct {
	Logger           *slog.Logger
	Repo             GCRepository
	Store            GCStore
	Bucket           string
	TempDir          string
	ObjectRetention  time.Duration
	BuildRetention   time.Duration
	PackageRetention time.Duration
	Interval         time.Duration
}

func (g *GC) Run(ctx context.Context) error {
	if g.ObjectRetention <= 0 || g.BuildRetention <= 0 || g.PackageRetention <= 0 || g.Interval <= 0 {
		return fmt.Errorf("GC retention and interval must be configured")
	}
	ticker := time.NewTicker(g.Interval)
	defer ticker.Stop()
	for {
		if err := g.Collect(ctx); err != nil && ctx.Err() == nil {
			g.Logger.ErrorContext(ctx, "problem garbage collection failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func (g *GC) Collect(ctx context.Context) error {
	if err := g.Repo.CloseExpiredUploads(ctx); err != nil {
		return err
	}
	if err := g.Repo.RecoverBuilds(ctx); err != nil {
		return err
	}
	if err := g.Repo.PrunePackages(ctx, g.PackageRetention); err != nil {
		return err
	}
	if err := g.Repo.PruneBuilds(ctx, g.BuildRetention); err != nil {
		return err
	}
	objects, err := g.Repo.ListGCObjects(ctx, g.ObjectRetention)
	if err != nil {
		return err
	}
	for _, o := range objects {
		deleting, err := g.Repo.BeginObjectDeletion(ctx, o.ID, g.ObjectRetention)
		if err != nil {
			return err
		}
		if !deleting {
			continue
		}
		// Physical deletion follows the committed deleting state. Unknown deletion
		// results leave that state intact, so a later run retries it.
		if err := g.Store.DeleteFile(ctx, g.Bucket, o.Key); err != nil {
			return err
		}
		if err := g.Repo.CompleteObjectDeletion(ctx, o.ID); err != nil {
			return err
		}
	}
	// Tombstones close UUIDs permanently. A late PUT can leave bytes there but
	// cannot publish a reference; reconciliation removes those bytes again.
	listed, err := g.Store.ListObjects(ctx, g.Bucket, "objects/")
	if err != nil {
		return err
	}
	for _, object := range listed {
		o, err := g.Repo.ObjectByKey(ctx, object.Key)
		switch {
		case errors.Is(err, pkg.ErrNotFound):
			if object.LastModified.IsZero() || time.Since(object.LastModified) < g.ObjectRetention {
				continue
			}
		case err != nil:
			return err
		case o.State != "deleted":
			continue
		}
		if err := g.Store.DeleteFile(ctx, g.Bucket, object.Key); err != nil {
			return err
		}
	}
	return g.cleanDirectories(ctx)
}
func (g *GC) cleanDirectories(ctx context.Context) error {
	entries, err := os.ReadDir(g.TempDir)
	if err != nil {
		return fmt.Errorf("readdir: %w", err)
	}
	for _, entry := range entries {
		name, ok := strings.CutPrefix(entry.Name(), "build-")
		if !ok || !entry.IsDir() || len(name) < 74 || name[36] != '-' || name[73] != '-' {
			continue
		}
		bid, err := uuid.Parse(name[:36])
		if err != nil {
			continue
		}
		token, err := uuid.Parse(name[37:73])
		if err != nil {
			continue
		}
		active, err := g.Repo.ActiveBuildAttempt(ctx, bid, token)
		if err != nil {
			return err
		}
		if active {
			continue
		}
		if err := os.RemoveAll(filepath.Join(g.TempDir, entry.Name())); err != nil {
			return fmt.Errorf("removeall: %w", err)
		}
	}
	return nil
}
