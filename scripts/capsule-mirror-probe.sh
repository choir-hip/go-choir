#!/usr/bin/env bash
# Deployed proof for the capsule package mirror (station FP10,
# docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md).
#
# Run INSIDE an implementation (L2) capsule on a computer the host granted
# (CHOIR_CAPSULE_MIRROR_COMPUTERS), after that computer's cold boot, for
# example as a capsule job: bash scripts/capsule-mirror-probe.sh only npm,pypi,git
#
#   mode        route: mirrors on, CONNECT allowlist unchanged
#               only:  mirrors only; CONNECT to the mirrored registries refused
#   ecosystems  the granted list (comma separated)
#
# It holds no credential and prints one JSON line per check, then a summary
# line; it exits 1 when any check fails. Every network call goes through the
# capsule's own proxy settings; nothing here names an address.
set -u
mode="${1:?usage: capsule-mirror-probe.sh route|only ecosystems}"
ecos=",${2:?usage: capsule-mirror-probe.sh route|only ecosystems},"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0

check() { # name expected(pass|fail) command...
	local name="$1" want="$2"
	shift 2
	local out rc got
	out="$("$@" 2>&1)"
	rc=$?
	got=pass
	[ "$rc" -eq 0 ] || got=fail
	if [ "$got" != "$want" ]; then fails=$((fails + 1)); fi
	printf '{"check":"%s","want":"%s","got":"%s","rc":%d,"tail":%s}\n' \
		"$name" "$want" "$got" "$rc" "$(printf '%s' "$out" | tail -c 300 | jq -Rs .)"
}
has() { case "$ecos" in *",$1,"*) return 0 ;; esac; return 1; }

check healthz pass sh -c 'curl -fsS http://mirror.choir.internal/healthz | jq -e .ok'

if has npm; then
	check npm-env pass sh -c '[ "$npm_config_registry" = "http://mirror.choir.internal/npm/" ]'
	check npm-install pass sh -c "cd '$work' && npm init -y >/dev/null && npm install --ignore-scripts is-number@7.0.0 && node -e 'require(\"is-number\")'"
	if [ "$mode" = only ]; then
		check npm-connect-refused fail curl -fsS -o /dev/null https://registry.npmjs.org/is-number
	fi
else
	check npm-env-absent pass sh -c '[ -z "${npm_config_registry:-}" ]'
fi

if has pypi; then
	check pypi-env pass sh -c '[ "$UV_DEFAULT_INDEX" = "http://mirror.choir.internal/pypi/simple/" ]'
	check pypi-install pass sh -c "uv pip install --no-cache --target '$work/py' six==1.16.0 && test -f '$work/py/six.py'"
	if [ "$mode" = only ]; then
		check pypi-connect-refused fail curl -fsS -o /dev/null https://pypi.org/simple/six/
	fi
else
	check pypi-env-absent pass sh -c '[ -z "${PIP_INDEX_URL:-}" ]'
fi

if has git; then
	check git-clone pass sh -c "git clone -q https://github.com/octocat/Hello-World '$work/hw' && test -f '$work/hw/README'"
	# Named residual: the dumb protocol has no shallow clone.
	check git-shallow-unsupported fail git clone -q --depth 1 https://github.com/octocat/Hello-World "$work/hw-shallow"
	if [ "$mode" = only ]; then
		check git-connect-refused fail curl -fsS -o /dev/null https://github.com/
	fi
fi

# A host no mirror replaces stays reachable in either mode.
check unmirrored-host pass curl -fsS -o /dev/null -I https://huggingface.co/
# The mirror refuses an upload.
check mirror-post-refused fail curl -fsS -o /dev/null -X POST --data x http://mirror.choir.internal/npm/is-number

printf '{"summary":"capsule-mirror-probe","mode":"%s","ecosystems":"%s","failures":%d}\n' "$mode" "${2}" "$fails"
[ "$fails" -eq 0 ]
