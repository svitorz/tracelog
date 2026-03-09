package cmd

import (
	"fmt"

	"svitorz/tracelog/internal/config"

	"github.com/spf13/cobra"
)

var appConfig config.Config

var rootCmd = &cobra.Command{
	Use:           "tracelog",
	Short:         "Aplicação local para rastreabilidade de eventos técnicos",
	Long:          "CLI oficial do TraceLog para operar a aplicação local no MVP.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute loads configuration and executes the root CLI command.
func Execute() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	appConfig = cfg

	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}
