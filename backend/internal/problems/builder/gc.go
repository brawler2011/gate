package builder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type GCRepository interface {
	RecoverBuilds(context.Context) error
	PrunePackages(context.Context, time.Duration) error
	PruneBuilds(context.Context, time.Duration) error
	ListUnreferencedObjects(context.Context) ([]model.StoredObject, error)
	DeleteObject(context.Context, uuid.UUID) (bool, error)
	ExistingObjectKeys(context.Context, []string) (map[string]bool, error)
	ActiveBuildAttempt(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}
type GCStore interface {
	DeleteFile(context.Context, string, string) error
	ListObjects(context.Context, string, string) ([]storage.ObjectInfo, error)
}
type GC struct {
	Logger  *slog.Logger
	Repo    GCRepository
	Store   GCStore
	Bucket  string
	TempDir string
	// ObjectRetention must exceed every upload lease and build deadline to protect pending references.
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
	if err := g.Repo.RecoverBuilds(ctx); err != nil {
		return err
	}
	if err := g.Repo.PrunePackages(ctx, g.PackageRetention); err != nil {
		return err
	}
	if err := g.Repo.PruneBuilds(ctx, g.BuildRetention); err != nil {
		return err
	}
	objects, err := g.Repo.ListUnreferencedObjects(ctx)
	if err != nil {
		return err
	}
	for _, o := range objects {
		deleted, err := g.Repo.DeleteObject(ctx, o.ID)
		if err != nil {
			return err
		}
		if !deleted {
			continue
		}
		// Failed byte deletion leaves an orphan for the next sweep.
		if err := g.Store.DeleteFile(ctx, g.Bucket, o.Key); err != nil {
			return err
		}
	}
	if err := g.sweepOrphans(ctx); err != nil {
		return err
	}
	return g.cleanDirectories(ctx)
}

// Keys are never reused, so expired unregistered bytes cannot gain a reference.
func (g *GC) sweepOrphans(ctx context.Context) error {
	// Freeze the cutoff before lookups; keys aging during the sweep may still gain a row.
	cutoff := time.Now().Add(-g.ObjectRetention)
	listed, err := g.Store.ListObjects(ctx, g.Bucket, "objects/")
	if err != nil {
		return err
	}
	var keys []string
	for _, object := range listed {
		if !object.LastModified.IsZero() && object.LastModified.Before(cutoff) {
			keys = append(keys, object.Key)
		}
	}
	for batch := range slices.Chunk(keys, 1000) {
		existing, err := g.Repo.ExistingObjectKeys(ctx, batch)
		if err != nil {
			return err
		}
		for _, key := range batch {
			if existing[key] {
				continue
			}
			if err := g.Store.DeleteFile(ctx, g.Bucket, key); err != nil {
				return err
			}
		}
	}
	return nil
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
