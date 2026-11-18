#!/bin/sh
# The one build tree on this Mac (CLAUDE.md, "Build output lives in one place"). Exact, the model libraries and the app's
# compiled output exist once, there. Any other tree links to that copy and never builds the app.
#   build-tree.sh path      print the build tree's mobile directory, or fail saying how to name one
#   build-tree.sh is        exit 0 when this tree is the build tree
#   build-tree.sh set       name this tree the build tree (the lead does this once per Mac)
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd -P)"
MARK="${AUTORADIO_BUILD_TREE_FILE:-$HOME/.cache/autoradio-build-tree}"
case "${1:-path}" in
  set) mkdir -p "$(dirname "$MARK")"; printf '%s\n' "$HERE" > "$MARK"; echo "build tree: $HERE" ;;
  path|is)
    [ -s "$MARK" ] || { echo "build-tree: no build tree is named on this Mac. In the lead's build tree run: mobile/scripts/build-tree.sh set" >&2; exit 1; }
    T="$(tr -d '\n' < "$MARK")"
    [ -d "$T" ] || { echo "build-tree: $MARK names $T, which does not exist. In the lead's build tree run: mobile/scripts/build-tree.sh set" >&2; exit 1; }
    if [ "${1:-path}" = is ]; then [ "$T" = "$HERE" ]; else printf '%s\n' "$T"; fi ;;
  *) echo "usage: build-tree.sh path|is|set" >&2; exit 2 ;;
esac
