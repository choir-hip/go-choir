#!/usr/bin/env bash
# Deployed proof for the egress tunnel-identity check (SDC SEC-06,
# docs/problems/egress-proxy-tunnel-identity-2026-10-21.md, "Observe first").
#
# Run INSIDE an implementation (L2) capsule on a test computer, for example
# as a capsule job: bash scripts/capsule-tunnel-identity-probe.sh
#
# Four real toolchains make CONNECT tunnels to their own hosts (each is
# pointed at the public host explicitly, so a package-mirror grant on the
# computer does not route it around the CONNECT path), then one TLS client
# CONNECTs to github.com and sends the server name pypi.org. It holds no
# credential and prints one JSON line per check, then a summary line; it
# exits 1 when a toolchain fails. The verdicts themselves are on the egress
# ledger, outside the capsule (the mismatch line here only says how the
# client saw the tunnel end).
set -u
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

check pip-download python3 -m pip download --no-deps --no-cache-dir --index-url https://pypi.org/simple -d "$work/pip" six==1.16.0
check npm-view npm view --registry https://registry.npmjs.org/ left-pad version
check git-ls-remote git ls-remote https://github.com/golang/go HEAD
check hf-install python3 -m pip install --no-cache-dir --index-url https://pypi.org/simple --target "$work/hf" huggingface_hub
check hf-download env PYTHONPATH="$work/hf" HF_HOME="$work/hf-home" python3 -c \
	'from huggingface_hub import hf_hub_download; print(hf_hub_download("openai-community/gpt2", "config.json"))'

# The mismatch. Under observe the proxy relays it and github.com answers with
# its own certificate, so the client fails verification for pypi.org
# (SSLCertVerificationError); under enforce the proxy closes the tunnel and
# no byte comes back (SSLEOFError or ConnectionResetError). Not a failure
# either way; the ledger entry is the evidence.
mismatch="$(python3 - <<'PY' 2>&1
import socket, ssl
s = socket.create_connection(("127.0.0.1", 3128), timeout=20)
s.sendall(b"CONNECT github.com:443 HTTP/1.1\r\nHost: github.com:443\r\n\r\n")
status = s.recv(200).split(b"\r\n", 1)[0].decode(errors="replace")
try:
    ssl.create_default_context().wrap_socket(s, server_hostname="pypi.org")
    print(status + " | UNEXPECTED: handshake completed")
except Exception as e:
    print(status + " | " + type(e).__name__)
PY
)"
printf '{"check":"sni-mismatch","saw":%s}\n' "$(printf '%s' "$mismatch" | tail -c 300 | jq -Rs .)"

printf '{"summary":"capsule-tunnel-identity-probe","failures":%d}\n' "$fails"
[ "$fails" -eq 0 ]
