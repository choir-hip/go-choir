#!/usr/bin/env bash
# Print the buildGoModule vendorHash that flake.nix needs for the current tree.
#
# Why this exists: the nix Go build vendors with `go mod vendor` inside a
# fixed-output derivation, so a go.mod/go.sum (or imported-package) change
# changes the hash. The hash does not depend on the CPU architecture, so it can
# be computed on any machine with nix and go, without a Linux builder: this
# script takes the exact filtered source the flake builds from (goServicesSrc),
# runs `go mod vendor` on it, and NAR-hashes vendor/ the way the derivation's
# output is hashed. Verified 2026-10-30: on the pre-cutover commit it reproduced
# the committed hash exactly.
#
# Usage: scripts/cutover/vendor-hash.sh [repo-dir]    (default: this repo)
# Then put the printed value in flake.nix (commonGoArgs.vendorHash). Commit or
# `git add` new files first: a flake sees only tracked files.
set -euo pipefail

repo="${1:-$(cd "$(dirname "$0")/../.." && pwd)}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cd "$repo"

drv="$(nix eval --no-write-lock-file --raw '.#packages.x86_64-linux.corpusd.drvPath' 2>/dev/null)"
nix derivation show "$drv" >"$work/corpusd.json" 2>/dev/null
svc="$(python3 -I -c "
import json
d = json.load(open('$work/corpusd.json'))
v = list(d['derivations'].values())[0]
print([k for k in v['inputs']['drvs'] if 'go-choir-services' in k][0])")"
nix derivation show "/nix/store/$svc" >"$work/svc.json" 2>/dev/null
src="$(python3 -I -c "
import json
d = json.load(open('$work/svc.json'))
v = list(d['derivations'].values())[0]
print(v['env']['src'])")"
current="$(python3 -I -c "
import json
d = json.load(open('$work/svc.json'))
v = list(d['derivations'].values())[0]
print(v['env'].get('vendorHash'))")"

cp -R "$src" "$work/src"
chmod -R u+w "$work/src"
cd "$work/src"
unset GOROOT
GOFLAGS=-trimpath GOPROXY="${GOPROXY:-https://proxy.golang.org}" GOSUMDB="${GOSUMDB:-sum.golang.org}" go mod vendor
new="$(nix hash path --sri vendor)"
echo "flake.nix vendorHash currently: $current" >&2
if [ "$new" = "$current" ]; then
  echo "unchanged" >&2
fi
echo "$new"
