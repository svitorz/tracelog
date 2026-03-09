package cmd

import (
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

// Execute executa o comando raiz da CLI.
func Execute() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	appConfig = cfg

	return rootCmd.Execute()
}
