#!/bin/sh
# Launch the lab on the booted simulator with the console attached (optionally as the Exact agent would:
# AGENT=1). Prints the app's stdout/stderr for $SECS seconds (default 15) then terminates it.
HERE="$(cd "$(dirname "$0")" && pwd)"
BUNDLE="$("$HERE/app-identity.sh" id)" || exit 1   # AR_VARIANT / AR_APP_ID choose the second app
SECS="${SECS:-15}"
if [ -n "${AGENT:-}" ]; then
  export SIMCTL_CHILD_EXACT_AGENT=1 SIMCTL_CHILD_EXACT_AGENT_SOCKET="${TMPDIR:-/tmp}/lab-agent-$(printf %s "$BUNDLE" | cksum | cut -d" " -f1).sock"      # one socket per app: two apps on one simulator did not collide
fi
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer /usr/bin/xcrun simctl launch --console --terminate-running-process booted "$BUNDLE" &
PID=$!
sleep "$SECS"
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer /usr/bin/xcrun simctl terminate booted "$BUNDLE" >/dev/null 2>&1
kill $PID 2>/dev/null
