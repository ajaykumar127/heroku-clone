-- =============================================================================
-- Migration 004: App / Release → Runtime Soft Foreign Keys
-- =============================================================================
-- Ensures that apps.runtime_id and releases.runtime_id exist as columns.
-- Both were included in migration 001 for fresh installs; this migration is
-- needed for databases that were created before migration 001 was updated to
-- include those columns (e.g. older POC deployments).
--
-- Design note — SQLite vs. PostgreSQL compatibility
-- -------------------------------------------------
-- SQLite does not support ALTER TABLE ... ADD COLUMN IF NOT EXISTS, and it
-- also does not support declarative FOREIGN KEY constraints added after table
-- creation.  We therefore use "soft" foreign keys — plain TEXT columns that
-- are enforced at the application layer — which work identically on both
-- engines.
--
-- Run the section that matches your database engine.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- PostgreSQL (supports IF NOT EXISTS on ALTER TABLE ADD COLUMN — PG 9.6+)
-- ---------------------------------------------------------------------------

-- Preferred runtime / runtime that last served this app.
ALTER TABLE apps      ADD COLUMN IF NOT EXISTS runtime_id TEXT;

-- Runtime that handled this particular release / build.
ALTER TABLE releases  ADD COLUMN IF NOT EXISTS runtime_id TEXT;


-- ---------------------------------------------------------------------------
-- SQLite  (does NOT support IF NOT EXISTS on ALTER TABLE ADD COLUMN)
--
-- Run these statements only when migrating an existing SQLite database that
-- was created without the runtime_id columns.  They will fail with
-- "duplicate column name" if the columns already exist — that error is safe
-- to ignore, or wrap the call in application-level idempotency logic.
-- ---------------------------------------------------------------------------

-- ALTER TABLE apps     ADD COLUMN runtime_id TEXT;
-- ALTER TABLE releases ADD COLUMN runtime_id TEXT;
