#!/usr/bin/env bash
# Post-cutover writer-purity gate (mission-2 measure: writer_vocabulary_purity).
#
# The V1 desk vocabulary is retired: no production source may emit V1 desk
# tokens (super, co-super, cosuper, coagent, co-agent, co_super, researcher,
# researchers, research-agent, web-research, web-researcher, cosuper-coding,
# co-super-coding) as role/profile values, agent-ID prefixes, YAML role ids,
# or TOML role sections. The frozen V1 decoder and the migration machinery are
# the only sanctioned carriers and are allowlisted by path.
#
# Frozen compound protocol identifiers (co-super-open, researcher_update,
# persistent-super-recovery, assign_co_super, open_persistent_super, ...) are
# not desk vocabulary and are out of scope by construction.
#
# Usage: scripts/check-v1-writer-purity.sh
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

# Frozen V1 decoder + migration machinery: the only files allowed to carry
# V1 tokens (they define the historic mapping, they never serve live writes).
ALLOWLIST='internal/vocabmigrate/|internal/store/vocab_migrate|internal/computerevent/decode|internal/store/vocab_drill'

FAIL=0
fail() { echo "writer-purity FAIL: $*" >&2; FAIL=1; }

# 1. Quoted V1 desk tokens (string literals and agent-ID prefixes) in
#    production sources.
QUOTED="$(grep -rn -E '"(super|co-super|cosuper|coagent|co-agent|co_super|researcher|researchers|research-agent|web-research|web-researcher|cosuper-coding|co-super-coding)(:|")' \
  --include='*.go' --include='*.ts' --include='*.svelte' --include='*.yaml' --include='*.toml' \
  internal cmd frontend/src 2>/dev/null \
  | grep -v '_test\.' | grep -vE "$ALLOWLIST" || true)"
if [[ -n "$QUOTED" ]]; then
  fail "V1 desk token literals in production sources:"
  echo "$QUOTED" >&2
fi

# 2. YAML role-id fields carrying V1 desk tokens.
YAML_ROLES="$(grep -rn -E '^role:\s*(super|co-super|cosuper|coagent|co-agent|co_super|researcher|researchers|research-agent|web-research|web-researcher)\s*$' \
  --include='*.yaml' --include='*.yml' internal cmd 2>/dev/null \
  | grep -vE "$ALLOWLIST" || true)"
if [[ -n "$YAML_ROLES" ]]; then
  fail "V1 desk token in YAML role id:"
  echo "$YAML_ROLES" >&2
fi

# 3. TOML model-policy role sections carrying V1 desk tokens.
TOML_ROLES="$(grep -rn -E '^\[roles\.(super|co-super|cosuper|coagent|co-agent|co_super|researcher|researchers|research-agent|web-research|web-researcher)\]' \
  --include='*.toml' --include='*.go' internal cmd 2>/dev/null \
  | grep -v '_test\.' | grep -vE "$ALLOWLIST" || true)"
if [[ -n "$TOML_ROLES" ]]; then
  fail "V1 desk token in TOML role section:"
  echo "$TOML_ROLES" >&2
fi

if [[ "$FAIL" != "0" ]]; then
  echo "writer-purity: FAILED" >&2
  exit 1
fi
echo "writer-purity: PASS"
