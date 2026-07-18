#!/usr/bin/env bash
set -u -o pipefail

usage() {
  cat <<'USAGE'
agentic-consensus-runner.sh --prompt TEXT | --prompt-file FILE [options]

Runs one prompt across an adaptive, preflighted consensus panel and writes one output file per selected member.
Default profile: balanced. External CLIs use their configured default model unless a --*-model override is passed.

Required input:
  --prompt TEXT                         Inline prompt.
  --prompt-file FILE                    Read prompt from file.

Panel selection:
  --profile PROFILE                     balanced (default), fast, thorough, or deep (alias for thorough).
  --include LIST                        Exact comma-separated member ids to run, in the given order.
                                        Explicit selection bypasses profile size limits, but not preflight.
  --exclude LIST                        Comma-separated member ids to skip.
  --list-agents                         Print supported member ids and exit.

Supported member ids:
  codex, devin, claude, cursor, opencode, omp-gpt55, omp-gemini35, omp-cursor-grok45

Model overrides, optional:
  --codex-model MODEL                   Pass -m MODEL to codex exec.
  --devin-model MODEL                   Pass --model MODEL to devin.
  --claude-model MODEL                  Pass --model MODEL to claude.
  --cursor-model MODEL                  Pass --model MODEL to Cursor agent.
  --opencode-model MODEL                Pass -m MODEL to opencode run.
  --omp-gpt55-model MODEL               Default: openai-codex/gpt-5.5.
  --omp-gemini-model MODEL              Default: google-antigravity/gemini-3.5-flash.
  --omp-cursor-grok-model MODEL         Default: cursor/cursor-grok-4.5-high.
  --omp-gpt55-thinking LEVEL            Default: high.
  --omp-gemini-thinking LEVEL           Default: high.
  --omp-cursor-grok-thinking LEVEL      Default: high.

Execution:
  --cwd DIR                             Working directory/context root. Default: current directory.
  --out-dir DIR                         Output directory. Default: /tmp/agentic-consensus-YYYYmmdd-HHMMSS.
  --sequential                          Run selected members one at a time. Default: parallel.
  --dry-run                             Preflight and render commands but do not run panel members.
  --keep-going                          Return 0 if at least one selected member succeeds.
  --no-tools-omp                        Add --no-tools to OMP runs. Default: OMP tools enabled.
  --timeout-seconds N                   Override every profile/member deadline with N seconds.
  Exit-zero whitespace/progress chatter is recorded as failed-non-substantive-output.
  --help                                Show this help.

Profile defaults:
  fast                                  3 routes; 60-90 second member deadlines.
  balanced                              5 routes; 120-210 second member deadlines.
  thorough/deep                         Up to 8 routes; 240-360 second member deadlines.

Output:
  <out-dir>/prompt.md                   Exact prompt sent to members.
  <out-dir>/manifest.tsv                Selection, preflight, effective identity, deadline, result, and command.
  <out-dir>/preflight-omp-models.txt      Exact OMP model listing used for selection, when OMP is present.
  <out-dir>/<member>.out                Combined stdout/stderr for each selected run.
  <out-dir>/<member>.cmd                Shell-quoted command for reproducibility.
USAGE
}

SUPPORTED_AGENTS=(codex devin claude cursor opencode omp-gpt55 omp-gemini35 omp-cursor-grok45)
PROFILE="balanced"
INCLUDE=""
INCLUDE_EXPLICIT=0
EXCLUDE=""
PROMPT=""
PROMPT_FILE=""
CWD="$PWD"
OUT_DIR=""
SEQUENTIAL=0
DRY_RUN=0
KEEP_GOING=0
NO_TOOLS_OMP=0
TIMEOUT_SECONDS=""

CODEX_MODEL=""
DEVIN_MODEL=""
CLAUDE_MODEL=""
CURSOR_MODEL=""
OPENCODE_MODEL=""
OMP_GPT55_MODEL="openai-codex/gpt-5.5"
OMP_GEMINI_MODEL="google-antigravity/gemini-3.5-flash"
OMP_CURSOR_GROK_MODEL="cursor/cursor-grok-4.5-high"
OMP_GPT55_THINKING="high"
OMP_GEMINI_THINKING="high"
OMP_CURSOR_GROK_THINKING="high"

is_supported() {
  local wanted="$1" candidate
  for candidate in "${SUPPORTED_AGENTS[@]}"; do
    [[ "$candidate" == "$wanted" ]] && return 0
  done
  return 1
}

contains_csv() {
  local csv=",$1," item="$2"
  [[ "$csv" == *",$item,"* ]]
}

validate_csv() {
  local flag="$1" csv="$2" item seen=""
  [[ -n "$csv" ]] || { echo "$flag requires a non-empty list" >&2; exit 2; }
  IFS=',' read -r -a csv_items <<< "$csv"
  for item in "${csv_items[@]}"; do
    [[ -n "$item" && "$item" != *[[:space:]]* ]] || { echo "$flag contains an empty or whitespace-padded id" >&2; exit 2; }
    is_supported "$item" || { echo "$flag contains unknown member id: $item" >&2; exit 2; }
    contains_csv "$seen" "$item" && { echo "$flag contains duplicate member id: $item" >&2; exit 2; }
    seen="${seen:+$seen,}$item"
  done
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prompt) [[ $# -ge 2 ]] || { echo "--prompt requires a value" >&2; exit 2; }; PROMPT="$2"; shift 2 ;;
    --prompt-file) [[ $# -ge 2 ]] || { echo "--prompt-file requires a value" >&2; exit 2; }; PROMPT_FILE="$2"; shift 2 ;;
    --profile) [[ $# -ge 2 ]] || { echo "--profile requires a value" >&2; exit 2; }; PROFILE="$2"; shift 2 ;;
    --include) [[ $# -ge 2 ]] || { echo "--include requires a value" >&2; exit 2; }; INCLUDE="$2"; INCLUDE_EXPLICIT=1; shift 2 ;;
    --exclude) [[ $# -ge 2 ]] || { echo "--exclude requires a value" >&2; exit 2; }; EXCLUDE="$2"; shift 2 ;;
    --cwd) [[ $# -ge 2 ]] || { echo "--cwd requires a value" >&2; exit 2; }; CWD="$2"; shift 2 ;;
    --out-dir) [[ $# -ge 2 ]] || { echo "--out-dir requires a value" >&2; exit 2; }; OUT_DIR="$2"; shift 2 ;;
    --codex-model) [[ $# -ge 2 ]] || { echo "--codex-model requires a value" >&2; exit 2; }; CODEX_MODEL="$2"; shift 2 ;;
    --devin-model) [[ $# -ge 2 ]] || { echo "--devin-model requires a value" >&2; exit 2; }; DEVIN_MODEL="$2"; shift 2 ;;
    --claude-model) [[ $# -ge 2 ]] || { echo "--claude-model requires a value" >&2; exit 2; }; CLAUDE_MODEL="$2"; shift 2 ;;
    --cursor-model) [[ $# -ge 2 ]] || { echo "--cursor-model requires a value" >&2; exit 2; }; CURSOR_MODEL="$2"; shift 2 ;;
    --opencode-model) [[ $# -ge 2 ]] || { echo "--opencode-model requires a value" >&2; exit 2; }; OPENCODE_MODEL="$2"; shift 2 ;;
    --omp-gpt55-model) [[ $# -ge 2 ]] || { echo "--omp-gpt55-model requires a value" >&2; exit 2; }; OMP_GPT55_MODEL="$2"; shift 2 ;;
    --omp-gemini-model) [[ $# -ge 2 ]] || { echo "--omp-gemini-model requires a value" >&2; exit 2; }; OMP_GEMINI_MODEL="$2"; shift 2 ;;
    --omp-cursor-grok-model) [[ $# -ge 2 ]] || { echo "--omp-cursor-grok-model requires a value" >&2; exit 2; }; OMP_CURSOR_GROK_MODEL="$2"; shift 2 ;;
    --omp-gpt55-thinking) [[ $# -ge 2 ]] || { echo "--omp-gpt55-thinking requires a value" >&2; exit 2; }; OMP_GPT55_THINKING="$2"; shift 2 ;;
    --omp-gemini-thinking) [[ $# -ge 2 ]] || { echo "--omp-gemini-thinking requires a value" >&2; exit 2; }; OMP_GEMINI_THINKING="$2"; shift 2 ;;
    --omp-cursor-grok-thinking) [[ $# -ge 2 ]] || { echo "--omp-cursor-grok-thinking requires a value" >&2; exit 2; }; OMP_CURSOR_GROK_THINKING="$2"; shift 2 ;;
    --sequential) SEQUENTIAL=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --keep-going) KEEP_GOING=1; shift ;;
    --no-tools-omp) NO_TOOLS_OMP=1; shift ;;
    --timeout-seconds) [[ $# -ge 2 ]] || { echo "--timeout-seconds requires a value" >&2; exit 2; }; TIMEOUT_SECONDS="$2"; shift 2 ;;
    --list-agents) printf '%s\n' "${SUPPORTED_AGENTS[@]}"; exit 0 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

case "$PROFILE" in
  balanced|fast|thorough) ;;
  deep) PROFILE="thorough" ;;
  *) echo "Unknown profile: $PROFILE (expected balanced, fast, thorough, or deep)" >&2; exit 2 ;;
esac
[[ "$INCLUDE_EXPLICIT" -eq 0 ]] || validate_csv --include "$INCLUDE"
[[ -z "$EXCLUDE" ]] || validate_csv --exclude "$EXCLUDE"
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
[[ -z "$TIMEOUT_SECONDS" || "$TIMEOUT_SECONDS" =~ ^[1-9][0-9]*$ ]] || { echo "--timeout-seconds must be a positive integer" >&2; exit 2; }
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

agent_bin() {
  case "$1" in
    cursor) printf 'agent' ;;
    omp-*) printf 'omp' ;;
    *) printf '%s' "$1" ;;
  esac
}
agent_provider() {
  case "$1" in
    codex|omp-gpt55) printf 'openai' ;;
    devin) printf 'devin' ;;
    claude) printf 'anthropic' ;;
    cursor|omp-cursor-grok45) printf 'cursor' ;;
    opencode) printf 'opencode' ;;
    omp-gemini35) printf 'google' ;;
  esac
}
agent_traits() {
  case "$1" in
    codex) printf 'quality:high,speed:medium,cost:medium,review:high' ;;
    devin) printf 'quality:high,speed:slow,cost:high,review:high' ;;
    claude) printf 'quality:high,speed:medium,cost:high,review:high' ;;
    cursor) printf 'quality:medium,speed:fast,cost:medium,review:medium' ;;
    opencode) printf 'quality:medium,speed:fast,cost:low,review:medium' ;;
    omp-gpt55) printf 'quality:high,speed:slow,cost:high,review:high' ;;
    omp-gemini35) printf 'quality:medium,speed:fast,cost:low,review:medium' ;;
    omp-cursor-grok45) printf 'quality:high,speed:medium,cost:medium,review:high' ;;
  esac
}
agent_speed() {
  case "$1" in
    cursor|opencode|omp-gemini35) printf 'fast' ;;
    codex|claude|omp-cursor-grok45) printf 'medium' ;;
    devin|omp-gpt55) printf 'slow' ;;
  esac
}
effective_model() {
  case "$1" in
    codex) printf '%s' "${CODEX_MODEL:-configured-default}" ;;
    devin) printf '%s' "${DEVIN_MODEL:-configured-default}" ;;
    claude) printf '%s' "${CLAUDE_MODEL:-configured-default}" ;;
    cursor) printf '%s' "${CURSOR_MODEL:-configured-default}" ;;
    opencode) printf '%s' "${OPENCODE_MODEL:-configured-default}" ;;
    omp-gpt55) printf '%s' "$OMP_GPT55_MODEL" ;;
    omp-gemini35) printf '%s' "$OMP_GEMINI_MODEL" ;;
    omp-cursor-grok45) printf '%s' "$OMP_CURSOR_GROK_MODEL" ;;
  esac
}
effective_route() {
  local agent="$1" model
  model="$(effective_model "$agent")"
  if [[ "$model" == "configured-default" ]]; then
    printf 'cli-default:%s' "$agent"
  else
    printf 'model:%s' "$model"
  fi
}
member_deadline() {
  local speed
  [[ -z "$TIMEOUT_SECONDS" ]] || { printf '%s' "$TIMEOUT_SECONDS"; return; }
  speed="$(agent_speed "$1")"
  case "$PROFILE:$speed" in
    fast:fast) printf '60' ;; fast:medium) printf '75' ;; fast:slow) printf '90' ;;
    balanced:fast) printf '120' ;; balanced:medium) printf '180' ;; balanced:slow) printf '210' ;;
    thorough:fast) printf '240' ;; thorough:medium) printf '300' ;; thorough:slow) printf '360' ;;
  esac
}

if [[ "$INCLUDE_EXPLICIT" -eq 1 ]]; then
  IFS=',' read -r -a considered_agents <<< "$INCLUDE"
  PROFILE_TARGET=${#considered_agents[@]}
else
  case "$PROFILE" in
    fast)
      considered_agents=(omp-gemini35 codex cursor opencode omp-cursor-grok45 devin claude omp-gpt55)
      PROFILE_TARGET=3 ;;
    balanced)
      considered_agents=(omp-gemini35 codex omp-cursor-grok45 devin opencode claude cursor omp-gpt55)
      PROFILE_TARGET=5 ;;
    thorough)
      considered_agents=(codex devin claude omp-cursor-grok45 omp-gemini35 opencode cursor omp-gpt55)
      PROFILE_TARGET=8 ;;
  esac
fi

needs_omp=0
for agent in "${considered_agents[@]}"; do
  [[ "$agent" == omp-* ]] && needs_omp=1
done
OMP_MODELS_STATUS="not-needed"
OMP_MODELS_OUTPUT=""
if [[ "$needs_omp" -eq 1 ]] && command -v omp >/dev/null 2>&1; then
  if omp models > "$OUT_DIR/preflight-omp-models.txt" 2>&1; then
    OMP_MODELS_STATUS="ok"
  else
    OMP_MODELS_STATUS="failed"
  fi
  OMP_MODELS_OUTPUT="$(cat "$OUT_DIR/preflight-omp-models.txt")"
fi
model_list_contains() {
  local model="$1"
  printf '%s\n' "$OMP_MODELS_OUTPUT" | awk -v wanted="$model" '
    {
      line=$0
      gsub(/\033\[[0-9;]*m/, "", line)
      if (line ~ /^[[:alnum:]_-]+ \([0-9]+\)$/) {
        split(line, heading, /[[:space:]]+/)
        provider=heading[1]
      }
      count=split(line, fields, /[[:space:]]+/)
      for (i=1; i<=count; i++) {
        token=fields[i]
        gsub(/^[^[:alnum:]_-]+/, "", token)
        gsub(/[^[:alnum:]_.\/-]+$/, "", token)
        if (token == wanted || (provider != "" && provider "/" token == wanted)) found=1
      }
    }
    END { exit found ? 0 : 1 }
  '
}

preflight_statuses=()
preflight_reasons=()
effective_models=()
effective_routes=()
providers=()
traits=()
deadlines=()
selected=()
selection_reasons=()
for ((i=0; i<${#considered_agents[@]}; i++)); do
  agent="${considered_agents[$i]}"
  bin="$(agent_bin "$agent")"
  effective_models[$i]="$(effective_model "$agent")"
  effective_routes[$i]="$(effective_route "$agent")"
  providers[$i]="$(agent_provider "$agent")"
  traits[$i]="$(agent_traits "$agent")"
  deadlines[$i]="$(member_deadline "$agent")"
  selected[$i]="no"
  selection_reasons[$i]="pending"
  if ! command -v "$bin" >/dev/null 2>&1; then
    preflight_statuses[$i]="missing-cli"
    preflight_reasons[$i]="$bin not found"
  elif [[ "$agent" == omp-* && "$OMP_MODELS_STATUS" == "failed" ]]; then
    preflight_statuses[$i]="preflight-error"
    preflight_reasons[$i]="omp models failed"
  elif [[ "$agent" == omp-* ]] && ! model_list_contains "${effective_models[$i]}"; then
    preflight_statuses[$i]="missing-model"
    preflight_reasons[$i]="omp models lacks exact id ${effective_models[$i]}"
  elif [[ "$agent" == omp-* ]]; then
    preflight_statuses[$i]="available"
    preflight_reasons[$i]="omp models contains exact id ${effective_models[$i]}"
  elif [[ "${effective_models[$i]}" == "configured-default" ]]; then
    preflight_statuses[$i]="available"
    preflight_reasons[$i]="$bin found; configured default is not listable"
  else
    preflight_statuses[$i]="available"
    preflight_reasons[$i]="$bin found; explicit model is not listable"
  fi
done

if [[ "$INCLUDE_EXPLICIT" -eq 1 ]]; then
  seen_explicit_routes=""
  for ((i=0; i<${#considered_agents[@]}; i++)); do
    agent="${considered_agents[$i]}"
    if [[ -n "$EXCLUDE" ]] && contains_csv "$EXCLUDE" "$agent"; then
      selection_reasons[$i]="explicitly excluded"
      continue
    fi
    selected[$i]="yes"
    if contains_csv "$seen_explicit_routes" "${effective_routes[$i]}"; then
      selection_reasons[$i]="explicit include preserves requested duplicate effective route ${effective_routes[$i]}"
    else
      selection_reasons[$i]="explicit include order=$((i + 1))"
      seen_explicit_routes="${seen_explicit_routes:+$seen_explicit_routes,}${effective_routes[$i]}"
    fi
  done
else
  selected_count=0
  seen_providers=""
  seen_routes=""
  for pass in provider-diversity additional-route; do
    for ((i=0; i<${#considered_agents[@]}; i++)); do
      [[ "$selected_count" -lt "$PROFILE_TARGET" ]] || break
      agent="${considered_agents[$i]}"
      [[ "${selection_reasons[$i]}" == "pending" ]] || continue
      if [[ -n "$EXCLUDE" ]] && contains_csv "$EXCLUDE" "$agent"; then
        selection_reasons[$i]="explicitly excluded"
        continue
      fi
      if [[ "${preflight_statuses[$i]}" != "available" ]]; then
        selection_reasons[$i]="not selected: ${preflight_statuses[$i]}"
        continue
      fi
      if contains_csv "$seen_routes" "${effective_routes[$i]}"; then
        selection_reasons[$i]="not selected: duplicate effective route ${effective_routes[$i]}"
        continue
      fi
      if [[ "$pass" == "provider-diversity" ]] && contains_csv "$seen_providers" "${providers[$i]}"; then
        continue
      fi
      selected[$i]="yes"
      selection_reasons[$i]="profile=$PROFILE priority=$((i + 1)) $pass ${traits[$i]}"
      seen_routes="${seen_routes:+$seen_routes,}${effective_routes[$i]}"
      seen_providers="${seen_providers:+$seen_providers,}${providers[$i]}"
      selected_count=$((selected_count + 1))
    done
  done
  for ((i=0; i<${#considered_agents[@]}; i++)); do
    if [[ "${selection_reasons[$i]}" == "pending" ]]; then
      agent="${considered_agents[$i]}"
      if [[ -n "$EXCLUDE" ]] && contains_csv "$EXCLUDE" "$agent"; then
        selection_reasons[$i]="explicitly excluded"
      elif [[ "${preflight_statuses[$i]}" != "available" ]]; then
        selection_reasons[$i]="not selected: ${preflight_statuses[$i]}"
      elif contains_csv "$seen_routes" "${effective_routes[$i]}"; then
        selection_reasons[$i]="not selected: duplicate effective route ${effective_routes[$i]}"
      else
        selection_reasons[$i]="not selected: profile limit $PROFILE_TARGET"
      fi
    fi
  done
fi

quote_cmd() { printf '%q ' "$@"; }
build_cmd() {
  local agent="$1" deadline="$2"
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
      CMD=(omp -p --mode text --model "$OMP_GPT55_MODEL" --thinking "$OMP_GPT55_THINKING" --no-session --max-time "$deadline" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$PROMPT") ;;
    omp-gemini35)
      CMD=(omp -p --mode text --model "$OMP_GEMINI_MODEL" --thinking "$OMP_GEMINI_THINKING" --no-session --max-time "$deadline" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$PROMPT") ;;
    omp-cursor-grok45)
      CMD=(omp -p --mode text --model "$OMP_CURSOR_GROK_MODEL" --thinking "$OMP_CURSOR_GROK_THINKING" --no-session --max-time "$deadline" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$PROMPT") ;;
    *) return 2 ;;
  esac
}

sanitize_field() { printf '%s' "$1" | tr '\t\r\n' '   '; }
write_row() {
  local idx="$1" status="$2" code="$3" duration="$4" output="$5" command="$6"
  local row="$OUT_DIR/.manifest-$idx"
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${considered_agents[$idx]}" "$PROFILE" "${providers[$idx]}" "${traits[$idx]}" "${selected[$idx]}" \
    "$(sanitize_field "${selection_reasons[$idx]}")" "${preflight_statuses[$idx]}" "$(sanitize_field "${preflight_reasons[$idx]}")" \
    "${effective_models[$idx]}" "${effective_routes[$idx]}" "${deadlines[$idx]}" "$status" "$code" "$duration" \
    "$(sanitize_field "$output")" "$(sanitize_field "$command")" > "$row"
}

has_substantive_output() {
  awk '
    function inspect(raw, count, parts, i, line, lowered) {
      gsub(/\033\[[0-9;]*m/, "", raw)
      gsub(/…/, "...", raw)
      count=split(raw, parts, /\r/)
      for (i=1; i<=count; i++) {
        line=parts[i]
        sub(/^[[:space:]]+/, "", line)
        sub(/[[:space:]]+$/, "", line)
        lowered=tolower(line)
        sub(/^[^[:alnum:]]+/, "", lowered)
        if (lowered == "") continue
        if (lowered ~ /^(working|thinking|loading|connecting|starting|initializing)[.[:space:]]*$/) continue
        found=1
      }
    }
    { inspect($0) }
    END { exit found ? 0 : 1 }
  ' "$1"
}

run_one() {
  local idx="$1"
  local agent="${considered_agents[$idx]}" deadline="${deadlines[$idx]}"
  local out="$OUT_DIR/$agent.out" cmdfile="$OUT_DIR/$agent.cmd" rendered started code duration
  build_cmd "$agent" "$deadline" || { write_row "$idx" "failed-build-command" "2" "0" "$out" "-"; return 2; }
  rendered="$(quote_cmd "${CMD[@]}")"
  printf '%s\n' "$rendered" > "$cmdfile"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf '%s\n' "$rendered" > "$out"
    write_row "$idx" "dry-run" "0" "0" "$out" "$rendered"
    return 0
  fi
  started=$SECONDS
  (cd "$CWD" && timeout --signal=KILL "$deadline" "${CMD[@]}") </dev/null >"$out" 2>&1
  code=$?
  duration=$((SECONDS - started))
  if [[ "$code" -eq 0 ]] && has_substantive_output "$out"; then
    write_row "$idx" "ok" "$code" "$duration" "$out" "$rendered"
  elif [[ "$code" -eq 0 ]]; then
    code=65
    write_row "$idx" "failed-non-substantive-output" "$code" "$duration" "$out" "$rendered"
  elif [[ "$code" -eq 124 || "$code" -eq 137 ]]; then
    write_row "$idx" "timed-out" "$code" "$duration" "$out" "$rendered"
  else
    write_row "$idx" "failed" "$code" "$duration" "$out" "$rendered"
  fi
  return "$code"
}

runnable_indices=()
preflight_failures=0
for ((i=0; i<${#considered_agents[@]}; i++)); do
  if [[ "${selected[$i]}" == "yes" && "${preflight_statuses[$i]}" == "available" ]]; then
    runnable_indices+=("$i")
  elif [[ "${selected[$i]}" == "yes" ]]; then
    out="$OUT_DIR/${considered_agents[$i]}.out"
    printf 'SKIPPED: %s\n' "${preflight_reasons[$i]}" > "$out"
    write_row "$i" "skipped-${preflight_statuses[$i]}" "127" "0" "$out" "-"
    preflight_failures=$((preflight_failures + 1))
  else
    write_row "$i" "not-selected" "-" "0" "-" "-"
  fi
done

successes=0
failures=$preflight_failures
pids=()
pid_indices=()
if [[ "$SEQUENTIAL" -eq 1 || "$DRY_RUN" -eq 1 ]]; then
  for i in "${runnable_indices[@]}"; do
    if run_one "$i"; then successes=$((successes + 1)); else failures=$((failures + 1)); fi
  done
else
  for i in "${runnable_indices[@]}"; do
    run_one "$i" &
    pids+=("$!")
    pid_indices+=("$i")
  done
  for pid in "${pids[@]}"; do
    if wait "$pid"; then successes=$((successes + 1)); else failures=$((failures + 1)); fi
  done
fi

printf 'agent\tprofile\tprovider\ttraits\tselected\tselection_reason\tpreflight_status\tpreflight_reason\teffective_model\teffective_route\tdeadline_seconds\tstatus\texit_code\tduration_seconds\toutput\tcommand\n' > "$OUT_DIR/manifest.tsv"
for ((i=0; i<${#considered_agents[@]}; i++)); do
  cat "$OUT_DIR/.manifest-$i" >> "$OUT_DIR/manifest.tsv"
  rm -f "$OUT_DIR/.manifest-$i"
done

echo "Profile: $PROFILE"
echo "Output directory: $OUT_DIR"
echo "Manifest: $OUT_DIR/manifest.tsv"
echo "Succeeded: $successes"
echo "Failed/skipped selected members: $failures"

if [[ "$KEEP_GOING" -eq 1 && "$successes" -gt 0 ]]; then exit 0; fi
if [[ "$failures" -gt 0 || "$successes" -eq 0 ]]; then exit 1; fi
exit 0
