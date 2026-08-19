#!/usr/bin/env bash
set -u -o pipefail

usage() {
  cat <<'USAGE'
agentic-consensus-runner.sh --prompt TEXT | --prompt-file FILE [options]

Runs one prompt across an agentic consensus panel and writes one output file per agent.
Default panel: codex, devin, cursor, opencode, omp-gpt56-sol, omp-gemini37, omp-cursor-grok46, omp-deepseek-v4-flash-free.
External CLIs use their configured default model unless a --*-model override is passed.

Required input:
  --prompt TEXT                 Inline prompt.
  --prompt-file FILE            Read prompt from file.

Thinking mode:
  --mode convergent|divergent|lateral
                                Inject a thinking-mode preamble into the prompt.
                                convergent (default): decide, seek agreement, recommend.
                                divergent: expand the option space; no ranking; contradictions are features.
                                lateral: break the frame; invert hidden assumptions; import analogies.
  --lenses LIST                 Comma-separated starting lenses assigned round-robin to
                                panelists (lens i to agent i). Most useful with --mode divergent
                                so same-family models do not cluster on one angle. The lens orients
                                a panelist; it does not confine them.

Panel selection:
  --include LIST                Comma-separated agent ids to run.
                                Default: codex,devin,cursor,opencode,omp-gpt56-sol,omp-gemini37,omp-cursor-grok46,omp-deepseek-v4-flash-free
  --exclude LIST                Comma-separated agent ids to skip.
  --list-agents                 Print supported agent ids and exit.

Model overrides, optional:
  --codex-model MODEL           Pass -m MODEL to codex exec.
  --devin-model MODEL           Pass --model MODEL to devin.
  --claude-model MODEL          Pass --model MODEL to claude.
  --cursor-model MODEL          Pass --model MODEL to Cursor agent.
  --opencode-model MODEL        Pass -m MODEL to opencode run.
  --omp-gpt56-sol-model MODEL   Default: openai-codex/gpt-5.6-sol.
  --omp-gpt56-terra-model MODEL Default: openai-codex/gpt-5.6-terra.
  --omp-gpt56-luna-model MODEL  Default: openai-codex/gpt-5.6-luna.
  --omp-gemini-model MODEL      Default: google-antigravity/gemini-3.7-flash.
  --omp-cursor-grok-model MODEL Default: cursor/cursor-grok-4.6-high.
  --omp-deepseek-model MODEL    Default: opencode-zen/deepseek-v4-flash-free.
  --omp-gpt56-sol-thinking LEVEL    Default: medium.
  --omp-gpt56-terra-thinking LEVEL   Default: xhigh.
  --omp-gpt56-luna-thinking LEVEL    Default: max.
  --omp-gemini-thinking LEVEL   Default: high.
  --omp-cursor-grok-thinking LEVEL   Default: high.
  --omp-deepseek-thinking LEVEL Default: high.

Execution:
  --cwd DIR                     Working directory/context root. Default: current directory.
  --out-dir DIR                 Output directory. Default: $CWD/.agentic-consensus/agentic-consensus-YYYYmmdd-HHMMSS.
  --sequential                  Run agents one at a time. Default: parallel.
  --dry-run                     Print commands but do not run them.
  --keep-going                  Return 0 if at least one agent succeeds. Default: fail if any selected agent fails.
  --no-tools-omp                Add --no-tools to OMP runs. Default: OMP tools enabled.
  --timeout-seconds N           Hard deadline for each agent. Default: 180.
  --help                       Show this help.

Output:
  <out-dir>/prompt.md           Exact prompt sent to agents.
  <out-dir>/manifest.tsv        agent, status, exit code, output path, command.
  <out-dir>/<agent>.out         stdout/stderr for each successful/failed run.
  <out-dir>/<agent>.cmd         shell-quoted command for reproducibility.
USAGE
}

DEFAULT_INCLUDE="codex,devin,cursor,opencode,omp-gpt56-sol,omp-gemini37,omp-cursor-grok46,omp-deepseek-v4-flash-free"
SUPPORTED_AGENTS=(codex devin claude cursor opencode omp-gpt56-sol omp-gpt56-terra omp-gpt56-luna omp-gemini37 omp-cursor-grok46 omp-deepseek-v4-flash-free)

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
MODE="convergent"
LENSES=""
LENS_LIST=()

CODEX_MODEL=""
DEVIN_MODEL=""
CLAUDE_MODEL=""
CURSOR_MODEL=""
OPENCODE_MODEL=""
OMP_GPT56_SOL_MODEL="openai-codex/gpt-5.6-sol"
OMP_GPT56_TERRA_MODEL="openai-codex/gpt-5.6-terra"
OMP_GPT56_LUNA_MODEL="openai-codex/gpt-5.6-luna"
OMP_GEMINI_MODEL="google-antigravity/gemini-3.7-flash"
OMP_CURSOR_GROK_MODEL="cursor/cursor-grok-4.6-high"
OMP_DEEPSEEK_MODEL="opencode-zen/deepseek-v4-flash-free"
OMP_GPT56_SOL_THINKING="medium"
OMP_GPT56_TERRA_THINKING="xhigh"
OMP_GPT56_LUNA_THINKING="max"
OMP_GEMINI_THINKING="high"
OMP_CURSOR_GROK_THINKING="high"
OMP_DEEPSEEK_THINKING="high"

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
    --omp-gpt56-sol-model)
      [[ $# -ge 2 ]] || { echo "--omp-gpt56-sol-model requires a value" >&2; exit 2; }
      OMP_GPT56_SOL_MODEL="$2"; shift 2 ;;
    --omp-gpt56-terra-model)
      [[ $# -ge 2 ]] || { echo "--omp-gpt56-terra-model requires a value" >&2; exit 2; }
      OMP_GPT56_TERRA_MODEL="$2"; shift 2 ;;
    --omp-gpt56-luna-model)
      [[ $# -ge 2 ]] || { echo "--omp-gpt56-luna-model requires a value" >&2; exit 2; }
      OMP_GPT56_LUNA_MODEL="$2"; shift 2 ;;
    --omp-gemini-model)
      [[ $# -ge 2 ]] || { echo "--omp-gemini-model requires a value" >&2; exit 2; }
      OMP_GEMINI_MODEL="$2"; shift 2 ;;
    --omp-cursor-grok-model)
      [[ $# -ge 2 ]] || { echo "--omp-cursor-grok-model requires a value" >&2; exit 2; }
      OMP_CURSOR_GROK_MODEL="$2"; shift 2 ;;
    --omp-deepseek-model)
      [[ $# -ge 2 ]] || { echo "--omp-deepseek-model requires a value" >&2; exit 2; }
      OMP_DEEPSEEK_MODEL="$2"; shift 2 ;;
    --omp-gpt56-sol-thinking)
      [[ $# -ge 2 ]] || { echo "--omp-gpt56-sol-thinking requires a value" >&2; exit 2; }
      OMP_GPT56_SOL_THINKING="$2"; shift 2 ;;
    --omp-gpt56-terra-thinking)
      [[ $# -ge 2 ]] || { echo "--omp-gpt56-terra-thinking requires a value" >&2; exit 2; }
      OMP_GPT56_TERRA_THINKING="$2"; shift 2 ;;
    --omp-gpt56-luna-thinking)
      [[ $# -ge 2 ]] || { echo "--omp-gpt56-luna-thinking requires a value" >&2; exit 2; }
      OMP_GPT56_LUNA_THINKING="$2"; shift 2 ;;
    --omp-gemini-thinking)
      [[ $# -ge 2 ]] || { echo "--omp-gemini-thinking requires a value" >&2; exit 2; }
      OMP_GEMINI_THINKING="$2"; shift 2 ;;
    --omp-cursor-grok-thinking)
      [[ $# -ge 2 ]] || { echo "--omp-cursor-grok-thinking requires a value" >&2; exit 2; }
      OMP_CURSOR_GROK_THINKING="$2"; shift 2 ;;
    --omp-deepseek-thinking)
      [[ $# -ge 2 ]] || { echo "--omp-deepseek-thinking requires a value" >&2; exit 2; }
      OMP_DEEPSEEK_THINKING="$2"; shift 2 ;;
    --sequential) SEQUENTIAL=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --keep-going) KEEP_GOING=1; shift ;;
    --no-tools-omp) NO_TOOLS_OMP=1; shift ;;
    --mode)
      [[ $# -ge 2 ]] || { echo "--mode requires a value" >&2; exit 2; }
      MODE="$2"; shift 2 ;;
    --lenses)
      [[ $# -ge 2 ]] || { echo "--lenses requires a value" >&2; exit 2; }
      LENSES="$2"; shift 2 ;;
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
case "$MODE" in
  convergent|divergent|lateral) ;;
  *) echo "--mode must be one of: convergent, divergent, lateral" >&2; exit 2 ;;
esac
if [[ -n "$LENSES" ]]; then
  IFS=',' read -r -a LENS_LIST <<< "$LENSES"
  if [[ ${#LENS_LIST[@]} -lt 1 ]]; then
    echo "--lenses must be a non-empty comma-separated list" >&2
    exit 2
  fi
fi
[[ -d "$CWD" ]] || { echo "--cwd is not a directory: $CWD" >&2; exit 2; }
if [[ -z "$OUT_DIR" ]]; then
  OUT_DIR="$CWD/.agentic-consensus/agentic-consensus-$(date +%Y%m%d-%H%M%S)"
fi
mkdir -p "$OUT_DIR" || exit 2

MODE_PREAMBLE=""
case "$MODE" in
  divergent)
    MODE_PREAMBLE="MODE: DIVERGENT — expand the option space, do not converge.

You are one member of an independent agentic consensus panel in divergent mode.
Your job is to maximize the number of distinct, well-formed options or framings
you return. Do not seek agreement, do not collapse to a verdict, and do not rank
options as if choosing. Contradictory options are a feature: each should be
internally coherent and genuinely different from the others. Prefer breadth,
novelty, and sharply separated alternatives over a single polished answer.

Return a numbered list of distinct options/framings, each with the core idea,
why it is genuinely different from the others, and its sharpest trade-off or
failure mode. End with the dimensions along which these options differ." ;;
  lateral)
    MODE_PREAMBLE="MODE: LATERAL — break the frame.

You are one member of an independent agentic consensus panel in lateral mode.
Your job is to find the hidden assumption or default frame that everyone else is
taking for granted and break it. Do not accept the question as posed. Identify
the implicit constraint, invert or sidestep it, and import a concrete analogy
from a distant domain if it sharpens the point.

Return: (1) the frame or assumption you rejected; (2) the reframed question or
alternative frame; (3) what that reframe would change in practice; (4) the
sharpest objection to your own reframe." ;;
  convergent)
    MODE_PREAMBLE="MODE: CONVERGENT — decide.

You are one member of an independent agentic consensus panel in convergent mode.
Return a clear verdict or recommendation, the strongest supporting findings,
the dissent you are aware of, risks/edge cases, your evidence or assumptions,
and a confidence level. Prioritize decision-useful output over breadth." ;;
esac

BASE_PROMPT="$PROMPT"
if [[ -n "$MODE_PREAMBLE" ]]; then
  BASE_PROMPT="$MODE_PREAMBLE

$PROMPT"
fi
printf '%s\n' "$BASE_PROMPT" > "$OUT_DIR/prompt.md"
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
      CMD=(codex exec --cd "$CWD" --autoputer read-only -c 'approval_policy="never"' --ephemeral --skip-git-repo-check)
      [[ -n "$CODEX_MODEL" ]] && CMD+=(-m "$CODEX_MODEL")
      CMD+=("$AGENT_PROMPT") ;;
    devin)
      CMD=(devin --permission-mode auto --respect-workspace-trust false)
      [[ -n "$DEVIN_MODEL" ]] && CMD+=(--model "$DEVIN_MODEL")
      CMD+=(-p "$AGENT_PROMPT") ;;
    claude)
      CMD=(claude -p --output-format text --permission-mode plan --no-session-persistence)
      [[ -n "$CLAUDE_MODEL" ]] && CMD+=(--model "$CLAUDE_MODEL")
      CMD+=("$AGENT_PROMPT") ;;
    cursor)
      CMD=(agent --print --output-format text --mode ask --trust --force --approve-mcps --workspace "$CWD")
      [[ -n "$CURSOR_MODEL" ]] && CMD+=(--model "$CURSOR_MODEL")
      CMD+=("$AGENT_PROMPT") ;;
    opencode)
      CMD=(opencode run --dir "$CWD")
      [[ -n "$OPENCODE_MODEL" ]] && CMD+=(-m "$OPENCODE_MODEL")
      CMD+=("$AGENT_PROMPT") ;;
    omp-gpt56-sol)
      CMD=(omp -p --mode text --model "$OMP_GPT56_SOL_MODEL" --thinking "$OMP_GPT56_SOL_THINKING" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$AGENT_PROMPT") ;;
    omp-gpt56-terra)
      CMD=(omp -p --mode text --model "$OMP_GPT56_TERRA_MODEL" --thinking "$OMP_GPT56_TERRA_THINKING" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$AGENT_PROMPT") ;;
    omp-gpt56-luna)
      CMD=(omp -p --mode text --model "$OMP_GPT56_LUNA_MODEL" --thinking "$OMP_GPT56_LUNA_THINKING" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$AGENT_PROMPT") ;;
    omp-gemini37)
      CMD=(omp -p --mode text --model "$OMP_GEMINI_MODEL" --thinking "$OMP_GEMINI_THINKING" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$AGENT_PROMPT") ;;
    omp-cursor-grok46)
      CMD=(omp -p --mode text --model "$OMP_CURSOR_GROK_MODEL" --thinking "$OMP_CURSOR_GROK_THINKING" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$AGENT_PROMPT") ;;
    omp-deepseek-v4-flash-free)
      CMD=(omp -p --mode text --model "$OMP_DEEPSEEK_MODEL" --thinking "$OMP_DEEPSEEK_THINKING" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
      [[ "$NO_TOOLS_OMP" -eq 1 ]] && CMD+=(--no-tools)
      CMD+=("$AGENT_PROMPT") ;;
    *) return 2 ;;
  esac
}

run_one() {
  local agent="$1"
  local out="$OUT_DIR/$agent.out"
  local cmdfile="$OUT_DIR/$agent.cmd"
  local bin=""
  local agent_idx=0
  for i in "${!selected_agents[@]}"; do
    if [[ "${selected_agents[$i]}" == "$agent" ]]; then agent_idx=$i; break; fi
  done

  AGENT_PROMPT="$BASE_PROMPT"
  if [[ ${#LENS_LIST[@]} -gt 0 ]]; then
    local lens="${LENS_LIST[$((agent_idx % ${#LENS_LIST[@]}))]}"
    AGENT_PROMPT="LENS: $lens

$BASE_PROMPT"
  fi

  case "$agent" in
    cursor) bin="agent" ;;
    omp-*) bin="omp" ;;
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
  printf 'prompt> %s\n' "$AGENT_PROMPT" > "$out"

  if [[ "$DRY_RUN" -eq 1 ]]; then
    append_manifest "$agent" "dry-run" "0" "0" "$out" "$rendered"
    printf '%s\n' "$rendered" > "$out"
    return 0
  fi

  local started=$SECONDS
  (
    cd "$CWD" || exit 2
    if command -v timeout >/dev/null 2>&1; then
      timeout --signal=TERM --kill-after=5 "$TIMEOUT_SECONDS" "${CMD[@]}"
    else
      "${CMD[@]}"
    fi
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
