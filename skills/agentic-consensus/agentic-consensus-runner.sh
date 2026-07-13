#!/usr/bin/env bash
set -u -o pipefail

usage() {
  cat <<'USAGE'
agentic-consensus-runner.sh --prompt TEXT | --prompt-file FILE [options]

Runs one prompt across an agentic consensus panel and writes one output file per agent.
Default panel: codex, devin, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52.
External CLIs use their configured default model unless a --*-model override is passed.

Required input:
  --prompt TEXT                 Inline prompt.
  --prompt-file FILE            Read prompt from file.

Panel selection:
  --include LIST                Comma-separated agent ids to run.
                                Default: codex,devin,cursor,opencode,omp-gpt55,omp-gemini35,omp-glm52
  --exclude LIST                Comma-separated agent ids to skip.
  --list-agents                 Print supported agent ids and exit.

Model overrides, optional:
  --codex-model MODEL           Pass -m MODEL to codex exec.
  --devin-model MODEL           Pass --model MODEL to devin.
  --claude-model MODEL          Pass --model MODEL to claude.
  --cursor-model MODEL          Pass --model MODEL to Cursor agent.
  --opencode-model MODEL        Pass -m MODEL to opencode run.
  --omp-gpt55-model MODEL       Default: openai-codex/gpt-5.5.
  --omp-gemini-model MODEL      Default: google-antigravity/gemini-3.5-flash.
  --omp-glm52-model MODEL       Default: zai/glm-5.2.
  --omp-gpt55-thinking LEVEL    Default: high.
  --omp-gemini-thinking LEVEL   Default: high.
  --omp-glm52-thinking LEVEL    Default: high.

Execution:
  --cwd DIR                     Working directory/context root. Default: current directory.
  --out-dir DIR                 Output directory. Default: /tmp/agentic-consensus-YYYYmmdd-HHMMSS.
  --sequential                  Run agents one at a time. Default: parallel.
  --dry-run                     Print commands but do not run them.
  --keep-going                  Return 0 if at least one agent succeeds. Default: fail if any selected agent fails.
  --no-tools-omp                Add --no-tools to OMP runs. Default: OMP tools enabled.
  --timeout-seconds N           Hard deadline for each agent. Default: 180.
  --help                       Show this help.

Output:
  <out-dir>/prompt.md           Exact prompt sent to agents.
  <out-dir>/manifest.tsv        agent, status, exit code, duration, output path, command.
  <out-dir>/<agent>.out         stdout/stderr for each successful/failed run.
  <out-dir>/<agent>.cmd         shell-quoted command for reproducibility.
USAGE
}

DEFAULT_INCLUDE="codex,devin,cursor,opencode,omp-gpt55,omp-gemini35,omp-glm52"
SUPPORTED_AGENTS=(codex devin claude cursor opencode omp-gpt55 omp-gemini35 omp-glm52)

PROMPT=""
PROMPT_FILE=""
INCLUDE="$DEFAULT_INCLUDE"
EXCLUDE=""
CWD="$PWD"
OUT_DIR=""
SEQUENTIAL=0
DRY_RUN=0
KEEP_GOING=0
NO_TOOLS_OMP=0
TIMEOUT_SECONDS=180

CODEX_MODEL=""
DEVIN_MODEL=""
CLAUDE_MODEL=""
CURSOR_MODEL=""
OPENCODE_MODEL=""
OMP_GPT55_MODEL="openai-codex/gpt-5.5"
OMP_GEMINI_MODEL="google-antigravity/gemini-3.5-flash"
OMP_GLM52_MODEL="zai/glm-5.2"
OMP_GPT55_THINKING="high"
OMP_GEMINI_THINKING="high"
OMP_GLM52_THINKING="high"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prompt)
      [[ $# -ge 2 ]] || { echo "--prompt requires a value" >&2; exit 2; }
      PROMPT="$2"; shift 2 ;;
    --prompt-file)
      [[ $# -ge 2 ]] || { echo "--prompt-file requires a value" >&2; exit 2; }
      PROMPT_FILE="$2"; shift 2 ;;
    --include)
      [[ $# -ge 2 ]] || { echo "--include requires a value" >&2; exit 2; }
      INCLUDE="$2"; shift 2 ;;
    --exclude)
      [[ $# -ge 2 ]] || { echo "--exclude requires a value" >&2; exit 2; }
      EXCLUDE="$2"; shift 2 ;;
    --cwd)
      [[ $# -ge 2 ]] || { echo "--cwd requires a value" >&2; exit 2; }
      CWD="$2"; shift 2 ;;
    --out-dir)
      [[ $# -ge 2 ]] || { echo "--out-dir requires a value" >&2; exit 2; }
      OUT_DIR="$2"; shift 2 ;;
    --codex-model)
      [[ $# -ge 2 ]] || { echo "--codex-model requires a value" >&2; exit 2; }
      CODEX_MODEL="$2"; shift 2 ;;
    --devin-model)
      [[ $# -ge 2 ]] || { echo "--devin-model requires a value" >&2; exit 2; }
      DEVIN_MODEL="$2"; shift 2 ;;
    --claude-model)
      [[ $# -ge 2 ]] || { echo "--claude-model requires a value" >&2; exit 2; }
      CLAUDE_MODEL="$2"; shift 2 ;;
    --cursor-model)
      [[ $# -ge 2 ]] || { echo "--cursor-model requires a value" >&2; exit 2; }
      CURSOR_MODEL="$2"; shift 2 ;;
    --opencode-model)
      [[ $# -ge 2 ]] || { echo "--opencode-model requires a value" >&2; exit 2; }
      OPENCODE_MODEL="$2"; shift 2 ;;
    --omp-gpt55-model)
      [[ $# -ge 2 ]] || { echo "--omp-gpt55-model requires a value" >&2; exit 2; }
      OMP_GPT55_MODEL="$2"; shift 2 ;;
    --omp-gemini-model)
      [[ $# -ge 2 ]] || { echo "--omp-gemini-model requires a value" >&2; exit 2; }
      OMP_GEMINI_MODEL="$2"; shift 2 ;;
    --omp-glm52-model)
      [[ $# -ge 2 ]] || { echo "--omp-glm52-model requires a value" >&2; exit 2; }
      OMP_GLM52_MODEL="$2"; shift 2 ;;
    --omp-gpt55-thinking)
      [[ $# -ge 2 ]] || { echo "--omp-gpt55-thinking requires a value" >&2; exit 2; }
      OMP_GPT55_THINKING="$2"; shift 2 ;;
    --omp-gemini-thinking)
      [[ $# -ge 2 ]] || { echo "--omp-gemini-thinking requires a value" >&2; exit 2; }
      OMP_GEMINI_THINKING="$2"; shift 2 ;;
    --omp-glm52-thinking)
      [[ $# -ge 2 ]] || { echo "--omp-glm52-thinking requires a value" >&2; exit 2; }
      OMP_GLM52_THINKING="$2"; shift 2 ;;
    --sequential) SEQUENTIAL=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --keep-going) KEEP_GOING=1; shift ;;
    --no-tools-omp) NO_TOOLS_OMP=1; shift ;;
    --timeout-seconds)
      [[ $# -ge 2 ]] || { echo "--timeout-seconds requires a value" >&2; exit 2; }
      TIMEOUT_SECONDS="$2"; shift 2 ;;
    --list-agents)
      printf '%s\n' "${SUPPORTED_AGENTS[@]}"; exit 0 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -n "$PROMPT" && -n "$PROMPT_FILE" ]]; then
  echo "Use either --prompt or --prompt-file, not both" >&2
  exit 2
fi
if [[ -n "$PROMPT_FILE" ]]; then
  [[ -f "$PROMPT_FILE" ]] || { echo "Prompt file not found: $PROMPT_FILE" >&2; exit 2; }
  PROMPT="$(cat "$PROMPT_FILE")"
fi
if [[ -z "$PROMPT" ]]; then
  echo "Missing --prompt or --prompt-file" >&2
  usage >&2
  exit 2
fi
[[ "$TIMEOUT_SECONDS" =~ ^[1-9][0-9]*$ ]] || { echo "--timeout-seconds must be a positive integer" >&2; exit 2; }
[[ -d "$CWD" ]] || { echo "--cwd is not a directory: $CWD" >&2; exit 2; }
if [[ "$DRY_RUN" -eq 0 ]]; then
  if ! command -v timeout >/dev/null 2>&1; then
    echo "GNU timeout is required for bounded agent execution; install coreutils or use --dry-run" >&2
    exit 2
  fi
  TIMEOUT_VERSION="$(timeout --version 2>&1 || true)"
  if [[ "$TIMEOUT_VERSION" != *"GNU coreutils"* ]]; then
    echo "GNU coreutils timeout is required; found an incompatible timeout implementation" >&2
    exit 2
  fi
fi
if [[ -z "$OUT_DIR" ]]; then
  OUT_DIR="/tmp/agentic-consensus-$(date +%Y%m%d-%H%M%S)"
fi
mkdir -p "$OUT_DIR" || exit 2
printf '%s\n' "$PROMPT" > "$OUT_DIR/prompt.md"
printf 'agent\tstatus\texit_code\tduration_seconds\toutput\tcommand\n' > "$OUT_DIR/manifest.tsv"

contains_csv() {
  local csv=",$1,"
  local item="$2"
  [[ "$csv" == *",$item,"* ]]
}

selected_agents=()
for agent in "${SUPPORTED_AGENTS[@]}"; do
  if contains_csv "$INCLUDE" "$agent" && ! contains_csv "$EXCLUDE" "$agent"; then
    selected_agents+=("$agent")
  fi
done
if [[ ${#selected_agents[@]} -eq 0 ]]; then
  echo "No agents selected" >&2
  exit 2
fi

quote_cmd() {
  printf '%q ' "$@"
}

append_manifest() {
  local agent="$1" status="$2" code="$3" duration="$4" output="$5" command="$6"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$agent" "$status" "$code" "$duration" "$output" "$command" >> "$OUT_DIR/manifest.tsv"
}

build_cmd() {
  local agent="$1"
  CMD=()
  case "$agent" in
    codex)
      CMD=(codex exec --cd "$CWD" --sandbox read-only -c 'approval_policy="never"' --ephemeral --skip-git-repo-check)
      [[ -n "$CODEX_MODEL" ]] && CMD+=(-m "$CODEX_MODEL")
      CMD+=("$PROMPT") ;;
    devin)
      CMD=(devin --permission-mode auto --respect-workspace-trust false)
      [[ -n "$DEVIN_MODEL" ]] && CMD+=(--model "$DEVIN_MODEL")
      CMD+=(-p "$PROMPT") ;;
    claude)
      CMD=(claude -p --output-format text --permission-mode plan --no-session-persistence)
      [[ -n "$CLAUDE_MODEL" ]] && CMD+=(--model "$CLAUDE_MODEL")
      CMD+=("$PROMPT") ;;
    cursor)
      CMD=(agent --print --output-format text --mode ask --trust --force --approve-mcps --workspace "$CWD")
      [[ -n "$CURSOR_MODEL" ]] && CMD+=(--model "$CURSOR_MODEL")
      CMD+=("$PROMPT") ;;
    opencode)
      CMD=(opencode run --dir "$CWD")
      [[ -n "$OPENCODE_MODEL" ]] && CMD+=(-m "$OPENCODE_MODEL")
      CMD+=("$PROMPT") ;;
    omp-gpt55)
      CMD=(omp -p --mode text --model "$OMP_GPT55_MODEL" --thinking "$OMP_GPT55_THINKING" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$PROMPT") ;;
    omp-gemini35)
      CMD=(omp -p --mode text --model "$OMP_GEMINI_MODEL" --thinking "$OMP_GEMINI_THINKING" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$PROMPT") ;;
    omp-glm52)
      CMD=(omp -p --mode text --model "$OMP_GLM52_MODEL" --thinking "$OMP_GLM52_THINKING" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$PROMPT") ;;
    *) return 2 ;;
  esac
}

run_one() {
  local agent="$1"
  local out="$OUT_DIR/$agent.out"
  local cmdfile="$OUT_DIR/$agent.cmd"
  local bin=""

  case "$agent" in
    cursor) bin="agent" ;;
    omp-gpt55|omp-gemini35|omp-glm52) bin="omp" ;;
    *) bin="$agent" ;;
  esac

  if ! command -v "$bin" >/dev/null 2>&1; then
    append_manifest "$agent" "skipped-missing-cli" "127" "0" "$out" "$bin not found"
    printf '%s\n' "SKIPPED: $bin not found" > "$out"
    return 127
  fi

  build_cmd "$agent" || return 2
  local rendered
  rendered="$(quote_cmd "${CMD[@]}")"
  printf '%s\n' "$rendered" > "$cmdfile"

  if [[ "$DRY_RUN" -eq 1 ]]; then
    append_manifest "$agent" "dry-run" "0" "0" "$out" "$rendered"
    printf '%s\n' "$rendered" > "$out"
    return 0
  fi

  local started=$SECONDS
  (
    cd "$CWD" || exit 2
    timeout --signal=KILL "$TIMEOUT_SECONDS" \
      bash -c '"$@"; code=$?; [[ $code -eq 124 ]] && exit 123; [[ $code -eq 137 ]] && exit 136; exit "$code"' _ "${CMD[@]}"
  ) </dev/null >"$out" 2>&1
  local code=$?
  local duration=$((SECONDS - started))
  if [[ $code -eq 0 ]]; then
    append_manifest "$agent" "ok" "$code" "$duration" "$out" "$rendered"
  elif [[ $code -eq 124 || $code -eq 137 ]]; then
    append_manifest "$agent" "timed-out" "$code" "$duration" "$out" "$rendered"
  else
    append_manifest "$agent" "failed" "$code" "$duration" "$out" "$rendered"
  fi
  return "$code"
}

pids=()
pid_agents=()
failures=0
successes=0

if [[ "$SEQUENTIAL" -eq 1 || "$DRY_RUN" -eq 1 ]]; then
  for agent in "${selected_agents[@]}"; do
    if run_one "$agent"; then
      successes=$((successes + 1))
    else
      failures=$((failures + 1))
    fi
  done
else
  for agent in "${selected_agents[@]}"; do
    run_one "$agent" &
    pids+=("$!")
    pid_agents+=("$agent")
  done
  for pid in "${pids[@]}"; do
    if wait "$pid"; then
      successes=$((successes + 1))
    else
      failures=$((failures + 1))
    fi
  done
fi

echo "Output directory: $OUT_DIR"
echo "Manifest: $OUT_DIR/manifest.tsv"
echo "Succeeded: $successes"
echo "Failed/skipped: $failures"

if [[ "$KEEP_GOING" -eq 1 && "$successes" -gt 0 ]]; then
  exit 0
fi
if [[ "$failures" -gt 0 ]]; then
  exit 1
fi
exit 0
