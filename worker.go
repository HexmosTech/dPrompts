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

func (w *DPromptsWorker) Work(ctx context.Context, job *river.Job[DPromptsJobArgs]) error {
	jobID := strconv.FormatInt(job.ID, 10)
	log.Info().
		Str("job_id", jobID).
		Interface("args", job.Args).
		Msg("Starting job processing")

	// Log DB connection info
	if w.db != nil {
		config := w.db.Config()
		log.Info().
			Str("job_id", jobID).
			Str("db_conn_info", fmt.Sprintf("host=%s port=%d user=%s dbname=%s", 
				config.ConnConfig.Host, config.ConnConfig.Port, config.ConnConfig.User, config.ConnConfig.Database)).
			Msg("Using DB connection")
	} else {
		log.Warn().Str("job_id", jobID).Msg("DB pool is nil")
	}

	// Get config path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Unable to determine home directory for config file")
		return err
	}
	configPath := homeDir + string(os.PathSeparator) + ".dprompts.toml"
	log.Info().Str("job_id", jobID).Str("configPath", configPath).Msg("Using config path for Ollama")

	// Call Ollama
	response, err := CallOllama(job.Args.Prompt, job.Args.Schema, configPath)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Ollama call failed")
		return err
	}
	log.Info().Str("job_id", jobID).Str("response_preview", response).Msg("Ollama call successful")

	// Marshal JSON response
	jsonResponse, err := json.Marshal(map[string]string{"response": response})
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to marshal response as JSON")
		return err
	}
	log.Debug().Str("job_id", jobID).Str("jsonResponse", string(jsonResponse)).Msg("Marshaled JSON response")

	// Begin transaction
	tx, err := w.db.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to begin DB transaction")
		return err
	}
	log.Info().Str("job_id", jobID).Msg("Transaction started")
	defer func() {
		if tx != nil {
			if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
				log.Error().Err(err).Str("job_id", jobID).Msg("Failed to rollback transaction")
			} else {
				log.Debug().Str("job_id", jobID).Msg("Transaction rolled back (deferred)")
			}
		}
	}()

	// Create group if needed
	var groupID *int // nullable

	if job.Args.GroupName != "" {
		var id int
		// Create group if it doesn't exist, otherwise return existing id
		err = tx.QueryRow(ctx, `
			INSERT INTO dprompt_groups (group_name)
			VALUES ($1)
			ON CONFLICT (group_name) DO NOTHING
			RETURNING id
		`, job.Args.GroupName).Scan(&id)
	
		if err != nil {
			if err == pgx.ErrNoRows {
				// Group already exists, fetch its ID
				err = tx.QueryRow(ctx, `SELECT id FROM dprompt_groups WHERE group_name = $1`, job.Args.GroupName).Scan(&id)
				if err != nil {
					log.Error().Err(err).Str("job_id", jobID).Str("group_name", job.Args.GroupName).Msg("Failed to fetch existing group id")
					return err
				}
			} else {
				log.Error().Err(err).Str("job_id", jobID).Str("group_name", job.Args.GroupName).Msg("Failed to create group")
				return err
			}
		}
	
		groupID = &id
		log.Info().Str("job_id", jobID).Int("group_id", id).Msg("Resolved group")
	} else {
		log.Info().Str("job_id", jobID).Msg("No group name provided, skipping group creation")
	}
	
	// Insert result into dprompts_results
	res, err := tx.Exec(ctx,
		`INSERT INTO dprompts_results (job_id, response, group_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (job_id)
		 DO UPDATE SET response = EXCLUDED.response,
					   group_id = EXCLUDED.group_id`,
		job.ID,
		jsonResponse,
		groupID, // nil = NULL if no group
	)
	
	
	
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to insert result into dprompts_results")
		return err
	}
	log.Info().Str("job_id", jobID).Interface("pg_result", res).Msg("Insert executed")

	// Complete job transactionally with River
	completeRes, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to complete job transactionally with River")
		return err
	}
	log.Info().Str("job_id", jobID).Interface("completeRes", completeRes).Msg("Job completion executed")

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to commit transaction")
		return err
	}
	log.Info().Str("job_id", jobID).Msg("Transaction committed successfully, job finished")

	tx = nil // prevent deferred rollback
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
