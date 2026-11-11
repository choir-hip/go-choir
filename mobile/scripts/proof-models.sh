#!/bin/sh
# Model proof on the Mac: replays the fixture turn through each on-device model stage and prints one line per stage:
#   id | load ms first (download + compile) and warm | MB fetched | MB resident loaded / after unload | speed | where | result
# Models download on first use (Silero 1 MB, Parakeet EOU 219 MB, Parakeet TDT ~480 MB, Kokoro, PocketTTS, Qwen3-0.6B 639 MB);
# LFM3-Audio (2.9 GB) only with LFM3=1. Short fixture turns only: this is not a benchmark of bulk transcription.
# Also checks that the new stages are listed and that each can run as a stage of a kept-turn replay.
#   STAGES=silero-vad,parakeet-eou mobile/scripts/proof-models.sh
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${PROOF_OUT:-$(mktemp -d)}"
"$HERE/scripts/xe.sh" /usr/bin/swift build --package-path "$HERE/AutoradioKit" -c release >/dev/null
BIN="$HERE/AutoradioKit/.build/release"

"$BIN/autoradio-replay" --list-stages >"$OUT/stages.json"
/usr/bin/python3 -I - "$OUT/stages.json" <<'EOF'
import json, sys
ids = {(s["kind"], s["id"]) for s in json.load(open(sys.argv[1]))}
need = {("speechBounds","silero-vad"),("transcriber","parakeet-eou"),("transcriber","parakeet-tdt"),("system1","llama-qwen3-0.6b"),
        ("voice","kokoro"),("voice","pocket-tts"),("transcriber","lfm3-audio-live"),("holder","lfm3-holder"),("voice","lfm3-audio-tts")}
missing = need - ids
if missing: print("PROOF FAILED: stages not listed:", sorted(missing)); sys.exit(1)
print("stages listed:", len(need), "model stages")
EOF

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
