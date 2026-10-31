#!/usr/bin/env bash
# Behavioural test for nix/guest-tool-exec-wrapper.sh, the guest tool exec log
# (docs/design/guest-tool-layers-2026-10-31.md 6). It fills the template the
# way nix/autoputer-vm.nix does (sed), with a fake logger, and runs it.
#
#   bash scripts/test-guest-tool-exec-wrapper.sh
#
# Failure modes, written before the wrapper:
#   W1 the log carries user data: arguments, environment or working directory
#   W2 the log floods: more than one line per tool per minute
#   W3 an absent tool layer hangs or fails silently: it must exit 127 at once
#      with a message naming the tool and its layer
#   W4 a present tool behaves differently: arguments, stdin, stdout and the
#      exit status must pass through unchanged
#   W5 a logging failure (no logger, unwritable stamp directory) breaks the
#      tool
set -u
here="$(cd "$(dirname "$0")" && pwd)"
template="$here/../nix/guest-tool-exec-wrapper.sh"
fail=0
pass=0
ok() { pass=$((pass + 1)); echo "ok:   $*"; }
bad() { fail=$((fail + 1)); echo "FAIL: $*"; }

shell="${GUEST_TOOL_WRAPPER_BASH:-$(command -v bash)}"
if ! "$shell" -c 'printf -v x "%(%s)T" -1 && [ -n "$x" ]' 2>/dev/null; then
	echo "skip: $shell has no printf %(...)T (bash 4.2+ needed; the guest's bash has it)"
	exit 0
fi

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/bin" "$root/run"
cat >"$root/logger" <<EOF
#!$shell
printf '%s\n' "\$*" >>"$root/log"
EOF
chmod +x "$root/logger"
cat >"$root/real" <<EOF
#!$shell
printf 'args:%s\n' "\$*"
cat
exit 7
EOF
chmod +x "$root/real"

mkwrap() { # NAME LAYER REAL LOGGER STAMPDIR
	sed -e "s|@bash@|$shell|" -e "s|@logger@|$4|" -e "s|@layer@|$2|g" -e "s|@name@|$1|g" -e "s|@real@|$3|" \
		-e "s|/run/choir-tool-exec/|$5/|" "$template" >"$root/bin/$1"
	chmod +x "$root/bin/$1"
}

mkwrap soffice office "$root/real" "$root/logger" "$root/run"
out="$(cd /tmp && echo stdin-data | SECRET_ENV=hunter2 "$root/bin/soffice" --convert "my secret.docx" 2>"$root/err")"
rc=$?
[ "$rc" = 7 ] && [ "$out" = "args:--convert my secret.docx
stdin-data" ] && ok "W4: a present tool gets its arguments and stdin and returns its status" || bad "W4: rc=$rc out=$out"
line="$(cat "$root/log" 2>/dev/null)"
[ "$line" = "-t choir-tool-exec -- tool=soffice layer=office present=1" ] && ok "W1: the log line is the tool, its layer and presence only" || bad "W1: log line is: $line"
grep -q 'secret\|hunter2\|convert\|/tmp' "$root/log" && bad "W1: the log carries arguments, environment or the working directory"

"$root/bin/soffice" a >/dev/null 2>&1 </dev/null
"$root/bin/soffice" b >/dev/null 2>&1 </dev/null
[ "$(wc -l <"$root/log" | tr -d ' ')" = 1 ] && ok "W2: three calls inside a minute log once" || bad "W2: $(wc -l <"$root/log") lines"
echo 1 >"$root/run/soffice" # a stamp from long ago
"$root/bin/soffice" c >/dev/null 2>&1 </dev/null
[ "$(wc -l <"$root/log" | tr -d ' ')" = 2 ] && ok "W2: a call after the minute logs again" || bad "W2: no second line after the minute"

mkwrap go toolchain "$root/absent/bin/go" "$root/logger" "$root/run"
start=$(date +%s)
"$root/bin/go" build ./... >"$root/out" 2>"$root/err" </dev/null
rc=$?
end=$(date +%s)
[ "$rc" = 127 ] && grep -q 'go is in the toolchain tool layer, which is not attached to this computer' "$root/err" && [ $((end - start)) -lt 5 ] \
	&& ok "W3: an absent tool layer exits 127 at once, naming the tool and the layer" || bad "W3: rc=$rc err=$(cat "$root/err")"
grep -q 'tool=go layer=toolchain present=0' "$root/log" && ok "W3: the absent call is logged with present=0" || bad "W3: absent call not logged: $(cat "$root/log")"

mkwrap cc toolchain "$root/real" "$root/logger" "$root/no/such/dir"
before=$(wc -l <"$root/log")
for _ in 1 2 3; do "$root/bin/cc" x >/dev/null 2>&1 </dev/null; done
[ "$(wc -l <"$root/log")" = "$before" ] && ok "W2: with no writable stamp directory nothing is logged (never a line per call)" || bad "W2: an unwritable stamp directory logged every call"

mkwrap pdf office "$root/real" "$root/no-such-logger" "$root/no/such/dir"
out="$(echo x | "$root/bin/pdf" y 2>"$root/err")"
rc=$?
[ "$rc" = 7 ] && [ "$out" = "args:y
x" ] && [ ! -s "$root/err" ] && ok "W5: no logger and no stamp directory still run the tool, quietly" || bad "W5: rc=$rc out=$out err=$(cat "$root/err")"

echo "guest tool exec wrapper tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
