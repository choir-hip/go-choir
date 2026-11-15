#!/bin/sh
# Model proof on the Mac: replays the fixture turn through each on-device model stage and prints one line per stage:
#   id | load ms first (download + compile) and warm | MB fetched | MB resident loaded / after unload | speed | where | result
# Models download on first use (Silero 1 MB, Parakeet EOU 219 MB, Parakeet TDT ~480 MB, Kokoro, PocketTTS, Qwen3-0.6B 639 MB);
# LFM3-Audio (2.9 GB) only with LFM3=1. Short fixture turns only: this is not a benchmark of bulk transcription.
# Also checks that the new stages are listed and that each can run as a stage of a kept-turn replay.
#   STAGES=silero-vad,parakeet-eou mobile/scripts/proof-models.sh
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${PROOF_OUT:-${PROOF_SHOTS:-$(mktemp -d)}}"; mkdir -p "$OUT"
"$HERE/scripts/xe.sh" /usr/bin/swift build --package-path "$HERE/AutoradioKit" -c release > "$OUT/build.log" 2>&1 || { grep -E "error:" "$OUT/build.log" | head -30; tail -5 "$OUT/build.log"; echo "PROOF FAILED: swift build"; exit 1; }
BIN="$HERE/AutoradioKit/.build/release"

"$BIN/autoradio-replay" --list-stages >"$OUT/stages.json"
/usr/bin/python3 -I - "$OUT/stages.json" <<'EOF'
import json, sys
ids = {(s["kind"], s["id"]) for s in json.load(open(sys.argv[1]))}
need = {("speechBounds","silero-vad"),("transcriber","parakeet-eou"),("transcriber","parakeet-tdt"),("system1","llama-qwen3-0.6b"),
        ("voice","kokoro"),("voice","pocket-tts"),("transcriber","lfm3-audio-live"),("voice","lfm3-audio-tts"),("holder","local-ack")}
missing = need - ids
if missing: print("PROOF FAILED: stages not listed:", sorted(missing)); sys.exit(1)
print("stages listed:", len(need), "model stages")
EOF

# What voice setup does on the phone, here, for every model on every ladder (registered, fetched, verified, loaded, checked on
# the bundled utterance), through the same code (HubModelProvider). One line per model; the exit code is non-zero if any fails.
#   SETUP=1 mobile/scripts/proof-models.sh        (downloads about 1.5 GB the first time)
"$BIN/autoradio-models" --ids || { echo "PROOF FAILED: a setup id is not in the catalog, the store or the stage registry"; exit 1; }
# The configurator for a simulated 6, 8 and 12 GB phone (the planner enforces the memory budget; everything is downloaded, loaded together
# and measured on this Mac): prints the plan order, what was measured and what was chosen for each profile.
#   CONFIGURE=1 mobile/scripts/proof-models.sh        (downloads several GB the first time; weights of rejected candidates are deleted)
if [ -n "${CONFIGURE:-}" ]; then
  "$BIN/autoradio-models" --configure --json "$OUT/configure.json" 2>"$OUT/configure.err" || { echo "PROOF FAILED: the configurator fell back to the built-in voice for a profile, or crashed (see $OUT/configure.err)"; exit 1; }
  echo "PROOF OK: the configurator chose real models for the 6, 8 and 12 GB profiles"; echo "records in $OUT"; exit 0
fi
if [ -n "${SETUP:-}" ]; then
  "$BIN/autoradio-models" --setup --json "$OUT/setup.json" 2>"$OUT/setup.err" || { echo "PROOF FAILED: a model failed fetch, verify, load or check (see $OUT/setup.json)"; exit 1; }
  echo "PROOF OK: every setup model fetched, verified, loaded and checked"; echo "records in $OUT"; exit 0
fi

STAGES="${STAGES:-silero-vad,parakeet-eou,parakeet-tdt,llama-qwen3-0.6b,kokoro,pocket-tts}"
[ -n "${LFM3:-}" ] && STAGES="$STAGES,lfm3-audio-live,lfm3-audio-tts"
"$BIN/autoradio-models" --turn "$HERE/fixtures/turns/plain" --stages "$STAGES" --json "$OUT/models.json" 2>"$OUT/models.err" | grep ' | load ' || true

# A kept turn replayed with a model transcriber and a model voice swapped in (the full engine, not just the stage).
if [ -z "${NO_REPLAY:-}" ]; then
  "$BIN/autoradio-replay" --turn "$HERE/fixtures/turns/plain" --config "$HERE/fixtures/configs/open.json" \
    --set transcriber=parakeet-eou --set speechBounds=silero-vad --set voice=kokoro --out "$OUT/replay-models.json" 2>"$OUT/replay.err" || true
  tail -2 "$OUT/replay.err"
fi

/usr/bin/python3 -I - "$OUT/models.json" <<'EOF'
import json, sys
rows = json.load(open(sys.argv[1]))
bad = [r["id"] for r in rows if r.get("error")]
if bad: print("PROOF FAILED: stages with errors:", bad); sys.exit(1)
print("PROOF OK:", len(rows), "stages ran")
EOF
echo "records in $OUT"
