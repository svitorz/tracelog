package cmd

import "github.com/spf13/cobra"

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Operações de métricas",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Placeholder: agregações e relatórios serão implementados depois.
		cmd.Println("[metrics] comando base disponível")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(metricsCmd)
}
