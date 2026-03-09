package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"svitorz/tracelog/internal/adapters/repository"

	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database inspection utilities",
}

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List database entities",
}

var dbListEnvironmentsCmd = &cobra.Command{
	Use:   "environments",
	Short: "List all environments",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		rows, err := db.QueryContext(context.Background(), `
			SELECT id, code, name, description
			FROM environment
			ORDER BY code
		`)
		if err != nil {
			return fmt.Errorf("failed to query environments: %w", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tCODE\tNAME\tDESCRIPTION")
		fmt.Fprintln(w, "--\t----\t----\t-----------")

		count := 0
		for rows.Next() {
			var id, code, name, desc string
			if err := rows.Scan(&id, &code, &name, &desc); err != nil {
				return err
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, code, name, desc)
			count++
		}

		w.Flush()
		cmd.Printf("\nTotal: %d environment(s)\n", count)
		return nil
	},
}

var dbListApplicationsCmd = &cobra.Command{
	Use:   "applications",
	Short: "List all monitored applications",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		rows, err := db.QueryContext(context.Background(), `
			SELECT id, name, service_key, description
			FROM monitored_application
			ORDER BY name
		`)
		if err != nil {
			return fmt.Errorf("failed to query applications: %w", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSERVICE_KEY\tDESCRIPTION")
		fmt.Fprintln(w, "--\t----\t-----------\t-----------")

		count := 0
		for rows.Next() {
			var id, name, key, desc string
			if err := rows.Scan(&id, &name, &key, &desc); err != nil {
				return err
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, name, key, desc)
			count++
		}

		w.Flush()
		cmd.Printf("\nTotal: %d application(s)\n", count)
		return nil
	},
}

var dbListSourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "List all event sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		rows, err := db.QueryContext(context.Background(), `
			SELECT id, name, source_type, description
			FROM event_source
			ORDER BY name
		`)
		if err != nil {
			return fmt.Errorf("failed to query sources: %w", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tTYPE\tDESCRIPTION")
		fmt.Fprintln(w, "--\t----\t----\t-----------")

		count := 0
		for rows.Next() {
			var id, name, stype, desc string
			if err := rows.Scan(&id, &name, &stype, &desc); err != nil {
				return err
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, name, stype, desc)
			count++
		}

		w.Flush()
		cmd.Printf("\nTotal: %d source(s)\n", count)
		return nil
	},
}

func init() {
	dbListCmd.AddCommand(dbListEnvironmentsCmd)
	dbListCmd.AddCommand(dbListApplicationsCmd)
	dbListCmd.AddCommand(dbListSourcesCmd)

	dbCmd.AddCommand(dbListCmd)
	rootCmd.AddCommand(dbCmd)
}
