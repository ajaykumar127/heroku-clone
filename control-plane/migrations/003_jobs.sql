-- =============================================================================
-- Migration 003: Job Queue
-- =============================================================================
-- Adds the jobs table — a persistent queue of build-and-deploy work items
-- that the control plane dispatches to Runtime Plane agents.
--
-- Each job is tied to a specific runtime, app, and release so that the
-- control plane can track progress and reconcile state after failures.
--
-- Compatible with: PostgreSQL 12+ and SQLite 3.x
-- Note: "commit" is quoted because it is a reserved word in SQLite.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- jobs
-- A job represents one build-and-deploy cycle dispatched to a runtime agent.
--
-- type values:   build_and_deploy (default; more types may be added later)
-- status values: pending | building | deploying | succeeded | failed
--
-- repo_url   — git URL the runtime agent will clone, e.g.
--              ssh://git@<host>:2222/<app>.git
-- image_name — OCI image the runtime agent will build and push, e.g.
--              <registry>/<app>:<commit>
-- log_output — captured stdout/stderr from the agent; appended as it streams.
-- updated_at — set to NOW() on every status transition.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS jobs (
    id          TEXT PRIMARY KEY,
    runtime_id  TEXT NOT NULL,
    app_id      TEXT NOT NULL,
    release_id  TEXT NOT NULL,
    app_name    TEXT NOT NULL,
    "commit"    TEXT NOT NULL,
    repo_url    TEXT NOT NULL,                          -- ssh://git@<host>:2222/<app>.git
    image_name  TEXT NOT NULL,                          -- <registry>/<app>:<commit>
    type        TEXT NOT NULL DEFAULT 'build_and_deploy',
    status      TEXT NOT NULL DEFAULT 'pending',        -- pending, building, deploying, succeeded, failed
    log_output  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL,
    updated_at  TIMESTAMP NOT NULL
);

-- Fetching pending/in-progress jobs for a given runtime is the hot path.
CREATE INDEX IF NOT EXISTS idx_jobs_runtime_status ON jobs(runtime_id, status);

-- Correlate a job back to its release (e.g. when streaming logs).
CREATE INDEX IF NOT EXISTS idx_jobs_release_id ON jobs(release_id);
