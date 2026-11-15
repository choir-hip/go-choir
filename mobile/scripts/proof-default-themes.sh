#!/bin/sh
# Proof of the default themes: a fresh app with NO theme chosen shows Chrome Pastel in a light scheme and Liquid Metal in a dark
# one. Same machinery as proof-app.sh (generators, the whole package's tests, one build, a simulator driven through Exact's agent
# socket in demo mode), but the app is never told a theme: proof-common.sh's reset_state writes Depth, so it is redefined here to
# delete the choice. Screenshots: default-light-NN-*.png and default-dark-NN-*.png in $PROOF_SHOTS.
# Last line: PROOF OK, or PROOF FAILED at <step>.      NOBUILD=1   skip the build        NOTESTS=1   skip swift test
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-proof-default-themes-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
. "$HERE/proof-common.sh"
echo "screenshots: $SHOTS"

# A clean app with no theme chosen: demo mode, the scheme given, no AutoradioThemeID.
reset_state() { # [light|dark] [unused]
  SIM terminate "$EXACT_SIM" $APP >/dev/null 2>&1
  sleep 2
  SIM spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
  SIM spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode "${1:-light}"
  SIM spawn "$EXACT_SIM" defaults delete $APP AutoradioThemeID >/dev/null 2>&1
  SIM spawn "$EXACT_SIM" defaults delete $APP AutoradioOpenOnboarding >/dev/null 2>&1
  for k in talkMode swipeLeft swipeRight; do SIM spawn "$EXACT_SIM" defaults delete $APP "AutoradioSetting.$k" >/dev/null 2>&1; done
  CONT=$(SIM get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
  if [ -n "$CONT" ]; then
    S="$CONT/Library/Application Support/Autoradio"
    rm -f "$S/open-pages.json" "$S/onboarding.json" "$S/interview.json" "$S/voice-setup.json" "$S/interview.md" "$S/signals.jsonl" "$S/recent-files.json"
    rm -rf "$S/themes" "$CONT/Library/Caches/Autoradio/files"
    rm -f "$CONT/Library/Preferences/$APP.plist"
  fi
}

python3 -I "$HERE/embed-assets.py" > "$SCRATCH/embed.txt" 2>&1 || { tail -20 "$SCRATCH/embed.txt"; fail "embed assets"; }
python3 -I "$HERE/gen-exact-interface.py" > "$SCRATCH/iface.txt" 2>&1 || { tail -20 "$SCRATCH/iface.txt"; fail "exact interface"; }
python3 -I "$HERE/check-strings.py" > "$SCRATCH/strings.txt" 2>&1 || { tail -20 "$SCRATCH/strings.txt"; fail "strings"; }
echo "ok: assets, interface, strings"

if [ -z "${NOTESTS:-}" ]; then
  echo "swift test (the whole package)"
  "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" > "$SCRATCH/test.txt" 2>&1 \
    || { grep -E "error:|failed|XCTAssert" "$SCRATCH/test.txt" | cut -c1-400 | head -40; tail -6 "$SCRATCH/test.txt"; cp "$SCRATCH/test.txt" "$SHOTS/swift-tests.txt"; fail "swift test"; }
  grep -E "Executed [0-9]+ tests" "$SCRATCH/test.txt" | tail -1
  cp "$SCRATCH/test.txt" "$SHOTS/swift-tests.txt"
fi

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"
if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { grep -v '^warning' "$SCRATCH/build.txt" | head -60; tail -15 "$SCRATCH/build.txt"; cp "$SCRATCH/build.txt" "$SHOTS/build.txt"; fail "build"; }
fi
SIM privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1

tour() { # <light|dark>
  m="$1"
  step "clock data"; wait_ 1500
  view default-$m-01-paper-front 800
  step "tap paper-scroll drag 0 -260"; wait_ 700
  step "tap paper-story-demo-grid-storage"; view default-$m-02-story-reader 1000
  step "tap bar-voice down"; step "tap up"; wait_ 1800
  step "tap bar-text"; view default-$m-03-player 900
  step "tap player-close"; wait_ 700
  step "tap bar-pages"; view default-$m-04-pile 700
  step "tap pile-places"; view default-$m-05-launcher 700
  step "tap place-mail"; view default-$m-06-mail-inbox 900
  step "tap mail-row-m6"; view default-$m-07-mail-message 900
  place settings 900; step tree; shot default-$m-08-settings
  if [ "$m" = dark ]; then
    # Choose the light-only theme, ask for dark (it refuses and shows light), then go back to the Default pair.
    step "tap settings-theme-chrome-pastel"; view default-$m-09-chose-chrome-pastel 700
    step "tap settings-mode-dark"; view default-$m-10-dark-asked-of-a-light-theme 700
    step "tap settings-theme-default"; view default-$m-11-back-to-default 700
  fi
  step logs
}
tour light; run_drive deflight light "" --touch platform
tour dark;  run_drive defdark dark "" --touch platform

must "light: the default is Chrome Pastel" 'Showing Chrome Pastel (light)'
must "dark: the default is Liquid Metal" 'Showing Liquid Metal (dark)'
must "settings lists both" '[settings-theme-chrome-pastel]'
must "settings lists both (dark)" '[settings-theme-liquid-metal]'
must "settings has the Default entry" '[settings-theme-default]'
must "paper: masthead" '[page-title] "AUTOPAPER"'
n=$(ls "$SHOTS"/default-light-*.png 2>/dev/null | wc -l); m=$(ls "$SHOTS"/default-dark-*.png 2>/dev/null | wc -l)
[ "$n" -ge 8 ] && [ "$m" -ge 8 ] || FAILS="$FAILS [screenshots: $n light, $m dark]"
echo "ok: screenshots in $SHOTS ($n light, $m dark)"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m8 'ERROR\|Fatal error' "$OUT"; fi
[ -z "$FAILS" ] || fail "checks:$FAILS"
echo "PROOF OK"
