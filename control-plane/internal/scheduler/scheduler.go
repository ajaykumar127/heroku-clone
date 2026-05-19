package scheduler

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNoRuntimes is returned when no active runtime planes are registered.
var ErrNoRuntimes = errors.New("no active runtime planes registered")

// Job represents a build/deploy job dispatched to a runtime plane.
type Job struct {
	ID        string    `json:"id"`
	RuntimeID string    `json:"runtime_id"`
	AppID     string    `json:"app_id"`
	ReleaseID string    `json:"release_id"`
	AppName   string    `json:"app_name"`
	Commit    string    `json:"commit"`
	RepoURL   string    `json:"repo_url"`
	ImageName string    `json:"image_name"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	LogOutput string    `json:"log_output,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Scheduler picks a runtime plane and writes job records to the jobs table.
type Scheduler struct {
	db       *sql.DB
	postgres bool
}

// New creates a new Scheduler backed by db.
// Set postgres=true when the database is PostgreSQL; false for SQLite.
func New(db *sql.DB, postgres bool) *Scheduler {
	return &Scheduler{db: db, postgres: postgres}
}

// ph returns the SQL placeholder for position n (1-based).
// PostgreSQL uses $1, $2, …; SQLite uses ?.
func ph(postgres bool, n int) string {
	if postgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// generateID returns a new random UUID v4 string (stdlib only).
func generateID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Schedule picks a runtime plane and creates a pending Job for the given
// app + commit.  Selection rules:
//  1. If the app has a preferred runtime_id, use that runtime (must be active).
//  2. Otherwise pick the active runtime with the oldest last_seen_at
//     (least-recently-used load balancing).
//  3. If no active runtimes exist, return ErrNoRuntimes so the caller can
//     fall back to local build mode.
func (s *Scheduler) Schedule(appID, appName, commit, repoURL, registryURL string) (*Job, error) {
	// ------------------------------------------------------------------
	// 1. Determine target runtime
	// ------------------------------------------------------------------
	var runtimeID string

	// Check whether the app has a preferred runtime.
	var preferredRuntimeID sql.NullString
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT preferred_runtime_id FROM apps WHERE id = %s", ph(s.postgres, 1)),
		appID,
	).Scan(&preferredRuntimeID)
	if err != nil {
		return nil, fmt.Errorf("scheduler: look up app %q: %w", appID, err)
	}

	if preferredRuntimeID.Valid && preferredRuntimeID.String != "" {
		// Verify the preferred runtime is active.
		var activeStatus string
		err = s.db.QueryRow(
			fmt.Sprintf("SELECT status FROM runtimes WHERE id = %s", ph(s.postgres, 1)),
			preferredRuntimeID.String,
		).Scan(&activeStatus)
		if err != nil {
			return nil, fmt.Errorf("scheduler: look up preferred runtime %q: %w", preferredRuntimeID.String, err)
		}
		if activeStatus != "active" {
			return nil, fmt.Errorf("scheduler: preferred runtime %q is not active (status=%q)", preferredRuntimeID.String, activeStatus)
		}
		runtimeID = preferredRuntimeID.String
	} else {
		// Pick the active runtime with the oldest last_seen_at (LRU).
		err = s.db.QueryRow(
			"SELECT id FROM runtimes WHERE status = 'active' ORDER BY last_seen_at ASC LIMIT 1",
		).Scan(&runtimeID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRuntimes
		}
		if err != nil {
			return nil, fmt.Errorf("scheduler: pick runtime: %w", err)
		}
	}

	// ------------------------------------------------------------------
	// 2. Create a release record (status = pending)
	// ------------------------------------------------------------------
	var version int
	err = s.db.QueryRow(
		fmt.Sprintf(
			"SELECT COALESCE(MAX(version), 0) + 1 FROM releases WHERE app_id = %s",
			ph(s.postgres, 1),
		),
		appID,
	).Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("scheduler: compute release version: %w", err)
	}

	releaseID := generateID()
	now := time.Now().UTC()
	_, err = s.db.Exec(
		fmt.Sprintf(
			`INSERT INTO releases (id, app_id, version, "commit", status, build_output, created_at)
			 VALUES (%s, %s, %s, %s, 'pending', '', %s)`,
			ph(s.postgres, 1), ph(s.postgres, 2), ph(s.postgres, 3),
			ph(s.postgres, 4), ph(s.postgres, 5),
		),
		releaseID, appID, version, commit, now,
	)
	if err != nil {
		return nil, fmt.Errorf("scheduler: create release: %w", err)
	}

	// ------------------------------------------------------------------
	// 3. Create a job record (status = pending)
	// ------------------------------------------------------------------
	jobID := generateID()
	imageName := fmt.Sprintf("%s/%s:%s", registryURL, appName, commit)
	job := &Job{
		ID:        jobID,
		RuntimeID: runtimeID,
		AppID:     appID,
		ReleaseID: releaseID,
		AppName:   appName,
		Commit:    commit,
		RepoURL:   repoURL,
		ImageName: imageName,
		Type:      "build_deploy",
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = s.db.Exec(
		fmt.Sprintf(
			`INSERT INTO jobs
			   (id, runtime_id, app_id, release_id, app_name, "commit", repo_url, image_name, type, status, log_output, created_at, updated_at)
			 VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, '', %s, %s)`,
			ph(s.postgres, 1), ph(s.postgres, 2), ph(s.postgres, 3), ph(s.postgres, 4),
			ph(s.postgres, 5), ph(s.postgres, 6), ph(s.postgres, 7), ph(s.postgres, 8),
			ph(s.postgres, 9), ph(s.postgres, 10), ph(s.postgres, 11), ph(s.postgres, 12),
		),
		job.ID, job.RuntimeID, job.AppID, job.ReleaseID,
		job.AppName, job.Commit, job.RepoURL, job.ImageName,
		job.Type, job.Status, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scheduler: create job: %w", err)
	}

	return job, nil
}

// PendingJobs returns up to 10 pending jobs for the given runtime, atomically
// transitioning them to status 'building' before returning them.
func (s *Scheduler) PendingJobs(runtimeID string) ([]Job, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("scheduler: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// SELECT the pending jobs (with a write lock when supported).
	var selectSQL string
	if s.postgres {
		selectSQL = fmt.Sprintf(
			`SELECT id, runtime_id, app_id, release_id, app_name, "commit", repo_url, image_name, type, status, COALESCE(log_output,''), created_at, updated_at
			   FROM jobs
			  WHERE runtime_id = %s AND status = 'pending'
			  LIMIT 10
			    FOR UPDATE`,
			ph(s.postgres, 1),
		)
	} else {
		// SQLite does not support SELECT … FOR UPDATE.
		selectSQL = fmt.Sprintf(
			`SELECT id, runtime_id, app_id, release_id, app_name, "commit", repo_url, image_name, type, status, COALESCE(log_output,''), created_at, updated_at
			   FROM jobs
			  WHERE runtime_id = %s AND status = 'pending'
			  LIMIT 10`,
			ph(s.postgres, 1),
		)
	}

	rows, err := tx.Query(selectSQL, runtimeID)
	if err != nil {
		return nil, fmt.Errorf("scheduler: query pending jobs: %w", err)
	}

	var jobs []Job
	var ids []string
	for rows.Next() {
		var j Job
		if err = rows.Scan(
			&j.ID, &j.RuntimeID, &j.AppID, &j.ReleaseID,
			&j.AppName, &j.Commit, &j.RepoURL, &j.ImageName,
			&j.Type, &j.Status, &j.LogOutput,
			&j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scheduler: scan job row: %w", err)
		}
		jobs = append(jobs, j)
		ids = append(ids, j.ID)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("scheduler: iterate job rows: %w", err)
	}

	if len(ids) == 0 {
		_ = tx.Commit()
		return nil, nil
	}

	// UPDATE the selected jobs to 'building'.
	now := time.Now().UTC()
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = now
	for i, id := range ids {
		placeholders[i] = ph(s.postgres, i+2) // $2, $3, … (or ?)
		args[i+1] = id
	}

	updateSQL := fmt.Sprintf(
		`UPDATE jobs SET status = 'building', updated_at = %s WHERE id IN (%s)`,
		ph(s.postgres, 1),
		strings.Join(placeholders, ", "),
	)
	if _, err = tx.Exec(updateSQL, args...); err != nil {
		return nil, fmt.Errorf("scheduler: mark jobs building: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("scheduler: commit tx: %w", err)
	}

	// Reflect the status change in the returned structs.
	for i := range jobs {
		jobs[i].Status = "building"
		jobs[i].UpdatedAt = now
	}

	return jobs, nil
}

// UpdateJobStatus appends logOutput to the job's existing log, updates its
// status, and — when the status is terminal ('succeeded' or 'failed') — also
// updates the linked release.
func (s *Scheduler) UpdateJobStatus(runtimeID, jobID, status, logOutput string) error {
	now := time.Now().UTC()

	// Append to existing log_output with a newline separator (skip when empty).
	var appendExpr string
	if s.postgres {
		appendExpr = "CASE WHEN log_output = '' OR log_output IS NULL THEN $1 ELSE log_output || E'\\n' || $1 END"
	} else {
		appendExpr = "CASE WHEN log_output = '' OR log_output IS NULL THEN ? ELSE log_output || char(10) || ? END"
	}

	var updateSQL string
	var jobArgs []interface{}
	if s.postgres {
		updateSQL = fmt.Sprintf(
			`UPDATE jobs SET status = %s, log_output = %s, updated_at = %s
			  WHERE id = %s AND runtime_id = %s`,
			ph(s.postgres, 2), appendExpr, ph(s.postgres, 3),
			ph(s.postgres, 4), ph(s.postgres, 5),
		)
		jobArgs = []interface{}{logOutput, status, now, jobID, runtimeID}
	} else {
		// SQLite: appendExpr already uses two ? for the logOutput.
		updateSQL = fmt.Sprintf(
			`UPDATE jobs SET status = %s, log_output = %s, updated_at = %s
			  WHERE id = %s AND runtime_id = %s`,
			ph(s.postgres, 3), appendExpr, ph(s.postgres, 4),
			ph(s.postgres, 5), ph(s.postgres, 6),
		)
		jobArgs = []interface{}{logOutput, logOutput, status, now, jobID, runtimeID}
	}

	_, err := s.db.Exec(updateSQL, jobArgs...)
	if err != nil {
		return fmt.Errorf("scheduler: update job status: %w", err)
	}

	// For terminal statuses, propagate to the releases table.
	if status != "succeeded" && status != "failed" {
		return nil
	}

	// Fetch the current full log_output and release_id from the job.
	var releaseID, fullLog string
	err = s.db.QueryRow(
		fmt.Sprintf(
			"SELECT release_id, COALESCE(log_output, '') FROM jobs WHERE id = %s",
			ph(s.postgres, 1),
		),
		jobID,
	).Scan(&releaseID, &fullLog)
	if err != nil {
		return fmt.Errorf("scheduler: fetch release_id for job %q: %w", jobID, err)
	}

	_, err = s.db.Exec(
		fmt.Sprintf(
			"UPDATE releases SET status = %s, build_output = %s WHERE id = %s",
			ph(s.postgres, 1), ph(s.postgres, 2), ph(s.postgres, 3),
		),
		status, fullLog, releaseID,
	)
	if err != nil {
		return fmt.Errorf("scheduler: update release %q: %w", releaseID, err)
	}

	return nil
}
