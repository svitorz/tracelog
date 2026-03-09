package cmd

import (
	"context"
	"fmt"

	"svitorz/tracelog/internal/adapters/repository"

	"github.com/spf13/cobra"
)

var (
	httpPort int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Inicializa os componentes de execução local",
	RunE: func(cmd *cobra.Command, args []string) error {
		port := httpPort
		if port == 0 {
			port = appConfig.HTTP.Port
		}

		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return err
		}
		defer db.Close()

		if err := repository.Ping(context.Background(), db); err != nil {
			return err
		}

		// Placeholder: bootstrap real de HTTP virá em etapa futura.
		cmd.Printf("[serve] %s (%s) porta HTTP %d\n", appConfig.App.Name, appConfig.App.Env, port)
		cmd.Printf("[serve] PostgreSQL OK em %s:%d/%s (%s)\n", appConfig.DB.Host, appConfig.DB.Port, appConfig.DB.Name, appConfig.DB.SSLMode)
		return nil
	},
}

func init() {
	serveCmd.Flags().IntVar(&httpPort, "http-port", 0, "porta HTTP (sobrescreve HTTP_PORT quando informado)")
	serveCmd.Example = fmt.Sprintf("%s\n%s", "  tracelog serve", "  tracelog serve --http-port 8081")
	rootCmd.AddCommand(serveCmd)
}
