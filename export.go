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
	Format     string
	OutDir     string
	FromDate   string
	FullExport bool
	DryRun     bool
	Overwrite  bool
}

type ExportResult struct {
	JobID     int64
	Response  []byte
	GroupName *string
	CreatedAt time.Time
}

func ExportResults(
	ctx context.Context,
	dbPool *pgxpool.Pool,
	opts ExportOptions,
) (int, error) {
	if opts.FullExport && opts.FromDate != "" {
		return 0, fmt.Errorf("--full-export cannot be used with --from-date")
	}

	start := time.Now()

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

	var (
		query string
		args  []any
	)

	if opts.FullExport {
		fmt.Println("Mode: full-export (no date filter)")
		query = `
			SELECT
				r.job_id,
				r.response,
				r.created_at,
				g.group_name
			FROM dprompts_results r
			LEFT JOIN dprompt_groups g
				ON r.group_id = g.id
			ORDER BY r.created_at ASC
		`
	} else {
		var fromTime time.Time

		if opts.FromDate != "" {
			fmt.Println("Mode: from-date", opts.FromDate)
			t, err := time.Parse("2006-01-02", opts.FromDate)
			if err != nil {
				return 0, fmt.Errorf("invalid --from-date format (expected YYYY-MM-DD)")
			}
			fromTime = t
		} else {
			fmt.Println("Mode: last-24-hours")
			fromTime = time.Now().Add(-24 * time.Hour)
		}

		query = `
			SELECT
				r.job_id,
				r.response,
				r.created_at,
				g.group_name
			FROM dprompts_results r
			LEFT JOIN dprompt_groups g
				ON r.group_id = g.id
			WHERE r.created_at >= $1
			ORDER BY r.created_at ASC
		`
		args = []any{fromTime}
	}

	// ---- count total matching rows ----
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) q", query)
	var totalMatched int
	if err := dbPool.QueryRow(ctx, countQuery, args...).Scan(&totalMatched); err != nil {
		return 0, err
	}

	skipped := len(existing)
	toExport := totalMatched - skipped
	if toExport < 0 {
		toExport = 0
	}

	fmt.Println("Matched jobs in DB:", totalMatched)
	fmt.Println("Already exported (skipped):", skipped)
	fmt.Println("Jobs to export:", toExport)
	fmt.Println()

	rows, err := dbPool.Query(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	exported := 0

	for rows.Next() {
		var r ExportResult
		if err := rows.Scan(
			&r.JobID,
			&r.Response,
			&r.CreatedAt,
			&r.GroupName,
		); err != nil {
			return exported, err
		}

		if _, ok := existing[r.JobID]; ok {
			continue
		}

		exported++

		if exported == 1 || exported%50 == 0 || exported == toExport {
			fmt.Printf(
				"Exporting %d/%d (job_id=%d)\n",
				exported,
				toExport,
				r.JobID,
			)
		}

		if !opts.DryRun {
			if err := writeExportFile(opts, r); err != nil {
				return exported, err
			}
		}
	}

	duration := time.Since(start)

	fmt.Println()
	fmt.Println("Export completed")
	fmt.Println("Exported:", exported)
	fmt.Println("Skipped:", skipped)
	fmt.Println("Duration:", duration.Round(time.Millisecond))

	return exported, rows.Err()
}

func writeExportFile(opts ExportOptions, r ExportResult) error {
	filename := fmt.Sprintf("%d.json", r.JobID)
	path := filepath.Join(opts.OutDir, filename)

	var resultValue any

	// Try JSON first
	var parsed any
	if err := json.Unmarshal(r.Response, &parsed); err == nil {
		resultValue = normalizeJSON(parsed)
	} else {
		// Not JSON → keep as text
		resultValue = string(r.Response)
	}

	out := map[string]any{
		"job_id":     r.JobID,
		"created_at": r.CreatedAt,
		"result":     resultValue,
		"metadata": map[string]any{
			"group_name": r.GroupName,
		},
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
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
