package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog/log"
)

type DPromptsWorker struct {
	river.WorkerDefaults[DPromptsJobArgs]
}

func (w *DPromptsWorker) Work(ctx context.Context, job *river.Job[DPromptsJobArgs]) error {
	log.Info().
		Str("job_id", strconv.FormatInt(job.ID, 10)).
		Interface("args", job.Args).
		Msg("Processing job")
	return nil
}

func RegisterWorkers() *river.Workers {
	workers := river.NewWorkers()

	river.AddWorker(workers, &DPromptsWorker{})
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

func RunWorker(ctx context.Context, driver *riverpgxv5.Driver, cancel context.CancelFunc) {
	workers := RegisterWorkers()
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
