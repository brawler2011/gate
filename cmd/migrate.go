package cmd

import (
	"embed"
	"fmt"

	"github.com/gate149/core/config"
	"github.com/gate149/core/pkg"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		envFile, _ := cmd.Flags().GetString("env")

		var cfg config.Config
		var err error

		if envFile != "" {
			err = cleanenv.ReadConfig(envFile, &cfg)
		} else {
			err = cleanenv.ReadConfig(".env", &cfg)
		}

		if err != nil {
			// Fallback to env vars if file not found or error
			if err := cleanenv.ReadEnv(&cfg); err != nil {
				panic(fmt.Sprintf("error reading config: %s", err.Error()))
			}
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
	},
}

func init() {
	migrateCmd.Flags().String("env", ".env", "path to environment file")
	rootCmd.AddCommand(migrateCmd)
}
