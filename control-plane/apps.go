package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type App struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	GitURL    string    `json:"git_url"`
	WebURL    string    `json:"web_url"`
	RuntimeID string    `json:"runtime_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *APIServer) handleListApps(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, name, git_url, web_url, COALESCE(runtime_id, ''), created_at FROM apps ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	apps := []App{}
	for rows.Next() {
		var app App
		if err := rows.Scan(&app.ID, &app.Name, &app.GitURL, &app.WebURL, &app.RuntimeID, &app.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		apps = append(apps, app)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apps)
}

func (s *APIServer) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if err := validateAppName(req.Name); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	app := App{
		ID:        uuid.New().String(),
		Name:      req.Name,
		GitURL:    fmt.Sprintf("ssh://git@%s:%s/%s.git", s.config.GitServerHost, s.config.GitServerPort, req.Name),
		WebURL:    "http://" + req.Name + ".localhost",
		CreatedAt: time.Now(),
	}

	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO apps (id, name, git_url, web_url, created_at) VALUES (%s, %s, %s, %s, %s)",
			ph(1), ph(2), ph(3), ph(4), ph(5)),
		app.ID, app.Name, app.GitURL, app.WebURL, app.CreatedAt,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	auditLog(r, "app.create", app.Name, "")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(app)
}

func (s *APIServer) handleGetApp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	var app App
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id, name, git_url, web_url, COALESCE(runtime_id, ''), created_at FROM apps WHERE name = %s", ph(1)),
		name,
	).Scan(&app.ID, &app.Name, &app.GitURL, &app.WebURL, &app.RuntimeID, &app.CreatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(app)
}

func (s *APIServer) handleDeleteApp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	result, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM apps WHERE name = %s", ph(1)), name,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	auditLog(r, "app.destroy", name, "")

	w.WriteHeader(http.StatusNoContent)
}

// handleCheckAppAccess is the internal endpoint used by the git server to verify
// that a given token_id is allowed to access the named app.
// For this single-tenant POC any valid token grants access to any app that exists.
// GET /internal/apps/{name}/check-access?token_id=xxx
// Returns 200 if allowed, 403 if the token is unknown, 404 if the app doesn't exist.
func (s *APIServer) handleCheckAppAccess(w http.ResponseWriter, r *http.Request) {
	// Validate internal secret if configured.
	secret := s.config.InternalSecret
	if secret != "" {
		if r.Header.Get("X-Internal-Secret") != secret {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
	}

	vars := mux.Vars(r)
	appName := vars["name"]
	tokenID := r.URL.Query().Get("token_id")

	if appName == "" || tokenID == "" {
		http.Error(w, `{"error":"app name and token_id are required"}`, http.StatusBadRequest)
		return
	}

	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	// Verify the app exists.
	var appID string
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id FROM apps WHERE name = %s", ph(1)),
		appName,
	).Scan(&appID)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"app not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Verify the token exists (single-tenant: any valid token may access any app).
	var id string
	err = s.db.QueryRow(
		fmt.Sprintf("SELECT id FROM api_tokens WHERE id = %s", ph(1)),
		tokenID,
	).Scan(&id)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"access denied"}`, http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
