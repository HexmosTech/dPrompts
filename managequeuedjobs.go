package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
