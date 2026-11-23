package cmd

import (
	"context"
	"fmt"

	"svitorz/tracelog/internal/adapters/repository"
	"svitorz/tracelog/migrations"

	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply pending database migrations",
	Long:  "Apply the SQL migrations embedded in the binary that are not yet recorded in schema_migrations. Safe to run repeatedly.",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		if err := repository.Ping(context.Background(), db); err != nil {
			return fmt.Errorf("failed to ping PostgreSQL: %w", err)
		}

		result, err := repository.Migrate(context.Background(), db, migrations.Files)
		out := cmd.OutOrStdout()
		for _, version := range result.Applied {
			_, _ = fmt.Fprintf(out, "[migrate] applied %s\n", version)
		}
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(out, "[migrate] database is up to date (%d migrations)\n", result.Total)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
