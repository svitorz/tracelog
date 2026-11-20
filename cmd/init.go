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

var initUseDefaults bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the minimum local system setup",
	Long:  "Collect and persist the minimum data required to run the local MVP. Interactive by default; --defaults stores the default values without reading stdin.",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		if err := repository.Ping(context.Background(), db); err != nil {
			return fmt.Errorf("failed to ping PostgreSQL: %w", err)
		}

		input := collectInitInput(cmd, initUseDefaults)

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
	initCmd.Flags().BoolVar(&initUseDefaults, "defaults", false, "Store the default values without prompting (for containers and scripts)")
	initCmd.Example = strings.Join([]string{
		"  # Interactively configure the local MVP",
		"  tracelog init",
		"",
		"  # Non-interactive, with the default values",
		"  tracelog init --defaults",
	}, "\n")

	rootCmd.AddCommand(initCmd)
}

type initField struct {
	label    string
	fallback string
	target   *string
}

// collectInitInput prompts for each field, or uses the defaults without touching stdin.
func collectInitInput(cmd *cobra.Command, useDefaults bool) services.InitializeSystemInput {
	var input services.InitializeSystemInput
	fields := []initField{
		{"Application name", "Local Application", &input.ApplicationName},
		{"Application service key", "local-app", &input.ApplicationServiceKey},
		{"Application description", "Default monitored application for the local MVP", &input.ApplicationDescription},
		{"Environment code", "local", &input.EnvironmentCode},
		{"Environment name", "Local", &input.EnvironmentName},
		{"Environment description", "Local development environment", &input.EnvironmentDescription},
		{"Event source name", "HTTP API", &input.SourceName},
		{"Event source type", "http", &input.SourceType},
		{"Event source description", "Events received via HTTP REST API", &input.SourceDescription},
	}

	if useDefaults {
		cmd.Println("[init] Using default values (--defaults).")
		for _, field := range fields {
			*field.target = field.fallback
		}
		return input
	}

	cmd.Println("[init] Configure the minimum local setup. Press Enter to accept defaults.")
	cmd.Println("")

	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()
	for _, field := range fields {
		*field.target = promptValue(reader, out, field.label, field.fallback)
	}

	return input
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
