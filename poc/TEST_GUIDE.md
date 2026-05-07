# Testing Guide - How to Verify Everything Works

This guide will help you test the Platform POC step-by-step.

## Prerequisites Check

Run these commands to verify what you have:

```bash
# Check Docker (REQUIRED)
docker --version
# Should show: Docker version 29.x.x or similar

# Check Docker Compose (REQUIRED)
docker compose version
# Should show: Docker Compose version 2.x.x

# Check if Docker is running
docker ps
# Should show list of containers (may be empty)
```

**If Docker is not installed**: [Install Docker Desktop](https://docs.docker.com/desktop/)

## Testing Methods

We'll test in 3 stages:

1. **Quick Test** (2 minutes) - Verify Docker Compose works
2. **API Test** (5 minutes) - Test all API endpoints
3. **Full Demo** (10 minutes) - Complete git-push workflow

---

## Stage 1: Quick Test (Docker Compose)

This tests the basic infrastructure without needing Go installed.

### Step 1: Start Services

```bash
cd ~/heroku-clone/poc

# Start all services
docker compose up -d

# Expected output:
# [+] Running 4/4
#  ✔ Container platform-postgres    Started
#  ✔ Container platform-registry    Started
#  ✔ Container platform-api         Started
#  ✔ Container platform-git         Started
```

### Step 2: Check Services Are Running

```bash
# Check all containers
docker compose ps

# Expected output (all "running"):
# NAME                 STATUS
# platform-api         Up
# platform-git         Up
# platform-postgres    Up
# platform-registry    Up
```

### Step 3: Check Logs

```bash
# View API server logs
docker compose logs api-server

# Should see:
# ✓ Database connected
# API Server listening on :8080
```

### Step 4: Test API Health

```bash
# Test health endpoint
curl http://localhost:8080/health

# Expected output:
# {"status":"ok"}
```

### Step 5: Open Dashboard

```bash
# macOS
open http://localhost:8080

# Linux
xdg-open http://localhost:8080

# Or just open in browser: http://localhost:8080
```

**You should see:**
- Beautiful purple gradient dashboard
- "Platform Dashboard" title
- Stats showing "0" total apps
- "No applications yet" message
- Quick start guide

**✅ If you see the dashboard, Stage 1 PASSED!**

---

## Stage 2: API Testing

Test all API endpoints to verify functionality.

### Test 1: Create an App

```bash
curl -X POST http://localhost:8080/v1/apps \
  -H "Content-Type: application/json" \
  -d '{"name":"test-app"}'
```

**Expected output:**
```json
{
  "id": "some-uuid",
  "name": "test-app",
  "git_url": "ssh://git@localhost:2222/test-app.git",
  "web_url": "http://test-app.localhost",
  "created_at": "2026-04-30T..."
}
```

**✅ Success**: You got a JSON response with app details  
**❌ Failure**: Error message or no response → Check logs with `docker compose logs api-server`

### Test 2: List Apps

```bash
curl http://localhost:8080/v1/apps | jq
```

**Expected output:**
```json
[
  {
    "id": "some-uuid",
    "name": "test-app",
    "git_url": "ssh://git@localhost:2222/test-app.git",
    "web_url": "http://test-app.localhost",
    "created_at": "2026-04-30T..."
  }
]
```

If you don't have `jq`, just remove the `| jq`:
```bash
curl http://localhost:8080/v1/apps
```

**✅ Success**: Array with your app(s)  
**❌ Failure**: Empty array `[]` → App wasn't created

### Test 3: Get Specific App

```bash
curl http://localhost:8080/v1/apps/test-app | jq
```

**Expected**: Same JSON as create response

### Test 4: Create Another App

```bash
curl -X POST http://localhost:8080/v1/apps \
  -H "Content-Type: application/json" \
  -d '{"name":"demo-app"}'
```

### Test 5: Refresh Dashboard

Open http://localhost:8080 in browser.

**You should now see:**
- "Total Apps: 2"
- Table with both apps listed
- Git URLs and Web URLs for each

**✅ If dashboard shows 2 apps, Stage 2 PASSED!**

---

## Stage 3: Full Demo Test

This simulates the complete git-push deployment workflow.

### Prerequisites for Stage 3

You need Git installed:
```bash
git --version
# Should show: git version 2.x.x
```

### Step 1: Create Demo App

```bash
curl -X POST http://localhost:8080/v1/apps \
  -H "Content-Type: application/json" \
  -d '{"name":"my-demo-app"}'
```

### Step 2: Prepare Example App

```bash
cd ~/heroku-clone/poc/example-app

# Initialize git (if not already)
git init

# Add files
git add .

# Commit
git commit -m "Initial commit"
```

### Step 3: Add Platform Remote

```bash
# Add the git remote
git remote add platform ssh://git@localhost:2222/my-demo-app.git

# Verify
git remote -v

# Expected output:
# platform  ssh://git@localhost:2222/my-demo-app.git (fetch)
# platform  ssh://git@localhost:2222/my-demo-app.git (push)
```

### Step 4: Test SSH Connection

```bash
# Test SSH to git server (will fail but shows it's listening)
ssh -p 2222 git@localhost

# Expected: Connection closes (that's OK!)
# The git server only accepts git commands, not interactive shells
```

### Step 5: Check Git Server Logs

Before pushing, check git server is ready:

```bash
docker compose logs git-server

# Should see:
# Git server starting on :2222
# Repos directory: /app/repos
```

### Step 6: Push to Platform (The Big Test!)

```bash
git push platform main
# or if your default branch is master:
git push platform master
```

**Expected output:**
```
Enumerating objects: 3, done.
Counting objects: 100% (3/3), done.
Writing objects: 100% (3/3), 1.23 KiB | 1.23 MiB/s, done.
Total 3 (delta 0), reused 0 (delta 0)
remote: -----> Receiving push for my-demo-app
remote:        ... -> ... (refs/heads/main)
remote: -----> Triggering build for commit abc123
remote: -----> Build queued successfully
remote: -----> Deploy in progress...
To ssh://localhost:2222/my-demo-app.git
 * [new branch]      main -> main
```

**✅ Success**: Push completes without errors  
**❌ Failure**: Connection refused → Git server not running

### Step 7: Verify Release Created

```bash
curl http://localhost:8080/v1/apps/my-demo-app/releases | jq
```

**Expected output:**
```json
[
  {
    "id": "release-uuid",
    "app_id": "app-uuid",
    "version": 1,
    "commit": "abc123",
    "status": "pending",
    "created_at": "2026-04-30T..."
  }
]
```

**✅ If you see a release, Stage 3 PASSED!**

---

## Testing Summary Checklist

After completing all stages, verify:

- [ ] Docker Compose services start: `docker compose ps`
- [ ] Dashboard loads: http://localhost:8080
- [ ] Health check works: `curl http://localhost:8080/health`
- [ ] Can create apps via API
- [ ] Apps appear in dashboard
- [ ] Can list apps: `curl http://localhost:8080/v1/apps`
- [ ] Git push completes successfully
- [ ] Release is created: `curl .../releases`
- [ ] Logs are visible: `docker compose logs`

**All checked? ✅ EVERYTHING WORKS!**

---

## Troubleshooting

### Issue: "Cannot connect to the Docker daemon"

**Solution:**
```bash
# Start Docker Desktop
# Or on Linux:
sudo systemctl start docker
```

### Issue: "Port 8080 already in use"

**Solution:**
```bash
# Find what's using the port
lsof -i :8080

# Kill the process
kill <PID>

# Or change the port in docker-compose.yml:
# ports:
#   - "8081:8080"  # Use 8081 instead
```

### Issue: "Services exit immediately"

**Check logs:**
```bash
docker compose logs api-server
docker compose logs git-server
docker compose logs postgres
```

Common causes:
- Database not ready → Wait 30 seconds and retry
- Port conflicts → Change ports in docker-compose.yml
- Build errors → Check Dockerfiles

### Issue: "Git push hangs"

**Solution:**
```bash
# Check git server is running
docker compose ps git-server

# Check git server logs
docker compose logs git-server

# Verify SSH port is open
nc -zv localhost 2222
```

### Issue: "Dashboard shows error loading applications"

**Solution:**
```bash
# Check API server logs
docker compose logs api-server

# Test API directly
curl http://localhost:8080/v1/apps

# Restart services
docker compose restart
```

### Issue: "Database connection errors"

**Solution:**
```bash
# Check PostgreSQL logs
docker compose logs postgres

# Restart PostgreSQL
docker compose restart postgres

# Wait for it to be ready (5-10 seconds)
sleep 10

# Restart API server
docker compose restart api-server
```

---

## Clean Up and Reset

If you want to start fresh:

```bash
# Stop all services
docker compose down

# Remove volumes (deletes all data)
docker compose down -v

# Remove everything including images
docker compose down -v --rmi all

# Start fresh
docker compose up -d
```

---

## Expected Test Results

After running all tests, you should have:

1. **4 running containers**
   - platform-postgres
   - platform-api
   - platform-git
   - platform-registry

2. **3+ apps created**
   - test-app
   - demo-app
   - my-demo-app

3. **1+ release**
   - From the git push

4. **Logs showing:**
   - API server listening
   - Git server listening
   - Database connected
   - HTTP requests logged

5. **Dashboard showing:**
   - All your apps
   - Release counts
   - "API Status: ✓ OK"

---

## Performance Tests (Optional)

### Test 1: Create 10 Apps

```bash
for i in {1..10}; do
  curl -X POST http://localhost:8080/v1/apps \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"app-$i\"}" &
done
wait

# Check dashboard - should show 10+ apps
```

### Test 2: Stress Test API

```bash
# Install apache bench (optional)
# macOS: brew install apache-bench
# Linux: sudo apt install apache2-utils

# Test 100 requests
ab -n 100 -c 10 http://localhost:8080/v1/apps
```

### Test 3: Check Memory Usage

```bash
docker stats --no-stream

# Should show reasonable memory usage:
# platform-api: ~50-100MB
# platform-git: ~30-50MB
# platform-postgres: ~50-150MB
```

---

## Success Criteria

✅ **POC is working if:**
- All services start without errors
- Dashboard loads and shows data
- API endpoints return correct responses
- Git push completes successfully
- Releases are created
- Logs are clean (no critical errors)

🎉 **You're ready to move forward!**

---

## Next: What to Test After This

Once basic functionality works:

1. **Test with different apps**
   - Python app
   - Ruby app
   - Custom Dockerfile

2. **Test scaling**
   - Multiple instances: `docker compose up -d --scale api-server=3`

3. **Test persistence**
   - Stop and start services
   - Verify data persists

4. **Test error handling**
   - Try creating duplicate app names
   - Push invalid git refs
   - Send malformed API requests

---

## Quick Test Script

Save this as `test.sh`:

```bash
#!/bin/bash
set -e

echo "🧪 Testing Platform POC..."

# Test 1: Health check
echo -n "Test 1: Health check... "
if curl -sf http://localhost:8080/health > /dev/null; then
  echo "✅ PASSED"
else
  echo "❌ FAILED"
  exit 1
fi

# Test 2: Create app
echo -n "Test 2: Create app... "
if curl -sf -X POST http://localhost:8080/v1/apps \
  -H "Content-Type: application/json" \
  -d '{"name":"auto-test-app"}' > /dev/null; then
  echo "✅ PASSED"
else
  echo "❌ FAILED"
  exit 1
fi

# Test 3: List apps
echo -n "Test 3: List apps... "
if curl -sf http://localhost:8080/v1/apps > /dev/null; then
  echo "✅ PASSED"
else
  echo "❌ FAILED"
  exit 1
fi

# Test 4: Get specific app
echo -n "Test 4: Get specific app... "
if curl -sf http://localhost:8080/v1/apps/auto-test-app > /dev/null; then
  echo "✅ PASSED"
else
  echo "❌ FAILED"
  exit 1
fi

echo ""
echo "🎉 All tests passed!"
echo ""
echo "Next: Open http://localhost:8080 in your browser"
```

Make it executable and run:
```bash
chmod +x test.sh
./test.sh
```

---

**Ready to test? Start with Stage 1!**

```bash
cd ~/heroku-clone/poc
docker compose up -d
```
