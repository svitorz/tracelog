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
	cmd.Println("Log import summary")
	cmd.Printf("Files processed: %d\n", result.FilesProcessed)
	cmd.Printf("Lines read: %d\n", result.LinesRead)
	cmd.Printf("Persisted: %d\n", result.Persisted)
	cmd.Printf("Blank lines skipped: %d\n", result.BlankSkipped)
	cmd.Printf("Normalized: %d\n", result.Normalized)
	cmd.Printf("Partially interpreted: %d\n", result.Partial)
	cmd.Printf("Uncategorizable: %d\n", result.Uncategorizable)

	if result.PersistenceError > 0 {
		cmd.Printf("Persistence errors: %d\n", result.PersistenceError)
	}
}
