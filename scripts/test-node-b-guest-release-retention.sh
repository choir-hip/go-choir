#!/usr/bin/env bash
# Behavioural tests for scripts/node-b-guest-release-materialize and
# scripts/node-b-guest-release-retention (D2 design S2, sections 4.1 and 4.6).
# They drive the real scripts against a fixture tree and a stub vmctl, so they
# need no root, no Nix and no host, and cannot touch /var/lib/go-choir.
#
#   bash scripts/test-node-b-guest-release-retention.sh
#
# Failure modes (design section 8), written before the scripts:
#   M1  retention deletes a running, current, previous or young release by any
#       rule (the support-window and cap rules included)
#   M4  releases pile up past the cap
#   M22 materializing a release fails the NixOS switch, leaves a .tmp-* tree,
#       or leaves a half-made release directory
#   M24 an unreadable vault root (complete:false) lets retention delete a
#       release a vault snapshot needs, or a running one
#   -   an unanswered vmctl lets retention delete anything
#   -   a symlink or an odd name is deleted
set -u
here="$(cd "$(dirname "$0")" && pwd)"
materialize="$here/node-b-guest-release-materialize"
retention="$here/node-b-guest-release-retention"
fail=0
pass=0
ok() { pass=$((pass + 1)); echo "ok:   $*"; }
bad() { fail=$((fail + 1)); echo "FAIL: $*"; }

root="$(mktemp -d)"
server_pid=""
trap '[ -n "$server_pid" ] && kill "$server_pid" 2>/dev/null; chmod -R u+w "$root" 2>/dev/null; rm -rf "$root"' EXIT

NOW=1800000000 # fixed clock for retention
hours() { echo $((NOW - $1 * 3600)); }
iso() { jq -rn --argjson t "$1" '$t | todate'; }

# ids use letters/digits valid in nix base32 (no e, o, u, t): a b c d f g h i j k l m n p q r s v w x y z, digits
A=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa; B=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb; C=cccccccccccccccccccccccccccccccc
D=dddddddddddddddddddddddddddddddd; F=ffffffffffffffffffffffffffffffff; G=gggggggggggggggggggggggggggggggg
H=hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh; I=iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii
CUR=$A; YOUNG=$B; MID=$C; OLD=$D; OOS=$F; OOSRUN=$G; USED=$H

# --- fixture ------------------------------------------------------------------
mkrelease() { # RELDIR ID HOURS_AGO
	local d="$1/$2"
	mkdir -p "$d"
	for f in vmlinux initrd rootfs.ext4 storedisk.erofs kernel-params build.json guest-image-manifest release.json; do echo "$f" >"$d/$f"; done
	printf '{"store_path":"/nix/store/%s-go-choir-guest-image","first_current_at":"%s"}\n' "$2" "$(iso "$(hours "$3")")" >"$d/installed.json"
}

fixture() { # DIR: the standard tree
	local rel="$1/rel"
	rm -rf "$1"
	mkdir -p "$rel"
	mkrelease "$rel" $CUR 10
	mkrelease "$rel" $YOUNG 1
	mkrelease "$rel" $MID 5
	mkrelease "$rel" $OLD 20
	mkrelease "$rel" $OOS 200
	mkrelease "$rel" $OOSRUN 200
	mkrelease "$rel" $USED 30
	ln -s "$rel/$CUR" "$rel/iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"           # a symlink with a release name
	mkdir -p "$rel/not-a-release" "$rel/.tmp-leftover"                  # odd names
	ln -s "/nix/store/${CUR}-go-choir-guest-image" "$1/guest"           # the guest link
}

# --- stub vmctl -----------------------------------------------------------------
cat >"$root/stub.py" <<'PY'
import http.server, sys
status = int(sys.argv[2]); body = open(sys.argv[3], "rb").read() if len(sys.argv) > 3 else b""
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(status); self.send_header("Content-Type", "application/json"); self.end_headers(); self.wfile.write(body)
    def log_message(self, *a): pass
s = http.server.HTTPServer(("127.0.0.1", 0), H)
open(sys.argv[1], "w").write(str(s.server_address[1])); s.serve_forever()
PY
start_stub() { # STATUS BODYFILE -> sets URL
	rm -f "$root/port"
	python3 -I "$root/stub.py" "$root/port" "$1" "${2:-/dev/null}" &
	server_pid=$!
	for _ in $(seq 1 50); do [ -s "$root/port" ] && break; sleep 0.1; done
	URL="http://127.0.0.1:$(cat "$root/port")"
}
stop_stub() { [ -n "$server_pid" ] && kill "$server_pid" 2>/dev/null; wait "$server_pid" 2>/dev/null; server_pid=""; }

answer() { # COMPLETE -> body file
	jq -n --arg cur "$CUR" --argjson complete "$1" --arg oosrun "$OOSRUN" --arg used "$USED" --arg mid "$MID" \
		'{retain:true,current:$cur,complete:$complete,in_use:[{id:$oosrun,running:["vm-a"],snapshots:[]},{id:$used,running:[],snapshots:["vm-b"]},{id:$mid,running:[],snapshots:["vm-c"]}]}' >"$root/answer.json"
}

run_retention() { # DIR URL [extra env...]
	local d="$1" url="$2"
	shift 2
	env GO_CHOIR_GUEST_RELEASES_DIR="$d/rel" GO_CHOIR_GUEST_LINK="$d/guest" GO_CHOIR_VMCTL_URL="$url" GO_CHOIR_NOW_EPOCH=$NOW "$@" bash "$retention" >"$d/out.log" 2>&1
}
exists() { [ -d "$1/rel/$2" ] && [ ! -L "$1/rel/$2" ]; }
expect_kept() { # DIR NAME ID...
	local d="$1" name="$2"
	shift 2
	for id in "$@"; do exists "$d" "$id" || { bad "$name: $id was deleted"; return; }; done
}
expect_gone() {
	local d="$1" name="$2"
	shift 2
	for id in "$@"; do exists "$d" "$id" && { bad "$name: $id was kept"; return; }; done
}
untouched() { # DIR NAME: symlink and odd names survive
	[ -L "$1/rel/iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii" ] && [ -d "$1/rel/not-a-release" ] && [ -d "$1/rel/.tmp-leftover" ] || bad "$2: a symlink or an odd name was deleted"
}

# --- retention ------------------------------------------------------------------
d="$root/t1"; fixture "$d"; answer true; start_stub 200 "$root/answer.json"
run_retention "$d" "$URL"; rc=$?
[ $rc -eq 0 ] || bad "200: exit $rc"
expect_kept "$d" "200" $CUR $YOUNG $MID $USED $OOSRUN
expect_gone "$d" "200" $OLD $OOS
untouched "$d" "200"
[ $fail -eq 0 ] && ok "200: only the old unused and the out-of-support unused releases go; current, previous, young, in-use and running stay"
stop_stub

f0=$fail
d="$root/t2"; fixture "$d"; answer false; start_stub 200 "$root/answer.json"
run_retention "$d" "$URL"
expect_kept "$d" "complete:false" $CUR $YOUNG $MID $USED $OOSRUN $OLD
expect_gone "$d" "complete:false" $OOS
untouched "$d" "complete:false"
[ $fail -eq $f0 ] && ok "complete:false: only past-window releases that are not running are deleted"
stop_stub

f0=$fail
d="$root/t3"; fixture "$d"
run_retention "$d" "http://127.0.0.1:9"   # nothing listens on the discard port
expect_kept "$d" "no answer" $CUR $YOUNG $MID $USED $OOSRUN $OLD $OOS
untouched "$d" "no answer"
[ $fail -eq $f0 ] && ok "connection refused: nothing is deleted (the script cannot know what runs)"

f0=$fail
d="$root/t4"; fixture "$d"; start_stub 503 ""
run_retention "$d" "$URL"
expect_kept "$d" "5xx" $CUR $YOUNG $MID $USED $OOSRUN $OLD $OOS
[ $fail -eq $f0 ] && ok "5xx: nothing is deleted"
stop_stub

f0=$fail
d="$root/t5"; fixture "$d"; start_stub 404 ""
run_retention "$d" "$URL"
expect_kept "$d" "404" $CUR $YOUNG
expect_gone "$d" "404" $MID $OLD $OOS $USED $OOSRUN
untouched "$d" "404"
[ $fail -eq $f0 ] && ok "404 (an older vmctl): in_use is empty; current and previous stay, the rest go"
stop_stub

f0=$fail
d="$root/t6"; fixture "$d"; answer true; start_stub 200 "$root/answer.json"
run_retention "$d" "$URL" GO_CHOIR_GUEST_RELEASE_KEEP_MAX=4
# after the window rule: cur, young, mid, used, oosrun remain (5); cap 4 deletes snapshot-only releases, oldest first: used (30h)
expect_kept "$d" "cap" $CUR $YOUNG $OOSRUN $MID
expect_gone "$d" "cap" $USED $OLD $OOS
[ $fail -eq $f0 ] && ok "the cap: the oldest snapshot-only release goes first"
stop_stub

f0=$fail
d="$root/t7"; fixture "$d"; answer true; start_stub 200 "$root/answer.json"
run_retention "$d" "$URL" GO_CHOIR_GUEST_RELEASE_KEEP_MAX=1
expect_kept "$d" "cap 1" $CUR $YOUNG $OOSRUN
expect_gone "$d" "cap 1" $USED $MID
[ $fail -eq $f0 ] && ok "cap 1: current, previous and a running release are never deleted, even past the window and the cap"
stop_stub

f0=$fail
d="$root/t8"; fixture "$d"; answer true; start_stub 200 "$root/answer.json"
# a young release that nothing uses: a launch that resolved just before a switch
mkrelease "$d/rel" kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk 0
run_retention "$d" "$URL"
expect_kept "$d" "young" kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
[ $fail -eq $f0 ] && ok "a release under 2 hours old is never pruned"
stop_stub

f0=$fail
d="$root/t9"; fixture "$d"; rm -f "$d/guest"
run_retention "$d" "http://127.0.0.1:9"
expect_kept "$d" "no link" $CUR $YOUNG $MID $USED $OOSRUN $OLD $OOS
[ $fail -eq $f0 ] && ok "an unreadable guest link deletes nothing"

# --- materialize ------------------------------------------------------------------
mksrc() { # DIR -> src path
	local src="$1/src/${A}-go-choir-guest-image"
	mkdir -p "$src"
	for f in vmlinux initrd rootfs.ext4 storedisk.erofs kernel-params build.json guest-image-manifest release.json; do echo "$f" >"$src/$f"; done
	echo "$src"
}
run_mat() { # SRC REL
	env GO_CHOIR_RELEASE_REFLINK=auto bash "$materialize" "$1" "$2" >"$root/mat.log" 2>&1
}

f0=$fail
d="$root/m1"; mkdir -p "$d"; src="$(mksrc "$d")"
run_mat "$src" "$d/rel"; rc=$?
[ $rc -eq 0 ] || bad "materialize: exit $rc"
rd="$d/rel/$A"
for f in vmlinux initrd rootfs.ext4 storedisk.erofs kernel-params build.json guest-image-manifest release.json installed.json; do [ -f "$rd/$f" ] || bad "materialize: $f missing"; done
[ "$(stat -c %a "$rd/vmlinux")" = 444 ] && [ "$(stat -c %a "$rd")" = 555 ] || bad "materialize: modes are $(stat -c %a "$rd/vmlinux") / $(stat -c %a "$rd"), want 444 / 555"
jq -e --arg s "$src" '.store_path == $s and (.first_current_at | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]+Z$"))' "$rd/installed.json" >/dev/null || bad "materialize: installed.json is wrong"
ls -A "$d/rel" | grep -q '^\.tmp-' && bad "materialize: a .tmp tree is left"
[ $fail -eq $f0 ] && ok "materialize: a release directory with every file, 0444 files, 0555 directory, installed.json"
before=$(stat -c %Y "$rd/installed.json"); sleep 1
run_mat "$src" "$d/rel"; rc=$?
[ $rc -eq 0 ] && [ "$(stat -c %Y "$rd/installed.json")" = "$before" ] && ok "materialize: an existing release is left alone (first_current_at is written once)" || bad "materialize: a second run changed the release (exit $rc)"

f0=$fail
d="$root/m2"; mkdir -p "$d"; src="$(mksrc "$d")"; rm "$src/storedisk.erofs"
run_mat "$src" "$d/rel"; rc=$?
[ $rc -eq 0 ] || bad "missing source file: exit $rc, want 0 (never fail the switch)"
[ -e "$d/rel/$A" ] && bad "missing source file: a release directory exists"
ls -A "$d/rel" 2>/dev/null | grep -q '^\.tmp-' && bad "missing source file: a .tmp tree is left"
grep -q "not materialized" "$root/mat.log" || bad "missing source file: the reason is not logged"
[ $fail -eq $f0 ] && ok "materialize: a source missing a file exits 0, leaves nothing, logs the reason"

f0=$fail
d="$root/m3"; mkdir -p "$d/rel"; src="$(mksrc "$d")"; chmod 0555 "$d/rel"
run_mat "$src" "$d/rel"; rc=$?
[ $rc -eq 0 ] || bad "read-only releases dir: exit $rc, want 0"
[ -e "$d/rel/$A" ] && bad "read-only releases dir: a release directory exists"
ls -A "$d/rel" | grep -q '^\.tmp-' && bad "read-only releases dir: a .tmp tree is left"
chmod 0755 "$d/rel"
d="$root/m4"; mkdir -p "$d"; src="$(mksrc "$d")"; : >"$d/rel"   # a file where the directory should be
run_mat "$src" "$d/rel"; rc=$?
[ $rc -eq 0 ] || bad "releases path is a file: exit $rc, want 0"
[ $fail -eq $f0 ] && ok "materialize: a read-only or unusable releases directory exits 0 and leaves nothing"

f0=$fail
d="$root/m5"; mkdir -p "$d"; src="$(mksrc "$d")"
env bash "$materialize" "$src" "$d/rel" >"$root/mat.log" 2>&1; rc=$?   # default: --reflink=always
[ $rc -eq 0 ] || bad "default reflink mode: exit $rc, want 0"
if [ -d "$d/rel/$A" ]; then ok "materialize: the default mode reflinked (a reflink filesystem)"; else
	ls -A "$d/rel" 2>/dev/null | grep -q '^\.tmp-' && bad "default reflink mode: a .tmp tree is left"
	[ $fail -eq $f0 ] && ok "materialize: the default mode never falls back to a full copy (no reflink here: nothing made, exit 0)"
fi

# --- wiring in nix/node-b.nix (text assertions) ---------------------------------------
nix="$here/../nix/node-b.nix"
grep -qE '^\s*snapshotReleaseRetain = false;' "$nix" && ok "nix: snapshotReleaseRetain is a let-binding, off" || bad "nix: snapshotReleaseRetain = false; is missing"
grep -q '"/var/lib/go-choir/guest-releases"' "$nix" && grep -E 'ReadOnlyPaths' "$nix" | grep -q 'guest-releases' && ok "nix: vmctl reads guest-releases read-only" || bad "nix: guest-releases is not in vmctl's ReadOnlyPaths"
grep -q 'd /var/lib/go-choir/guest-releases 0755 root root -' "$nix" && ok "nix: the tmpfiles rule" || bad "nix: the tmpfiles rule for guest-releases is missing"
grep -q 'system.activationScripts.go-choir-guest-release' "$nix" && grep -q 'deps = \[ "go-choir-guest-release" \]' "$nix" && ok "nix: go-choir-guest-image depends on the release snippet" || bad "nix: the guest-image snippet does not depend on go-choir-guest-release"
grep -q 'systemd.services.go-choir-guest-release-retention' "$nix" && grep -q 'systemd.timers.go-choir-guest-release-retention' "$nix" && ok "nix: the retention oneshot and timer" || bad "nix: the retention unit or timer is missing"
grep -q 'guestReleaseRetention' "$nix" && grep -c 'guestReleaseRetention' "$nix" | grep -qE '^[3-9]' && ok "nix: the disk retention sweep calls it" || bad "nix: diskRetentionSweep does not call the release retention"

echo "node-b-guest-release-retention tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
