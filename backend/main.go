package main

import (
	"errors"
	"flag"
	"log/slog"
	"os"

	"github.com/brawler2011/gate/backend/pkg/logging"
)

func main() {
	slog.SetDefault(logging.New(os.Stdout))
	var configFile, envFile string
	var migrate bool

	flag.StringVar(&configFile, "config", "", "path to the YAML config file")
	flag.StringVar(&envFile, "env", "", "path to an environment file with secrets for local runs")
	flag.BoolVar(&migrate, "migrate", false, "run database migrations and exit")
	flag.Parse()

	if flag.NArg() != 0 {
		slog.Error("unexpected arguments", "arguments", flag.Args())
		flag.Usage()
		os.Exit(2)
	}

	var err error
	switch {
	case migrate:
		err = runMigrations(envFile)
	case configFile == "":
		slog.Error("missing -config")
		flag.Usage()
		os.Exit(2)
	default:
		err = runApp(configFile, envFile)
	}

	if err == nil || errors.Is(err, os.ErrClosed) {
		return
	}

	slog.Error("application startup failed", "error", err)
	os.Exit(1)
}
