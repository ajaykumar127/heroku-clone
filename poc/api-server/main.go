package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	GitURL    string    `json:"git_url"`
	WebURL    string    `json:"web_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Release struct {
	ID          string    `json:"id"`
	AppID       string    `json:"app_id"`
	Version     int       `json:"version"`
	Commit      string    `json:"commit"`
	Status      string    `json:"status"`
	BuildOutput string    `json:"build_output,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type APIServer struct {
	db           *sql.DB
	router       *mux.Router
	config       *Config
	buildManager *BuildManager
}

func NewAPIServer(db *sql.DB, config *Config) *APIServer {
	s := &APIServer{
		db:           db,
		router:       mux.NewRouter(),
		config:       config,
		buildManager: NewBuildManager(db, config),
	}

	// Add middleware
	s.router.Use(mux.MiddlewareFunc(LoggingMiddleware))
	s.router.Use(mux.MiddlewareFunc(CORSMiddleware))
	s.router.Use(mux.MiddlewareFunc(RecoveryMiddleware))
	s.router.Use(s.AuthMiddleware)

	s.routes()
	return s
}

func (s *APIServer) routes() {
	// Dashboard
	s.router.HandleFunc("/", s.handleDashboard).Methods("GET")
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Apps
	s.router.HandleFunc("/v1/apps", s.handleListApps).Methods("GET")
	s.router.HandleFunc("/v1/apps", s.handleCreateApp).Methods("POST")
	s.router.HandleFunc("/v1/apps/{name}", s.handleGetApp).Methods("GET")
	s.router.HandleFunc("/v1/apps/{name}", s.handleDeleteApp).Methods("DELETE")

	// Releases
	s.router.HandleFunc("/v1/apps/{name}/releases", s.handleListReleases).Methods("GET")
	s.router.HandleFunc("/v1/apps/{name}/releases", s.handleCreateRelease).Methods("POST")

	// Auth tokens
	s.router.HandleFunc("/v1/auth/tokens", s.handleCreateToken).Methods("POST")
	s.router.HandleFunc("/v1/auth/tokens", s.handleListTokens).Methods("GET")
	s.router.HandleFunc("/v1/auth/tokens", s.handleDeleteToken).Methods("DELETE")

	// Builds (webhook from git server)
	s.router.HandleFunc("/internal/builds", s.handleBuildWebhook).Methods("POST")

	// Build logs (snapshot)
	s.router.HandleFunc("/v1/apps/{name}/releases/{release_id}/logs", s.handleReleaseLogs).Methods("GET")
	// Build logs (streaming SSE)
	s.router.HandleFunc("/v1/apps/{name}/releases/{release_id}/logs/stream", s.handleStreamLogs).Methods("GET")

	// Config vars
	s.router.HandleFunc("/v1/apps/{name}/config-vars", s.handleGetConfigVars).Methods("GET")
	s.router.HandleFunc("/v1/apps/{name}/config-vars", s.handleSetConfigVars).Methods("PATCH")
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *APIServer) handleListApps(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, name, git_url, web_url, created_at FROM apps ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	apps := []App{}
	for rows.Next() {
		var app App
		if err := rows.Scan(&app.ID, &app.Name, &app.GitURL, &app.WebURL, &app.CreatedAt); err != nil {
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

	app := App{
		ID:        uuid.New().String(),
		Name:      req.Name,
		GitURL:    "ssh://git@localhost:2222/" + req.Name + ".git",
		WebURL:    "http://" + req.Name + ".localhost",
		CreatedAt: time.Now(),
	}

	bm := s.buildManager
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO apps (id, name, git_url, web_url, created_at) VALUES (%s, %s, %s, %s, %s)",
			bm.ph(1), bm.ph(2), bm.ph(3), bm.ph(4), bm.ph(5)),
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
	bm := s.buildManager

	var app App
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id, name, git_url, web_url, created_at FROM apps WHERE name = %s", bm.ph(1)),
		name,
	).Scan(&app.ID, &app.Name, &app.GitURL, &app.WebURL, &app.CreatedAt)

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
	bm := s.buildManager

	result, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM apps WHERE name = %s", bm.ph(1)), name,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) handleListReleases(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	bm := s.buildManager

	var appID string
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id FROM apps WHERE name = %s", bm.ph(1)), name,
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
		fmt.Sprintf("SELECT id, app_id, version, commit, status, created_at FROM releases WHERE app_id = %s ORDER BY version DESC", bm.ph(1)),
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
		if err := rows.Scan(&rel.ID, &rel.AppID, &rel.Version, &rel.Commit, &rel.Status, &rel.CreatedAt); err != nil {
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

	releaseID, err := s.buildManager.TriggerBuild(name, req.Commit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"release_id": releaseID,
		"status":     "building",
	})
}

func (s *APIServer) handleBuildWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppName string `json:"app_name"`
		Commit  string `json:"commit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.AppName == "" || req.Commit == "" {
		http.Error(w, "app_name and commit are required", http.StatusBadRequest)
		return
	}

	releaseID, err := s.buildManager.TriggerBuild(req.AppName, req.Commit)
	if err != nil {
		log.Printf("[webhook] failed to trigger build app=%s: %v", req.AppName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[webhook] build queued app=%s commit=%s release=%s", req.AppName, req.Commit, releaseID)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "building",
		"release_id": releaseID,
	})
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

func initDB(config *Config) (*sql.DB, error) {
	var db *sql.DB
	var err error

	if config.IsPostgres() {
		// PostgreSQL
		db, err = sql.Open("postgres", config.DatabaseURL)
		if err != nil {
			return nil, err
		}

		// PostgreSQL schema (simplified for POC)
		schema := `
		CREATE TABLE IF NOT EXISTS apps (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			git_url TEXT NOT NULL,
			web_url TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS releases (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			commit TEXT NOT NULL,
			status TEXT NOT NULL,
			build_output TEXT,
			created_at TIMESTAMP NOT NULL,
			UNIQUE(app_id, version)
		);

		CREATE INDEX IF NOT EXISTS idx_releases_app_id ON releases(app_id);
		`

		if _, err := db.Exec(schema); err != nil {
			return nil, err
		}
	} else {
		// SQLite
		db, err = sql.Open("sqlite3", config.DatabaseURL)
		if err != nil {
			return nil, err
		}

		schema := `
		CREATE TABLE IF NOT EXISTS apps (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			git_url TEXT NOT NULL,
			web_url TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS releases (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			commit TEXT NOT NULL,
			status TEXT NOT NULL,
			build_output TEXT,
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE,
			UNIQUE(app_id, version)
		);

		CREATE INDEX IF NOT EXISTS idx_releases_app_id ON releases(app_id);
		`

		if _, err := db.Exec(schema); err != nil {
			return nil, err
		}
	}

	return db, nil
}

func main() {
	config := LoadConfig()

	log.Printf("Starting API Server")
	log.Printf("Environment: %s", config.Environment)
	log.Printf("Database: %s", maskDatabaseURL(config.DatabaseURL))
	log.Printf("Port: %s", config.Port)

	db, err := initDB(config)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	log.Println("✓ Database connected")

	server := NewAPIServer(db, config)

	if err := server.initAuthSchema(); err != nil {
		log.Fatal("Failed to initialize auth schema:", err)
	}
	log.Println("✓ Auth schema ready")

	if err := server.initConfigVarsSchema(); err != nil {
		log.Fatal("Failed to initialize config vars schema:", err)
	}
	log.Println("✓ Config vars schema ready")

	addr := ":" + config.Port
	log.Printf("API Server listening on %s", addr)
	if err := http.ListenAndServe(addr, server.router); err != nil {
		log.Fatal("Server failed:", err)
	}
}

// maskDatabaseURL hides sensitive parts of the database URL
func maskDatabaseURL(url string) string {
	if strings.HasPrefix(url, "postgres://") {
		// Show only the protocol and host
		parts := strings.Split(url, "@")
		if len(parts) == 2 {
			return "postgres://***@" + parts[1]
		}
	}
	if strings.HasSuffix(url, ".db") {
		return url
	}
	return "***"
}
