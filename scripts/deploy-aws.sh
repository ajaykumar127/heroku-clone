#!/usr/bin/env bash
# =============================================================================
# Platform — One-Click AWS Deployment
# Usage: ./scripts/deploy-aws.sh [--region us-east-1] [--cluster-name platform-runtime]
# =============================================================================
set -euo pipefail

# ── Colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'

log()     { echo -e "${BLUE}[platform]${NC} $*"; }
success() { echo -e "${GREEN}[✓]${NC} $*"; }
warn()    { echo -e "${YELLOW}[!]${NC} $*"; }
fail()    { echo -e "${RED}[✗]${NC} $*"; exit 1; }
header()  { echo -e "\n${BOLD}══════════════════════════════════════${NC}"; echo -e "${BOLD} $* ${NC}"; echo -e "${BOLD}══════════════════════════════════════${NC}\n"; }

# ── Defaults (override via flags or environment) ──────────────────────────────
REGION="${PLATFORM_AWS_REGION:-us-east-1}"
CLUSTER_NAME="${PLATFORM_CLUSTER_NAME:-platform-runtime}"
CONTROL_PLANE_PORT="${PLATFORM_CP_PORT:-8080}"
RUNTIME_NAME="${PLATFORM_RUNTIME_NAME:-aws-${REGION}}"
NODE_TYPE="${PLATFORM_NODE_TYPE:-t3.medium}"

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case $1 in
    --region)        REGION="$2";       shift 2 ;;
    --cluster-name)  CLUSTER_NAME="$2"; shift 2 ;;
    --port)          CONTROL_PLANE_PORT="$2"; shift 2 ;;
    --node-type)     NODE_TYPE="$2";    shift 2 ;;
    --help|-h)
      echo "Usage: $0 [--region REGION] [--cluster-name NAME] [--port PORT] [--node-type TYPE]"
      echo ""
      echo "Environment variables:"
      echo "  PLATFORM_AWS_REGION      AWS region (default: us-east-1)"
      echo "  PLATFORM_CLUSTER_NAME    EKS cluster name (default: platform-runtime)"
      echo "  PLATFORM_CP_PORT         Control Plane port (default: 8080)"
      echo "  PLATFORM_NODE_TYPE       EC2 instance type (default: t3.medium)"
      exit 0 ;;
    *) fail "Unknown argument: $1. Run with --help for usage." ;;
  esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── State file — allows resume if interrupted ────────────────────────────────
STATE_FILE="${REPO_ROOT}/.deploy-state"
state_set()  { echo "$1=done" >> "$STATE_FILE"; }
state_done() { grep -q "^$1=done" "$STATE_FILE" 2>/dev/null; }

echo ""
echo -e "${BOLD}  Platform — One-Click AWS Deployment${NC}"
echo -e "  Region: ${REGION} · Cluster: ${CLUSTER_NAME}"
echo ""

# =============================================================================
# STEP 1 — Check prerequisites
# =============================================================================
header "Step 1/7 — Checking prerequisites"

check_cmd() {
  local cmd=$1 install_hint=$2
  if command -v "$cmd" &>/dev/null; then
    success "$cmd found ($(${cmd} --version 2>&1 | head -1))"
  else
    fail "$cmd not found. Install with: $install_hint"
  fi
}

check_cmd go        "brew install go"
check_cmd terraform "brew install terraform"
check_cmd aws       "brew install awscli"
check_cmd kubectl   "brew install kubectl"
check_cmd git       "pre-installed on macOS"

# Verify AWS credentials
log "Verifying AWS credentials..."
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text 2>/dev/null) \
  || fail "AWS credentials not configured. Run: aws configure"
success "AWS account: ${ACCOUNT_ID} (region: ${REGION})"

# =============================================================================
# STEP 2 — Build binaries
# =============================================================================
header "Step 2/7 — Building platform binaries"

if ! state_done "build"; then
  log "Building Control Plane..."
  cd "${REPO_ROOT}/control-plane"
  go build -o bin/control-plane . 2>&1 | sed 's/^/  /'
  success "Control Plane built"

  log "Building CLI..."
  cd "${REPO_ROOT}/cli"
  go build -o bin/platform . 2>&1 | sed 's/^/  /'
  success "CLI built"

  log "Building Git Server..."
  cd "${REPO_ROOT}/poc/git-server"
  go build -o bin/git-server . 2>&1 | sed 's/^/  /'
  success "Git Server built"

  state_set "build"
else
  success "Binaries already built (skipping)"
fi

# ── Add CLI to PATH for this session ─────────────────────────────────────────
export PATH="${REPO_ROOT}/cli/bin:${PATH}"

# =============================================================================
# STEP 3 — Start Control Plane
# =============================================================================
header "Step 3/7 — Starting Control Plane"

CP_PID_FILE="${REPO_ROOT}/.control-plane.pid"
CP_LOG_FILE="${REPO_ROOT}/.control-plane.log"
CP_DB="${REPO_ROOT}/control-plane/platform.db"

# Detect this machine's LAN/public IP so EKS can reach it
MACHINE_IP=$(curl -s --max-time 5 https://checkip.amazonaws.com 2>/dev/null \
  || ipconfig getifaddr en0 2>/dev/null \
  || hostname -I 2>/dev/null | awk '{print $1}' \
  || echo "127.0.0.1")
CONTROL_PLANE_URL="http://${MACHINE_IP}:${CONTROL_PLANE_PORT}"

if [[ -f "$CP_PID_FILE" ]] && kill -0 "$(cat "$CP_PID_FILE")" 2>/dev/null; then
  success "Control Plane already running (PID $(cat "$CP_PID_FILE"))"
else
  log "Starting Control Plane on port ${CONTROL_PLANE_PORT}..."
  DATABASE_URL="${CP_DB}" \
  PORT="${CONTROL_PLANE_PORT}" \
  GIT_SERVER_HOST="${MACHINE_IP}" \
  GIT_SERVER_PORT="2222" \
  "${REPO_ROOT}/control-plane/bin/control-plane" > "$CP_LOG_FILE" 2>&1 &
  echo $! > "$CP_PID_FILE"

  # Wait for it to become healthy
  for i in $(seq 1 20); do
    if curl -sf "http://localhost:${CONTROL_PLANE_PORT}/health" &>/dev/null; then
      success "Control Plane is healthy at http://localhost:${CONTROL_PLANE_PORT}"
      break
    fi
    if [[ $i -eq 20 ]]; then
      fail "Control Plane did not start. Check log: ${CP_LOG_FILE}"
    fi
    sleep 1
  done
fi

# =============================================================================
# STEP 4 — Start Git Server
# =============================================================================
header "Step 4/7 — Starting Git Server"

GIT_PID_FILE="${REPO_ROOT}/.git-server.pid"
GIT_LOG_FILE="${REPO_ROOT}/.git-server.log"

if [[ -f "$GIT_PID_FILE" ]] && kill -0 "$(cat "$GIT_PID_FILE")" 2>/dev/null; then
  success "Git Server already running (PID $(cat "$GIT_PID_FILE"))"
else
  log "Starting Git Server on port 2222..."
  cd "${REPO_ROOT}/poc/git-server"
  API_URL="http://localhost:${CONTROL_PLANE_PORT}" \
  "${REPO_ROOT}/poc/git-server/bin/git-server" > "$GIT_LOG_FILE" 2>&1 &
  echo $! > "$GIT_PID_FILE"
  sleep 2
  if kill -0 "$(cat "$GIT_PID_FILE")" 2>/dev/null; then
    success "Git Server is running on port 2222"
  else
    fail "Git Server failed to start. Check log: ${GIT_LOG_FILE}"
  fi
fi

# =============================================================================
# STEP 5 — Create API token
# =============================================================================
header "Step 5/7 — Setting up authentication"

TOKEN_FILE="${HOME}/.platform/config"

if [[ -f "$TOKEN_FILE" ]] && grep -q "token:" "$TOKEN_FILE" 2>/dev/null; then
  success "API token already exists at ${TOKEN_FILE}"
  PLATFORM_TOKEN=$(grep "token:" "$TOKEN_FILE" | awk '{print $2}')
else
  log "Creating API token..."
  TOKEN_RESPONSE=$(curl -sf -X POST \
    "http://localhost:${CONTROL_PLANE_PORT}/v1/auth/tokens" \
    -H "Content-Type: application/json" \
    -d '{"comment":"deploy-aws one-click"}')
  PLATFORM_TOKEN=$(echo "$TOKEN_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

  mkdir -p "${HOME}/.platform"
  cat > "$TOKEN_FILE" <<EOF
token: ${PLATFORM_TOKEN}
api_url: http://localhost:${CONTROL_PLANE_PORT}
EOF
  chmod 600 "$TOKEN_FILE"
  success "API token created and saved to ${TOKEN_FILE}"
fi

export PLATFORM_TOKEN

# =============================================================================
# STEP 6 — Terraform: provision EKS runtime plane
# =============================================================================
header "Step 6/7 — Provisioning AWS Runtime Plane (EKS)"

cd "${REPO_ROOT}/infra/aws"

if ! state_done "terraform_init"; then
  log "Running terraform init..."
  terraform init -input=false 2>&1 | grep -E "(Initializing|provider|complete|error)" | sed 's/^/  /'
  state_set "terraform_init"
else
  success "Terraform already initialised (skipping)"
fi

log "Running terraform apply (this takes 10–15 minutes)..."
warn "EKS cluster, VPC, ECR, and IAM roles will be created in ${REGION}"

terraform apply \
  -auto-approve \
  -input=false \
  -var="region=${REGION}" \
  -var="cluster_name=${CLUSTER_NAME}" \
  -var="runtime_name=${RUNTIME_NAME}" \
  -var="control_plane_url=${CONTROL_PLANE_URL}" \
  -var="node_instance_type=${NODE_TYPE}" \
  2>&1 | grep -E "(aws_|module\.|Plan:|Apply complete|Error)" | sed 's/^/  /'

success "AWS Runtime Plane provisioned"

# Capture Terraform outputs
ECR_URL=$(terraform output -raw ecr_repository_url 2>/dev/null || echo "")
KUBECONFIG_CMD=$(terraform output -raw kubeconfig_command 2>/dev/null || echo "")

state_set "terraform"

# =============================================================================
# STEP 7 — Configure kubectl + verify agent
# =============================================================================
header "Step 7/7 — Connecting kubectl and verifying Runtime Agent"

log "Configuring kubectl for the new cluster..."
eval "$KUBECONFIG_CMD"
success "kubectl configured"

log "Waiting for Runtime Agent to start and register (up to 3 minutes)..."
for i in $(seq 1 36); do
  AGENT_STATUS=$(kubectl get pods -n platform-system -l app=runtime-agent \
    --no-headers 2>/dev/null | awk '{print $3}' | head -1)
  if [[ "$AGENT_STATUS" == "Running" ]]; then
    success "Runtime Agent pod is Running"
    break
  fi
  if [[ $i -eq 36 ]]; then
    warn "Agent pod not yet Running — check: kubectl get pods -n platform-system"
  fi
  printf "  waiting for agent pod (%ss)...\r" "$((i*5))"
  sleep 5
done

log "Waiting for Runtime Agent to register with Control Plane..."
for i in $(seq 1 12); do
  RUNTIME_COUNT=$(curl -sf \
    -H "Authorization: Bearer ${PLATFORM_TOKEN}" \
    "http://localhost:${CONTROL_PLANE_PORT}/v1/runtimes" \
    2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 0)" 2>/dev/null || echo "0")
  if [[ "$RUNTIME_COUNT" -gt 0 ]]; then
    success "Runtime Plane registered with Control Plane"
    break
  fi
  if [[ $i -eq 12 ]]; then
    warn "Runtime not yet registered — it may still be starting up"
  fi
  sleep 5
done

# =============================================================================
# Done — print summary
# =============================================================================
echo ""
echo -e "${GREEN}${BOLD}════════════════════════════════════════${NC}"
echo -e "${GREEN}${BOLD}  Deployment complete!${NC}"
echo -e "${GREEN}${BOLD}════════════════════════════════════════${NC}"
echo ""
echo -e "  ${BOLD}Control Plane${NC}   http://localhost:${CONTROL_PLANE_PORT}"
echo -e "  ${BOLD}Runtime Plane${NC}   ${RUNTIME_NAME} (${REGION})"
[[ -n "$ECR_URL" ]] && echo -e "  ${BOLD}ECR Registry${NC}    ${ECR_URL}"
echo -e "  ${BOLD}API Token${NC}       ${TOKEN_FILE}"
echo ""
echo -e "${BOLD}Next steps:${NC}"
echo ""
echo -e "  1. Verify runtime:"
echo -e "     ${BLUE}platform runtime list --api-url http://localhost:${CONTROL_PLANE_PORT}${NC}"
echo ""
echo -e "  2. Create and deploy an app:"
echo -e "     ${BLUE}platform apps create my-app --api-url http://localhost:${CONTROL_PLANE_PORT}${NC}"
echo -e "     ${BLUE}cd my-app && git init && git add . && git commit -m 'Initial'${NC}"
echo -e "     ${BLUE}git remote add platform ssh://git@localhost:2222/my-app.git${NC}"
echo -e "     ${BLUE}git push platform main${NC}"
echo ""
echo -e "  3. Stream build logs:"
echo -e "     ${BLUE}platform logs my-app <release-id> --tail --api-url http://localhost:${CONTROL_PLANE_PORT}${NC}"
echo ""
echo -e "  Logs:"
echo -e "    Control Plane → ${CP_LOG_FILE}"
echo -e "    Git Server    → ${GIT_LOG_FILE}"
echo ""
echo -e "  To tear down: ${YELLOW}make destroy-aws${NC} or ${YELLOW}./scripts/destroy-aws.sh${NC}"
echo ""
