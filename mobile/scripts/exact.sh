#!/bin/sh
# Run an Exact command for AutoradioLab outside the Nix dev shell, e.g.  mobile/scripts/exact.sh contract build app.contract
# BUN names a Bun 1.4.2 binary (default ~/.bun-1.4.2/bin/bun, installed by setup-exact.sh).
HERE="$(cd "$(dirname "$0")/.." && pwd)"
BUN="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
cd "$HERE/AutoradioLab" || exit 1
exec env -i HOME="$HOME" USER="$USER" TMPDIR="${TMPDIR:-/tmp}" BUN="$BUN" \
  PATH="$(dirname "$BUN"):$HOME/.cargo/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin" \
  DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer EXACT2="$HERE/.exact-src" EXACT_FEEDBACK=0 \
  "$BUN" exact.mjs "$@"
