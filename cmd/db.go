package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"svitorz/tracelog/internal/adapters/repository"
	"svitorz/tracelog/internal/core/services"

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
		return withReferenceData(func(ctx context.Context, uc *services.ListReferenceDataUseCase) error {
			items, err := uc.Environments(ctx)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tCODE\tNAME\tDESCRIPTION")
			fmt.Fprintln(w, "--\t----\t----\t-----------")
			for _, e := range items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.ID, e.Code, e.Name, e.Description)
			}
			w.Flush()

			cmd.Printf("\nTotal: %d environment(s)\n", len(items))
			return nil
		})
	},
}

var dbListApplicationsCmd = &cobra.Command{
	Use:   "applications",
	Short: "List all monitored applications",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withReferenceData(func(ctx context.Context, uc *services.ListReferenceDataUseCase) error {
			items, err := uc.Applications(ctx)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSERVICE_KEY\tDESCRIPTION")
			fmt.Fprintln(w, "--\t----\t-----------\t-----------")
			for _, a := range items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, a.Name, a.ServiceKey, a.Description)
			}
			w.Flush()

			cmd.Printf("\nTotal: %d application(s)\n", len(items))
			return nil
		})
	},
}

var dbListSourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "List all event sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withReferenceData(func(ctx context.Context, uc *services.ListReferenceDataUseCase) error {
			items, err := uc.Sources(ctx)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tTYPE\tDESCRIPTION")
			fmt.Fprintln(w, "--\t----\t----\t-----------")
			for _, src := range items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", src.ID, src.Name, src.SourceType, src.Description)
			}
			w.Flush()

			cmd.Printf("\nTotal: %d source(s)\n", len(items))
			return nil
		})
	},
}

// withReferenceData opens the database and runs fn with the reference data use case.
func withReferenceData(fn func(context.Context, *services.ListReferenceDataUseCase) error) error {
	db, err := repository.OpenPostgres(appConfig.DB)
	if err != nil {
		return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}
	defer db.Close()

	uc := services.NewListReferenceDataUseCase(repository.NewPostgresReferenceDataRepository(db))
	return fn(context.Background(), uc)
}

func init() {
	dbListCmd.AddCommand(dbListEnvironmentsCmd)
	dbListCmd.AddCommand(dbListApplicationsCmd)
	dbListCmd.AddCommand(dbListSourcesCmd)

	dbCmd.AddCommand(dbListCmd)
	rootCmd.AddCommand(dbCmd)
}
