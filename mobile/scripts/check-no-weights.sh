#!/bin/sh
# Nothing ships with the app (owner, 2026-11-11): fail if a built app bundle contains model weights.
#   check-no-weights.sh <path to the .app bundle>      (the simulator proof passes `simctl get_app_container ... app`)
set -eu
B="${1:?usage: check-no-weights.sh <app bundle>}"
[ -d "$B" ] || { echo "no such bundle: $B" >&2; exit 2; }
BAD=$(find "$B" \( -name '*.gguf' -o -name '*.mlmodelc' -o -name '*.mlpackage' -o -name '*.mlmodel' -o -name '*.onnx' -o -name '*.safetensors' -o -name '*.bin' \) -size +100k 2>/dev/null || true)
if [ -n "$BAD" ]; then echo "PROOF FAILED: model weights inside the app bundle:"; echo "$BAD"; exit 1; fi
BIG=$(find "$B" -type f -size +30M 2>/dev/null | head -5 || true)
echo "ok: no model weights in the bundle ($(du -sh "$B" | cut -f1) total)"
[ -z "$BIG" ] || { echo "note: files over 30 MB:"; echo "$BIG"; }
