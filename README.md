# Spectrum — Observability Platform

Unified observability platform with AI-powered diagnostics, automatic correlation of metrics/traces/logs via OpenTelemetry, and native Grafana integration. Uses **Model Context Protocol (MCP)** to connect LLMs (GPT-4, Claude) for automated incident diagnosis, release analysis, and postmortem reports.

## Architecture

```
Apps (OTLP) → OTel Collector ─┬─→ Prometheus (metrics via remote write)
                               ├─→ Tempo     (traces via OTLP HTTP)
                               ├─→ Loki      (logs)
                               └─→ Kafka ──→ Correlation Engine (Go + Redis)
                                                      ↓
                                            AI Engine (Go + MCP + LLMs)
                                                      ↓
                                            API Gateway (Go) → Admin UI (React)
                                                      ↓
                                            Grafana Dashboards
```

> The OTel Collector fans out each signal type to both the storage backends (Prometheus/Tempo/Loki) **and** Kafka. The Correlation Engine consumes from Kafka and links traces + metrics + logs by `traceId` in Redis.

## Quick Start

```bash
# 1. Clone and enter
git clone https://github.com/IA-PlayGround/observabilidade-ai
cd observabilidade-ai

# 2. Set up environment
cp .env.example .env
# Edit .env to add OPENAI_API_KEY or ANTHROPIC_API_KEY

# 3. Start the platform
docker compose up -d --build

# 4. (Optional) Start demo apps
make demo

# 5. Wait ~60s for services to stabilize, then seed test data
./scripts/seed-data.sh
```

## URLs

| Service            | URL                      | Credentials               |
|--------------------|--------------------------|---------------------------|
| Grafana            | http://localhost:3000    | admin / spectrum          |
| Prometheus         | http://localhost:9090    | —                         |
| Tempo              | http://localhost:3200    | —                         |
| Loki               | http://localhost:3100    | —                         |
| API Gateway        | http://localhost:8080    | Token via /api/v1/auth/login |
| Correlation Engine | http://localhost:8081    | —                         |
| AI Engine          | http://localhost:8082    | —                         |
| Admin UI           | http://localhost:5173    | admin / spectrum          |
| OTel Collector     | grpc://localhost:4317    | — (OTLP gRPC)             |
|                    | http://localhost:4318    | — (OTLP HTTP)             |

## Make Commands

```bash
make help          # Show all commands
make up            # Start platform (builds images)
make down          # Stop platform and remove volumes (data is lost)
make restart       # down + up
make logs          # Tail all logs
make test          # Run all tests
make lint          # Lint all services
make setup-dev     # Install dev dependencies (go mod tidy + npm install)
make demo          # Start instrumented Go + Node.js demo apps
make demo-down     # Stop demo apps
make build         # Build all binaries locally
make clean         # Remove build artifacts
```

## Project Structure

```
observabilidade-ai/
├── docker-compose.yml              # Full platform stack
├── Makefile                        # Dev commands
├── .env.example                    # Environment variable reference (copy to .env)
├── infrastructure/
│   ├── docker/                     # Configs (Prometheus, Tempo, Loki, OTel, Grafana)
│   │   └── grafana/dashboards/     # Pre-built dashboards
│   ├── kubernetes/                 # Raw K8s manifests
│   ├── helm/spectrum/              # Helm chart
│   └── terraform/                  # AWS infra (EKS, MSK, ElastiCache)
├── services/
│   ├── correlation-engine/         # Go — Kafka → Redis correlation
│   ├── ai-engine/                  # Go — MCP client + LLM backends (OpenAI / Anthropic)
│   └── api-gateway/                # Go — Auth (JWT), proxy, webhooks
├── web/
│   └── admin-ui/                   # React + TypeScript admin panel
├── demo/
│   └── sample-apps/                # Go + Node.js apps with OTel auto-instrumentation
├── scripts/                        # setup.sh, seed-data.sh
└── PRD-Observabilidade-AI.md       # Full product requirements document
```

## Key Features

- **OpenTelemetry Native**: OTLP ingestion via collector (gRPC :4317, HTTP :4318), multi-language SDKs
- **Automatic Correlation**: Metrics + Traces + Logs linked by `traceId` in Redis with configurable time window
- **AI-Powered Diagnostics**: MCP integration with GPT-4 and Claude for root cause analysis
- **Incident Reports**: Auto-generated postmortems with timeline, evidence, and recommendations
- **Release Analysis**: Pre/post deploy metric comparison with regression detection
- **Grafana Dashboards**: Service Overview, Incident Report, Release Analysis (pre-provisioned)
- **Admin UI**: Source management, AI console, system health, correlation explorer

## API

All routes are on the API Gateway (`http://localhost:8080`). JWT token required on protected routes — obtain via `POST /api/v1/auth/login`.

### Auth (public)
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/auth/login` | Get JWT token (`{"username":"admin","password":"spectrum"}`) |

### Correlation (protected)
| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/api/v1/correlation/{traceId}` | Get correlated telemetry for a trace |
| `POST` | `/api/v1/correlation/query` | Query correlation by traceId (body) |
| `GET`  | `/api/v1/correlation/service/{service}` | List all correlations for a service |

### AI / MCP (protected)
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/ai/diagnose` | Run AI diagnosis on an incident |
| `POST` | `/api/v1/ai/report` | Generate full incident postmortem report |
| `POST` | `/api/v1/ai/release-analysis` | Analyze deploy impact (pre/post metrics) |
| `POST` | `/api/v1/ai/query` | Ad-hoc natural language query |

### Admin (protected)
| Method | Path | Description |
|--------|------|-------------|
| `GET`    | `/api/v1/admin/sources` | List configured data sources |
| `POST`   | `/api/v1/admin/sources` | Add a data source |
| `PUT`    | `/api/v1/admin/sources/{id}` | Update a data source |
| `DELETE` | `/api/v1/admin/sources/{id}` | Remove a data source |
| `GET`    | `/api/v1/admin/health` | System health (checks correlation + AI engine) |

### Webhooks (public)
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/webhooks/deploy` | CI/CD deploy hook — triggers release analysis |
| `POST` | `/api/v1/webhooks/alert` | Alert hook (AlertManager, PagerDuty) — triggers diagnosis |

### Grafana proxy (protected)
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/grafana/*` | Reverse proxy to Grafana |

## Instrumenting Your Apps

Point your OTLP exporters to the OTel Collector:

```bash
# gRPC (recommended)
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

# HTTP
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

See `demo/sample-apps/` for working Go and Node.js examples with auto-instrumentation.

## Requirements

- Docker 24+
- Go 1.22+ (for local development)
- Node.js 20+ (for UI development)
- LLM API key (OpenAI or Anthropic) for AI features

## Production Deployment

```bash
# Terraform (infrastructure)
cd infrastructure/terraform
terraform init
terraform apply

# Helm (platform)
helm upgrade --install spectrum infrastructure/helm/spectrum \
  --namespace observability --create-namespace
```

## License

Internal — All rights reserved.
