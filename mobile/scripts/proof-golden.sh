#!/bin/sh
# The native half of the golden-documents check: AutoradioKit's GoldenDocumentsTests parse every file in
# internal/texturedoc/golden with TextureDocParser (the phone's own parser) and write their projections to
# $GOLDEN_OUT/native (default: $PROOF_SHOTS/native). It needs no Exact app build and no simulator, only `swift test`
# in mobile/AutoradioKit. Run it through the shared lock: shared-proof.sh <pushed-branch> proof-golden.sh.
# Last line: PROOF OK, or PROOF FAILED at <step>.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
export GOLDEN_OUT="${GOLDEN_OUT:-${PROOF_SHOTS:-${TMPDIR:-/tmp}/golden-out}}"
mkdir -p "$GOLDEN_OUT"
SCRATCH="${TMPDIR:-/tmp}/proof-golden.$$"
mkdir -p "$SCRATCH"
trap 'rm -rf "$SCRATCH"' EXIT
echo "swift test (GoldenDocumentsTests), projections to $GOLDEN_OUT/native"
"$HERE/xe.sh" /usr/bin/env GOLDEN_OUT="$GOLDEN_OUT" /usr/bin/swift test --package-path "$HERE/../AutoradioKit" --filter GoldenDocumentsTests > "$SCRATCH/swift-test.txt" 2>&1 \
  || { grep -E "error:|failed|Fatal" "$SCRATCH/swift-test.txt" | head -30; tail -15 "$SCRATCH/swift-test.txt"; echo "PROOF FAILED at swift test"; exit 1; }
grep -E "Executed [0-9]+ tests" "$SCRATCH/swift-test.txt" | tail -1
n=$(ls "$GOLDEN_OUT/native"/*.json 2>/dev/null | wc -l | tr -d ' ')
[ "$n" -ge 8 ] || { echo "PROOF FAILED at projections ($n files)"; exit 1; }
echo "$n native projections written"
echo "PROOF OK"
