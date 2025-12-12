package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func viewLastResults(ctx context.Context, db *pgxpool.Pool, n int) error {
	rows, err := db.Query(ctx, `
        SELECT r.id, r.job_id, r.response, r.created_at, g.group_name
        FROM dprompts_results r
        LEFT JOIN dprompt_groups g ON r.group_id = g.id
        ORDER BY r.created_at DESC
        LIMIT $1
    `, n)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var jobID int64
		var responseData []byte
		var createdAt time.Time
		var groupName *string

		if err := rows.Scan(&id, &jobID, &responseData, &createdAt, &groupName); err != nil {
			return err
		}

		gn := "NULL"
		if groupName != nil {
			gn = *groupName
		}

		// Step 1: Unmarshal outer JSON
		var outer map[string]string
		if err := json.Unmarshal(responseData, &outer); err != nil {
			fmt.Printf("ID: %d | JobID: %d | Group: %s | CreatedAt: %s\nResponse: %s\n\n",
				id, jobID, gn, createdAt.Format(time.RFC3339), string(responseData))
			continue
		}

		// Step 2: Unmarshal inner "response" string
		var inner any
		if err := json.Unmarshal([]byte(outer["response"]), &inner); err != nil {
			// fallback: just print inner string raw
			fmt.Printf("ID: %d | JobID: %d | Group: %s | CreatedAt: %s\nResponse: %s\n\n",
				id, jobID, gn, createdAt.Format(time.RFC3339), outer["response"])
			continue
		}

		// Step 3: Pretty-print the inner JSON
		prettyInner, _ := json.MarshalIndent(inner, "", "  ")

		fmt.Printf("ID: %d | JobID: %d | Group: %s | CreatedAt: %s\nResponse:\n%s\n\n",
			id, jobID, gn, createdAt.Format(time.RFC3339), string(prettyInner))
	}

	return rows.Err()
}


// CLI: Display total number of groups
// CLI: Display all groups with ID and name
func viewTotalGroups(ctx context.Context, db *pgxpool.Pool) error {
	rows, err := db.Query(ctx, `
        SELECT id, group_name, created_at
        FROM dprompt_groups
        ORDER BY id
    `)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("Groups:")
	for rows.Next() {
		var id int
		var name string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &createdAt); err != nil {
			return err
		}
		fmt.Printf("ID: %d | Name: %s | CreatedAt: %s\n", id, name, createdAt.Format(time.RFC3339))
	}
	return rows.Err()
}


// CLI: Display results filtered by group ID
func viewResultsByGroup(ctx context.Context, db *pgxpool.Pool, groupID int) error {
	rows, err := db.Query(ctx, `
        SELECT r.id, r.job_id, r.response, r.created_at, g.group_name
        FROM dprompts_results r
        JOIN dprompt_groups g ON r.group_id = g.id
        WHERE g.id = $1
        ORDER BY r.created_at DESC
    `, groupID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var jobID int64
		var responseData []byte
		var createdAt time.Time
		var groupName string

		if err := rows.Scan(&id, &jobID, &responseData, &createdAt, &groupName); err != nil {
			return err
		}

		var outer map[string]string
		if err := json.Unmarshal(responseData, &outer); err != nil {
			fmt.Printf("ID: %d | JobID: %d | Group: %s | CreatedAt: %s\nResponse: %s\n\n",
				id, jobID, groupName, createdAt.Format(time.RFC3339), string(responseData))
			continue
		}

		var inner any
		if err := json.Unmarshal([]byte(outer["response"]), &inner); err != nil {
			fmt.Printf("ID: %d | JobID: %d | Group: %s | CreatedAt: %s\nResponse: %s\n\n",
				id, jobID, groupName, createdAt.Format(time.RFC3339), outer["response"])
			continue
		}

		prettyInner, _ := json.MarshalIndent(inner, "", "  ")
		fmt.Printf("ID: %d | JobID: %d | Group: %s | CreatedAt: %s\nResponse:\n%s\n\n",
			id, jobID, groupName, createdAt.Format(time.RFC3339), string(prettyInner))
	}

	return rows.Err()
}

