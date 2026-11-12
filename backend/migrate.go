package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/brawler2011/gate/backend/internal/datareset"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/nats-io/nats.go/jetstream"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// runMigrations applies the schema, resetting all data if the schema changed.
func runMigrations(configFile, envFile string) error {
	cfg, err := loadConfig(configFile, envFile)
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

	redisClient, err := pkg.NewRedisClient(cfg.Secrets.RedisURL)
	if err != nil {
		return err
	}
	defer func() { _ = redisClient.Close() }()

	natsConn, err := pkg.NewNatsConn(cfg.Secrets.NatsURL)
	if err != nil {
		return err
	}
	defer natsConn.Close()
	js, err := jetstream.New(natsConn)
	if err != nil {
		return fmt.Errorf("create jetstream: %w", err)
	}

	store := storage.NewS3Storage(storage.S3Config{
		Endpoint:  cfg.Storage.S3Endpoint,
		AccessKey: cfg.Secrets.S3AccessKey,
		SecretKey: cfg.Secrets.S3SecretKey,
		Region:    defaultS3Region,
	})

	reset, err := datareset.Apply(context.Background(), datareset.Stores{DB: db, Redis: redisClient, S3: store, JetStream: js}, migrations)
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
