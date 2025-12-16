package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog/log"
)

type DPromptsWorker struct {
	river.WorkerDefaults[DPromptsJobArgs]
	db *pgxpool.Pool // Database pool
}

func (w *DPromptsWorker) Timeout(job *river.Job[DPromptsJobArgs]) time.Duration {
	// Set the timeout to 5 minutes
	return 5 * time.Minute
}

func humanizeDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", m, s)
	}
}

func (w *DPromptsWorker) Work(ctx context.Context, job *river.Job[DPromptsJobArgs]) error {
	jobStart := time.Now()
	jobID := strconv.FormatInt(job.ID, 10)

	var groupName string
	if len(job.Metadata) > 0 {
		var meta map[string]any
		if err := json.Unmarshal(job.Metadata, &meta); err == nil {
			if v, ok := meta["group_name"].(string); ok {
				groupName = v
			}
		}
	}

	log.Info().
		Str("job_id", jobID).
		Msg("Job started")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error().Err(err).Msg("Unable to determine home directory for config file")
		return err
	}
	configPath := homeDir + string(os.PathSeparator) + ".dprompts.toml"
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	groupID, err := w.resolveGroup(ctx, tx, jobID, groupName)
	if err != nil {
		return err
	}

	results := make(map[string]string)

	var ollamaTotal time.Duration
	var dbTotal time.Duration

	// ---- subtasks ----
	for i, sub := range job.Args.SubTasks {
		log.Info().
			Str("job_id", jobID).
			Int("subtask", i).
			Any("metadata", sub.Metadata).
			Msg("Subtask started")
		ollamaStart := time.Now()

		response, err := CallOllama(
			sub.Prompt,
			sub.Schema,
			configPath,
			job.Args.BasePrompt,
		)

		ollamaDur := time.Since(ollamaStart)
		ollamaTotal += ollamaDur

		if err != nil {
			log.Error().
				Err(err).
				Str("job_id", jobID).
				Int("subtask", i).
				Msg("Subtask failed")
			return err
		}

		results[fmt.Sprintf("subtask_%d", i)] = response

		log.Info().
			Str("job_id", jobID).
			Int("subtask", i).
			Str("time_taken_by_ollama", humanizeDuration(ollamaDur)).
			Msg("Subtask completed")
	}

	// ---- DB work ----
	dbStart := time.Now()

	jsonResponse, err := json.Marshal(results)
	if err != nil {
		return err
	}

	if err := w.insertResult(ctx, tx, job.ID, jsonResponse, groupID); err != nil {
		return err
	}

	if _, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	dbTotal += time.Since(dbStart)

	// ---- final summary ----
	totalTime := time.Since(jobStart)

	log.Info().
		Str("job_id", jobID).
		Int("subtasks", len(job.Args.SubTasks)).
		Str("ollama_total", humanizeDuration(ollamaTotal)).
		Str("db_total", humanizeDuration(dbTotal)).
		Str("total_time", humanizeDuration(totalTime)).
		Msg("Job completed")

	return nil
}

func RegisterWorkers(db *pgxpool.Pool) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, &DPromptsWorker{db: db})
	return workers
}

func createWorkerClient(
	driver *riverpgxv5.Driver,
	workers *river.Workers,
	concurrentWorkers int) (*river.Client[pgx.Tx], error) {
	log.Info().
		Int("concurrent_workers", concurrentWorkers).
		Msg("Initializing River worker client")
	return river.NewClient[pgx.Tx](driver, &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: concurrentWorkers},
		},
		Workers:                     workers,
		CompletedJobRetentionPeriod: 72 * time.Hour,
	})
}

func RunWorker(ctx context.Context, driver *riverpgxv5.Driver, cancel context.CancelFunc, db *pgxpool.Pool) {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to determine home directory")
	}

	configPath := homeDir + string(os.PathSeparator) + ".dprompts.toml"

	workerConfig, err := LoadWorkerConfig(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load worker config")
	}

	workers := RegisterWorkers(db)
	riverClient, err := createWorkerClient(driver, workers, workerConfig.ConcurrentWorkers)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create River client")
	}

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop

		log.Info().Msg("Shutting down...")
		if err := riverClient.Stop(ctx); err != nil {
			log.Fatal().Err(err).Msg("Failed to stop client")
		}
		cancel()
	}()

	log.Info().Msg("Worker started. Press Ctrl+C to exit.")
	if err := riverClient.Start(ctx); err != nil {
		if err != context.Canceled {
			log.Fatal().Err(err).Msg("Client service error")
		}
	}
	<-ctx.Done()
	log.Info().Msg("Worker shut down.")
}

// resolveGroup ensures the group exists and returns its ID (or nil if no group)
func (w *DPromptsWorker) resolveGroup(ctx context.Context, tx pgx.Tx, jobID string, groupName string) (*int, error) {
	if groupName == "" {
		log.Info().Str("job_id", jobID).Msg("No group name provided, skipping group creation")
		return nil, nil
	}

	var id int
	err := tx.QueryRow(ctx, `
		INSERT INTO dprompt_groups (group_name)
		VALUES ($1)
		ON CONFLICT (group_name) DO NOTHING
		RETURNING id
	`, groupName).Scan(&id)

	if err != nil {
		if err == pgx.ErrNoRows {
			// Group already exists, fetch its ID
			err = tx.QueryRow(ctx, `SELECT id FROM dprompt_groups WHERE group_name = $1`, groupName).Scan(&id)
			if err != nil {
				log.Error().Err(err).Str("job_id", jobID).Str("group_name", groupName).Msg("Failed to fetch existing group id")
				return nil, err
			}
		} else {
			log.Error().Err(err).Str("job_id", jobID).Str("group_name", groupName).Msg("Failed to create group")
			return nil, err
		}
	}

	return &id, nil
}

// insertResult inserts or updates a dprompt result for a job
func (w *DPromptsWorker) insertResult(ctx context.Context, tx pgx.Tx, jobID int64, jsonResponse []byte, groupID *int) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO dprompts_results (job_id, response, group_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (job_id)
		 DO UPDATE SET response = EXCLUDED.response,
					   group_id = EXCLUDED.group_id`,
		jobID,
		jsonResponse,
		groupID, // nil = NULL if no group
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store Ollama result in database")
		return err
	}
	return nil
}
