package main

import (
	"embed"
	"fmt"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func runMigrations(envFile string) error {
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			return fmt.Errorf("load env file %s: %w", envFile, err)
		}
	}
	// Migrations need only the database, so they do not require the YAML config.
	var cfg struct {
		PostgresDSN string `env:"POSTGRES_DSN" env-required:"true"`
	}
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return fmt.Errorf("read migration config: %w", err)
	}

	db, err := pkg.NewPostgresDBForMigrations(cfg.PostgresDSN)
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
