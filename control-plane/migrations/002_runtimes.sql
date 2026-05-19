-- =============================================================================
-- Migration 002: Runtime Plane Registry
-- =============================================================================
-- Introduces the runtimes table so the control plane can track every
-- Runtime Plane agent that has registered itself.  Each runtime corresponds
-- to a cloud-region combination (e.g. "aws-us-east-1", "gcp-us-central1").
--
-- Compatible with: PostgreSQL 12+ and SQLite 3.x
-- =============================================================================

-- ---------------------------------------------------------------------------
-- runtimes
-- Registry of Runtime Plane agents known to this control plane.
--
-- cloud values:  aws | gcp | azure | local
-- status values: active | inactive | draining
--
-- agent_url is the HTTP endpoint the control plane calls to dispatch jobs
-- (e.g. http://runtime-agent.internal:8080).
--
-- last_seen_at is updated via heartbeat; use it to detect stale runtimes.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS runtimes (
    id            TEXT PRIMARY KEY,
    name          TEXT UNIQUE NOT NULL,            -- e.g. "aws-us-east-1"
    cloud         TEXT NOT NULL,                   -- aws, gcp, azure, local
    region        TEXT NOT NULL,
    agent_url     TEXT NOT NULL DEFAULT '',        -- HTTP endpoint of the runtime agent
    status        TEXT NOT NULL DEFAULT 'active',  -- active, inactive, draining
    registered_at TIMESTAMP NOT NULL,
    last_seen_at  TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_runtimes_status ON runtimes(status);
