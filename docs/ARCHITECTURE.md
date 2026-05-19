# Cloud-Agnostic PaaS Architecture

## Overview

This platform is a Heroku-style PaaS that runs on any public cloud or on-premises infrastructure. Unlike Heroku, which is locked to a single cloud, this platform separates the **Control Plane** (runs once, anywhere — manages apps, auth, and job dispatch) from **Runtime Planes** (one per cloud/region — handle builds, deployments, and traffic). Developers interact with one API and one CLI regardless of which cloud their app ultimately runs on. The Control Plane schedules work to whichever Runtime Plane is best suited for each app, and Runtime Agents on each cluster poll for jobs independently, so no direct inbound connectivity to the Control Plane is required from the cloud networks.

## Architecture Diagram (ASCII)

```
Developer Tools (CLI / Git Push / Dashboard)
            │
    ┌───────▼────────┐
    │  Control Plane │  ← runs once, anywhere
    │  ─────────────│
    │  API Server    │
    │  Job Queue     │
    │  App Registry  │
    │  Auth          │
    └───────┬────────┘
            │  HTTP (job polling)
    ┌───────┴──────────────────────────────┐
    │              │                       │
┌───▼────┐  ┌─────▼─────┐  ┌─────────────▼──┐
│AWS EKS │  │ GCP GKE   │  │  Azure AKS     │
│+ ECR   │  │ + GAR     │  │  + ACR         │
│Runtime │  │ Runtime   │  │  Runtime Agent │
│ Agent  │  │  Agent    │  │                │
└────────┘  └───────────┘  └────────────────┘
```

## Components

### Control Plane

- **API Server** (Go/Gin): All REST endpoints, auth, app/release management. Exposes `GET /v1/apps`, `POST /v1/apps`, `GET /v1/runtimes`, `POST /internal/runtime/register`, and all other platform APIs.
- **Job Queue**: DB-backed queue (Postgres), polled by Runtime Agents every 5 seconds. Jobs include `build`, `deploy`, and `release` tasks.
- **Runtime Registry**: Tracks all registered Runtime Planes including cloud provider, region, health status, and last-seen timestamp. Supports manual registration via `POST /internal/runtime/register` and automatic registration when an agent starts up.
- **Scheduler**: Selects which Runtime Plane handles each deploy using a least-recently-used (LRU) policy, with per-app affinity so repeat deploys land on the same runtime unless overridden by `--runtime`.

### Runtime Plane

- **Runtime Agent** (Go): Registers with the Control Plane on startup, polls the Job Queue for pending work, drives the build and deploy pipeline, and reports results back via the Control Plane API.
- **Builder** (Go + CNB): Clones the application repo, runs Cloud Native Buildpacks (Paketo), and pushes the resulting OCI image to the cloud-specific container registry.
- **Deployer** (Go + kubectl): Generates and applies Kubernetes manifests (Deployment, Service, Ingress) to the local cluster. Handles rolling updates and reports status back to the Control Plane.
- **Container Registry**: ECR (AWS) / Artifact Registry (GCP) / ACR (Azure). Each Runtime Plane uses its cloud's native registry; the agent is pre-authorized at startup via cloud IAM.
- **Nginx Ingress**: Routes HTTP/HTTPS traffic to application pods within the cluster. SSL certificates are managed by cert-manager and Let's Encrypt.

## Communication Flow

1. Developer runs `git push platform main`. The Git server authenticates the push and enqueues a `build` job in the Control Plane Job Queue.
2. The Control Plane Scheduler selects a target Runtime Plane (using per-app preference or LRU fallback) and tags the job with the runtime ID.
3. The Runtime Agent on the selected cluster polls `GET /internal/jobs/next` and picks up the build job.
4. The Agent's Builder clones the source, runs CNB buildpacks, produces an OCI image, and pushes it to the cluster's container registry.
5. The Agent marks the build complete and the Control Plane creates a new Release record (image digest + config vars = release vN).
6. A `deploy` job is enqueued and immediately picked up by the same Runtime Agent.
7. The Agent's Deployer generates a Kubernetes Deployment manifest pointing to the new image, applies it via kubectl, and waits for the rolling update to complete.
8. Once all pods pass health checks, the Nginx Ingress routes traffic to the new pods and old pods are drained.
9. The Agent reports success to the Control Plane. The release is marked active and build logs are stored.
10. The developer sees a success message in their terminal and the app is live at `https://<app-name>.platform.example.com`.

## Directory Structure

```
heroku-clone/
├── api-design/
│   ├── API_SCHEMA.md          # Full REST API specification (OpenAPI-compatible)
│   └── DATABASE_SCHEMA.sql    # PostgreSQL schema for Control Plane metadata
├── cli/
│   ├── cmd/
│   │   ├── root.go            # CLI root command, global flags, config loading
│   │   ├── apps.go            # platform apps list/create/destroy
│   │   ├── auth.go            # platform login/logout/whoami
│   │   ├── client.go          # HTTP API client used by all commands
│   │   ├── config.go          # platform config set/get/unset
│   │   ├── releases.go        # platform releases list/rollback
│   │   └── runtime.go         # platform runtime list/register/deregister
│   ├── main.go                # CLI entry point
│   ├── go.mod
│   └── go.sum
├── docs/
│   ├── ARCHITECTURE.md        # This document
│   ├── IMPLEMENTATION_GUIDE.md
│   ├── OPEN_SOURCE_ANALYSIS.md
│   └── QUICK_REFERENCE.md
├── infra/
│   ├── aws/                   # Terraform for AWS (EKS + ECR + VPC)
│   ├── gcp/                   # Terraform for GCP (GKE + Artifact Registry + VPC)
│   ├── azure/                 # Terraform for Azure (AKS + ACR + VNet)
│   └── README.md              # Infra provisioning guide
├── poc/
│   ├── api-server/            # Proof-of-concept Go API server
│   ├── git-server/            # Proof-of-concept Go Git server
│   ├── builder/               # Buildpack runner shell scripts
│   ├── deployer/              # Kubernetes deployment shell scripts
│   ├── example-app/           # Sample Node.js application
│   └── scripts/               # Deployment helper scripts
└── README.md                  # Project overview and quick start
```

## Cloud Support

| Feature            | AWS                          | GCP                          | Azure                        |
|--------------------|------------------------------|------------------------------|------------------------------|
| Kubernetes         | EKS (Elastic Kubernetes)     | GKE (Google Kubernetes)      | AKS (Azure Kubernetes)       |
| Container Registry | ECR (Elastic Container Reg.) | Artifact Registry (GAR)      | ACR (Azure Container Reg.)   |
| IAM / Auth         | IAM Roles for Service Accts  | Workload Identity            | Azure Managed Identity       |
| Infra-as-Code      | Terraform aws provider       | Terraform google provider    | Terraform azurerm provider   |
| Load Balancer      | AWS Load Balancer Controller | GCP L7 Load Balancer         | Azure Application Gateway    |
| Object Storage     | S3                           | Cloud Storage (GCS)          | Azure Blob Storage           |

## Developer Experience

### Quick Start

```bash
# 1. Install the CLI (build from source)
cd cli && go build -o platform . && mv platform /usr/local/bin/

# 2. Authenticate against the Control Plane
platform login --api-url https://api.platform.example.com

# 3. Create an app (optionally pin it to a specific runtime)
platform apps create my-app --runtime aws-us-east-1

# 4. Add the git remote and deploy
git remote add platform $(platform apps list | grep my-app | awk '{print $2}')
git push platform main

# 5. Check logs
platform logs --tail -a my-app

# 6. Scale up
platform ps:scale web=3 -a my-app

# 7. Roll back if something goes wrong
platform releases -a my-app
platform rollback v4 -a my-app
```

---

<!-- The original detailed architecture reference is preserved below. -->

## Original Architecture Reference

## Design Principles

1. **Cloud Agnostic**: No vendor lock-in, portable across providers
2. **Developer Experience First**: Simple git-push deployment workflow
3. **Stateless Applications**: Embrace 12-factor app methodology
4. **Immutable Infrastructure**: Reproducible builds and deployments
5. **API-Driven**: Everything accessible via API and CLI
6. **Multi-Tenant**: Secure isolation between customer applications
7. **Horizontally Scalable**: Handle thousands of applications

## System Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Interface Layer                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │   CLI    │  │ Dashboard│  │  API     │  │  GitHub  │        │
│  │  Client  │  │   Web    │  │  Tokens  │  │  Webhook │        │
│  └─────┬────┘  └─────┬────┘  └─────┬────┘  └─────┬────┘        │
└────────┼─────────────┼─────────────┼─────────────┼──────────────┘
         │             │             │             │
         └─────────────┴─────────────┴─────────────┘
                           │
┌──────────────────────────▼───────────────────────────────────────┐
│                   Control Plane (API Server)                     │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐│
│  │    Auth    │  │    Apps    │  │  Releases  │  │   Config   ││
│  │  Service   │  │  Manager   │  │  Manager   │  │  Manager   ││
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘│
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐│
│  │   Billing  │  │  Add-ons   │  │   Domains  │  │    SSL     ││
│  │  Service   │  │  Manager   │  │  Manager   │  │  Manager   ││
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘│
└──────────────────────────┬───────────────────────────────────────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
         ▼                 ▼                 ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│  Build System   │ │   Scheduler     │ │  Router/Ingress │
│                 │ │   (K8s API)     │ │    Controller   │
│ ┌─────────────┐ │ │                 │ │                 │
│ │ Git Server  │ │ │  ┌───────────┐  │ │ ┌────────────┐  │
│ │ (Receive    │ │ │  │ Scaling   │  │ │ │  Nginx/    │  │
│ │  Hooks)     │ │ │  │ Engine    │  │ │ │  Envoy     │  │
│ └──────┬──────┘ │ │  └───────────┘  │ │ └────────────┘  │
│        │        │ │                 │ │                 │
│        ▼        │ │  ┌───────────┐  │ │ ┌────────────┐  │
│ ┌─────────────┐ │ │  │ Health    │  │ │ │   SSL      │  │
│ │ Buildpack   │ │ │  │ Monitor   │  │ │ │   Terminator│ │
│ │ Runner      │ │ │  └───────────┘  │ │ └────────────┘  │
│ └──────┬──────┘ │ └─────────────────┘ └─────────────────┘
│        │        │
│        ▼        │
│ ┌─────────────┐ │
│ │   Slug      │ │
│ │ Compiler    │ │
│ └──────┬──────┘ │
└────────┼────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│              Data Plane (Kubernetes Clusters)                    │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐     │
│  │                Application Namespaces                   │     │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐             │     │
│  │  │ App Pod  │  │ App Pod  │  │ App Pod  │  ...        │     │
│  │  │ (Dyno)   │  │ (Dyno)   │  │ (Dyno)   │             │     │
│  │  └──────────┘  └──────────┘  └──────────┘             │     │
│  └────────────────────────────────────────────────────────┘     │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐     │
│  │              Platform Services Namespace                │     │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐             │     │
│  │  │ Logging  │  │ Metrics  │  │ Service  │             │     │
│  │  │ Agents   │  │ Scraper  │  │  Mesh    │             │     │
│  │  └──────────┘  └──────────┘  └──────────┘             │     │
│  └────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
         │                      │                     │
         ▼                      ▼                     ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│  Object Storage  │  │  Log Aggregator  │  │  Metrics Store   │
│  (Slugs/Assets)  │  │  (Loki/ES)       │  │  (Prometheus)    │
└──────────────────┘  └──────────────────┘  └──────────────────┘
```

## Core Components

### 1. Control Plane

The control plane is the brain of the platform, managing all operations via a RESTful API.

#### 1.1 API Server
- **Technology**: Go with Gin/Echo framework or Rust with Actix
- **Database**: PostgreSQL for metadata (apps, releases, config, users)
- **Authentication**: OAuth2 + JWT tokens + API keys
- **Features**:
  - Application CRUD operations
  - Release management
  - Configuration management
  - Scaling operations
  - Log streaming endpoints
  - Webhook management

#### 1.2 Authentication Service
- User management (email/password, OAuth providers)
- Team/organization management
- Role-Based Access Control (RBAC)
- API token generation and rotation
- SSH key management for git operations

#### 1.3 Apps Manager
- Application lifecycle management
- Formation configuration (process types and quantities)
- Environment variable management
- Domain mapping
- Maintenance mode

#### 1.4 Releases Manager
- Version tracking (v1, v2, v3...)
- Slug + config hash = release
- Rollback capability
- Release phase execution (pre-deployment tasks)
- Audit logging

#### 1.5 Add-ons Manager
- Service provisioning API
- Integration with external providers
- Config var injection
- Billing integration
- Webhook notifications

### 2. Build System

Transforms source code into executable container images (slugs).

#### 2.1 Git Server
- **Technology**: Custom SSH server with Git hooks or Gitea
- **Flow**:
  1. Developer: `git push platform main`
  2. Git receive hook authenticates user
  3. Trigger build job
  4. Return build logs to git client

#### 2.2 Buildpack Runner
- **Technology**: Cloud Native Buildpacks (Paketo, Heroku CNBs)
- **Process**:
  1. Detect: Identify application type
  2. Build: Install dependencies, compile code
  3. Export: Create OCI image (slug)
  4. Store in registry
- **Caching**: Layer caching for fast rebuilds
- **Isolation**: Build in ephemeral containers

#### 2.3 Slug Compiler
- Packages application code + dependencies
- Compression and optimization
- Size limits (500MB default)
- Stored in object storage with versioning

#### 2.4 Build Queue
- **Technology**: Redis/NATS for job queue
- **Features**:
  - Priority queuing
  - Concurrent builds
  - Build timeout handling
  - Retry logic for failures

### 3. Scheduler (Orchestration Layer)

Manages where and how applications run.

#### 3.1 Kubernetes Integration
- **Deployment Strategy**: One K8s Deployment per process type
- **Namespace Strategy**: One namespace per application or customer
- **Resource Limits**: CPU/Memory requests and limits per dyno size
- **Pod Spec Generation**: Convert platform abstractions to K8s resources

**Dyno Size Mapping**:
```yaml
standard-1x:
  cpu: 0.5 cores
  memory: 512Mi
  
standard-2x:
  cpu: 1 core
  memory: 1Gi
  
performance-m:
  cpu: 2.5 cores
  memory: 2.5Gi
```

#### 3.2 Scaling Engine
- **Horizontal**: Adjust replica count
- **Vertical**: Change dyno size (requires restart)
- **Auto-scaling**: CPU/memory-based HPA
- **Manual scaling**: Via CLI/API

#### 3.3 Health Monitoring
- HTTP health checks
- Process crash detection
- Auto-restart failed containers
- Boot timeout enforcement (60s default)

### 4. Router (Ingress System)

Routes HTTP/HTTPS traffic to application containers.

#### 4.1 Ingress Controller
- **Technology**: Nginx Ingress Controller or Envoy Gateway
- **Features**:
  - Host-based routing (app-name.platform.com)
  - Custom domain support
  - WebSocket support
  - HTTP/2 and gRPC
  - Request queuing with backpressure

#### 4.2 SSL/TLS Management
- **Technology**: cert-manager with Let's Encrypt
- **Features**:
  - Automatic certificate provisioning
  - Auto-renewal
  - SNI support for custom domains
  - HSTS headers

#### 4.3 Load Balancing
- Round-robin across healthy pods
- Connection draining during deploys
- Request timeout (30s default)
- Rate limiting per app

### 5. Logging System

Aggregates and streams logs from all platform components.

#### 5.1 Log Collection
- **Technology**: Fluent Bit daemonset on K8s nodes
- **Sources**:
  - Application stdout/stderr
  - Router access logs
  - System events
  - Build logs

#### 5.2 Log Routing
- **Technology**: Loki or Elasticsearch
- **Features**:
  - Real-time streaming (`heroku logs --tail`)
  - Historical search (24h retention default)
  - Log drains to external services (Splunk, Datadog, etc.)
  - Structured logging support

#### 5.3 Log Drains
- HTTP/HTTPS endpoints
- Syslog protocol
- Kafka integration
- Retry logic with backoff

### 6. Metrics & Monitoring

Application and platform observability.

#### 6.1 Metrics Collection
- **Technology**: Prometheus with Grafana
- **Metrics**:
  - Application: Request rate, latency, error rate
  - Platform: Dyno CPU/memory, build times, API latency
  - Router: Request volume, status codes

#### 6.2 Alerting
- Critical system alerts
- Application health alerts
- Custom metric alerting
- PagerDuty/OpsGenie integration

### 7. One-off Dynos & Scheduler

Run commands and scheduled tasks.

#### 7.1 Run Command Execution
- **Implementation**: Kubernetes Jobs
- **Features**:
  - Same slug as web dynos
  - Same config vars
  - Interactive terminal attachment
  - Time limits (1 hour default)
  - Example: `platform run bash`

#### 7.2 Scheduled Jobs
- **Technology**: Kubernetes CronJobs
- **Features**:
  - Cron expression syntax
  - Timezone support
  - Failure notifications
  - Concurrency policies

### 8. Data Storage Layer

#### 8.1 Platform Metadata Database
- **Technology**: PostgreSQL (or CockroachDB for multi-region)
- **Schema**:
  - Users, teams, organizations
  - Applications, releases, formations
  - Config vars, domains, SSL certificates
  - Add-ons, billing records

#### 8.2 Object Storage
- **Technology**: S3-compatible (AWS S3, GCS, Azure Blob, MinIO)
- **Contents**:
  - Build slugs
  - Build cache
  - Application assets
  - Log archives

#### 8.3 Container Registry
- **Technology**: Harbor, Docker Registry, or cloud-native
- **Usage**: Store OCI images for dynos

### 9. CLI Tool

Developer's primary interface.

#### 9.1 Core Commands
```bash
platform login
platform create my-app
platform git:remote -a my-app
platform config:set KEY=value
platform ps:scale web=3 worker=2
platform logs --tail
platform run bash
platform releases
platform rollback v42
platform domains:add example.com
platform addons:create redis:premium
```

#### 9.2 Implementation
- **Language**: Go (cross-platform single binary)
- **Features**:
  - API client with retries
  - Real-time log streaming (WebSocket)
  - Interactive shell for `run`
  - Plugin system for extensibility
  - Auto-updates

## Cloud Abstraction Strategy

### Kubernetes as Foundation
Use Kubernetes as the abstraction layer. All major clouds offer managed K8s:
- **AWS**: EKS (Elastic Kubernetes Service)
- **GCP**: GKE (Google Kubernetes Engine)
- **Azure**: AKS (Azure Kubernetes Service)
- **On-prem**: Kubeadm, Rancher, OpenShift

### Cloud-Specific Resource Mapping

| Feature | AWS | GCP | Azure | Abstraction |
|---------|-----|-----|-------|-------------|
| Load Balancer | ELB/ALB | Cloud Load Balancing | Azure Load Balancer | K8s LoadBalancer Service |
| Object Storage | S3 | Cloud Storage | Blob Storage | S3-compatible API |
| Container Registry | ECR | GCR | ACR | Harbor or cloud-native |
| DNS | Route53 | Cloud DNS | Azure DNS | ExternalDNS controller |
| Secrets | Secrets Manager | Secret Manager | Key Vault | K8s Secrets + External Secrets Operator |
| Monitoring | CloudWatch | Cloud Monitoring | Azure Monitor | Prometheus/Grafana |

### Infrastructure as Code
- **Tool**: Terraform with provider modules
- **Structure**:
  ```
  terraform/
  ├── modules/
  │   ├── kubernetes/      # K8s cluster setup
  │   ├── networking/      # VPC, subnets, firewalls
  │   ├── storage/         # Object storage buckets
  │   └── database/        # Managed PostgreSQL
  ├── aws/                 # AWS-specific config
  ├── gcp/                 # GCP-specific config
  └── azure/               # Azure-specific config
  ```

## Security Architecture

### 1. Multi-Tenancy Isolation
- **Namespace Isolation**: One K8s namespace per app or customer
- **Network Policies**: Restrict inter-app communication
- **Resource Quotas**: Prevent resource exhaustion
- **Pod Security Standards**: Enforce security baselines

### 2. Secret Management
- Config vars encrypted at rest
- Secrets injected as environment variables
- External Secrets Operator for cloud secret stores
- Automatic secret rotation

### 3. Authentication & Authorization
- JWT tokens for API access
- API keys for CI/CD integration
- RBAC for team permissions
- Audit logging for all actions

### 4. Network Security
- TLS everywhere (API, internal services, data plane)
- Private container registries
- Egress filtering
- DDoS protection at ingress

### 5. Container Security
- Base image scanning (Trivy, Clair)
- Runtime security (Falco)
- Immutable containers
- Non-root user execution

## Data Flow Examples

### Example 1: Application Deployment

```
1. Developer: git push platform main
2. Git server authenticates, creates build job
3. Build worker pulls source code
4. Buildpack runner:
   - Detects Node.js application
   - Runs npm install
   - Creates slug (app code + node_modules)
5. Slug stored in S3 with version hash
6. Release created (slug + config vars = v23)
7. Scheduler:
   - Generates K8s Deployment manifest
   - Points to slug image in registry
   - Sets replicas to formation count
   - Applies to cluster
8. K8s creates pods with new release
9. Pods start, pass health checks
10. Router updates routing table
11. Old pods drained and terminated
12. Deployment complete, logs streamed to developer
```

### Example 2: Auto-scaling Event

```
1. Application receives traffic spike
2. Metrics show CPU > 80% for 2 minutes
3. Horizontal Pod Autoscaler (HPA) triggers
4. HPA increases replica count: 2 → 5
5. K8s scheduler places new pods
6. New pods pull slug from registry
7. Pods start with same config vars
8. Health checks pass
9. Router adds new pods to load balancer
10. Traffic distributed across 5 pods
11. CPU drops below threshold
12. After cooldown, HPA scales down: 5 → 2
```

### Example 3: Log Streaming

```
1. Developer: platform logs --tail -a my-app
2. CLI connects to API via WebSocket
3. API authenticates request
4. API queries log aggregator for recent logs
5. API subscribes to real-time log stream
6. Application pod logs to stdout
7. Fluent Bit on node collects logs
8. Logs forwarded to Loki/ES
9. Log aggregator pushes to API WebSocket
10. API forwards to CLI WebSocket
11. CLI displays logs in terminal
```

## Scalability Considerations

### Horizontal Scaling
- **Control Plane**: Stateless API servers behind load balancer
- **Build System**: Multiple build workers, queue-based
- **Data Plane**: Thousands of K8s nodes
- **Router**: Multiple ingress replicas with shared config

### Performance Targets
- **API Latency**: p95 < 200ms
- **Build Time**: < 2 minutes for typical app
- **Deploy Time**: < 30 seconds from release to live
- **Log Delivery**: < 1 second lag
- **Router Latency**: < 10ms overhead

### Capacity Planning
- **Applications**: 10,000+ apps per cluster
- **Requests**: 100,000+ req/sec aggregate
- **Builds**: 500+ concurrent builds
- **Logs**: 1TB+ per day

## High Availability & Disaster Recovery

### High Availability
- **Control Plane**: Multi-AZ deployment, 3+ replicas
- **Data Plane**: Multi-AZ K8s nodes
- **Database**: PostgreSQL with streaming replication
- **Object Storage**: S3 with versioning enabled
- **RPO**: < 1 minute (Release Point Objective)
- **RTO**: < 5 minutes (Recovery Time Objective)

### Backup Strategy
- **Database**: Continuous WAL archiving + daily snapshots
- **Object Storage**: Cross-region replication
- **Configuration**: GitOps with infrastructure as code

### Disaster Recovery
- Multi-region deployment capability
- Automated failover procedures
- Regular DR drills
- Documented runbooks

## Technology Stack Summary

| Component | Technology Choices |
|-----------|-------------------|
| **Orchestration** | Kubernetes 1.28+ |
| **Control Plane API** | Go (Gin/Echo) or Rust (Actix) |
| **Database** | PostgreSQL 15+ or CockroachDB |
| **Build System** | Cloud Native Buildpacks (Paketo) |
| **Container Runtime** | containerd via K8s |
| **Ingress** | Nginx Ingress Controller or Envoy Gateway |
| **Service Mesh** | Istio or Linkerd (optional) |
| **Object Storage** | S3-compatible (MinIO/S3/GCS/Azure Blob) |
| **Container Registry** | Harbor or cloud-native |
| **Logging** | Fluent Bit → Loki or Elasticsearch |
| **Metrics** | Prometheus + Grafana |
| **Message Queue** | NATS or RabbitMQ |
| **Cache** | Redis |
| **SSL Management** | cert-manager + Let's Encrypt |
| **CLI** | Go with Cobra framework |
| **IaC** | Terraform + Helm |

## Development Roadmap

### Phase 1: Foundation (Months 1-3)
- [ ] K8s cluster setup on target clouds
- [ ] Control plane API (basic CRUD)
- [ ] PostgreSQL schema design
- [ ] Git server with receive hooks
- [ ] Simple buildpack runner (Node.js only)
- [ ] Basic CLI (login, create, deploy)

### Phase 2: Core Features (Months 4-6)
- [ ] Release management & rollback
- [ ] Config var management
- [ ] Nginx ingress with SSL
- [ ] Log aggregation (Loki)
- [ ] Multi-language buildpacks (Python, Ruby, Go, Java)
- [ ] Scaling operations (manual)
- [ ] One-off dynos (`run` command)

### Phase 3: Production Readiness (Months 7-12)
- [ ] Metrics & monitoring
- [ ] Auto-scaling
- [ ] Custom domains
- [ ] Add-ons framework
- [ ] Log drains
- [ ] Scheduled jobs (cron)
- [ ] Teams & permissions
- [ ] Audit logging
- [ ] Documentation site

### Phase 4: Advanced Features (Months 13-18)
- [ ] Review apps (PR environments)
- [ ] Pipelines (promote between environments)
- [ ] Private spaces (network isolation)
- [ ] Multi-region support
- [ ] Advanced metrics & APM
- [ ] CI/CD integrations
- [ ] Marketplace for add-ons

### Phase 5: Enterprise (Months 19-24)
- [ ] SSO integration
- [ ] Compliance features (audit logs, encryption)
- [ ] Advanced RBAC
- [ ] SLA guarantees
- [ ] Dedicated infrastructure options
- [ ] White-label capability

## Team Structure

### Minimum Viable Team
- **2x Platform Engineers**: K8s, infrastructure, networking
- **2x Backend Engineers**: Control plane API, services
- **1x Frontend Engineer**: Web dashboard
- **1x DevOps/SRE**: CI/CD, monitoring, operations
- **1x Product Manager**: Requirements, prioritization
- **1x Technical Writer**: Documentation

### Full Team (Scale Phase)
- Add: Security engineer, QA engineer, support engineers

## Cost Estimates

### Infrastructure (AWS example, monthly)
- **EKS Cluster**: $150 (control plane)
- **EC2 Nodes** (20x m5.2xlarge): $5,000
- **RDS PostgreSQL** (db.r5.xlarge): $600
- **S3 Storage** (10TB): $230
- **Load Balancers**: $50
- **Data Transfer**: $500
- **Total**: ~$6,500/month for platform infrastructure

**Customer Application Resources**: Additional, varies by usage

### Development Costs (18 months to production)
- **Team**: 7 people × $150k avg × 1.5 years = $1.575M
- **Infrastructure**: $6.5k/month × 18 = $117k
- **Tools & Services**: $50k
- **Total**: ~$1.75M

## Conclusion

Building a Heroku-like PaaS is a significant undertaking but achievable with modern cloud-native technologies. By leveraging Kubernetes as the foundation and Cloud Native Buildpacks for builds, you can create a portable platform that works across any cloud provider.

The key differentiators will be:
1. **Developer Experience**: Simplicity of the git-push workflow
2. **Reliability**: Robust orchestration and monitoring
3. **Portability**: True cloud-agnostic operation
4. **Ecosystem**: Add-ons and integrations

Success depends on disciplined execution across infrastructure, developer tools, and operational excellence.
