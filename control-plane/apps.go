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

	w.WriteHeader(http.StatusNoContent)
}
