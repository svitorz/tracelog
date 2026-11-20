package cmd

import (
	"context"
	"fmt"
	"strings"

	"svitorz/tracelog/internal/adapters/repository"
	"svitorz/tracelog/internal/core/services"

	"github.com/spf13/cobra"
)

var importLogFiles []string

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import technical events from external sources",
}

var importLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Import log lines from one or more files",
	Long:  "Read one or more log files line by line, normalize the maximum possible number of records, categorize them, and persist them as technical events.",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := repository.OpenPostgres(appConfig.DB)
		if err != nil {
			return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
		}
		defer db.Close()

		if err := repository.Ping(context.Background(), db); err != nil {
			return fmt.Errorf("failed to ping PostgreSQL: %w", err)
		}

		setupRepo := repository.NewPostgresSystemSetupRepository(db)
		ensureSetupUseCase := services.NewEnsureSystemSetupUseCase(setupRepo)
		if err := ensureSetupUseCase.Execute(context.Background()); err != nil {
			return err
		}

		eventRepo := repository.NewPostgresTechnicalEventRepository(db)
		registerUseCase := services.NewRegisterTechnicalEventUseCase(eventRepo, setupRepo)
		importUseCase := services.NewImportLogFilesUseCase(registerUseCase)

		result, err := importUseCase.Execute(context.Background(), services.ImportLogFilesInput{
			Files: importLogFiles,
		})
		if err != nil {
			return err
		}

		printImportLogsSummary(cmd, result)
		return nil
	},
}

func init() {
	importLogsCmd.Flags().StringArrayVar(&importLogFiles, "file", nil, "Path to a log file to import; repeat the flag for multiple files")
	_ = importLogsCmd.MarkFlagRequired("file")

	importLogsCmd.Example = strings.Join([]string{
		"  tracelog import logs --file ./app.log",
		"  tracelog import logs --file ./app.log --file ./worker.log",
	}, "\n")

	importCmd.AddCommand(importLogsCmd)
	rootCmd.AddCommand(importCmd)
}

func printImportLogsSummary(cmd *cobra.Command, result *services.ImportLogFilesResult) {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintln(out, "Log import summary")
	_, _ = fmt.Fprintf(out, "Files processed: %d\n", result.FilesProcessed)
	_, _ = fmt.Fprintf(out, "Lines read: %d\n", result.LinesRead)
	_, _ = fmt.Fprintf(out, "Persisted: %d\n", result.Persisted)
	_, _ = fmt.Fprintf(out, "Blank lines skipped: %d\n", result.BlankSkipped)
	_, _ = fmt.Fprintf(out, "Normalized: %d\n", result.Normalized)
	_, _ = fmt.Fprintf(out, "Partially interpreted: %d\n", result.Partial)
	_, _ = fmt.Fprintf(out, "Uncategorizable: %d\n", result.Uncategorizable)

	if result.PersistenceError > 0 {
		_, _ = fmt.Fprintf(out, "Persistence errors: %d\n", result.PersistenceError)
		for _, lineErr := range result.Errors {
			_, _ = fmt.Fprintf(out, "  %s:%d: %s\n", lineErr.File, lineErr.Line, lineErr.Reason)
		}
		if hidden := result.PersistenceError - len(result.Errors); hidden > 0 {
			_, _ = fmt.Fprintf(out, "  ... and %d more\n", hidden)
		}
	}
}
