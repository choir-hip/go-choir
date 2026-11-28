#!/bin/sh
# Print the udid of a simulator that is yours alone (created on first use, booted if shut down), named
# "${AR_SIM_NAME:-autoradio-lab}". Exact allows one app per bundle id per simulator and a launch replaces a
# running copy, so two drivers on one simulator kill each other's app. Use:  export EXACT_SIM=$(mobile/scripts/sim-device.sh)
set -eu
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
NAME="${AR_SIM_NAME:-autoradio-lab}"
SIMCTL="${AR_SIMCTL:-/usr/bin/xcrun simctl}"   # AR_SIMCTL: a stand-in for the tests (test-proof-helpers.sh), never a way to a stock simulator
TYPE=com.apple.CoreSimulator.SimDeviceType.iPhone-18-Pro
udid=$($SIMCTL list devices 2>/dev/null | grep -F "    $NAME (" | head -1 | sed -e 's/.*(\([0-9A-F-]\{36\}\)).*/\1/' || true)
if [ -z "$udid" ]; then
  # The newest runtime that can host the device type, read from simctl's JSON (the last line of the text list was iOS 27.1, which lists one device type and not
  # the iPhone 18 Pro: `create` failed, its error was hidden and the script died with no message). Versions compare as numbers. None fits: say so and stop.
  runtime=$($SIMCTL list -j runtimes | /usr/bin/python3 -I -c '
import json, sys
dev = sys.argv[1]
rs = [r for r in json.load(sys.stdin)["runtimes"]
      if r.get("isAvailable") and r.get("platform", "iOS") == "iOS"
      and any(d["identifier"] == dev for d in r.get("supportedDeviceTypes", []))]
rs.sort(key=lambda r: tuple(int(p) for p in r["version"].split(".")))
print(rs[-1]["identifier"] if rs else "")
' "$TYPE")
  [ -n "$runtime" ] || { echo "sim-device: no available iOS runtime supports $TYPE" >&2; exit 1; }
  udid=$($SIMCTL create "$NAME" "$TYPE" "$runtime") || { echo "sim-device: simctl could not create $NAME ($TYPE on $runtime)" >&2; exit 1; }
  # A new simulator is slimmed before its first use (sim-slim.sh refuses the default name, autoradio-lab).
  # No simslim binary, or a failed slim, leaves a stock simulator and says so.
  if [ "$NAME" != autoradio-lab ]; then
    "$(dirname "$0")/sim-slim.sh" "$udid" >&2 || echo "sim-device: $NAME is not slimmed (see mobile/README.md, Slim simulators)" >&2
  fi
fi
if ! $SIMCTL list devices 2>/dev/null | grep -F "$udid" | grep -q Booted; then $SIMCTL boot "$udid" >/dev/null 2>&1 || true; fi
echo "$udid"
