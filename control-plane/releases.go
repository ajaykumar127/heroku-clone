package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type Release struct {
	ID          string    `json:"id"`
	AppID       string    `json:"app_id"`
	Version     int       `json:"version"`
	Commit      string    `json:"commit"`
	Status      string    `json:"status"`
	BuildOutput string    `json:"build_output,omitempty"`
	RuntimeID   string    `json:"runtime_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *APIServer) handleListReleases(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	var appID string
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id FROM apps WHERE name = %s", ph(1)), name,
	).Scan(&appID)
	if err == sql.ErrNoRows {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT id, app_id, version, "commit", status, COALESCE(runtime_id, ''), created_at FROM releases WHERE app_id = %s ORDER BY version DESC`, ph(1)),
		appID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	releases := []Release{}
	for rows.Next() {
		var rel Release
		if err := rows.Scan(&rel.ID, &rel.AppID, &rel.Version, &rel.Commit, &rel.Status, &rel.RuntimeID, &rel.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		releases = append(releases, rel)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(releases)
}

func (s *APIServer) handleCreateRelease(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var req struct {
		Commit string `json:"commit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Commit == "" {
		http.Error(w, "commit is required", http.StatusBadRequest)
		return
	}

	releaseID, jobID, err := s.buildManager.TriggerBuild(name, req.Commit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]string{
		"release_id": releaseID,
		"status":     "building",
	}
	if jobID != "" {
		resp["job_id"] = jobID
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (s *APIServer) handleReleaseLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	releaseID := vars["release_id"]

	status, output, err := s.buildManager.GetBuildLogs(releaseID)
	if err != nil {
		http.Error(w, "release not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"release_id": releaseID,
		"status":     status,
		"output":     output,
	})
}
