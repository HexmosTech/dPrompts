package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog/log"
)

type BulkJob struct {
	SubTasks   []DPromptsSubTask `json:"sub_tasks"`
	BasePrompt string            `json:"base_prompt,omitempty"`
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

	decoder := json.NewDecoder(bufio.NewReader(file))

	// Peek first non-whitespace token
	tok, err := nextNonSpaceToken(decoder)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	// NDJSON format (each line = JSON object)
	if tok != json.Delim('[') {
		return processNDJSON(ctx, decoder, riverClient, dbPool)
	}

	return processJSONArray(ctx, decoder, riverClient, dbPool)
}

// ------ JSON ARRAY VERSION ------
func processJSONArray(ctx context.Context, decoder *json.Decoder, riverClient *river.Client[pgx.Tx], dbPool *pgxpool.Pool) error {
	const batchSize = 500

	batch := make([]river.InsertManyParams, 0, batchSize)
	total := 0
	count := 0

	for decoder.More() {
		var job BulkJob
		if err := decoder.Decode(&job); err != nil {
			return fmt.Errorf("decode error at item %d: %w", total, err)
		}

		params, err := toInsertParams(job)
		if err != nil {
			log.Error().
				Int("job_index", total).
				Err(err).
				Msg("Failed to convert job to InsertManyParams")
			return err
		}

		batch = append(batch, params)
		total++
		count++

		if total%50 == 0 {
			log.Info().Msgf("Loaded %d jobs into batch...", total)
		}

		if count == batchSize {
			log.Info().Msgf("Inserting batch of %d jobs (total so far: %d)", batchSize, total)
			if err := insertBatch(ctx, riverClient, dbPool, batch); err != nil {
				log.Error().Err(err).Msg("Failed to insert batch")
				return err
			}
			batch = batch[:0]
			count = 0
		}
	}

	if len(batch) > 0 {
		log.Info().Msgf("Inserting final batch of %d jobs (total: %d)", len(batch), total)
		if err := insertBatch(ctx, riverClient, dbPool, batch); err != nil {
			log.Error().Err(err).Msg("Failed to insert final batch")
			return err
		}
	}

	log.Info().Msgf("Bulk insert complete. Total jobs inserted: %d", total)
	return nil
}

// ------ NDJSON VERSION ------
func processNDJSON(ctx context.Context, decoder *json.Decoder, riverClient *river.Client[pgx.Tx], dbPool *pgxpool.Pool) error {
	const batchSize = 500

	batch := make([]river.InsertManyParams, 0, batchSize)
	total := 0
	count := 0

	for {
		var job BulkJob
		if err := decoder.Decode(&job); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		params, err := toInsertParams(job)
		if err != nil {
			log.Error().
				Int("job_index", total).
				Err(err).
				Msg("Failed to convert job to InsertManyParams")
			return err
		}

		batch = append(batch, params)
		total++
		count++

		if total%50 == 0 {
			log.Info().Msgf("Loaded %d jobs into batch...", total)
		}

		if count == batchSize {
			log.Info().Msgf("Inserting batch of %d jobs (total so far: %d)", batchSize, total)
			if err := insertBatch(ctx, riverClient, dbPool, batch); err != nil {
				log.Error().Err(err).Msg("Failed to insert batch")
				return err
			}
			batch = batch[:0]
			count = 0
		}
	}

	if len(batch) > 0 {
		log.Info().Msgf("Inserting final batch of %d jobs (total: %d)", len(batch), total)
		if err := insertBatch(ctx, riverClient, dbPool, batch); err != nil {
			log.Error().Err(err).Msg("Failed to insert final batch")
			return err
		}
	}

	log.Info().Msgf("Bulk insert complete. Total jobs inserted: %d", total)
	return nil
}

// Helper: Read first non-space token
func nextNonSpaceToken(dec *json.Decoder) (json.Token, error) {
	for {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if _, ok := t.(json.Delim); ok || t != nil {
			return t, nil
		}
	}
}

func toInsertParams(job BulkJob) (river.InsertManyParams, error) {
	if len(job.SubTasks) == 0 {
		return river.InsertManyParams{}, fmt.Errorf("job has no sub_tasks")
	}

	for i, st := range job.SubTasks {
		if strings.TrimSpace(st.Prompt) == "" {
			return river.InsertManyParams{}, fmt.Errorf("sub_task[%d] has empty prompt", i)
		}
	}

	var opts *river.InsertOpts
	if job.SubTasks[0].Metadata != nil {
		metadataBytes, _ := json.Marshal(job.SubTasks[0].Metadata)
		opts = &river.InsertOpts{
			Metadata: metadataBytes,
		}
	}

	return river.InsertManyParams{
		Args: DPromptsJobArgs{
			BasePrompt: job.BasePrompt,
			SubTasks:   job.SubTasks,
		},
		InsertOpts: opts,
	}, nil
}

func insertBatch(
	ctx context.Context,
	riverClient *river.Client[pgx.Tx],
	dbPool *pgxpool.Pool,
	batch []river.InsertManyParams,
) error {

	tx, err := dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx) // safe no-op if already committed
	}()

	if _, err := riverClient.InsertManyTx(ctx, tx, batch); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func newRiverClient(driver *riverpgxv5.Driver) (*river.Client[pgx.Tx], error) {
	return river.NewClient[pgx.Tx](driver, &river.Config{})
}
