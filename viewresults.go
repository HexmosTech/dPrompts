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

func normalizeJSON(v any) any {
	switch t := v.(type) {

	case map[string]any:
		for k, v2 := range t {
			t[k] = normalizeJSON(v2)
		}
		return t

	case []any:
		for i, v2 := range t {
			t[i] = normalizeJSON(v2)
		}
		return t

	case string:
		// Try to decode string as JSON
		var inner any
		if err := json.Unmarshal([]byte(t), &inner); err == nil {
			return normalizeJSON(inner)
		}
		// Not JSON → keep original string
		return t

	default:
		return t
	}
}

// CLI: Display total number of groups
// CLI: Display all groups with ID and name
func viewTotalGroups(ctx context.Context, db *pgxpool.Pool) error {
	rows, err := db.Query(ctx, `
        SELECT
            g.id,
            g.group_name,
            g.created_at,
            COUNT(r.id) AS job_count
        FROM dprompt_groups g
        LEFT JOIN dprompts_results r
            ON r.group_id = g.id
        GROUP BY g.id, g.group_name, g.created_at
        ORDER BY g.id
    `)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("Groups:")

	found := false
	for rows.Next() {
		found = true

		var (
			id        int
			name      string
			createdAt time.Time
			jobCount  int
		)

		if err := rows.Scan(&id, &name, &createdAt, &jobCount); err != nil {
			return err
		}

		fmt.Printf(
			"ID: %d | Name: %s | Jobs: %d | CreatedAt: %s\n",
			id,
			name,
			jobCount,
			createdAt.Format(time.RFC3339),
		)
	}

	if !found {
		fmt.Println("No groups found")
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
		var (
			id          int
			jobID       int64
			responseRaw []byte
			createdAt   time.Time
			groupName   string
		)

		if err := rows.Scan(&id, &jobID, &responseRaw, &createdAt, &groupName); err != nil {
			return err
		}

		fmt.Printf(
			"ID: %d | JobID: %d | Group: %s | CreatedAt: %s\n",
			id,
			jobID,
			groupName,
			createdAt.Format(time.RFC3339),
		)

		// Try to decode top-level JSON
		var data any
		if err := json.Unmarshal(responseRaw, &data); err != nil {
			// Not JSON at all → print raw string
			fmt.Printf("Response:\n%s\n\n", string(responseRaw))
			continue
		}

		normalized := normalizeJSON(data)

		pretty, err := json.MarshalIndent(normalized, "", "  ")
		if err != nil {
			fmt.Printf("Response:\n%v\n\n", normalized)
			continue
		}

		fmt.Printf("Response:\n%s\n\n", pretty)
	}

	return rows.Err()
}
