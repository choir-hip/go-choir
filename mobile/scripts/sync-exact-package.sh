#!/bin/sh
# Copy AutoradioKit's sources into the Exact package (Exact compiles a package's Swift files into the
# app's module) and write exact-package/apple/exact.json listing them. The copy is gitignored.
#
# Also stage the on-device models into the app's own module: Sources/AutoradioModels/*.swift become
# AutoradioLab/modules/apple/Models_*.swift (importing the package module instead of AutoradioKit), and any static
# xcframeworks built by scripts/build-xcframeworks.sh are copied beside them (Exact links modules/apple/*.xcframework).
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
KIT="$HERE/AutoradioKit/Sources/AutoradioKit"
MODELS="$HERE/AutoradioKit/Sources/AutoradioModels"
OUT="$HERE/exact-package/apple"
APPMOD="$HERE/AutoradioLab/modules/apple"
rm -rf "$OUT/Kit"; mkdir -p "$OUT/Kit"
find "$KIT" -name '*.swift' -exec cp {} "$OUT/Kit/" \;   # flat: Choir/ and other sub-directories are copied in; names are unique
/usr/bin/python3 -I - "$OUT" <<'EOF'
import json, os, sys
out = sys.argv[1]
files = ["AutoradioModule.swift"] + ["Kit/" + f for f in sorted(os.listdir(os.path.join(out, "Kit"))) if f.endswith(".swift")]
json.dump({"kind": "implementation", "implements": "@choir/autoradio@^1.0", "platform": "apple", "targets": ["ios"],
           "shapes": {}, "options": {}, "names": [], "views": [], "hatches": [],
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
    sed -e 's/^import AutoradioKit$/import ExactPkg_autoradio/' "$f" > "$APPMOD/Models_$(basename "$f")"
  done
  for fw in "$HERE"/.deps/xcframeworks/*.xcframework; do cp -R "$fw" "$APPMOD/"; done
  echo "app module: $(ls "$APPMOD"/Models_*.swift | wc -l | tr -d ' ') model files, $(ls -d "$APPMOD"/*.xcframework | wc -l | tr -d ' ') xcframeworks"
else
  echo "app module: no .deps/xcframeworks/FluidAudio.xcframework (run scripts/build-xcframeworks.sh fluidaudio); building without models"
fi
