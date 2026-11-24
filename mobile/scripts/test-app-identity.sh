#!/bin/sh
# Pure-shell test of the app's build identity (scripts/app-identity.sh, and what follows it: exact.sh, build-lab.sh, proof-common.sh, sim-launch.sh,
# make-profile.sh). A copy of the scripts runs in a temporary tree over stubs (a stub Bun, a stub xe.sh and xcodegen, a throwaway Exact git repo):
# no Exact build, no Xcode, no simulator, no phone.   sh mobile/scripts/test-app-identity.sh
#
# Failure list (written before the code, CLAUDE.md "Proof"):
#  1. the default build is not the production app: no variant is selected, the id is news.choir.autoradio, the name "Autoradio 1".
#  2. a side-by-side build is chosen by something that app.json does not declare, so Exact refuses it late or builds the wrong app:
#     an unknown variant, an AR_APP_ID that is no declared id, an AR_APP_NAME that is not the declared name must each fail at once.
#  3. the choice does not reach Exact: exact.sh runs Bun under `env -i`, so an exported variable is dropped unless it is passed on.
#  4. a script keeps its own copy of the id: proof-common.sh (APP), sim-launch.sh (BUNDLE), make-profile.sh (the stub's bundle id).
#  6. make-profile.sh with no choice would make a development profile for the production id (a store app never has one): it must refuse and make nothing;
#     with a variant it makes the stub of that variant's id.
#  5. the build changes the tracked app.json (the build queue refuses a dirty tree): after a build that succeeds, one that fails, one that
#     is refused early (not the build tree) and one with an unknown variant, the file must be byte-identical.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SRC="$(cd "$HERE/.." && pwd)"
T="${TMPDIR:-/tmp}/test-app-identity.$$"; mkdir -p "$T"; trap 'rm -rf "$T"' EXIT
fails=0; ok() { echo "ok: $1"; }; no() { echo "FAIL: $1"; fails=$((fails + 1)); }
eq() { [ "$2" = "$3" ] && ok "$1" || no "$1: got '$2', wanted '$3'"; }

# the tree: scripts and app.json as committed, an Exact checkout at its pin, a Bun that only records
M="$T/mobile"; mkdir -p "$M/AutoradioLab" "$T/bin" "$T/home/Library/Developer/Xcode/UserData/Provisioning Profiles"
cp -R "$SRC/scripts" "$M/scripts"; cp "$SRC/AutoradioLab/app.json" "$M/AutoradioLab/app.json"
git init -q "$M/.exact-src" && git -C "$M/.exact-src" -c user.name=t -c user.email=t@t commit -q --allow-empty -m pin
git -C "$M/.exact-src" rev-parse HEAD > "$M/exact.pin"
printf '#!/bin/sh\nexit 0\n' > "$M/scripts/sync-exact-package.sh"
: > "$M/scripts/gen-exact-interface.py"   # run by python3: an empty script is fine
cat > "$T/bin/bun" <<'B'
#!/bin/sh
# records the variant Exact would see, and the arguments; a file named "fail" beside it makes the Exact command itself fail
D="$(dirname "$0")"
[ "${1:-}" = install ] && exit 0
printf 'variant=[%s] args=[%s]\n' "${EXACT_VARIANT:-}" "$*" >> "$D/bun.log"
printf '%s' "${APP_ID-unset}" > "$D/bun.scheme"
[ -f "$D/fail" ] && exit 3
exit 0
B
chmod +x "$T/bin/bun"
printf "%s\n" "$(cd "$M" && pwd -P)" > "$T/tree"
APPJSON="$M/AutoradioLab/app.json"; SUM0="$(shasum -a 256 < "$APPJSON" 2>/dev/null)"
# the byte checks below compare two checksums: with no shasum both are empty and every one of them would pass
command -v shasum >/dev/null 2>&1 && ok "shasum is there" || no "shasum is missing: the app.json byte checks cannot run"
[ -n "$SUM0" ] && ok "app.json has a checksum to compare against" || no "app.json checksum is empty: the byte checks would pass on two empty strings"
run_build() { # run_build <command...>: over the stubs, the Bun log cleared first
  : > "$T/bin/bun.log"
  env -u EXACT2 HOME="$T/home" BUN="$T/bin/bun" ALLOW_NO_MODELS=1 AUTORADIO_BUILD_TREE_FILE="$T/tree" "$@" > "$T/out.txt" 2>&1; rc=$?; [ -n "${TEST_DEBUG:-}" ] && cat "$T/out.txt" >&2; return $rc
}
ID=news.choir.autoradio.lab.one; LAB=news.choir.autoradio.lab; PROD=news.choir.autoradio

# 1. the default
eq "default id" "$(env -u AR_VARIANT -u AR_APP_ID -u AR_APP_NAME "$M/scripts/app-identity.sh" id 2>&1)" $PROD
eq "default name" "$("$M/scripts/app-identity.sh" name 2>&1)" "Autoradio 1"
eq "default variant is none" "$("$M/scripts/app-identity.sh" variant 2>&1)" ""
run_build "$M/scripts/build-lab.sh" ios; rc=$?
eq "default build: Exact sees no variant" "$(cat "$T/bin/bun.log")" "variant=[] args=[exact.mjs ios]"
eq "default build exits 0" "$rc" 0

# 2. the choice is checked against what app.json declares
eq "AR_VARIANT=lab-one id" "$(AR_VARIANT=lab-one "$M/scripts/app-identity.sh" id 2>&1)" $ID
eq "AR_VARIANT=lab-one name" "$(AR_VARIANT=lab-one "$M/scripts/app-identity.sh" name 2>&1)" "Autoradio 1 Dev"
eq "AR_APP_ID alone selects the variant" "$(AR_APP_ID=$ID "$M/scripts/app-identity.sh" variant 2>&1)" lab-one
eq "AR_APP_ID and AR_APP_NAME that agree" "$(AR_APP_ID=$ID AR_APP_NAME="Autoradio 1 Dev" "$M/scripts/app-identity.sh" id 2>&1)" $ID
eq "AR_APP_ID naming the base is the default" "$(AR_APP_ID=$PROD "$M/scripts/app-identity.sh" variant 2>&1)" ""
for bad in "AR_VARIANT=nope" "AR_VARIANT=one" "AR_APP_ID=news.choir.autoradio.one" "AR_VARIANT=lab-one AR_APP_ID=news.choir.autoradio.lab" "AR_VARIANT=lab-one AR_APP_NAME=Other" "AR_APP_NAME=Other"; do
  if env $bad "$M/scripts/app-identity.sh" id >/dev/null 2>&1; then no "refused: $bad"; else ok "refused: $bad"; fi
done

# 3. the choice reaches Exact (through env -i), for the build and for every other verb that goes through exact.sh
run_build env AR_VARIANT=lab-one "$M/scripts/build-lab.sh" ios --device --phone X
eq "variant build: Exact sees lab-one" "$(cat "$T/bin/bun.log")" "variant=[lab-one] args=[exact.mjs ios --device --phone X]"
run_build env AR_APP_ID=$ID "$M/scripts/exact.sh" agent ios tree
eq "exact.sh agent: Exact sees lab-one" "$(cat "$T/bin/bun.log")" "variant=[lab-one] args=[exact.mjs agent ios tree]"
run_build env AR_VARIANT=nope "$M/scripts/exact.sh" ios; rc=$?
[ "$rc" -ne 0 ] && [ ! -s "$T/bin/bun.log" ] && ok "an unknown variant stops before Exact runs" || no "unknown variant ran Exact (rc $rc)"

# 3b. the app id (APP_ID), handed to Exact explicitly for EVERY identity (app.ts reads APP_ID with `??`, which does not catch an empty value):
#     the default no longer rests on a literal fallback, and a stray empty APP_ID in the caller's environment cannot reach the grant.
run_build "$M/scripts/exact.sh" agent ios tree
eq "default build: APP_ID is the production id" "$(cat "$T/bin/bun.scheme")" $PROD
run_build env AR_VARIANT=lab-one "$M/scripts/exact.sh" agent ios tree
eq "variant build: APP_ID is the variant's id" "$(cat "$T/bin/bun.scheme")" $ID
run_build env APP_ID= "$M/scripts/exact.sh" agent ios tree
eq "an empty APP_ID in the caller's environment does not reach Exact" "$(cat "$T/bin/bun.scheme")" $PROD

# 4. nothing else keeps its own copy of the id
if grep -n "news\.choir\.autoradio" "$M/scripts/proof-common.sh" "$M/scripts/sim-launch.sh" "$M/scripts/make-profile.sh" "$M/scripts/proof-real.sh" "$M/scripts/exact.sh" "$M/scripts/build-lab.sh" >/dev/null; then no "a script still spells the bundle id"; else ok "no script spells the bundle id"; fi
for v in "" lab lab-one; do
  case "$v" in "") want=$PROD ;; lab) want=$LAB ;; lab-one) want=$ID ;; esac
  got="$(env AR_VARIANT="$v" HERE="$M/scripts" SHOTS="" TMPDIR="$T" sh -c '. "$HERE/proof-common.sh"; printf %s "$APP"' 2>&1)"
  eq "proof-common.sh APP (variant '$v')" "$got" "$want"
  # sim-launch.sh: its simctl line, with xcrun replaced by echo
  sed "s#/usr/bin/xcrun#echo xcrun#g" "$M/scripts/sim-launch.sh" > "$M/scripts/sl.sh"
  got="$(env AR_VARIANT="$v" SECS=0 sh "$M/scripts/sl.sh" 2>&1 | grep -o "booted [^ ]*" | sort -u | tr '\n' ' ')"
  eq "sim-launch.sh bundle (variant '$v')" "$got" "booted $want "
  # make-profile.sh: the stub project's bundle id, over a recording xcodegen and a xe.sh that does nothing
  printf '#!/bin/sh\ncp project.yml "%s/project.yml.seen"\n' "$T" > "$T/bin/xcodegen"; chmod +x "$T/bin/xcodegen"
  printf '#!/bin/sh\nexit 0\n' > "$M/scripts/xe.sh"
  rm -f "$T/project.yml.seen"
  env AR_VARIANT="$v" HOME="$T/home" PATH="$T/bin:$PATH" "$M/scripts/make-profile.sh" >"$T/mp.out" 2>&1; mprc=$?
  if [ -z "$v" ]; then
    [ "$mprc" -ne 0 ] && [ ! -e "$T/project.yml.seen" ] && grep -q AR_VARIANT "$T/mp.out" && ok "make-profile.sh with no choice refuses and makes nothing" || no "make-profile.sh with no choice (rc $mprc, stub seen: $([ -e "$T/project.yml.seen" ] && echo yes || echo no))"
  else
    eq "make-profile.sh PRODUCT_BUNDLE_IDENTIFIER (variant '$v')" "$(sed -n 's/.*PRODUCT_BUNDLE_IDENTIFIER: //p' "$T/project.yml.seen" 2>/dev/null)" "$want"
  fi
done

# 5. the tracked app.json is never touched
: > "$T/bin/fail"; run_build env AR_VARIANT=lab-one "$M/scripts/build-lab.sh" ios; rc=$?; rm -f "$T/bin/fail"
[ "$rc" -ne 0 ] && ok "a failing Exact command fails the build (rc $rc)" || no "a failing Exact command passed"
eq "app.json after a failed build" "$(shasum -a 256 < "$APPJSON")" "$SUM0"
mkdir -p "$T/elsewhere"; printf '%s\n' "$T/elsewhere" > "$T/tree2"
env HOME="$T/home" BUN="$T/bin/bun" AR_VARIANT=lab-one AUTORADIO_BUILD_TREE_FILE="$T/tree2" "$M/scripts/build-lab.sh" ios >/dev/null 2>&1; rc=$?
[ "$rc" -ne 0 ] && ok "a tree that is not the build tree is refused (rc $rc)" || no "not-the-build-tree build passed"
eq "app.json after an early refusal" "$(shasum -a 256 < "$APPJSON")" "$SUM0"
run_build env AR_VARIANT=nope "$M/scripts/build-lab.sh" ios
eq "app.json after an unknown variant" "$(shasum -a 256 < "$APPJSON")" "$SUM0"
run_build env AR_VARIANT=lab-one "$M/scripts/build-lab.sh" ios
eq "app.json after a variant build" "$(shasum -a 256 < "$APPJSON")" "$SUM0"

[ "$fails" -eq 0 ] && { echo "ALL OK"; exit 0; } || { echo "$fails FAILED"; exit 1; }
