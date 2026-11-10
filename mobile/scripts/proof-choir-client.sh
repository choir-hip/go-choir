#!/bin/sh
# Proof of the Choir client stages.
#   proof-choir-client.sh                      unit checks against a local stub server (no network, no key)
#   proof-choir-client.sh --api-key-file F     also replays the kept turns with choir-system1 / choir-desk
#                                              against staging (https://choir.news, or --base-url U) as the
#                                              listener who owns the key. One result line per turn.
# The key is read from the file only. It is never a flag value and is never printed. The script does not
# look for a key anywhere else.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
KEYFILE=""; BASE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --api-key-file) KEYFILE="$2"; shift 2 ;;
    --base-url) BASE="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

echo "== unit checks against a local URLProtocol stub (System 1, sign-in, front page, desk) =="
"$HERE/scripts/xe.sh" /usr/bin/swift test --package-path "$HERE/AutoradioKit" --filter ChoirClientTests 2>&1 | grep -E "Executed|error:" | tail -3

if [ -z "$KEYFILE" ]; then
  echo "STAGING RUN NOT DONE: no --api-key-file given. Only the stub checks above ran."
  exit 0
fi
[ -r "$KEYFILE" ] || { echo "cannot read the key file" >&2; exit 2; }

"$HERE/scripts/xe.sh" /usr/bin/swift build --package-path "$HERE/AutoradioKit" -c release >/dev/null
BIN="$HERE/AutoradioKit/.build/release/autoradio-replay"
OUT="${PROOF_OUT:-$(mktemp -d)}"
EXTRA=""
[ -n "$BASE" ] && EXTRA="--base-url $BASE"
FAIL=0
echo "== replay against staging =="
for turn in plain mixed; do
  # shellcheck disable=SC2086
  "$BIN" --turn "$HERE/fixtures/turns/$turn" --config "$HERE/fixtures/configs/choir.json" --api-key-file "$KEYFILE" $EXTRA \
      --out "$OUT/$turn.json" 2>"$OUT/$turn.err" || FAIL=1
  /usr/bin/python3 -I - "$OUT/$turn.json" "$turn" <<'EOF'
import json, sys
try:
    r = json.load(open(sys.argv[1]))
except Exception as e:
    print("%-6s NO RECORD (%s)" % (sys.argv[2], e)); sys.exit(0)
s = r["summary"]
t = {x["stage"]: x for x in s["timings"]}
calls = [k for k in t if k.startswith("choir ")]
ms = lambda k: ("%.0f" % ((t[k]["endedAt"] - t[k]["startedAt"]) * 1000)) if k in t else "-"
print("%-6s transcript=%r  system1: %d answers [%s] %s ms  desk revisions=%d  errors=%s" % (
    sys.argv[2], (s.get("transcript") or "")[:40],
    len(s["answers"]), ", ".join("%s=%s(%.2f)" % (a["questionID"], a["option"], a["score"]) for a in s["answers"]),
    ms("system1"), len(s["revisions"]), s["errors"] or "none"))
EOF
done
echo "records in $OUT"
exit $FAIL
