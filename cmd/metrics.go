package cmd

import (
	"context"
	"fmt"
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
	Short: "Show error and fatal frequency by day",
	Long:  "Display the number of error and fatal events grouped by UTC day. Days without error or fatal events are omitted.",
	RunE: func(cmd *cobra.Command, args []string) error {
		occurredFrom, err := parsePeriodValue(metricsErrorsFrom)
		if err != nil {
			return fmt.Errorf("invalid --from value: %w", err)
		}

		occurredTo, err := parsePeriodEnd(metricsErrorsTo)
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
	metricsErrorsCmd.Flags().StringVar(&metricsErrorsTo, "to", "", "Period end (RFC3339, or YYYY-MM-DD to include the whole day)")

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
	out := cmd.OutOrStdout()
	if len(frequencies) == 0 {
		_, _ = fmt.Fprintln(out, "No errors found for the provided filters.")
		return
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "DATE\tERRORS\tFATAL\tTOTAL")
	_, _ = fmt.Fprintln(w, "----\t------\t-----\t-----")

	var totalErrors, totalFatal int64
	for _, freq := range frequencies {
		_, _ = fmt.Fprintf(w, "%s\t%d\t%d\t%d\n", freq.Date.Format("2006-01-02"), freq.Errors, freq.Fatal, freq.Total())
		totalErrors += freq.Errors
		totalFatal += freq.Fatal
	}

	_ = w.Flush()

	total := totalErrors + totalFatal
	_, _ = fmt.Fprintf(out, "\nSummary:\n")
	_, _ = fmt.Fprintf(out, "  Days with errors: %d\n", len(frequencies))
	_, _ = fmt.Fprintf(out, "  Errors: %d\n", totalErrors)
	_, _ = fmt.Fprintf(out, "  Fatal: %d\n", totalFatal)
	_, _ = fmt.Fprintf(out, "  Average (error+fatal) per day with errors: %.1f\n", float64(total)/float64(len(frequencies)))
}
