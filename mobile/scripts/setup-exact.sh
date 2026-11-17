#!/bin/sh
# Check out the pinned Exact source at mobile/.exact-src (gitignored) and install Bun 1.4.2 at ~/.bun-1.4.2.
# EXACT_REPO overrides the clone source (a local checkout is fine). No full `exact.mjs setup`.
set -eu
PIN=de212e671a69f4fea18bed67d5b73b259ddf3061
HERE="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$HERE/.exact-src"
REPO="${EXACT_REPO:-https://github.com/expo/exact.git}"
if [ ! -d "$SRC/.git" ]; then git clone -q "$REPO" "$SRC"; fi
if [ "$(git -C "$SRC" rev-parse HEAD)" != "$PIN" ]; then
  git -C "$SRC" fetch -q origin 2>/dev/null || true
  git -C "$SRC" checkout -q "$PIN"
fi
# Local patches (mobile/exact-patches/README.md): applied in order; an applied one is skipped; one that does not apply stops here.
for P in "$HERE"/exact-patches/*.patch; do
  [ -f "$P" ] || continue
  if git -C "$SRC" apply --check "$P" 2>/dev/null; then git -C "$SRC" apply "$P"; echo "patched: $(basename "$P")"
  elif git -C "$SRC" apply --check -R "$P" 2>/dev/null; then :
  else echo "patch does not apply to the pin: $P" >&2; exit 1; fi
done
BUNDIR="$HOME/.bun-1.4.2"
if [ ! -x "$BUNDIR/bin/bun" ]; then
  mkdir -p "$BUNDIR"
  BUN_INSTALL="$BUNDIR" curl -fsSL https://bun.sh/install | BUN_INSTALL="$BUNDIR" bash -s "bun-v1.4.2"
fi
# Exact's own build dependencies (every build's type check imports rolldown from them): installed from the pin's lockfile when
# missing or when the pin changed (a stamp in node_modules records the pin that was installed).
STAMP="$SRC/node_modules/.choir-pin"
if [ ! -d "$SRC/node_modules" ] || [ "$(cat "$STAMP" 2>/dev/null)" != "$PIN" ]; then
  (cd "$SRC" && "$BUNDIR/bin/bun" install --frozen-lockfile) && echo "$PIN" > "$STAMP"
fi
# Rust targets and Hermes bundles for the platforms we build (iOS simulator, iOS device, this Mac). Both were missing on a fresh
# machine at the de212e671 pin: "--ios needs Rust's aarch64-apple-ios-sim std, which the pinned toolchain lacks" and "Hermes bundle
# ... is not cached ... and offline mode is enabled". Exact's CLI sets HERMES_LEAN_SYS_OFFLINE=1 for every build it spawns
# (scripts/app-cli.mjs), so a build never downloads: the bundles (sha256-pinned by Exact, kept in ~/.cargo/hermes-lean-sys) are installed here.
export PATH="$HOME/.cargo/bin:$PATH"
if command -v rustup >/dev/null 2>&1; then
  CHAN="$(sed -n 's/^channel *= *"\(.*\)"/\1/p' "$SRC/rust-toolchain.toml" | head -1)"
  [ -n "$CHAN" ] || { echo "setup-exact: no channel in $SRC/rust-toolchain.toml" >&2; exit 1; }
  rustup toolchain list | grep -q "^$CHAN-" || rustup toolchain install "$CHAN" --profile minimal
  HAVE="$(rustup target list --toolchain "$CHAN" --installed)"
  for T in aarch64-apple-ios-sim aarch64-apple-ios aarch64-apple-darwin; do
    echo "$HAVE" | grep -qx "$T" || { echo "rust: adding $T to $CHAN"; rustup target add "$T" --toolchain "$CHAN"; }
  done
  INST="$SRC/vendor/ibex/crates/hermes-lean-sys-installer/Cargo.toml"
  TARGETS="--target aarch64-apple-darwin --target aarch64-apple-ios-sim --target aarch64-apple-ios"
  # shellcheck disable=SC2086
  if ! (cd "$SRC" && env -u HERMES_LEAN_SYS_OFFLINE -u CARGO_NET_OFFLINE cargo run -q --manifest-path "$INST" -- --check $TARGETS) >/dev/null 2>&1; then
    echo "hermes: installing the pinned bundles (online, once per pin)"
    (cd "$SRC" && env -u HERMES_LEAN_SYS_OFFLINE -u CARGO_NET_OFFLINE cargo run -q --manifest-path "$INST" -- $TARGETS)
  fi
else
  echo "setup-exact: rustup not found; install Rust (rustup) before building" >&2
fi
echo "exact $(git -C "$SRC" rev-parse --short HEAD), bun $("$BUNDIR/bin/bun" --version)"
