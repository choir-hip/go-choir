#!/bin/sh
# Print the udid of a simulator that is yours alone (created on first use, booted if shut down), named
# "${AR_SIM_NAME:-autoradio-lab}". Exact allows one app per bundle id per simulator and a launch replaces a
# running copy, so two drivers on one simulator kill each other's app. Use:  export EXACT_SIM=$(mobile/scripts/sim-device.sh)
set -eu
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
NAME="${AR_SIM_NAME:-autoradio-lab}"
SIMCTL="/usr/bin/xcrun simctl"
udid=$($SIMCTL list devices 2>/dev/null | grep -F "    $NAME (" | head -1 | sed -e 's/.*(\([0-9A-F-]\{36\}\)).*/\1/' || true)
if [ -z "$udid" ]; then
  runtime=$($SIMCTL list runtimes 2>/dev/null | grep -o 'com.apple.CoreSimulator.SimRuntime.iOS-[0-9-]*' | tail -1)
  udid=$($SIMCTL create "$NAME" com.apple.CoreSimulator.SimDeviceType.iPhone-18-Pro "$runtime" 2>/dev/null)
fi
if ! $SIMCTL list devices 2>/dev/null | grep -F "$udid" | grep -q Booted; then $SIMCTL boot "$udid" >/dev/null 2>&1 || true; fi
echo "$udid"
