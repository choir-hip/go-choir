#!/usr/bin/env bash
set -u -o pipefail

usage() {
  cat <<'USAGE'
agentic-consensus-runner.sh --prompt TEXT | --prompt-file FILE [options]

Runs one prompt across an agentic consensus panel and writes one output file per agent.
Default panel: codex, devin, claude, opencode, omp-gpt61-sol, omp-gpt6-luna, omp-gemini38,
omp-space-bunny, omp-muse-spark, omp-qwen38max, omp-glm53-flash.
External CLIs use their configured default model unless a --*-model override is passed.

Every pinned OMP model was identity-verified with agentic-consensus-model-probe.sh.
Re-verify with that script after touching this file; a retired model id silently
fuzzy-matches onto an older generation instead of failing.
See docs/problems/agentic-consensus-panel-anchor-rot-2026-10-03.md.

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
                                Default: codex,devin,claude,opencode,omp-gpt61-sol,omp-gpt6-luna,omp-gemini38,omp-space-bunny,omp-muse-spark,omp-qwen38max,omp-glm53-flash
  --exclude LIST                Comma-separated agent ids to skip.
  --list-agents                 Print supported agent ids and exit.

  Supported ids beyond the default panel:
    cursor            Cursor `agent` CLI. Quota-gated: the account must have
                      Agent usage left, otherwise it exits with
                      "ActionRequiredError: You've hit your usage limit".
    omp-gpt6-sol      Previous flagship; still current generation. Kept because
                      in-flight /goal files pass it in --include.
    omp-gpt6-astra    GPT-6 balanced tier (replaces the retired gpt-6-terra).
    omp-claude-opus46 Anthropic route via antigravity, independent of the
                      `claude` CLI's credential path.
    omp-grok47        xAI family, routed via opencode-go. The cursor/* provider
                      cannot name models on a free plan (429).
    omp-deepseek-v4-pro, omp-kimi-k3, omp-minimax-m3
                      Additional independent families for the full sub-panel.

Model overrides, optional:
  --codex-model MODEL           Pass -m MODEL to codex exec.
  --devin-model MODEL           Pass --model MODEL to devin.
  --claude-model MODEL          Pass --model MODEL to claude. Default: opus.
                                 fable is request-only.
  --cursor-model MODEL          Pass --model MODEL to Cursor agent.
  --opencode-model MODEL        Pass -m MODEL to opencode run.
  --omp-gpt61-sol-model MODEL   Default: openai-codex/gpt-6.1-sol.
  --omp-gpt6-sol-model MODEL    Default: openai-codex/gpt-6-sol.
  --omp-gpt6-astra-model MODEL  Default: openai-codex/gpt-6-astra.
  --omp-gpt6-luna-model MODEL   Default: openai-codex/gpt-6-luna.
  --omp-gemini-model MODEL      Default: google-antigravity/gemini-3.8-flash.
  --omp-claude-opus46-model MODEL Default: google-antigravity/claude-opus-4-6.
  --omp-space-bunny-model MODEL Default: opencode-go/space-bunny-free.
  --omp-muse-spark-model MODEL  Default: opencode-go/muse-spark-1.3-contributor.
  --omp-qwen38max-model MODEL   Default: opencode-go/qwen3.8-max.
  --omp-glm53-flash-model MODEL Default: opencode-go/glm-5.3-flash.
  --omp-grok47-model MODEL      Default: opencode-go/grok-4.7.
  --omp-deepseek-v4-pro-model MODEL Default: opencode-go/deepseek-v4-pro.
  --omp-kimi-k3-model MODEL     Default: opencode-go/kimi-k3.
  --omp-minimax-m3-model MODEL  Default: opencode-go/minimax-m3.

  Thinking levels, each with its default:
  --omp-gpt61-sol-thinking LEVEL      xhigh
  --omp-gpt6-sol-thinking LEVEL       medium
  --omp-gpt6-astra-thinking LEVEL     high
  --omp-gpt6-luna-thinking LEVEL      max
  --omp-gemini-thinking LEVEL         high
  --omp-claude-opus46-thinking LEVEL  high
  --omp-space-bunny-thinking LEVEL    high
  --omp-muse-spark-thinking LEVEL     high
  --omp-qwen38max-thinking LEVEL      high
  --omp-glm53-flash-thinking LEVEL    high
  --omp-grok47-thinking LEVEL         high
  --omp-deepseek-v4-pro-thinking LEVEL high
  --omp-kimi-k3-thinking LEVEL        high
  --omp-minimax-m3-thinking LEVEL     high

Execution:
  --cwd DIR                     Working directory/context root. Default: current directory.
  --out-dir DIR                 Output directory. Default: $CWD/.agentic-consensus/agentic-consensus-YYYYmmdd-HHMMSS.
  --sequential                  Run agents one at a time. Default: parallel.
  --dry-run                     Print commands but do not run them.
  --keep-going                  Return 0 if at least one agent succeeds. Default: fail if any selected agent fails.
  --no-tools-omp                Add --no-tools to OMP runs. Default: OMP tools enabled.
  --timeout-seconds N           Hard deadline for each agent. Default: 1200.
  --help                       Show this help.

Output:
  <out-dir>/prompt.md           Exact prompt sent to agents.
  <out-dir>/manifest.tsv        agent, status, exit code, output path, command.
  <out-dir>/<agent>.out         stdout/stderr for each successful/failed run.
  <out-dir>/<agent>.cmd         shell-quoted command for reproducibility.
USAGE
}

DEFAULT_INCLUDE="codex,devin,claude,opencode,omp-gpt61-sol,omp-gpt6-luna,omp-gemini38,omp-space-bunny,omp-muse-spark,omp-qwen38max,omp-glm53-flash"
SUPPORTED_AGENTS=(codex devin claude cursor opencode omp-gpt61-sol omp-gpt6-sol omp-gpt6-astra omp-gpt6-luna omp-gemini38 omp-claude-opus46 omp-space-bunny omp-muse-spark omp-qwen38max omp-glm53-flash omp-grok47 omp-deepseek-v4-pro omp-kimi-k3 omp-minimax-m3)

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
DEBUG_PERMISSIONS=0
TIMEOUT_SECONDS=1200
MODE="convergent"
LENSES=""
LENS_LIST=()

CODEX_MODEL=""
DEVIN_MODEL=""
CLAUDE_MODEL="opus"
CURSOR_MODEL=""
OPENCODE_MODEL=""
OMP_GPT61_SOL_MODEL="openai-codex/gpt-6.1-sol"
OMP_GPT6_SOL_MODEL="openai-codex/gpt-6-sol"
OMP_GPT6_ASTRA_MODEL="openai-codex/gpt-6-astra"
OMP_GPT6_LUNA_MODEL="openai-codex/gpt-6-luna"
OMP_GEMINI38_MODEL="google-antigravity/gemini-3.8-flash"
OMP_CLAUDE_OPUS46_MODEL="google-antigravity/claude-opus-4-6"
OMP_SPACE_BUNNY_MODEL="opencode-go/space-bunny-free"
OMP_MUSE_SPARK_MODEL="opencode-go/muse-spark-1.3-contributor"
OMP_QWEN38MAX_MODEL="opencode-go/qwen3.8-max"
OMP_GLM53_FLASH_MODEL="opencode-go/glm-5.3-flash"
OMP_GROK47_MODEL="opencode-go/grok-4.7"
OMP_DEEPSEEK_V4_PRO_MODEL="opencode-go/deepseek-v4-pro"
OMP_KIMI_K3_MODEL="opencode-go/kimi-k3"
OMP_MINIMAX_M3_MODEL="opencode-go/minimax-m3"
OMP_GPT61_SOL_THINKING="xhigh"
OMP_GPT6_SOL_THINKING="medium"
OMP_GPT6_ASTRA_THINKING="high"
OMP_GPT6_LUNA_THINKING="max"
OMP_GEMINI38_THINKING="high"
OMP_CLAUDE_OPUS46_THINKING="high"
OMP_SPACE_BUNNY_THINKING="high"
OMP_MUSE_SPARK_THINKING="high"
OMP_QWEN38MAX_THINKING="high"
OMP_GLM53_FLASH_THINKING="high"
OMP_GROK47_THINKING="high"
OMP_DEEPSEEK_V4_PRO_THINKING="high"
OMP_KIMI_K3_THINKING="high"
OMP_MINIMAX_M3_THINKING="high"

# Self-check: every supported omp-* id must resolve to a pinned model and
# thinking level under the mechanical name mapping used by build_cmd. This is
# the guard against the drift class in
# docs/problems/agentic-consensus-panel-anchor-rot-2026-10-03.md -- an anchor
# whose id no longer matches its pin would otherwise run against an empty
# --model and fuzzy-match onto whatever happened to be closest.
for _agent in "${SUPPORTED_AGENTS[@]}"; do
  case "$_agent" in
    omp-*)
      _stem="$(printf '%s' "$_agent" | tr '[:lower:]-' '[:upper:]_')"
      eval "_pin_model=\"\${${_stem}_MODEL:-}\""
      eval "_pin_thinking=\"\${${_stem}_THINKING:-}\""
      if [[ -z "$_pin_model" ]]; then
        echo "runner self-check: $_agent has no \$${_stem}_MODEL pin" >&2
        exit 2
      fi
      if [[ -z "$_pin_thinking" ]]; then
        echo "runner self-check: $_agent has no \$${_stem}_THINKING pin" >&2
        exit 2
      fi
      ;;
  esac
done
unset _agent _stem _pin_model _pin_thinking

# flag-suffix:variable table for every OMP anchor's --*-model / --*-thinking
# override. Consumed by the `--omp-*)` parse arm below.
OMP_FLAG_VARS=(
  "omp-gpt61-sol-model:OMP_GPT61_SOL_MODEL"
  "omp-gpt61-sol-thinking:OMP_GPT61_SOL_THINKING"
  "omp-gpt6-sol-model:OMP_GPT6_SOL_MODEL"
  "omp-gpt6-sol-thinking:OMP_GPT6_SOL_THINKING"
  "omp-gpt6-astra-model:OMP_GPT6_ASTRA_MODEL"
  "omp-gpt6-astra-thinking:OMP_GPT6_ASTRA_THINKING"
  "omp-gpt6-luna-model:OMP_GPT6_LUNA_MODEL"
  "omp-gpt6-luna-thinking:OMP_GPT6_LUNA_THINKING"
  "omp-gemini38-model:OMP_GEMINI38_MODEL"
  "omp-gemini38-thinking:OMP_GEMINI38_THINKING"
  "omp-claude-opus46-model:OMP_CLAUDE_OPUS46_MODEL"
  "omp-claude-opus46-thinking:OMP_CLAUDE_OPUS46_THINKING"
  "omp-space-bunny-model:OMP_SPACE_BUNNY_MODEL"
  "omp-space-bunny-thinking:OMP_SPACE_BUNNY_THINKING"
  "omp-muse-spark-model:OMP_MUSE_SPARK_MODEL"
  "omp-muse-spark-thinking:OMP_MUSE_SPARK_THINKING"
  "omp-qwen38max-model:OMP_QWEN38MAX_MODEL"
  "omp-qwen38max-thinking:OMP_QWEN38MAX_THINKING"
  "omp-glm53-flash-model:OMP_GLM53_FLASH_MODEL"
  "omp-glm53-flash-thinking:OMP_GLM53_FLASH_THINKING"
  "omp-grok47-model:OMP_GROK47_MODEL"
  "omp-grok47-thinking:OMP_GROK47_THINKING"
  "omp-deepseek-v4-pro-model:OMP_DEEPSEEK_V4_PRO_MODEL"
  "omp-deepseek-v4-pro-thinking:OMP_DEEPSEEK_V4_PRO_THINKING"
  "omp-kimi-k3-model:OMP_KIMI_K3_MODEL"
  "omp-kimi-k3-thinking:OMP_KIMI_K3_THINKING"
  "omp-minimax-m3-model:OMP_MINIMAX_M3_MODEL"
  "omp-minimax-m3-thinking:OMP_MINIMAX_M3_THINKING"
)


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
    --omp-*)
      # Table-driven: one entry per anchor, instead of one hand-written case
      # per flag. Adding, renaming or dropping an anchor is a single-line edit
      # here plus one line in build_cmd, which is what keeps the flag surface
      # from drifting out of sync with the panel.
      key="${1#--}"
      for entry in "${OMP_FLAG_VARS[@]}"; do
        if [[ "$key" == "${entry%%:*}" ]]; then
          [[ $# -ge 2 ]] || { echo "$1 requires a value" >&2; exit 2; }
          varname="${entry##*:}"
          printf -v "$varname" '%s' "$2"
          shift 2
          continue 2
        fi
      done
      echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
    --debug) DEBUG_PERMISSIONS=1; shift ;;
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
      if [[ "$DEBUG_PERMISSIONS" -eq 1 ]]; then
        CMD=(codex exec --cd "$CWD" --sandbox workspace-write -c 'approval_policy="on-failure"' --ephemeral --skip-git-repo-check)
      else
        CMD=(codex exec --cd "$CWD" --sandbox read-only -c 'approval_policy="never"' --ephemeral --skip-git-repo-check)
      fi
      [[ -n "$CODEX_MODEL" ]] && CMD+=(-m "$CODEX_MODEL")
      CMD+=("$AGENT_PROMPT") ;;
    devin)
      CMD=(devin --permission-mode auto --respect-workspace-trust false)
      [[ -n "$DEVIN_MODEL" ]] && CMD+=(--model "$DEVIN_MODEL")
      CMD+=(-p "$AGENT_PROMPT") ;;
    claude)
      if [[ "$DEBUG_PERMISSIONS" -eq 1 ]]; then
        CMD=(claude -p --output-format text --permission-mode auto --no-session-persistence)
      else
        CMD=(claude -p --output-format text --permission-mode plan --no-session-persistence)
      fi
      [[ -n "$CLAUDE_MODEL" ]] && CMD+=(--model "$CLAUDE_MODEL")
      CMD+=("$AGENT_PROMPT") ;;
    cursor)
      if [[ "$DEBUG_PERMISSIONS" -eq 1 ]]; then
        CMD=(agent --print --output-format text --trust --force --approve-mcps --workspace "$CWD")
      else
        CMD=(agent --print --output-format text --mode ask --trust --force --approve-mcps --workspace "$CWD")
      fi
      [[ -n "$CURSOR_MODEL" ]] && CMD+=(--model "$CURSOR_MODEL")
      CMD+=("$AGENT_PROMPT") ;;
    opencode)
      # `opencode run --dir` is gone in opencode v2.x; it exits 1 printing
      # usage. run_one already cd's into $CWD, so the working directory is
      # correct without the flag.
      CMD=(opencode run)
      [[ -n "$OPENCODE_MODEL" ]] && CMD+=(-m "$OPENCODE_MODEL")
      CMD+=("$AGENT_PROMPT") ;;
    omp-*)
      # Anchor id -> pin variable is mechanical: uppercase, '-' becomes '_',
      # suffix _MODEL / _THINKING. asserted at startup so a renamed id fails
      # loudly there instead of running with an empty --model here.
      stem="$(printf '%s' "$agent" | tr '[:lower:]-' '[:upper:]_')"
      eval "model=\"\${${stem}_MODEL:-}\""
      eval "thinking=\"\${${stem}_THINKING:-}\""
      if [[ -z "$model" || -z "$thinking" ]]; then
        echo "No model pin for $agent" >&2
        return 2
      fi
      CMD=(omp -p --mode text --model "$model" --thinking "$thinking" --no-session --max-time "$TIMEOUT_SECONDS" --auto-approve)
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
