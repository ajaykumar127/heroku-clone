# Open Source PaaS Platform Analysis

This document analyzes existing open source PaaS platforms that could be forked, extended, or used as reference implementations.

## Summary Comparison

| Platform | Status | Language | Orchestration | Complexity | Recommendation |
|----------|--------|----------|---------------|------------|----------------|
| **Dokku** | Active | Bash/Go | Docker | Low | ⭐⭐⭐⭐⭐ Best for learning |
| **CapRover** | Active | Node.js | Docker Swarm | Low | ⭐⭐⭐⭐ Good for small scale |
| **Coolify** | Active | PHP/Laravel | Docker | Medium | ⭐⭐⭐ Modern alternative |
| **Flynn** | Archived | Go | Custom | High | ⭐⭐ Reference only |
| **Deis Workflow** | Archived | Go | Kubernetes | High | ⭐⭐⭐⭐ Excellent reference |
| **Cloud Foundry** | Active | Go/Ruby | Custom | Very High | ⭐⭐ Enterprise, complex |
| **Tsuru** | Active | Go | K8s/Docker | Medium | ⭐⭐⭐ Niche, good ideas |
| **Convox** | Active | Go | K8s/ECS | Medium | ⭐⭐⭐⭐ Commercial-friendly |

---

## 1. Dokku

**Status**: ⭐ Active (10+ years, 27k+ stars)  
**Repository**: https://github.com/dokku/dokku  
**License**: MIT

### Overview
Dokku is a Docker-powered mini-Heroku. It's the simplest and most battle-tested single-server PaaS.

### Architecture
- **Single-server**: Designed for one machine
- **Orchestration**: Docker directly (not Swarm/K8s)
- **Buildpacks**: Heroku buildpacks via herokuish or Cloud Native Buildpacks
- **Routing**: Nginx with auto-SSL via Let's Encrypt
- **Storage**: Docker volumes
- **Database**: Plugin system for PostgreSQL, MySQL, Redis, etc.

### Pros
✅ Extremely simple to understand  
✅ Well-documented and mature  
✅ Active community and plugin ecosystem  
✅ Perfect for learning how PaaS works  
✅ Git push deployment works flawlessly  
✅ Low resource requirements  

### Cons
❌ Single-server only (not distributed)  
❌ Bash scripts (harder to extend than Go/Rust)  
❌ Limited multi-tenancy  
❌ No built-in HA or auto-scaling  
❌ Not cloud-agnostic (runs on one server)  

### Code Example: How Dokku Works
```bash
# Git receive hook triggers
/var/lib/dokku/plugins/git/pre-receive-hook
  ↓
# Extract app name from git remote
APP=myapp
  ↓
# Run buildpack build
dokku git-build $APP
  ↓
# Create Docker image
docker build -t dokku/$APP:latest
  ↓
# Deploy (create/update container)
dokku deploy $APP
  ↓
# Update Nginx config
dokku nginx:build-config $APP
  ↓
# Reload Nginx
sudo systemctl reload nginx
```

### Key Files to Study
- `plugins/git/commands` - Git push handling
- `plugins/nginx/` - Routing and SSL
- `plugins/config/` - Environment variable management
- `plugins/ps/` - Process/container management

### Recommendation
**⭐⭐⭐⭐⭐ Highly recommend** studying Dokku to understand PaaS fundamentals. Consider forking if you only need single-server deployment. For cloud-agnostic multi-node, use as reference only.

---

## 2. CapRover

**Status**: ⭐ Active (5+ years, 13k+ stars)  
**Repository**: https://github.com/caprover/caprover  
**License**: Apache 2.0

### Overview
CapRover is a modern, easy-to-use PaaS with a nice web UI. Built on Docker Swarm.

### Architecture
- **Orchestration**: Docker Swarm (multi-node)
- **Buildpacks**: Dockerfile or Heroku buildpacks
- **Routing**: Nginx with Let's Encrypt
- **Storage**: Docker volumes or NFS
- **Web UI**: React dashboard
- **Database**: SQLite for metadata (!)

### Pros
✅ Beautiful web UI  
✅ One-click app templates (WordPress, Ghost, etc.)  
✅ Supports Docker Swarm clustering  
✅ Easy installation  
✅ Good documentation  
✅ Active maintainer  

### Cons
❌ Docker Swarm (declining ecosystem vs K8s)  
❌ SQLite doesn't scale for large multi-tenant  
❌ Limited enterprise features  
❌ Not truly cloud-agnostic (Swarm limitations)  

### Architecture Diagram
```
┌─────────────┐
│   Web UI    │
│  (React)    │
└──────┬──────┘
       │
┌──────▼──────┐
│   Captain   │ ← API Server (Node.js/Express)
│   Service   │
└──────┬──────┘
       │
┌──────▼──────────────────────┐
│     Docker Swarm Manager     │
└──────┬──────────────────────┘
       │
   ┌───┴───┬──────────┐
   ▼       ▼          ▼
┌──────┐ ┌──────┐ ┌──────┐
│ Node │ │ Node │ │ Node │
│  1   │ │  2   │ │  3   │
└──────┘ └──────┘ └──────┘
```

### Key Components
- `src/user/` - User management, auth
- `src/datastore/` - SQLite abstraction
- `src/docker/` - Docker API client
- `src/routes/` - API endpoints

### Recommendation
**⭐⭐⭐⭐ Good option** if you want a turnkey solution with nice UI and don't need Kubernetes. Not ideal for cloud-agnostic requirements due to Swarm dependency.

---

## 3. Deis Workflow (Archived)

**Status**: ❌ Archived (2017)  
**Repository**: https://github.com/deis/workflow  
**License**: MIT

### Overview
Deis was a Kubernetes-native PaaS that Microsoft acquired and shut down. It's archived but contains excellent design patterns.

### Architecture
- **Orchestration**: Kubernetes
- **Buildpacks**: Heroku buildpacks via slugbuilder
- **Routing**: Custom router (Go) on top of K8s services
- **Storage**: Minio (S3-compatible)
- **Registry**: Deis Registry (Docker registry)

### Pros
✅ Kubernetes-native design  
✅ Clean architecture, well-written Go code  
✅ True multi-tenancy  
✅ Excellent reference for K8s PaaS patterns  
✅ Helm charts available  

### Cons
❌ Project is dead (last commit 2018)  
❌ Would need significant modernization  
❌ Uses old K8s APIs  
❌ No community support  

### Architecture Overview
```
                    ┌─────────────┐
                    │   Deis CLI  │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   Controller│ ← API server (Django/Python)
                    │   (deis-api)│
                    └──────┬──────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
   ┌────▼─────┐      ┌────▼────┐      ┌─────▼─────┐
   │ Builder  │      │ Registry│      │  Database │
   │(slugs)   │      │(images) │      │(postgres) │
   └────┬─────┘      └────┬────┘      └───────────┘
        │                 │
   ┌────▼─────────────────▼────┐
   │     Kubernetes Cluster     │
   │  ┌──────┐  ┌──────┐       │
   │  │ Pod  │  │ Pod  │  ...  │
   │  └──────┘  └──────┘       │
   └────────────────────────────┘
```

### Key Patterns to Study

**1. App Model** (`controller/api/models/app.py`)
```python
class App(models.Model):
    owner = models.ForeignKey(User)
    id = models.CharField(max_length=64, unique=True)
    structure = JSONField(default={})  # Process types
    
    def deploy(self, release):
        """Deploy a new release"""
        # 1. Update K8s Deployment
        # 2. Wait for rollout
        # 3. Update router
```

**2. Release Model** (`controller/api/models/release.py`)
```python
class Release(models.Model):
    app = models.ForeignKey(App)
    version = models.PositiveIntegerField()
    slug = models.ForeignKey(Build)
    config = models.ForeignKey(Config)
    
    def publish(self):
        """Make this release live"""
        # Generate K8s manifests
        # Apply to cluster
```

**3. Builder Component** (`builder/`)
- Receives git push
- Runs buildpack build
- Creates slug (tarball)
- Uploads to Minio
- Triggers release creation

**4. Router Component** (`router/`)
- Custom Nginx-based router
- Dynamic configuration from K8s API
- SSL termination
- Load balancing

### Files to Study
- `controller/api/models/` - Data models
- `controller/scheduler/` - K8s API client
- `builder/` - Build system
- `router/` - Routing layer

### Recommendation
**⭐⭐⭐⭐ Excellent reference** for understanding K8s-native PaaS architecture. Don't fork (too outdated), but study the design patterns extensively.

---

## 4. Flynn (Archived)

**Status**: ❌ Archived (2019)  
**Repository**: https://github.com/flynn/flynn  
**License**: BSD-3-Clause

### Overview
Flynn was a complete "Heroku in a box" written in Go. Highly innovative but complex.

### Architecture
- **Orchestration**: Custom scheduler (pre-K8s era)
- **Buildpacks**: Heroku buildpacks
- **Service Discovery**: Custom (discoverd)
- **Routing**: Custom router
- **Database**: PostgreSQL with HA built-in

### Pros
✅ Complete, batteries-included  
✅ Innovative architecture (for its time)  
✅ Well-written Go code  
✅ Built-in PostgreSQL clustering  

### Cons
❌ Dead project (2019)  
❌ Custom orchestration vs K8s  
❌ Overly complex for modern needs  
❌ Not cloud-agnostic  

### Recommendation
**⭐⭐ Reference only**. Study the ideas but don't use as base. Kubernetes has replaced the need for custom schedulers.

---

## 5. Cloud Foundry

**Status**: ⭐ Active (Enterprise, VMware-backed)  
**Repository**: https://github.com/cloudfoundry  
**License**: Apache 2.0

### Overview
Enterprise-grade PaaS used by large organizations. Extremely mature and feature-rich but very complex.

### Architecture
- **Orchestration**: Custom (Diego scheduler)
- **Buildpacks**: Cloud Foundry buildpacks (became CNB standard)
- **Routing**: Gorouter
- **Service Broker**: OSBAPI (Open Service Broker)
- **BOSH**: Custom infrastructure management tool

### Components (150+ repos!)
- **Cloud Controller**: API server
- **Diego**: Container scheduler
- **Garden**: Container runtime
- **Gorouter**: HTTP router
- **UAA**: Auth service
- **Loggregator**: Logging system

### Pros
✅ Proven at massive scale (Fortune 500)  
✅ Extensive enterprise features  
✅ Strong multi-tenancy  
✅ Comprehensive security model  
✅ Active development  

### Cons
❌ Extremely complex (steep learning curve)  
❌ Custom orchestration (not K8s)  
❌ Heavy resource requirements  
❌ Difficult to operate  
❌ Not truly cloud-agnostic (BOSH complexity)  

### Recommendation
**⭐⭐ Not recommended as base**. Too complex for most use cases. Study the buildpack system and service broker pattern only.

---

## 6. Convox

**Status**: ⭐ Active (Commercial + OSS)  
**Repository**: https://github.com/convox/convox  
**License**: Apache 2.0

### Overview
Convox is a commercial PaaS that supports both Kubernetes and AWS ECS. Has both open source and paid versions.

### Architecture
- **Orchestration**: Kubernetes OR AWS ECS
- **Buildpacks**: Dockerfile-based
- **Routing**: K8s Ingress or AWS ALB
- **Storage**: Cloud-native (EBS, EFS, etc.)

### Pros
✅ Multi-cloud support (AWS, GCP, Azure, DO)  
✅ Active commercial backing  
✅ Good documentation  
✅ Modern architecture  
✅ Supports both K8s and ECS  

### Cons
❌ Commercial features behind paywall  
❌ Less community activity (small team)  
❌ Dockerfile-only (no buildpacks)  

### Architecture
```
┌──────────────┐
│  Convox CLI  │
└──────┬───────┘
       │
┌──────▼───────┐
│   Convox     │ ← API + Operator
│   Rack       │
└──────┬───────┘
       │
    ┌──┴──┐
    │     │
┌───▼─┐ ┌─▼────┐
│ K8s │ │ ECS  │ ← Choose orchestrator
└─────┘ └──────┘
```

### Recommendation
**⭐⭐⭐⭐ Good option** if you want cloud-agnostic capabilities out of the box. Consider forking or using as reference for multi-cloud patterns.

---

## 7. Tsuru

**Status**: ⭐ Active (Brazilian open source project)  
**Repository**: https://github.com/tsuru/tsuru  
**License**: BSD-3-Clause

### Overview
Tsuru is a Go-based PaaS that supports both Kubernetes and Docker Swarm.

### Architecture
- **Orchestration**: Kubernetes or Docker Swarm
- **Buildpacks**: Cloud Native Buildpacks
- **Routing**: Custom router or K8s Ingress
- **Database**: MongoDB for metadata

### Pros
✅ Supports K8s and Swarm  
✅ Cloud Native Buildpacks  
✅ Active development  
✅ Multi-pool support (different clusters)  

### Cons
❌ MongoDB dependency (not PostgreSQL)  
❌ Smaller community  
❌ Documentation mostly in Portuguese  
❌ Complex configuration  

### Recommendation
**⭐⭐⭐ Decent option** for K8s-based PaaS. Study the CNB integration and multi-pool architecture.

---

## 8. Coolify

**Status**: ⭐ Very Active (Modern alternative)  
**Repository**: https://github.com/coollabsio/coolify  
**License**: Apache 2.0

### Overview
Modern, self-hosted alternative to Heroku/Netlify/Vercel. Built with PHP/Laravel and growing fast.

### Architecture
- **Orchestration**: Docker (single or Swarm)
- **Buildpacks**: Nixpacks (Rust-based, fast)
- **Routing**: Traefik
- **UI**: Livewire (Laravel)

### Pros
✅ Modern, active community (15k+ stars)  
✅ Beautiful web UI  
✅ Easy to self-host  
✅ Good for modern web apps  
✅ Active development  

### Cons
❌ PHP/Laravel (if you prefer Go/Rust)  
❌ Not K8s-based  
❌ Young project (less battle-tested)  

### Recommendation
**⭐⭐⭐ Worth watching**. Great for inspiration on modern PaaS UX, but not ideal for enterprise cloud-agnostic requirements.

---

## Detailed Recommendation

### For Building from Scratch
**Study in this order:**
1. **Dokku** - Understand git-push workflow, buildpacks, routing
2. **Deis Workflow** - Learn K8s-native patterns, release management
3. **Convox** - Multi-cloud abstraction patterns

### For Forking/Extending

**Option A: Start with Convox**
- Already multi-cloud
- Modern Go codebase
- K8s-native
- Active project

**Option B: Modernize Deis Workflow**
- Update K8s APIs (1.28+)
- Replace Python with Go
- Adopt Cloud Native Buildpacks
- Add modern features

**Option C: Build on Dokku Concepts**
- Rewrite in Go
- Add K8s orchestration
- Keep simple UX

### My Top Pick: **Hybrid Approach**

```
Convox (multi-cloud abstraction)
    +
Deis Workflow (K8s patterns)
    +
Dokku (developer UX simplicity)
    +
Cloud Native Buildpacks (standard builds)
```

**Why:**
- Convox gives you cloud abstraction
- Deis patterns for K8s-native design
- Dokku's simplicity for UX
- CNB for build standards

---

## Code Repositories to Clone

```bash
# Primary study
git clone https://github.com/dokku/dokku
git clone https://github.com/deis/workflow
git clone https://github.com/convox/convox

# Secondary reference
git clone https://github.com/caprover/caprover
git clone https://github.com/tsuru/tsuru
git clone https://github.com/coollabsio/coolify

# Archived but valuable
git clone https://github.com/flynn/flynn
```

---

## Key Takeaways

1. **Kubernetes is the right foundation** for cloud-agnostic PaaS (2024+)
2. **Cloud Native Buildpacks** are the modern standard (not legacy Heroku buildpacks)
3. **Don't build custom schedulers** - use K8s
4. **Study Deis Workflow** for best K8s-native PaaS architecture patterns
5. **Study Dokku** for best developer UX patterns
6. **Use Convox** for multi-cloud abstraction inspiration
7. **Don't fork Cloud Foundry** - too complex

---

## Recommended Path Forward

### Phase 1: Research (1-2 weeks)
- Set up local Dokku instance
- Deploy test apps to understand workflow
- Read Deis Workflow source code
- Study Convox multi-cloud approach

### Phase 2: Prototype (1-2 months)
- Build minimal API server (Go)
- Integrate CNB for builds
- Deploy to local K8s cluster
- Implement git-push workflow

### Phase 3: Decide (After prototype)
- Fork Convox and extend?
- Build new with Deis patterns?
- Hybrid approach?

**My recommendation**: Build new with:
- Go API server (like Deis)
- Cloud Native Buildpacks
- Kubernetes-native (like Deis)
- Multi-cloud abstraction (like Convox)
- Developer UX inspiration from Dokku
