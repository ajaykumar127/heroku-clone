.PHONY: help build start stop clean logs test deploy

help: ## Show this help message
	@echo "Platform POC - Makefile commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build all components
	@echo "=====> Building components..."
	@cd api-server && go mod tidy && go build -o ../bin/api-server .
	@cd git-server && go mod tidy && go build -o ../bin/git-server .
	@echo "✓ Build complete"

start: build ## Start all services
	@echo "=====> Starting services..."
	@mkdir -p logs repos
	@./bin/api-server > logs/api-server.log 2>&1 &
	@echo $$! > logs/api-server.pid
	@sleep 2
	@./bin/git-server > logs/git-server.log 2>&1 &
	@echo $$! > logs/git-server.pid
	@sleep 1
	@echo "✓ Services started"
	@echo ""
	@echo "API Server:  http://localhost:8080"
	@echo "Git Server:  ssh://localhost:2222"
	@echo ""
	@echo "Logs:"
	@echo "  tail -f logs/api-server.log"
	@echo "  tail -f logs/git-server.log"
	@echo ""
	@echo "PIDs saved in logs/*.pid"

stop: ## Stop all services
	@echo "=====> Stopping services..."
	@if [ -f logs/api-server.pid ]; then kill `cat logs/api-server.pid` 2>/dev/null || true; rm logs/api-server.pid; fi
	@if [ -f logs/git-server.pid ]; then kill `cat logs/git-server.pid` 2>/dev/null || true; rm logs/git-server.pid; fi
	@echo "✓ Services stopped"

restart: stop start ## Restart all services

status: ## Show service status
	@echo "=====> Service Status"
	@echo ""
	@if [ -f logs/api-server.pid ] && ps -p `cat logs/api-server.pid` > /dev/null; then \
		echo "✓ API Server:  RUNNING (PID: `cat logs/api-server.pid`)"; \
	else \
		echo "✗ API Server:  STOPPED"; \
	fi
	@if [ -f logs/git-server.pid ] && ps -p `cat logs/git-server.pid` > /dev/null; then \
		echo "✓ Git Server:  RUNNING (PID: `cat logs/git-server.pid`)"; \
	else \
		echo "✗ Git Server:  STOPPED"; \
	fi

logs: ## Tail logs from all services
	@tail -f logs/*.log

logs-api: ## Tail API server logs
	@tail -f logs/api-server.log

logs-git: ## Tail Git server logs
	@tail -f logs/git-server.log

clean: stop ## Clean all build artifacts and data
	@echo "=====> Cleaning..."
	@rm -rf bin/* logs/* repos/* platform.db
	@echo "✓ Clean complete"

test: ## Run tests
	@echo "=====> Running tests..."
	@cd api-server && go test -v ./...
	@cd git-server && go test -v ./...
	@echo "✓ Tests complete"

create-app: ## Create a new app (usage: make create-app NAME=myapp)
	@if [ -z "$(NAME)" ]; then echo "Usage: make create-app NAME=myapp"; exit 1; fi
	@echo "=====> Creating app: $(NAME)"
	@curl -s -X POST http://localhost:8080/v1/apps \
		-H "Content-Type: application/json" \
		-d '{"name":"$(NAME)"}' | jq .
	@echo ""
	@echo "To deploy:"
	@echo "  cd example-app"
	@echo "  git init && git add . && git commit -m 'Initial'"
	@echo "  git remote add platform ssh://git@localhost:2222/$(NAME).git"
	@echo "  git push platform main"

list-apps: ## List all apps
	@curl -s http://localhost:8080/v1/apps | jq .

demo: ## Run complete demo
	@echo "=====> Running demo..."
	@make start
	@sleep 3
	@echo ""
	@echo "=====> Creating demo app..."
	@make create-app NAME=demo-app
	@sleep 2
	@echo ""
	@echo "=====> Demo complete!"
	@echo ""
	@echo "Next: Deploy the example app:"
	@echo "  cd example-app"
	@echo "  git init && git add . && git commit -m 'Initial commit'"
	@echo "  git remote add platform ssh://git@localhost:2222/demo-app.git"
	@echo "  git push platform main"

docker-build: ## Build Docker images
	@echo "=====> Building Docker images..."
	@docker build -t platform/api-server:latest -f docker/api-server.Dockerfile .
	@docker build -t platform/git-server:latest -f docker/git-server.Dockerfile .
	@echo "✓ Docker images built"

docker-up: ## Start services with Docker Compose
	@docker-compose up -d
	@echo "✓ Services started with Docker Compose"

docker-down: ## Stop Docker Compose services
	@docker-compose down
	@echo "✓ Services stopped"

docker-logs: ## View Docker Compose logs
	@docker-compose logs -f
