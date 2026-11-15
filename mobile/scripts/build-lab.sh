#!/bin/sh
# Sync the kit into the Exact package, refresh the app's copy of it (bun copies file: dependencies), then
# run an Exact command in the app:  build-lab.sh ios   |   build-lab.sh ios --device --phone <udid>
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
# The model libraries first: a build without them has no model stages (only fallbacks), which is a defect, not a build.
[ -n "${ALLOW_NO_MODELS:-}" ] || "$HERE/scripts/ensure-deps.sh"
"$HERE/scripts/sync-exact-package.sh"
python3 -I "$HERE/scripts/gen-exact-interface.py" >/dev/null
BUN="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
(cd "$HERE/AutoradioLab" && rm -rf node_modules/@choir && "$BUN" install >/dev/null)
exec "$HERE/scripts/exact.sh" "$@"
