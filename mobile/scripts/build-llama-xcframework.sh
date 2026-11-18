#!/bin/sh
# Build llama.cpp b12259 as a STATIC xcframework (iOS device, iOS simulator, macOS arm64) for the Exact app.
# Why not the release zip: its llama.xcframework is a dynamic framework with no simulator slice, and Exact links only
# static slices (it does not embed a dynamic framework into the app). This runs the release's own
# build-xcframework.sh for ios-sim and ios-device, takes the merged static archive each leaves in <build>/temp/combined.a,
# and wraps it with the C headers and a small module map so `import llama` works.
# The macOS slice is built here, not by the release script: that script deletes every other build-* directory when it starts, so a
# `macos` run would throw away the iOS builds, and its macOS build is a universal arm64+x86_64 one at macOS 13.3. Ours is arm64 only
# at macOS 15.0 (the Kit's floor), with the same cmake options the release script uses (b12259, build_macos), in build-macos-arm64.
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

if [ ! -f build-macos-arm64/src/Release/libllama.a ]; then
  CF="-Wno-macro-redefined -Wno-shorten-64-to-32 -Wno-unused-command-line-argument -g"
  PATHS=/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin
  env -i HOME="$HOME" PATH="$PATHS" DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer \
    cmake -B build-macos-arm64 -G Xcode \
      -DCMAKE_XCODE_ATTRIBUTE_CODE_SIGNING_REQUIRED=NO -DCMAKE_XCODE_ATTRIBUTE_CODE_SIGN_IDENTITY="" -DCMAKE_XCODE_ATTRIBUTE_CODE_SIGNING_ALLOWED=NO \
      -DBUILD_SHARED_LIBS=OFF -DLLAMA_BUILD_APP=OFF -DLLAMA_BUILD_COMMON=OFF -DLLAMA_BUILD_EXAMPLES=OFF -DLLAMA_BUILD_TOOLS=OFF \
      -DLLAMA_BUILD_TESTS=OFF -DLLAMA_BUILD_SERVER=OFF -DLLAMA_BUILD_MTMD=ON -DGGML_METAL=ON -DGGML_METAL_EMBED_LIBRARY=ON \
      -DGGML_BLAS_DEFAULT=ON -DGGML_OPENMP=OFF \
      -DCMAKE_OSX_DEPLOYMENT_TARGET=15.0 -DCMAKE_OSX_ARCHITECTURES=arm64 \
      -DCMAKE_C_FLAGS="$CF" -DCMAKE_CXX_FLAGS="$CF" -DLLAMA_OPENSSL=OFF -S .
  env -i HOME="$HOME" PATH="$PATHS" DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer \
    cmake --build build-macos-arm64 --config Release -j "$(sysctl -n hw.logicalcpu)" -- -quiet
fi

rm -rf "$DEPS/headers-llama"
mkdir -p "$DEPS/headers-llama"
cp include/llama.h ggml/include/*.h "$DEPS/headers-llama/"
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
mkdir -p "$DEPS/llama-slices/sim" "$DEPS/llama-slices/dev" "$DEPS/llama-slices/mac"
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
merge build-macos-arm64 Release "$DEPS/llama-slices/mac/libllama.a"
"$HERE/scripts/xe.sh" /usr/bin/xcodebuild -create-xcframework \
  -library "$DEPS/llama-slices/sim/libllama.a" -headers "$DEPS/headers-llama" \
  -library "$DEPS/llama-slices/dev/libllama.a" -headers "$DEPS/headers-llama" \
  -library "$DEPS/llama-slices/mac/libllama.a" -headers "$DEPS/headers-llama" \
  -output "$OUT"
echo "built $OUT"
