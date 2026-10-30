#!/bin/bash
if [ -z "${IN_NIX_SHELL:-}" ] && [ "${CHOIR_ALLOW_HOST_LOCAL_SERVICES:-0}" != "1" ]; then
  cat >&2 <<'EOF'
start-services.sh must run inside the repo dev shell:

  nix develop -c ./start-services.sh

Set CHOIR_ALLOW_HOST_LOCAL_SERVICES=1 only for a short diagnostic run when
investigating host-local harness configuration.
EOF
  exit 1
fi

CHOIR_AUTH_SIGNING_KEY_PATH="${CHOIR_AUTH_SIGNING_KEY_PATH:-/tmp/go-choir-m2/auth-signing-key}"
mkdir -p "$(dirname "$CHOIR_AUTH_SIGNING_KEY_PATH")"
if [ ! -f "$CHOIR_AUTH_SIGNING_KEY_PATH" ]; then
  ssh-keygen -q -t ed25519 -N "" -f "$CHOIR_AUTH_SIGNING_KEY_PATH" >/dev/null
fi
if [ ! -f "${CHOIR_AUTH_SIGNING_KEY_PATH}.pub" ]; then
  ssh-keygen -y -f "$CHOIR_AUTH_SIGNING_KEY_PATH" > "${CHOIR_AUTH_SIGNING_KEY_PATH}.pub"
fi

wait_for_url() {
  local url="$1"
  local label="$2"
  local attempts="${3:-60}"
  local delay="${4:-1}"
  for _ in $(seq 1 "$attempts"); do
    if curl -sf "$url" >/dev/null; then
      return 0
    fi
    sleep "$delay"
  done
  echo "$label failed"
  return 1
}

export AUTH_JWT_PRIVATE_KEY_PATH="$CHOIR_AUTH_SIGNING_KEY_PATH"
export PROXY_AUTH_PUBLIC_KEY_PATH="${CHOIR_AUTH_SIGNING_KEY_PATH}.pub"
export AUTH_PORT=8081 AUTH_RP_ID="localhost" AUTH_RP_ORIGINS="http://localhost:4173" AUTH_ACCESS_TOKEN_TTL="5m" AUTH_REFRESH_TOKEN_TTL="720h" AUTH_COOKIE_SECURE="false"
nohup go run ./cmd/auth > auth.log 2>&1 &
AUTH_PID=$!
[ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$AUTH_PID"

export CHATGPT_AUTH_PATH="${CHATGPT_AUTH_PATH:-$HOME/.codex/auth.json}"
export GATEWAY_CHATGPT_MODELS="${GATEWAY_CHATGPT_MODELS:-gpt-6-luna}"
export GATEWAY_CHATGPT_REASONING_EFFORT="${GATEWAY_CHATGPT_REASONING_EFFORT:-low}"
export GATEWAY_PORT=8084
nohup go run ./cmd/gateway > gateway.log 2>&1 &
GATEWAY_PID=$!
[ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$GATEWAY_PID"
wait_for_url http://127.0.0.1:8084/health gateway || exit 1

export VMCTL_PORT="${VMCTL_PORT:-8083}"
export VMCTL_AUTOPUTER_URL_BASE="${VMCTL_AUTOPUTER_URL_BASE:-http://127.0.0.1:8085}"
export VMCTL_GATEWAY_URL="${VMCTL_GATEWAY_URL:-http://127.0.0.1:8084}"
export VMCTL_IDLE_TIMEOUT="${VMCTL_IDLE_TIMEOUT:-30m}"
export VMCTL_PRIMARY_KEEPALIVE_MODE="${VMCTL_PRIMARY_KEEPALIVE_MODE:-under-capacity}"
nohup go run ./cmd/vmctl > vmctl.log 2>&1 &
VMCTL_PID=$!
[ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$VMCTL_PID"
wait_for_url "http://127.0.0.1:${VMCTL_PORT}/health" vmctl || exit 1

PLATFORM_PG_DIR=""
CORPUSD_PID=""
if [ "${CHOIR_ENABLE_CORPUSD:-1}" = "1" ]; then
  # Local PostgreSQL for corpusd (the host data system: databases control and
  # graph; the repo dev shell provides postgresql_17). Trust auth on loopback
  # only, a throwaway cluster under /tmp: dev data, never staging.
  INITDB_BIN="${INITDB_BIN:-$(command -v initdb || true)}"
  PG_CTL_BIN="${PG_CTL_BIN:-$(command -v pg_ctl || true)}"
  if [ -z "$INITDB_BIN" ] || [ -z "$PG_CTL_BIN" ]; then
    echo "corpusd startup requires PostgreSQL (initdb, pg_ctl); enter nix develop or set INITDB_BIN and PG_CTL_BIN"
    exit 1
  fi
  export CORPUSD_PORT="${CORPUSD_PORT:-8086}"
  export PLATFORM_PG_PORT="${PLATFORM_PG_PORT:-15432}"
  PLATFORM_PG_DIR="${PLATFORM_PG_DIR:-/tmp/go-choir-m2/postgres}"
  export CORPUSD_ARTIFACTS_ROOT="${CORPUSD_ARTIFACTS_ROOT:-/tmp/go-choir-m2/platform-artifacts}"
  export CHOIR_PG_CONTROL_DSN="${CHOIR_PG_CONTROL_DSN:-postgres://choir@127.0.0.1:${PLATFORM_PG_PORT}/control?sslmode=disable}"
  export CHOIR_PG_GRAPH_DSN="${CHOIR_PG_GRAPH_DSN:-postgres://choir@127.0.0.1:${PLATFORM_PG_PORT}/graph?sslmode=disable}"
  mkdir -p "$PLATFORM_PG_DIR" "$CORPUSD_ARTIFACTS_ROOT"
  if [ ! -f "$PLATFORM_PG_DIR/data/PG_VERSION" ]; then
    "$INITDB_BIN" -D "$PLATFORM_PG_DIR/data" -U choir -A trust -E UTF8 --locale=C >/dev/null || exit 1
  fi
  if ! "$PG_CTL_BIN" -D "$PLATFORM_PG_DIR/data" status >/dev/null 2>&1; then
    "$PG_CTL_BIN" -D "$PLATFORM_PG_DIR/data" -l "$PLATFORM_PG_DIR/postgres.log" -w \
      -o "-p $PLATFORM_PG_PORT -c listen_addresses=127.0.0.1 -k $PLATFORM_PG_DIR" start >/dev/null || exit 1
  fi
  for db in control graph; do
    if ! psql -h 127.0.0.1 -p "$PLATFORM_PG_PORT" -U choir -d postgres -Atc "select 1 from pg_database where datname='$db'" | grep -q 1; then
      psql -h 127.0.0.1 -p "$PLATFORM_PG_PORT" -U choir -d postgres -qc "create database $db" || exit 1
    fi
  done

  nohup go run ./cmd/corpusd > corpusd.log 2>&1 &
  CORPUSD_PID=$!
  [ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$CORPUSD_PID"
  wait_for_url "http://127.0.0.1:${CORPUSD_PORT}/health" corpusd 90 1 || exit 1
fi

RUNTIME_GATEWAY_TOKEN="$(curl -sf -X POST \
  http://127.0.0.1:8084/provider/v1/credentials/issue \
  -H "Content-Type: application/json" \
  -H "X-Internal-Caller: true" \
  -d '{"computer_id":"autoputer-dev"}' | jq -r .RawToken)"
export RUNTIME_GATEWAY_TOKEN
export RUNTIME_GATEWAY_URL="http://127.0.0.1:8084"
export RUNTIME_LLM_PROVIDER="${RUNTIME_LLM_PROVIDER:-chatgpt}"
export RUNTIME_LLM_MODEL="${RUNTIME_LLM_MODEL:-gpt-6-luna}"
export RUNTIME_LLM_REASONING_EFFORT="${RUNTIME_LLM_REASONING_EFFORT:-low}"
export RUNTIME_VMCTL_URL="${RUNTIME_VMCTL_URL:-http://127.0.0.1:${VMCTL_PORT}}"
export AUTOPUTER_PORT="${AUTOPUTER_PORT:-8085}" AUTOPUTER_ID="${AUTOPUTER_ID:-autoputer-dev}" RUNTIME_ENABLE_TEST_APIS="${RUNTIME_ENABLE_TEST_APIS:-0}"
# The local proxy reaches this autoputer over 127.0.0.1, so loopback is the
# host side here. A computer's guest image never sets this: inside a VM,
# loopback is every in-guest process (internal/server/guest_owner.go).
export CHOIR_GUEST_TRUST_LOOPBACK=1
nohup go run ./cmd/autoputer > autoputer.log 2>&1 &
AUTOPUTER_PID=$!
[ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$AUTOPUTER_PID"
wait_for_url http://127.0.0.1:8081/health auth || exit 1
wait_for_url http://127.0.0.1:8085/health autoputer || exit 1

export PROXY_PORT=8082 PROXY_AUTOPUTER_URL="http://127.0.0.1:8085" PROXY_VMCTL_URL="${PROXY_VMCTL_URL:-http://127.0.0.1:${VMCTL_PORT}}" PROXY_CORPUSD_URL="${PROXY_CORPUSD_URL:-http://127.0.0.1:${CORPUSD_PORT:-8086}}" PROXY_VMCTL_TIMEOUT="${PROXY_VMCTL_TIMEOUT:-60s}"
nohup go run ./cmd/proxy > proxy.log 2>&1 &
PROXY_PID=$!
[ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$PROXY_PID"
wait_for_url http://127.0.0.1:8082/health proxy || exit 1

(cd frontend && nohup pnpm dev --host localhost --port 4173 > frontend.log 2>&1) &
FRONTEND_PID=$!
[ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$FRONTEND_PID"
wait_for_url http://localhost:4173 frontend || exit 1
echo "Services started successfully"
if [ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ]; then
  cleanup_services() {
    kill "$FRONTEND_PID" "$PROXY_PID" "$AUTOPUTER_PID" "$CORPUSD_PID" "$VMCTL_PID" "$GATEWAY_PID" "$AUTH_PID" 2>/dev/null || true
    if [ -n "$PLATFORM_PG_DIR" ]; then
      "$PG_CTL_BIN" -D "$PLATFORM_PG_DIR/data" -m fast stop >/dev/null 2>&1 || true
    fi
  }
  trap cleanup_services INT TERM EXIT
  wait
fi
