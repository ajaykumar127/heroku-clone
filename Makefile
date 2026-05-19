.PHONY: deploy-aws destroy-aws status logs stop help

# ── One-click deployment ──────────────────────────────────────────────────────

## Deploy the full platform to AWS (EKS + Control Plane + CLI)
deploy-aws:
	@bash scripts/deploy-aws.sh

## Tear down all AWS resources
destroy-aws:
	@bash scripts/destroy-aws.sh

## Show runtime plane status
status:
	@platform runtime list 2>/dev/null || echo "Control Plane not running. Run: make deploy-aws"

## Tail Control Plane logs
logs:
	@tail -f .control-plane.log 2>/dev/null || echo "Control Plane not running."

## Stop the Control Plane and Git Server (keep AWS resources)
stop:
	@[ -f .control-plane.pid ] && kill $$(cat .control-plane.pid) 2>/dev/null && echo "Control Plane stopped" || true
	@[ -f .git-server.pid ]    && kill $$(cat .git-server.pid)    2>/dev/null && echo "Git Server stopped"    || true
	@rm -f .control-plane.pid .git-server.pid

## Build all Go binaries
build:
	@cd control-plane && go build -o bin/control-plane . && echo "✓ control-plane"
	@cd cli           && go build -o bin/platform .       && echo "✓ cli"
	@cd poc/git-server && go build -o bin/git-server .    && echo "✓ git-server"
	@cd runtime-plane/agent    && go build ./...          && echo "✓ runtime-agent"
	@cd runtime-plane/builder  && go build ./...          && echo "✓ builder"
	@cd runtime-plane/deployer && go build ./...          && echo "✓ deployer"

## Show this help
help:
	@echo ""
	@echo "  Platform — Cloud-Agnostic PaaS"
	@echo ""
	@echo "  make deploy-aws     One-click deploy to AWS (EKS + Control Plane)"
	@echo "  make destroy-aws    Tear down all AWS resources"
	@echo "  make status         Show registered runtime planes"
	@echo "  make logs           Tail Control Plane logs"
	@echo "  make stop           Stop local processes (keep AWS resources)"
	@echo "  make build          Build all Go binaries"
	@echo ""
	@echo "  Options (via environment variables):"
	@echo "    PLATFORM_AWS_REGION=us-west-2 make deploy-aws"
	@echo "    PLATFORM_CLUSTER_NAME=my-cluster make deploy-aws"
	@echo "    PLATFORM_NODE_TYPE=t3.large make deploy-aws"
	@echo ""
