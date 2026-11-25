#!/bin/sh
# Build the unchanged standalone Apache-2.0 Dizione port as a static XCFramework.
# ios arm64 (device) and macos arm64 slices; a C++ module map for the Swift side.
set -eu
SRC="${1:?dizione source directory}"
OUT="${2:?output directory}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
[ -f "$SRC/dizione.h" ] || { echo "missing $SRC/dizione.h" >&2; exit 1; }
mkdir -p "$OUT/dizione.xcframework"
for platform in ios sim macos; do
  case "$platform" in
    ios) sdk=iphoneos; sysroot="$(xcrun --sdk iphoneos --show-sdk-path)"
         triple=arm64-apple-ios18.0; suffix=ios-arm64 ;;
    sim) sdk=iphonesimulator; sysroot="$(xcrun --sdk iphonesimulator --show-sdk-path)"
         triple=arm64-apple-ios18.0-simulator; suffix=ios-arm64-simulator ;;
    macos) sdk=macosx; sysroot="$(xcrun --sdk macosx --show-sdk-path)"
         triple=arm64-apple-macos15.0; suffix=macos-arm64 ;;
  esac
  work="$OUT/dizione-$platform"
  mkdir -p "$work/include"
  cp "$SRC"/*.h "$work/include/"
  cp "$ROOT/scripts/tiny-voca/dizione_abi.h" "$work/include/"
  # A Swift module for Exact's app module: `-I Headers` finds this map.
  cat > "$work/include/module.modulemap" <<'EOF'
module dizione {
  header "dizione_abi.h"
  link "c++"
  export *
}
EOF
  for source in "$SRC"/*.cpp; do
    xcrun --sdk "$sdk" clang++ -target "$triple" -isysroot "$sysroot" -std=c++17 -O2 -fPIC \
      -I "$SRC" -I "$work/include" -c "$source" -o "$work/$(basename "$source" .cpp).o"
  done
  xcrun --sdk "$sdk" clang++ -target "$triple" -isysroot "$sysroot" -std=c++17 -O2 -fPIC \
    -I "$SRC" -I "$work/include" -c "$ROOT/scripts/tiny-voca/DizioneBridge.cpp" -o "$work/DizioneBridge.o"
  /usr/bin/libtool -static -o "$work/libdizione.a" "$work"/*.o
done
# Create it the way scripts/build-llama-xcframework.sh makes llama's: a static
# library plus Headers per slice, which Exact turns into a Swift module.
rm -rf "$OUT/dizione.xcframework"
xcrun xcodebuild -create-xcframework \
  -library "$OUT/dizione-sim/libdizione.a" -headers "$OUT/dizione-sim/include" \
  -library "$OUT/dizione-ios/libdizione.a" -headers "$OUT/dizione-ios/include" \
  -library "$OUT/dizione-macos/libdizione.a" -headers "$OUT/dizione-macos/include" \
  -output "$OUT/dizione.xcframework" >/dev/null
