#!/usr/bin/env bash
# The provider keys never travel in the daily off-host backup (PR-13,
# docs/problems/provider-keys-ride-in-off-host-backup-2026-10-22.md). One
# backup secret would otherwise open every provider API key and the ChatGPT
# OAuth token set together with the data. Interim until the provider vault
# lands (owner R5 2026-10-23, "for now just kept out of backups"; owner
# 2026-10-26 02:4x, FP0's default: exclude by name, keep existing snapshots).
#
# Failure modes (written before the exclusion):
#  1. gateway-provider.env or codex-auth.json is still added as a backup path.
#  2. A copy under another backed-up path (secrets/, unit-env/) under the same
#     name still travels: the exclusion must be a name pattern, not one path.
#  3. The exclusion drops neighbours that must stay (maild.env, searxng.env,
#     gateway-identities.json, secrets/).
#
# Run: bash scripts/off-host-backup-provider-keys-exclude-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
BASH_BIN="$(command -v bash)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "$OUT"; fails=$((fails + 1)); fi; }

mkdir -p "$WORK/root/platform-artifacts" "$WORK/root/secrets" "$WORK/secrets"
for f in gateway-provider.env codex-auth.json maild.env searxng.env gateway-identities.json; do
  : > "$WORK/root/$f"
done
OUT="$(env -i PATH="$PATH" OFFHOST_DATA_ROOT="$WORK/root" OFFHOST_SECRETS_DIR="$WORK/secrets" \
  "$BASH_BIN" "$SCRIPT" plan 2>&1 </dev/null)"
section() { printf '%s\n' "$OUT" | awk -v h="$1:" '$0==h{on=1;next} /^[a-z]/{on=0} on{sub(/^  /,""); print}'; }
PATHS_OUT="$(section 'would back up')"
EXCLUDED="$(section 'excluded')"

check "gateway-provider.env is not a backup path" \
  "$(! printf '%s\n' "$PATHS_OUT" | grep -q 'gateway-provider.env'; echo $?)"
check "codex-auth.json is not a backup path" \
  "$(! printf '%s\n' "$PATHS_OUT" | grep -q 'codex-auth.json'; echo $?)"
check "gateway-provider.env is excluded by name at any depth" \
  "$(printf '%s\n' "$EXCLUDED" | grep -qx 'gateway-provider.env'; echo $?)"
check "codex-auth.json is excluded by name at any depth" \
  "$(printf '%s\n' "$EXCLUDED" | grep -qx 'codex-auth.json'; echo $?)"
for keep in maild.env searxng.env gateway-identities.json secrets; do
  check "$keep still travels" "$(printf '%s\n' "$PATHS_OUT" | grep -q "/$keep\$"; echo $?)"
done

[ "$fails" = 0 ] && echo "PASS" || { echo "$fails failure(s)"; exit 1; }
