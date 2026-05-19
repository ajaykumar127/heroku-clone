-- =============================================================================
-- Migration 001: Initial Schema
-- =============================================================================
-- Baseline tables for the control plane: apps, releases, api_tokens,
-- and config_vars. These mirror the schema initialised by initDB() in
-- poc/api-server/main.go, cleaned up and made fully portable.
--
-- Compatible with: PostgreSQL 12+ and SQLite 3.x
-- Note: "commit" is quoted throughout because it is a reserved word in SQLite.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- apps
-- Stores every application registered on the platform.
-- runtime_id is nullable here; it is populated once a Runtime Plane is
-- assigned (see migration 002/004).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS apps (
    id         TEXT PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    git_url    TEXT NOT NULL,
    web_url    TEXT NOT NULL,
    runtime_id TEXT,           -- which runtime plane hosts this app (nullable at first)
    created_at TIMESTAMP NOT NULL
);

-- ---------------------------------------------------------------------------
-- releases
-- An immutable record of every deployment attempt for an app.
-- status values: building | succeeded | failed
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS releases (
    id           TEXT PRIMARY KEY,
    app_id       TEXT NOT NULL,
    version      INTEGER NOT NULL,
    "commit"     TEXT NOT NULL,
    status       TEXT NOT NULL,                  -- building, succeeded, failed
    build_output TEXT NOT NULL DEFAULT '',
    runtime_id   TEXT,                           -- which runtime handled this release
    created_at   TIMESTAMP NOT NULL,
    UNIQUE(app_id, version)
);

CREATE INDEX IF NOT EXISTS idx_releases_app_id ON releases(app_id);

-- ---------------------------------------------------------------------------
-- api_tokens
-- Simple bearer tokens used to authenticate CLI and API calls.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS api_tokens (
    id         TEXT PRIMARY KEY,
    token      TEXT UNIQUE NOT NULL,
    comment    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL
);

-- ---------------------------------------------------------------------------
-- config_vars
-- Per-app environment variable store (analogous to `heroku config:set`).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS config_vars (
    id         TEXT PRIMARY KEY,
    app_id     TEXT NOT NULL,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    UNIQUE(app_id, key)
);

CREATE INDEX IF NOT EXISTS idx_config_vars_app_id ON config_vars(app_id);
