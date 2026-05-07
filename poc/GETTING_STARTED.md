# Getting Started with Platform POC

This guide will help you get the Platform POC running locally.

## Prerequisites

Before you begin, make sure you have:

- **Go 1.21+** - [Install Go](https://golang.org/dl/)
- **Git** - [Install Git](https://git-scm.com/downloads)
- **Make** (optional, but recommended) - Usually pre-installed on macOS/Linux
- **Docker** (optional, for Docker Compose setup) - [Install Docker](https://docs.docker.com/get-docker/)

### Optional (for full Kubernetes deployment)
- **kubectl** - [Install kubectl](https://kubernetes.io/docs/tasks/tools/)
- **Docker Desktop** with Kubernetes enabled, or **kind** - [Install kind](https://kind.sigs.k8s.io/)
- **pack CLI** (Cloud Native Buildpacks) - [Install pack](https://buildpacks.io/docs/tools/pack/)

## Quick Start (3 Methods)

### Method 1: Using Make (Recommended)

```bash
# Clone the repository (if you haven't)
cd ~/heroku-clone/poc

# Build and start services
make start

# In another terminal, create an app
make create-app NAME=my-app

# Check status
make status

# View logs
make logs

# Stop services
make stop
```

**Access the dashboard**: Open http://localhost:8080 in your browser

### Method 2: Using Docker Compose

```bash
cd ~/heroku-clone/poc

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

**Access the dashboard**: Open http://localhost:8080 in your browser

### Method 3: Manual Setup

```bash
cd ~/heroku-clone/poc

# Build API server
cd api-server
go mod tidy
go build -o ../bin/api-server .
cd ..

# Build Git server
cd git-server
go mod tidy
go build -o ../bin/git-server .
cd ..

# Start API server
./bin/api-server > logs/api-server.log 2>&1 &

# Start Git server
./bin/git-server > logs/git-server.log 2>&1 &
```

## Deploying Your First Application

### Step 1: Create an App

Using the API:
```bash
curl -X POST http://localhost:8080/v1/apps \
  -H "Content-Type: application/json" \
  -d '{"name":"my-app"}'
```

Or using Make:
```bash
make create-app NAME=my-app
```

You should get a response like:
```json
{
  "id": "...",
  "name": "my-app",
  "git_url": "ssh://git@localhost:2222/my-app.git",
  "web_url": "http://my-app.localhost",
  "created_at": "2024-04-29T..."
}
```

### Step 2: Deploy the Example App

```bash
# Navigate to the example app
cd example-app

# Initialize git repository
git init

# Add files
git add .

# Commit
git commit -m "Initial commit"

# Add platform remote
git remote add platform ssh://git@localhost:2222/my-app.git

# Push to deploy!
git push platform main
```

You'll see output like:
```
-----> Receiving push for my-app
-----> Triggering build for commit abc123
-----> Build queued successfully
-----> Deploy in progress...
```

### Step 3: Access Your App

The POC doesn't deploy to Kubernetes automatically (that's phase 2), but you can:

1. **View app in dashboard**: http://localhost:8080
2. **Check releases**: 
   ```bash
   curl http://localhost:8080/v1/apps/my-app/releases | jq
   ```

## Available Commands

### Makefile Commands

```bash
make help              # Show all available commands
make build             # Build all components
make start             # Start all services
make stop              # Stop all services
make restart           # Restart all services
make status            # Show service status
make logs              # Tail all logs
make logs-api          # Tail API server logs only
make logs-git          # Tail Git server logs only
make clean             # Clean all build artifacts
make test              # Run tests
make create-app        # Create a new app (NAME=appname)
make list-apps         # List all apps
make demo              # Run complete demo
```

### API Endpoints

**Apps:**
```bash
# List apps
curl http://localhost:8080/v1/apps

# Create app
curl -X POST http://localhost:8080/v1/apps \
  -H "Content-Type: application/json" \
  -d '{"name":"test-app"}'

# Get app
curl http://localhost:8080/v1/apps/test-app

# Delete app
curl -X DELETE http://localhost:8080/v1/apps/test-app
```

**Releases:**
```bash
# List releases
curl http://localhost:8080/v1/apps/my-app/releases

# Create release
curl -X POST http://localhost:8080/v1/apps/my-app/releases \
  -H "Content-Type: application/json" \
  -d '{"commit":"abc123"}'
```

**Health Check:**
```bash
curl http://localhost:8080/health
```

## Configuration

### Environment Variables

**API Server:**
- `DATABASE_URL` - Database connection string (default: `./platform.db`)
  - SQLite: `./platform.db`
  - PostgreSQL: `postgres://user:pass@host:5432/dbname?sslmode=disable`
- `PORT` - HTTP port (default: `8080`)
- `ENVIRONMENT` - Environment name (default: `development`)

**Git Server:**
- `API_URL` - API server URL (default: `http://localhost:8080`)
- `LISTEN_ADDR` - SSH listen address (default: `:2222`)

### Using PostgreSQL Instead of SQLite

With Docker Compose (already configured):
```bash
docker-compose up -d
```

Or manually:
```bash
# Start PostgreSQL
docker run -d \
  --name platform-postgres \
  -e POSTGRES_DB=platform \
  -e POSTGRES_USER=platform \
  -e POSTGRES_PASSWORD=platform_secret \
  -p 5432:5432 \
  postgres:15-alpine

# Start API server with PostgreSQL
DATABASE_URL="postgres://platform:platform_secret@localhost:5432/platform?sslmode=disable" \
  ./bin/api-server
```

## Project Structure

```
poc/
├── api-server/           # API server (Go)
│   ├── main.go          # Main application
│   ├── config.go        # Configuration
│   ├── middleware.go    # HTTP middleware
│   └── dashboard.go     # Web dashboard
├── git-server/          # Git server (Go)
│   └── main.go          # SSH git server
├── builder/             # Build scripts
│   └── builder.sh       # Buildpack runner
├── deployer/            # Deployment scripts
│   └── deploy.sh        # Kubernetes deployer
├── example-app/         # Sample Node.js app
│   ├── index.js
│   └── package.json
├── docker/              # Dockerfiles
│   ├── api-server.Dockerfile
│   └── git-server.Dockerfile
├── bin/                 # Compiled binaries
├── logs/                # Log files
├── repos/               # Git repositories
├── Makefile            # Build automation
├── docker-compose.yml  # Docker Compose config
└── README.md           # Documentation
```

## Troubleshooting

### Services Won't Start

**Check if ports are already in use:**
```bash
lsof -i :8080   # API server
lsof -i :2222   # Git server
lsof -i :5432   # PostgreSQL (if using)
```

**Kill existing processes:**
```bash
make stop
# or
pkill -f "bin/api-server"
pkill -f "bin/git-server"
```

### Git Push Fails

**Check Git server logs:**
```bash
make logs-git
# or
tail -f logs/git-server.log
```

**Verify SSH key permissions:**
```bash
ssh-add -l  # List SSH keys
```

**Test SSH connection:**
```bash
ssh -p 2222 git@localhost
```

### Database Connection Errors

**SQLite:**
```bash
# Check if database file exists
ls -la platform.db

# Check permissions
chmod 644 platform.db
```

**PostgreSQL:**
```bash
# Test connection
psql postgres://platform:platform_secret@localhost:5432/platform

# Check if PostgreSQL is running
docker ps | grep postgres
```

### Can't Access Dashboard

**Verify API server is running:**
```bash
make status
# or
curl http://localhost:8080/health
```

**Check logs for errors:**
```bash
make logs-api
```

## Next Steps

Once you have the basic POC working:

1. **Add Kubernetes Deployment** - Use the deployer scripts to deploy to a local K8s cluster
2. **Integrate Cloud Native Buildpacks** - Install `pack` CLI and test builds
3. **Add Authentication** - Implement JWT or API token auth
4. **Build a CLI Tool** - Create a command-line interface
5. **Add More Features** - Config vars, scaling, rollback, etc.

## Development

### Running Tests

```bash
make test
```

### Building for Production

```bash
# Build optimized binaries
cd api-server
CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o ../bin/api-server .

cd ../git-server
CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ../bin/git-server .
```

### Docker Images

```bash
# Build Docker images
make docker-build

# Push to registry
docker tag platform/api-server:latest your-registry/platform-api:latest
docker push your-registry/platform-api:latest
```

## Additional Resources

- **Full Documentation**: `../docs/`
- **Architecture**: `../docs/ARCHITECTURE.md`
- **API Schema**: `../api-design/API_SCHEMA.md`
- **Database Schema**: `../api-design/DATABASE_SCHEMA.sql`
- **Implementation Guide**: `../docs/IMPLEMENTATION_GUIDE.md`

## Getting Help

- Check the logs: `make logs`
- Review the documentation in `../docs/`
- Inspect the API: http://localhost:8080
- View the dashboard: http://localhost:8080

## Clean Up

```bash
# Stop services and clean everything
make clean

# Remove Docker containers
docker-compose down -v

# Remove built images
docker rmi platform/api-server platform/git-server
```

---

**Happy deploying! 🚀**
