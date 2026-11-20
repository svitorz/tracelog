package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

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
	Short: "Start the local TraceLog HTTP server",
	Long:  "Initialize and start the HTTP server for receiving technical events.",
	RunE: func(cmd *cobra.Command, args []string) error {
		port := httpPort
		if port == 0 {
			port = appConfig.HTTP.Port
		}

		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid HTTP port: %d", port)
		}

		cmd.Printf("[serve] %s (%s)\n", appConfig.App.Name, appConfig.App.Env)
		cmd.Printf("[serve] Initializing PostgreSQL connection...\n")

		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		if err := repository.Ping(context.Background(), db); err != nil {
			return fmt.Errorf("failed to ping PostgreSQL: %w", err)
		}

		cmd.Printf("[serve] PostgreSQL connected at %s:%d/%s\n", appConfig.DB.Host, appConfig.DB.Port, appConfig.DB.Name)
		cmd.Printf("[serve] Initializing application dependencies...\n")

		setupRepo := repository.NewPostgresSystemSetupRepository(db)
		ensureSetupUseCase := services.NewEnsureSystemSetupUseCase(setupRepo)
		if err := ensureSetupUseCase.Execute(context.Background()); err != nil {
			return err
		}

		eventRepo := repository.NewPostgresTechnicalEventRepository(db)
		registerUseCase := services.NewRegisterTechnicalEventUseCase(eventRepo, setupRepo)

		server := httpadapter.NewServer(fmt.Sprintf(":%d", port), registerUseCase)

		ln, err := net.Listen("tcp", server.Addr())
		if err != nil {
			return fmt.Errorf("failed to listen on port %d: %w", port, err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		cmd.Printf("[serve] HTTP server listening on port %d\n", port)
		cmd.Printf("[serve] Ready to receive events at POST /events\n")

		if err := server.Serve(ctx, ln); err != nil {
			return err
		}

		cmd.Printf("[serve] Shutdown complete\n")
		return nil
	},
}

func init() {
	serveCmd.Flags().IntVar(&httpPort, "http-port", 0, "HTTP port (overrides HTTP_PORT env variable)")
	serveCmd.Example = fmt.Sprintf(
		"  # Start with default configuration\n" +
			"  tracelog serve\n\n" +
			"  # Start with custom HTTP port\n" +
			"  tracelog serve --http-port 8081",
	)
	rootCmd.AddCommand(serveCmd)
}
