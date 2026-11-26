#!/usr/bin/env bash
# Prove the Android model runtime that ships in the Autoradio lab-one APK, on the named phone.
#
# It pulls the installed APK, extracts lib/arm64-v8a, checks each library's ABI and 16 KiB page
# alignment, then runs the APK's own choir.autoradio.NativeModels (Kotlin + JNI) and its ONNX
# Runtime against the packaged shared libraries:
#   - Qwen3-0.6B Q4_K_M generates one answer;
#   - EmbeddingGemma 2 Q8_0 returns a 768-dimensional unit vector;
#   - Silero v6.2.1 returns per-window speech probabilities on the repository's VAD fixture.
# Nothing is installed or changed by this script: it uses the installed package and deletes its
# own /data/local/tmp work directory on exit.
#
#   ANDROID_SERIAL=ZY22H5K8QR scripts/proofs/android-a6-model-runtime.sh [--out DIR]
#
# Requires adb, the NDK named by mobile/android-model-runtime.json (ANDROID_NDK_HOME or the SDK),
# JDK 21 (JAVA_HOME), and the model files under ~/.cache/model-bench (fetch them with
# scripts/model-bench/fetch_native.py and the EmbeddingGemma/Silero jobs; --fetch downloads them).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ADB="${ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"
SERIAL="${ANDROID_SERIAL:-ZY22H5K8QR}"
PKG="${PKG:-news.choir.autoradio.lab.one}"
CACHE="${AUTORADIO_PROOF_CACHE:-$HOME/.cache/autoradio-proof/android-a6}"
OUT="$ROOT/docs/evidence/android-a6-model-runtime-2026-11-26"
FETCH=0
while [ $# -gt 0 ]; do
  case "$1" in
    --out) OUT="$2"; shift 2 ;;
    --fetch) FETCH=1; shift ;;
    *) echo "usage: $0 [--out DIR] [--fetch]" >&2; exit 2 ;;
  esac
done
mkdir -p "$OUT" "$CACHE"

NDK="${ANDROID_NDK_HOME:-}"
if [ -z "$NDK" ]; then
  REV="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["ndk"]["revision"])' "$ROOT/mobile/android-model-runtime.json")"
  NDK="$HOME/Library/Android/sdk/ndk/$REV"
fi
CLANGXX="$NDK/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android34-clang++"
READELF="$NDK/toolchains/llvm/prebuilt/darwin-x86_64/bin/llvm-readelf"
JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}"
D8="$(ls -d "$HOME"/Library/Android/sdk/build-tools/* | tail -1)/d8"
ANDROID_JAR="$(ls -d "$HOME"/Library/Android/sdk/platforms/android-* | tail -1)/android.jar"
[ -x "$CLANGXX" ] && [ -x "$READELF" ] || { echo "NDK toolchain not found at $NDK" >&2; exit 1; }
[ -x "$JAVA_HOME/bin/javac" ] || { echo "javac not found at $JAVA_HOME" >&2; exit 1; }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/a6-model-runtime.XXXXXX")"
DEVICE="/data/local/tmp/a6-runtime-proof"
cleanup() {
  "$ADB" -s "$SERIAL" shell "rm -rf $DEVICE" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

run_adb() { "$ADB" -s "$SERIAL" "$@"; }

echo "== device"
run_adb shell 'getprop ro.product.model; getprop ro.build.version.release; getprop ro.build.version.sdk' | tee "$OUT/device.txt"
APK_PATH="$(run_adb shell pm path "$PKG" | sed 's/^package://' | tr -d '\r')"
[ -n "$APK_PATH" ] || { echo "$PKG is not installed on $SERIAL" >&2; exit 1; }
run_adb pull "$APK_PATH" "$WORK/base.apk" >/dev/null
APK_SHA="$(shasum -a 256 "$WORK/base.apk" | cut -d' ' -f1)"
VERSION="$(run_adb shell "dumpsys package $PKG" | grep -E 'versionName=|versionCode=' | sed 's/^ *//' | tr '\n' ' ' | sed 's/ *$//')"
{
  echo "package=$PKG"
  echo "apk=$APK_PATH"
  echo "apk_sha256=$APK_SHA"
  echo "apk_bytes=$(wc -c < "$WORK/base.apk" | tr -d ' ')"
  echo "installed=$VERSION"
} | tee "$OUT/apk.txt"

echo "== packaged libraries"
mkdir -p "$WORK/libs"
(cd "$WORK" && unzip -o -q base.apk 'lib/arm64-v8a/*.so' -d apk-libs)
cp "$WORK"/apk-libs/lib/arm64-v8a/*.so "$WORK/libs/"
: > "$OUT/packaged-libs.txt"
for lib in "$WORK"/libs/*.so; do
  name="$(basename "$lib")"
  sha="$(shasum -a 256 "$lib" | cut -d' ' -f1)"
  alignments="$("$READELF" -lW "$lib" | awk '/LOAD/{print $NF}' | sort -u)"
  align="$(printf '%s' "$alignments" | tr '\n' ',' | sed 's/,$//')"
  soname="$("$READELF" -dW "$lib" | sed -n 's/.*(SONAME).*\[\(.*\)\]/\1/p')"
  printf '%s sha256=%s load=%s soname=%s\n' "$name" "$sha" "$align" "$soname" >> "$OUT/packaged-libs.txt"
  if [ -z "$alignments" ]; then echo "$name has no LOAD segment" >&2; exit 1; fi
  for value in $alignments; do
    if [ "$((value))" -lt 16384 ]; then echo "$name is not 16 KiB page aligned: $align" >&2; exit 1; fi
  done
done
cat "$OUT/packaged-libs.txt"

echo "== proof programs"
"$JAVA_HOME/bin/javac" --release 8 -d "$WORK/classes" "$ROOT/scripts/proofs/android-model-probe/ModelProbe.java" 2>/dev/null
"$D8" --release --lib "$ANDROID_JAR" --output "$WORK/probe.jar" "$WORK/classes/choir/probe/ModelProbe.class"
AAR="$CACHE/onnxruntime-android-1.32.0.aar"
AAR_SHA=8e8dbb412ed4939675a157702f0c1e3621b436a645af7f7244536ac20bfad628
if [ ! -f "$AAR" ] || [ "$(shasum -a 256 "$AAR" | cut -d' ' -f1)" != "$AAR_SHA" ]; then
  echo "fetching the pinned ONNX Runtime AAR (headers)"
  curl -fsSL -o "$AAR.tmp" "https://repo1.maven.org/maven2/com/microsoft/onnxruntime/onnxruntime-android/1.32.0/onnxruntime-android-1.32.0.aar"
  [ "$(shasum -a 256 "$AAR.tmp" | cut -d' ' -f1)" = "$AAR_SHA" ] || { echo "ONNX Runtime AAR hash does not match its pin" >&2; exit 1; }
  mv "$AAR.tmp" "$AAR"
fi
(cd "$WORK" && unzip -o -q "$AAR" 'headers/*' -d ort)
"$CLANGXX" -std=c++17 -O2 -fPIC -I "$WORK/ort/headers" "$ROOT/scripts/proofs/android-model-probe/silero_ort.cpp" \
  -L "$WORK/libs" -lonnxruntime -Wl,-rpath,'$ORIGIN' -static-libstdc++ -o "$WORK/silero-ort"

echo "== models"
python3 - "$ROOT/mobile/android-model-runtime.json" "$WORK" "$FETCH" <<'PY'
import hashlib, json, os, pathlib, shutil, sys, urllib.request
manifest, work, fetch = json.load(open(sys.argv[1])), pathlib.Path(sys.argv[2]), sys.argv[3] == "1"
cache = pathlib.Path(os.path.expanduser("~/.cache/model-bench/android-models"))

def digest(path):
    h = hashlib.sha256()
    with open(path, "rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""): h.update(chunk)
    return h.hexdigest()
wanted = {"qwen3-0.6b-q4_k_m", "embeddinggemma-2-q8_0", "silero-vad-v6.2.1"}
by_id = {model["id"]: model for model in manifest["models"]}
for model_id in sorted(wanted):
    file = by_id[model_id]["files"][0]
    source = next((cache / folder / file["name"] for folder in ("qwen-q4", "qwen-q8", "embeddinggemma", "silero", "llama-generic") if (cache / folder / file["name"]).is_file()), cache / "missing" / file["name"])
    if source.is_file() and digest(source) == file["sha256"]:
        shutil.copyfile(source, work / file["name"])
        print(f"{file['name']} from {source}", file=sys.stderr)
        continue
    if not fetch:
        sys.exit(f"{source} is missing or wrong; rerun with --fetch")
    target = work / file["name"]
    with urllib.request.urlopen(urllib.request.Request(file["url"], headers={"User-Agent": "Autoradio-proof"}), timeout=180) as response, target.open("wb") as output:
        shutil.copyfileobj(response, output, 1024 * 1024)
    if target.stat().st_size != file["bytes"] or digest(target) != file["sha256"]:
        sys.exit(f"{file['name']} does not match its pin")
    print(f"{file['name']} downloaded", file=sys.stderr)
PY

printf '%s' '<|im_start|>system
You are a helpful assistant.<|im_end|>
<|im_start|>user
What is the capital of France? Answer in one short sentence.<|im_end|>
<|im_start|>assistant
<think>

</think>

' > "$WORK/text.prompt"
printf '%s' 'title: Harbor barrier | text: The harbor council voted to approve a tide barrier across the east channel.' > "$WORK/embed.prompt"
python3 - "$ROOT/docs/evidence/model-bench-android-2026-11-26/silero-v6.2.1/output.wav" "$WORK/silero.f32" <<'PY'
import array, sys, wave
with wave.open(sys.argv[1], "rb") as f:
    assert (f.getnchannels(), f.getframerate(), f.getsampwidth()) == (1, 16000, 2), "the VAD fixture must be 16 kHz mono 16-bit PCM"
    pcm = f.readframes(f.getnframes())
samples = array.array("h"); samples.frombytes(pcm)
with open(sys.argv[2], "wb") as out: array.array("f", [s / 32768.0 for s in samples]).tofile(out)
PY

echo "== push"
run_adb shell "rm -rf $DEVICE && mkdir -p $DEVICE/libs"
run_adb push "$WORK/probe.jar" "$WORK/silero-ort" "$WORK/text.prompt" "$WORK/embed.prompt" "$WORK/silero.f32" "$WORK"/Qwen3-0.6B-Q4_K_M.gguf "$WORK"/embeddinggemma-2-Q8_0.gguf "$WORK"/silero_vad_16k_op15.onnx "$DEVICE/" >/dev/null
run_adb push "$WORK"/libs/*.so "$DEVICE/libs/" >/dev/null
run_adb shell "chmod 755 $DEVICE/silero-ort"

probe() { # kind model-id model-file prompt threads max-tokens report
  run_adb shell "cd $DEVICE && LD_LIBRARY_PATH=$DEVICE/libs /system/bin/app_process -Djava.class.path=$DEVICE/probe.jar:$APK_PATH /system/bin --nice-name=a6probe choir.probe.ModelProbe $1 $2 $DEVICE/$3 @$DEVICE/$4 $5 $6 $DEVICE/$7"
  run_adb shell "cat $DEVICE/$7"
}

echo "== Qwen3-0.6B Q4_K_M through the APK's NativeModels"
probe text qwen3-0.6b-q4_k_m Qwen3-0.6B-Q4_K_M.gguf text.prompt 4 32 text.report | tee "$OUT/qwen3-q4.txt"
echo "== EmbeddingGemma 2 Q8_0 through the APK's NativeModels"
probe embedding embeddinggemma-2-q8_0 embeddinggemma-2-Q8_0.gguf embed.prompt 4 1 embed.report | tee "$OUT/embeddinggemma-q8.txt"
echo "== Silero v6.2.1 through the APK's ONNX Runtime"
run_adb shell "cd $DEVICE && LD_LIBRARY_PATH=$DEVICE/libs ./silero-ort $DEVICE/silero_vad_16k_op15.onnx $DEVICE/silero.f32" | tee "$OUT/silero-vad.txt"

echo "evidence written to $OUT"
