#!/bin/sh
# Copy AutoradioKit's sources into the Exact package (Exact compiles a package's Swift files into the
# app's module) and write exact-package/apple/exact.json listing them. The copy is gitignored.
#
# Also stage the on-device models into the app's own module: Sources/AutoradioModels/*.swift become
# AutoradioLab/modules/apple/Models_*.swift (importing the package module instead of AutoradioKit), and any static
# xcframeworks built by scripts/build-xcframeworks.sh are copied beside them (Exact links modules/apple/*.xcframework).
#
# AR_BUILD_KIND (default development; store is archive-store.sh's): the store kind leaves out the developer launch hooks, which a development build keeps:
# EmbedDeviceTrial.swift, EmbedTrialFixture.swift and ModelProofLaunch.swift are not staged, and the two `runFromLaunchArguments()` calls (ModelProof, EmbedDeviceTrial: a launch
# argument runs a measurement and writes into Documents) are dropped from the staged ModelHub copy. The names this leaves out of a store binary are listed in
# store-dev-names.txt, which check-store-ipa.py holds the executable of an ipa to, and which store-build.test.ts keeps true. Exact defines EXACT_DEVELOPMENT only for package Swift, not for the
# app module these files are staged into, so the choice is made here, by file, at build time: a build kind like debug symbols, not a setting the app reads.
set -eu
KIND="${AR_BUILD_KIND:-development}"
case "$KIND" in development|store) ;; *) echo "sync-exact-package: AR_BUILD_KIND must be development or store, not $KIND" >&2; exit 1 ;; esac
[ "$KIND" != store ] || [ -z "${ALLOW_NO_MODELS:-}" ] || { echo "sync-exact-package: the store kind never builds without the model libraries: unset ALLOW_NO_MODELS (a build with no voice models is a defect, not a store build)" >&2; exit 1; }
HERE="$(cd "$(dirname "$0")/.." && pwd)"
KIT="$HERE/AutoradioKit/Sources/AutoradioKit"
MODELS="$HERE/AutoradioKit/Sources/AutoradioModels"
OUT="$HERE/exact-package/apple"
APPMOD="$HERE/AutoradioLab/modules/apple"
"$HERE/scripts/gen-build-info.sh"                        # the commit -> AutoradioLab/build-info.gen.ts (About)
rm -rf "$OUT/Kit"; mkdir -p "$OUT/Kit"
find "$KIT" -name '*.swift' -exec cp {} "$OUT/Kit/" \;   # flat: Choir/ and other sub-directories are copied in; names are unique
/usr/bin/python3 -I - "$OUT" <<'EOF'
import json, os, sys
out = sys.argv[1]
files = ["AutoradioModule.swift"] + sorted(f for f in os.listdir(out) if f.startswith("AutoradioModule+") and f.endswith(".swift")) + ["Kit/" + f for f in sorted(os.listdir(os.path.join(out, "Kit"))) if f.endswith(".swift")]
json.dump({"kind": "implementation", "implements": "@choir/autoradio@^1.0", "platform": "apple", "targets": ["ios", "macos"],
           "shapes": {}, "options": {}, "names": [], "views": ["web-page"], "hatches": [],
           "sources": {"swift": files}, "artifacts": [], "class": "AutoradioModule"},
          open(os.path.join(out, "exact.json"), "w"), indent=2)
print("exact-package/apple/exact.json:", len(files), "swift files")
EOF

# The models: only when the libraries they import exist as xcframeworks (otherwise the app builds without them,
# and the registrar class is simply not found).
mkdir -p "$APPMOD"
rm -f "$APPMOD"/Models_*.swift; rm -rf "$APPMOD"/*.xcframework
if [ -d "$HERE/.deps/xcframeworks/FluidAudio.xcframework" ]; then
  for f in "$MODELS"/*.swift; do
    b="$(basename "$f")"
    if [ "$KIND" = store ]; then
      case "$b" in EmbedDeviceTrial.swift|EmbedTrialFixture.swift|ModelProofLaunch.swift) continue ;; esac
      if [ "$b" = ModelHub.swift ]; then
        [ "$(grep -c 'runFromLaunchArguments()' "$f")" = 2 ] || { echo "sync-exact-package: ModelHub.swift no longer has exactly the two launch-hook calls the store kind removes; update this script" >&2; exit 1; }
        sed -e 's/^import AutoradioKit$/import ExactPkg_autoradio/' "$f" | grep -v 'runFromLaunchArguments()' > "$APPMOD/Models_$b"
        continue
      fi
    fi
    sed -e 's/^import AutoradioKit$/import ExactPkg_autoradio/' "$f" > "$APPMOD/Models_$b"
  done
  for fw in "$HERE"/.deps/xcframeworks/*.xcframework; do cp -R "$fw" "$APPMOD/"; done
  echo "app module: $(ls "$APPMOD"/Models_*.swift | wc -l | tr -d ' ') model files, $(ls -d "$APPMOD"/*.xcframework | wc -l | tr -d ' ') xcframeworks"
else
  if [ -z "${ALLOW_NO_MODELS:-}" ]; then
    echo "app module: no .deps/xcframeworks/FluidAudio.xcframework. A build without the model libraries has no voice models, only fallbacks (run scripts/ensure-deps.sh, or set ALLOW_NO_MODELS=1 to build one on purpose)" >&2
    exit 1
  fi
  echo "app module: ALLOW_NO_MODELS=1, building without models"
fi
