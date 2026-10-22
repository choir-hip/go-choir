#!/usr/bin/env bash
# Deployed proof for the egress tunnel-identity check (SDC SEC-06,
# docs/problems/egress-proxy-tunnel-identity-2026-10-21.md, "Observe first"
# and "Review round").
#
# Run INSIDE an implementation (L2) capsule, for example as a capsule job:
#   bash scripts/capsule-tunnel-identity-probe.sh observe   on an observe computer
#   bash scripts/capsule-tunnel-identity-probe.sh enforce   on the computer vmctl's
#                                                           VM_CAPSULE_TUNNEL_ENFORCE_ONLY names
#
# Precondition: a computer with no package-mirror grant, or one whose grant is
# not mirrors_only. Under mirrors_only the proxy refuses CONNECT to pypi.org,
# registry.npmjs.org and github.com outright (403), so no tunnel is judged;
# the script then fails rather than report a verdict it never measured.
#
# Four real toolchains make CONNECT tunnels to their own hosts. Each is
# pointed at the public host so a mirror grant does not route it around the
# CONNECT path: pip and npm by an explicit index or registry, git by dropping
# the mirror's GIT_CONFIG_* url.insteadOf rewrite for that one command
# (internal/capsule/mirror.go; `git -c` cannot cancel an insteadOf).
# huggingface_hub has no mirror. Then one TLS client CONNECTs to github.com
# and sends the server name pypi.org. It holds no credential and prints one
# JSON line per check, then a summary line; it exits 1 when any check fails.
# The verdicts themselves are on the egress ledger, outside the capsule.
set -u
mode="${1:?usage: capsule-tunnel-identity-probe.sh observe|enforce}"
case "$mode" in observe | enforce) ;; *) echo "mode must be observe or enforce" >&2; exit 2 ;; esac
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0

check() { # name command...
	local name="$1"
	shift
	local out rc got
	out="$("$@" 2>&1)"
	rc=$?
	got=pass
	if [ "$rc" -ne 0 ]; then got=fail; fails=$((fails + 1)); fi
	printf '{"check":"%s","got":"%s","rc":%d,"tail":%s}\n' \
		"$name" "$got" "$rc" "$(printf '%s' "$out" | tail -c 300 | jq -Rs .)"
}

# git without the mirror's insteadOf rewrite (GIT_CONFIG_COUNT and its
# GIT_CONFIG_KEY_n / GIT_CONFIG_VALUE_n pairs).
git_public() {
	local unset=(-u GIT_CONFIG_COUNT) v
	for v in $(compgen -v GIT_CONFIG_KEY_) $(compgen -v GIT_CONFIG_VALUE_); do unset+=(-u "$v"); done
	env "${unset[@]}" git "$@"
}

check pip-download python3 -m pip download --no-deps --no-cache-dir --index-url https://pypi.org/simple -d "$work/pip" six==1.16.0
check npm-view npm view --registry https://registry.npmjs.org/ left-pad version
check git-ls-remote git_public ls-remote https://github.com/golang/go HEAD
check hf-install python3 -m pip install --no-cache-dir --index-url https://pypi.org/simple --target "$work/hf" huggingface_hub
check hf-download env PYTHONPATH="$work/hf" HF_HOME="$work/hf-home" python3 -c \
	'from huggingface_hub import hf_hub_download; print(hf_hub_download("openai-community/gpt2", "config.json"))'

# The mismatch. The CONNECT must be answered 200, or nothing was judged.
# Observe relays it and github.com answers with its own certificate, so the
# client fails verification for pypi.org (SSLCertVerificationError). Enforce
# closes the tunnel and no byte comes back (SSLEOFError, SSLZeroReturnError,
# ConnectionResetError or BrokenPipeError).
mismatch="$(python3 - <<'PY' 2>&1
import socket, ssl
s = socket.create_connection(("127.0.0.1", 3128), timeout=20)
s.sendall(b"CONNECT github.com:443 HTTP/1.1\r\nHost: github.com:443\r\n\r\n")
status = s.recv(200).split(b"\r\n", 1)[0].decode(errors="replace")
try:
    ssl.create_default_context().wrap_socket(s, server_hostname="pypi.org")
    print(status + " | handshake completed")
except Exception as e:
    print(status + " | " + type(e).__name__)
PY
)"
got=pass
case "$mismatch" in
"HTTP/1.1 200 "*) ;;
*) got=fail ;;
esac
if [ "$got" = pass ]; then
	case "$mode:$mismatch" in
	observe:*"| SSLCertVerificationError") ;;
	enforce:*"| SSLEOFError" | enforce:*"| SSLZeroReturnError" | enforce:*"| ConnectionResetError" | enforce:*"| BrokenPipeError") ;;
	*) got=fail ;;
	esac
fi
[ "$got" = pass ] || fails=$((fails + 1))
printf '{"check":"sni-mismatch","mode":"%s","got":"%s","saw":%s}\n' "$mode" "$got" "$(printf '%s' "$mismatch" | tail -c 300 | jq -Rs .)"

printf '{"summary":"capsule-tunnel-identity-probe","mode":"%s","failures":%d}\n' "$mode" "$fails"
[ "$fails" -eq 0 ]
