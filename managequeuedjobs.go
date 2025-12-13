package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CountQueuedJobs(ctx context.Context, db *pgxpool.Pool) error {
	var count int64

	err := db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM river_job
		WHERE state IN ('available', 'scheduled')
	`).Scan(&count)

	if err != nil {
		return err
	}

	fmt.Printf("Total queued jobs remaining: %d\n", count)
	return nil
}


func ViewQueuedJobs(ctx context.Context, db *pgxpool.Pool, n int) error {
	rows, err := db.Query(ctx, `
		SELECT id, state, created_at, scheduled_at
		FROM river_job
		WHERE state IN ('available', 'scheduled')
		ORDER BY created_at DESC
		LIMIT $1
	`, n)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("Last %d queued jobs:\n", n)
	for rows.Next() {
		var id int64
		var state string
		var createdAt, scheduledAt time.Time
		if err := rows.Scan(&id, &state, &createdAt, &scheduledAt); err != nil {
			return err
		}
		fmt.Printf("ID: %d | State: %s | CreatedAt: %s | ScheduledAt: %s\n",
			id, state, createdAt.Format(time.RFC3339), scheduledAt.Format(time.RFC3339))
	}
	return rows.Err()
}

func ClearQueuedJobs(ctx context.Context, db *pgxpool.Pool) error {
	res, err := db.Exec(ctx, `
		DELETE FROM river_job
		WHERE state IN ('available', 'scheduled')
	`)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted %d queued jobs\n", res.RowsAffected())
	return nil
}

func CountCompletedJobs(ctx context.Context, db *pgxpool.Pool) error {
	var count int64

	err := db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM river_job
		WHERE state = 'completed'
	`).Scan(&count)

	if err != nil {
		return err
	}

	fmt.Printf("Total completed jobs: %d\n", count)
	return nil
}

func ViewFirstCompletedJobs(ctx context.Context, db *pgxpool.Pool, n int) error {
	rows, err := db.Query(ctx, `
		SELECT id, created_at, finalized_at, args
		FROM river_job
		WHERE state = 'completed'
		ORDER BY finalized_at ASC
		LIMIT $1
	`, n)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("First %d completed jobs:\n", n)
	for rows.Next() {
		var (
			id          int64
			createdAt   time.Time
			finalizedAt time.Time
			args        []byte
		)

		if err := rows.Scan(&id, &createdAt, &finalizedAt, &args); err != nil {
			return err
		}

		fmt.Printf(
			"ID: %d | CreatedAt: %s | CompletedAt: %s | Args: %s\n",
			id,
			createdAt.Format(time.RFC3339),
			finalizedAt.Format(time.RFC3339),
			string(args),
		)
	}

	return rows.Err()
}



func ViewLastCompletedJobs(ctx context.Context, db *pgxpool.Pool, n int) error {
	rows, err := db.Query(ctx, `
		SELECT id, created_at, finalized_at, args
		FROM river_job
		WHERE state = 'completed'
		ORDER BY finalized_at DESC
		LIMIT $1
	`, n)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("Last %d completed jobs:\n", n)
	for rows.Next() {
		var (
			id          int64
			createdAt   time.Time
			finalizedAt time.Time
			args        []byte
		)

		if err := rows.Scan(&id, &createdAt, &finalizedAt, &args); err != nil {
			return err
		}

		fmt.Printf(
			"ID: %d | CreatedAt: %s | CompletedAt: %s | Args: %s\n",
			id,
			createdAt.Format(time.RFC3339),
			finalizedAt.Format(time.RFC3339),
			string(args),
		)
	}

	return rows.Err()
}


