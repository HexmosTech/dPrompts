package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog/log"
)

type BulkJob struct {
	Args     DPromptsJobArgs        `json:"args"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RunClient enqueues a job with args and metadata as JSON strings.
func RunClient(ctx context.Context, driver *riverpgxv5.Driver, argsJSON string, metadataJSON string, bulkFile string, dbPool *pgxpool.Pool) {
	riverClient, err := newRiverClient(driver)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create River client")
	}

	if bulkFile != "" {
		if err := enqueueBulkJobsFromFile(ctx, riverClient, dbPool, bulkFile); err != nil {
			log.Fatal().Err(err).Msg("Bulk insert failed")
		}
		return
	}

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

	if _, err := riverClient.Insert(ctx, &args, insertOpts); err != nil {
		log.Fatal().Err(err).Msg("Failed to enqueue job")
	}

	log.Info().
		Interface("args", args).
		Interface("metadata", insertOpts).
		Msg("Enqueued job")
}

func enqueueBulkJobsFromFile(ctx context.Context, riverClient *river.Client[pgx.Tx], dbPool *pgxpool.Pool, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	var jobs []BulkJob
	if err := json.NewDecoder(file).Decode(&jobs); err != nil {
		return err
	}

	tx, err := dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var jobsToInsert []river.InsertManyParams

	for i := range jobs {

		var insertOpts *river.InsertOpts
		if jobs[i].Metadata != nil {
			metadataBytes, err := json.Marshal(jobs[i].Metadata)
			if err != nil {
				return err
			}
			insertOpts = &river.InsertOpts{Metadata: metadataBytes}
		}
		jobsToInsert = append(jobsToInsert, river.InsertManyParams{
			Args:       jobs[i].Args,
			InsertOpts: insertOpts,
		})
	}

	results, err := riverClient.InsertManyTx(ctx, tx, jobsToInsert)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	log.Info().Msgf("Successfully enqueued %d jobs", len(results))
	return nil
}

func newRiverClient(driver *riverpgxv5.Driver) (*river.Client[pgx.Tx], error) {
	return river.NewClient[pgx.Tx](driver, &river.Config{})
}
