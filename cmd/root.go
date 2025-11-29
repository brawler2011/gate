package cmd

import (
	"fmt"
	"os"

	"github.com/gate149/core/cmd/migrate"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "core",
	Short: "Core application for Gate149",
	Long:  `Core application containing server, migration tools, and kratos webhooks.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(migrate.NewMigrateCmd())
}

