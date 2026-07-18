#!/usr/bin/env bash
set -u -o pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
RUNNER="$ROOT/agentic-consensus-runner.sh"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/agentic-consensus-test.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT
BASE_PATH="/usr/bin:/bin"
TESTS=0

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { TESTS=$((TESTS + 1)); }
assert_file_contains() {
  local file="$1" text="$2"
  grep -F -- "$text" "$file" >/dev/null || fail "$file does not contain: $text"
}
assert_eq() {
  [[ "$1" == "$2" ]] || fail "expected '$2', got '$1'${3:+ ($3)}"
}
manifest_field() {
  local manifest="$1" agent="$2" field="$3"
  awk -F '\t' -v wanted="$agent" -v name="$field" '
    NR == 1 { for (i=1; i<=NF; i++) column[$i]=i; next }
    $1 == wanted { print $column[name]; exit }
  ' "$manifest"
}
manifest_selected_count() {
  awk -F '\t' 'NR > 1 && $5 == "yes" && ($12 == "dry-run" || $12 == "ok" || $12 == "failed" || $12 == "timed-out") { count++ } END { print count+0 }' "$1"
}

make_bin_dir() {
  local dir="$1"
  mkdir -p "$dir"
  cat > "$dir/timeout" <<'STUB'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then echo "timeout (GNU coreutils) 9.0"; exit 0; fi
[[ "${1:-}" == --signal=* ]] && shift
shift
exec "$@"
STUB
  chmod +x "$dir/timeout"
}
make_cli() {
  local dir="$1" name="$2"
  cat > "$dir/$name" <<'STUB'
#!/usr/bin/env bash
name="$(basename "$0")"
if [[ "${SILENT_CLI:-}" == "$name" ]]; then
  printf '   \n\t\n'
  exit 0
fi
echo "$name response"
if [[ "${FAIL_CLI:-}" == "$name" ]]; then
  exit "${FAIL_CLI_CODE:-1}"
fi
exit 0
STUB
  chmod +x "$dir/$name"
}
make_omp() {
  local dir="$1"
  cat > "$dir/omp" <<'STUB'
#!/usr/bin/env bash
if [[ "${1:-}" == "models" ]]; then
  cat "$OMP_MODELS_FILE"
  exit "${OMP_MODELS_EXIT:-0}"
fi
model=""
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--model" ]]; then model="$2"; shift 2; else shift; fi
done
if [[ "$model" == "${CHATTER_MODEL:-}" ]]; then
  printf '  \n\r⠋ Working...\rWorking…\n\t\n'
  exit 0
fi
echo "omp response model=$model"
[[ "$model" == "${FAIL_MODEL:-}" ]] && exit 9
exit 0
STUB
  chmod +x "$dir/omp"
}
write_all_models() {
  cat > "$1" <<'MODELS'
openai-codex (1)
│ gpt-5.5 │
google-antigravity (1)
│ gemini-3.5-flash │
cursor (1)
│ cursor-grok-4.5-high │
MODELS
}

# Help, docs, and catalog expose exactly the same replacement member surface.
expected_list="$(printf '%s\n' codex devin claude cursor opencode omp-gpt55 omp-gemini35 omp-cursor-grok45)"
list="$($RUNNER --list-agents)" || fail "--list-agents failed"
assert_eq "$list" "$expected_list" "supported member catalog"
help="$($RUNNER --help)" || fail "--help failed"
for member in $expected_list; do
  [[ "$help" == *"$member"* ]] || fail "$member missing from help"
  grep -F -- "$member" "$ROOT/SKILL.md" >/dev/null || fail "$member missing from docs"
done
[[ "$help" == *"fast, thorough, or deep"* ]] || fail "profiles missing from help"
pass

# Default selection adapts around missing CLIs and an absent OMP model, while selecting exact Grok.
bin1="$TMP/bin-missing"
make_bin_dir "$bin1"
make_cli "$bin1" codex
make_omp "$bin1"
models1="$TMP/models-missing.txt"
cat > "$models1" <<'MODELS'
google-antigravity (1)
│ gemini-3.5-flash │
cursor (1)
│ cursor-grok-4.5-high │
MODELS
out1="$TMP/out-missing"
PATH="$bin1:$BASE_PATH" OMP_MODELS_FILE="$models1" "$RUNNER" --dry-run --prompt review --out-dir "$out1" >/dev/null || fail "adaptive dry-run failed"
assert_eq "$(manifest_field "$out1/manifest.tsv" omp-cursor-grok45 effective_model)" "cursor/cursor-grok-4.5-high" "exact Grok model"
assert_eq "$(manifest_field "$out1/manifest.tsv" omp-cursor-grok45 status)" "dry-run" "Grok selected"
assert_eq "$(manifest_field "$out1/manifest.tsv" omp-gpt55 preflight_status)" "missing-model" "missing OMP model"
assert_eq "$(manifest_field "$out1/manifest.tsv" devin preflight_status)" "missing-cli" "missing external CLI"
assert_file_contains "$out1/omp-cursor-grok45.cmd" '--model cursor/cursor-grok-4.5-high'
assert_file_contains "$out1/preflight-omp-models.txt" 'cursor-grok-4.5-high'
pass

# Profiles have deterministic, materially different breadth and deadlines.
bin2="$TMP/bin-all"
make_bin_dir "$bin2"
for cli in codex devin claude agent opencode; do make_cli "$bin2" "$cli"; done
make_omp "$bin2"
models2="$TMP/models-all.txt"
write_all_models "$models2"
out_fast="$TMP/out-fast"
out_deep="$TMP/out-deep"
PATH="$bin2:$BASE_PATH" OMP_MODELS_FILE="$models2" "$RUNNER" --dry-run --profile fast --prompt review --out-dir "$out_fast" >/dev/null || fail "fast dry-run failed"
PATH="$bin2:$BASE_PATH" OMP_MODELS_FILE="$models2" "$RUNNER" --dry-run --profile thorough --prompt review --out-dir "$out_deep" >/dev/null || fail "thorough dry-run failed"
assert_eq "$(manifest_selected_count "$out_fast/manifest.tsv")" "3" "fast breadth"
assert_eq "$(manifest_selected_count "$out_deep/manifest.tsv")" "8" "thorough breadth"
assert_eq "$(manifest_field "$out_fast/manifest.tsv" omp-gemini35 deadline_seconds)" "60" "fast deadline"
assert_eq "$(manifest_field "$out_deep/manifest.tsv" omp-gemini35 deadline_seconds)" "240" "thorough deadline"
pass

# Adaptive selection avoids an exact effective-model duplicate.
out_dup="$TMP/out-duplicate"
PATH="$bin2:$BASE_PATH" OMP_MODELS_FILE="$models2" "$RUNNER" --dry-run --profile thorough \
  --codex-model openai-codex/gpt-5.5 --prompt review --out-dir "$out_dup" >/dev/null || fail "duplicate-route dry-run failed"
assert_eq "$(manifest_field "$out_dup/manifest.tsv" omp-gpt55 selected)" "no" "duplicate route selection"
assert_file_contains "$out_dup/manifest.tsv" 'duplicate effective route model:openai-codex/gpt-5.5'
pass

# Explicit include order is preserved, exclusion is visible, and profile limits are bypassed.
out_explicit="$TMP/out-explicit"
PATH="$bin2:$BASE_PATH" OMP_MODELS_FILE="$models2" "$RUNNER" --dry-run --profile fast \
  --include omp-cursor-grok45,codex,omp-gemini35,claude --exclude codex \
  --prompt review --out-dir "$out_explicit" >/dev/null || fail "explicit include/exclude failed"
order="$(awk -F '\t' 'NR > 1 { print $1 }' "$out_explicit/manifest.tsv" | paste -sd, -)"
assert_eq "$order" "omp-cursor-grok45,codex,omp-gemini35,claude" "explicit order"
assert_eq "$(manifest_field "$out_explicit/manifest.tsv" codex selected)" "no" "explicit exclusion"
assert_eq "$(manifest_field "$out_explicit/manifest.tsv" claude status)" "dry-run" "profile limit bypass"
pass

# Partial runtime failure is explicit in the manifest and keep-going succeeds with one good member.
out_partial="$TMP/out-partial"
set +e
PATH="$bin2:$BASE_PATH" OMP_MODELS_FILE="$models2" FAIL_CLI=codex FAIL_CLI_CODE=7 "$RUNNER" --sequential --keep-going \
  --include codex,omp-cursor-grok45 --prompt review --out-dir "$out_partial" > "$TMP/partial.stdout" 2>&1
partial_code=$?
set -e
assert_eq "$partial_code" "0" "keep-going exit"
assert_eq "$(manifest_field "$out_partial/manifest.tsv" codex status)" "failed" "failed member telemetry"
assert_eq "$(manifest_field "$out_partial/manifest.tsv" codex exit_code)" "7" "failed exit code"
assert_eq "$(manifest_field "$out_partial/manifest.tsv" omp-cursor-grok45 status)" "ok" "successful member telemetry"
assert_file_contains "$TMP/partial.stdout" 'Succeeded: 1'
assert_file_contains "$TMP/partial.stdout" 'Failed/skipped selected members: 1'
pass

# Exit-zero whitespace and runner chatter are failures, not successful panel opinions.
out_empty="$TMP/out-non-substantive"
set +e
PATH="$bin2:$BASE_PATH" OMP_MODELS_FILE="$models2" SILENT_CLI=codex \
  CHATTER_MODEL=cursor/cursor-grok-4.5-high "$RUNNER" --sequential \
  --include codex,omp-cursor-grok45 --prompt review --out-dir "$out_empty" > "$TMP/non-substantive.stdout" 2>&1
empty_code=$?
set -e
assert_eq "$empty_code" "1" "non-substantive panel exit"
assert_eq "$(manifest_field "$out_empty/manifest.tsv" codex status)" "failed-non-substantive-output" "whitespace-only telemetry"
assert_eq "$(manifest_field "$out_empty/manifest.tsv" codex exit_code)" "65" "whitespace-only validation code"
assert_eq "$(manifest_field "$out_empty/manifest.tsv" omp-cursor-grok45 status)" "failed-non-substantive-output" "runner chatter telemetry"
assert_eq "$(manifest_field "$out_empty/manifest.tsv" omp-cursor-grok45 exit_code)" "65" "runner chatter validation code"
assert_file_contains "$TMP/non-substantive.stdout" 'Succeeded: 0'
assert_file_contains "$TMP/non-substantive.stdout" 'Failed/skipped selected members: 2'
pass

echo "PASS: $TESTS focused agentic-consensus runner tests"
