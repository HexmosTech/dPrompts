package main

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog/log"
)


func RunClient(ctx context.Context, driver *riverpgxv5.Driver, argsJSON string, metadataJSON string) {
	if argsJSON == "" {
		log.Fatal().Msg("Args JSON is required in client mode")
	}

	var args DPromptsJobArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		log.Fatal().Err(err).Msg("Failed to parse args JSON")
	}

	var insertOpts *river.InsertOpts
	if metadataJSON != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			log.Fatal().Err(err).Msg("Failed to parse metadata JSON")
		}
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to marshal metadata to JSON bytes")
		}
		insertOpts = &river.InsertOpts{
			Metadata: metadataBytes,
		}
	}

	riverClient, err := newRiverClient(driver)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create River client")
	}

	if _, err := riverClient.Insert(ctx, &args, insertOpts); err != nil {
		log.Fatal().Err(err).Msg("Failed to enqueue job")
	}
	log.Info().Interface("args", args).Interface("metadata", insertOpts).Msg("Enqueued job")
}

func newRiverClient(driver *riverpgxv5.Driver) (*river.Client[pgx.Tx], error) {
	return river.NewClient[pgx.Tx](driver, &river.Config{})
}
