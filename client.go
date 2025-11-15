package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog/log"
)

// runClientMode enqueues a job with the given message.
func RunClient(ctx context.Context, driver *riverpgxv5.Driver, message string) {
	if message == "" {
		log.Fatal().Msg("Message is required in client mode")
	}
	riverClient, err := createClient(driver)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create River client")
	}
	jobArgs := &DPromptsJobArgs{Message: message}
	if _, err := riverClient.Insert(ctx, jobArgs, nil); err != nil {
		log.Fatal().Err(err).Msg("Failed to enqueue job")
	}
	log.Info().Str("message", message).Msg("Enqueued job")
}

func createClient(driver *riverpgxv5.Driver) (*river.Client[pgx.Tx], error) {
	return river.NewClient[pgx.Tx](driver, &river.Config{})
}
