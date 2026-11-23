package datareset

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"

	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/pressly/goose/v3"
)

type Stores struct {
	DB *sql.DB
	S3 *storage.S3Storage
}

func Hash(migrations fs.FS) (string, error) {
	names, err := fs.Glob(migrations, "*.sql")
	if err != nil {
		return "", fmt.Errorf("list migrations: %w", err)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no migrations found")
	}
	h := sha256.New()
	for _, name := range names {
		data, err := fs.ReadFile(migrations, name)
		if err != nil {
			return "", fmt.Errorf("read migration %s: %w", name, err)
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", name, len(data))
		_, _ = h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func Apply(ctx context.Context, stores Stores, migrations fs.FS) (bool, error) {
	want, err := Hash(migrations)
	if err != nil {
		return false, err
	}
	have, err := storedHash(ctx, stores.DB)
	if err != nil {
		return false, err
	}
	if have == want {
		return false, nil
	}
	// Record the hash only after every store has reset successfully.
	if err := wipe(ctx, stores); err != nil {
		return false, err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, stores.DB, migrations)
	if err != nil {
		return false, fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return false, fmt.Errorf("apply schema: %w", err)
	}
	if _, err := stores.DB.ExecContext(ctx, "CREATE TABLE schema_hash (hash text NOT NULL)"); err != nil {
		return false, fmt.Errorf("create schema_hash: %w", err)
	}
	if _, err := stores.DB.ExecContext(ctx, "INSERT INTO schema_hash (hash) VALUES ($1)", want); err != nil {
		return false, fmt.Errorf("store schema hash: %w", err)
	}
	return true, nil
}

func storedHash(ctx context.Context, db *sql.DB) (string, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.schema_hash') IS NOT NULL").Scan(&exists); err != nil {
		return "", fmt.Errorf("check schema_hash: %w", err)
	}
	if !exists {
		return "", nil
	}
	var hash string
	err := db.QueryRowContext(ctx, "SELECT hash FROM schema_hash").Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read schema hash: %w", err)
	}
	return hash, nil
}

func wipe(ctx context.Context, stores Stores) error {
	buckets, err := stores.S3.ListBuckets(ctx)
	if err != nil {
		return err
	}
	for _, bucket := range buckets {
		if err := stores.S3.EmptyBucket(ctx, bucket); err != nil {
			return fmt.Errorf("empty bucket %s: %w", bucket, err)
		}
	}

	if _, err := stores.DB.ExecContext(ctx, "DROP SCHEMA public CASCADE"); err != nil {
		return fmt.Errorf("drop schema: %w", err)
	}
	if _, err := stores.DB.ExecContext(ctx, "CREATE SCHEMA public"); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}
