package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	gossh "golang.org/x/crypto/ssh"
)

// SSHKey represents a registered SSH public key.
type SSHKey struct {
	ID          string    `json:"id"`
	TokenID     string    `json:"token_id"`
	PublicKey   string    `json:"public_key"`
	Fingerprint string    `json:"fingerprint"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
}

// initSSHKeysSchema creates the ssh_keys table if it does not exist.
func (s *APIServer) initSSHKeysSchema() error {
	q := `CREATE TABLE IF NOT EXISTS ssh_keys (
		id TEXT PRIMARY KEY,
		token_id TEXT NOT NULL,
		public_key TEXT NOT NULL,
		fingerprint TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := s.db.Exec(q)
	return err
}

// tokenIDFromRequest resolves the token_id for the Bearer token in the request.
// Returns "" if no valid token is found.
func (s *APIServer) tokenIDFromRequest(r *http.Request) string {
	token := extractBearerToken(r)
	if token == "" {
		return ""
	}
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }
	var id string
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT id FROM api_tokens WHERE token = %s", ph(1)),
		token,
	).Scan(&id)
	if err != nil {
		return ""
	}
	return id
}

// handleAddSSHKey registers a new SSH public key for the authenticated token owner.
// POST /v1/auth/ssh-keys
func (s *APIServer) handleAddSSHKey(w http.ResponseWriter, r *http.Request) {
	tokenID := s.tokenIDFromRequest(r)
	if tokenID == "" {
		http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		PublicKey string `json:"public_key"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.PublicKey = strings.TrimSpace(req.PublicKey)
	if req.PublicKey == "" {
		http.Error(w, `{"error":"public_key is required"}`, http.StatusBadRequest)
		return
	}

	// Validate the public key and compute fingerprint.
	parsed, _, _, _, err := gossh.ParseAuthorizedKey([]byte(req.PublicKey))
	if err != nil {
		http.Error(w, `{"error":"invalid SSH public key"}`, http.StatusBadRequest)
		return
	}
	fingerprint := gossh.FingerprintSHA256(parsed)

	id := generateID()
	now := time.Now()
	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	_, err = s.db.Exec(
		fmt.Sprintf(
			"INSERT INTO ssh_keys (id, token_id, public_key, fingerprint, name, created_at) VALUES (%s, %s, %s, %s, %s, %s)",
			ph(1), ph(2), ph(3), ph(4), ph(5), ph(6),
		),
		id, tokenID, req.PublicKey, fingerprint, req.Name, now,
	)
	if err != nil {
		// Duplicate fingerprint
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			http.Error(w, `{"error":"key already registered"}`, http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	key := SSHKey{
		ID:          id,
		TokenID:     tokenID,
		PublicKey:   req.PublicKey,
		Fingerprint: fingerprint,
		Name:        req.Name,
		CreatedAt:   now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(key)
}

// handleListSSHKeys returns all SSH keys owned by the authenticated token.
// GET /v1/auth/ssh-keys
func (s *APIServer) handleListSSHKeys(w http.ResponseWriter, r *http.Request) {
	tokenID := s.tokenIDFromRequest(r)
	if tokenID == "" {
		http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
		return
	}

	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	rows, err := s.db.Query(
		fmt.Sprintf(
			"SELECT id, token_id, public_key, fingerprint, name, created_at FROM ssh_keys WHERE token_id = %s ORDER BY created_at DESC",
			ph(1),
		),
		tokenID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	keys := []SSHKey{}
	for rows.Next() {
		var k SSHKey
		if err := rows.Scan(&k.ID, &k.TokenID, &k.PublicKey, &k.Fingerprint, &k.Name, &k.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		keys = append(keys, k)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

// handleDeleteSSHKey removes an SSH key by ID, only if it belongs to the authenticated token.
// DELETE /v1/auth/ssh-keys/:id
func (s *APIServer) handleDeleteSSHKey(w http.ResponseWriter, r *http.Request) {
	tokenID := s.tokenIDFromRequest(r)
	if tokenID == "" {
		http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	keyID := vars["id"]
	if keyID == "" {
		http.Error(w, `{"error":"key id required"}`, http.StatusBadRequest)
		return
	}

	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	result, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM ssh_keys WHERE id = %s AND token_id = %s", ph(1), ph(2)),
		keyID, tokenID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleVerifySSHKey is the internal endpoint used by the git server to validate
// an SSH public key fingerprint.
// GET /internal/ssh-keys/verify?fingerprint=SHA256:xxxx
// Protected by X-Internal-Secret header.
func (s *APIServer) handleVerifySSHKey(w http.ResponseWriter, r *http.Request) {
	// Validate internal secret if configured.
	secret := s.config.InternalSecret
	if secret != "" {
		if r.Header.Get("X-Internal-Secret") != secret {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
	}

	fingerprint := r.URL.Query().Get("fingerprint")
	if fingerprint == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false})
		return
	}

	pg := s.config.IsPostgres()
	ph := func(n int) string { return placeholder(pg, n) }

	var tokenID string
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT token_id FROM ssh_keys WHERE fingerprint = %s", ph(1)),
		fingerprint,
	).Scan(&tokenID)

	w.Header().Set("Content-Type", "application/json")
	if err == sql.ErrNoRows {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false})
		return
	}
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":    true,
		"token_id": tokenID,
	})
}
