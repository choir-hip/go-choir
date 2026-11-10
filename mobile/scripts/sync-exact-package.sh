#!/bin/sh
# Copy AutoradioKit's sources into the Exact package (Exact compiles a package's Swift files into the
# app's module) and write exact-package/apple/exact.json listing them. The copy is gitignored.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
KIT="$HERE/AutoradioKit/Sources/AutoradioKit"
OUT="$HERE/exact-package/apple"
rm -rf "$OUT/Kit"; mkdir -p "$OUT/Kit"
cp "$KIT"/*.swift "$OUT/Kit/"
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
