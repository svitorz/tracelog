package cmd

import (
	"github.com/spf13/cobra"
)

var (
	httpAddr string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Inicializa os componentes de execução local",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Placeholder: bootstrap real de HTTP virá em etapa futura.
		cmd.Printf("[serve] inicialização solicitada em %s\n", httpAddr)
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVar(&httpAddr, "http-addr", ":8080", "endereço HTTP para bind")
	rootCmd.AddCommand(serveCmd)
}
