# Platform — Cloud-Agnostic PaaS (Heroku Clone)

Deploy apps with `git push` to any cloud. AWS, GCP, Azure, or on-premises — same CLI, same workflow.

```
git push platform main   # that's it
```

---

## What is this?

Platform is an open-source Heroku-style PaaS you host yourself. It separates into two planes:

- **Control Plane** — runs once, anywhere. Manages apps, auth, config, and job dispatch.
- **Runtime Plane** — one per cloud/region. Handles builds, deployments, and traffic. Deployable to AWS EKS, GCP GKE, or Azure AKS with the included Terraform.

Developers use one CLI and one API regardless of which cloud their apps run on.

---

## Architecture

```
Developer Tools (CLI / git push / Dashboard)
            │
    ┌───────▼────────────┐
    │    Control Plane   │  ← runs once, anywhere
    │  API · Auth · Jobs │
    └───────┬────────────┘
            │  HTTP polling (no inbound required)
   ┌────────┼────────────────┐
   │        │                │
┌──▼──┐  ┌──▼──┐  ┌──────────▼───┐
│ AWS │  │ GCP │  │    Azure     │
│ EKS │  │ GKE │  │    AKS       │
│ ECR │  │ GAR │  │    ACR       │
└─────┘  └─────┘  └──────────────┘
 Runtime Agent + Builder + Deployer on each cluster
```

---

## Project Structure

```
heroku-clone/
├── control-plane/          # Control Plane API server (Go)
│   ├── internal/
│   │   └── scheduler/      # Job dispatch — picks which runtime handles each deploy
│   └── migrations/         # SQL migrations (001–004)
├── runtime-plane/
│   ├── agent/              # Runtime Agent — registers, polls jobs, drives pipeline
│   ├── builder/            # Builder service — CNB/pack integration
│   └── deployer/           # Deployer service — kubectl apply
├── cli/                    # platform CLI (Go + Cobra)
├── infra/
│   ├── aws/                # Terraform: EKS + ECR + VPC + IAM
│   ├── gcp/                # Terraform: GKE + Artifact Registry + VPC
│   └── azure/              # Terraform: AKS + ACR + VNet
├── poc/                    # Original proof-of-concept (local dev)
├── api-design/             # API schema and DB schema docs
└── docs/                   # Architecture, implementation guide
```

---

## Getting Started — AWS (Step by Step)

This guide takes you from zero to a working platform on AWS EKS in about 30 minutes. You will end up with:

- A running Control Plane (locally or on any server)
- An EKS cluster running the Runtime Agent
- The `platform` CLI configured and working
- A deployed sample app accessible via HTTP

### Prerequisites

Install these tools before starting:

| Tool | Version | Install |
|---|---|---|
| Go | 1.21+ | `brew install go` |
| Terraform | 1.5+ | `brew install terraform` |
| AWS CLI | 2.x | `brew install awscli` |
| kubectl | 1.28+ | `brew install kubectl` |
| git | any | pre-installed on macOS |
| pack CLI | 0.33+ | `brew install buildpacks/tap/pack` |

Configure your AWS credentials:

```bash
aws configure
# Enter your AWS Access Key ID, Secret, region (e.g. us-east-1), output format (json)
```

Verify access:

```bash
aws sts get-caller-identity
# Should return your AWS account ID and IAM user/role ARN
```

---

### Step 1 — Clone the repo and build the CLI

```bash
git clone https://github.com/ajaykumar127/heroku-clone.git
cd heroku-clone

# Build the CLI
cd cli
go build -o bin/platform .

# Add to your PATH (or use ./bin/platform throughout this guide)
export PATH="$PWD/bin:$PATH"
cd ..

# Verify
platform --help
```

---

### Step 2 — Start the Control Plane locally

The Control Plane is a single Go binary backed by SQLite (for local dev) or PostgreSQL (for production).

```bash
cd control-plane

# Build
go build -o bin/control-plane .

# Start (SQLite mode — no database setup needed)
DATABASE_URL=./platform.db \
PORT=8080 \
GIT_SERVER_HOST=localhost \
GIT_SERVER_PORT=2222 \
./bin/control-plane
```

You should see:

```
Starting Control Plane API
Environment: development
Database: ./platform.db
Port: 8080
✓ Database connected
✓ Auth schema ready
✓ Config vars schema ready
API Server listening on :8080
```

Leave this running in a terminal tab. In a new tab, verify it:

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

> For production, run the Control Plane on a server with a public IP and use PostgreSQL.
> Set `DATABASE_URL=postgres://user:pass@host:5432/platform` instead of the SQLite path.

---

### Step 3 — Create your first API token

```bash
cd cli
platform auth login --api-url http://localhost:8080
```

Output:

```
Logged in. Token saved to ~/.platform/config
Token: pltf_abc123...
```

Your token is now saved. Every subsequent `platform` command reads it automatically from `~/.platform/config`.

---

### Step 4 — Provision the AWS Runtime Plane with Terraform

This creates an EKS cluster, ECR registry, VPC, and IAM roles — and deploys the Runtime Agent into the cluster automatically.

```bash
cd infra/aws

# Initialise Terraform providers
terraform init

# Preview what will be created (no changes yet)
terraform plan \
  -var="control_plane_url=http://<YOUR_MACHINE_IP>:8080" \
  -var="region=us-east-1" \
  -var="cluster_name=platform-runtime"
```

> Replace `<YOUR_MACHINE_IP>` with the IP address the EKS cluster can reach your Control Plane on.
> If running the Control Plane on the same machine, use your machine's LAN IP (not 127.0.0.1).

When the plan looks good, apply:

```bash
terraform apply \
  -var="control_plane_url=http://<YOUR_MACHINE_IP>:8080" \
  -var="region=us-east-1" \
  -var="cluster_name=platform-runtime"
```

Type `yes` when prompted. This takes 10–15 minutes. When complete you will see:

```
Apply complete! Resources: 42 added.

Outputs:
cluster_endpoint        = "https://XXXXX.gr7.us-east-1.eks.amazonaws.com"
cluster_name            = "platform-runtime"
ecr_repository_url      = "123456789.dkr.ecr.us-east-1.amazonaws.com/platform-apps"
kubeconfig_command      = "aws eks update-kubeconfig --name platform-runtime --region us-east-1"
```

Configure kubectl to talk to your new cluster:

```bash
aws eks update-kubeconfig --name platform-runtime --region us-east-1

# Verify the agent is running
kubectl get pods -n platform-system
# NAME                             READY   STATUS    RESTARTS
# runtime-agent-7d9f8b5c4-xk2p9   1/1     Running   0
```

---

### Step 5 — Verify the Runtime Plane registered

Switch back to your CLI:

```bash
platform runtime list --api-url http://localhost:8080
```

Output:

```
NAME              CLOUD   REGION       STATUS   LAST SEEN
aws-us-east-1     aws     us-east-1    active   12 seconds ago
```

The Runtime Agent running in EKS registered itself with the Control Plane on startup and is sending heartbeats every 30 seconds.

---

### Step 6 — Start the Git Server

The Git server accepts `git push` from developers and triggers builds. Run it in a new terminal tab:

```bash
cd poc/git-server
go build -o bin/git-server .
API_URL=http://localhost:8080 ./bin/git-server
```

Output:

```
Git server starting on :2222
Repos directory: ./repos
```

---

### Step 7 — Create an app

```bash
platform apps create my-first-app \
  --api-url http://localhost:8080 \
  --runtime aws-us-east-1
```

Output:

```
Created app my-first-app
  Git URL: ssh://git@localhost:2222/my-first-app.git
  Web URL: http://my-first-app.localhost

Add the remote:
  git remote add platform ssh://git@localhost:2222/my-first-app.git
Deploy:
  git push platform main
```

---

### Step 8 — Set config vars (optional)

Config vars are injected as environment variables into your app at deploy time:

```bash
platform config set my-first-app \
  NODE_ENV=production \
  DATABASE_URL=postgres://prod-host/mydb \
  --api-url http://localhost:8080

# Verify
platform config get my-first-app --api-url http://localhost:8080
# DATABASE_URL:  postgres://prod-host/mydb
# NODE_ENV:      production
```

---

### Step 9 — Deploy an app

Use the included example Node.js app, or use your own.

```bash
cd poc/example-app

# Initialise a git repo if it isn't one already
git init
git add .
git commit -m "Initial commit"

# Point it at the platform Git server
git remote add platform ssh://git@localhost:2222/my-first-app.git

# Deploy
git push platform main
```

You will see build output in your terminal:

```
-----> Receiving push for my-first-app
       abc1234 -> def5678 (refs/heads/main)
-----> Triggering build for commit def5678
-----> Build queued successfully
-----> Deploy in progress...
```

---

### Step 10 — Watch the build logs

In a new terminal, stream the build logs in real time:

```bash
# First get the release ID
platform releases list my-first-app --api-url http://localhost:8080
# VERSION  COMMIT   STATUS     CREATED
# v1       def5678  building   2026-05-19 18:00:00

# Stream logs (replace <release-id> with the ID from above)
platform logs my-first-app <release-id> \
  --tail \
  --api-url http://localhost:8080
```

Output while building:

```
=====> Building application: my-first-app
       Commit: def5678
-----> Cloning repository
-----> Detecting application type
       Detected: Node.js
-----> Building with buildpacks
...
=====> Build complete!
       Image: 123456789.dkr.ecr.us-east-1.amazonaws.com/platform-apps/my-first-app:def5678

=====> Deploying to Kubernetes (aws-us-east-1)
-----> Applying manifests
-----> Waiting for rollout...
deployment "my-first-app" successfully rolled out

==> Build succeeded
```

---

### Step 11 — Access your app

Your app is now running on EKS and accessible via the Nginx Ingress. By default it is available at:

```
http://my-first-app.localhost
```

For a production domain, update your DNS to point `*.yourdomain.com` at the Nginx Ingress load balancer IP, then set:

```bash
terraform apply \
  -var="apps_domain=yourdomain.com" \
  ...
```

---

### Step 12 — Scale, update, repeat

```bash
# Push a code change — a new release is created automatically
git commit -am "Fix bug" && git push platform main

# Check release history
platform releases list my-first-app --api-url http://localhost:8080
# VERSION  COMMIT   STATUS      CREATED
# v2       a1b2c3   succeeded   2026-05-19 18:05:00
# v1       def5678  succeeded   2026-05-19 18:00:00

# Update a config var (triggers redeploy)
platform config set my-first-app LOG_LEVEL=debug --api-url http://localhost:8080
```

---

## CLI Reference

```bash
# Authentication
platform auth login                        # Create a token and save it locally
platform auth tokens                       # List all tokens

# Apps
platform apps list                         # List all apps
platform apps create <name>                # Create a new app
platform apps create <name> --runtime aws-us-east-1  # Create on a specific runtime
platform apps destroy <name> --confirm     # Delete an app

# Deployments & releases
platform releases list <app>               # List all releases
platform logs <app> <release-id>           # View build logs (snapshot)
platform logs <app> <release-id> --tail    # Stream build logs live

# Config vars
platform config get <app>                  # Show all config vars
platform config set <app> KEY=VALUE ...    # Set one or more vars
platform config unset <app> KEY ...        # Remove vars

# Runtime planes
platform runtime list                      # List registered runtime planes
platform runtime register                  # Manually register a runtime
  --name aws-us-east-1
  --cloud aws
  --region us-east-1
  --agent-url http://agent-host:9090
```

Global flags available on every command:

```bash
--api-url  http://localhost:8080   # Control Plane URL
--token    pltf_abc123...          # API token (or set PLATFORM_TOKEN env var)
```

---

## Adding More Clouds

Once your Control Plane is running, adding a second cloud is just a `terraform apply`:

### GCP

```bash
cd infra/gcp
terraform init
terraform apply \
  -var="project_id=my-gcp-project" \
  -var="control_plane_url=http://<YOUR_CONTROL_PLANE>:8080" \
  -var="region=us-central1"

platform runtime list
# NAME               CLOUD   REGION         STATUS
# aws-us-east-1      aws     us-east-1      active
# gcp-us-central1    gcp     us-central1    active   ← new
```

### Azure

```bash
cd infra/azure
terraform init
terraform apply \
  -var="control_plane_url=http://<YOUR_CONTROL_PLANE>:8080" \
  -var="location=East US"

platform runtime list
# NAME               CLOUD   REGION         STATUS
# aws-us-east-1      aws     us-east-1      active
# gcp-us-central1    gcp     us-central1    active
# azure-eastus       azure   eastus         active   ← new
```

Now you can deploy to any cloud:

```bash
platform apps create my-app --runtime gcp-us-central1
platform apps create another-app --runtime azure-eastus
```

---

## Running the Control Plane in Production

For production, run the Control Plane on a server (EC2, GCE, or any VPS) with PostgreSQL:

```bash
# On your server
export DATABASE_URL=postgres://platform:secret@localhost:5432/platform
export PORT=8080
export ENVIRONMENT=production
export GIT_SERVER_HOST=git.yourdomain.com
export GIT_SERVER_PORT=2222

./control-plane
```

Apply the DB migrations first:

```bash
psql $DATABASE_URL -f control-plane/migrations/001_initial_schema.sql
psql $DATABASE_URL -f control-plane/migrations/002_runtimes.sql
psql $DATABASE_URL -f control-plane/migrations/003_jobs.sql
psql $DATABASE_URL -f control-plane/migrations/004_app_runtime_fk.sql
```

Then point your Terraform at the public URL:

```bash
terraform apply -var="control_plane_url=https://platform.yourdomain.com" ...
```

---

## Troubleshooting

**Runtime not showing up after `terraform apply`**

Check the agent logs in the cluster:
```bash
kubectl logs -n platform-system -l app=runtime-agent --tail=50
```
Common cause: the Control Plane URL is not reachable from inside the cluster. Make sure you used your machine's public/LAN IP, not `localhost`.

**Build stuck in `building` status**

Check the agent is polling:
```bash
kubectl logs -n platform-system -l app=runtime-agent -f
```
If the agent shows errors, the Control Plane may have restarted and the agent needs to re-register. Restart the agent pod:
```bash
kubectl rollout restart deployment/runtime-agent -n platform-system
```

**`pack` CLI not found during build**

The builder falls back to `builder.sh` automatically. To get full CNB support, ensure `pack` is installed in the runtime agent image or set `BUILDER_SCRIPT` to point to a working script.

**`git push` rejected with permission denied**

The Git server accepts all SSH keys for the POC. If you see permission errors, ensure the SSH key is being sent:
```bash
ssh -p 2222 -v git@localhost
```

---

## Technology Stack

| Layer | Technology |
|---|---|
| Control Plane | Go — `net/http`, `gorilla/mux`, SQLite / PostgreSQL |
| Runtime Agent | Go — stdlib only, polls over HTTP |
| Builder | Go + Cloud Native Buildpacks (`pack` CLI) |
| Deployer | Go + `kubectl` |
| Orchestration | Kubernetes 1.28+ |
| Container Registry | ECR (AWS) / Artifact Registry (GCP) / ACR (Azure) |
| Ingress | Nginx Ingress Controller |
| Infrastructure | Terraform 1.5+ |
| CLI | Go + Cobra |

---

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — Control Plane / Runtime Plane deep dive
- [API Schema](api-design/API_SCHEMA.md) — Full REST API specification
- [Database Schema](api-design/DATABASE_SCHEMA.sql) — PostgreSQL schema
- [Implementation Guide](docs/IMPLEMENTATION_GUIDE.md) — How each component is built
- [Infra README](infra/README.md) — Terraform usage for all three clouds
- [POC README](poc/README.md) — Local proof-of-concept setup

---

## License

Reference design and educational resource. Adapt freely.
