#!/bin/sh
# End-to-end proof of the lab app on a simulator, driven through Exact's agent socket (no human, no device):
# a fresh launch with no key shows Sign in on the Radio tab; the Lab screen selects the stub stages explicitly (the app's default is the real listener configuration, which needs a key and the network); Lab and Trace screens; Start audio (virtual engine: the agent opens no hardware); hold-to-talk;
# a kept turn injected through the live engine (yielding observed while it speaks); the turn record in Trace with
# stage timings and the audio switch; the offline replay of the kept turn; the JSON export.
# Last line: PROOF OK, or PROOF FAILED at <step>. Elements are found by test id, never by position.
#   NOBUILD=1      skip the build (use the installed app)
#   EXACT_SIM=...  a simulator name or udid; default: a simulator of our own (scripts/sim-device.sh)
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-sim.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-sim-last.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"

if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { tail -20 "$SCRATCH/build.txt"; fail "build"; }
fi

# One drive, many operations. `tap start` then the first frame: the app opens its agent socket only after its first
# frame, in the foreground (Exact: AgentFrontIOS.swift:29, ExactIOS/main.swift:134), and the driver waits 20 s for it
# (agent.mjs:567): a cold launch on a loaded machine misses that, so a refused socket is retried.
# A permission alert (the speech recognizer asks at its first use) takes the foreground from the app: the driver then
# reads "scene not foreground-active" and a later launch never opens its socket. Grant everything first; that also
# clears an alert left by an earlier run.
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer /usr/bin/xcrun simctl privacy "$EXACT_SIM" grant all news.choir.autoradio.lab >/dev/null 2>&1

attempt=1
while :; do
  "$HERE/exact.sh" agent ios \
    "clock data" tree \
    "tap tab-lab" "clock data" tree \
    "tap stage-addressing-hybrid" "tap stage-transcriber-scripted" "tap stage-system1-fixed" "tap stage-holder-fixed-lines" \
      "tap stage-desk-stub-echo" "tap stage-programme-fixture" "tap ack-immediate" "clock data" tree \
    "tap start" "clock +1500 real" "clock data" tree \
    "tap lab-talk down" "clock +700 real" "clock data" tree "tap up" "clock +1500 real" \
    "tap inject" "clock +3000 real" "clock data" tree \
    "clock +9000 real" "clock data" "tap open-trace" "clock data" tree "tap tab-lab" \
    "tap stage-addressing-open" "tap inject" "clock +2500 real" "clock data" tree \
    "clock +9000 real" "clock data" "tap open-trace" "clock data" tree "tap tab-lab" \
    "tap replay-fixture" "clock +9000 real" "clock data" "tap open-trace" "clock data" tree \
    "tap trace-export" "clock +800 real" "clock data" tree \
    logs > "$OUT" 2>&1
  rc=$?
  if grep -q "never opened its agent socket\|before opening its agent socket\|the app hung up" "$OUT" && [ $attempt -lt 3 ]; then
    attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; sleep 5; continue
  fi
  break
done
[ $rc -eq 0 ] || { grep -v "^warning" "$OUT" | tail -15; fail "drive (exit $rc)"; }

# Assertions on the one transcript. The tree lists every tab's panel in a fixed order, so the order of lines inside
# one tree says nothing: each text must simply appear, and each is a text that exists only after its step.
must() { # <step name> <fixed text>
  grep -qF -- "$2" "$OUT" || fail "$1" "missing: $2"
  echo "ok: $1"
}

# A fresh launch with no key: the Radio tab shows Sign in, in plain words, and no player.
must "fresh launch: sign-in button on the Radio tab" '[signin]'
must "fresh launch: plain-words state" '[signin-intro] "Sign in to hear the paper and talk to your computer."'
if grep -q '\[story-title\]' "$OUT"; then fail "fresh launch" "the player shows without a key"; fi
echo "ok: fresh launch: no player without a key"
must "lab screen: talk control" '[lab-talk]'
must "lab screen: acknowledgement timing choice" '[ack-immediate]'
must "lab screen: stub configuration selected explicitly" '"● System 1'
must "lab screen: audio state before start" 'State none'
must "lab screen: policy choices" '[policy-alwaysTalking]'
must "lab screen: inject control" '[inject]'
must "trace screen: export control" '[trace-export]'
must "start audio: listening claim" 'State listening'
must "start audio: virtual engine" 'engine virtual'
must "start audio: microphone on" 'Phase: playing · mic on'
must "hold to talk: phase leaves playing" 'Phase: addressed'
must "yielding (button turn): programme paused" '[yield] "Paused"'
must "yielding (open speech): programme dipped" '[yield] "Dipped: you are speaking"'
must "yielding (open speech): node volume lowered" 'volume 0.25'
must "audio switch events recorded" 'audio switch listening -> talking in'
grep -qE '\[trace-id\] "turn-' "$OUT" || fail "trace: turn record" "no live turn record in Trace"
echo "ok: trace: live turn record"
grep -qE 'Audio switch [0-9]+ ms · [1-9][0-9]* stage timings' "$OUT" || fail "trace: stage timings" "no stage timings in the record"
echo "ok: trace: stage timings"
grep -qE 'Acknowledgement [0-9]+ ms' "$OUT" || fail "trace: acknowledgement time" "no acknowledgement time in the record"
echo "ok: trace: acknowledgement time"
grep -qE 'Audio switch [1-9][0-9]* ms' "$OUT" || fail "trace: switch latency" "no audio switch latency in the turn record"
echo "ok: trace: switch latency in the record"
must "replay: record in trace" 'replay-fixture-plain'
must "export: file written" 'Saved '
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m5 'ERROR\|Fatal error' "$OUT"; fi
echo "PROOF OK"
