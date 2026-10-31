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
	var envFile string
	var migrate bool

	flag.StringVar(&envFile, "env", "", "path to environment file")
	flag.BoolVar(&migrate, "migrate", false, "run database migrations and exit")
	flag.Parse()

	if flag.NArg() != 0 {
		slog.Error("unexpected arguments", "arguments", flag.Args())
		flag.Usage()
		os.Exit(2)
	}

	var err error
	if migrate {
		err = runMigrations(envFile)
	} else {
		err = runApp(envFile)
	}

	if err == nil || errors.Is(err, os.ErrClosed) {
		return
	}

	slog.Error("application startup failed", "error", err)
	os.Exit(1)
}
