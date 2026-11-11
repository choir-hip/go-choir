#!/bin/sh
# Lane 7 proof (settings, themes, accounts, Activity), in demo mode on a simulator, driven through Exact's agent socket.
# Unit tests first (swift test --filter for the lane-7 test classes), then the build, then six drives:
#   A the Settings root with each bundled theme in light and dark
#   B Appearance: the preview cards, a theme from a Texture (demo fixture), switching to it
#   C Account and devices, Talk (choosing a mode), Voice and models, Computer (Restart to update, Wake)
#   D What the paper has learned (with Reset), Swipes (a gesture meaning), About, Activity
#   E the restyled Components and Trace pages
#   F a dark pass and two more themes over the new pages
# Screenshots go to PROOF_SHOTS. Last line: PROOF OK, or PROOF FAILED at <step>. Elements are found by test id.
#   NOBUILD=1 skips the build; NOTEST=1 skips swift test.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-lane7.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
: > "$OUT"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-lane7-last.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }

if [ -z "${NOTEST:-}" ]; then
  echo "swift test (lane 7)"
  "$HERE/xe.sh" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" --filter 'ThemeLoaderTests|AccountsTests|SettingsCatalogTests|ActivityFeedTests|SpineTests' > "$SCRATCH/test.txt" 2>&1 \
    || { tail -40 "$SCRATCH/test.txt"; fail "swift test"; }
  tail -3 "$SCRATCH/test.txt"
fi

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"

if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { tail -30 "$SCRATCH/build.txt"; fail "build"; }
fi

DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1
# Fresh state: demo mode, light, Depth, the defaults of the lane-7 settings, no open pages, no installed texture themes.
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode light
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
for k in talkMode swipeLeft swipeRight; do DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults delete $APP "AutoradioSetting.$k" >/dev/null 2>&1; done
CONT=$(DEVELOPER_DIR=$DD /usr/bin/xcrun simctl get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
if [ -n "$CONT" ]; then
  rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json" "$CONT/Library/Application Support/Autoradio/signals.jsonl"
  rm -rf "$CONT/Library/Application Support/Autoradio/themes"
fi
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-l7-shots-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"

drive() { # <label> <args...>; the first launch of a fresh install can miss the agent socket, so it is retried
  label="$1"; shift
  attempt=1
  while :; do
    : > "$SCRATCH/this.txt"
    "$HERE/exact.sh" agent ios "$@" > "$SCRATCH/this.txt" 2>&1
    rc=$?
    if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$SCRATCH/this.txt" && [ $attempt -lt 3 ]; then
      attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; sleep 5; continue
    fi
    break
  done
  cat "$SCRATCH/this.txt" >> "$OUT"
  [ $rc -eq 0 ] || { grep -v "^warning" "$SCRATCH/this.txt" | tail -15; fail "drive $label (exit $rc)"; }
  echo "drive $label ok"
}

drive "A themes on the root" \
    "clock data" "clock +1500 real" "clock data" tree "screenshot $SHOTS/01-paper.png" \
    "tap bar-pages" "clock +700 real" "clock data" "tap pile-places" "clock +700 real" "clock data" "tap place-settings" "clock +700 real" "clock data" tree "screenshot $SHOTS/02-settings-depth-light.png" \
    "tap settings-theme-lantern" "clock +700 real" "clock data" tree "screenshot $SHOTS/03-lantern-light.png" \
    "tap settings-theme-signal" "clock +700 real" "clock data" tree "screenshot $SHOTS/04-signal-light.png" \
    "tap settings-theme-newsprint" "clock +700 real" "clock data" tree "screenshot $SHOTS/05-newsprint-light.png" \
    "tap settings-theme-depth" "clock +700 real" "clock data" "tap settings-mode-dark" "clock +700 real" "clock data" tree "screenshot $SHOTS/06-depth-dark.png" \
    "tap settings-theme-lantern" "clock +700 real" "clock data" tree "screenshot $SHOTS/07-lantern-dark.png" \
    "tap settings-theme-signal" "clock +700 real" "clock data" tree "screenshot $SHOTS/08-signal-dark.png" \
    "tap settings-theme-newsprint" "clock +700 real" "clock data" tree "screenshot $SHOTS/09-newsprint-dark.png" \
    "tap settings-theme-depth" "clock +700 real" "clock data" "tap settings-mode-light" "clock +700 real" "clock data" tree "screenshot $SHOTS/09b-depth-light-again.png"

drive "B appearance" \
    "tap settings-nav-appearance" "clock +700 real" "clock data" tree "screenshot $SHOTS/10-appearance.png" \
    "tap settings-theme-demo-texture" "clock +700 real" "clock data" tree "screenshot $SHOTS/11-texture-added.png" \
    "tap settings-theme-ember" "clock +700 real" "clock data" tree "screenshot $SHOTS/12-ember-from-a-texture.png" \
    "tap settings-theme-depth" "clock +700 real" "clock data" "screenshot $SHOTS/13-appearance-depth.png"

drive "C account talk voice computer" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-account" "clock +700 real" "clock data" tree "screenshot $SHOTS/14-account.png" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-talk" "clock +700 real" "clock data" tree "screenshot $SHOTS/15-talk.png" \
    "tap settings-choice-talkMode-open" "clock +700 real" "clock data" tree "screenshot $SHOTS/16-talk-open.png" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-voice" "clock +700 real" "clock data" tree "screenshot $SHOTS/17-voice.png" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-computer" "clock +700 real" "clock data" tree "screenshot $SHOTS/18-computer.png" \
    "tap settings-update" "clock +300 real" "clock data" tree "screenshot $SHOTS/19a-computer-restarting.png" \
    "clock +4500 real" "clock data" tree "screenshot $SHOTS/19-computer-updated.png" \
    "tap settings-wake" "clock +700 real" "clock data" tree "screenshot $SHOTS/20-computer-wake.png"

drive "D learned gestures about activity" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-learned" "clock +700 real" "clock data" tree "screenshot $SHOTS/21-learned.png" \
    "tap settings-learned-reset" "clock +700 real" "clock data" tree "screenshot $SHOTS/22-learned-reset.png" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-gestures" "clock +700 real" "clock data" tree "screenshot $SHOTS/23-gestures.png" \
    "tap settings-choice-swipeLeft-closeLess" "clock +700 real" "clock data" tree "screenshot $SHOTS/24-gestures-chosen.png" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-about" "clock +700 real" "clock data" tree "screenshot $SHOTS/25-about.png" \
    "tap settings-back" "clock +1000 real" "clock data" "tap settings-nav-activity" "clock +1000 real" "clock data" tree "screenshot $SHOTS/26-activity.png"

drive "E components trace" \
    "tap bar-pages" "clock +700 real" "clock data" "tap pile-places" "clock +700 real" "clock data" "tap place-settings" "clock +700 real" "clock data" "tap settings-components" "clock +700 real" "clock data" tree "screenshot $SHOTS/27-components.png" \
    "tap open-trace" "clock +700 real" "clock data" tree "screenshot $SHOTS/28-trace.png"

drive "F dark pass" \
    "tap bar-pages" "clock +700 real" "clock data" "tap pile-places" "clock +700 real" "clock data" "tap place-settings" "clock +700 real" "clock data" "tap settings-mode-dark" "clock +700 real" "clock data" "tap settings-nav-appearance" "clock +700 real" "clock data" tree "screenshot $SHOTS/30-appearance-dark.png" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-account" "clock +700 real" "clock data" "screenshot $SHOTS/31-account-dark.png" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-computer" "clock +700 real" "clock data" "screenshot $SHOTS/32-computer-dark.png" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-learned" "clock +700 real" "clock data" "screenshot $SHOTS/33-learned-dark.png" \
    "tap settings-back" "clock +700 real" "clock data" "tap settings-nav-gestures" "clock +700 real" "clock data" "screenshot $SHOTS/34-gestures-dark.png" \
    "tap settings-back" "clock +900 real" "clock data" "tap settings-nav-activity" "clock +900 real" "clock data" tree "screenshot $SHOTS/35-activity-dark.png" \
    "tap bar-pages" "clock +700 real" "clock data" "tap pile-places" "clock +700 real" "clock data" "tap place-settings" "clock +700 real" "clock data" "tap settings-theme-lantern" "clock +700 real" "clock data" tree "screenshot $SHOTS/36-lantern-dark-root.png" \
    "tap settings-nav-activity" "clock +900 real" "clock data" tree "screenshot $SHOTS/37-activity-lantern-dark.png" \
    "tap bar-pages" "clock +700 real" "clock data" "tap pile-places" "clock +700 real" "clock data" "tap place-settings" "clock +700 real" "clock data" "tap settings-theme-signal" "clock +700 real" "clock data" "tap settings-nav-appearance" "clock +700 real" "clock data" tree "screenshot $SHOTS/38-signal-dark-appearance.png" \
    "tap settings-back" "clock +900 real" "clock data" "tap settings-mode-light" "clock +900 real" "clock data" "tap settings-nav-activity" "clock +900 real" "clock data" tree "screenshot $SHOTS/39-activity-signal-light.png"

must() { # <step name> <fixed text>
  grep -qF -- "$2" "$OUT" || fail "$1" "missing: $2"
  echo "ok: $1"
}

must "root: the settings page" '[page-title] "Settings"'
must "root: depth in light" 'Showing Depth (light)'
must "themes: four bundled preview cards" '[settings-theme-signal]'
must "themes: lantern" 'Showing Lantern (light)'
must "themes: signal" 'Showing Signal (light)'
must "themes: newsprint" 'Showing Newsprint (light)'
must "themes: depth dark" 'Showing Depth (dark)'
must "themes: lantern dark" 'Showing Lantern (dark)'
must "themes: signal dark" 'Showing Signal (dark)'
must "themes: newsprint dark" 'Showing Newsprint (dark)'
must "appearance: the page" '[page-title] "Appearance"'
must "appearance: a theme field" '[settings-theme-doc]'
must "appearance: demo texture offered" '[settings-theme-demo-texture]'
must "texture theme: added from the demo Texture" 'Added Ember from the Texture'
must "texture theme: listed" '[settings-theme-ember]'
must "texture theme: dark-only theme shown dark in a light phone" 'Showing Ember in dark'
must "account: the page" '[page-title] "Account and devices"'
must "account: who am I" 'quiet-heron-41'
must "account: the device key label" 'Autoradio on this phone (demo)'
must "account: sign out is offered" '[settings-signout]'
must "talk: the page" '[page-title] "Talk"'
must "talk: choices" '[settings-choice-talkMode-word]'
must "talk: a chosen mode shows on the list" 'Always listening'
must "voice: the page" '[page-title] "Voice and models"'
must "computer: the page" '[page-title] "Computer"'
must "computer: an update waits" 'Waiting: demo-build-2'
must "computer: restart to update offered" '[settings-update]'
must "computer: wake offered" '[settings-wake]'
must "computer: restart to update ends current" 'Updated. Your computer restarted.'
must "learned: the page" '[page-title] "What the paper has learned"'
must "learned: a line from the demo list" 'Harbor barrier vote'
must "learned: reset empties it" 'Nothing yet.'
must "gestures: the page" '[page-title] "Swipes"'
must "gestures: choices" '[settings-choice-swipeLeft-closeLess]'
must "about: the page" '[page-title] "About"'
must "about: demo mode" 'Demo fixtures'
must "activity: the page" '[page-title] "Activity"'
must "activity: a revision from the changelog" 'Revision saved'
must "activity: the computer's update" 'Your computer finished updating and restarted.'
must "activity: unseen count" '2 new since you last looked'
must "components: restyled page still works" '[page-title] "Components"'
must "components: talk control kept" '[lab-talk]'
must "trace: restyled page still works" '[page-title] "Trace"'
[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 30 ] || fail "screenshots" "fewer than 30 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m5 'ERROR\|Fatal error' "$OUT"; fi
echo "PROOF OK"
