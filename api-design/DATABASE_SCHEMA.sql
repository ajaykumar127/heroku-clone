-- Platform Database Schema
-- PostgreSQL 15+
-- This schema stores all platform metadata

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- Users & Authentication
-- ============================================================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    name VARCHAR(255),
    verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_users_email ON users(email);

CREATE TABLE api_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    description VARCHAR(255),
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_token_hash ON api_tokens(token_hash);

CREATE TABLE ssh_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255),
    fingerprint VARCHAR(255) UNIQUE NOT NULL,
    public_key TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_ssh_keys_user_id ON ssh_keys(user_id);
CREATE INDEX idx_ssh_keys_fingerprint ON ssh_keys(fingerprint);

-- ============================================================================
-- Teams & Organizations
-- ============================================================================

CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255),
    owner_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_teams_owner_id ON teams(owner_id);

CREATE TYPE team_role AS ENUM ('admin', 'member', 'viewer');

CREATE TABLE team_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role team_role NOT NULL DEFAULT 'member',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(team_id, user_id)
);

CREATE INDEX idx_team_memberships_team_id ON team_memberships(team_id);
CREATE INDEX idx_team_memberships_user_id ON team_memberships(user_id);

-- ============================================================================
-- Applications
-- ============================================================================

CREATE TYPE app_status AS ENUM ('active', 'maintenance', 'archived');

CREATE TABLE apps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id),
    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    region VARCHAR(50) NOT NULL DEFAULT 'us-west-2',
    stack VARCHAR(50) NOT NULL DEFAULT 'heroku-22',
    status app_status NOT NULL DEFAULT 'active',
    git_url VARCHAR(500),
    web_url VARCHAR(500),
    repo_size BIGINT DEFAULT 0,
    slug_size BIGINT DEFAULT 0,
    buildpack_provided_description VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_apps_owner_id ON apps(owner_id);
CREATE INDEX idx_apps_team_id ON apps(team_id);
CREATE INDEX idx_apps_name ON apps(name);
CREATE INDEX idx_apps_status ON apps(status);

-- ============================================================================
-- Slugs (Build Artifacts)
-- ============================================================================

CREATE TABLE slugs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    checksum VARCHAR(255) NOT NULL,
    commit VARCHAR(255),
    commit_description TEXT,
    size BIGINT,
    storage_url TEXT NOT NULL,
    buildpack_provided_description VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_slugs_app_id ON slugs(app_id);
CREATE INDEX idx_slugs_checksum ON slugs(checksum);

-- ============================================================================
-- Builds
-- ============================================================================

CREATE TYPE build_status AS ENUM ('pending', 'running', 'succeeded', 'failed');

CREATE TABLE builds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    slug_id UUID REFERENCES slugs(id),
    user_id UUID REFERENCES users(id),
    source_blob_url TEXT,
    source_blob_version VARCHAR(255),
    status build_status NOT NULL DEFAULT 'pending',
    stack VARCHAR(50) NOT NULL,
    buildpacks JSONB,
    output_stream_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_builds_app_id ON builds(app_id);
CREATE INDEX idx_builds_status ON builds(status);
CREATE INDEX idx_builds_slug_id ON builds(slug_id);

-- ============================================================================
-- Config Vars (Environment Variables)
-- ============================================================================

CREATE TABLE config_vars (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(app_id, key)
);

CREATE INDEX idx_config_vars_app_id ON config_vars(app_id);

-- ============================================================================
-- Releases
-- ============================================================================

CREATE TYPE release_status AS ENUM ('pending', 'succeeded', 'failed');

CREATE TABLE releases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    slug_id UUID REFERENCES slugs(id),
    user_id UUID REFERENCES users(id),
    description TEXT,
    status release_status NOT NULL DEFAULT 'pending',
    current BOOLEAN DEFAULT FALSE,
    config_vars JSONB NOT NULL DEFAULT '{}',
    output_stream_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(app_id, version)
);

CREATE INDEX idx_releases_app_id ON releases(app_id);
CREATE INDEX idx_releases_version ON releases(app_id, version DESC);
CREATE INDEX idx_releases_current ON releases(app_id, current);
CREATE INDEX idx_releases_slug_id ON releases(slug_id);

-- ============================================================================
-- Formation (Process Types & Scaling)
-- ============================================================================

CREATE TABLE formations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 0,
    size VARCHAR(50) NOT NULL DEFAULT 'standard-1x',
    command TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(app_id, type)
);

CREATE INDEX idx_formations_app_id ON formations(app_id);

-- ============================================================================
-- Dynos (Running Containers)
-- ============================================================================

CREATE TYPE dyno_state AS ENUM ('starting', 'up', 'idle', 'crashed', 'down');

CREATE TABLE dynos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    release_id UUID NOT NULL REFERENCES releases(id),
    name VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,
    size VARCHAR(50) NOT NULL,
    state dyno_state NOT NULL DEFAULT 'starting',
    command TEXT,
    k8s_namespace VARCHAR(255),
    k8s_pod_name VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_dynos_app_id ON dynos(app_id);
CREATE INDEX idx_dynos_state ON dynos(state);
CREATE INDEX idx_dynos_k8s_pod ON dynos(k8s_namespace, k8s_pod_name);

-- ============================================================================
-- Domains
-- ============================================================================

CREATE TYPE domain_kind AS ENUM ('default', 'custom');
CREATE TYPE domain_status AS ENUM ('pending', 'active', 'error');

CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    hostname VARCHAR(255) UNIQUE NOT NULL,
    cname VARCHAR(255),
    kind domain_kind NOT NULL DEFAULT 'custom',
    status domain_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_domains_app_id ON domains(app_id);
CREATE INDEX idx_domains_hostname ON domains(hostname);

-- ============================================================================
-- SSL Certificates
-- ============================================================================

CREATE TYPE ssl_status AS ENUM ('pending', 'ok', 'error', 'expiring_soon', 'expired');

CREATE TABLE ssl_certificates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    status ssl_status NOT NULL DEFAULT 'pending',
    issuer VARCHAR(255),
    expires_at TIMESTAMP WITH TIME ZONE,
    cert_pem TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_ssl_certificates_domain_id ON ssl_certificates(domain_id);
CREATE INDEX idx_ssl_certificates_expires_at ON ssl_certificates(expires_at);

-- ============================================================================
-- Add-ons
-- ============================================================================

CREATE TABLE addon_services (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255),
    description TEXT,
    cli_plugin_name VARCHAR(255),
    human_name VARCHAR(255),
    state VARCHAR(50) DEFAULT 'ga',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE addon_plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    addon_service_id UUID NOT NULL REFERENCES addon_services(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    description TEXT,
    price_cents INTEGER NOT NULL DEFAULT 0,
    price_unit VARCHAR(50) DEFAULT 'month',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(addon_service_id, name)
);

CREATE INDEX idx_addon_plans_service_id ON addon_plans(addon_service_id);

CREATE TYPE addon_state AS ENUM ('provisioning', 'provisioned', 'deprovisioned', 'error');

CREATE TABLE addons (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    addon_service_id UUID NOT NULL REFERENCES addon_services(id),
    plan_id UUID NOT NULL REFERENCES addon_plans(id),
    name VARCHAR(255) NOT NULL,
    provider_id VARCHAR(255),
    state addon_state NOT NULL DEFAULT 'provisioning',
    config_vars JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_addons_app_id ON addons(app_id);
CREATE INDEX idx_addons_state ON addons(state);

-- ============================================================================
-- Collaborators
-- ============================================================================

CREATE TABLE collaborators (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role team_role NOT NULL DEFAULT 'member',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(app_id, user_id)
);

CREATE INDEX idx_collaborators_app_id ON collaborators(app_id);
CREATE INDEX idx_collaborators_user_id ON collaborators(user_id);

-- ============================================================================
-- Pipelines
-- ============================================================================

CREATE TABLE pipelines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_pipelines_owner_id ON pipelines(owner_id);

CREATE TYPE pipeline_stage AS ENUM ('development', 'staging', 'production');

CREATE TABLE pipeline_couplings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pipeline_id UUID NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    stage pipeline_stage NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(pipeline_id, app_id),
    UNIQUE(pipeline_id, stage)
);

CREATE INDEX idx_pipeline_couplings_pipeline_id ON pipeline_couplings(pipeline_id);
CREATE INDEX idx_pipeline_couplings_app_id ON pipeline_couplings(app_id);

-- ============================================================================
-- Log Drains
-- ============================================================================

CREATE TABLE log_drains (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    token VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_log_drains_app_id ON log_drains(app_id);

-- ============================================================================
-- Webhooks
-- ============================================================================

CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret VARCHAR(255),
    events TEXT[] NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_webhooks_app_id ON webhooks(app_id);

-- ============================================================================
-- Audit Log
-- ============================================================================

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    app_id UUID REFERENCES apps(id),
    action VARCHAR(255) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID,
    metadata JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_app_id ON audit_logs(app_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);

-- ============================================================================
-- Scheduled Jobs (Cron)
-- ============================================================================

CREATE TABLE scheduled_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    schedule VARCHAR(255) NOT NULL,
    command TEXT NOT NULL,
    size VARCHAR(50) NOT NULL DEFAULT 'standard-1x',
    enabled BOOLEAN DEFAULT TRUE,
    last_run_at TIMESTAMP WITH TIME ZONE,
    next_run_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(app_id, name)
);

CREATE INDEX idx_scheduled_jobs_app_id ON scheduled_jobs(app_id);
CREATE INDEX idx_scheduled_jobs_next_run ON scheduled_jobs(next_run_at) WHERE enabled = TRUE;

-- ============================================================================
-- Regions & Stacks
-- ============================================================================

CREATE TABLE regions (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    available BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE stacks (
    name VARCHAR(50) PRIMARY KEY,
    state VARCHAR(50) NOT NULL DEFAULT 'available',
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- Billing (Optional)
-- ============================================================================

CREATE TABLE billing_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    team_id UUID REFERENCES teams(id),
    stripe_customer_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT billing_owner CHECK (
        (user_id IS NOT NULL AND team_id IS NULL) OR
        (user_id IS NULL AND team_id IS NOT NULL)
    )
);

CREATE INDEX idx_billing_accounts_user_id ON billing_accounts(user_id);
CREATE INDEX idx_billing_accounts_team_id ON billing_accounts(team_id);

CREATE TABLE usage_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    billing_account_id UUID NOT NULL REFERENCES billing_accounts(id),
    resource_type VARCHAR(50) NOT NULL,
    quantity DECIMAL(10, 2) NOT NULL,
    unit VARCHAR(50) NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_usage_records_app_id ON usage_records(app_id);
CREATE INDEX idx_usage_records_billing_account ON usage_records(billing_account_id);
CREATE INDEX idx_usage_records_time ON usage_records(start_time, end_time);

-- ============================================================================
-- Functions & Triggers
-- ============================================================================

-- Update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply to all tables with updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_teams_updated_at BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_apps_updated_at BEFORE UPDATE ON apps
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_builds_updated_at BEFORE UPDATE ON builds
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_releases_updated_at BEFORE UPDATE ON releases
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_formations_updated_at BEFORE UPDATE ON formations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_dynos_updated_at BEFORE UPDATE ON dynos
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_domains_updated_at BEFORE UPDATE ON domains
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_ssl_certificates_updated_at BEFORE UPDATE ON ssl_certificates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Auto-increment release version
CREATE OR REPLACE FUNCTION increment_release_version()
RETURNS TRIGGER AS $$
BEGIN
    NEW.version := COALESCE(
        (SELECT MAX(version) FROM releases WHERE app_id = NEW.app_id),
        0
    ) + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER auto_increment_release_version BEFORE INSERT ON releases
    FOR EACH ROW EXECUTE FUNCTION increment_release_version();

-- Ensure only one current release per app
CREATE OR REPLACE FUNCTION ensure_single_current_release()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.current = TRUE THEN
        UPDATE releases SET current = FALSE
        WHERE app_id = NEW.app_id AND id != NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ensure_single_current_release_trigger AFTER INSERT OR UPDATE ON releases
    FOR EACH ROW EXECUTE FUNCTION ensure_single_current_release();

-- ============================================================================
-- Seed Data
-- ============================================================================

-- Insert default regions
INSERT INTO regions (id, name, provider, available) VALUES
    ('us-west-2', 'US West (Oregon)', 'aws', true),
    ('us-east-1', 'US East (N. Virginia)', 'aws', true),
    ('us-central1', 'US Central (Iowa)', 'gcp', true),
    ('europe-west1', 'Europe West (Belgium)', 'gcp', true),
    ('eastus', 'East US', 'azure', true);

-- Insert default stacks
INSERT INTO stacks (name, state, description) VALUES
    ('heroku-22', 'available', 'Ubuntu 22.04 LTS'),
    ('heroku-20', 'deprecated', 'Ubuntu 20.04 LTS');

-- ============================================================================
-- Views for Common Queries
-- ============================================================================

-- Current release for each app
CREATE VIEW current_releases AS
SELECT r.*
FROM releases r
WHERE r.current = TRUE;

-- App with current release info
CREATE VIEW apps_with_current_release AS
SELECT
    a.*,
    r.id as current_release_id,
    r.version as current_release_version,
    r.slug_id as current_slug_id
FROM apps a
LEFT JOIN current_releases r ON r.app_id = a.id;

-- Dyno counts by app
CREATE VIEW dyno_counts AS
SELECT
    app_id,
    type,
    COUNT(*) as count,
    SUM(CASE WHEN state = 'up' THEN 1 ELSE 0 END) as up_count
FROM dynos
GROUP BY app_id, type;

-- ============================================================================
-- Indexes for Performance
-- ============================================================================

-- Composite indexes for common queries
CREATE INDEX idx_releases_app_current ON releases(app_id, current) WHERE current = TRUE;
CREATE INDEX idx_dynos_app_state ON dynos(app_id, state);
CREATE INDEX idx_builds_app_status_created ON builds(app_id, status, created_at DESC);

-- Full-text search on apps (optional)
-- CREATE INDEX idx_apps_name_trgm ON apps USING gin(name gin_trgm_ops);

-- ============================================================================
-- Comments for Documentation
-- ============================================================================

COMMENT ON TABLE apps IS 'Application metadata and configuration';
COMMENT ON TABLE releases IS 'Immutable deployment versions combining slugs and config';
COMMENT ON TABLE slugs IS 'Compiled application artifacts from builds';
COMMENT ON TABLE builds IS 'Build process tracking and results';
COMMENT ON TABLE dynos IS 'Running container instances';
COMMENT ON TABLE formations IS 'Desired state for process types and scaling';
COMMENT ON TABLE config_vars IS 'Environment variables for applications';
COMMENT ON TABLE domains IS 'Custom domains mapped to applications';
COMMENT ON TABLE addons IS 'Provisioned third-party services';
COMMENT ON TABLE audit_logs IS 'Audit trail of all platform actions';
