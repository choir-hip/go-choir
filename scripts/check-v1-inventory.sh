#!/usr/bin/env bash
# Standing CI gate for the mission-2 V1 desk-vocabulary inventory (acceptance item 1).
#
# Default (CI) mode checks inventory integrity:
#   1. corpus manifest is current (no new/removed production paths);
#   2. no tracked file uses an extension outside the covered + known-tooling sets;
#   3. every frozen anchor text is still present in its file;
#   4. every live sweep hit is covered by a frozen anchor (set difference fails);
#   5. no non-allowlisted unknown-verification row exists.
# Rows allowlisted as charter-pending (owner token classification, mapping-table
# item 4) warn in CI mode and fail only under --freeze, which is the acceptance
# proof for the freeze: zero unknown rows of any kind.
#
# Usage: scripts/check-v1-inventory.sh [--freeze]
set -euo pipefail

MODE="ci"
if [[ "${1:-}" == "--freeze" ]]; then MODE="freeze"; fi

ROOT="$(git rev-parse --show-toplevel)"
MANIFEST="$ROOT/docs/evidence/choir-rlm-v1-inventory-corpus-2026-09-10.txt"
FROZEN="$ROOT/docs/evidence/choir-rlm-v1-inventory-2026-09-10.json"
QUERIES="$ROOT/scripts/v1-inventory-queries.py"
FAIL=0

warn() { echo "inventory-gate: $*" >&2; }
fail() { echo "inventory-gate FAIL: $*" >&2; FAIL=1; }

# --- 1+2. corpus + extension tripwire ---------------------------------------
CORPUS_JSON="$(mktemp)"
trap 'rm -f "$CORPUS_JSON"' EXIT
python3 "$ROOT/scripts/v1-inventory-corpus.py" "$ROOT" > "$CORPUS_JSON"
test -s "$CORPUS_JSON" || { echo "inventory-gate FAIL: corpus python produced no output" >&2; exit 1; }

MANIFEST_FILES="$(mktemp)"
trap 'rm -f "$CORPUS_JSON" "$MANIFEST_FILES"' EXIT
grep -v '^#' "$MANIFEST" | grep -v '^$' | sort > "$MANIFEST_FILES"
DIFF_OUT="$(python3 - "$CORPUS_JSON" "$MANIFEST_FILES" <<'PYEOF'
import json, sys
live = json.load(open(sys.argv[1]))["prod"]
frozen = [l.strip() for l in open(sys.argv[2]) if l.strip()]
ls, fs = set(live), set(frozen)
for f in sorted(ls - fs):
    print(f"NEW_UNCLASSIFIED:{f}")
for f in sorted(fs - ls):
    print(f"REMOVED_FROM_TREE:{f}")
PYEOF
)"
if [[ -n "$DIFF_OUT" ]]; then
  while IFS= read -r line; do fail "corpus drift: $line (refresh the manifest in a Define update)"; done <<< "$DIFF_OUT"
else
  echo "inventory-gate: corpus manifest current"
fi

BAD_EXT="$(python3 -c "import json; print('\n'.join(json.load(open('$CORPUS_JSON'))['bad_ext']))")"
if [[ -n "$BAD_EXT" ]]; then
  while IFS= read -r line; do fail "unclassified extension in scope: $line"; done <<< "$BAD_EXT"
else
  echo "inventory-gate: extension set clean"
fi
# --- 3+4+5. anchors, sweep coverage, unknowns -------------------------------
GATE_OUT="$(python3 "$QUERIES" "$MANIFEST" | python3 "$ROOT/scripts/v1-inventory-gate.py" "$FROZEN" "$MODE" || true)"
echo "$GATE_OUT" | grep '^WARN:' | while IFS= read -r line; do warn "$line"; done || true
FAIL_LINES="$(echo "$GATE_OUT" | grep '^FAIL:' || true)"
if [[ -n "$FAIL_LINES" ]]; then
  while IFS= read -r line; do fail "$line"; done <<< "$FAIL_LINES"
else
  echo "inventory-gate: anchors + sweep coverage clean"
fi
echo "$GATE_OUT" | grep '^ROWS:' || true

if [[ "$MODE" == "freeze" && "$FAIL" == "0" ]]; then
  echo "inventory-gate: FREEZE ACCEPTED (zero unknown rows)"
fi
if [[ "$FAIL" != "0" ]]; then
  echo "inventory-gate: FAILED (mode=$MODE)" >&2
  exit 1
fi
echo "inventory-gate: PASS (mode=$MODE)"
