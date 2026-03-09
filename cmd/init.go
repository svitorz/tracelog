package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"svitorz/tracelog/internal/adapters/repository"
	"svitorz/tracelog/internal/core/services"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the minimum local system setup",
	Long:  "Interactively collect and persist the minimum data required to run the local MVP.",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		if err := repository.Ping(context.Background(), db); err != nil {
			return fmt.Errorf("failed to ping PostgreSQL: %w", err)
		}

		reader := bufio.NewReader(cmd.InOrStdin())
		out := cmd.OutOrStdout()

		cmd.Println("[init] Configure the minimum local setup. Press Enter to accept defaults.")
		cmd.Println("")

		input := services.InitializeSystemInput{
			ApplicationName:        promptValue(reader, out, "Application name", "Local Application"),
			ApplicationServiceKey:  promptValue(reader, out, "Application service key", "local-app"),
			ApplicationDescription: promptValue(reader, out, "Application description", "Default monitored application for the local MVP"),
			EnvironmentCode:        promptValue(reader, out, "Environment code", "local"),
			EnvironmentName:        promptValue(reader, out, "Environment name", "Local"),
			EnvironmentDescription: promptValue(reader, out, "Environment description", "Local development environment"),
			SourceName:             promptValue(reader, out, "Event source name", "HTTP API"),
			SourceType:             promptValue(reader, out, "Event source type", "http"),
			SourceDescription:      promptValue(reader, out, "Event source description", "Events received via HTTP REST API"),
		}

		setupRepo := repository.NewPostgresSystemSetupRepository(db)
		initUseCase := services.NewInitializeSystemUseCase(setupRepo)

		setup, err := initUseCase.Execute(context.Background(), input)
		if err != nil {
			return err
		}

		cmd.Println("")
		cmd.Println("[init] Basic system setup saved successfully.")
		cmd.Printf("[init] Application: %s (%s) [%s]\n", setup.Application.Name, setup.Application.ServiceKey, setup.Application.ID)
		cmd.Printf("[init] Environment: %s (%s) [%s]\n", setup.Environment.Name, setup.Environment.Code, setup.Environment.ID)
		cmd.Printf("[init] Event source: %s (%s) [%s]\n", setup.Source.Name, setup.Source.SourceType, setup.Source.ID)
		cmd.Println("")
		cmd.Println("[init] The application is ready to start with `tracelog serve`.")

		return nil
	},
}

func init() {
	initCmd.Example = strings.Join([]string{
		"  # Interactively configure the local MVP",
		"  tracelog init",
	}, "\n")

	rootCmd.AddCommand(initCmd)
}

func promptValue(reader *bufio.Reader, out io.Writer, label, fallback string) string {
	for {
		_, _ = fmt.Fprintf(out, "%s [%s]: ", label, fallback)

		raw, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fallback
		}

		value := strings.TrimSpace(raw)
		if value == "" {
			value = fallback
		}

		if value != "" {
			return value
		}

		if err == io.EOF {
			return fallback
		}
	}
}
