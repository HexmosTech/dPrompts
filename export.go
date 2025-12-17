package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ExportOptions struct {
	Format    string
	OutDir    string
	FromDate  string
	DryRun    bool
	Overwrite bool
}

type ExportResult struct {
	JobID     int64     `json:"job_id"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}

func ExportResults(
	ctx context.Context,
	dbPool *pgxpool.Pool,
	opts ExportOptions,
) (int, error) {

	// Ensure directory exists
	if err := os.MkdirAll(opts.OutDir, 0755); err != nil {
		return 0, err
	}

	// Handle overwrite
	if opts.Overwrite {
		if err := clearDirectory(opts.OutDir); err != nil {
			return 0, err
		}
	}

	// Scan existing exports (unless overwrite)
	existing := map[int64]struct{}{}
	if !opts.Overwrite {
		existing = scanExistingExports(opts.OutDir, opts.Format)
	}

	// Determine fromTime
	var fromTime time.Time
	if opts.FromDate != "" {
		t, err := time.Parse("2006-01-02", opts.FromDate)
		if err != nil {
			return 0, fmt.Errorf("invalid --from-date format (expected YYYY-MM-DD)")
		}
		fromTime = t
	} else {
		fromTime = time.Now().Add(-24 * time.Hour)
	}

	query := `
		SELECT job_id, response, created_at
		FROM dprompts_results
		WHERE created_at >= $1
		ORDER BY created_at ASC
	`
	fmt.Println(fromTime)
	rows, err := dbPool.Query(ctx, query, fromTime)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0

	for rows.Next() {
		var r ExportResult
		if err := rows.Scan(
			&r.JobID,
			&r.Result,
			&r.CreatedAt,
		); err != nil {
			return count, err
		}

		// Skip already exported jobs
		if _, ok := existing[r.JobID]; ok {
			continue
		}

		if !opts.DryRun {
			if err := writeExportFile(opts, r); err != nil {
				return count, err
			}
		}

		count++
	}

	return count, rows.Err()
}

func writeExportFile(opts ExportOptions, r ExportResult) error {
	filename := fmt.Sprintf("%d.%s", r.JobID, opts.Format)
	path := filepath.Join(opts.OutDir, filename)

	switch opts.Format {
	case "json":
		var parsed any
		if err := json.Unmarshal([]byte(r.Result), &parsed); err != nil {
			return fmt.Errorf("invalid JSON in result for job %d: %w", r.JobID, err)
		}
	
		normalized := normalizeJSON(parsed)
	
		out := map[string]any{
			"job_id":     r.JobID,
			"created_at": r.CreatedAt,
			"result":     normalized,
		}
	
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
	
		return os.WriteFile(path, data, 0644)

	case "txt":
		content := fmt.Sprintf(
			"Job ID: %d\nCreated At: %s\n\nResult:\n%s\n",
			r.JobID,
			r.CreatedAt.Format(time.RFC3339),
			r.Result,
		)
		return os.WriteFile(path, []byte(content), 0644)

	default:
		return fmt.Errorf("unsupported export format: %s", opts.Format)
	}
}

func scanExistingExports(dir, format string) map[int64]struct{} {
	result := make(map[int64]struct{})

	entries, err := os.ReadDir(dir)
	if err != nil {
		return result
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if !strings.HasSuffix(name, "."+format) {
			continue
		}

		base := strings.TrimSuffix(name, "."+format)
		jobID, err := strconv.ParseInt(base, 10, 64)
		if err == nil {
			result[jobID] = struct{}{}
		}
	}

	return result
}

func clearDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
