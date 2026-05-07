# Quick Reference Guide

## Git Push to Production Flow

```
Developer                Git Server              Builder                K8s Cluster
    │                        │                      │                       │
    │  git push platform     │                      │                       │
    │───────────────────────>│                      │                       │
    │                        │                      │                       │
    │                        │  Trigger Build       │                       │
    │                        │─────────────────────>│                       │
    │                        │                      │                       │
    │                        │                      │  1. Clone repo        │
    │                        │                      │  2. Detect language   │
    │                        │                      │  3. Run buildpack     │
    │                        │                      │  4. Create image      │
    │                        │                      │  5. Push to registry  │
    │                        │                      │                       │
    │                        │                      │  Deploy to K8s        │
    │                        │                      │──────────────────────>│
    │                        │                      │                       │
    │                        │                      │                       │  Create Pods
    │                        │                      │                       │  Update Ingress
    │                        │                      │                       │  Rolling Update
    │                        │                      │                       │
    │                        │  Stream logs         │                       │
    │<───────────────────────│<─────────────────────│<──────────────────────│
    │                        │                      │                       │
    │  "Deploy complete!"    │                      │                       │
    │<───────────────────────│                      │                       │
    │                        │                      │                       │
```

## CLI Commands Cheat Sheet

### Authentication
```bash
platform login                          # Login to platform
platform logout                         # Logout
platform auth:whoami                    # Show current user
```

### App Management
```bash
platform create my-app                  # Create new app
platform apps                           # List all apps
platform apps:info -a my-app            # Show app details
platform apps:destroy -a my-app         # Delete app
```

### Deployment
```bash
git push platform main                  # Deploy via git
platform releases -a my-app             # List releases
platform releases:info v42 -a my-app    # Release details
platform rollback v40 -a my-app         # Rollback to version
```

### Configuration
```bash
platform config -a my-app               # Show config vars
platform config:set KEY=value           # Set config var
platform config:get KEY                 # Get single var
platform config:unset KEY               # Remove var
```

### Scaling
```bash
platform ps -a my-app                   # List dynos
platform ps:scale web=3 worker=2        # Scale processes
platform ps:type                        # List dyno sizes
platform ps:restart -a my-app           # Restart all dynos
```

### Logs
```bash
platform logs -a my-app                 # Show recent logs
platform logs --tail                    # Stream logs
platform logs --source app              # Filter by source
platform logs --dyno web.1              # Filter by dyno
```

### Domains
```bash
platform domains -a my-app              # List domains
platform domains:add example.com        # Add custom domain
platform domains:remove example.com     # Remove domain
```

### Add-ons
```bash
platform addons                         # List available add-ons
platform addons -a my-app               # List app add-ons
platform addons:create postgres:std-0   # Provision add-on
platform addons:destroy postgres-123    # Remove add-on
```

### Running Commands
```bash
platform run bash -a my-app             # Interactive shell
platform run rake db:migrate            # Run one-off command
platform run:detached worker.js         # Background job
```

### Teams
```bash
platform teams                          # List teams
platform teams:create engineering       # Create team
platform access -a my-app               # List collaborators
platform access:add user@example.com    # Add collaborator
```

## API Endpoints Quick Reference

### Apps
```
GET    /v1/apps                    # List apps
POST   /v1/apps                    # Create app
GET    /v1/apps/:app               # Get app
PATCH  /v1/apps/:app               # Update app
DELETE /v1/apps/:app               # Delete app
```

### Releases
```
GET    /v1/apps/:app/releases           # List releases
POST   /v1/apps/:app/releases           # Create release
GET    /v1/apps/:app/releases/:version  # Get release
POST   /v1/apps/:app/releases/:version/rollback
```

### Config
```
GET    /v1/apps/:app/config-vars        # Get config
PATCH  /v1/apps/:app/config-vars        # Update config
```

### Formation (Scaling)
```
GET    /v1/apps/:app/formation          # Get formation
PATCH  /v1/apps/:app/formation          # Scale app
```

### Dynos
```
GET    /v1/apps/:app/dynos              # List dynos
POST   /v1/apps/:app/dynos              # Run command
DELETE /v1/apps/:app/dynos/:dyno        # Stop dyno
```

### Logs
```
GET    /v1/apps/:app/logs               # Stream logs (SSE)
```

## Database Schema Quick Reference

### Core Tables
```sql
users              -- User accounts
api_tokens         -- API authentication
teams              -- Organizations
team_memberships   -- User-team relationships

apps               -- Applications
releases           -- Deployment versions
slugs              -- Build artifacts
builds             -- Build processes
config_vars        -- Environment variables

formations         -- Process types & scaling
dynos              -- Running containers

domains            -- Custom domains
ssl_certificates   -- SSL/TLS certificates

addons             -- Provisioned services
collaborators      -- App access control
audit_logs         -- Action history
```

### Key Relationships
```
User ──< Apps ──< Releases ──< Slugs
              ──< Builds
              ──< Config Vars
              ──< Formations ──< Dynos
              ──< Domains ──< SSL Certificates
              ──< Add-ons
              ──< Collaborators
```

## Buildpack Detection Order

1. **Node.js**: `package.json` present
2. **Python**: `requirements.txt`, `Pipfile`, or `setup.py`
3. **Ruby**: `Gemfile`
4. **Go**: `go.mod`
5. **Java**: `pom.xml` or `build.gradle`
6. **PHP**: `composer.json`
7. **Dockerfile**: `Dockerfile` present
8. **Static**: `index.html` in root

## Dyno Sizes & Pricing

| Size | CPU | Memory | Price/month |
|------|-----|--------|-------------|
| `standard-1x` | 0.5 cores | 512 MB | $25 |
| `standard-2x` | 1 core | 1 GB | $50 |
| `performance-m` | 2.5 cores | 2.5 GB | $250 |
| `performance-l` | 12 cores | 14 GB | $500 |

## Common Procfile Examples

### Node.js
```
web: npm start
worker: node worker.js
```

### Python
```
web: gunicorn app:app
worker: celery -A tasks worker
clock: python clock.py
```

### Ruby
```
web: bundle exec puma -C config/puma.rb
worker: bundle exec sidekiq
```

### Go
```
web: ./bin/server
worker: ./bin/worker
```

## Environment Variables

### Automatic Variables
```bash
PORT              # Port to listen on (assigned by platform)
DATABASE_URL      # Primary database URL (if postgres add-on)
REDIS_URL         # Redis URL (if redis add-on)
```

### Custom Variables
```bash
platform config:set \
  NODE_ENV=production \
  SECRET_KEY=abc123 \
  API_KEY=xyz789
```

## Kubernetes Mapping

| Platform Concept | Kubernetes Resource |
|------------------|---------------------|
| App | Namespace |
| Dyno | Pod |
| Process Type | Deployment |
| Formation | ReplicaSet |
| Domain | Ingress |
| SSL Certificate | Secret (TLS) |
| Config Var | ConfigMap / Secret |
| One-off Dyno | Job |
| Scheduled Job | CronJob |

## Common Troubleshooting

### Build Failures
```bash
# View build logs
platform releases:output -a my-app

# Check buildpack detection
ls -la                          # Ensure required files exist
cat package.json                # Verify correct format

# Clear build cache
platform repo:purge-cache -a my-app
```

### Runtime Crashes
```bash
# Check logs
platform logs --tail -a my-app

# Check dyno status
platform ps -a my-app

# Restart dynos
platform ps:restart -a my-app

# Check config
platform config -a my-app
```

### Scaling Issues
```bash
# Check current formation
platform ps -a my-app

# Check dyno metrics
platform ps:metrics -a my-app

# Enable autoscaling
platform ps:autoscale:enable web --min 2 --max 10
```

### Domain/SSL Issues
```bash
# Check DNS
dig example.com

# Check SSL status
platform certs -a my-app

# Check ingress
kubectl get ingress -n app-<uuid>
```

## Monitoring Queries

### Prometheus Queries
```promql
# Request rate
rate(http_requests_total[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m])

# Latency p95
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Dyno CPU usage
container_cpu_usage_seconds_total{namespace="apps"}

# Dyno memory usage
container_memory_usage_bytes{namespace="apps"}
```

### Loki Queries
```logql
# All app logs
{app="my-app"}

# Error logs only
{app="my-app"} |= "error"

# Router logs
{source="router", app="my-app"}

# Rate of errors
rate({app="my-app"} |= "error" [5m])
```

## Security Best Practices

1. **Never commit secrets** - Use config vars
2. **Use API tokens** for CI/CD, not passwords
3. **Rotate tokens regularly** - Every 90 days
4. **Enable 2FA** on platform account
5. **Review collaborators** - Remove unused access
6. **Use teams** for shared apps
7. **Audit logs** - Review regularly
8. **Custom domains** - Always use HTTPS
9. **Database encryption** - Enable at rest encryption
10. **Network policies** - Restrict inter-app communication

## Performance Tuning

### Application Level
- Enable HTTP caching headers
- Use CDN for static assets
- Implement database connection pooling
- Add Redis for session storage
- Use async/background jobs for slow tasks

### Platform Level
- Right-size dynos (avoid over-provisioning)
- Enable autoscaling for traffic spikes
- Use performance dynos for CPU-intensive apps
- Configure health checks properly
- Set appropriate timeouts

### Database Level
- Add indexes for common queries
- Use read replicas for read-heavy apps
- Enable connection pooling
- Monitor slow queries
- Regular VACUUM on Postgres

## Getting Help

```bash
platform help                    # List all commands
platform help <command>          # Command-specific help
platform status                  # Platform status
```

**Documentation**: https://docs.platform.com  
**Status Page**: https://status.platform.com  
**Support**: support@platform.com
