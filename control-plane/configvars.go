package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// handleGetConfigVars returns all config vars for an app as a key->value map.
func (s *APIServer) handleGetConfigVars(w http.ResponseWriter, r *http.Request) {
	appID, ok := s.resolveApp(w, r)
	if !ok {
		return
	}

	pg := s.config.IsPostgres()
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT key, value FROM config_vars WHERE app_id = %s", placeholder(pg, 1)),
		appID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	vars := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		vars[k] = v
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vars)
}

// handleSetConfigVars merges the provided key->value pairs into the app's config.
// Sending a null value removes that key.
func (s *APIServer) handleSetConfigVars(w http.ResponseWriter, r *http.Request) {
	appID, ok := s.resolveApp(w, r)
	if !ok {
		return
	}

	var incoming map[string]*string
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	pg := s.config.IsPostgres()

	for key, val := range incoming {
		if key == "" {
			continue
		}
		if err := validateConfigKey(key); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if val != nil {
			if err := validateConfigValue(*val); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
		}
		if val == nil {
			s.db.Exec(
				fmt.Sprintf("DELETE FROM config_vars WHERE app_id = %s AND key = %s", placeholder(pg, 1), placeholder(pg, 2)),
				appID, key,
			)
			continue
		}
		// upsert
		now := time.Now()
		if s.config.IsPostgres() {
			s.db.Exec(
				`INSERT INTO config_vars (id, app_id, key, value, created_at)
				 VALUES ($1, $2, $3, $4, $5)
				 ON CONFLICT (app_id, key) DO UPDATE SET value = EXCLUDED.value`,
				generateID(), appID, key, *val, now,
			)
		} else {
			s.db.Exec(
				`INSERT OR REPLACE INTO config_vars (id, app_id, key, value, created_at)
				 VALUES (?, ?, ?, ?, ?)`,
				generateID(), appID, key, *val, now,
			)
		}
	}

	auditLog(r, "config.set", mux.Vars(r)["name"], fmt.Sprintf("%d keys", len(incoming)))

	// Return the full updated config
	s.handleGetConfigVars(w, r)
}

func (s *APIServer) initConfigVarsSchema() error {
	var q string
	if s.config.IsPostgres() {
		q = `CREATE TABLE IF NOT EXISTS config_vars (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			UNIQUE(app_id, key)
		);
		CREATE INDEX IF NOT EXISTS idx_config_vars_app_id ON config_vars(app_id);`
	} else {
		q = `CREATE TABLE IF NOT EXISTS config_vars (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			UNIQUE(app_id, key),
			FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_config_vars_app_id ON config_vars(app_id);`
	}
	_, err := s.db.Exec(q)
	return err
}

// resolveApp looks up an app by name from the URL and returns its ID.
func (s *APIServer) resolveApp(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := mux.Vars(r)["name"]
	var appID string
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id FROM apps WHERE name = %s", placeholder(s.config.IsPostgres(), 1)), name,
	).Scan(&appID)
	if err != nil {
		http.Error(w, "app not found", http.StatusNotFound)
		return "", false
	}
	return appID, true
}
