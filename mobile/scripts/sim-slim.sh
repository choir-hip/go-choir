#!/bin/sh
# Slim a named simulator with simslim (https://github.com/MobAI-App/simslim, MIT): switch off the background daemons
# Autoradio does not use, keeping Siri and speech (see "Slim simulators" in mobile/README.md).
#   sim-slim.sh <name|udid>            slim it (reboots it once, offline: about 25 s)
#   sim-slim.sh <name|udid> --off      put it back to stock
#   sim-slim.sh <name|udid> --status   show how many managed services are off
# Refuses autoradio-lab (the owner's simulator) unless --allow-lab is given. SIMSLIM names the binary
# (default ~/.local/bin/simslim). Every simctl call goes through xe.sh, like the rest of this directory.
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
SIMSLIM="${SIMSLIM:-$HOME/.local/bin/simslim}"
[ -x "$SIMSLIM" ] || { echo "no simslim at $SIMSLIM (see mobile/README.md, Slim simulators)" >&2; exit 2; }
target="" mode=on lab=""
for a in "$@"; do
  case "$a" in
    --off) mode=off ;; --status) mode=status ;; --allow-lab) lab=1 ;;
    -*) echo "unknown flag $a" >&2; exit 2 ;;
    *) [ -z "$target" ] || { echo "one simulator only" >&2; exit 2; }; target="$a" ;;
  esac
done
[ -n "$target" ] || { echo "usage: sim-slim.sh <name|udid> [--off|--status] [--allow-lab]" >&2; exit 2; }
XC="$HERE/xe.sh /usr/bin/xcrun"
list=$($XC simctl list devices 2>/dev/null)
case "$target" in
  *-*-*-*-*) udid="$target" ;;
  *) udid=$(printf '%s\n' "$list" | grep -F "    $target (" | head -1 | sed -e 's/.*(\([0-9A-F-]\{36\}\)).*/\1/' || true) ;;
esac
[ -n "${udid:-}" ] || { echo "no simulator named $target" >&2; exit 2; }
name=$(printf '%s\n' "$list" | grep -F "($udid)" | head -1 | sed -e 's/^ *//' -e 's/ ([0-9A-F-]\{36\}).*//')
[ -n "$name" ] || { echo "no simulator $udid" >&2; exit 2; }
if [ "$name" = autoradio-lab ] && [ -z "$lab" ] && [ "$mode" = on ]; then
  echo "refusing: autoradio-lab is the owner's simulator; pass --allow-lab to slim it" >&2; exit 3
fi
case "$mode" in
  on)
    # Kept: siri (speech recognition, text to speech, on-device language models) and the asset daemons
    # (mobileassetd, axassetsd) that fetch voices and speech models. Everything else simslim manages is switched off.
    exec "$HERE/xe.sh" "$SIMSLIM" on "$udid" --except siri --keep com.apple.mobileassetd,com.apple.accessibility.axassetsd ;;
  off) exec "$HERE/xe.sh" "$SIMSLIM" off "$udid" ;;
  status) exec "$HERE/xe.sh" "$SIMSLIM" status "$udid" ;;
esac
