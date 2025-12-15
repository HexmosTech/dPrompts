package main

import (
	"context"
	"os"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	var configPath string

	rootCmd := &cobra.Command{
		Use:   "dpr",
		Short: "dpr CLI tool for job management",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if configPath == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				configPath = home + "/.dprompt.toml"
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: $HOME/.dprompt.toml)")

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
	deleteGroupCmd := &cobra.Command{
		Use:   "delete-group",
		Short: "Delete a group and its results",
		Run: func(cmd *cobra.Command, args []string) {
			if deleteGroupID == 0 {
				log.Fatal().Msg("Please provide --group-id")
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

	// ---- Queue subcommand ----
	var queueAction string
	var queueN int
	queueCmd := &cobra.Command{
		Use:   "queue",
		Short: "Queue operations",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			dbPool, err := NewDBPool(ctx, configPath)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to database")
			}
			defer dbPool.Close()

			switch queueAction {
			case "view":
				if err := ViewQueuedJobs(ctx, dbPool, queueN); err != nil {
					log.Fatal().Err(err).Msg("Failed to view queued jobs")
				}
			case "clear":
				if err := ClearQueuedJobs(ctx, dbPool); err != nil {
					log.Fatal().Err(err).Msg("Failed to clear queued jobs")
				}
			case "count":
				if err := CountQueuedJobs(ctx, dbPool); err != nil {
					log.Fatal().Err(err).Msg("Failed to count queued jobs")
				}
			case "completed-count":
				if err := CountCompletedJobs(ctx, dbPool); err != nil {
					log.Fatal().Err(err).Msg("Failed to count completed jobs")
				}
			case "completed-first":
				if err := ViewFirstCompletedJobs(ctx, dbPool, queueN); err != nil {
					log.Fatal().Err(err).Msg("Failed to view first completed jobs")
				}
			case "completed-last":
				if err := ViewLastCompletedJobs(ctx, dbPool, queueN); err != nil {
					log.Fatal().Err(err).Msg("Failed to view last completed jobs")
				}
			default:
				log.Fatal().Str("action", queueAction).Msg("Unknown queue action")
			}
		},
	}
	queueCmd.Flags().StringVar(&queueAction, "action", "", "Queue action: view | clear | count | completed-count | completed-first | completed-last")
	queueCmd.Flags().IntVar(&queueN, "queue-n", 10, "Number of jobs to display")

	// Add subcommands
	rootCmd.AddCommand(clientCmd, workerCmd, viewCmd, deleteGroupCmd, queueCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Command execution failed")
	}
}
