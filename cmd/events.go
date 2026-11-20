package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"svitorz/tracelog/internal/adapters/repository"
	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/services"

	"github.com/spf13/cobra"
)

var (
	eventsSeverity      string
	eventsApplicationID string
	eventsEnvironmentID string
	eventsFrom          string
	eventsTo            string
	eventsLimit         int
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Technical event operations",
}

var eventsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List technical events",
	Long:  "List technical events with basic filters (severity, application_id, environment_id, and period).",
	RunE: func(cmd *cobra.Command, args []string) error {
		occurredFrom, err := parsePeriodValue(eventsFrom)
		if err != nil {
			return fmt.Errorf("invalid --from value: %w", err)
		}

		occurredTo, err := parsePeriodEnd(eventsTo)
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

		eventRepo := repository.NewPostgresTechnicalEventRepository(db)
		listUseCase := services.NewListTechnicalEventsUseCase(eventRepo)

		events, err := listUseCase.Execute(context.Background(), services.ListTechnicalEventsInput{
			Severity:      domain.Severity(strings.TrimSpace(eventsSeverity)),
			ApplicationID: strings.TrimSpace(eventsApplicationID),
			EnvironmentID: strings.TrimSpace(eventsEnvironmentID),
			OccurredFrom:  occurredFrom,
			OccurredTo:    occurredTo,
			Limit:         eventsLimit,
		})
		if err != nil {
			return err
		}

		printEventsTable(cmd, events)
		return nil
	},
}

func init() {
	eventsListCmd.Flags().StringVar(&eventsSeverity, "severity", "", "Filter by severity (debug, info, warn, error, fatal)")
	eventsListCmd.Flags().StringVar(&eventsApplicationID, "application-id", "", "Filter by application UUID")
	eventsListCmd.Flags().StringVar(&eventsEnvironmentID, "environment-id", "", "Filter by environment UUID")
	eventsListCmd.Flags().StringVar(&eventsFrom, "from", "", "Period start (RFC3339 or YYYY-MM-DD)")
	eventsListCmd.Flags().StringVar(&eventsTo, "to", "", "Period end (RFC3339, or YYYY-MM-DD to include the whole day)")
	eventsListCmd.Flags().IntVar(&eventsLimit, "limit", 50, "Maximum number of events to return")

	eventsListCmd.Example = strings.Join([]string{
		"  tracelog events list",
		"  tracelog events list --severity error",
		"  tracelog events list --application-id 11111111-1111-1111-1111-111111111111",
		"  tracelog events list --environment-id 22222222-2222-2222-2222-222222222222 --from 2026-03-01 --to 2026-03-09",
		"  tracelog events list --severity warn --from 2026-03-09T00:00:00Z --to 2026-03-09T23:59:59Z --limit 20",
	}, "\n")

	eventsCmd.AddCommand(eventsListCmd)
	rootCmd.AddCommand(eventsCmd)
}

// parsePeriodValue parses a --from value: RFC3339, or YYYY-MM-DD as 00:00 UTC.
func parsePeriodValue(raw string) (*time.Time, error) {
	return parsePeriod(raw, false)
}

// parsePeriodEnd parses a --to value. YYYY-MM-DD covers the whole day (up to
// 23:59:59.999999999 UTC), so --to 2026-03-09 includes events of the 9th.
func parsePeriodEnd(raw string) (*time.Time, error) {
	return parsePeriod(raw, true)
}

func parsePeriod(raw string, endOfDay bool) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		v := t.UTC()
		return &v, nil
	}

	if t, err := time.Parse("2006-01-02", raw); err == nil {
		v := t.UTC()
		if endOfDay {
			v = v.Add(24*time.Hour - time.Nanosecond)
		}
		return &v, nil
	}

	return nil, fmt.Errorf("expected RFC3339 or YYYY-MM-DD")
}

func printEventsTable(cmd *cobra.Command, events []domain.TechnicalEvent) {
	if len(events) == 0 {
		cmd.Println("No events found for the provided filters.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "OCCURRED_AT\tSEVERITY\tAPPLICATION_ID\tENVIRONMENT_ID\tMESSAGE")
	_, _ = fmt.Fprintln(w, "-----------\t--------\t--------------\t--------------\t-------")

	for _, event := range events {
		message := strings.TrimSpace(event.Message)
		if len(message) > 100 {
			message = message[:97] + "..."
		}

		_, _ = fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\n",
			event.OccurredAt.UTC().Format(time.RFC3339),
			string(event.Severity),
			event.ApplicationID,
			event.EnvironmentID,
			message,
		)
	}

	_ = w.Flush()
	cmd.Printf("\nTotal: %d event(s).\n", len(events))
}
