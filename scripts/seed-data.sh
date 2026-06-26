#!/usr/bin/env bash
# Spectrum Platform — Seed Data Script
# Generates sample telemetry data for testing

set -euo pipefail

GREEN="\033[0;32m"
NC="\033[0m"
ok() { echo -e "${GREEN}[OK]${NC} $*"; }

API_GW="${API_GW:-http://localhost:8080}"

echo "Seeding test data into Spectrum..."

# 1. Login to get token
token=$(curl -sf -X POST "$API_GW/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"spectrum"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$token" ]; then
  echo "Warning: could not get auth token, trying without..."
  AUTH_HEADER=""
else
  AUTH_HEADER="Authorization: Bearer $token"
  ok "Authenticated"
fi

# 2. Simulate a deploy webhook
curl -sf -X POST "$API_GW/api/v1/webhooks/deploy" \
  -H "Content-Type: application/json" \
  -d '{
    "service": "checkout",
    "version": "v2.3.1",
    "commitSha": "a1b2c3d4e5f6",
    "environment": "production",
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }' > /dev/null
ok "Deploy webhook sent (checkout v2.3.1)"

# 3. Simulate an alert
curl -sf -X POST "$API_GW/api/v1/webhooks/alert" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "High Error Rate",
    "service": "checkout",
    "severity": "critical",
    "description": "Error rate exceeded 5% for 5 minutes",
    "traceId": "0af7651916cd43dd8448eb211c80319c",
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }' > /dev/null
ok "Alert webhook sent (checkout critical)"

# 4. Simulate an incident report request via AI
curl -sf -X POST "$API_GW/api/v1/ai/diagnose" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER:+-H "$AUTH_HEADER"} \
  -d '{
    "incident": {
      "alertName": "High Error Rate",
      "serviceName": "checkout",
      "severity": "critical",
      "traceId": "0af7651916cd43dd8448eb211c80319c",
      "description": "Users reporting checkout failures",
      "startTime": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
    },
    "action": "diagnose",
    "telemetry": {
      "metrics": [
        {"name": "http_error_rate", "service": "checkout", "value": 12.5, "unit": "percent"},
        {"name": "http_latency_p95", "service": "checkout", "value": 2500, "unit": "ms"}
      ],
      "spans": [
        {"name": "process_payment", "serviceName": "checkout", "duration": "2.3s", "statusCode": 2}
      ],
      "logs": [
        {"timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'", "severity": "ERROR", "body": "payment gateway timeout after 5s", "traceId": "0af7651916cd43dd8448eb211c80319c"}
      ]
    }
  }' > /dev/null || echo "Warning: AI diagnosis requires LLM API key"
ok "Diagnosis request sent"

echo ""
ok "Seed data complete."
echo "  Open http://localhost:3000 for Grafana dashboards"
echo "  Open http://localhost:5173 for Admin UI"
