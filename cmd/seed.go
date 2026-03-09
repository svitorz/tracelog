package cmd

import (
	"context"
	"fmt"

	"svitorz/tracelog/internal/adapters/repository"

	"github.com/spf13/cobra"
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Bootstrap database with initial data",
	Long:  "Populate the database with default environments, applications, and event sources for local development.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("[seed] Connecting to PostgreSQL...")

		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		if err := repository.Ping(context.Background(), db); err != nil {
			return fmt.Errorf("failed to ping PostgreSQL: %w", err)
		}

		cmd.Println("[seed] Seeding default data...")

		// Seed environments
		envResult, err := db.ExecContext(context.Background(), `
			INSERT INTO environment (code, name, description)
			VALUES
				('local', 'Local', 'Local development environment'),
				('dev', 'Development', 'Shared development environment'),
				('staging', 'Staging', 'Pre-production environment'),
				('prod', 'Production', 'Production environment')
			ON CONFLICT (code) DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("failed to seed environments: %w", err)
		}

		envRows, _ := envResult.RowsAffected()

		// Seed monitored applications
		appResult, err := db.ExecContext(context.Background(), `
			INSERT INTO monitored_application (name, service_key, description)
			VALUES
				('Sample Web API', 'sample-web-api', 'Default sample application for testing and demonstration'),
				('Background Worker', 'background-worker', 'Sample background job processor'),
				('Frontend App', 'frontend-app', 'Sample frontend application')
			ON CONFLICT (service_key) DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("failed to seed applications: %w", err)
		}

		appRows, _ := appResult.RowsAffected()

		// Seed event sources
		sourceResult, err := db.ExecContext(context.Background(), `
			INSERT INTO event_source (name, source_type, description)
			VALUES
				('HTTP API', 'http', 'Events received via HTTP REST API'),
				('CLI Tool', 'cli', 'Events registered via command-line interface'),
				('File Import', 'file', 'Events imported from log files'),
				('System Monitor', 'system', 'System-generated monitoring events')
			ON CONFLICT (name, source_type) DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("failed to seed event sources: %w", err)
		}

		sourceRows, _ := sourceResult.RowsAffected()

		cmd.Printf("[seed] ✓ Environments: %d inserted\n", envRows)
		cmd.Printf("[seed] ✓ Applications: %d inserted\n", appRows)
		cmd.Printf("[seed] ✓ Event Sources: %d inserted\n", sourceRows)
		cmd.Println("")
		cmd.Println("[seed] Database bootstrapped successfully!")
		cmd.Println("")
		cmd.Println("To list available data, run:")
		cmd.Println("  tracelog db list environments")
		cmd.Println("  tracelog db list applications")
		cmd.Println("  tracelog db list sources")

		return nil
	},
}

func init() {
	seedCmd.Example = `  # Bootstrap database with default data
  tracelog seed`

	rootCmd.AddCommand(seedCmd)
}
