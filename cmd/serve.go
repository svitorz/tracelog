package cmd

import (
	"context"
	"fmt"

	httpadapter "svitorz/tracelog/internal/adapters/http"
	"svitorz/tracelog/internal/adapters/repository"
	"svitorz/tracelog/internal/core/services"

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

		cmd.Printf("[serve] %s (%s) starting on port %d\n", appConfig.App.Name, appConfig.App.Env, port)
		cmd.Printf("[serve] PostgreSQL OK at %s:%d/%s (%s)\n", appConfig.DB.Host, appConfig.DB.Port, appConfig.DB.Name, appConfig.DB.SSLMode)

		eventRepo := repository.NewPostgresTechnicalEventRepository(db)
		registerUseCase := services.NewRegisterTechnicalEventUseCase(eventRepo)

		addr := fmt.Sprintf(":%d", port)
		server := httpadapter.NewServer(addr, registerUseCase)

		cmd.Printf("[serve] HTTP server listening...\n")
		return server.Start()
	},
}

func init() {
	serveCmd.Flags().IntVar(&httpPort, "http-port", 0, "porta HTTP (sobrescreve HTTP_PORT quando informado)")
	serveCmd.Example = fmt.Sprintf("%s\n%s", "  tracelog serve", "  tracelog serve --http-port 8081")
	rootCmd.AddCommand(serveCmd)
}
