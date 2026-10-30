#!/usr/bin/env bash
# The escrow key's KEK never travels in the daily backup (station FP9,
# docs/archive/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md, "The escrow
# key"). A sealed escrow key in the backup is worth nothing if the key that
# opens it rides along.
#
# Failure modes (written before the exclusion):
#  1. The KEK under $ROOT/secrets (a backed-up path) is not excluded.
#  2. A KEK placed elsewhere under a backed-up path, under the same name, is
#     not excluded (the exclusion must be a name pattern, not one path).
#  3. The sealed escrow key itself (key-escrow-key) is excluded too: it must
#     stay in the backup, it is useless without the KEK.
#
# Run: bash scripts/off-host-backup-kek-exclude-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
BASH_BIN="$(command -v bash)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "$OUT"; fails=$((fails + 1)); fi; }

mkdir -p "$WORK/root/platform-artifacts" "$WORK/root/secrets" "$WORK/secrets"
OUT="$(env -i PATH="$PATH" OFFHOST_DATA_ROOT="$WORK/root" OFFHOST_SECRETS_DIR="$WORK/secrets" \
  "$BASH_BIN" "$SCRIPT" plan 2>&1 </dev/null)"
EXCLUDED="$(printf '%s\n' "$OUT" | awk '/^excluded:/{on=1;next} /^[a-z]/{on=0} on{sub(/^  /,""); print}')"

check "the excluded list is printed" "$([ -n "$EXCLUDED" ]; echo $?)"
# restic treats a pattern without a slash as a name matched at any depth.
check "a key-escrow-kek name pattern is excluded" \
  "$(printf '%s\n' "$EXCLUDED" | grep -qx 'key-escrow-kek\*'; echo $?)"
check "the sealed escrow key itself is not excluded" \
  "$(! printf '%s\n' "$EXCLUDED" | grep -q 'key-escrow-key'; echo $?)"

# B3 (bunch 4b review): the KEK the units actually load is excluded whatever
# its file is called. A stub systemctl reports a key-escrow-kek credential
# whose source is /var/lib/go-choir/secrets/custom-kek, a KEK path set in the
# environment, and an unrelated credential that must stay in the backup.
mkdir -p "$WORK/bin"
cat > "$WORK/bin/systemctl" <<'STUB'
#!/bin/sh
[ "$1" = show ] || exit 1
case "$*" in
*go-choir-corpusd*)
  echo "LoadCredential=key-escrow-kek:/var/lib/go-choir/secrets/custom-kek"
  echo "LoadCredential=choir-dolt-password:/var/lib/go-choir/secrets/dolt-pw"
  echo "Environment=CHOIR_KEY_ESCROW_SEAL=require CHOIR_KEY_ESCROW_KEK_PATH=%d/key-escrow-kek" ;;
*go-choir-checkpointd*)
  echo "LoadCredential="
  echo "Environment=FOO=1 CHOIR_KEY_ESCROW_KEK_PATH=/srv/keys/other-kek" ;;
esac
STUB
chmod +x "$WORK/bin/systemctl"
OUT="$(env -i PATH="$WORK/bin:$PATH" OFFHOST_DATA_ROOT="$WORK/root" OFFHOST_SECRETS_DIR="$WORK/secrets" \
  "$BASH_BIN" "$SCRIPT" plan 2>&1 </dev/null)"
EXCLUDED="$(printf '%s\n' "$OUT" | awk '/^excluded:/{on=1;next} /^[a-z]/{on=0} on{sub(/^  /,""); print}')"
check "B3 the source of a key-escrow-kek credential is excluded" \
  "$(printf '%s\n' "$EXCLUDED" | grep -qx '/var/lib/go-choir/secrets/custom-kek'; echo $?)"
check "B3 an absolute CHOIR_KEY_ESCROW_KEK_PATH is excluded" \
  "$(printf '%s\n' "$EXCLUDED" | grep -qx '/srv/keys/other-kek'; echo $?)"
check "B3 an unrelated credential stays in the backup" \
  "$(! printf '%s\n' "$EXCLUDED" | grep -q 'dolt-pw'; echo $?)"
check "B3 a credential-directory path (%d/...) is not added" \
  "$(! printf '%s\n' "$EXCLUDED" | grep -q '^%d'; echo $?)"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed"; exit 1; }
