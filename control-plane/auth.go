package main

import (
	"context"
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
	ID         string     `json:"id"`
	Token      string     `json:"token,omitempty"`
	Comment    string     `json:"comment"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pltf_" + hex.EncodeToString(b), nil
}

func (s *APIServer) initAuthSchema() error {
	q := `CREATE TABLE IF NOT EXISTS api_tokens (
		id TEXT PRIMARY KEY,
		token TEXT UNIQUE NOT NULL,
		comment TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP,
		last_used_at TIMESTAMP
	);`
	_, err := s.db.Exec(q)
	return err
}

// AuthMiddleware validates Bearer tokens on all /v1/ routes.
// /internal/ routes are protected by a shared-secret check instead.
// /health and / are fully public. Token creation (POST /v1/auth/tokens) is public for bootstrap.
func (s *APIServer) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Internal routes: enforce shared-secret header instead of bearer token.
		if strings.HasPrefix(r.URL.Path, "/internal/") {
			s.internalAuthCheck(w, r, next)
			return
		}

		// Skip bearer auth for health, root, and token creation (bootstrap).
		if r.URL.Path == "/health" ||
			r.URL.Path == "/" ||
			(r.URL.Path == "/v1/auth/tokens" && r.Method == http.MethodPost) {
			next.ServeHTTP(w, r)
			return
		}

		token := extractBearerToken(r)
		if token == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="platform"`)
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}

		tokenID, err := s.validateToken(token)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyTokenID, tokenID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// internalAuthCheck enforces X-Internal-Secret on /internal/ routes.
// If internalSecret is empty the check is skipped (dev mode).
func (s *APIServer) internalAuthCheck(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if s.internalSecret != "" {
		if r.Header.Get("X-Internal-Secret") != s.internalSecret {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
	}
	next.ServeHTTP(w, r)
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// validateToken looks up the token, checks expiry, updates last_used_at, and
// returns the token's DB id. Returns an error if the token is invalid or expired.
func (s *APIServer) validateToken(token string) (string, error) {
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	var id string
	var expiresAt sql.NullTime
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id, expires_at FROM api_tokens WHERE token = %s", ph(1)),
		token,
	).Scan(&id, &expiresAt)
	if err != nil {
		return "", fmt.Errorf("invalid token")
	}

	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		return "", fmt.Errorf("token expired")
	}

	// Update last_used_at asynchronously so it doesn't slow the request path.
	go func() {
		_, _ = s.db.Exec(
			fmt.Sprintf("UPDATE api_tokens SET last_used_at = %s WHERE id = %s",
				nowExpr(pg), ph(1)),
			id,
		)
	}()

	return id, nil
}

// nowExpr returns the current-timestamp SQL expression for the configured DB.
func nowExpr(postgres bool) string {
	if postgres {
		return "NOW()"
	}
	return "CURRENT_TIMESTAMP"
}

func (s *APIServer) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Comment       string `json:"comment"`
		ExpiresInDays int    `json:"expires_in_days"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	raw, err := generateToken()
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	id := generateID()
	now := time.Now()
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	var expiresAt *time.Time
	var execErr error

	if req.ExpiresInDays > 0 {
		t := now.AddDate(0, 0, req.ExpiresInDays)
		expiresAt = &t

		var expiresExpr string
		if pg {
			expiresExpr = fmt.Sprintf("NOW() + INTERVAL '%d days'", req.ExpiresInDays)
		} else {
			expiresExpr = fmt.Sprintf("datetime('now', '+%d days')", req.ExpiresInDays)
		}

		_, execErr = s.db.Exec(
			fmt.Sprintf(
				"INSERT INTO api_tokens (id, token, comment, created_at, expires_at) VALUES (%s, %s, %s, %s, %s)",
				ph(1), ph(2), ph(3), ph(4), expiresExpr,
			),
			id, raw, req.Comment, now,
		)
	} else {
		_, execErr = s.db.Exec(
			fmt.Sprintf(
				"INSERT INTO api_tokens (id, token, comment, created_at) VALUES (%s, %s, %s, %s)",
				ph(1), ph(2), ph(3), ph(4),
			),
			id, raw, req.Comment, now,
		)
	}
	if execErr != nil {
		http.Error(w, execErr.Error(), http.StatusInternalServerError)
		return
	}

	auditLog(r, "token.create", "", req.Comment)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(Token{
		ID:        id,
		Token:     raw,
		Comment:   req.Comment,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	})
}

func (s *APIServer) handleListTokens(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, comment, created_at, expires_at, last_used_at FROM api_tokens ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tokens := []Token{}
	for rows.Next() {
		var t Token
		var expiresAt sql.NullTime
		var lastUsedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.Comment, &t.CreatedAt, &expiresAt, &lastUsedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if expiresAt.Valid {
			t.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			t.LastUsedAt = &lastUsedAt.Time
		}
		tokens = append(tokens, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

func (s *APIServer) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("value")
	ph := placeholder(s.config.IsPostgres(), 1)

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

	auditLog(r, "token.delete", token, "")

	w.WriteHeader(http.StatusNoContent)
}
