#!/usr/bin/env bash
# Tool preflight for scripts/off-host-backup.sh. No network, no secrets:
# every tool the script calls is either a stub or absent.
#
# Failure modes (written before the preflight; problem record
# docs/problems/no-off-host-backup-2026-10-20.md section 13 item 5):
#
#  1. A subcommand starts with a tool missing and does part of its work
#     (a status write, a tunnel, staging, a key on disk): never. It
#     refuses first, exit 5.
#  2. The refusal does not say which tool is missing or where to get it:
#     it names the tool and the nix-shell line.
#  3. A subcommand refuses for a tool it does not use (freshness without
#     restic): never.
#  4. keygen without htpasswd writes keys and then stops, so a rerun
#     refuses because the keys exist: never. Without htpasswd and without
#     nix-shell it refuses before writing; with nix-shell it falls back.
#
# Run: bash scripts/off-host-backup-preflight-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
BASH_BIN="$(command -v bash)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0

# A PATH with only the basic tools, plus stubs named by the caller.
toolbox() { # dir, stub names...
  local dir="$1" t p; shift
  mkdir -p "$dir"
  for t in env date install cat head tr base64 seq mkdir rm basename dirname df awk sed grep tail mv cp mktemp ls sort sleep; do
    p="$(command -v "$t" 2>/dev/null)" && ln -sf "$p" "$dir/$t"
  done
  for t in "$@"; do printf '#!/bin/sh\nexit 0\n' > "$dir/$t"; chmod +x "$dir/$t"; done
}

# run name, subcommand, stub names... ; sets RC, ERR, ROOT
run() {
  local name="$1" sub="$2"; shift 2
  ROOT="$WORK/$name"
  mkdir -p "$ROOT/root" "$ROOT/secrets"
  toolbox "$ROOT/bin" "$@"
  ERR="$(cd "$ROOT" && env -i PATH="$ROOT/bin" OFFHOST_DATA_ROOT="$ROOT/root" \
    OFFHOST_SECRETS_DIR="$ROOT/secrets" OFFHOST_HOST=192.0.2.1 \
    "$BASH_BIN" "$SCRIPT" "$sub" 2>&1 </dev/null)"
  RC=$?
}
check() { # description, condition result (0 = pass)
  if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC out=$ERR"; fails=$((fails + 1)); fi
}
nothing_written() { [ -z "$(ls -A "$ROOT/root")" ] && [ -z "$(ls -A "$ROOT/secrets")" ]; }

# 1, 2: backup with no restic refuses before any status write.
run backup-no-restic backup ssh jq sqlite3 pg_dump pg_restore
check "backup without restic exits 5" "$([ "$RC" = 5 ]; echo $?)"
check "backup without restic names restic and nix-shell" "$(printf '%s' "$ERR" | grep -q 'restic' && printf '%s' "$ERR" | grep -q 'nix-shell -p'; echo $?)"
check "backup without restic writes nothing" "$(nothing_written; echo $?)"

# 1, 2: the case that happened: everything but sqlite3.
run backup-no-sqlite backup ssh restic jq pg_dump pg_restore
check "backup without sqlite3 exits 5" "$([ "$RC" = 5 ]; echo $?)"
check "backup without sqlite3 names sqlite3" "$(printf '%s' "$ERR" | grep -q 'sqlite3'; echo $?)"
check "backup without sqlite3 writes nothing" "$(nothing_written; echo $?)"

# 1, 2: no pg_dump or pg_restore: refuses before anything is written.
run backup-no-pg backup ssh restic jq sqlite3
check "backup without pg_dump exits 5" "$([ "$RC" = 5 ]; echo $?)"
check "backup without pg_dump names pg_dump and pg_restore" "$(printf '%s' "$ERR" | grep -q 'pg_dump' && printf '%s' "$ERR" | grep -q 'pg_restore'; echo $?)"
check "backup without pg_dump writes nothing" "$(nothing_written; echo $?)"

# 1: init and verify need the tunnel and restic.
run init-no-ssh init restic
check "init without ssh exits 5" "$([ "$RC" = 5 ]; echo $?)"
run verify-no-jq verify ssh restic
check "verify without jq exits 5" "$([ "$RC" = 5 ]; echo $?)"

# 3: freshness needs only jq; with no status it fails for that reason.
run freshness-jq-only freshness jq
check "freshness with only jq passes the preflight" "$([ "$RC" != 5 ]; echo $?)"
check "freshness reports the missing status, not a tool" "$(printf '%s' "$ERR" | grep -q 'never run'; echo $?)"

# 4: keygen without htpasswd or nix-shell refuses before any key exists.
run keygen-none keygen ssh-keygen
check "keygen without htpasswd or nix-shell exits 5" "$([ "$RC" = 5 ]; echo $?)"
check "keygen without htpasswd or nix-shell writes no key" "$(nothing_written; echo $?)"

# 4: with nix-shell it passes the preflight (then stops: no terminal).
run keygen-nixshell keygen ssh-keygen nix-shell
check "keygen falls back to nix-shell for htpasswd" "$([ "$RC" = 2 ] && printf '%s' "$ERR" | grep -q 'needs a terminal'; echo $?)"
check "keygen without a terminal writes no key" "$(nothing_written; echo $?)"

[ "$fails" = 0 ] && echo "all preflight checks passed" || { echo "$fails failed"; exit 1; }
