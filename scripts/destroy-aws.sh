#!/usr/bin/env bash
# =============================================================================
# Platform — Tear down the AWS Runtime Plane
# Usage: ./scripts/destroy-aws.sh [--region us-east-1] [--cluster-name platform-runtime]
# =============================================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'

log()     { echo -e "${BLUE}[platform]${NC} $*"; }
success() { echo -e "${GREEN}[✓]${NC} $*"; }
warn()    { echo -e "${YELLOW}[!]${NC} $*"; }
fail()    { echo -e "${RED}[✗]${NC} $*"; exit 1; }

REGION="${PLATFORM_AWS_REGION:-us-east-1}"
CLUSTER_NAME="${PLATFORM_CLUSTER_NAME:-platform-runtime}"

while [[ $# -gt 0 ]]; do
  case $1 in
    --region)       REGION="$2";       shift 2 ;;
    --cluster-name) CLUSTER_NAME="$2"; shift 2 ;;
    *) fail "Unknown argument: $1" ;;
  esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo ""
echo -e "${YELLOW}${BOLD}  Platform — Destroy AWS Runtime Plane${NC}"
echo -e "  Region: ${REGION} · Cluster: ${CLUSTER_NAME}"
echo ""
warn "This will destroy the EKS cluster, ECR registry, VPC, and all related AWS resources."
echo -n "  Are you sure? Type 'yes' to continue: "
read -r CONFIRM
[[ "$CONFIRM" == "yes" ]] || { log "Aborted."; exit 0; }

# Stop Control Plane and Git Server
CP_PID_FILE="${REPO_ROOT}/.control-plane.pid"
GIT_PID_FILE="${REPO_ROOT}/.git-server.pid"

if [[ -f "$CP_PID_FILE" ]]; then
  log "Stopping Control Plane..."
  kill "$(cat "$CP_PID_FILE")" 2>/dev/null && success "Control Plane stopped" || true
  rm -f "$CP_PID_FILE"
fi

if [[ -f "$GIT_PID_FILE" ]]; then
  log "Stopping Git Server..."
  kill "$(cat "$GIT_PID_FILE")" 2>/dev/null && success "Git Server stopped" || true
  rm -f "$GIT_PID_FILE"
fi

# Terraform destroy
cd "${REPO_ROOT}/infra/aws"
log "Running terraform destroy..."
terraform destroy \
  -auto-approve \
  -input=false \
  -var="region=${REGION}" \
  -var="cluster_name=${CLUSTER_NAME}" \
  -var="control_plane_url=http://localhost:8080" \
  2>&1 | grep -E "(aws_|Destroy complete|Error)" | sed 's/^/  /'

success "AWS infrastructure destroyed"

# Clean up state
rm -f "${REPO_ROOT}/.deploy-state" \
      "${REPO_ROOT}/control-plane/platform.db" \
      "${REPO_ROOT}/.control-plane.log" \
      "${REPO_ROOT}/.git-server.log"

echo ""
echo -e "${GREEN}${BOLD}  Teardown complete. All AWS resources deleted.${NC}"
echo ""
