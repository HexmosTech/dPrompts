package main

import (
	"context"
	"flag"
	"os"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	mode := flag.String("mode", "worker", "Mode: 'client' to enqueue a job, 'worker' to run worker")
	argsJSON := flag.String("args", "", "Job args as JSON (for client mode)")
	metadataJSON := flag.String("metadata", "", "Job metadata as JSON (for client mode)")
	bulkFile := flag.String("bulk-from-file", "", "Bulk insert jobs from JSON file")
	configPath := flag.String("config", "", "Path to config file (default: $HOME/.dprompt.toml)")
	
	totalGroups := flag.Bool("total-groups", false, "Display total number of groups (view mode)")
	groupID := flag.Int("group", 0, "Display results for a specific group ID (view mode)")
	
	n := flag.Int("n", 10, "Number of results to display (view mode)")
	
	queueN := flag.Int("queue-n", 10, "Number of queued jobs to display (for view action)")
	queueAction := flag.String("action", "", "Queue action: 'view' to list queued jobs, 'clear' to delete all queued jobs")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbPool, err := NewDBPool(ctx, *configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer dbPool.Close()
	log.Info().Msg("Database connection successful.")

	driver := riverpgxv5.New(dbPool)

	switch *mode {
	case "client":
		RunClient(ctx, driver, *argsJSON, *metadataJSON, *bulkFile, dbPool)
	case "worker":
		RunWorker(ctx, driver, cancel, dbPool)
	case "view":
		if *totalGroups {
			if err := viewTotalGroups(ctx, dbPool); err != nil {
				log.Fatal().Err(err).Msg("Failed to get total groups")
			}
		} else if *groupID != 0 {
			if err := viewResultsByGroup(ctx, dbPool, *groupID); err != nil {
				log.Fatal().Err(err).Msg("Failed to get results by group")
			}
		} else {
			if err := viewLastResults(ctx, dbPool, *n); err != nil {
				log.Fatal().Err(err).Msg("Failed to get last results")
			}
		}
	case "queue":
		switch *queueAction {
		case "view":
			if err := ViewQueuedJobs(ctx, dbPool, *queueN); err != nil {
				log.Fatal().Err(err).Msg("Failed to view queued jobs")
			}
		case "clear":
			if err := ClearQueuedJobs(ctx, dbPool); err != nil {
				log.Fatal().Err(err).Msg("Failed to clear queued jobs")
			}
		default:
			log.Fatal().Str("action", *queueAction).Msg("Unknown queue action")
		}
	default:
		log.Fatal().Str("mode", *mode).Msg("Unknown mode")
	}
}
