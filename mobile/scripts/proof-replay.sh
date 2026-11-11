#!/bin/sh
# End-to-end proof on the Mac: replay kept turns (generated with `say`) through the real engine and
# check the resulting TurnRecords. Exits non-zero on any failure. Run outside the Nix shell is fine; it
# re-enters the Xcode toolchain itself.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${PROOF_OUT:-$(mktemp -d)}"
mkdir -p "$OUT"
"$HERE/scripts/xe.sh" /usr/bin/swift build --package-path "$HERE/AutoradioKit" -c release >/dev/null
BIN="$HERE/AutoradioKit/.build/release/autoradio-replay"

"$BIN" --list-stages >"$OUT/stages.json"

run() { # name turn config
  "$BIN" --turn "$HERE/fixtures/turns/$2" --config "$HERE/fixtures/configs/$3.json" --out "$OUT/$1.json"
}
run plain-open plain open
run mixed-open mixed open
run mixed-button mixed button

/usr/bin/python3 -I - "$OUT" "$HERE" <<'EOF'
import json, sys, os
out, here = sys.argv[1], sys.argv[2]
fail = []
stages = json.load(open(os.path.join(out, "stages.json")))
kinds = {s["kind"] for s in stages}
need = {"micSource","addressing","speechBounds","speakerCheck","transcriber","yieldPolicy","system1","holder","voice","desk","programme"}
if need - kinds: fail.append("missing stage kinds: %s" % sorted(need - kinds))
for name, turn in (("plain-open","plain"),("mixed-open","mixed"),("mixed-button","mixed")):
    r = json.load(open(os.path.join(out, name + ".json")))
    s = r["summary"]
    words = json.load(open(os.path.join(here, "fixtures/turns", turn, "context.json")))["expectedWords"]
    text = (s.get("transcript") or "").lower()
    for w in words:
        if w not in text: fail.append("%s: transcript %r lacks %r" % (name, text, w))
    for k in ("turnEndToAckMs", "turnEndToRevisionMs"):
        if s.get(k) is None: fail.append("%s: %s missing" % (name, k))
    stg = {t["stage"] for t in s["timings"]}
    for st in ("transcribe", "system1", "voice.firstAudio"):
        if st not in stg: fail.append("%s: timing %s missing" % (name, st))
    if s["errors"]: fail.append("%s: errors %s" % (name, s["errors"]))
    print("%-13s ack=%4.0f ms  revision=%4.0f ms  addressedBy=%s  transcript=%r" % (
        name, s.get("turnEndToAckMs") or -1, s.get("turnEndToRevisionMs") or -1, s.get("addressedBy"), text))
if fail:
    print("PROOF FAILED:\n  " + "\n  ".join(fail)); sys.exit(1)
print("PROOF OK  records in", out)
EOF
