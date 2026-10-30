#!/usr/bin/env bash
# C4 rehearsal of the guest migration: copy a computer's data directory, run
# the guest Dolt -> SQLite migrator on the COPY, and print the differential
# report (rows and digests per table, tables and columns dropped or new). The
# original is never opened.
#
# Usage:
#   guest-migrate-dryrun.sh <data-dir> <store-basename> [scratch-dir]
#
#   data-dir        a directory holding the computer's runtime store, for
#                   example a mounted copy of d@e's data image, or the
#                   directory itself if you have already reflink-copied it
#   store-basename  the configured runtime store file name inside it (the
#                   legacy marker, e.g. choir.db); the Dolt workspace is the
#                   sibling <name>.texture directory and the new state file is
#                   state.db beside them (internal/store.StateDBPath)
#   scratch-dir     where the copy goes (default: a new mktemp directory)
#
# The copy uses reflinks where the filesystem has them (btrfs on Node B), so a
# multi-GB workspace costs no extra space. Leaves the scratch directory in
# place for inspection and prints how to remove it.
#
# Needs the repo toolchain (go) or CHOIR_MIGRATE_GUEST_BIN. The migrator opens
# the real store (store.Open) in the copy to create the schema, so run it on a
# tree whose store package is the SQLite one (after WP-A is merged).
set -euo pipefail

if [ $# -lt 2 ]; then
  sed -n 2,23p "$0"
  exit 2
fi
src="$1"; base="$2"
scratch="${3:-$(mktemp -d "${TMPDIR:-/tmp}/guest-migrate-dryrun.XXXXXX")}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"

[ -d "$src" ] || { echo "no such directory: $src" >&2; exit 2; }
mkdir -p "$scratch"
echo "copying $src -> $scratch" >&2
if cp -R --reflink=auto "$src/." "$scratch/" 2>/dev/null; then
  :
elif cp -Rc "$src/." "$scratch/" 2>/dev/null; then # macOS clonefile
  :
else
  cp -R "$src/." "$scratch/"
fi

workspace="$scratch/${base%.*}.texture"
[ -d "$workspace" ] || { echo "no legacy Dolt workspace at $workspace" >&2; exit 2; }
[ ! -e "$scratch/state.db" ] || { echo "state.db already exists in the copy; nothing to migrate" >&2; exit 2; }

bin="${CHOIR_MIGRATE_GUEST_BIN:-$(command -v choir-migrate-guest || true)}"
if [ -n "$bin" ]; then
  cmd=("$bin")
else
  cmd=(go run "$repo/cmd/choir-migrate-guest")
fi
rc=0
"${cmd[@]}" -store-path "$scratch/$base" || rc=$?
echo
echo "scratch copy kept at $scratch (rm -rf it when done); report: $scratch/state.db.migration.json" >&2
exit "$rc"
