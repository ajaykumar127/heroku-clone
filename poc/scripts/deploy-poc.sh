#!/bin/bash
# Deploy the complete POC to a local Kubernetes cluster
# Requires: kind or Docker Desktop with Kubernetes enabled

set -e

echo "=====> Platform POC Deployment"
echo ""

# Check prerequisites
echo "-----> Checking prerequisites"

if ! command -v kubectl &> /dev/null; then
    echo "ERROR: kubectl not found. Please install kubectl."
    exit 1
fi

if ! command -v docker &> /dev/null; then
    echo "ERROR: docker not found. Please install Docker."
    exit 1
fi

if ! command -v pack &> /dev/null; then
    echo "WARNING: pack CLI not found. Installing..."
    # Install pack CLI
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install buildpacks/tap/pack
    else
        curl -sSL "https://github.com/buildpacks/pack/releases/download/v0.33.2/pack-v0.33.2-linux.tgz" | tar -xz -C /tmp
        sudo mv /tmp/pack /usr/local/bin/
    fi
fi

echo "       ✓ All prerequisites met"
echo ""

# Create/verify Kubernetes cluster
echo "-----> Setting up Kubernetes cluster"

if kubectl cluster-info &> /dev/null; then
    echo "       ✓ Kubernetes cluster is running"
else
    echo "ERROR: No Kubernetes cluster found."
    echo "Please start Docker Desktop Kubernetes or create a kind cluster:"
    echo "  kind create cluster --name platform-poc"
    exit 1
fi

# Install Nginx Ingress Controller
echo "-----> Installing Nginx Ingress Controller"

if kubectl get namespace ingress-nginx &> /dev/null; then
    echo "       ✓ Ingress controller already installed"
else
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.10.1/deploy/static/provider/cloud/deploy.yaml
    echo "       Waiting for ingress controller to be ready..."
    kubectl wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=180s
fi

echo ""

# Set up local Docker registry
echo "-----> Setting up local Docker registry"

if docker ps | grep -q registry:2; then
    echo "       ✓ Registry already running"
else
    docker run -d -p 5000:5000 --name platform-registry registry:2
    echo "       ✓ Registry started on localhost:5000"
fi

echo ""

# Build and start API server
echo "-----> Building API server"
cd api-server
if [ ! -f "go.mod" ]; then
    go mod init github.com/platform/api-server
    go get github.com/google/uuid
    go get github.com/gorilla/mux
    go get github.com/mattn/go-sqlite3
fi
go build -o ../bin/api-server .
cd ..

echo "       ✓ API server built"
echo ""

# Build and start Git server
echo "-----> Building Git server"
cd git-server
if [ ! -f "go.mod" ]; then
    go mod init github.com/platform/git-server
    go get github.com/gliderlabs/ssh
    go get golang.org/x/crypto/ssh
fi
go build -o ../bin/git-server .
cd ..

echo "       ✓ Git server built"
echo ""

# Start services
echo "-----> Starting services"

# Kill existing processes
pkill -f "bin/api-server" || true
pkill -f "bin/git-server" || true
sleep 1

# Start API server
./bin/api-server > logs/api-server.log 2>&1 &
API_PID=$!
echo "       ✓ API server started (PID: $API_PID)"

# Start Git server
./bin/git-server > logs/git-server.log 2>&1 &
GIT_PID=$!
echo "       ✓ Git server started (PID: $GIT_PID)"

# Wait for services to be ready
sleep 2

if ! ps -p $API_PID > /dev/null; then
    echo "ERROR: API server failed to start"
    cat logs/api-server.log
    exit 1
fi

if ! ps -p $GIT_PID > /dev/null; then
    echo "ERROR: Git server failed to start"
    cat logs/git-server.log
    exit 1
fi

echo ""
echo "=====> POC Deployment Complete!"
echo ""
echo "Services running:"
echo "  • API Server:  http://localhost:8080"
echo "  • Git Server:  ssh://localhost:2222"
echo "  • Registry:    http://localhost:5000"
echo ""
echo "Process IDs:"
echo "  • API Server:  $API_PID"
echo "  • Git Server:  $GIT_PID"
echo ""
echo "Logs:"
echo "  • tail -f logs/api-server.log"
echo "  • tail -f logs/git-server.log"
echo ""
echo "Next steps:"
echo "  1. Create an app:    curl -X POST http://localhost:8080/v1/apps -H 'Content-Type: application/json' -d '{\"name\":\"my-app\"}'"
echo "  2. Deploy example:   cd example-app && git init && git add . && git commit -m 'Initial' && git remote add platform ssh://git@localhost:2222/my-app.git && git push platform main"
echo "  3. Access app:       curl http://my-app.localhost"
echo ""
