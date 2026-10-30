#!/usr/bin/env bash
# Serve the retired Dolt data directories by hand, so the host migrator can read
# them once more after the new release removed the go-choir-platform-dolt and
# go-choir-corpus-dolt units (data system cutover). Node B only, as root.
#
# This is the "run dolt sql-server by hand" path chosen over a NixOS option:
# the migration normally runs at C5 BEFORE the new release deploys, against
# the old release's own servers, and needs none of this. Use it only to rerun
# the copy after the deploy (the data directories stay until C5 deletes them).
#
# Usage:  legacy-dolt-serve.sh start | stop | status
#
# It runs two transient systemd units (choir-legacy-dolt-platform on 13306,
# choir-legacy-dolt-corpus on 13307) on the same data directories, memory
# limits and ports as the old units. The Dolt users and passwords live in the
# data directories (.doltcfg) and /var/lib/go-choir/secrets/dolt-*-password,
# which are untouched, so choir-migrate-host connects as before. Do not start
# these while the old release's Dolt units are running (same ports).
set -euo pipefail

platform_dir=/var/lib/go-choir/platform-dolt
corpus_dir=/var/lib/go-choir/corpus-dolt
dolt="${DOLT_BIN:-$(command -v dolt)}"

case "${1:-}" in
  start)
    [ -d "$platform_dir/platform/.dolt" ] || { echo "no platform Dolt repo at $platform_dir/platform" >&2; exit 1; }
    systemd-run --unit=choir-legacy-dolt-platform -p MemoryHigh=4G -p MemorySwapMax=0 \
      -p WorkingDirectory="$platform_dir/platform" -E HOME="$platform_dir" -E GOMEMLIMIT=3GiB \
      "$dolt" sql-server --host 127.0.0.1 --port 13306
    if [ -d "$corpus_dir/corpus/.dolt" ]; then
      systemd-run --unit=choir-legacy-dolt-corpus -p MemoryHigh=5G -p MemoryMax=7G -p MemorySwapMax=0 \
        -p WorkingDirectory="$corpus_dir" -E HOME="$corpus_dir" \
        "$dolt" sql-server --host 127.0.0.1 --port 13307
    fi
    ;;
  stop)
    systemctl stop choir-legacy-dolt-platform.service choir-legacy-dolt-corpus.service 2>/dev/null || true
    ;;
  status)
    systemctl --no-pager status choir-legacy-dolt-platform.service choir-legacy-dolt-corpus.service || true
    ;;
  *)
    sed -n 2,18p "$0"
    exit 2
    ;;
esac
