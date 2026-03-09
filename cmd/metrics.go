package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"svitorz/tracelog/internal/adapters/repository"
	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/services"

	"github.com/spf13/cobra"
)

var (
	metricsErrorsApplicationID string
	metricsErrorsEnvironmentID string
	metricsErrorsFrom          string
	metricsErrorsTo            string
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Metrics and aggregation operations",
}

var metricsErrorsCmd = &cobra.Command{
	Use:   "errors",
	Short: "Show error frequency by day",
	Long:  "Display the frequency of errors (severity=error) grouped by day. Useful for identifying error patterns over time.",
	RunE: func(cmd *cobra.Command, args []string) error {
		occurredFrom, err := parsePeriodValue(metricsErrorsFrom)
		if err != nil {
			return fmt.Errorf("invalid --from value: %w", err)
		}

		occurredTo, err := parsePeriodValue(metricsErrorsTo)
		if err != nil {
			return fmt.Errorf("invalid --to value: %w", err)
		}

		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		if err := repository.Ping(context.Background(), db); err != nil {
			return fmt.Errorf("failed to ping PostgreSQL: %w", err)
		}

		errorFreqRepo := repository.NewPostgresErrorFrequencyRepository(db)
		errorFreqUseCase := services.NewGetErrorFrequencyUseCase(errorFreqRepo)

		frequencies, err := errorFreqUseCase.Execute(context.Background(), services.GetErrorFrequencyInput{
			ApplicationID: strings.TrimSpace(metricsErrorsApplicationID),
			EnvironmentID: strings.TrimSpace(metricsErrorsEnvironmentID),
			OccurredFrom:  occurredFrom,
			OccurredTo:    occurredTo,
		})
		if err != nil {
			return err
		}

		printErrorFrequencyTable(cmd, frequencies)
		return nil
	},
}

func init() {
	metricsErrorsCmd.Flags().StringVar(&metricsErrorsApplicationID, "application-id", "", "Filter by application UUID")
	metricsErrorsCmd.Flags().StringVar(&metricsErrorsEnvironmentID, "environment-id", "", "Filter by environment UUID")
	metricsErrorsCmd.Flags().StringVar(&metricsErrorsFrom, "from", "", "Period start (RFC3339 or YYYY-MM-DD)")
	metricsErrorsCmd.Flags().StringVar(&metricsErrorsTo, "to", "", "Period end (RFC3339 or YYYY-MM-DD)")

	metricsErrorsCmd.Example = strings.Join([]string{
		"  # Show error frequency for all applications",
		"  tracelog metrics errors",
		"",
		"  # Show errors for a specific application in March 2026",
		"  tracelog metrics errors --application-id 11111111-1111-1111-1111-111111111111 --from 2026-03-01 --to 2026-03-31",
		"",
		"  # Show errors in production environment for last week",
		"  tracelog metrics errors --environment-id 22222222-2222-2222-2222-222222222222 --from 2026-03-01 --to 2026-03-09",
	}, "\n")

	metricsCmd.AddCommand(metricsErrorsCmd)
	rootCmd.AddCommand(metricsCmd)
}

func printErrorFrequencyTable(cmd *cobra.Command, frequencies []domain.ErrorFrequency) {
	if len(frequencies) == 0 {
		cmd.Println("No errors found for the provided filters.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "DATE\tERROR_COUNT")
	_, _ = fmt.Fprintln(w, "----\t-----------")

	totalErrors := int64(0)
	for _, freq := range frequencies {
		_, _ = fmt.Fprintf(w, "%s\t%d\n", freq.Date.Format("2006-01-02"), freq.Count)
		totalErrors += freq.Count
	}

	_ = w.Flush()

	cmd.Printf("\nSummary:\n")
	cmd.Printf("  Total days: %d\n", len(frequencies))
	cmd.Printf("  Total errors: %d\n", totalErrors)

	if len(frequencies) > 0 {
		avgErrors := float64(totalErrors) / float64(len(frequencies))
		cmd.Printf("  Average errors/day: %.1f\n", avgErrors)
	}
}
