#!/usr/bin/env bash
# The restore drill's "deletions" piece (round 3, B2 of the account-deletion
# review): the accounts deleted since a chosen snapshot are found by reading the
# NEWEST ledger in the repository against the CHOSEN (possibly older) auth.db.
#
# Failure modes (written before the piece):
#  1. The ledger is read from the chosen snapshot: an older snapshot's ledger
#     lacks every deletion since, so the check finds nothing.
#  2. auth.db is read from the newest snapshot: it already lacks the deleted
#     accounts, so the check finds nothing either.
#  3. The drill passes -reapply: a drill only reports; a real restore applies.
#  4. The check's exit status is lost: a drill that finds restored deleted
#     accounts reports green.
#  5. A restore with no ledger anywhere in the repository passes: it must fail.
#
# Run: bash scripts/off-host-restore-drill-deletions-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-restore-drill.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC out=$(printf '%s' "$OUT" | tail -6)"; fails=$((fails + 1)); fi; }

setup() { # name
  R="$WORK/$1"; mkdir -p "$R/bin" "$R/log" "$R/sec" "$R/drill"
  printf '#!/bin/sh\nexit 0\n' > "$R/bin/ssh"
  # restic restore <snap>[:path] --target T --include P ...: write what that snapshot holds.
  cat > "$R/bin/restic" <<EOF
#!/usr/bin/env bash
printf 'restic %s\n' "\$*" >> "$R/log/argv"
[ "\$2" = restore ] || [ "\$1" = restore ] || exit 0
snap=""; target=""
args=("\$@")
for ((i=0;i<\${#args[@]};i++)); do
  case "\${args[i]}" in restore) snap="\${args[i+1]}" ;; --target) target="\${args[i+1]}" ;; esac
done
st=/var/lib/go-choir/offhost-backup/staging/sqlite
mkdir -p "\$target\$st"
case "\$snap" in
  OLD) printf 'OLD-AUTH\n' > "\$target\$st/auth.db"; printf '{"user_hash":"h1","job_id":"j1"}\n' > "\$target\$st/account-deletion-ledger.jsonl" ;;
  latest) printf 'NEW-AUTH\n' > "\$target\$st/auth.db"; [ -z "\${FAKE_NO_LEDGER:-}" ] && printf '{"user_hash":"h1","job_id":"j1"}\n{"user_hash":"h2","job_id":"j2"}\n' > "\$target\$st/account-deletion-ledger.jsonl" ;;
esac
exit 0
EOF
  # The check: record the databases and ledger it was handed and what they hold.
  cat > "$R/bin/auth" <<EOF
#!/usr/bin/env bash
printf 'auth %s\n' "\$*" >> "$R/log/argv"
db=""; ledger=""
while [ "\$#" -gt 0 ]; do case "\$1" in -db) db="\$2"; shift ;; -ledger) ledger="\$2"; shift ;; esac; shift; done
echo "db=\$(cat "\$db")" >> "$R/log/auth-saw"; echo "ledger-lines=\$(grep -c '' "\$ledger")" >> "$R/log/auth-saw"
echo "ledger lines: 2; restored accounts to delete again: \${FAKE_FOUND:-0}"
[ "\${FAKE_FOUND:-0}" = 0 ] && exit 0 || exit 1
EOF
  printf '#!/bin/sh\necho "Filesystem 1K-blocks Used Available Use%% Mounted"\necho "x 1 1 99999999 1%% /var/lib"\n' > "$R/bin/df"
  chmod +x "$R/bin/"*
  printf x > "$R/sec/restic-password"; printf x > "$R/sec/rest-password"; printf x > "$R/sec/ssh-key"; printf x > "$R/sec/known-hosts"
}
run() { # snapshot, extra env...
  local snap="$1"; shift
  OUT="$(env PATH="$R/bin:$PATH" OFFHOST_SECRETS_DIR="$R/sec" OFFHOST_DRILL_DIR="$R/drill" OFFHOST_AUTH_BIN="$R/bin/auth" "$@" \
    bash "$SCRIPT" "$snap" deletions 2>&1)"; RC=$?
}

setup found; run OLD FAKE_FOUND=1
check "a finding is the drill's exit status (1)" "$([ "$RC" = 1 ]; echo $?)"
check "the ledger was restored from the newest snapshot (latest)" "$(grep -E '^restic .*restore latest .*account-deletion-ledger.jsonl' "$R/log/argv" >/dev/null; echo $?)"
check "auth.db was restored from the chosen snapshot" "$(grep -E '^restic .*restore OLD .*auth.db' "$R/log/argv" >/dev/null; echo $?)"
check "the check read the OLD auth.db" "$(grep -qx 'db=OLD-AUTH' "$R/log/auth-saw"; echo $?)"
check "the check read the NEWEST ledger (two lines)" "$(grep -qx 'ledger-lines=2' "$R/log/auth-saw"; echo $?)"
check "the drill did not apply anything (-reapply is not passed)" "$(! grep -q -- '-reapply' "$R/log/argv"; echo $?)"

setup none; run OLD FAKE_FOUND=0
check "nothing found: exit 0" "$([ "$RC" = 0 ]; echo $?)"

setup noledger; run OLD FAKE_NO_LEDGER=1
check "no ledger in the repository: the drill fails (not green)" "$([ "$RC" != 0 ]; echo $?)"
check "no ledger in the repository: it says so" "$(printf '%s' "$OUT" | grep -q 'no deletion ledger'; echo $?)"

[ "$fails" = 0 ] && echo "all ok" || { echo "$fails failed"; exit 1; }
