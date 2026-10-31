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
#   -   an unanswered vmctl (no answer, 5xx, 404 from a rolled-back vmctl) lets
#       retention delete anything
#   -   a symlink or an odd name is deleted
#   M30 retention deletes a release a running VM needs (vmctl says so, or a
#       live Firecracker's fc-config.json names it), a release a snapshot or a
#       VM's guest-release.json names inside the support window, or the
#       rollback target (the release older than current)
#   M31 retention deletes while a deploy is materializing (a fresh .tmp-* tree)
#   M32 retention drops below 2 releases
#   M33 a stale-jail sweep removes the jail of a VM that has an ownership row
#       (running or hibernated), of a VM a Firecracker still names, or acts on
#       an unreadable ownership file; or the report mode removes anything
#   M34 the dedupe tool fails the deploy (always exit 0), corrupts or loses a
#       release, or misses shared blocks (same offset or shifted)
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
	touch -t 202001010000 "$rel/.tmp-leftover"                          # a stale leftover, not a running materialize
	mkdir -p "$1/state"                                                 # an empty vm-state
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
	env GO_CHOIR_GUEST_RELEASES_DIR="$d/rel" GO_CHOIR_GUEST_LINK="$d/guest" GO_CHOIR_VMCTL_URL="$url" GO_CHOIR_VM_STATE_DIR="$d/state" GO_CHOIR_NOW_EPOCH=$NOW "$@" bash "$retention" >"$d/out.log" 2>&1
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
expect_kept "$d" "200" $CUR $YOUNG $MID $USED $OOSRUN $OLD
expect_gone "$d" "200" $OOS
untouched "$d" "200"
[ $fail -eq 0 ] && ok "200: only the out-of-support unused release goes; current, previous, the rollback target, young, in-use and running stay"
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
expect_kept "$d" "404" $CUR $YOUNG $MID $USED $OOSRUN $OLD $OOS
untouched "$d" "404"
[ $fail -eq $f0 ] && ok "404 deletes nothing: only a parsed 200 is an answer (review 331: a vmctl rollback while release-bound VMs run)"
stop_stub

# no count cap: with the old KEEP_MAX=1 nothing extra is deleted
f0=$fail
d="$root/t6"; fixture "$d"; answer true; start_stub 200 "$root/answer.json"
run_retention "$d" "$URL" GO_CHOIR_GUEST_RELEASE_KEEP_MAX=1
expect_kept "$d" "no cap" $CUR $YOUNG $MID $USED $OOSRUN $OLD
expect_gone "$d" "no cap" $OOS
[ $fail -eq $f0 ] && ok "retention is by reference: KEEP_MAX no longer deletes referenced releases"
stop_stub

# a release no one references and that is not current, previous or the rollback target goes
f0=$fail
d="$root/t7"; fixture "$d"; answer true; start_stub 200 "$root/answer.json"
mkrelease "$d/rel" nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn 40
run_retention "$d" "$URL"
expect_gone "$d" "unreferenced" nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn
# ... unless a VM's guest-release.json (soft) names it
fixture "$d"; mkrelease "$d/rel" nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn 40
mkdir -p "$d/state/vm-x"; echo '{"release":"nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn"}' >"$d/state/vm-x/guest-release.json"
run_retention "$d" "$URL"
expect_kept "$d" "guest-release.json" nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn
# ... and past the support window the soft reference stops counting (its snapshots refuse at wake)
fixture "$d"; mkrelease "$d/rel" nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn 300
mkdir -p "$d/state/vm-x"; echo '{"release":"nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn"}' >"$d/state/vm-x/guest-release.json"
run_retention "$d" "$URL"
expect_gone "$d" "soft ref past window" nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn
# ... but an fc-config.json of a live Firecracker is hard, even past the window; a dead VM's is soft
mkdir -p "$root/bin"
printf '#!/bin/sh\ncase "$*" in *vm-live*) exit 0;; esac\nexit 1\n' >"$root/bin/pgrep"; chmod +x "$root/bin/pgrep"
fixture "$d"; mkrelease "$d/rel" nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn 300; mkrelease "$d/rel" pppppppppppppppppppppppppppppppp 300
mkdir -p "$d/state/vm-live" "$d/state/vm-dead"
echo '{"drives":[{"path_on_host":"'"$d"'/rel/nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn/storedisk.erofs"}]}' | sed "s#$d/rel#/var/lib/go-choir/guest-releases#" >"$d/state/vm-live/fc-config.json"
echo '{"drives":[{"path_on_host":"/var/lib/go-choir/guest-releases/pppppppppppppppppppppppppppppppp/storedisk.erofs"}]}' >"$d/state/vm-dead/fc-config.json"
run_retention "$d" "$URL" PATH="$root/bin:$PATH"
expect_kept "$d" "live fc-config" nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn
expect_gone "$d" "dead fc-config past window" pppppppppppppppppppppppppppppppp
[ $fail -eq $f0 ] && ok "references: unreferenced goes; guest-release.json keeps it in the window; a live VM's fc-config.json keeps it always"
stop_stub

# the rollback target and the minimum of two
f0=$fail
d="$root/t7b"; rm -rf "$d"; mkdir -p "$d/rel" "$d/state"
mkrelease "$d/rel" $CUR 1000; mkrelease "$d/rel" $OLD 2000
ln -s "/nix/store/${CUR}-go-choir-guest-image" "$d/guest"
answer true; start_stub 200 "$root/answer.json"
run_retention "$d" "$URL"
expect_kept "$d" "min 2" $CUR $OLD
[ $fail -eq $f0 ] && ok "two releases, both past the window and unreferenced: both stay (rollback target, hard minimum of 2)"
stop_stub

# a deploy materializing: a fresh .tmp-* tree blocks every deletion
f0=$fail
d="$root/t7c"; fixture "$d"; answer true; start_stub 200 "$root/answer.json"
mkdir -p "$d/rel/.tmp-dddddddddddddddddddddddddddddddd"
run_retention "$d" "$URL"
expect_kept "$d" "materializing" $CUR $YOUNG $MID $USED $OOSRUN $OLD $OOS
grep -q "tmp_active=true" "$d/out.log" || bad "materializing: tmp_active not reported"
[ $fail -eq $f0 ] && ok "a fresh .tmp-* tree (a deploy materializing) deletes nothing; a stale one does not block"
stop_stub

# --- stale jails ----------------------------------------------------------------
f0=$fail
d="$root/j1"; fixture "$d"; answer true; start_stub 200 "$root/answer.json"
jb="$d/state/.jail/firecracker"
mkdir -p "$jb/vm-a/root" "$jb/vm-hib/root" "$jb/vm-orphan/root" "$jb/vm-live/root" "$d/state/.jail/.shared"
echo x >"$jb/vm-orphan/root/store.img"; echo x >"$d/state/.jail/.shared/abc-store.img"
echo '{"ownerships":[{"vm_id":"vm-a","state":"running"},{"vm_id":"vm-hib","state":"hibernated"}]}' >"$d/state/ownerships.json"
run_retention "$d" "$URL" PATH="$root/bin:$PATH"
[ -d "$jb/vm-orphan" ] && grep -q "stale jail vm-orphan" "$d/out.log" || bad "jails report: orphan removed or not reported"
run_retention "$d" "$URL" PATH="$root/bin:$PATH" GO_CHOIR_PRUNE_JAILS=1
[ ! -e "$jb/vm-orphan" ] || bad "jails prune: the orphan jail is still there"
[ -d "$jb/vm-a" ] && [ -d "$jb/vm-hib" ] || bad "jails prune: the jail of a running or hibernated VM was removed"
[ -d "$jb/vm-live" ] && grep -q "Firecracker names it" "$d/out.log" || bad "jails prune: a jail a Firecracker names was removed"
[ -f "$d/state/.jail/.shared/abc-store.img" ] || bad "jails prune: .shared was touched"
mkdir -p "$jb/vm-orphan2"; echo 'not json' >"$d/state/ownerships.json"
run_retention "$d" "$URL" GO_CHOIR_PRUNE_JAILS=1
[ -d "$jb/vm-orphan2" ] || bad "jails prune: removed with an unreadable ownership file"
rm "$d/state/ownerships.json"
env GO_CHOIR_GUEST_RELEASES_DIR="$d/rel" GO_CHOIR_GUEST_LINK="$d/guest" GO_CHOIR_VMCTL_URL="$URL" GO_CHOIR_VM_STATE_DIR="$d/state" GO_CHOIR_NOW_EPOCH=$NOW bash "$retention" --prune-jails >"$d/out.log" 2>&1
[ -d "$jb/vm-orphan2" ] || bad "jails prune: removed with a missing ownership file"
[ $fail -eq $f0 ] && ok "jails: reported by default; --prune-jails removes only row-less, Firecracker-less jails; an unreadable ownership file removes nothing"
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

# --- dedupe -------------------------------------------------------------------------
# GO_CHOIR_DEDUPE_FAKE=1 compares bytes and counts what the kernel would share
# (macOS has no FIDEDUPERANGE); the ioctl itself is exercised on Linux below.
dedupe="$here/node-b-guest-release-dedupe"
mkimg() { # PATH MODE: 64-block image; shift = one extra leading block; edit = 3 blocks rewritten
	python3 -I - "$1" "$2" <<'PY'
import sys, random
random.seed(7)
blocks = [random.randbytes(4096) for _ in range(64)]
if sys.argv[2] in ("edit", "both"):
    for i in (5, 20, 21): blocks[i] = random.randbytes(4096)
if sys.argv[2] in ("shift", "both"):
    blocks = [random.randbytes(4096)] + blocks
open(sys.argv[1], "wb").write(b"".join(blocks))
PY
}
mkdd() { # DIR MODE
	rm -rf "$1"; mkdir -p "$1/rel"
	mkrelease "$1/rel" $OLD 50; mkrelease "$1/rel" $CUR 10
	chmod u+w "$1/rel/$OLD" "$1/rel/$CUR"
	rm -f "$1/rel/$OLD/storedisk.erofs" "$1/rel/$CUR/storedisk.erofs"
	mkimg "$1/rel/$OLD/storedisk.erofs" none
	mkimg "$1/rel/$CUR/storedisk.erofs" "$2"
}
rundd() { env GO_CHOIR_DEDUPE_FAKE=1 GO_CHOIR_DEDUPE_MIN_BYTES=4096 "$@" python3 -I "$dedupe" "$d/rel" >"$d/dd.log" 2>&1; }
if command -v python3 >/dev/null 2>&1 && python3 -c 'import random; random.randbytes' 2>/dev/null; then
f0=$fail
d="$root/d1"; mkdd "$d" edit
rundd; rc=$?
res="$d/rel/.dedupe/$CUR.json"
[ $rc -eq 0 ] || bad "dedupe: exit $rc"
[ "$(jq -r '.bytes_deduped' "$res")" = "$((61 * 4096))" ] && [ "$(jq -r '.files["storedisk.erofs"].same_offset_bytes' "$res")" = "$((61 * 4096))" ] || bad "dedupe same-offset: $(cat "$res")"
jq -e '.old == "'$OLD'" and .status == "ok" and .peak_rss_kb > 0' "$res" >/dev/null || bad "dedupe: result lacks old/status/peak_rss"
jq -e '.status == "no_prior"' "$d/rel/.dedupe/$OLD.json" >/dev/null || bad "dedupe: the oldest release should record no_prior"
[ $fail -eq $f0 ] && ok "dedupe: unshifted image shares every unchanged block at the same offset; result records bytes and peak RSS"

f0=$fail
d="$root/d2"; mkdd "$d" shift
rundd; res="$d/rel/.dedupe/$CUR.json"
[ "$(jq -r '.files["storedisk.erofs"].moved_bytes' "$res")" = "$((63 * 4096))" ] || bad "dedupe shifted: $(cat "$res")"
d="$root/d3"; mkdd "$d" both
rundd; res="$d/rel/.dedupe/$CUR.json"
[ "$(jq -r '.bytes_deduped' "$res")" = "$((60 * 4096))" ] || bad "dedupe shifted+edited: $(cat "$res")"
[ $fail -eq $f0 ] && ok "dedupe: a shifted image (one block inserted at the front) is found by the moved-block pass; edited blocks are not shared"

f0=$fail
d="$root/d4"; mkdd "$d" edit
cmp -s "$d/rel/$CUR/storedisk.erofs" <(mkimg "$root/ref.img" edit; cat "$root/ref.img") || bad "dedupe: the new image's bytes changed"
rundd GO_CHOIR_DEDUPE_MAX_INDEX=2; rc=$?
[ $rc -eq 0 ] && jq -e '.status == "ok"' "$d/rel/.dedupe/$CUR.json" >/dev/null || bad "dedupe: a tiny index cap failed (exit $rc)"
rm -rf "$d/rel/.dedupe"; chmod 0 "$d/rel/$OLD/storedisk.erofs" 2>/dev/null; chmod 0 "$d/rel/$CUR/storedisk.erofs" 2>/dev/null
rundd; rc=$?
chmod 644 "$d/rel/$OLD/storedisk.erofs" "$d/rel/$CUR/storedisk.erofs"
[ $rc -eq 0 ] || bad "dedupe: an unreadable image exited $rc, want 0 (never fail the deploy)"
env python3 -I "$dedupe" "$root/does-not-exist" >/dev/null 2>&1 || bad "dedupe: a missing releases dir failed"
# without the fake: the real ioctl; where the filesystem has no FIDEDUPERANGE (macOS, tmpfs) the run records unsupported and exits 0
d="$root/d5"; mkdd "$d" edit
env GO_CHOIR_DEDUPE_MIN_BYTES=4096 python3 -I "$dedupe" "$d/rel" >"$d/dd.log" 2>&1; rc=$?
[ $rc -eq 0 ] || bad "dedupe real ioctl: exit $rc"
st="$(jq -r '.status' "$d/rel/.dedupe/$CUR.json")"
case "$st" in ok | unsupported) ;; *) bad "dedupe real ioctl: status $st";; esac
cmp -s "$d/rel/$OLD/storedisk.erofs" <(mkimg "$root/ref.img" none; cat "$root/ref.img") && cmp -s "$d/rel/$CUR/storedisk.erofs" <(mkimg "$root/ref.img" edit; cat "$root/ref.img") || bad "dedupe real ioctl: image bytes changed"
echo "note: dedupe real ioctl status on this filesystem: $st ($(jq -r '.bytes_deduped' "$d/rel/.dedupe/$CUR.json") bytes)"
[ $fail -eq $f0 ] && ok "dedupe: a tiny index cap, unreadable images, a missing directory and an unsupported filesystem all exit 0 and leave the images intact"
else
	echo "skip: dedupe cases need python3 >= 3.9"
fi

# --- layered bases (docs/design/layered-guest-image-2026-10-31.md) -----------------------
# Failure modes: a base deleted while a retained release, an in-use release or a
# live Firecracker names it; a base kept forever after its last release went; a
# base swept on its own age; an unreadable layers.json (fail closed); a base
# materialized a moment ago (young) swept before its release is installed.
BC=jjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjj; BX=kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk; BU=llllllllllllllllllllllllllllllll
BZ=mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm; BY=nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn; BI=pppppppppppppppppppppppppppppppp; BF=qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq
mkbase() { # DIR ID HOURS_AGO
	mkdir -p "$1/guest-bases/$2"
	echo base >"$1/guest-bases/$2/storedisk-base.erofs"
	touch -d "@$(hours "$3")" "$1/guest-bases/$2"
}
layer() { # DIR RELEASE BASE: make an existing release layered on BASE
	local rd="$1/rel/$2"
	mv "$rd/storedisk.erofs" "$rd/storedisk-app.erofs"
	printf '{"schema":1,"layout":"layered","base_id":"%s","base_store_path":"/nix/store/%s-guest-store-base.erofs"}\n' "$3" "$3" >"$rd/layers.json"
}
basefixture() {
	fixture "$1"
	layer "$1" $CUR $BC
	layer "$1" $OOS $BX
	layer "$1" $USED $BU
	mkbase "$1" $BC 100; mkbase "$1" $BX 100; mkbase "$1" $BU 100
	mkbase "$1" $BZ 100 # nothing names it
	mkbase "$1" $BY 1   # young
	mkbase "$1" $BI 100 # named only by vmctl's in_use answer
	mkdir -p "$1/guest-bases/not-a-base" "$1/guest-bases/.tmp-old"
	touch -t 202001010000 "$1/guest-bases/.tmp-old"
}
base_exists() { [ -d "$1/guest-bases/$2" ] && [ ! -L "$1/guest-bases/$2" ]; }
baseanswer() { # the answer, with an in_use entry that names BI
	answer true
	jq --arg bi "$BI" --arg r "$OOSRUN" '.in_use |= map(if .id == $r then . + {base_id:$bi} else . end)' "$root/answer.json" >"$root/answer2.json" && mv "$root/answer2.json" "$root/answer.json"
}

f0=$fail
d="$root/b1"; basefixture "$d"; baseanswer; start_stub 200 "$root/answer.json"
run_retention "$d" "$URL"; rc=$?
[ $rc -eq 0 ] || bad "bases: exit $rc"
for b in $BC $BU $BI $BY; do base_exists "$d" $b || bad "bases: $b was deleted (named by a retained or in-use release, or young)"; done
for b in $BX $BZ; do base_exists "$d" $b && bad "bases: $b was kept (nothing retained names it)"; done
exists "$d" $OOS && bad "bases: the out-of-support release was kept"
[ -d "$d/guest-bases/not-a-base" ] && [ -d "$d/guest-bases/.tmp-old" ] || bad "bases: an odd name was deleted"
[ $fail -eq $f0 ] && ok "bases: a base stays while a retained, in-use or young release names it; one whose last release went, and an unnamed one, are swept"
stop_stub

f0=$fail
d="$root/b2"; basefixture "$d"; answer true; echo '{' >"$d/rel/$USED/layers.json"; start_stub 200 "$root/answer.json"
run_retention "$d" "$URL"
for b in $BC $BU $BX $BZ $BI $BY; do base_exists "$d" $b || bad "bases: $b was deleted with an unreadable layers.json (must fail closed)"; done
[ $fail -eq $f0 ] && ok "bases: an unreadable layers.json of a retained release deletes no base"
stop_stub

f0=$fail
d="$root/b3"; basefixture "$d"; answer true; start_stub 404
run_retention "$d" "$URL"
for b in $BC $BU $BX $BZ $BI $BY; do base_exists "$d" $b || bad "bases: $b was deleted without a vmctl answer"; done
stop_stub
start_stub 200 "$root/answer.json"; touch "$d/rel/.tmp-fresh"
run_retention "$d" "$URL"
for b in $BC $BU $BX $BZ $BI $BY; do base_exists "$d" $b || bad "bases: $b was deleted while a deploy materializes"; done
[ $fail -eq $f0 ] && ok "bases: no vmctl answer, or a release materializing, deletes no base"
stop_stub

f0=$fail
d="$root/b4"; basefixture "$d"; baseanswer
mkdir -p "$d/state/vm-live"
echo '{"drives":[{"path_on_host":"/var/lib/go-choir/guest-bases/'$BF'/storedisk-base.erofs"}]}' >"$d/state/vm-live/fc-config.json"
mkbase "$d" $BF 100
start_stub 200 "$root/answer.json"
run_retention "$d" "$URL"
base_exists "$d" $BF || bad "bases: a base a VM's fc-config.json names was deleted"
[ $fail -eq $f0 ] && ok "bases: a base a VM's fc-config.json names stays"
stop_stub

f0=$fail
d="$root/b5"; fixture "$d"; answer true; start_stub 200 "$root/answer.json"
run_retention "$d" "$URL"; rc=$?
[ $rc -eq 0 ] && [ ! -e "$d/guest-bases" ] || bad "bases: a fleet with no guest-bases directory changed (exit $rc)"
expect_kept "$d" "single-image" $CUR $YOUNG $MID $USED $OOSRUN $OLD
expect_gone "$d" "single-image" $OOS
[ $fail -eq $f0 ] && ok "bases: a single-image fleet (no guest-bases) retains exactly as before"
stop_stub

# The layered ring's second release (design step 3/4): the layered link's target
# stays; a layered release is outside the previous/rollback choice and the
# "newer than current" keep, so installing one never displaces a single image's
# rollback target nor lingers as a pending switch.
LAYA=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx; LAYB=yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy
f0=$fail
d="$root/b6"; fixture "$d"; answer true
mkrelease "$d/rel" $LAYA 3; layer "$d" $LAYA $BC     # newer than current, not young, not the ring's current
mkrelease "$d/rel" $LAYB 400; layer "$d" $LAYB $BX   # the ring's current, long out of support
mkbase "$d" $BC 100; mkbase "$d" $BX 100
ln -s "/nix/store/${LAYB}-go-choir-guest-image-layered" "$d/guest-layered"
start_stub 200 "$root/answer.json"
run_retention "$d" "$URL" GO_CHOIR_GUEST_LAYERED_LINK="$d/guest-layered"; rc=$?
[ $rc -eq 0 ] || bad "layered ring: exit $rc"
expect_kept "$d" "layered ring" $CUR $YOUNG $MID $USED $OOSRUN $OLD $LAYB
expect_gone "$d" "layered ring" $OOS $LAYA
base_exists "$d" $BX || bad "layered ring: the ring current's base was deleted"
base_exists "$d" $BC && bad "layered ring: a deleted layered release's base was kept"
[ $fail -eq $f0 ] && ok "retention: the layered ring's current release (and its base) stays; other layered releases follow the normal rules and do not displace previous/rollback"
stop_stub

f0=$fail
d="$root/b7"; fixture "$d"; answer true
mkrelease "$d/rel" $LAYB 400; layer "$d" $LAYB $BX; mkbase "$d" $BX 100
ln -s "/nix/store/${LAYB}-go-choir-guest-image-layered" "$d/guest-layered"
start_stub 200 "$root/answer.json"
run_retention "$d" "$URL" GO_CHOIR_GUEST_LAYERED_LINK="$d/no-such-link"
exists "$d" $LAYB && bad "layered ring: without the link the stale layered release was kept"
[ $fail -eq $f0 ] && ok "retention: an unreadable layered link keeps nothing extra (the ring falls back to single at launch)"
stop_stub

# --- materialize ------------------------------------------------------------------
# These cases need GNU cp (--reflink) and stat (-c), as Node B has; elsewhere
# they are skipped, not failed (review 331 Devin 2).
if ! cp --help 2>&1 | grep -q -- '--reflink' || ! stat -c %a / >/dev/null 2>&1; then
	echo "skip: materialize cases need GNU coreutils"
	skip_materialize=1
else
	skip_materialize=0
fi
if [ "$skip_materialize" = 0 ]; then
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

f0=$fail
d="$root/m6"; mkdir -p "$d"; src="$(mksrc "$d")"
mkdir -p "$d/rel"; mkrelease "$d/rel" $OLD 50
printf '{"first_current_at":"2020-01-01T00:00:00Z"}\n' >"$d/rel/$OLD/installed.json"   # the fixture clock is 2027; the real clock stamps the new release
env GO_CHOIR_RELEASE_REFLINK=auto GO_CHOIR_GUEST_RELEASE_DEDUPE="$dedupe" GO_CHOIR_DEDUPE_SPAWN=direct GO_CHOIR_DEDUPE_FAKE=1 GO_CHOIR_DEDUPE_MIN_BYTES=1 bash "$materialize" "$src" "$d/rel" >"$root/mat.log" 2>&1; rc=$?
[ $rc -eq 0 ] && [ -f "$d/rel/.dedupe/$A.json" ] && jq -e --arg o "$OLD" '.old == $o' "$d/rel/.dedupe/$A.json" >/dev/null || bad "materialize+dedupe: exit $rc, no result against the previous release"
env GO_CHOIR_RELEASE_REFLINK=auto GO_CHOIR_GUEST_RELEASE_DEDUPE=/nonexistent bash "$materialize" "$d/src/${B}-go-choir-guest-image" "$d/rel" >/dev/null 2>&1 || bad "materialize: a missing dedupe tool failed the switch"
[ $fail -eq $f0 ] && ok "materialize: starts the dedupe against the previous release after the install; a missing tool is ignored"

# layered sources (design step 2): base once into guest-bases, app into the release
mklsrc() { # DIR [BASE_ID] -> src path of a layered image
	local src="$1/src/${A}-go-choir-guest-image" bid="${2:-$BC}"
	mkdir -p "$src"
	for f in vmlinux initrd rootfs.ext4 storedisk-app.erofs storedisk-base.erofs kernel-params build.json guest-image-manifest release.json; do echo "$f" >"$src/$f"; done
	printf '{"schema":1,"base":"/nix/store/%s-guest-store-base.erofs","app":"/nix/store/x-guest-store-app.erofs"}\n' "$bid" >"$src/layers.json"
	echo "$src"
}
f0=$fail
d="$root/l1"; mkdir -p "$d"; src="$(mklsrc "$d")"
run_mat "$src" "$d/rel"; rc=$?
rd="$d/rel/$A"; bd="$d/guest-bases/$BC"
[ $rc -eq 0 ] || bad "layered materialize: exit $rc"
for f in vmlinux initrd rootfs.ext4 storedisk-app.erofs kernel-params build.json guest-image-manifest release.json installed.json layers.json; do [ -f "$rd/$f" ] || bad "layered materialize: $f missing"; done
[ -e "$rd/storedisk.erofs" ] && bad "layered materialize: the release has storedisk.erofs (an old vmctl would launch it with one drive)"
[ -e "$rd/storedisk-base.erofs" ] && bad "layered materialize: the base was copied into the release"
[ -f "$bd/storedisk-base.erofs" ] && [ "$(stat -c %a "$bd/storedisk-base.erofs")" = 444 ] && [ "$(stat -c %a "$bd")" = 555 ] || bad "layered materialize: the base is missing or not 0444/0555"
jq -e --arg b "$BC" '.schema == 1 and .layout == "layered" and .base_id == $b' "$rd/layers.json" >/dev/null || bad "layered materialize: layers.json is wrong"
ls -A "$d/rel" "$d/guest-bases" | grep -q '^\.tmp-' && bad "layered materialize: a .tmp tree is left"
[ $fail -eq $f0 ] && ok "materialize: a layered image puts the base once in guest-bases (0444), the app layer in the release, no storedisk.erofs"

f0=$fail
inode=$(stat -c %i "$bd/storedisk-base.erofs"); echo changed >"$d/src/${A}-go-choir-guest-image/storedisk-base.erofs"
mkdir -p "$d/src2"; src2="$d/src2/${B}-go-choir-guest-image"; cp -r "$d/src/${A}-go-choir-guest-image" "$src2"
run_mat "$src2" "$d/rel"; rc=$?
[ $rc -eq 0 ] && [ -d "$d/rel/$B" ] || bad "layered materialize: a second release on the same base failed (exit $rc)"
[ "$(cat "$bd/storedisk-base.erofs")" = "storedisk-base.erofs" ] && [ "$(stat -c %i "$bd/storedisk-base.erofs")" = "$inode" ] || bad "layered materialize: an existing base was rewritten"
[ $fail -eq $f0 ] && ok "materialize: a second release on the same base leaves the base untouched (never rewritten)"

f0=$fail
d="$root/l2"; mkdir -p "$d"; src="$(mklsrc "$d")"; rm "$src/storedisk-base.erofs"
run_mat "$src" "$d/rel"; rc=$?
[ $rc -eq 0 ] || bad "layered missing base: exit $rc"
[ -e "$d/rel/$A" ] && bad "layered missing base: a release directory exists"
ls -A "$d/rel" "$d/guest-bases" 2>/dev/null | grep -q '^\.tmp-' && bad "layered missing base: a .tmp tree is left"
d="$root/l3"; mkdir -p "$d"; src="$(mklsrc "$d")"; echo '{}' >"$src/layers.json"
run_mat "$src" "$d/rel"; rc=$?
[ $rc -eq 0 ] && [ ! -e "$d/rel/$A" ] || bad "layered layers.json without a base: exit $rc or a release exists"
[ $fail -eq $f0 ] && ok "materialize: a layered source missing its base, or naming none, exits 0 and leaves nothing"

fi
# --- wiring in nix/node-b.nix (text assertions) ---------------------------------------
nix="$here/../nix/node-b.nix"
grep -qE '^\s*snapshotReleaseRetain = (true|false);' "$nix" && ok "nix: snapshotReleaseRetain is a let-binding" || bad "nix: the snapshotReleaseRetain let-binding is missing"
grep -E 'ReadOnlyPaths' "$nix" | grep -q 'guest-releases' && ok "nix: vmctl reads guest-releases read-only" || bad "nix: guest-releases is not in vmctl's ReadOnlyPaths"
grep -q 'd /var/lib/go-choir/guest-releases 0755 root root -' "$nix" && ok "nix: the tmpfiles rule" || bad "nix: the tmpfiles rule for guest-releases is missing"
grep -q 'system.activationScripts.go-choir-guest-release' "$nix" && grep -q 'deps = \[ "go-choir-guest-release" \]' "$nix" && ok "nix: go-choir-guest-image depends on the release snippet" || bad "nix: the guest-image snippet does not depend on go-choir-guest-release"
grep -q 'systemd.services.go-choir-guest-release-retention' "$nix" && grep -q 'systemd.timers.go-choir-guest-release-retention' "$nix" && ok "nix: the retention oneshot and timer" || bad "nix: the retention unit or timer is missing"
grep -q 'guestReleaseDedupe' "$nix" && grep -q 'go-choir-guest-release-dedupe' "$nix" && ok "nix: the dedupe tool is wired" || bad "nix: the dedupe tool is not wired"
grep -q 'guestReleaseRetention' "$nix" && grep -c 'guestReleaseRetention' "$nix" | grep -qE '^[3-9]' && ok "nix: the disk retention sweep calls it" || bad "nix: diskRetentionSweep does not call the release retention"

grep -q 'VM_SNAPSHOT_RELEASE_RETAIN=${if snapshotReleaseRetain then "on" else "off"}' "$nix" && grep -q 'VM_GUEST_RELEASES_DIR=/var/lib/go-choir/guest-releases' "$nix" && grep -q 'VM_GUEST_RELEASE_SUPPORT_WINDOW=168h' "$nix" && grep -q 'VM_GUEST_RELEASE_LINK=/var/lib/go-choir/guest"' "$nix" && ok "nix: vmctl's four release variables follow the one switch" || bad "nix: the vmctl release env lines are missing"

grep -E 'ReadOnlyPaths' "$nix" | grep -q 'guest-bases' && grep -q 'd /var/lib/go-choir/guest-bases 0755 root root -' "$nix" && ok "nix: guest-bases exists (tmpfiles) and vmctl reads it read-only" || bad "nix: guest-bases is not wired for vmctl"

grep -q 'VM_GUEST_LAYERED_COMPUTERS=' "$nix" && grep -q 'VM_GUEST_LAYERED_RELEASE_LINK=/var/lib/go-choir/guest-layered"' "$nix" && grep -qE '^\s*guestLayeredComputers = \[ "computer-59c902c5d46caecc0a180a8557164e72" \];' "$nix" && ok "nix: the layered ring selector is on for d@e (ComputerIDs only)" || bad "nix: the layered ring selector is missing or not set to d@e"
grep -q 'system.activationScripts.go-choir-guest-release-layered' "$nix" && grep -q 'guestImageLayered' "$nix" && ok "nix: the layered image is a second release and a second link, never the guest link" || bad "nix: the layered release activation is missing"
awk '/system.activationScripts.go-choir-guest-image = \{/{f=1} f&&/src="\$\{guestImage\}"/{print "single"; exit}' "$nix" | grep -q single && ok "nix: the guest link still names the single image" || bad "nix: the guest link no longer names guestImage"

# Activation runs materialize with the wrapper's PATH alone (no sed on it),
# so every tool the script calls must be on the wrapper's list; a missing sed
# made the layered branch log "layers.json names no base" (fc5076ab0).
mat_path=$(awk '/guestReleaseMaterialize = pkgs.writeShellScript/{f=1} f&&/export PATH=/{print; exit}' "$nix")
missing=""
grep -qE '(^|[^a-z])sed ' "$here/node-b-guest-release-materialize" && ! printf '%s' "$mat_path" | grep -q 'pkgs.gnused' && missing="$missing sed"
grep -qE '(^|[^a-z])grep ' "$here/node-b-guest-release-materialize" && ! printf '%s' "$mat_path" | grep -q 'pkgs.gnugrep' && missing="$missing grep"
grep -qE 'systemd-run' "$here/node-b-guest-release-materialize" && ! printf '%s' "$mat_path" | grep -q 'pkgs.systemd' && missing="$missing systemd-run"
[ -z "$missing" ] && ok "nix: the materialize wrapper's PATH has every tool the script calls" || bad "nix: the materialize wrapper's PATH lacks:$missing"

echo "node-b-guest-release-retention tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
