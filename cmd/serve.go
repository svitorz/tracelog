package cmd

import (
	httpadapter "svitorz/tracelog/internal/adapters/http"

	"github.com/spf13/cobra"
)

var (
	httpAddr string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Inicia o servidor HTTP do TraceLog",
	RunE: func(cmd *cobra.Command, args []string) error {
		server := httpadapter.NewServer(httpAddr)
		return server.Start()
	},
}

func init() {
	serveCmd.Flags().StringVar(&httpAddr, "http-addr", ":8080", "endereço HTTP para bind")
	rootCmd.AddCommand(serveCmd)
}
