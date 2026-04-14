package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	var envFile string
	var migrate bool

	flag.StringVar(&envFile, "env", "", "path to environment file")
	flag.BoolVar(&migrate, "migrate", false, "run database migrations and exit")
	flag.Parse()

	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments: %v\n", flag.Args())
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

	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
