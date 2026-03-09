package cmd

import "github.com/spf13/cobra"

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Operações de eventos técnicos",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Placeholder: comandos de consulta/registro serão adicionados depois.
		cmd.Println("[events] comando base disponível")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
}
