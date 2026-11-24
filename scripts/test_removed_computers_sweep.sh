#!/bin/sh
# The operator command refuses what it must before it reaches corpusd: no list,
# a list with apply but no run id or manifest hash, apply's flags on a dry run.
# A fake curl proves nothing was sent in any of those cases.
set -eu
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin"
printf '#!/bin/sh\necho sent >>"%s/sent"\nexit 1\n' "$tmp" >"$tmp/bin/curl"
chmod +x "$tmp/bin/curl"
fail=0
refuses() {
	name=$1
	shift
	if PATH="$tmp/bin:$PATH" "$here/removed-computers-sweep" "$@" >/dev/null 2>&1; then
		echo "FAIL: $name was accepted" >&2
		fail=1
	fi
}
refuses "no action"
refuses "an unknown action" delete --ids computer-a
refuses "a dry run without ids" dry-run
refuses "an apply without ids" apply --run-id ts-20261124T000000Z-aaaaaaaa --manifest-sha256 abc
refuses "an apply without a run id" apply --ids computer-a --manifest-sha256 abc
refuses "an apply without a manifest hash" apply --ids computer-a --run-id ts-20261124T000000Z-aaaaaaaa
refuses "a dry run with an apply flag" dry-run --ids computer-a --run-id ts-20261124T000000Z-aaaaaaaa
refuses "an unknown flag" dry-run --ids computer-a --everything
if [ -e "$tmp/sent" ]; then
	echo "FAIL: a refused command reached curl" >&2
	fail=1
fi
# A dry run with ids does reach curl, and sends the named list.
PATH="$tmp/bin:$PATH" "$here/removed-computers-sweep" dry-run --ids computer-a,computer-b >/dev/null 2>&1 || true
[ -e "$tmp/sent" ] || { echo "FAIL: a valid dry run never reached curl" >&2; fail=1; }
[ "$fail" = 0 ] && echo "ok: removed-computers-sweep"
exit "$fail"
