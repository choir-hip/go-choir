#!/bin/sh
# Build llama.cpp b12259 as a STATIC xcframework (iOS device + iOS simulator) for the Exact app.
# Why not the release zip: its llama.xcframework is a dynamic framework with no simulator slice, and Exact links only
# static slices (it does not embed a dynamic framework into the app). This runs the release's own
# build-xcframework.sh for ios-sim and ios-device, takes the merged static archive each leaves in <build>/temp/combined.a,
# and wraps it with the C headers and a small module map so `import llama` works.
# Output: mobile/.deps/xcframeworks/llama.xcframework (gitignored).
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
DEPS="$HERE/.deps"
SRC="$DEPS/llama.cpp"
TAG=b12259
[ -d "$SRC" ] || git clone -q --depth 1 --branch "$TAG" https://github.com/ggml-org/llama.cpp.git "$SRC"
cd "$SRC"
if [ ! -f build-ios-device/src/Release-iphoneos/libllama.a ] || [ ! -f build-ios-sim/src/Release-iphonesimulator/libllama.a ]; then
  # The release script may fail after the static libraries are built (its dynamic-framework step); only they are needed.
  env -i HOME="$HOME" PATH=/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer \
    /bin/bash ./build-xcframework.sh ios-sim ios-device || true
fi

mkdir -p "$DEPS/headers-llama"
rm -rf "$DEPS/headers-llama"/*
cp include/llama.h ggml/include/ggml.h ggml/include/ggml-alloc.h ggml/include/ggml-backend.h ggml/include/ggml-cpu.h ggml/include/gguf.h "$DEPS/headers-llama/"
cat > "$DEPS/headers-llama/module.modulemap" <<'EOF'
module llama {
    header "llama.h"
    link "c++"
    link framework "Accelerate"
    link framework "Metal"
    link framework "Foundation"
    export *
}
EOF
OUT="$DEPS/xcframeworks/llama.xcframework"
mkdir -p "$DEPS/xcframeworks"; rm -rf "$OUT"
mkdir -p "$DEPS/llama-slices/sim" "$DEPS/llama-slices/dev"
# Merge the static libraries the release script builds (the same list its combine step uses).
merge() { # build-dir release-dir out
  LIBS=""
  for L in src/$2/libllama.a ggml/src/$2/libggml.a ggml/src/$2/libggml-base.a ggml/src/$2/libggml-cpu.a \
           ggml/src/ggml-metal/$2/libggml-metal.a ggml/src/ggml-blas/$2/libggml-blas.a tools/mtmd/$2/libmtmd.a vendor/hash/$2/libvendor-hash.a; do
    [ -f "$1/$L" ] && LIBS="$LIBS $1/$L"
  done
  "$HERE/scripts/xe.sh" /usr/bin/xcrun libtool -static -o "$3" $LIBS 2>/dev/null
}
merge build-ios-sim Release-iphonesimulator "$DEPS/llama-slices/sim/libllama.a"
merge build-ios-device Release-iphoneos "$DEPS/llama-slices/dev/libllama.a"
"$HERE/scripts/xe.sh" /usr/bin/xcodebuild -create-xcframework \
  -library "$DEPS/llama-slices/sim/libllama.a" -headers "$DEPS/headers-llama" \
  -library "$DEPS/llama-slices/dev/libllama.a" -headers "$DEPS/headers-llama" \
  -output "$OUT"
echo "built $OUT"
