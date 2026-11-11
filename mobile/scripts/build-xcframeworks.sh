#!/bin/sh
# Build the third-party Swift/Core ML libraries the Exact app links, as STATIC xcframeworks that Exact's
# `modules/apple/*.xcframework` mechanism accepts (it links a static slice with -L/-l or a static framework with -F/-framework;
# it never embeds a dynamic framework). Output: mobile/.deps/xcframeworks/*.xcframework (gitignored, not committed).
#
#   build-xcframeworks.sh fluidaudio    # FluidAudio 0.18.3, device + simulator slices, TTS trait on
#   build-xcframeworks.sh llama         # llama.cpp b12259, device + simulator slices, static, Metal on
#
# The Mac replay command does not use these: SwiftPM resolves the same libraries itself (Package.swift).
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
X="$HERE/scripts/xe.sh"
DEPS="$HERE/.deps"
OUT="$DEPS/xcframeworks"
mkdir -p "$OUT"

fluidaudio() {
  TAG=v0.18.3
  SRC="$DEPS/FluidAudio"
  [ -d "$SRC" ] || git clone -q --depth 1 --branch "$TAG" https://github.com/FluidInference/FluidAudio.git "$SRC"
  for SLICE in sim dev; do
    # FluidAudio's own deployment target is iOS 17; Kokoro v3 and LFM3 are gated by #available(iOS 18) inside it.
    if [ "$SLICE" = sim ]; then SDKN=iphonesimulator; TRIPLE=arm64-apple-ios17.0-simulator; else SDKN=iphoneos; TRIPLE=arm64-apple-ios17.0; fi
    SDK="$($X /usr/bin/xcrun --sdk $SDKN --show-sdk-path)"
    # Disable default traits, enable only TTS: no Diarizer, and no NemoTextProcessing (a prebuilt Rust staticlib; TTS
    # frontends then pass text through unchanged).
    $X /usr/bin/swift build -c release --package-path "$SRC" --scratch-path "$DEPS/build-fa-$SLICE" --product FluidAudio \
      --triple "$TRIPLE" --sdk "$SDK" --disable-default-traits --traits TTS -Xswiftc -swift-version -Xswiftc 5
    P="$DEPS/build-fa-$SLICE/out/Products/Release-$SDKN"
    H="$DEPS/headers-fa-$SLICE"
    rm -rf "$H"; mkdir -p "$H"
    cp "$SRC/Sources/FastClusterWrapper/include/FastClusterWrapper.h" "$SRC/Sources/MachTaskSelfWrapper/include/MachTaskSelf.h" "$H/"
    cat > "$H/module.modulemap" <<'EOF'
module FastClusterWrapper {
    header "FastClusterWrapper.h"
    export *
}
module MachTaskSelfWrapper {
    header "MachTaskSelf.h"
    export *
}
EOF
  done
  rm -rf "$OUT/FluidAudio.xcframework"
  $X /usr/bin/xcodebuild -create-xcframework \
    -library "$DEPS/build-fa-sim/out/Products/Release-iphonesimulator/libFluidAudio.a" -headers "$DEPS/headers-fa-sim" \
    -library "$DEPS/build-fa-dev/out/Products/Release-iphoneos/libFluidAudio.a" -headers "$DEPS/headers-fa-dev" \
    -output "$OUT/FluidAudio.xcframework"
  # xcodebuild refuses a .swiftmodule without a .swiftinterface in -headers, so the modules go into each slice afterwards.
  for D in "$OUT"/FluidAudio.xcframework/*/Headers; do
    case "$D" in *simulator*) P="$DEPS/build-fa-sim/out/Products/Release-iphonesimulator" ;; *) P="$DEPS/build-fa-dev/out/Products/Release-iphoneos" ;; esac
    cp -R "$P/FluidAudio.swiftmodule" "$P/LuxTtsG2pResources.swiftmodule" "$D/"
  done
  echo "built $OUT/FluidAudio.xcframework"
}

llama() {
  TAG=b12259
  SRC="$DEPS/llama.cpp"
  [ -d "$SRC" ] || git clone -q --depth 1 --branch "$TAG" https://github.com/ggml-org/llama.cpp.git "$SRC"
  echo "llama: see build-llama-xcframework.sh" >&2
  exec "$HERE/scripts/build-llama-xcframework.sh"
}

case "${1:-}" in
  fluidaudio) fluidaudio ;;
  llama) llama ;;
  *) echo "usage: $0 fluidaudio|llama" >&2; exit 2 ;;
esac
