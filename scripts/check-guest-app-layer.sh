#!/usr/bin/env bash
# Refuse a layered guest image whose app layer is over 200 MB, and record how
# many store paths it carries (docs/design/layered-guest-image-2026-10-31.md
# 1.5: "the app layer silently grows"). A path the base probe misses lands in
# the app layer on every deploy; this names the largest and fails.
#
#   scripts/check-guest-app-layer.sh DIR [--record FILE]
#
# DIR is a built `.#guest-image-layered` (or `.#guest-store-layers`): it holds
# storedisk-app.erofs and app-paths (one store path per line). Environment:
#   GUEST_APP_LAYER_MAX_BYTES  limit on the app erofs, default 200000000
#   GUEST_APP_LAYER_MAX_PATHS  limit on the path count, default 200 (measured 25)
# --record writes {"app_bytes","app_paths","max_bytes","max_paths","largest"}
# for the deploy record. Exit 0 within limits, 1 over, 2 unusable input.
set -u

dir="${1:-}"
[ -n "$dir" ] && [ -d "$dir" ] || { echo "usage: $0 DIR [--record FILE]" >&2; exit 2; }
shift
record=""
while [ $# -gt 0 ]; do
	case "$1" in
	--record) record="${2:-}"; shift 2 || exit 2 ;;
	*) echo "unknown argument $1" >&2; exit 2 ;;
	esac
done

max_bytes="${GUEST_APP_LAYER_MAX_BYTES:-200000000}"
max_paths="${GUEST_APP_LAYER_MAX_PATHS:-200}"
for v in "$max_bytes" "$max_paths"; do
	case "$v" in '' | *[!0-9]*) echo "limits must be numbers" >&2; exit 2 ;; esac
done

app="$dir/storedisk-app.erofs"
paths="$dir/app-paths"
[ -e "$app" ] && [ -r "$paths" ] || { echo "check-guest-app-layer: $dir has no storedisk-app.erofs and app-paths" >&2; exit 2; }

# -L: the image output links the erofs from its own store path
app_bytes="$(stat -L -c %s "$app" 2>/dev/null || stat -L -f %z "$app" 2>/dev/null)"
case "$app_bytes" in '' | *[!0-9]*) echo "check-guest-app-layer: cannot size $app" >&2; exit 2 ;; esac
app_paths="$(grep -c . "$paths")"

# largest app paths, by disk use, when the store paths exist on this host
largest=""
while IFS= read -r p; do
	[ -n "$p" ] && [ -e "$p" ] || continue
	printf '%s\t%s\n' "$(du -sk "$p" 2>/dev/null | cut -f1)" "$p"
done <"$paths" | sort -rn | head -5 >"${TMPDIR:-/tmp}/.app-layer-largest.$$" 2>/dev/null
if [ -s "${TMPDIR:-/tmp}/.app-layer-largest.$$" ]; then
	largest="$(awk -F'\t' '{printf "%s%s (%s KiB)", sep, $2, $1; sep=", "}' "${TMPDIR:-/tmp}/.app-layer-largest.$$")"
fi
rm -f "${TMPDIR:-/tmp}/.app-layer-largest.$$"

echo "check-guest-app-layer: app layer $app_bytes bytes (limit $max_bytes), $app_paths paths (limit $max_paths)"
[ -n "$largest" ] && echo "check-guest-app-layer: largest app paths: $largest"

if [ -n "$record" ]; then
	printf '{"app_bytes":%s,"app_paths":%s,"max_bytes":%s,"max_paths":%s,"largest":"%s"}\n' \
		"$app_bytes" "$app_paths" "$max_bytes" "$max_paths" "$(printf '%s' "$largest" | sed 's/"/\\"/g')" >"$record" \
		|| { echo "check-guest-app-layer: cannot write $record" >&2; exit 2; }
fi

status=0
if [ "$app_bytes" -gt "$max_bytes" ]; then
	echo "FAIL: the app layer is $app_bytes bytes, over $max_bytes. A path the base probe misses lands here on every deploy; move it into the base (nix/guest-layered-store.nix, guestLayerSlowPackages in flake.nix)." >&2
	[ -z "$largest" ] && echo "FAIL: the store paths are not on this host; list them with: sort -u $paths | xargs du -sk | sort -rn | head" >&2
	status=1
fi
if [ "$app_paths" -gt "$max_paths" ]; then
	echo "FAIL: the app layer carries $app_paths store paths, over $max_paths (measured 25)." >&2
	status=1
fi
exit "$status"
