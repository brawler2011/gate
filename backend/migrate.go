package main

import (
	"embed"
	"fmt"

	"github.com/gate149/gate/backend/pkg"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func runMigrations(envFile string) error {
	cfg, err := loadConfig(envFile)
	if err != nil {
		return err
	}

	db, err := pkg.NewPostgresDBForMigrations(cfg.GetPostgresDSN())
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
