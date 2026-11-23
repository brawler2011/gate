package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/brawler2011/gate/backend/config"
	"github.com/brawler2011/gate/backend/internal/datareset"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func runMigrations(configFile, envFile string) error {
	cfg, err := loadConfig(configFile, envFile, config.ModeAll)
	if err != nil {
		return err
	}
	migrations, err := fs.Sub(embedMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	db, err := pkg.NewPostgresDBForMigrations(cfg.Secrets.PostgresDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	store := storage.NewS3Storage(storage.S3Config{
		Endpoint:  cfg.Storage.S3Endpoint,
		AccessKey: cfg.Secrets.S3AccessKey,
		SecretKey: cfg.Secrets.S3SecretKey,
		Region:    defaultS3Region,
	})

	reset, err := datareset.Apply(context.Background(), datareset.Stores{DB: db, S3: store}, migrations)
	if err != nil {
		return err
	}
	if reset {
		slog.Warn("schema changed: all data was reset and the schema applied")
	} else {
		slog.Info("schema unchanged: data kept")
	}
	return nil
}
