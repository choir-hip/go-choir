#!/bin/sh
# The laps of a first open, read from a device or simulator log (lap.ts `lapOpen`; docs/reference/exact/testing-and-debugging.md, "Reading the laps of a first open").
#
#   mobile/scripts/laps.sh --device <udid> [--last 10m]     collect the phone's unified log (needs sudo: `log collect` asks for it), then print the laps
#   mobile/scripts/laps.sh --sim <name|udid> [--last 10m]   the simulator's log, no sudo
#   mobile/scripts/laps.sh --file <log>                     a `log show --style compact` dump, or a `log stream` capture
#
# Shared code cannot read a clock, so a lap line carries none: the platform stamps it. Every line the script prints is one lap with its time of day, the milliseconds since the line before,
# and (for a line that carries a page address) the milliseconds since that page's `pagesOpen entry` or `openRadioOf entry`. The bundle id is the app's (app-identity.sh, AR_VARIANT).
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
LAST=10m; MODE=""; ARG=""
while [ $# -gt 0 ]; do
  case "$1" in
    --device) MODE=device; ARG="$2"; shift 2 ;;
    --sim) MODE=sim; ARG="$2"; shift 2 ;;
    --file) MODE=file; ARG="$2"; shift 2 ;;
    --last) LAST="$2"; shift 2 ;;
    *) echo "usage: laps.sh (--device <udid> | --sim <name> | --file <log>) [--last 10m]" >&2; exit 2 ;;
  esac
done
[ -n "$MODE" ] || { echo "usage: laps.sh (--device <udid> | --sim <name> | --file <log>) [--last 10m]" >&2; exit 2; }
APP="$("$HERE/app-identity.sh" id)"
PRED="subsystem == \"$APP\" AND category == \"console\" AND eventMessage BEGINSWITH \"lap: \""
RAW="$(mktemp "${TMPDIR:-/tmp}/laps.XXXXXX")"
trap 'rm -rf "$RAW" "$RAW.logarchive"' EXIT
case "$MODE" in
  device) sudo /usr/bin/log collect --device-udid "$ARG" --last "$LAST" --output "$RAW.logarchive" >&2
          /usr/bin/log show --archive "$RAW.logarchive" --style compact --info --debug --predicate "$PRED" > "$RAW" ;;
  sim)    /usr/bin/xcrun simctl spawn "$ARG" log show --last "$LAST" --style compact --info --debug --predicate "$PRED" > "$RAW" ;;
  file)   cp "$ARG" "$RAW" ;;
esac
awk '
/lap: (open|net) / {
  split($2, t, ":"); ms = (t[1]*3600 + t[2]*60 + t[3]) * 1000
  label = substr($0, index($0, "lap: ") + 5)
  n = split(label, w, " "); addr = w[n]
  if (label ~ /^open (pagesOpen|openRadioOf) entry /) base[addr] = ms
  d = (prev == "") ? 0 : ms - pms
  since = (label ~ /^open / && (addr in base)) ? sprintf("%7d ms since open", ms - base[addr]) : sprintf("%25s", "")
  printf "%s  +%5d ms  %s  %s\n", $2, d, since, label
  prev = label; pms = ms
}' "$RAW"
