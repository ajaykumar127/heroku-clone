package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type Runtime struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Cloud        string    `json:"cloud"`  // aws, gcp, azure, local
	Region       string    `json:"region"`
	AgentURL     string    `json:"agent_url"`
	Status       string    `json:"status"` // active, inactive
	RegisteredAt time.Time `json:"registered_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// POST /internal/runtime/register
// Agent registers itself on startup.
func (s *APIServer) handleRegisterRuntime(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Cloud    string `json:"cloud"`
		Region   string `json:"region"`
		AgentURL string `json:"agent_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Cloud == "" || req.Region == "" {
		http.Error(w, "name, cloud, and region are required", http.StatusBadRequest)
		return
	}

	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	now := time.Now()

	// Upsert: if a runtime with the same name exists, update it; otherwise insert.
	if pg {
		// PostgreSQL upsert
		_, err := s.db.Exec(
			`INSERT INTO runtimes (id, name, cloud, region, agent_url, status, registered_at, last_seen_at)
			 VALUES ($1, $2, $3, $4, $5, 'active', $6, $7)
			 ON CONFLICT (name) DO UPDATE SET
			   cloud = EXCLUDED.cloud,
			   region = EXCLUDED.region,
			   agent_url = EXCLUDED.agent_url,
			   status = 'active',
			   last_seen_at = EXCLUDED.last_seen_at`,
			generateID(), req.Name, req.Cloud, req.Region, req.AgentURL, now, now,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// SQLite upsert
		_, err := s.db.Exec(
			`INSERT OR REPLACE INTO runtimes (id, name, cloud, region, agent_url, status, registered_at, last_seen_at)
			 VALUES (
			   COALESCE((SELECT id FROM runtimes WHERE name = ?), ?),
			   ?, ?, ?, ?, 'active', ?, ?
			 )`,
			req.Name, generateID(),
			req.Name, req.Cloud, req.Region, req.AgentURL, now, now,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Fetch the resulting runtime record.
	var rt Runtime
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id, name, cloud, region, agent_url, status, registered_at, last_seen_at FROM runtimes WHERE name = %s", ph(1)),
		req.Name,
	).Scan(&rt.ID, &rt.Name, &rt.Cloud, &rt.Region, &rt.AgentURL, &rt.Status, &rt.RegisteredAt, &rt.LastSeenAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	auditLog(r, "runtime.register", req.Name, req.Cloud+"/"+req.Region)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rt)
}

// GET /v1/runtimes
// List all registered runtimes (authenticated).
func (s *APIServer) handleListRuntimes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, name, cloud, region, agent_url, status, registered_at, last_seen_at FROM runtimes ORDER BY registered_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	runtimes := []Runtime{}
	for rows.Next() {
		var rt Runtime
		if err := rows.Scan(&rt.ID, &rt.Name, &rt.Cloud, &rt.Region, &rt.AgentURL, &rt.Status, &rt.RegisteredAt, &rt.LastSeenAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		runtimes = append(runtimes, rt)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runtimes)
}

// GET /v1/runtimes/:id
// Get a single runtime by ID.
func (s *APIServer) handleGetRuntime(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	var rt Runtime
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id, name, cloud, region, agent_url, status, registered_at, last_seen_at FROM runtimes WHERE id = %s", ph(1)),
		id,
	).Scan(&rt.ID, &rt.Name, &rt.Cloud, &rt.Region, &rt.AgentURL, &rt.Status, &rt.RegisteredAt, &rt.LastSeenAt)
	if err == sql.ErrNoRows {
		http.Error(w, "runtime not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rt)
}

// PATCH /internal/runtime/:id/heartbeat
// Agent sends a heartbeat to update last_seen_at.
func (s *APIServer) handleRuntimeHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	now := time.Now()
	result, err := s.db.Exec(
		fmt.Sprintf("UPDATE runtimes SET last_seen_at = %s, status = 'active' WHERE id = %s", ph(1), ph(2)),
		now, id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "runtime not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":       "ok",
		"last_seen_at": now.Format(time.RFC3339),
	})
}
