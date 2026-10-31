#!/usr/bin/env bash
# Tests for scripts/check-guest-app-layer.sh: within limits passes and records
# the path count; an oversize layer, too many paths, and unusable input fail.
set -u
here="$(cd "$(dirname "$0")" && pwd)"
check="$here/check-guest-app-layer.sh"
fail=0
root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
ok() { echo "ok:   $*"; }
bad() { fail=$((fail + 1)); echo "FAIL: $*"; }

mk() { # NAME BYTES PATHS
	mkdir -p "$root/$1"
	head -c "$2" /dev/zero >"$root/$1/storedisk-app.erofs"
	: >"$root/$1/app-paths"
	for i in $(seq 1 "$3"); do echo "/nix/store/p$i-x" >>"$root/$1/app-paths"; done
}

mk good 1000000 25
bash "$check" "$root/good" --record "$root/rec.json" >/dev/null 2>&1 && ok "within limits passes" || bad "within limits failed"
jq -e '.app_paths == 25 and .app_bytes == 1000000' "$root/rec.json" >/dev/null && ok "records the path count and bytes" || bad "record is wrong"

mk big 300000 25
GUEST_APP_LAYER_MAX_BYTES=200000 bash "$check" "$root/big" >/dev/null 2>&1 && bad "an oversize app layer passed" || ok "an oversize app layer is refused"

mk many 1000 30
GUEST_APP_LAYER_MAX_PATHS=10 bash "$check" "$root/many" >/dev/null 2>&1 && bad "too many paths passed" || ok "too many paths are refused"

mkdir "$root/empty"
bash "$check" "$root/empty" >/dev/null 2>&1; [ $? -eq 2 ] && ok "a directory without the layer is unusable (exit 2)" || bad "missing input did not exit 2"

GUEST_APP_LAYER_MAX_BYTES=abc bash "$check" "$root/good" >/dev/null 2>&1; [ $? -eq 2 ] && ok "a non-numeric limit is unusable" || bad "bad limit accepted"

ln -s "$root/good/storedisk-app.erofs" "$root/good/link.erofs"
mkdir "$root/linked"; ln -s "$root/good/storedisk-app.erofs" "$root/linked/storedisk-app.erofs"; cp "$root/good/app-paths" "$root/linked/"
bash "$check" "$root/linked" >/dev/null 2>&1 && ok "a symlinked app layer (the flake output) is sized through the link" || bad "symlinked layer failed"

echo "check-guest-app-layer tests: $fail failed"
[ "$fail" -eq 0 ]
