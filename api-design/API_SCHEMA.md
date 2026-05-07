# Platform API Schema & Design

Complete RESTful API specification for the cloud-agnostic PaaS platform.

## API Design Principles

1. **RESTful**: Resources with standard HTTP verbs
2. **Versioned**: `/v1/` prefix for stability
3. **JSON**: All requests/responses in JSON
4. **Idempotent**: PUT/DELETE operations safe to retry
5. **Paginated**: List endpoints support pagination
6. **Filterable**: Query parameters for filtering
7. **Authenticated**: All endpoints require auth (except public)
8. **Rate Limited**: Per-user and per-org limits

## Base URL

```
https://api.platform.example.com/v1
```

## Authentication

### Methods

**1. API Tokens** (recommended for CI/CD)
```http
Authorization: Bearer <api-token>
```

**2. OAuth2** (for web applications)
```http
Authorization: Bearer <oauth-access-token>
```

**3. SSH Keys** (for git operations)
```bash
ssh-keygen -t ed25519 -C "user@example.com"
# Upload public key via API
```

### Endpoints

```http
POST   /auth/login              # Email/password login
POST   /auth/logout             # Invalidate token
POST   /auth/refresh            # Refresh access token
GET    /auth/user               # Get current user
POST   /auth/tokens             # Create API token
GET    /auth/tokens             # List API tokens
DELETE /auth/tokens/:id         # Revoke API token
POST   /auth/keys               # Add SSH key
GET    /auth/keys               # List SSH keys
DELETE /auth/keys/:id           # Remove SSH key
```

---

## Core Resources

## 1. Applications

The primary resource representing a deployed application.

### Data Model

```json
{
  "id": "01234567-89ab-cdef-0123-456789abcdef",
  "name": "my-app",
  "owner": {
    "id": "user-uuid",
    "email": "user@example.com"
  },
  "team": {
    "id": "team-uuid",
    "name": "engineering"
  },
  "region": "us-west-2",
  "stack": "heroku-22",
  "buildpack_provided_description": "Node.js",
  "git_url": "git@platform.example.com:my-app.git",
  "web_url": "https://my-app.platform.example.com",
  "repo_size": 45678901,
  "slug_size": 23456789,
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-04-29T14:22:00Z",
  "maintenance": false,
  "archived": false,
  "formation": {
    "web": {
      "quantity": 2,
      "size": "standard-1x"
    },
    "worker": {
      "quantity": 1,
      "size": "standard-2x"
    }
  }
}
```

### Endpoints

```http
# Application Management
GET    /apps                    # List all apps
POST   /apps                    # Create new app
GET    /apps/:app               # Get app details
PATCH  /apps/:app               # Update app
DELETE /apps/:app               # Delete app

# App State
POST   /apps/:app/maintenance   # Enable maintenance mode
DELETE /apps/:app/maintenance   # Disable maintenance mode
POST   /apps/:app/refresh       # Refresh git repo
```

### Examples

**Create Application**
```http
POST /v1/apps
Content-Type: application/json

{
  "name": "my-app",
  "region": "us-west-2",
  "stack": "heroku-22",
  "team": "engineering"
}
```

**Response**
```json
{
  "id": "01234567-89ab-cdef-0123-456789abcdef",
  "name": "my-app",
  "git_url": "git@platform.example.com:my-app.git",
  "web_url": "https://my-app.platform.example.com",
  "created_at": "2024-04-29T14:22:00Z"
}
```

**List Applications**
```http
GET /v1/apps?team=engineering&limit=50&offset=0
```

---

## 2. Releases

Immutable combination of slug + config. Each deployment creates a new release.

### Data Model

```json
{
  "id": "release-uuid",
  "version": 42,
  "app": {
    "id": "app-uuid",
    "name": "my-app"
  },
  "slug": {
    "id": "slug-uuid",
    "commit": "abc123def456",
    "checksum": "SHA256:1234567890abcdef...",
    "size": 23456789
  },
  "user": {
    "id": "user-uuid",
    "email": "deployer@example.com"
  },
  "description": "Deploy abc123d",
  "status": "succeeded",
  "current": true,
  "created_at": "2024-04-29T14:22:00Z",
  "updated_at": "2024-04-29T14:25:00Z",
  "output_stream_url": "https://logs.platform.example.com/streams/release-uuid"
}
```

### Endpoints

```http
GET    /apps/:app/releases           # List releases
POST   /apps/:app/releases           # Create release
GET    /apps/:app/releases/:version  # Get release details
POST   /apps/:app/releases/:version/rollback  # Rollback
```

### Examples

**List Releases**
```http
GET /v1/apps/my-app/releases
```

**Response**
```json
{
  "releases": [
    {
      "version": 42,
      "description": "Deploy abc123d",
      "status": "succeeded",
      "current": true,
      "created_at": "2024-04-29T14:22:00Z"
    },
    {
      "version": 41,
      "description": "Set DATABASE_URL config",
      "status": "succeeded",
      "current": false,
      "created_at": "2024-04-28T10:15:00Z"
    }
  ],
  "pagination": {
    "next": "/v1/apps/my-app/releases?offset=50",
    "total": 42
  }
}
```

**Rollback**
```http
POST /v1/apps/my-app/releases/40/rollback
```

---

## 3. Config Vars (Environment Variables)

Key-value pairs for application configuration.

### Data Model

```json
{
  "DATABASE_URL": "postgres://...",
  "REDIS_URL": "redis://...",
  "SECRET_KEY": "supersecret",
  "NODE_ENV": "production"
}
```

### Endpoints

```http
GET    /apps/:app/config-vars        # Get all config vars
PATCH  /apps/:app/config-vars        # Set/update config vars
DELETE /apps/:app/config-vars/:key   # Delete config var
```

### Examples

**Get Config Vars**
```http
GET /v1/apps/my-app/config-vars
```

**Response**
```json
{
  "DATABASE_URL": "postgres://...",
  "REDIS_URL": "redis://...",
  "NODE_ENV": "production"
}
```

**Set Config Vars** (creates new release)
```http
PATCH /v1/apps/my-app/config-vars
Content-Type: application/json

{
  "DATABASE_URL": "postgres://new-db...",
  "NEW_VAR": "value"
}
```

**Response**
```json
{
  "release": {
    "version": 43,
    "description": "Set DATABASE_URL, NEW_VAR config vars"
  },
  "config": {
    "DATABASE_URL": "postgres://new-db...",
    "REDIS_URL": "redis://...",
    "NODE_ENV": "production",
    "NEW_VAR": "value"
  }
}
```

---

## 4. Dynos (Containers/Processes)

Running instances of your application.

### Data Model

```json
{
  "id": "dyno-uuid",
  "name": "web.1",
  "type": "web",
  "size": "standard-1x",
  "state": "up",
  "app": {
    "id": "app-uuid",
    "name": "my-app"
  },
  "release": {
    "version": 42
  },
  "command": "npm start",
  "created_at": "2024-04-29T14:25:00Z",
  "updated_at": "2024-04-29T14:25:30Z"
}
```

### States
- `starting` - Container is booting
- `up` - Running and healthy
- `idle` - Running but no traffic
- `crashed` - Exited with error
- `down` - Stopped

### Endpoints

```http
GET    /apps/:app/dynos             # List dynos
POST   /apps/:app/dynos             # Run one-off dyno
GET    /apps/:app/dynos/:dyno       # Get dyno info
DELETE /apps/:app/dynos/:dyno       # Stop dyno
POST   /apps/:app/dynos/:dyno/restart  # Restart dyno
```

### Examples

**List Dynos**
```http
GET /v1/apps/my-app/dynos
```

**Response**
```json
{
  "dynos": [
    {
      "name": "web.1",
      "type": "web",
      "size": "standard-1x",
      "state": "up",
      "command": "npm start"
    },
    {
      "name": "web.2",
      "type": "web",
      "size": "standard-1x",
      "state": "up",
      "command": "npm start"
    },
    {
      "name": "worker.1",
      "type": "worker",
      "size": "standard-2x",
      "state": "up",
      "command": "node worker.js"
    }
  ]
}
```

**Run One-off Command**
```http
POST /v1/apps/my-app/dynos
Content-Type: application/json

{
  "command": "rake db:migrate",
  "size": "standard-1x",
  "attach": false,
  "timeout": 3600
}
```

**Restart All Dynos**
```http
DELETE /v1/apps/my-app/dynos
```

---

## 5. Formation (Process Scaling)

Defines the number and size of dynos for each process type.

### Data Model

```json
{
  "web": {
    "quantity": 2,
    "size": "standard-1x"
  },
  "worker": {
    "quantity": 1,
    "size": "standard-2x"
  },
  "clock": {
    "quantity": 1,
    "size": "standard-1x"
  }
}
```

### Dyno Sizes

| Size | CPU | Memory | Price/month |
|------|-----|--------|-------------|
| `standard-1x` | 0.5 cores | 512 MB | $25 |
| `standard-2x` | 1 core | 1 GB | $50 |
| `performance-m` | 2.5 cores | 2.5 GB | $250 |
| `performance-l` | 12 cores | 14 GB | $500 |

### Endpoints

```http
GET   /apps/:app/formation          # Get formation
PATCH /apps/:app/formation          # Update formation
```

### Examples

**Get Formation**
```http
GET /v1/apps/my-app/formation
```

**Scale Application**
```http
PATCH /v1/apps/my-app/formation
Content-Type: application/json

{
  "web": {
    "quantity": 5,
    "size": "performance-m"
  },
  "worker": {
    "quantity": 3,
    "size": "standard-2x"
  }
}
```

---

## 6. Builds

The process of converting source code into a slug.

### Data Model

```json
{
  "id": "build-uuid",
  "app": {
    "id": "app-uuid",
    "name": "my-app"
  },
  "source_blob": {
    "url": "https://github.com/user/repo/archive/abc123.tar.gz",
    "version": "abc123def456",
    "version_description": "Deploy abc123d"
  },
  "slug": {
    "id": "slug-uuid"
  },
  "status": "succeeded",
  "buildpacks": [
    {
      "name": "heroku/nodejs",
      "url": "https://buildpack-registry.example.com/nodejs",
      "version": "2.0.0"
    }
  ],
  "stack": "heroku-22",
  "user": {
    "id": "user-uuid",
    "email": "deployer@example.com"
  },
  "created_at": "2024-04-29T14:20:00Z",
  "updated_at": "2024-04-29T14:22:00Z",
  "output_stream_url": "https://logs.platform.example.com/streams/build-uuid"
}
```

### Endpoints

```http
GET  /apps/:app/builds              # List builds
POST /apps/:app/builds              # Create build
GET  /apps/:app/builds/:id          # Get build details
GET  /apps/:app/builds/:id/output   # Stream build output
```

### Examples

**Create Build from GitHub**
```http
POST /v1/apps/my-app/builds
Content-Type: application/json

{
  "source_blob": {
    "url": "https://github.com/user/repo/archive/main.tar.gz",
    "version": "abc123def456"
  }
}
```

**Stream Build Output** (Server-Sent Events)
```http
GET /v1/apps/my-app/builds/build-uuid/output
Accept: text/event-stream
```

**Response**
```
data: -----> Node.js app detected
data: -----> Installing node 18.x
data: -----> Installing dependencies
data:        npm install
data: -----> Build succeeded!
```

---

## 7. Domains

Custom domains for your application.

### Data Model

```json
{
  "id": "domain-uuid",
  "hostname": "www.example.com",
  "cname": "my-app.platform.example.com",
  "app": {
    "id": "app-uuid",
    "name": "my-app"
  },
  "kind": "custom",
  "ssl": {
    "enabled": true,
    "status": "ok",
    "issuer": "Let's Encrypt",
    "expires_at": "2024-07-28T00:00:00Z"
  },
  "status": "active",
  "created_at": "2024-04-01T10:00:00Z",
  "updated_at": "2024-04-29T14:22:00Z"
}
```

### Endpoints

```http
GET    /apps/:app/domains           # List domains
POST   /apps/:app/domains           # Add domain
GET    /apps/:app/domains/:domain   # Get domain info
DELETE /apps/:app/domains/:domain   # Remove domain
```

### Examples

**Add Custom Domain**
```http
POST /v1/apps/my-app/domains
Content-Type: application/json

{
  "hostname": "www.example.com"
}
```

**Response**
```json
{
  "id": "domain-uuid",
  "hostname": "www.example.com",
  "cname": "my-app.platform.example.com",
  "ssl": {
    "enabled": false,
    "status": "pending"
  },
  "instructions": {
    "message": "Configure your DNS provider to point www.example.com to my-app.platform.example.com",
    "cname_target": "my-app.platform.example.com"
  }
}
```

---

## 8. Logs

Application and system logs.

### Endpoints

```http
GET /apps/:app/logs                 # Stream logs (SSE)
GET /apps/:app/log-sessions         # Create log session
```

### Query Parameters

- `tail` - Stream real-time logs (boolean)
- `lines` - Number of lines (default: 100)
- `source` - Filter by source (app, heroku, router)
- `dyno` - Filter by dyno (web.1, worker.2)
- `since` - Start time (ISO 8601)

### Examples

**Get Recent Logs**
```http
GET /v1/apps/my-app/logs?lines=100&source=app
```

**Response**
```json
{
  "logs": [
    {
      "timestamp": "2024-04-29T14:22:00.123Z",
      "source": "app",
      "dyno": "web.1",
      "message": "Server listening on port 3000"
    },
    {
      "timestamp": "2024-04-29T14:22:15.456Z",
      "source": "router",
      "dyno": "web.1",
      "message": "GET / 200 12ms"
    }
  ]
}
```

**Stream Logs** (Server-Sent Events)
```http
GET /v1/apps/my-app/logs?tail=true
Accept: text/event-stream
```

**Response**
```
data: {"timestamp":"2024-04-29T14:22:00.123Z","source":"app","dyno":"web.1","message":"Server started"}
data: {"timestamp":"2024-04-29T14:22:15.456Z","source":"router","message":"GET / 200 12ms"}
```

---

## 9. Log Drains

Forward logs to external services.

### Data Model

```json
{
  "id": "drain-uuid",
  "app": {
    "id": "app-uuid",
    "name": "my-app"
  },
  "url": "syslog+tls://logs.example.com:514",
  "token": "drain-token-abc123",
  "created_at": "2024-04-01T10:00:00Z"
}
```

### Supported Protocols
- `https://` - HTTP/HTTPS endpoint
- `syslog://` - Syslog (TCP)
- `syslog+tls://` - Syslog over TLS

### Endpoints

```http
GET    /apps/:app/log-drains        # List log drains
POST   /apps/:app/log-drains        # Add log drain
DELETE /apps/:app/log-drains/:id    # Remove log drain
```

---

## 10. Add-ons

Provisioned services (databases, caching, monitoring, etc.).

### Data Model

```json
{
  "id": "addon-uuid",
  "name": "postgresql-primary",
  "addon_service": {
    "name": "heroku-postgresql"
  },
  "plan": {
    "name": "standard-0",
    "price": 50
  },
  "app": {
    "id": "app-uuid",
    "name": "my-app"
  },
  "config_vars": [
    "DATABASE_URL",
    "DATABASE_CONNECTION_POOL_URL"
  ],
  "state": "provisioned",
  "created_at": "2024-04-01T10:00:00Z",
  "updated_at": "2024-04-29T14:22:00Z"
}
```

### Endpoints

```http
# Add-on Management
GET    /apps/:app/addons            # List app add-ons
POST   /apps/:app/addons            # Provision add-on
GET    /apps/:app/addons/:addon     # Get add-on info
PATCH  /apps/:app/addons/:addon     # Update add-on (upgrade/downgrade)
DELETE /apps/:app/addons/:addon     # Deprovision add-on

# Add-on Services (Marketplace)
GET    /addon-services              # List available services
GET    /addon-services/:service     # Get service details
GET    /addon-services/:service/plans  # List service plans
```

### Examples

**Provision Add-on**
```http
POST /v1/apps/my-app/addons
Content-Type: application/json

{
  "plan": "heroku-postgresql:standard-0",
  "name": "postgresql-primary"
}
```

**Response**
```json
{
  "id": "addon-uuid",
  "name": "postgresql-primary",
  "plan": "standard-0",
  "config_vars": ["DATABASE_URL"],
  "state": "provisioning"
}
```

---

## 11. Collaborators

Users who have access to an application.

### Data Model

```json
{
  "id": "collaborator-uuid",
  "user": {
    "id": "user-uuid",
    "email": "collaborator@example.com"
  },
  "app": {
    "id": "app-uuid",
    "name": "my-app"
  },
  "role": "member",
  "created_at": "2024-04-01T10:00:00Z"
}
```

### Roles
- `admin` - Full access including deletion
- `member` - Deploy, scale, view logs
- `viewer` - Read-only access

### Endpoints

```http
GET    /apps/:app/collaborators     # List collaborators
POST   /apps/:app/collaborators     # Add collaborator
DELETE /apps/:app/collaborators/:id # Remove collaborator
```

---

## 12. Pipelines

Continuous delivery workflows between environments.

### Data Model

```json
{
  "id": "pipeline-uuid",
  "name": "my-app-pipeline",
  "owner": {
    "id": "user-uuid",
    "email": "owner@example.com"
  },
  "apps": {
    "development": {
      "id": "dev-app-uuid",
      "name": "my-app-dev"
    },
    "staging": {
      "id": "staging-app-uuid",
      "name": "my-app-staging"
    },
    "production": {
      "id": "prod-app-uuid",
      "name": "my-app"
    }
  },
  "created_at": "2024-04-01T10:00:00Z"
}
```

### Endpoints

```http
GET    /pipelines                   # List pipelines
POST   /pipelines                   # Create pipeline
GET    /pipelines/:id               # Get pipeline
DELETE /pipelines/:id               # Delete pipeline

# Promotions
POST   /pipelines/:id/promotions    # Promote release between stages
```

---

## Supporting Resources

## 13. Teams/Organizations

### Endpoints

```http
GET    /teams                       # List teams
POST   /teams                       # Create team
GET    /teams/:team                 # Get team details
PATCH  /teams/:team                 # Update team
DELETE /teams/:team                 # Delete team

# Members
GET    /teams/:team/members         # List members
POST   /teams/:team/members         # Add member
DELETE /teams/:team/members/:user   # Remove member
```

## 14. Regions

Available deployment regions.

```http
GET /regions
```

**Response**
```json
{
  "regions": [
    {
      "id": "us-west-2",
      "name": "US West (Oregon)",
      "provider": "aws",
      "available": true
    },
    {
      "id": "us-central1",
      "name": "US Central (Iowa)",
      "provider": "gcp",
      "available": true
    }
  ]
}
```

## 15. Stacks

Available runtime stacks (OS + base image).

```http
GET /stacks
```

**Response**
```json
{
  "stacks": [
    {
      "name": "heroku-22",
      "state": "available",
      "description": "Ubuntu 22.04 LTS"
    },
    {
      "name": "heroku-20",
      "state": "deprecated",
      "description": "Ubuntu 20.04 LTS"
    }
  ]
}
```

---

## Webhooks

Subscribe to application events.

### Events
- `api:build` - Build created/completed
- `api:release` - Release created
- `api:formation` - Formation changed (scale)
- `api:addon` - Add-on provisioned/deprovisioned
- `dyno` - Dyno state changed

### Endpoints

```http
POST   /apps/:app/webhooks          # Create webhook
GET    /apps/:app/webhooks          # List webhooks
DELETE /apps/:app/webhooks/:id      # Delete webhook
```

### Example Payload

```json
{
  "id": "event-uuid",
  "action": "create",
  "resource": "build",
  "data": {
    "id": "build-uuid",
    "status": "succeeded",
    "app": "my-app"
  },
  "created_at": "2024-04-29T14:22:00Z"
}
```

---

## Error Responses

### Standard Error Format

```json
{
  "error": {
    "id": "error-uuid",
    "message": "App not found",
    "code": "not_found",
    "details": {
      "app": "nonexistent-app"
    }
  }
}
```

### HTTP Status Codes

| Code | Meaning | Usage |
|------|---------|-------|
| 200 | OK | Successful GET, PATCH, DELETE |
| 201 | Created | Successful POST |
| 202 | Accepted | Async operation started |
| 400 | Bad Request | Invalid input |
| 401 | Unauthorized | Missing/invalid auth |
| 403 | Forbidden | Insufficient permissions |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Resource already exists |
| 422 | Unprocessable | Validation failed |
| 429 | Too Many Requests | Rate limited |
| 500 | Internal Server Error | Server error |
| 503 | Service Unavailable | Maintenance mode |

---

## Rate Limiting

### Headers

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1588291200
```

### Limits
- **Authenticated users**: 1000 req/hour
- **Organizations**: 5000 req/hour
- **Enterprise**: Custom limits

---

## Pagination

### Query Parameters
- `limit` - Items per page (default: 50, max: 200)
- `offset` - Offset for pagination

### Response

```json
{
  "data": [...],
  "pagination": {
    "next": "/v1/apps?offset=50&limit=50",
    "previous": "/v1/apps?offset=0&limit=50",
    "total": 142
  }
}
```

---

## API Client Libraries

### Official SDKs

```bash
# Node.js
npm install @platform/client

# Python
pip install platform-client

# Go
go get github.com/platform/go-client

# Ruby
gem install platform-client
```

### Example Usage

```javascript
// Node.js
const Platform = require('@platform/client');
const client = new Platform({ token: process.env.PLATFORM_API_TOKEN });

// Create app
const app = await client.apps.create({ name: 'my-app', region: 'us-west-2' });

// Deploy
await client.builds.create(app.id, {
  source_blob: { url: 'https://github.com/user/repo/archive/main.tar.gz' }
});

// Scale
await client.formation.update(app.id, {
  web: { quantity: 5, size: 'standard-2x' }
});

// Stream logs
const logs = client.logs.stream(app.id);
logs.on('data', (line) => console.log(line));
```

---

## OpenAPI Specification

Full OpenAPI 3.0 spec available at:
```
https://api.platform.example.com/v1/openapi.json
```

Can be used with:
- Swagger UI
- Postman
- Code generators (swagger-codegen, openapi-generator)

---

## Summary

This API provides comprehensive control over:
- ✅ Application lifecycle (create, deploy, scale, delete)
- ✅ Release management (versions, rollbacks)
- ✅ Configuration (environment variables)
- ✅ Process management (dynos, formation)
- ✅ Domains and SSL
- ✅ Logging and monitoring
- ✅ Add-ons and integrations
- ✅ Collaboration and access control
- ✅ CI/CD workflows (pipelines)

All operations available via:
- REST API (programmatic access)
- CLI tool (developer workflow)
- Web dashboard (visual interface)
