package main

import (
	"embed"
	"fmt"

	"github.com/gate149/core/config"
	"github.com/gate149/core/pkg"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func main() {
	var cfg config.Config
	err := cleanenv.ReadConfig(".env", &cfg)
	if err != nil {
		panic(fmt.Sprintf("error reading config: %s", err.Error()))
	}

	db, err := pkg.NewPostgresDBForMigrations(cfg.PostgresDSN)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		panic(err)
	}
}
