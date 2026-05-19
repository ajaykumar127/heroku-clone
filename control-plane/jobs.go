package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type Job struct {
	ID        string    `json:"id"`
	RuntimeID string    `json:"runtime_id"`
	AppID     string    `json:"app_id"`
	ReleaseID string    `json:"release_id"`
	AppName   string    `json:"app_name"`
	Commit    string    `json:"commit"`
	RepoURL   string    `json:"repo_url"`   // ssh://git@<host>:2222/<app>.git
	ImageName string    `json:"image_name"` // <registry>/<app>:<commit>
	Type      string    `json:"type"`       // build_and_deploy
	Status    string    `json:"status"`     // pending, building, deploying, succeeded, failed
	LogOutput string    `json:"log_output,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GET /internal/runtime/:runtime_id/jobs
// Agent polls for pending jobs. Returns the array and atomically marks them as "building".
func (s *APIServer) handlePollJobs(w http.ResponseWriter, r *http.Request) {
	runtimeID := mux.Vars(r)["runtime_id"]
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	// Fetch pending jobs for this runtime.
	rows, err := s.db.Query(
		fmt.Sprintf(
			`SELECT id, runtime_id, app_id, release_id, app_name, "commit", repo_url, image_name, type, status, log_output, created_at, updated_at
			 FROM jobs
			 WHERE runtime_id = %s AND status = 'pending'
			 ORDER BY created_at ASC`,
			ph(1),
		),
		runtimeID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		var j Job
		if err := rows.Scan(
			&j.ID, &j.RuntimeID, &j.AppID, &j.ReleaseID, &j.AppName,
			&j.Commit, &j.RepoURL, &j.ImageName, &j.Type, &j.Status,
			&j.LogOutput, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jobs = append(jobs, j)
	}
	rows.Close()

	// Mark fetched jobs as "building".
	now := time.Now()
	for _, j := range jobs {
		_, err := s.db.Exec(
			fmt.Sprintf("UPDATE jobs SET status = 'building', updated_at = %s WHERE id = %s", ph(1), ph(2)),
			now, j.ID,
		)
		if err != nil {
			log.Printf("[jobs] failed to mark job %s as building: %v", j.ID, err)
		} else {
			j.Status = "building"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// POST /internal/runtime/:runtime_id/jobs/:job_id/status
// Agent updates a job's status and appends to log output.
// When the job reaches succeeded or failed, the corresponding release is also updated.
func (s *APIServer) handleUpdateJobStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	runtimeID := vars["runtime_id"]
	jobID := vars["job_id"]
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	var req struct {
		Status    string `json:"status"`
		LogOutput string `json:"log_output"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	validStatuses := map[string]bool{
		"building":   true,
		"deploying":  true,
		"succeeded":  true,
		"failed":     true,
	}
	if !validStatuses[req.Status] {
		http.Error(w, "invalid status: must be building, deploying, succeeded, or failed", http.StatusBadRequest)
		return
	}

	// Fetch the current job to verify it belongs to this runtime and get its release_id.
	var job Job
	err := s.db.QueryRow(
		fmt.Sprintf(
			`SELECT id, runtime_id, app_id, release_id, app_name, "commit", repo_url, image_name, type, status, log_output, created_at, updated_at
			 FROM jobs WHERE id = %s AND runtime_id = %s`,
			ph(1), ph(2),
		),
		jobID, runtimeID,
	).Scan(
		&job.ID, &job.RuntimeID, &job.AppID, &job.ReleaseID, &job.AppName,
		&job.Commit, &job.RepoURL, &job.ImageName, &job.Type, &job.Status,
		&job.LogOutput, &job.CreatedAt, &job.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Append new log output.
	newLog := job.LogOutput
	if req.LogOutput != "" {
		if newLog != "" {
			newLog += "\n"
		}
		newLog += req.LogOutput
	}
	if len(newLog) > 64000 {
		newLog = newLog[:64000]
	}

	now := time.Now()
	_, err = s.db.Exec(
		fmt.Sprintf("UPDATE jobs SET status = %s, log_output = %s, updated_at = %s WHERE id = %s",
			ph(1), ph(2), ph(3), ph(4)),
		req.Status, newLog, now, jobID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// When terminal: propagate status to the release.
	if req.Status == "succeeded" || req.Status == "failed" {
		releaseStatus := req.Status
		_, err = s.db.Exec(
			fmt.Sprintf("UPDATE releases SET status = %s, build_output = %s WHERE id = %s",
				ph(1), ph(2), ph(3)),
			releaseStatus, newLog, job.ReleaseID,
		)
		if err != nil {
			log.Printf("[jobs] failed to update release %s: %v", job.ReleaseID, err)
		}
	}

	job.Status = req.Status
	job.LogOutput = newLog
	job.UpdatedAt = now

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}
