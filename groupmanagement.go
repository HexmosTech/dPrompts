package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DeleteGroupAndResults deletes a group by its ID and all associated results
func DeleteGroupAndResults(ctx context.Context, db *pgxpool.Pool, groupID int) error {
	// Delete associated results first
	res1, err := db.Exec(ctx, `DELETE FROM dprompts_results WHERE group_id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete results: %w", err)
	}

	// Delete the group itself
	res2, err := db.Exec(ctx, `DELETE FROM dprompt_groups WHERE id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	fmt.Printf("Deleted %d results and %d group(s) for group ID %d\n", res1.RowsAffected(), res2.RowsAffected(), groupID)
	return nil
}
