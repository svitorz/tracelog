package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "tracelog",
	Short: "Aplicação local para rastreabilidade de eventos técnicos",
}

// Execute executa o comando raiz da CLI.
func Execute() error {
	return rootCmd.Execute()
}
