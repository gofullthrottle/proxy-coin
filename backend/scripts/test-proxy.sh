#!/usr/bin/env bash
# test-proxy.sh — E2E proxy integration test runner.
#
# Usage:
#   ./scripts/test-proxy.sh            # runs unit + integration tests
#   ./scripts/test-proxy.sh --unit     # unit tests only
#   ./scripts/test-proxy.sh --e2e      # full E2E (requires running stack)
#
# Environment variables (for --e2e mode):
#   PROXY_HOST  backend host (default: localhost)
#   PROXY_PORT  orchestrator port (default: 8080)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(dirname "$SCRIPT_DIR")"

MODE="${1:---unit}"

unit_tests() {
  echo "==> Running proxy unit and integration tests..."
  cd "$BACKEND_DIR"
  go test ./internal/proxy/ -v -run TestHandler -timeout 30s
  echo "==> Proxy tests passed."
}

e2e_tests() {
  local host="${PROXY_HOST:-localhost}"
  local port="${PROXY_PORT:-8080}"
  local base_url="http://${host}:${port}"

  echo "==> Running E2E proxy test against ${base_url}..."

  # Health-check: wait up to 10 seconds for the orchestrator to be ready.
  local attempts=0
  until curl -sf "${base_url}/healthz" > /dev/null 2>&1; do
    attempts=$((attempts + 1))
    if [ $attempts -ge 10 ]; then
      echo "ERROR: Orchestrator not reachable at ${base_url}/healthz after 10 seconds."
      echo "SOLUTION: Start the stack first with: cd infrastructure && docker compose up -d"
      exit 1
    fi
    echo "  Waiting for orchestrator... (${attempts}/10)"
    sleep 1
  done

  echo "  Orchestrator is up."

  # Send a test proxy request through the customer API.
  local response
  response=$(curl -sf -X POST "${base_url}/v1/proxy" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: test-key" \
    -d '{"url":"http://httpbin.org/get","method":"GET"}' \
    --max-time 10) || {
    echo "ERROR: Proxy request failed."
    echo "SOLUTION: Check orchestrator logs: docker compose logs orchestrator"
    exit 1
  }

  echo "  Response received:"
  echo "$response" | head -5

  echo "==> E2E proxy test passed."
}

case "$MODE" in
  --unit)   unit_tests ;;
  --e2e)    e2e_tests ;;
  *)
    unit_tests
    echo ""
    echo "==> Tip: run with --e2e for full stack testing (requires running infrastructure)."
    ;;
esac
