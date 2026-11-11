#!/bin/sh
# Lane 6 (Mail) proof on a simulator, in demo mode (bundled fixtures, no network): the Kit tests first (MailTests: parsing,
# threading, sanitising, the draft refusals, offline, the approval path), then the app: inbox with threads, a message as a
# page (attachment, signature), the attachment opening as a page, the prompt aimed at a message, a reply as a draft saved and
# the approval link asked for, a thread with folded quotes, an HTML-only message and its sandboxed original, the quarantine
# folder, drafts and a draft as a page, the four whole-screen states (empty, loading, offline, error), a new message refused
# when empty, and the same screens in dark. Screenshots go to PROOF_SHOTS. Last line: PROOF OK, or PROOF FAILED at <step>.
#   NOBUILD=1 skip the app build.   NOTESTS=1 skip the Kit tests.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
SCRATCH="${TMPDIR:-/tmp}/proof-lane6.$$"
mkdir -p "$SCRATCH"
OUT="$SCRATCH/drive.txt"
trap 'rm -rf "$SCRATCH"' EXIT
fail() { cp "$OUT" "${TMPDIR:-/tmp}/proof-lane6-last.txt" 2>/dev/null; echo "PROOF FAILED at $1"; [ -n "${2:-}" ] && echo "$2"; exit 1; }

python3 -I "$ROOT/scripts/check-strings.py" || fail "strings"
python3 -I "$ROOT/scripts/embed-assets.py" >/dev/null || fail "embed assets"

if [ -z "${NOTESTS:-}" ]; then
  echo "kit tests"
  "$HERE/xe.sh" /usr/bin/swift test --package-path "$ROOT/AutoradioKit" --filter MailTests > "$SCRATCH/test.txt" 2>&1 \
    || { grep -E "error:|failed|XCTAssert" "$SCRATCH/test.txt" | head -30; fail "kit tests"; }
  grep -E "Executed .* tests" "$SCRATCH/test.txt" | tail -1
fi

export EXACT_SIM="${EXACT_SIM:-$("$HERE/sim-device.sh")}"
[ -n "$EXACT_SIM" ] || fail "simulator" "no simulator"
echo "simulator $EXACT_SIM"

if [ -z "${NOBUILD:-}" ]; then
  echo "building"
  "$HERE/build-lab.sh" ios > "$SCRATCH/build.txt" 2>&1 || { grep -E "error|Error" "$SCRATCH/build.txt" | head -30; tail -5 "$SCRATCH/build.txt"; fail "build"; }
fi

DD=/Applications/Xcode.app/Contents/Developer
APP=news.choir.autoradio.lab
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl privacy "$EXACT_SIM" grant all $APP >/dev/null 2>&1
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioDemo -bool YES
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode light
DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeID depth
CONT=$(DEVELOPER_DIR=$DD /usr/bin/xcrun simctl get_app_container "$EXACT_SIM" $APP data 2>/dev/null)
[ -n "$CONT" ] && rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json"
SHOTS="${PROOF_SHOTS:-${TMPDIR:-/tmp}/autoradio-lane6-shots-$(date +%H%M%S)}"
mkdir -p "$SHOTS"
echo "screenshots: $SHOTS"

W="clock +900 real"
attempt=1
while :; do
  # Drive 1: reading. Inbox, a message with an attachment, the prompt aimed at it, the attachment as a page, then a reply
  # saved as a draft and the approval link asked for.
  "$HERE/exact.sh" agent ios --touch platform \
    "clock data" "clock +1500 real" "clock data" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" tree "screenshot $SHOTS/01-inbox.png" \
    "tap mail-row-m6" "$W" "clock data" tree "screenshot $SHOTS/02-message.png" \
    "tap mail-summarise" "$W" "clock data" tree "screenshot $SHOTS/03-asked.png" \
    "tap mail-read" "$W" "clock data" tree "screenshot $SHOTS/03b-read-aloud.png" \
    "tap mail-attachment-a1" "$W" "clock data" tree "screenshot $SHOTS/04-attachment-page.png" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap launcher-open-mail:m6" "$W" "clock data" \
    "tap mail-reply" "$W" "clock data" tree "screenshot $SHOTS/05-reply-seed.png" \
    "type mail-field-body Yes, I will be there on Thursday at ten." "$W" "clock data" tree "screenshot $SHOTS/05b-reply-typed.png" \
    "tap mail-save" "$W" "clock data" tree "screenshot $SHOTS/06-draft-saved.png" \
    "tap mail-approval" "$W" "clock data" tree "screenshot $SHOTS/07-approval.png" \
    logs > "$OUT" 2>&1
  rc=$?
  # Drive 2: threads, quotes, HTML-only and its original, quarantine, drafts, the four states, a new message, dark.
  [ $rc -eq 0 ] && "$HERE/exact.sh" agent ios \
    "clock data" "clock +1000 real" "clock data" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" \
    "tap mail-row-m5" "$W" "clock data" tree "screenshot $SHOTS/08-thread-folded.png" \
    "tap mail-quoted-toggle" "$W" "clock data" tree "screenshot $SHOTS/09-quoted-shown.png" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" \
    "tap mail-row-m2" "$W" "clock data" tree "screenshot $SHOTS/10-html-only.png" \
    "tap mail-original-toggle" "clock +2000 real" "clock data" tree "screenshot $SHOTS/11-original.png" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" \
    "tap mail-folder-quarantine" "$W" "clock data" tree "screenshot $SHOTS/12-quarantine.png" \
    "tap mail-row-q1" "$W" "clock data" tree "screenshot $SHOTS/12b-quarantined-message.png" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" \
    "tap mail-folder-drafts" "$W" "clock data" tree "screenshot $SHOTS/13-drafts.png" \
    "tap mail-row-demo-draft-1" "$W" "clock data" tree "screenshot $SHOTS/14-draft-page.png" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-settings" "$W" "tap settings-dev-details" "$W" "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" \
    "tap mail-demo-empty" "$W" "clock data" tree "screenshot $SHOTS/15-empty.png" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" \
    "tap mail-demo-offline" "$W" "clock data" tree "screenshot $SHOTS/16-offline.png" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" \
    "tap mail-demo-error" "$W" "clock data" tree "screenshot $SHOTS/17-error.png" \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" \
    "tap mail-demo-loading" "screenshot $SHOTS/18-loading.png" \
    "clock +13000 real" "clock data" tree \
    "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" \
    "tap mail-compose" "$W" "clock data" tree "screenshot $SHOTS/19-new-message.png" \
    "tap mail-save" "$W" "clock data" tree "screenshot $SHOTS/20-new-refused.png" \
    logs >> "$OUT" 2>&1
  rc=$?
  # Drive 3: the same screens in dark, from a fresh launch (the launcher is long once many pages are open).
  if [ $rc -eq 0 ]; then
    DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1
    sleep 4
    DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults write $APP AutoradioThemeMode dark
    DEVELOPER_DIR=$DD /usr/bin/xcrun simctl spawn "$EXACT_SIM" defaults read $APP AutoradioThemeMode
    [ -n "$CONT" ] && rm -f "$CONT/Library/Application Support/Autoradio/open-pages.json"
    sleep 1
    "$HERE/exact.sh" agent ios --touch platform \
      "clock data" "clock +1500 real" "clock data" \
      "tap bar-pages" "$W" "tap pile-places" "$W" "tap place-mail" "$W" "clock data" tree "screenshot $SHOTS/21-inbox-dark.png" \
      "tap mail-row-m6" "$W" "clock data" tree "screenshot $SHOTS/22-message-dark.png" \
      "tap mail-reply" "$W" "clock data" tree "screenshot $SHOTS/23-reply-dark.png" \
      logs >> "$OUT" 2>&1
    rc=$?
  fi
  if grep -q "never opened its agent socket\|not foreground-active\|before opening its agent socket\|the app hung up" "$OUT" && [ $attempt -lt 3 ]; then
    attempt=$((attempt + 1)); echo "launch refused, retry $attempt"; DEVELOPER_DIR=$DD /usr/bin/xcrun simctl terminate "$EXACT_SIM" $APP >/dev/null 2>&1; sleep 5; continue
  fi
  break
done
[ $rc -eq 0 ] || { grep -v "^warning" "$OUT" | tail -15; fail "drive (exit $rc)"; }

must() { # <step name> <fixed text>
  grep -qF -- "$2" "$OUT" || fail "$1" "missing: $2"
  echo "ok: $1"
}

must "inbox: a row per thread" '[mail-row-m6]'
must "inbox: the thread is one row with a count" '3 in this thread'
must "inbox: folders" '[mail-folder-quarantine]'
must "message: heading is the subject" '[page-title] "Harbour barrier: site visit on Thursday"'
must "message: header lines" '[mail-from]'
must "message: the body in the reader" '[mail-body]'
must "message: reply label names the sender" 'Reply to Ana Reyes'
must "message: attachment row" '[mail-attachment-a1]'
must "prompt: summarise records an ask" 'Asked: Summarise this thread'
must "radio: read aloud is requested" 'The radio has been asked to read this.'
must "attachment: opens as a page by address" 'file:/Mail/m6/site-plan.pdf'
must "reply: a draft page" '[mail-field-body]'
must "reply: seeded recipient" 'ana@example.com'
must "reply: saved, not sent" 'Saved as a draft. It has not been sent.'
must "reply: approval link offered" '[mail-approval]'
must "reply: approval emailed" 'An approval link was emailed to you.'
must "reply: open the approval page" '[mail-open-review]'
must "thread: quoted text is folded" '[mail-quoted-toggle]'
must "html: text made from the formatted version" 'This message has only a formatted version'
must "html: original in a sandboxed frame" '[mail-original-frame]'
must "quarantine: the trust note" 'Held back'
must "drafts: a draft as a row" '[mail-row-demo-draft-1]'
must "draft: opened as a page" 'Festival numbers'
must "state: empty" '[mail-empty]'
must "state: offline" '[mail-offline]'
must "state: error" '[mail-error]'
[ -s "$SHOTS/18-loading.png" ] || fail "state: loading" "no screenshot of the loading screen"
echo "ok: state: loading (screenshot 18)"
must "new message: refused when empty" 'Add who it is for.'
must "dark: inbox again" '[mail-row-m5]'
[ "$(ls "$SHOTS"/*.png 2>/dev/null | wc -l)" -ge 20 ] || fail "screenshots" "fewer than 20 in $SHOTS"
echo "ok: screenshots in $SHOTS"
if grep -q 'ERROR\|Fatal error' "$OUT"; then echo "note: errors in the app log:"; grep -m8 'ERROR\|Fatal error' "$OUT"; fi
echo "PROOF OK"
