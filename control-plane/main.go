package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

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

	// Build logs (snapshot)
	s.router.HandleFunc("/v1/apps/{name}/releases/{release_id}/logs", s.handleReleaseLogs).Methods("GET")
	// Build logs (streaming SSE)
	s.router.HandleFunc("/v1/apps/{name}/releases/{release_id}/logs/stream", s.handleStreamLogs).Methods("GET")

	// Config vars
	s.router.HandleFunc("/v1/apps/{name}/config-vars", s.handleGetConfigVars).Methods("GET")
	s.router.HandleFunc("/v1/apps/{name}/config-vars", s.handleSetConfigVars).Methods("PATCH")

	// Runtimes (authenticated)
	s.router.HandleFunc("/v1/runtimes", s.handleListRuntimes).Methods("GET")
	s.router.HandleFunc("/v1/runtimes/{id}", s.handleGetRuntime).Methods("GET")

	// Internal: runtime agent endpoints (no auth required)
	s.router.HandleFunc("/internal/builds", s.handleBuildWebhook).Methods("POST")
	s.router.HandleFunc("/internal/runtime/register", s.handleRegisterRuntime).Methods("POST")
	s.router.HandleFunc("/internal/runtime/{id}/heartbeat", s.handleRuntimeHeartbeat).Methods("PATCH")
	s.router.HandleFunc("/internal/runtime/{runtime_id}/jobs", s.handlePollJobs).Methods("GET")
	s.router.HandleFunc("/internal/runtime/{runtime_id}/jobs/{job_id}/status", s.handleUpdateJobStatus).Methods("POST")
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	releaseID, jobID, err := s.buildManager.TriggerBuild(req.AppName, req.Commit)
	if err != nil {
		log.Printf("[webhook] failed to trigger build app=%s: %v", req.AppName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[webhook] build queued app=%s commit=%s release=%s", req.AppName, req.Commit, releaseID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	resp := map[string]string{
		"status":     "building",
		"release_id": releaseID,
	}
	if jobID != "" {
		resp["job_id"] = jobID
	}
	json.NewEncoder(w).Encode(resp)
}

func initDB(config *Config) (*sql.DB, error) {
	var db *sql.DB
	var err error

	if config.IsPostgres() {
		db, err = sql.Open("postgres", config.DatabaseURL)
		if err != nil {
			return nil, err
		}

		schema := `
		CREATE TABLE IF NOT EXISTS apps (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			git_url TEXT NOT NULL,
			web_url TEXT NOT NULL,
			runtime_id TEXT,
			created_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS releases (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			"commit" TEXT NOT NULL,
			status TEXT NOT NULL,
			build_output TEXT,
			runtime_id TEXT,
			created_at TIMESTAMP NOT NULL,
			UNIQUE(app_id, version)
		);

		CREATE INDEX IF NOT EXISTS idx_releases_app_id ON releases(app_id);

		CREATE TABLE IF NOT EXISTS runtimes (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			cloud TEXT NOT NULL,
			region TEXT NOT NULL,
			agent_url TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			registered_at TIMESTAMP NOT NULL,
			last_seen_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			runtime_id TEXT NOT NULL,
			app_id TEXT NOT NULL,
			release_id TEXT NOT NULL,
			app_name TEXT NOT NULL,
			"commit" TEXT NOT NULL,
			repo_url TEXT NOT NULL,
			image_name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'build_and_deploy',
			status TEXT NOT NULL DEFAULT 'pending',
			log_output TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_jobs_runtime_status ON jobs(runtime_id, status);
		`

		if _, err := db.Exec(schema); err != nil {
			return nil, err
		}
	} else {
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
			runtime_id TEXT,
			created_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS releases (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			"commit" TEXT NOT NULL,
			status TEXT NOT NULL,
			build_output TEXT,
			runtime_id TEXT,
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE,
			UNIQUE(app_id, version)
		);

		CREATE INDEX IF NOT EXISTS idx_releases_app_id ON releases(app_id);

		CREATE TABLE IF NOT EXISTS runtimes (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			cloud TEXT NOT NULL,
			region TEXT NOT NULL,
			agent_url TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			registered_at TIMESTAMP NOT NULL,
			last_seen_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			runtime_id TEXT NOT NULL,
			app_id TEXT NOT NULL,
			release_id TEXT NOT NULL,
			app_name TEXT NOT NULL,
			"commit" TEXT NOT NULL,
			repo_url TEXT NOT NULL,
			image_name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'build_and_deploy',
			status TEXT NOT NULL DEFAULT 'pending',
			log_output TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_jobs_runtime_status ON jobs(runtime_id, status);
		`

		if _, err := db.Exec(schema); err != nil {
			return nil, err
		}
	}

	return db, nil
}

func main() {
	config := LoadConfig()

	log.Printf("Starting Control Plane API Server")
	log.Printf("Environment: %s", config.Environment)
	log.Printf("Database: %s", maskDatabaseURL(config.DatabaseURL))
	log.Printf("Port: %s", config.Port)
	log.Printf("Git Server: %s:%s", config.GitServerHost, config.GitServerPort)

	db, err := initDB(config)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	log.Println("Database connected")

	server := NewAPIServer(db, config)

	if err := server.initAuthSchema(); err != nil {
		log.Fatal("Failed to initialize auth schema:", err)
	}
	log.Println("Auth schema ready")

	if err := server.initConfigVarsSchema(); err != nil {
		log.Fatal("Failed to initialize config vars schema:", err)
	}
	log.Println("Config vars schema ready")

	addr := ":" + config.Port
	log.Printf("Control Plane API listening on %s", addr)
	if err := http.ListenAndServe(addr, server.router); err != nil {
		log.Fatal("Server failed:", err)
	}
}

// maskDatabaseURL hides sensitive parts of the database URL
func maskDatabaseURL(url string) string {
	if strings.HasPrefix(url, "postgres://") {
		parts := strings.Split(url, "@")
		if len(parts) == 2 {
			return "postgres://***@" + parts[1]
		}
	}
	if strings.HasSuffix(url, ".db") {
		return url
	}
	return fmt.Sprintf("%s...", url[:min(len(url), 8)])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
