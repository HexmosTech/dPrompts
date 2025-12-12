package main

import (
	"context"
	"encoding/json"
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

func (w *DPromptsWorker) Work(ctx context.Context, job *river.Job[DPromptsJobArgs]) error {
	jobID := strconv.FormatInt(job.ID, 10)
	log.Info().
		Str("job_id", strconv.FormatInt(job.ID, 10)).
		Interface("args", job.Args).
		Msg("Processing job")


	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error().Err(err).Msg("Unable to determine home directory for config file")
		return err
	}
	configPath := homeDir + string(os.PathSeparator) + ".dprompts.toml"

	// Call Ollama
	response, err := CallOllama(job.Args.Prompt, job.Args.Schema, configPath)
	if err != nil {
		log.Error().Err(err).Msg("Ollama call failed")
		return err
	}
	log.Info().
		Str("job_id", strconv.FormatInt(job.ID, 10)).
		Msg("Ollama call successful, saving to DB")
	
	jsonResponse, err := json.Marshal(map[string]string{"response": response})
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal response as JSON")
		return err
	}

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get Group ID if it exists
	groupID, err := w.resolveGroup(ctx, tx, jobID, job.Args.GroupName)
	if err != nil {
		return err
	}
	
	// Insert the results
	if err := w.insertResult(ctx, tx, job.ID, jsonResponse, groupID); err != nil {
		return err
	}
	

	_, err = river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
	if err != nil {
		log.Error().Err(err).Msg("Failed to complete job transactionally")
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to commit transaction")
		return err
	}

	log.Info().
		Str("job_id", strconv.FormatInt(job.ID, 10)).
		Msg("Job completed and saved")
	return nil
}

func RegisterWorkers(db *pgxpool.Pool) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, &DPromptsWorker{db: db})
	return workers
}

func createWorkerClient(driver *riverpgxv5.Driver, workers *river.Workers) (*river.Client[pgx.Tx], error) {
	return river.NewClient[pgx.Tx](driver, &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		Workers:                     workers,
		CompletedJobRetentionPeriod: 72 * time.Hour,
	})
}

func RunWorker(ctx context.Context, driver *riverpgxv5.Driver, cancel context.CancelFunc, db *pgxpool.Pool) {
	workers := RegisterWorkers(db)
	riverClient, err := createWorkerClient(driver, workers)
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

	log.Info().Str("job_id", jobID).Int("group_id", id).Msg("Resolved group")
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


