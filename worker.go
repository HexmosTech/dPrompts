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

	response, err := CallOllama(job.Args.Prompt, configPath)
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

	_, err = tx.Exec(ctx,
		"INSERT INTO dprompts_results (job_id, response) VALUES ($1, $2) ON CONFLICT (job_id) DO NOTHING",
		job.ID,
		jsonResponse,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store Ollama result in database")
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
		Workers: workers,
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
