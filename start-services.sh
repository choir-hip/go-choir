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
export GATEWAY_CHATGPT_MODELS="${GATEWAY_CHATGPT_MODELS:-gpt-5.5,gpt-5.4,gpt-5.4-mini}"
export GATEWAY_CHATGPT_REASONING_EFFORT="${GATEWAY_CHATGPT_REASONING_EFFORT:-low}"
export GATEWAY_PORT=8084
nohup go run ./cmd/gateway > gateway.log 2>&1 &
GATEWAY_PID=$!
[ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$GATEWAY_PID"
wait_for_url http://127.0.0.1:8084/health gateway || exit 1

export VMCTL_PORT="${VMCTL_PORT:-8083}"
export VMCTL_SANDBOX_URL_BASE="${VMCTL_SANDBOX_URL_BASE:-http://127.0.0.1:8085}"
export VMCTL_GATEWAY_URL="${VMCTL_GATEWAY_URL:-http://127.0.0.1:8084}"
export VMCTL_IDLE_TIMEOUT="${VMCTL_IDLE_TIMEOUT:-30m}"
export VMCTL_PRIMARY_KEEPALIVE_MODE="${VMCTL_PRIMARY_KEEPALIVE_MODE:-under-capacity}"
nohup go run ./cmd/vmctl > vmctl.log 2>&1 &
VMCTL_PID=$!
[ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$VMCTL_PID"
wait_for_url "http://127.0.0.1:${VMCTL_PORT}/health" vmctl || exit 1

PLATFORM_DOLT_PID=""
CORPUSD_PID=""
if [ "${CHOIR_ENABLE_CORPUSD:-1}" = "1" ]; then
  DOLT_BIN="${DOLT_BIN:-$(command -v dolt || true)}"
  if [ -z "$DOLT_BIN" ] && command -v nix >/dev/null 2>&1; then
    DOLT_OUT="$(nix build --no-link --print-out-paths nixpkgs#dolt 2>/dev/null | head -n 1 || true)"
    if [ -n "$DOLT_OUT" ] && [ -x "$DOLT_OUT/bin/dolt" ]; then
      DOLT_BIN="$DOLT_OUT/bin/dolt"
    fi
  fi
  if [ -z "$DOLT_BIN" ]; then
    echo "corpusd startup requires dolt; enter nix develop or set DOLT_BIN=/path/to/dolt"
    exit 1
  fi
  export CORPUSD_PORT="${CORPUSD_PORT:-8086}"
  export PLATFORM_DOLT_PORT="${PLATFORM_DOLT_PORT:-13306}"
  export PLATFORM_DOLT_DIR="${PLATFORM_DOLT_DIR:-/tmp/go-choir-m2/platform-dolt}"
  export PLATFORM_DOLT_DB_DIR="${PLATFORM_DOLT_DB_DIR:-${PLATFORM_DOLT_DIR}/platform}"
  export CORPUSD_ARTIFACTS_ROOT="${CORPUSD_ARTIFACTS_ROOT:-/tmp/go-choir-m2/platform-artifacts}"
  export CORPUSD_DOLT_DSN="${CORPUSD_DOLT_DSN:-root@tcp(127.0.0.1:${PLATFORM_DOLT_PORT})/platform?parseTime=true&multiStatements=true&clientFoundRows=true}"
  mkdir -p "$PLATFORM_DOLT_DB_DIR" "$CORPUSD_ARTIFACTS_ROOT"
  (
    export HOME="$PLATFORM_DOLT_DIR"
    cd "$PLATFORM_DOLT_DB_DIR"
    "$DOLT_BIN" config --global --add user.name "Choir Platform" >/dev/null 2>&1 || true
    "$DOLT_BIN" config --global --add user.email "platform@choir.local" >/dev/null 2>&1 || true
    if [ ! -d .dolt ]; then
      "$DOLT_BIN" init >/dev/null
    fi
    exec "$DOLT_BIN" sql-server --host 127.0.0.1 --port "$PLATFORM_DOLT_PORT"
  ) > platform-dolt.log 2>&1 &
  PLATFORM_DOLT_PID=$!
  [ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$PLATFORM_DOLT_PID"

  nohup go run ./cmd/corpusd > corpusd.log 2>&1 &
  CORPUSD_PID=$!
  [ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$CORPUSD_PID"
  wait_for_url "http://127.0.0.1:${CORPUSD_PORT}/health" corpusd 90 1 || exit 1
fi

RUNTIME_GATEWAY_TOKEN="$(curl -sf -X POST \
  http://127.0.0.1:8084/provider/v1/credentials/issue \
  -H "Content-Type: application/json" \
  -H "X-Internal-Caller: true" \
  -d '{"sandbox_id":"sandbox-dev"}' | jq -r .RawToken)"
export RUNTIME_GATEWAY_TOKEN
export RUNTIME_GATEWAY_URL="http://127.0.0.1:8084"
export RUNTIME_LLM_PROVIDER="${RUNTIME_LLM_PROVIDER:-chatgpt}"
export RUNTIME_LLM_MODEL="${RUNTIME_LLM_MODEL:-gpt-5.5}"
export RUNTIME_LLM_REASONING_EFFORT="${RUNTIME_LLM_REASONING_EFFORT:-low}"
export RUNTIME_VMCTL_URL="${RUNTIME_VMCTL_URL:-http://127.0.0.1:${VMCTL_PORT}}"
export RUNTIME_SELF_URL="${RUNTIME_SELF_URL:-http://127.0.0.1:${SANDBOX_PORT:-8085}}"
export RUNTIME_LOCAL_WORKER_MODE="${RUNTIME_LOCAL_WORKER_MODE:-worktree}"
export RUNTIME_SUPER_FOREGROUND_MUTATION_MODE="${RUNTIME_SUPER_FOREGROUND_MUTATION_MODE:-worker_only}"
export RUNTIME_TOOL_CWD="${RUNTIME_TOOL_CWD:-$(pwd)}"
export SANDBOX_PORT="${SANDBOX_PORT:-8085}" SANDBOX_ID="${SANDBOX_ID:-sandbox-dev}" RUNTIME_ENABLE_TEST_APIS="${RUNTIME_ENABLE_TEST_APIS:-0}"
nohup go run ./cmd/sandbox > sandbox.log 2>&1 &
SANDBOX_PID=$!
[ "${CHOIR_SERVICES_FOREGROUND:-0}" = "1" ] || disown "$SANDBOX_PID"
wait_for_url http://127.0.0.1:8081/health auth || exit 1
wait_for_url http://127.0.0.1:8085/health sandbox || exit 1

export PROXY_PORT=8082 PROXY_SANDBOX_URL="http://127.0.0.1:8085" PROXY_VMCTL_URL="${PROXY_VMCTL_URL:-http://127.0.0.1:${VMCTL_PORT}}" PROXY_CORPUSD_URL="${PROXY_CORPUSD_URL:-http://127.0.0.1:${CORPUSD_PORT:-8086}}" PROXY_VMCTL_TIMEOUT="${PROXY_VMCTL_TIMEOUT:-180s}"
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
    kill "$FRONTEND_PID" "$PROXY_PID" "$SANDBOX_PID" "$CORPUSD_PID" "$PLATFORM_DOLT_PID" "$VMCTL_PID" "$GATEWAY_PID" "$AUTH_PID" 2>/dev/null || true
  }
  trap cleanup_services INT TERM EXIT
  wait
fi
