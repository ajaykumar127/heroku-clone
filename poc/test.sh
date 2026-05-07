#!/bin/bash
# Quick automated test script for Platform POC

set -e

echo "🧪 Testing Platform POC..."
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track results
PASSED=0
FAILED=0

# Test function
test_api() {
  local name=$1
  local cmd=$2

  echo -n "Testing: $name ... "

  if eval "$cmd" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASSED${NC}"
    PASSED=$((PASSED + 1))
    return 0
  else
    echo -e "${RED}❌ FAILED${NC}"
    FAILED=$((FAILED + 1))
    return 1
  fi
}

# Check prerequisites
echo "Checking prerequisites..."
if ! command -v docker &> /dev/null; then
  echo -e "${RED}❌ Docker is not installed${NC}"
  echo "Please install Docker Desktop: https://docs.docker.com/desktop/"
  exit 1
fi

if ! command -v curl &> /dev/null; then
  echo -e "${RED}❌ curl is not installed${NC}"
  exit 1
fi

echo -e "${GREEN}✅ Prerequisites met${NC}"
echo ""

# Check if services are running
echo "Checking if services are running..."
if ! docker compose ps | grep -q "platform-api"; then
  echo -e "${YELLOW}⚠️  Services not running. Starting them now...${NC}"
  echo "This may take 30-60 seconds..."
  docker compose up -d
  echo "Waiting for services to be ready..."
  sleep 15
fi

# Test 1: Docker services
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Stage 1: Infrastructure Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

test_api "API container running" "docker compose ps | grep -q 'platform-api.*Up'"
test_api "Git server container running" "docker compose ps | grep -q 'platform-git.*Up'"
test_api "PostgreSQL container running" "docker compose ps | grep -q 'platform-postgres.*Up'"

# Test 2: API endpoints
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Stage 2: API Endpoint Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

test_api "Health check" "curl -sf http://localhost:8080/health"
test_api "List apps (empty)" "curl -sf http://localhost:8080/v1/apps"
test_api "Dashboard loads" "curl -sf http://localhost:8080 | grep -q 'Platform Dashboard'"

# Test 3: Create and manage apps
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Stage 3: App Management Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

test_api "Create app 'test-app-1'" "curl -sf -X POST http://localhost:8080/v1/apps -H 'Content-Type: application/json' -d '{\"name\":\"test-app-1\"}'"
test_api "Get specific app" "curl -sf http://localhost:8080/v1/apps/test-app-1"
test_api "List apps (with data)" "curl -sf http://localhost:8080/v1/apps | grep -q 'test-app-1'"
test_api "Create app 'test-app-2'" "curl -sf -X POST http://localhost:8080/v1/apps -H 'Content-Type: application/json' -d '{\"name\":\"test-app-2\"}'"
test_api "List releases (empty)" "curl -sf http://localhost:8080/v1/apps/test-app-1/releases"

# Test 4: Error handling
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Stage 4: Error Handling Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

test_api "404 for non-existent app" "curl -sf http://localhost:8080/v1/apps/does-not-exist || true"
test_api "Duplicate app name fails" "! curl -sf -X POST http://localhost:8080/v1/apps -H 'Content-Type: application/json' -d '{\"name\":\"test-app-1\"}'"

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${GREEN}✅ Passed: $PASSED${NC}"
echo -e "${RED}❌ Failed: $FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
  echo -e "${GREEN}🎉 All tests passed!${NC}"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Next Steps:"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "1. View the dashboard:"
  echo "   open http://localhost:8080"
  echo ""
  echo "2. Check your apps:"
  echo "   curl http://localhost:8080/v1/apps | jq"
  echo ""
  echo "3. View logs:"
  echo "   docker compose logs -f"
  echo ""
  echo "4. Test git push deployment:"
  echo "   cd example-app"
  echo "   git init && git add . && git commit -m 'Initial'"
  echo "   git remote add platform ssh://git@localhost:2222/test-app-1.git"
  echo "   git push platform main"
  echo ""
  exit 0
else
  echo -e "${RED}⚠️  Some tests failed${NC}"
  echo ""
  echo "Troubleshooting:"
  echo "  1. Check service logs: docker compose logs"
  echo "  2. Restart services: docker compose restart"
  echo "  3. See TEST_GUIDE.md for detailed troubleshooting"
  echo ""
  exit 1
fi
