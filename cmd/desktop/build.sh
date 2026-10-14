#!/usr/bin/env bash
# Build, sign (ad hoc) and run Choir.app on macOS.
#
#   cmd/desktop/build.sh            # build bin/Choir.app (default)
#   cmd/desktop/build.sh run        # build, then run it in the foreground (logs to the terminal)
#   cmd/desktop/build.sh test       # go test + go vet for this module
#   cmd/desktop/build.sh icon       # regenerate build/darwin/icons.icns from appicon.png
#   cmd/desktop/build.sh clean
#
# CHOIR_BACKEND picks the server at run time (default https://choir.news).
#
# The build deliberately runs outside the repo's Nix dev environment. That
# environment exports CGO_LDFLAGS=-licui18n -licuuc (for Dolt), the Nix clang
# wrapper and a Nix Apple SDK, which link /nix/store dylibs into the app and
# make it unrunnable on any other Mac. Here cgo uses Xcode's clang and SDK, and
# the build fails if the binary references /nix/store.
#
# Signing is ad hoc: no Apple Developer account is needed, and the result runs
# on this Mac. Distribution needs Developer ID signing and notarization.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cmd="${1:-build}"
min_macos="12.0"
bundle="$here/bin/Choir.app"

go_bin="$(command -v go || true)"
if [[ -z "$go_bin" ]]; then
  echo "build.sh: go not found on PATH" >&2
  exit 1
fi

clean_env() {
  unset CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_LDFLAGS PKG_CONFIG_PATH \
    LD_LIBRARY_PATH DYLD_LIBRARY_PATH NIX_CFLAGS_COMPILE NIX_LDFLAGS \
    DEVELOPER_DIR SDKROOT CC CXX LD AR
  export PATH="/usr/bin:/bin:/usr/sbin:/sbin:$(dirname "$go_bin")"
  local clang sdk
  if ! clang="$(xcrun --find clang 2>/dev/null)" || ! sdk="$(xcrun --sdk macosx --show-sdk-path 2>/dev/null)"; then
    echo "build.sh: Xcode or the Command Line Tools are required (xcode-select --install)" >&2
    exit 1
  fi
  export CGO_ENABLED=1 CC="$clang" SDKROOT="$sdk" MACOSX_DEPLOYMENT_TARGET="$min_macos"
  export CGO_CFLAGS="-mmacosx-version-min=$min_macos" CGO_LDFLAGS="-mmacosx-version-min=$min_macos"
}

build() {
  clean_env
  cd "$here"
  local version commit built_at
  version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' build/darwin/Info.plist)"
  commit="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
  if [[ -n "$(git status --porcelain -- . 2>/dev/null)" ]]; then commit="$commit-dirty"; fi
  built_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

  mkdir -p bin
  go build -trimpath -o bin/Choir \
    -ldflags "-extldflags=-mmacosx-version-min=$min_macos -X main.appVersion=$version -X main.appCommit=$commit -X main.appBuiltAt=$built_at" .

  if otool -L bin/Choir | grep -q /nix/store; then
    echo "build.sh: bin/Choir links /nix/store libraries; refusing to package:" >&2
    otool -L bin/Choir | grep /nix/store >&2
    exit 1
  fi

  rm -rf "$bundle"
  mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"
  cp bin/Choir "$bundle/Contents/MacOS/Choir"
  cp build/darwin/Info.plist "$bundle/Contents/Info.plist"
  cp build/darwin/icons.icns "$bundle/Contents/Resources/icons.icns"

  codesign --force --sign - --entitlements build/darwin/entitlements.plist "$bundle"
  codesign --verify --strict "$bundle"
  echo "Built $bundle ($version, $commit, ad-hoc signed)"
}

case "$cmd" in
  build)
    build
    ;;
  run)
    build
    exec "$bundle/Contents/MacOS/Choir"
    ;;
  test)
    clean_env
    cd "$here"
    go vet ./...
    go test -ldflags "-extldflags=-mmacosx-version-min=$min_macos" ./...
    ;;
  icon)
    cd "$here/build/darwin"
    rm -rf icon.iconset && mkdir icon.iconset
    for size in 16 32 128 256 512; do
      sips -s format png -z "$size" "$size" appicon.png --out "icon.iconset/icon_${size}x${size}.png" >/dev/null
      double=$((size * 2))
      sips -s format png -z "$double" "$double" appicon.png --out "icon.iconset/icon_${size}x${size}@2x.png" >/dev/null
    done
    iconutil -c icns icon.iconset -o icons.icns
    echo "Generated build/darwin/icons.icns"
    ;;
  clean)
    rm -rf "$here/bin"
    ;;
  *)
    echo "usage: $0 [build|run|test|icon|clean]" >&2
    exit 2
    ;;
esac
