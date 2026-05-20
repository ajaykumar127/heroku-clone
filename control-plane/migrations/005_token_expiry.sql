-- Add expiry to tokens and SSH keys table
ALTER TABLE api_tokens ADD COLUMN expires_at TIMESTAMP;
ALTER TABLE api_tokens ADD COLUMN last_used_at TIMESTAMP;

CREATE TABLE IF NOT EXISTS ssh_keys (
    id TEXT PRIMARY KEY,
    token_id TEXT NOT NULL REFERENCES api_tokens(id) ON DELETE CASCADE,
    public_key TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
