.PHONY: help up down build lint test clean setup-dev setup-k8s demo

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Docker Compose ──────────────────────────────────────────────────
up: ## Start full platform stack
	docker compose up -d --build

down: ## Stop all services
	docker compose down -v

restart: down up ## Restart everything

logs: ## Tail all logs
	docker compose logs -f

logs-correlation: ## Correlation engine logs
	docker compose logs -f correlation-engine

logs-ai: ## AI engine logs
	docker compose logs -f ai-engine

# ── Build ───────────────────────────────────────────────────────────
build-correlation: ## Build correlation engine
	cd services/correlation-engine && go build -o ../../bin/correlation-engine ./cmd

build-ai: ## Build AI engine
	cd services/ai-engine && go build -o ../../bin/ai-engine ./cmd

build-gateway: ## Build API gateway
	cd services/api-gateway && go build -o ../../bin/api-gateway ./cmd

build-ui: ## Build admin UI
	cd web/admin-ui && npm run build

build: build-correlation build-ai build-gateway build-ui ## Build all services

# ── Lint & Test ─────────────────────────────────────────────────────
lint: ## Lint all services
	cd services/correlation-engine && golangci-lint run ./...
	cd services/ai-engine && golangci-lint run ./...
	cd services/api-gateway && golangci-lint run ./...
	cd web/admin-ui && npm run lint

test: ## Run all tests
	cd services/correlation-engine && go test -race -v ./...
	cd services/ai-engine && go test -race -v ./...
	cd services/api-gateway && go test -race -v ./...
	cd web/admin-ui && npm test -- --watchAll=false

# ── Development ─────────────────────────────────────────────────────
setup-dev: ## Install dev dependencies
	cd services/correlation-engine && go mod tidy
	cd services/ai-engine && go mod tidy
	cd services/api-gateway && go mod tidy
	cd web/admin-ui && npm install

dev-correlation: ## Run correlation engine locally
	cd services/correlation-engine && go run ./cmd

dev-ai: ## Run AI engine locally
	cd services/ai-engine && go run ./cmd

dev-gateway: ## Run API gateway locally
	cd services/api-gateway && go run ./cmd

dev-ui: ## Run admin UI dev server
	cd web/admin-ui && npm run dev

# ── Kubernetes ──────────────────────────────────────────────────────
setup-k8s: ## Apply K8s manifests
	kubectl apply -f infrastructure/kubernetes/namespace.yaml
	kubectl apply -f infrastructure/kubernetes/

helm-install: ## Install via Helm
	helm upgrade --install spectrum infrastructure/helm/spectrum \
		--namespace observability --create-namespace

helm-uninstall: ## Uninstall Helm release
	helm uninstall spectrum -n observability

# ── Terraform ───────────────────────────────────────────────────────
tf-init: ## Terraform init
	cd infrastructure/terraform && terraform init

tf-plan: ## Terraform plan
	cd infrastructure/terraform && terraform plan

tf-apply: ## Terraform apply
	cd infrastructure/terraform && terraform apply -auto-approve

# ── Demo ────────────────────────────────────────────────────────────
demo: ## Start demo apps
	docker compose -f demo/sample-apps/docker-compose.yml up -d --build

demo-down: ## Stop demo apps
	docker compose -f demo/sample-apps/docker-compose.yml down

# ── Clean ───────────────────────────────────────────────────────────
clean: ## Remove build artifacts
	rm -rf bin/
	cd services/correlation-engine && go clean
	cd services/ai-engine && go clean
	cd services/api-gateway && go clean
