package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Token struct {
	ID        string    `json:"id"`
	Token     string    `json:"token,omitempty"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pltf_" + hex.EncodeToString(b), nil
}

func (s *APIServer) initAuthSchema() error {
	var q string
	if s.config.IsPostgres() {
		q = `CREATE TABLE IF NOT EXISTS api_tokens (
			id TEXT PRIMARY KEY,
			token TEXT UNIQUE NOT NULL,
			comment TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL
		);`
	} else {
		q = `CREATE TABLE IF NOT EXISTS api_tokens (
			id TEXT PRIMARY KEY,
			token TEXT UNIQUE NOT NULL,
			comment TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL
		);`
	}
	_, err := s.db.Exec(q)
	return err
}

// AuthMiddleware validates Bearer tokens on all /v1/ routes.
// The /internal/ and /health routes are exempt.
func (s *APIServer) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for internal webhook and health
		if strings.HasPrefix(r.URL.Path, "/internal/") ||
			r.URL.Path == "/health" ||
			r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		token := extractBearerToken(r)
		if token == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="platform"`)
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}

		if !s.validateToken(token) {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// also accept ?token= query param for easy curl testing
	return r.URL.Query().Get("token")
}

func (s *APIServer) validateToken(token string) bool {
	var id string
	ph := "?"
	if s.config.IsPostgres() {
		ph = "$1"
	}
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id FROM api_tokens WHERE token = %s", ph), token,
	).Scan(&id)
	return err == nil
}

func (s *APIServer) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Comment string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	raw, err := generateToken()
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	id := generateID()
	now := time.Now()
	bm := NewBuildManager(s.db, s.config)
	ph := func(n int) string { return bm.ph(n) }

	_, err = s.db.Exec(
		fmt.Sprintf(
			"INSERT INTO api_tokens (id, token, comment, created_at) VALUES (%s, %s, %s, %s)",
			ph(1), ph(2), ph(3), ph(4),
		),
		id, raw, req.Comment, now,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(Token{
		ID:        id,
		Token:     raw,
		Comment:   req.Comment,
		CreatedAt: now,
	})
}

func (s *APIServer) handleListTokens(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, comment, created_at FROM api_tokens ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tokens := []Token{}
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.Comment, &t.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tokens = append(tokens, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

func (s *APIServer) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	// Support delete by token value passed as query param for CLI convenience
	token := r.URL.Query().Get("value")
	bm := NewBuildManager(s.db, s.config)
	ph := bm.ph(1)

	var result sql.Result
	var execErr error
	if token != "" {
		result, execErr = s.db.Exec(
			fmt.Sprintf("DELETE FROM api_tokens WHERE token = %s", ph), token,
		)
	} else {
		http.Error(w, "token value required", http.StatusBadRequest)
		return
	}
	if execErr != nil {
		http.Error(w, execErr.Error(), http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "token not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
