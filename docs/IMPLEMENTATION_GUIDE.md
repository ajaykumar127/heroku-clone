# Implementation Guide: Building a Cloud-Agnostic PaaS

This guide provides step-by-step instructions for implementing each component of the platform.

## Table of Contents

1. [Control Plane Implementation](#1-control-plane-implementation)
2. [Build System Implementation](#2-build-system-implementation)
3. [Kubernetes Integration](#3-kubernetes-integration)
4. [Routing & Ingress](#4-routing--ingress)
5. [Logging System](#5-logging-system)
6. [CLI Tool](#6-cli-tool)
7. [Testing Strategy](#7-testing-strategy)
8. [Production Deployment](#8-production-deployment)

---

## 1. Control Plane Implementation

### 1.1 API Server Setup

**Technology**: Go with Gin framework

**File**: `cmd/api-server/main.go`

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/platform/internal/api"
    "github.com/platform/internal/db"
)

func main() {
    // Initialize database
    database := db.NewPostgres(config.DatabaseURL)
    
    // Initialize API router
    router := gin.Default()
    
    // Middleware
    router.Use(api.AuthMiddleware())
    router.Use(api.LoggingMiddleware())
    router.Use(api.ErrorMiddleware())
    
    // Routes
    v1 := router.Group("/v1")
    {
        apps := v1.Group("/apps")
        {
            apps.GET("", api.ListApps)
            apps.POST("", api.CreateApp)
            apps.GET("/:name", api.GetApp)
            apps.DELETE("/:name", api.DeleteApp)
            
            // Nested resources
            apps.GET("/:name/releases", api.ListReleases)
            apps.POST("/:name/releases", api.CreateRelease)
            apps.PATCH("/:name/config-vars", api.UpdateConfig)
            apps.PATCH("/:name/formation", api.ScaleApp)
        }
        
        // Other routes...
    }
    
    router.Run(":8080")
}
```

**Key Packages**:
- `internal/api` - HTTP handlers
- `internal/models` - Data models
- `internal/db` - Database layer
- `internal/k8s` - Kubernetes client
- `internal/builds` - Build system

### 1.2 Database Layer

**File**: `internal/db/postgres.go`

```go
package db

import (
    "database/sql"
    "github.com/jmoiron/sqlx"
)

type Database struct {
    *sqlx.DB
}

func NewPostgres(dsn string) (*Database, error) {
    db, err := sqlx.Connect("postgres", dsn)
    if err != nil {
        return nil, err
    }
    
    // Connection pooling
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    
    return &Database{db}, nil
}

// App operations
func (db *Database) CreateApp(app *models.App) error {
    query := `
        INSERT INTO apps (id, name, owner_id, region, git_url, web_url)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING created_at
    `
    return db.QueryRow(query, app.ID, app.Name, app.OwnerID, 
        app.Region, app.GitURL, app.WebURL).Scan(&app.CreatedAt)
}

func (db *Database) GetApp(name string) (*models.App, error) {
    var app models.App
    query := `SELECT * FROM apps WHERE name = $1`
    err := db.Get(&app, query, name)
    return &app, err
}

// Release operations
func (db *Database) CreateRelease(release *models.Release) error {
    // Version auto-incremented by trigger
    query := `
        INSERT INTO releases (id, app_id, slug_id, config_vars, description)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING version, created_at
    `
    return db.QueryRow(query, release.ID, release.AppID, 
        release.SlugID, release.ConfigVars, release.Description).
        Scan(&release.Version, &release.CreatedAt)
}
```

### 1.3 Authentication Middleware

**File**: `internal/api/auth.go`

```go
package api

import (
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract token from Authorization header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(401, gin.H{"error": "Authorization required"})
            c.Abort()
            return
        }
        
        // Parse Bearer token
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        
        // Verify JWT or API token
        claims, err := verifyToken(tokenString)
        if err != nil {
            c.JSON(401, gin.H{"error": "Invalid token"})
            c.Abort()
            return
        }
        
        // Set user context
        c.Set("user_id", claims.UserID)
        c.Set("user_email", claims.Email)
        
        c.Next()
    }
}
```

---

## 2. Build System Implementation

### 2.1 Build Worker

**File**: `cmd/build-worker/main.go`

```go
package main

import (
    "context"
    "log"
    "github.com/platform/internal/builds"
)

func main() {
    // Connect to job queue (NATS, RabbitMQ, etc.)
    queue := builds.NewJobQueue(config.QueueURL)
    
    // Start workers
    for i := 0; i < config.NumWorkers; i++ {
        go worker(i, queue)
    }
    
    select {}
}

func worker(id int, queue *builds.JobQueue) {
    for {
        job, err := queue.Next()
        if err != nil {
            log.Printf("Worker %d: error fetching job: %v", id, err)
            continue
        }
        
        log.Printf("Worker %d: building app=%s commit=%s", 
            id, job.AppName, job.Commit)
        
        // Build with CNB
        builder := builds.NewCNBBuilder()
        slug, err := builder.Build(context.Background(), job)
        if err != nil {
            log.Printf("Worker %d: build failed: %v", id, err)
            job.MarkFailed(err)
            continue
        }
        
        log.Printf("Worker %d: build succeeded, slug=%s", id, slug.ID)
        job.MarkSuccess(slug)
    }
}
```

### 2.2 Cloud Native Buildpacks Integration

**File**: `internal/builds/cnb.go`

```go
package builds

import (
    "context"
    "os/exec"
    "github.com/buildpacks/pack"
)

type CNBBuilder struct {
    packClient pack.Client
}

func NewCNBBuilder() *CNBBuilder {
    client, _ := pack.NewClient()
    return &CNBBuilder{packClient: client}
}

func (b *CNBBuilder) Build(ctx context.Context, job *BuildJob) (*Slug, error) {
    // Clone repo
    repoPath := cloneRepo(job.RepoURL, job.Commit)
    defer os.RemoveAll(repoPath)
    
    // Prepare build options
    imageName := fmt.Sprintf("registry.platform.com/%s:%s", 
        job.AppName, job.Commit)
    
    opts := pack.BuildOptions{
        Image:      imageName,
        Builder:    "paketobuildpacks/builder:base",
        AppPath:    repoPath,
        Publish:    true,
        TrustBuilder: true,
    }
    
    // Execute build
    err := b.packClient.Build(ctx, opts)
    if err != nil {
        return nil, err
    }
    
    // Create slug record
    slug := &Slug{
        ID:       uuid.New().String(),
        AppID:    job.AppID,
        Commit:   job.Commit,
        ImageURL: imageName,
    }
    
    return slug, nil
}
```

### 2.3 Alternative: Dockerfile Builder

**File**: `internal/builds/docker.go`

```go
package builds

func (b *DockerBuilder) Build(ctx context.Context, job *BuildJob) (*Slug, error) {
    // Clone repo
    repoPath := cloneRepo(job.RepoURL, job.Commit)
    defer os.RemoveAll(repoPath)
    
    // Check for Dockerfile
    dockerfilePath := filepath.Join(repoPath, "Dockerfile")
    if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
        return nil, fmt.Errorf("Dockerfile not found")
    }
    
    // Build with Docker
    imageName := fmt.Sprintf("registry.platform.com/%s:%s", 
        job.AppName, job.Commit)
    
    cmd := exec.CommandContext(ctx, "docker", "build", 
        "-t", imageName, 
        "-f", dockerfilePath,
        repoPath)
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("build failed: %s", output)
    }
    
    // Push to registry
    cmd = exec.CommandContext(ctx, "docker", "push", imageName)
    if err := cmd.Run(); err != nil {
        return nil, err
    }
    
    return &Slug{ImageURL: imageName}, nil
}
```

---

## 3. Kubernetes Integration

### 3.1 Kubernetes Client

**File**: `internal/k8s/client.go`

```go
package k8s

import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

type Client struct {
    clientset *kubernetes.Clientset
}

func NewClient() (*Client, error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        // Fallback to kubeconfig
        config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
        if err != nil {
            return nil, err
        }
    }
    
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, err
    }
    
    return &Client{clientset: clientset}, nil
}
```

### 3.2 Deployment Manager

**File**: `internal/k8s/deploy.go`

```go
package k8s

import (
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) DeployApp(app *models.App, release *models.Release) error {
    namespace := getAppNamespace(app.ID)
    
    // Create namespace if needed
    c.ensureNamespace(namespace)
    
    // Create/update Deployment
    deployment := c.buildDeployment(app, release)
    _, err := c.clientset.AppsV1().Deployments(namespace).Update(
        context.Background(), deployment, metav1.UpdateOptions{})
    
    if errors.IsNotFound(err) {
        _, err = c.clientset.AppsV1().Deployments(namespace).Create(
            context.Background(), deployment, metav1.CreateOptions{})
    }
    
    if err != nil {
        return err
    }
    
    // Create/update Service
    service := c.buildService(app)
    c.clientset.CoreV1().Services(namespace).Update(
        context.Background(), service, metav1.UpdateOptions{})
    
    // Create/update Ingress
    ingress := c.buildIngress(app)
    c.clientset.NetworkingV1().Ingresses(namespace).Update(
        context.Background(), ingress, metav1.UpdateOptions{})
    
    return nil
}

func (c *Client) buildDeployment(app *models.App, release *models.Release) *appsv1.Deployment {
    labels := map[string]string{
        "app":     app.Name,
        "release": fmt.Sprintf("v%d", release.Version),
    }
    
    replicas := int32(getReplicaCount(app))
    
    return &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name:   app.Name,
            Labels: labels,
        },
        Spec: appsv1.DeploymentSpec{
            Replicas: &replicas,
            Selector: &metav1.LabelSelector{
                MatchLabels: map[string]string{"app": app.Name},
            },
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: labels,
                },
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{
                        {
                            Name:  "web",
                            Image: release.Slug.ImageURL,
                            Ports: []corev1.ContainerPort{
                                {ContainerPort: 8080, Name: "http"},
                            },
                            Env: buildEnvVars(release.ConfigVars),
                            Resources: corev1.ResourceRequirements{
                                Requests: corev1.ResourceList{
                                    corev1.ResourceCPU:    resource.MustParse("500m"),
                                    corev1.ResourceMemory: resource.MustParse("512Mi"),
                                },
                                Limits: corev1.ResourceList{
                                    corev1.ResourceCPU:    resource.MustParse("1000m"),
                                    corev1.ResourceMemory: resource.MustParse("1Gi"),
                                },
                            },
                            LivenessProbe: &corev1.Probe{
                                ProbeHandler: corev1.ProbeHandler{
                                    HTTPGet: &corev1.HTTPGetAction{
                                        Path: "/",
                                        Port: intstr.FromInt(8080),
                                    },
                                },
                                InitialDelaySeconds: 30,
                                PeriodSeconds:       10,
                            },
                        },
                    },
                },
            },
        },
    }
}
```

### 3.3 Scaling

**File**: `internal/k8s/scale.go`

```go
package k8s

func (c *Client) ScaleApp(app *models.App, processType string, quantity int) error {
    namespace := getAppNamespace(app.ID)
    deploymentName := fmt.Sprintf("%s-%s", app.Name, processType)
    
    scale, err := c.clientset.AppsV1().Deployments(namespace).
        GetScale(context.Background(), deploymentName, metav1.GetOptions{})
    if err != nil {
        return err
    }
    
    scale.Spec.Replicas = int32(quantity)
    
    _, err = c.clientset.AppsV1().Deployments(namespace).
        UpdateScale(context.Background(), deploymentName, scale, metav1.UpdateOptions{})
    
    return err
}
```

---

## 4. Routing & Ingress

### 4.1 Install Nginx Ingress Controller

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.10.1/deploy/static/provider/cloud/deploy.yaml
```

### 4.2 SSL Certificate Manager

**File**: `internal/ssl/certmanager.go`

```go
package ssl

import (
    certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

func (c *Client) ProvisionSSL(domain *models.Domain) error {
    // Create Certificate resource
    cert := &certmanagerv1.Certificate{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("%s-tls", domain.Hostname),
            Namespace: getAppNamespace(domain.AppID),
        },
        Spec: certmanagerv1.CertificateSpec{
            DNSNames: []string{domain.Hostname},
            SecretName: fmt.Sprintf("%s-tls", domain.Hostname),
            IssuerRef: certmanagerv1.ObjectReference{
                Name: "letsencrypt-prod",
                Kind: "ClusterIssuer",
            },
        },
    }
    
    _, err := c.certClient.CertmanagerV1().Certificates(
        cert.Namespace).Create(context.Background(), cert, metav1.CreateOptions{})
    
    return err
}
```

---

## 5. Logging System

### 5.1 Deploy Loki

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm install loki grafana/loki-stack \
  --set promtail.enabled=true \
  --set loki.persistence.enabled=true
```

### 5.2 Log Streaming API

**File**: `internal/api/logs.go`

```go
package api

func StreamLogs(c *gin.Context) {
    appName := c.Param("name")
    
    // Get user context
    userID := c.GetString("user_id")
    
    // Verify access
    app, err := db.GetApp(appName)
    if err != nil || !hasAccess(userID, app) {
        c.JSON(404, gin.H{"error": "App not found"})
        return
    }
    
    // Set SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    
    // Query Loki
    lokiClient := loki.NewClient(config.LokiURL)
    stream, err := lokiClient.QueryStream(context.Background(), loki.QueryOptions{
        Query: fmt.Sprintf(`{app="%s"}`, appName),
        Tail:  true,
    })
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    // Stream logs
    for log := range stream {
        c.SSEvent("log", log)
        c.Writer.Flush()
    }
}
```

---

## 6. CLI Tool

### 6.1 CLI Structure

**File**: `cmd/platform/main.go`

```go
package main

import (
    "github.com/spf13/cobra"
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "platform",
        Short: "Platform CLI",
    }
    
    // Auth commands
    rootCmd.AddCommand(loginCmd)
    rootCmd.AddCommand(logoutCmd)
    
    // App commands
    rootCmd.AddCommand(createCmd)
    rootCmd.AddCommand(appsCmd)
    rootCmd.AddCommand(infoCmd)
    rootCmd.AddCommand(destroyCmd)
    
    // Deployment commands
    rootCmd.AddCommand(releasesCmd)
    rootCmd.AddCommand(rollbackCmd)
    
    // Config commands
    rootCmd.AddCommand(configCmd)
    rootCmd.AddCommand(configSetCmd)
    
    // Process commands
    rootCmd.AddCommand(psCmd)
    rootCmd.AddCommand(scaleCmd)
    rootCmd.AddCommand(restartCmd)
    
    // Logs
    rootCmd.AddCommand(logsCmd)
    
    // Run command
    rootCmd.AddCommand(runCmd)
    
    rootCmd.Execute()
}
```

### 6.2 Key Commands

**Create App**:
```go
var createCmd = &cobra.Command{
    Use:   "create [name]",
    Short: "Create a new app",
    Run: func(cmd *cobra.Command, args []string) {
        client := api.NewClient(config.APIToken)
        app, err := client.CreateApp(api.CreateAppRequest{
            Name:   args[0],
            Region: region,
        })
        if err != nil {
            log.Fatal(err)
        }
        
        fmt.Printf("Created %s | %s\n", app.Name, app.WebURL)
        fmt.Printf("Git remote: %s\n", app.GitURL)
    },
}
```

**Stream Logs**:
```go
var logsCmd = &cobra.Command{
    Use:   "logs",
    Short: "Stream application logs",
    Run: func(cmd *cobra.Command, args []string) {
        client := api.NewClient(config.APIToken)
        stream, err := client.StreamLogs(appName, tail)
        if err != nil {
            log.Fatal(err)
        }
        
        for log := range stream {
            fmt.Println(log.Format())
        }
    },
}
```

---

## 7. Testing Strategy

### 7.1 Unit Tests

```go
// internal/api/apps_test.go
func TestCreateApp(t *testing.T) {
    db := testdb.New()
    router := setupRouter(db)
    
    req := httptest.NewRequest("POST", "/v1/apps", 
        strings.NewReader(`{"name":"test-app"}`))
    req.Header.Set("Authorization", "Bearer "+testToken)
    
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 201, w.Code)
}
```

### 7.2 Integration Tests

```go
// test/integration/deploy_test.go
func TestFullDeployment(t *testing.T) {
    // Create app
    app := createTestApp(t, "test-app")
    
    // Push code
    repo := git.Clone(t, app.GitURL)
    repo.Add("index.js", nodeApp)
    repo.Commit("Initial")
    repo.Push("main")
    
    // Wait for deployment
    waitForDeployment(t, app.Name, 2*time.Minute)
    
    // Test app is accessible
    resp, err := http.Get(app.WebURL)
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
}
```

---

## 8. Production Deployment

### 8.1 Infrastructure Setup (Terraform)

```hcl
# terraform/aws/main.tf
module "eks" {
  source = "terraform-aws-modules/eks/aws"
  
  cluster_name    = "platform-prod"
  cluster_version = "1.28"
  
  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets
  
  eks_managed_node_groups = {
    main = {
      min_size     = 3
      max_size     = 10
      desired_size = 5
      
      instance_types = ["m5.2xlarge"]
    }
  }
}
```

### 8.2 Deployment with Helm

```yaml
# helm/platform/values.yaml
apiServer:
  replicaCount: 3
  image:
    repository: platform/api-server
    tag: "1.0.0"
  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: 2000m
      memory: 2Gi

database:
  host: postgres.prod.svc.cluster.local
  name: platform
  
ingress:
  enabled: true
  host: api.platform.com
  tls:
    enabled: true
```

### 8.3 Monitoring

```yaml
# Prometheus ServiceMonitor
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: platform-api
spec:
  selector:
    matchLabels:
      app: platform-api
  endpoints:
  - port: metrics
    interval: 30s
```

---

## Next Steps

1. **Week 1-2**: Implement API server and database layer
2. **Week 3-4**: Build system with CNB integration
3. **Week 5-6**: Kubernetes deployment manager
4. **Week 7-8**: Git server and receive hooks
5. **Week 9-10**: Logging and monitoring
6. **Week 11-12**: CLI tool
7. **Week 13-16**: Testing, documentation, polish
8. **Week 17-18**: Production deployment and hardening

Total: 4-5 months with a team of 5-7 engineers.
