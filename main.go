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
	message := flag.String("message", "", "Job message (for client mode)")
	configPath := flag.String("config", "", "Path to config file (default: $HOME/.dprompt.toml)")

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
		RunClient(ctx, driver, *message)
	case "worker":
		RunWorker(ctx, driver, cancel)
	default:
		log.Fatal().Str("mode", *mode).Msg("Unknown mode")
	}
}
