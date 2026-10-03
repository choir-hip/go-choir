#!/usr/bin/env bash
# agentic-consensus-model-probe.sh — verify that the consensus panel's pinned
# OMP models are alive AND that the model serving each turn is the model the
# panel pinned.
#
# Why identity and not just liveness: `omp --model` fuzzy-matches, so a
# retired model id can silently resolve to an older generation and still exit
# 0 with plausible prose. That is worse than an outage — it corrodes the
# panel's independence claim while reporting success. `omp -p --mode json`
# reports the provider/model that served the turn; this script asserts on it.
#
# See docs/problems/agentic-consensus-panel-anchor-rot-2026-10-03.md
#
# Usage:
#   agentic-consensus-model-probe.sh                    # probe the default panel
#   agentic-consensus-model-probe.sh --all              # probe every supported anchor
#   agentic-consensus-model-probe.sh provider/model:thinking ...
#
# Exit: 0 if every probe is OK, 1 if any probe is not.
set -u -o pipefail

usage() {
  cat <<'USAGE'
agentic-consensus-model-probe.sh [--all] [--out-dir DIR] [--jobs N]
                                  [--timeout-seconds N] [provider/model:thinking ...]

With no model arguments, probes the runner's default panel anchors.

Verdicts:
  OK            exit 0, serving model == requested model, output contract met
  MISROUTE      exit 0, but a DIFFERENT model served the turn (retired id
                fuzzy-matched onto an older generation) — the dangerous one
  WRONG-ANSWER  exit 0, right model, failed the one-line output contract
  DEAD          non-zero exit (retired/unknown id, plan entitlement, 403/429)
  TIMEOUT       exceeded the hard deadline

Options:
  --all               Probe every supported anchor, not just the default panel.
  --out-dir DIR       Output directory. Default: a mktemp -d under TMPDIR.
  --jobs N            Parallel probes. Default: 6.
  --timeout-seconds N Hard deadline per probe. Default: 240.
USAGE
}

# Must match agentic-consensus-runner.sh DEFAULT_INCLUDE / SUPPORTED_AGENTS.
PROBE_CHECKSUM="1be0686a" # sha256("choir")[:8]
DEFAULT_SPECS=(
  "openai-codex/gpt-6.1-sol:xhigh"
  "openai-codex/gpt-6-luna:max"
  "google-antigravity/gemini-3.8-flash:high"
  "opencode-go/space-bunny-free:high"
  "opencode-go/muse-spark-1.3-contributor:high"
  "opencode-go/qwen3.8-max:high"
  "opencode-go/glm-5.3-flash:high"
)
ALL_SPECS=(
  "${DEFAULT_SPECS[@]}"
  "google-antigravity/claude-opus-4-6:high"
  "opencode-go/deepseek-v4-pro:high"
  "opencode-go/kimi-k3:high"
  "opencode-go/minimax-m3:high"
)

OUT_DIR=""
JOBS=6
TIMEOUT_SECONDS=240
PROBE_ALL=0
SPECS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --all) PROBE_ALL=1; shift ;;
    --out-dir)
      [[ $# -ge 2 ]] || { echo "--out-dir requires a value" >&2; exit 2; }
      OUT_DIR="$2"; shift 2 ;;
    --jobs)
      [[ $# -ge 2 ]] || { echo "--jobs requires a value" >&2; exit 2; }
      JOBS="$2"; shift 2 ;;
    --timeout-seconds)
      [[ $# -ge 2 ]] || { echo "--timeout-seconds requires a value" >&2; exit 2; }
      TIMEOUT_SECONDS="$2"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    -*) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
    *) SPECS+=("$1"); shift ;;
  esac
done

[[ "$JOBS" =~ ^[1-9][0-9]*$ ]] || { echo "--jobs must be a positive integer" >&2; exit 2; }
[[ "$TIMEOUT_SECONDS" =~ ^[1-9][0-9]*$ ]] || { echo "--timeout-seconds must be a positive integer" >&2; exit 2; }
command -v omp >/dev/null 2>&1 || { echo "omp not found on PATH" >&2; exit 2; }

if [[ ${#SPECS[@]} -eq 0 ]]; then
  if [[ "$PROBE_ALL" -eq 1 ]]; then
    SPECS=("${ALL_SPECS[@]}")
  else
    SPECS=("${DEFAULT_SPECS[@]}")
  fi
fi

if [[ -z "$OUT_DIR" ]]; then
  OUT_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agentic-consensus-probe-XXXXXX")"
fi
mkdir -p "$OUT_DIR" || exit 2

probe_one() {
  local spec="$1"
  local model="${spec%:*}" thinking="${spec##*:}"
  local slug="${model//\//_}__${thinking}"
  local raw="$OUT_DIR/$slug.json"
  local prompt="Reply with exactly one line and nothing else: ACK $PROBE_CHECKSUM"
  local started=$SECONDS served verdict code dur

  if command -v timeout >/dev/null 2>&1; then
    timeout --signal=TERM --kill-after=5 "$TIMEOUT_SECONDS" \
      omp -p --mode json --model "$model" --thinking "$thinking" \
      --no-session --no-tools --auto-approve "$prompt" </dev/null >"$raw" 2>&1
  else
    omp -p --mode json --model "$model" --thinking "$thinking" \
      --no-session --no-tools --auto-approve "$prompt" </dev/null >"$raw" 2>&1
  fi
  code=$?
  dur=$((SECONDS - started))

  served=$(python3 - "$raw" <<'PY' 2>/dev/null || true
import json, sys
served = ""
for line in open(sys.argv[1], errors="replace"):
    line = line.strip()
    if not line.startswith("{"):
        continue
    try:
        event = json.loads(line)
    except Exception:
        continue
    if event.get("type") == "message_end":
        message = event.get("message") or {}
        if message.get("model"):
            served = "%s/%s" % (message.get("provider", ""), message["model"])
print(served)
PY
)

  if [[ $code -eq 124 || $code -eq 137 ]]; then
    verdict="TIMEOUT"
  elif [[ $code -ne 0 ]]; then
    verdict="DEAD"
  elif [[ "$served" != "$model" ]]; then
    verdict="MISROUTE"
  elif ! grep -q "ACK $PROBE_CHECKSUM" "$raw"; then
    verdict="WRONG-ANSWER"
  else
    verdict="OK"
  fi

  printf '%s\t%s\t%s\t%s\t%s\n' "$verdict" "$dur" "$model" "$thinking" "${served:--}" \
    >> "$OUT_DIR/manifest.tsv"
}

export -f probe_one
export OUT_DIR TIMEOUT_SECONDS PROBE_CHECKSUM

: > "$OUT_DIR/manifest.tsv"
printf '%s\n' "${SPECS[@]}" | xargs -P "$JOBS" -I{} bash -c 'probe_one "$@"' _ {}

# Stable report order: specs in probe order, not completion order.
python3 - "$OUT_DIR/manifest.tsv" "${SPECS[@]}" <<'PY'
import sys
manifest = {}
for line in open(sys.argv[1]):
    parts = line.rstrip("\n").split("\t")
    if len(parts) == 5:
        manifest[(parts[2], parts[3])] = parts
bad = 0
print("%-13s %5s  %-46s %-7s %s" % ("VERDICT", "SECS", "REQUESTED", "THINK", "SERVED"))
for spec in sys.argv[2:]:
    model, _, thinking = spec.rpartition(":")
    row = manifest.get((model, thinking))
    if row is None:
        print("%-13s %5s  %-46s %-7s %s" % ("NO-RESULT", "-", model, thinking, "-"))
        bad += 1
        continue
    verdict, dur = row[0], row[1]
    served = row[4]
    if verdict != "OK":
        bad += 1
    print("%-13s %5s  %-46s %-7s %s" % (verdict, dur, model, thinking, served))
sys.exit(1 if bad else 0)
PY
status=$?

echo
echo "manifest: $OUT_DIR/manifest.tsv"
exit "$status"
