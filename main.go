package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	var configPath string

	rootCmd := &cobra.Command{
		Use: "dpr",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if configPath == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				configPath = home + "/.dprompts.toml"
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: $HOME/.dprompts.toml)")

	// ---- Client subcommand ----
	var argsJSON, metadataJSON, bulkFile string
	clientCmd := &cobra.Command{
		Use:   "client",
		Short: "Enqueue a job",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()
			driver := riverpgxv5.New(dbPool)
			RunClient(ctx, driver, argsJSON, metadataJSON, bulkFile, dbPool)
		},
	}
	clientCmd.Flags().StringVar(&argsJSON, "args", "", "Job args as JSON")
	clientCmd.Flags().StringVar(&metadataJSON, "metadata", "", "Job metadata as JSON")
	clientCmd.Flags().StringVar(&bulkFile, "bulk-from-file", "", "Bulk insert jobs from JSON file")

	// ---- Worker subcommand ----
	workerCmd := &cobra.Command{
		Use:   "worker",
		Short: "Run the worker",
		Run: func(cmd *cobra.Command, args []string) {

			if !isOllamaRunning() {
				log.Warn().Msg("Ollama server is not running")

				if !askForConfirmation("Ollama is not running. Do you want me to start it for you?") {
					log.Fatal().Msg("Ollama is required to run the worker")
				}

				log.Info().Msg("Starting Ollama...")
				if err := startOllama(); err != nil {
					log.Fatal().Err(err).Msg("Failed to start Ollama")
				}

				log.Info().Msg("Waiting for Ollama to become ready...")
				if err := waitForOllama(15 * time.Second); err != nil {
					log.Fatal().Err(err).Msg("Ollama did not become ready")
				}

				log.Info().Msg("Ollama is running")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()

			driver := riverpgxv5.New(dbPool)
			RunWorker(ctx, driver, cancel, dbPool)
		},
	}

	// ---- View subcommand ----
	var totalGroups bool
	var groupID, n int
	viewCmd := &cobra.Command{
		Use:   "view",
		Short: "View results",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()

			if totalGroups {
				if err := viewTotalGroups(ctx, dbPool); err != nil {
					log.Fatal().Err(err).Msg("Failed to get total groups")
				}
			} else if groupID != 0 {
				if err := viewResultsByGroup(ctx, dbPool, groupID); err != nil {
					log.Fatal().Err(err).Msg("Failed to get results by group")
				}
			} else {
				if err := viewLastResults(ctx, dbPool, n); err != nil {
					log.Fatal().Err(err).Msg("Failed to get last results")
				}
			}
		},
	}
	viewCmd.Flags().BoolVar(&totalGroups, "total-groups", false, "Display total number of groups")
	viewCmd.Flags().IntVar(&groupID, "group", 0, "Display results for a specific group ID")
	viewCmd.Flags().IntVarP(&n, "number", "n", 10, "Number of results to display")

	// ---- Delete-group subcommand ----
	var deleteGroupID int
	// ---- Delete-group subcommand ----

	deleteGroupCmd := &cobra.Command{
		Use:   "delete-group",
		Short: "Delete a group and its results",
		Run: func(cmd *cobra.Command, args []string) {
			if deleteGroupID == 0 {
				log.Fatal().Msg("Please provide --group-id")
			}

			if !askForConfirmation(fmt.Sprintf("Are you sure you want to delete group %d and all its results?", deleteGroupID)) {
				log.Info().Msg("Deletion cancelled by user")
				return
			}

			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()
			if err := DeleteGroupAndResults(ctx, dbPool, deleteGroupID); err != nil {
				log.Fatal().Err(err).Msg("Failed to delete group and results")
			}
			log.Info().Int("group_id", deleteGroupID).Msg("Deleted group and associated results")
		},
	}
	deleteGroupCmd.Flags().IntVar(&deleteGroupID, "group-id", 0, "Group ID to delete")

	// ---- Queue subcommands ----
	var queueN int

	queueCmd := &cobra.Command{
		Use:   "queue",
		Short: "Queue operations",
	}

	queueViewCmd := &cobra.Command{
		Use:   "view",
		Short: "View queued jobs",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()
			if err := ViewQueuedJobs(ctx, dbPool, queueN); err != nil {
				log.Fatal().Err(err).Msg("Failed to view queued jobs")
			}
		},
	}
	queueViewCmd.Flags().IntVarP(&queueN, "number", "n", 10, "Number of jobs to display")

	queueCountCmd := &cobra.Command{
		Use:   "count",
		Short: "Count queued jobs",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()
			if err := CountQueuedJobs(ctx, dbPool); err != nil {
				log.Fatal().Err(err).Msg("Failed to count queued jobs")
			}
		},
	}

	queueClearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear queued jobs",
		Run: func(cmd *cobra.Command, args []string) {
			if !askForConfirmation("Are you sure you want to clear all queued jobs?") {
				log.Info().Msg("Clearing queued jobs cancelled by user")
				return
			}

			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()
			if err := ClearQueuedJobs(ctx, dbPool); err != nil {
				log.Fatal().Err(err).Msg("Failed to clear queued jobs")
			}
			log.Info().Msg("All queued jobs cleared")
		},
	}

	queueFailedCmd := &cobra.Command{
		Use:   "failed-attempts",
		Short: "View jobs with failed attempts",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()
			if err := ViewJobsWithFailedAttempts(ctx, dbPool, queueN); err != nil {
				log.Fatal().Err(err).Msg("Failed to view jobs with failed attempts")
			}
		},
	}
	queueFailedCmd.Flags().IntVarP(&queueN, "number", "n", 10, "Number of jobs to display")

	// Completed jobs subcommand
	queueCompletedCmd := &cobra.Command{
		Use:   "completed",
		Short: "Completed job operations",
	}

	queueCompletedCountCmd := &cobra.Command{
		Use:   "count",
		Short: "Count completed jobs",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()
			if err := CountCompletedJobs(ctx, dbPool); err != nil {
				log.Fatal().Err(err).Msg("Failed to count completed jobs")
			}
		},
	}

	queueCompletedFirstCmd := &cobra.Command{
		Use:   "first",
		Short: "View first completed jobs",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()
			if err := ViewFirstCompletedJobs(ctx, dbPool, queueN); err != nil {
				log.Fatal().Err(err).Msg("Failed to view first completed jobs")
			}
		},
	}
	queueCompletedFirstCmd.Flags().IntVarP(&queueN, "number", "n", 10, "Number of jobs to display")
	queueCompletedLastCmd := &cobra.Command{
		Use:   "last",
		Short: "View last completed jobs",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()
			if err := ViewLastCompletedJobs(ctx, dbPool, queueN); err != nil {
				log.Fatal().Err(err).Msg("Failed to view last completed jobs")
			}
		},
	}
	queueCompletedLastCmd.Flags().IntVarP(&queueN, "number", "n", 10, "Number of jobs to display")
	queueCompletedCmd.AddCommand(queueCompletedCountCmd, queueCompletedFirstCmd, queueCompletedLastCmd)
	queueCmd.AddCommand(queueViewCmd, queueCountCmd, queueClearCmd, queueFailedCmd, queueCompletedCmd)

	// ---- Export subcommand ----
	var (
		exportFormat     string
		exportOutDir     string
		exportFromDate   string
		exportDryRun     bool
		exportOverwrite  bool
		exportFullExport bool
	)

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export dprompts results to files",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()

			count, err := ExportResults(ctx, dbPool, ExportOptions{
				Format:     exportFormat,
				OutDir:     exportOutDir,
				FromDate:   exportFromDate,
				FullExport: exportFullExport,
				DryRun:     exportDryRun,
				Overwrite:  exportOverwrite,
			})

			if err != nil {
				log.Fatal().Err(err).Msg("Export failed")
			}

			log.Info().
				Int("exported_count", count).
				Msg("Export completed successfully")
		},
	}

	exportCmd.Flags().StringVar(&exportFormat, "format", "json", "Export format: json | txt")
	exportCmd.Flags().StringVar(&exportOutDir, "out", "./dprompts_exports", "Output directory")
	exportCmd.Flags().StringVar(&exportFromDate, "from-date", "", "Export results created after this date (YYYY-MM-DD)")
	exportCmd.Flags().BoolVar(&exportDryRun, "dry-run", false, "Show what would be exported without writing files")
	exportCmd.Flags().BoolVar(&exportOverwrite, "overwrite", false, "Overwrite existing exported files")
	exportCmd.Flags().BoolVar(
		&exportFullExport,
		"full-export",
		false,
		"Export all results (ignores --from-date)",
	)

	// Add subcommands
	rootCmd.AddCommand(clientCmd, workerCmd, viewCmd, deleteGroupCmd, queueCmd, exportCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Command execution failed")
	}
}
