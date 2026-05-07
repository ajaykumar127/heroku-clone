# Proof of Concept: Git-Push Deployment with Buildpacks

This POC demonstrates the core functionality of a Heroku-like PaaS:
1. Git push triggers build
2. Buildpack detects and builds application
3. Deploy to Kubernetes
4. Route traffic to application

## Components

```
┌─────────────┐
│   Git Push  │
└──────┬──────┘
       │
┌──────▼──────────────┐
│  Git Server (SSH)   │
│  - Receive hook     │
│  - Trigger build    │
└──────┬──────────────┘
       │
┌──────▼──────────────┐
│  Build Controller   │
│  - CNB integration  │
│  - Create OCI image │
└──────┬──────────────┘
       │
┌──────▼──────────────┐
│  K8s Deployment     │
│  - Create pods      │
│  - Service/Ingress  │
└──────┬──────────────┘
       │
┌──────▼──────────────┐
│  Running App        │
└─────────────────────┘
```

## Files

- `api-server/` - Simple API server (Go)
- `git-server/` - Git server with receive hooks (Go)
- `builder/` - Buildpack runner using Cloud Native Buildpacks
- `deployer/` - Kubernetes deployment manager
- `k8s/` - Kubernetes manifests
- `example-app/` - Sample Node.js app to deploy

## Quick Start

```bash
# 1. Start local Kubernetes (Docker Desktop or kind)
kind create cluster --name platform-poc

# 2. Build and deploy POC components
./scripts/deploy-poc.sh

# 3. Deploy example app
cd example-app
git init
git add .
git commit -m "Initial commit"
git remote add platform ssh://git@localhost:2222/my-app.git
git push platform main

# 4. Access app
curl http://my-app.localhost
```

## Architecture Details

### 1. Git Server
- SSH server listening on port 2222
- Authenticates via SSH keys
- `pre-receive` hook triggers build on push
- Streams build logs back to git client

### 2. Build System
- Uses `pack` CLI from Cloud Native Buildpacks
- Detects language automatically
- Builds OCI image
- Pushes to local registry

### 3. Deployment
- Generates Kubernetes manifests dynamically
- Creates Deployment, Service, and Ingress
- Rolling updates on new releases
- Health checks and readiness probes

### 4. Routing
- Nginx Ingress Controller
- Automatic routing: `{app-name}.localhost`
- Custom domain support

## Technology Stack

- **Language**: Go 1.21+
- **Container Runtime**: Docker
- **Orchestration**: Kubernetes 1.28+
- **Buildpacks**: Paketo Buildpacks
- **Registry**: Local Docker registry
- **Database**: SQLite (for POC simplicity)

## Next Steps

After POC validation:
1. Replace SQLite with PostgreSQL
2. Add release management
3. Implement config vars
4. Add scaling API
5. Implement log aggregation
6. Add authentication/authorization
7. Build CLI tool
