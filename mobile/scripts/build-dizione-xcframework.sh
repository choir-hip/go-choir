#!/bin/sh
# Build the unchanged standalone Apache-2.0 Dizione port as a static XCFramework.
# ios arm64 (device) and macos arm64 slices; a C++ module map for the Swift side.
set -eu
SRC="${1:?dizione source directory}"
OUT="${2:?output directory}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
[ -f "$SRC/dizione.h" ] || { echo "missing $SRC/dizione.h" >&2; exit 1; }
mkdir -p "$OUT/dizione.xcframework"
for platform in ios macos; do
  case "$platform" in
    ios) sdk=iphoneos; sysroot="$(xcrun --sdk iphoneos --show-sdk-path)"
         triple=arm64-apple-ios18.0; suffix=ios-arm64; variant='' ;;
    macos) sdk=macosx; sysroot="$(xcrun --sdk macosx --show-sdk-path)"
         triple=arm64-apple-macos15.0; suffix=macos-arm64; variant='' ;;
  esac
  work="$OUT/dizione-$platform"
  mkdir -p "$work/include" "$OUT/dizione.xcframework/$suffix/Headers"
  cp "$SRC/dizione.h" "$work/include/"
  cp "$SRC/dizione.h" "$OUT/dizione.xcframework/$suffix/Headers/"
  for source in "$SRC"/*.cpp; do
    xcrun --sdk "$sdk" clang++ -target "$triple" -isysroot "$sysroot" -std=c++17 -O2 -fPIC \
      -I "$SRC" -I "$work/include" -c "$source" -o "$work/$(basename "$source" .cpp).o"
  done
  xcrun --sdk "$sdk" clang++ -target "$triple" -isysroot "$sysroot" -std=c++17 -O2 -fPIC \
    -I "$SRC" -I "$work/include" -c "$ROOT/scripts/tiny-voca/DizioneBridge.cpp" -o "$work/DizioneBridge.o"
  /usr/bin/libtool -static -o "$work/libdizione.a" "$work"/*.o
  cp "$work/libdizione.a" "$OUT/dizione.xcframework/$suffix/"
  cat > "$OUT/dizione.xcframework/$suffix/Headers/module.modulemap" <<'EOF'
module dizione {
  header "dizione.h"
  export *
}
EOF
done
cat > "$OUT/dizione.xcframework/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>AvailableLibraries</key><array>
    <dict><key>LibraryIdentifier</key><string>ios-arm64</string>
      <key>LibraryPath</key><string>libdizione.a</string>
      <key>SupportedArchitectures</key><array><string>arm64</string></array>
      <key>SupportedPlatform</key><string>ios</string></dict>
    <dict><key>LibraryIdentifier</key><string>macos-arm64</string>
      <key>LibraryPath</key><string>libdizione.a</string>
      <key>SupportedArchitectures</key><array><string>arm64</string></array>
      <key>SupportedPlatform</key><string>macos</string></dict>
  </array>
  <key>CFBundlePackageType</key><string>XFWK</string>
  <key>XCFrameworkFormatVersion</key><string>1.0</string>
</dict></plist>
PLIST
cat > "$OUT/dizione-module.modulemap" <<'EOF'
module dizione {
  header "dizione.h"
  export *
}
EOF
