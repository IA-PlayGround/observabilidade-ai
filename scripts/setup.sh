#!/usr/bin/env bash
# Spectrum Platform — Setup Script
# Bootstraps the entire platform for development

set -euo pipefail

RED="\033[0;31m"
GREEN="\033[0;32m"
YELLOW="\033[0;33m"
BLUE="\033[0;34m"
NC="\033[0m"

info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
fail()  { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

echo ""
echo "========================================="
echo "  Spectrum Platform — Development Setup"
echo "========================================="
echo ""

# 1. Check prerequisites
info "Checking prerequisites..."

if ! command -v docker &>/dev/null; then fail "docker is required but not installed"; fi
if ! command -v go &>/dev/null; then warn "go not found — Go services won't build locally"; fi
if ! command -v node &>/dev/null; then warn "node not found — UI won't build locally"; fi

ok "Prerequisites checked"

# 2. Setup .env
if [ ! -f .env ]; then
  info "Creating .env from .env.example..."
  cp .env.example .env
  ok ".env created — review and set LLM API keys"
else
  ok ".env already exists"
fi

# 3. Start infrastructure
info "Starting platform infrastructure..."
docker compose up -d --wait 2>&1 | grep -v "Warning"

info "Waiting for services to stabilize (30s)..."
sleep 30

# 4. Verify services
info "Verifying service health..."
SERVICES=(
  "http://localhost:9090/-/healthy"    # Prometheus
  "http://localhost:3200/ready"        # Tempo
  "http://localhost:3100/ready"        # Loki
  "http://localhost:3000/api/health"    # Grafana
  "http://localhost:8081/health"       # Correlation
  "http://localhost:8082/health"       # AI
  "http://localhost:8080/health"       # Gateway
)

for url in "${SERVICES[@]}"; do
  if curl -sf "$url" > /dev/null 2>&1; then
    ok "$url"
  else
    warn "$url — not ready yet"
  fi
done

# 5. Print info
echo ""
echo "========================================="
echo "  Platform URLs"
echo "========================================="
echo ""
echo "  Grafana:          http://localhost:3000  (admin / spectrum)"
echo "  Prometheus:       http://localhost:9090"
echo "  Tempo:            http://localhost:3200"
echo "  Loki:             http://localhost:3100"
echo "  API Gateway:      http://localhost:8080"
echo "  Correlation API:  http://localhost:8081"
echo "  AI Engine:         http://localhost:8082"
echo "  Admin UI:         http://localhost:5173"
echo ""
echo "  Make commands:"
echo "    make setup-dev    — install dev dependencies"
echo "    make up           — start platform"
echo "    make down         — stop platform"
echo "    make demo         — start demo apps"
echo "    make logs         — tail all logs"
echo ""
ok "Setup complete! 🚀"
