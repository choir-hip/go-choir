#!/usr/bin/env bash
# Behavioural test for scripts/node-b-nix-retention. It drives the real script
# with a fake nix-env and a fake store, profile and gcroots tree under a temp
# dir, so it needs no Nix and no root and cannot touch a real store.
#
#   bash scripts/test-node-b-nix-retention.sh
#
# Failure modes (docs/problems/node-b-nix-store-and-host-disk-policy-2026-10-22.md),
# written before the script:
#   R1  deletes the current generation (after a rollback it is not the newest)
#   R2  keeps fewer than current plus the previous (the deploy rollback target)
#   R3  a keep count under 2, or not a number, prunes anyway
#   R4  unparseable --list-generations output still deletes something
#   R5  observe mode deletes more than the legacy +4 did
#   R6  unroots a prebuild-results, /tmp deploy or host-build link
#   R7  unroots one of the newest builder releases, or a young one
#   R8  removes a directory, a regular file, or a link outside the store
#   R9  unroots a builder link whose store path the current system, the
#       booted system or the current guest image uses
#   R10 an unknown mode acts as enforce
#   R11 a ".." path escapes the builder releases root
#   R12 observe is not exactly the legacy `nix-env --delete-generations +4`
#       (nix keeps every generation newer than current, current, and N-1
#       older; review round 2 S-F1, reproduced with real nix-env)
#   R13 enforce deletes the generation just below a rolled-back current
#   R14 generation numbers compare as text (99 after 100)
#   R15 an empty, relative or "/" builder root classifies every root as a
#       builder release (DS-6)
#   R16 a failed stat reads as mtime 0 and unroots the link (DS-6)
#   R17 the keep count is not a guard when every release is old (S-F5)
#   R18 the shipped default protect list loses an entry (S-F5)
set -u
here="$(cd "$(dirname "$0")" && pwd)"
script="$here/node-b-nix-retention"
fail=0
pass=0

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

mkdir -p "$root/bin"
cat >"$root/bin/stat" <<'EOF'
#!/usr/bin/env bash
# fake stat: fails for the path in $FAKE_STAT_FAIL, else the real one.
for a in "$@"; do [ -n "${FAKE_STAT_FAIL:-}" ] && [ "$a" = "$FAKE_STAT_FAIL" ] && exit 1; done
exec "$REAL_STAT" "$@"
EOF
chmod +x "$root/bin/stat"
REAL_STAT="$(command -v stat)"
export REAL_STAT
cat >"$root/bin/nix-env" <<'EOF'
#!/usr/bin/env bash
# fake nix-env: --list-generations prints $FAKE_DIR/generations;
# --delete-generations records its arguments.
echo "nix-env $*" >>"$FAKE_DIR/nix-env.log"
for a in "$@"; do
  case "$a" in
    --list-generations) cat "$FAKE_DIR/generations"; exit 0 ;;
  esac
done
exit 0
EOF
chmod +x "$root/bin/nix-env"

check() {
  local name="$1" cond="$2"
  if eval "$cond"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
    echo "FAIL: $name"
    echo "  condition: $cond"
    echo "  output:"; sed 's/^/    /' "$case_dir/out"
    echo "  nix-env:"; sed 's/^/    /' "$case_dir/nix-env.log" 2>/dev/null
  fi
}

# new_case builds a fresh tree: a store, builder releases, prebuild and
# host-build links, the auto gcroots that register them, and protected links.
new_case() {
  case_dir="$root/case-$1"
  rm -rf "$case_dir"
  mkdir -p "$case_dir/nix/store" "$case_dir/gcroots/auto" "$case_dir/rel" \
    "$case_dir/prebuild" "$case_dir/host-build/work/j1" "$case_dir/tmp" "$case_dir/run"
  : >"$case_dir/nix-env.log"
  for p in sys-cur sys-boot guest-cur r1 r2 r3 r4 r5 pre svc hb tmpres outside; do
    mkdir -p "$case_dir/nix/store/aaaa-$p"
  done
  ln -s "$case_dir/nix/store/aaaa-sys-cur" "$case_dir/run/current-system"
  ln -s "$case_dir/nix/store/aaaa-sys-boot" "$case_dir/run/booted-system"
  ln -s "$case_dir/nix/store/aaaa-guest-cur" "$case_dir/run/guest"
  n=0
  add_root() { # link-path target age-days
    mkdir -p "$(dirname "$1")"
    ln -s "$2" "$1"
    touch -h -d "@$(( $(date +%s) - $3 * 86400 ))" "$1"
    n=$((n + 1))
    ln -s "$1" "$case_dir/gcroots/auto/root$n"
  }
  # five builder releases, oldest first: r1 30d, r2 20d, r3 10d, r4 3d, r5 1d
  add_root "$case_dir/rel/c1/out/builder-result" "$case_dir/nix/store/aaaa-r1" 30
  add_root "$case_dir/rel/c2/out/builder-result" "$case_dir/nix/store/aaaa-r2" 20
  add_root "$case_dir/rel/c3/out/builder-result" "$case_dir/nix/store/aaaa-r3" 10
  add_root "$case_dir/rel/c4/out/builder-result" "$case_dir/nix/store/aaaa-r4" 3
  add_root "$case_dir/rel/c5/out/builder-result" "$case_dir/nix/store/aaaa-r5" 1
  add_root "$case_dir/prebuild/nixos-result" "$case_dir/nix/store/aaaa-pre" 40
  add_root "$case_dir/prebuild/service-vmctl-result" "$case_dir/nix/store/aaaa-svc" 40
  add_root "$case_dir/host-build/work/j1/result" "$case_dir/nix/store/aaaa-hb" 40
  add_root "$case_dir/tmp/go-choir-nixos-result" "$case_dir/nix/store/aaaa-tmpres" 40
  cat >"$case_dir/generations" <<'EOF'
   101   2026-10-15 10:00:00
   102   2026-10-16 10:00:00
   103   2026-10-18 10:00:00
   104   2026-10-20 10:00:00
   105   2026-10-21 10:00:00
   106   2026-10-22 10:00:00   (current)
EOF
}

run_script() {
  env PATH="$root/bin:$PATH" FAKE_DIR="$case_dir" \
    NIX_RETENTION_PROFILE="$case_dir/profiles/system" \
    NIX_RETENTION_GCROOTS_AUTO="$case_dir/gcroots/auto" \
    NIX_RETENTION_BUILDER_ROOT="$case_dir/rel" \
    NIX_RETENTION_STORE_PREFIX="$case_dir/nix/store/" \
    NIX_RETENTION_PROTECT="$case_dir/run/current-system $case_dir/run/booted-system $case_dir/run/guest" \
    "$@" bash "$script" >"$case_dir/out" 2>&1
  echo $? >"$case_dir/rc"
}

deleted_gens() { grep -- '--delete-generations' "$case_dir/nix-env.log" | sed 's/.*--delete-generations//' | tr -s ' ' | sed 's/^ //'; }

# ---- enforce, defaults: keep 3 generations, 2 newest releases, 7 days -----
new_case enforce
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "enforce exits 0" '[ "$(cat "$case_dir/rc")" = 0 ]'
check "R2 enforce runs nix-env +3" '[ "$(deleted_gens)" = "+3" ]'
check "R2 enforce logs 101 102 103" 'grep -q "deleting system generations (nix-env +3): 101 102 103" "$case_dir/out"'
check "R7 old releases unrooted (c1 c2 c3)" '[ ! -e "$case_dir/rel/c1/out/builder-result" ] && [ ! -L "$case_dir/rel/c1/out/builder-result" ] && [ ! -L "$case_dir/rel/c2/out/builder-result" ] && [ ! -L "$case_dir/rel/c3/out/builder-result" ]'
check "R7 newest two releases kept" '[ -L "$case_dir/rel/c4/out/builder-result" ] && [ -L "$case_dir/rel/c5/out/builder-result" ]'
check "R8 release directories kept" '[ -d "$case_dir/rel/c1/out" ]'
check "R6 prebuild, host-build and tmp links kept" '[ -L "$case_dir/prebuild/nixos-result" ] && [ -L "$case_dir/prebuild/service-vmctl-result" ] && [ -L "$case_dir/host-build/work/j1/result" ] && [ -L "$case_dir/tmp/go-choir-nixos-result" ]'
check "inventory names each auto root" '[ "$(grep -c "^nix retention: root " "$case_dir/out")" = 9 ]'
check "enforce logs unrooted" 'grep -q "unrooted .*rel/c1/out/builder-result" "$case_dir/out"'

# ---- R7 a young release past the keep count stays ---------------------------
new_case young
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce NIX_RETENTION_BUILDER_KEEP=1
check "R7 keep=1: c4 (3d) younger than 7d stays" '[ -L "$case_dir/rel/c4/out/builder-result" ]'
check "R7 keep=1: c3 (10d) unrooted" '[ ! -L "$case_dir/rel/c3/out/builder-result" ]'

# ---- observe: legacy +4 deletion only; nothing unrooted ----------------------
new_case observe
run_script
check "R10 default mode is observe" 'grep -q "mode=observe" "$case_dir/out"'
check "R12 observe runs exactly nix-env +4" '[ "$(deleted_gens)" = "+4" ]'
check "R12 observe logs 101 102" 'grep -q "deleting system generations (nix-env +4): 101 102" "$case_dir/out"'
check "R5 observe logs what keep=3 would delete (103)" 'grep -q "would delete system generations: 103" "$case_dir/out"'
check "R5 observe unroots nothing" '[ -L "$case_dir/rel/c1/out/builder-result" ] && [ -L "$case_dir/rel/c3/out/builder-result" ]'
check "R5 observe logs would unroot c1" 'grep -q "would unroot .*rel/c1/out/builder-result" "$case_dir/out"'

new_case unknown
run_script GO_CHOIR_NIX_RETENTION_MODE=yes-please
check "R10 unknown mode logged and treated as observe" 'grep -q "unknown mode" "$case_dir/out" && grep -q "mode=observe" "$case_dir/out" && [ -L "$case_dir/rel/c1/out/builder-result" ]'

# ---- R1 rollback: the current generation is not the newest -------------------
new_case rollback
cat >"$case_dir/generations" <<'EOF'
   101   2026-10-15 10:00:00
   102   2026-10-16 10:00:00   (current)
   103   2026-10-18 10:00:00
   104   2026-10-20 10:00:00
   105   2026-10-21 10:00:00
   106   2026-10-22 10:00:00
EOF
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "R1/R13 enforce after a rollback to 102: newer and 101 kept, nothing deleted" '[ -z "$(deleted_gens)" ]'
new_case rollback-observe
cp "$root/case-rollback/generations" "$case_dir/generations"
run_script
check "R12 observe after a rollback to 102 deletes nothing (nix +4 keeps 101)" '[ -z "$(deleted_gens)" ]'

# ---- R12 Sonnet's real-nix case: 1..8, current 5; nix +4 deletes only 1 ------
new_case sonnet
for g in 1 2 3 4 6 7 8; do printf '   %s   2026-10-0%s 10:00:00\n' "$g" "$g"; done >"$case_dir/generations"
printf '   5   2026-10-05 10:00:00   (current)\n' >>"$case_dir/generations"
run_script
check "R12 observe: nix-env +4 (real nix deletes only 1)" '[ "$(deleted_gens)" = "+4" ] && grep -q "deleting system generations (nix-env +4): 1 (current 5)" "$case_dir/out"'
check "R12 observe logs what +3 would add: 2" 'grep -q "would delete system generations: 2 (keep 3, current 5)" "$case_dir/out"'
new_case sonnet-enforce
cp "$root/case-sonnet/generations" "$case_dir/generations"
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "R13 enforce keeps 4 3 (current and previous) and 6 7 8" '[ "$(deleted_gens)" = "+3" ] && grep -q "deleting system generations (nix-env +3): 1 2 (current 5)" "$case_dir/out"'

# ---- R14 numeric order across 99/100 ------------------------------------------
new_case numeric
printf '   98   a b\n   99   a b\n   100   a b\n   101   a b\n   102   a b   (current)\n' >"$case_dir/generations"
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "R14 enforce deletes 98 99, not 100" 'grep -q "deleting system generations (nix-env +3): 98 99 (current 102)" "$case_dir/out"'

# ---- R3 a keep count under 2 or not a number prunes no generation ------------
new_case keep1
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce NIX_RETENTION_KEEP_GENERATIONS=1
check "R3 keep=1 refused: no generation deleted" '[ -z "$(deleted_gens)" ] && grep -q "refused" "$case_dir/out"'
new_case keepx
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce NIX_RETENTION_KEEP_GENERATIONS=three
check "R3 keep=three refused: no generation deleted" '[ -z "$(deleted_gens)" ] && grep -q "refused" "$case_dir/out"'

# ---- R4 unparseable listing deletes nothing ----------------------------------
new_case garbage
printf 'error: cannot open profile\n' >"$case_dir/generations"
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "R4 no current generation found: nothing deleted" '[ -z "$(deleted_gens)" ] && grep -q "skipped" "$case_dir/out"'
new_case fewer
printf '   106   2026-10-22 10:00:00   (current)\n   105   2026-10-21 10:00:00\n' >"$case_dir/generations"
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "only two generations: nothing deleted" '[ -z "$(deleted_gens)" ]'
new_case garbage-observe
printf 'error: cannot open profile\n' >"$case_dir/generations"
run_script
check "R4 observe: no current generation found, nothing deleted" '[ -z "$(deleted_gens)" ]'

# ---- R15 a bad builder root refuses the builder step --------------------------
new_case emptyroot
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce NIX_RETENTION_BUILDER_ROOT=
check "R15 an empty builder root means the default, never /*" 'grep -q "builder_root=/var/lib/go-choir/builder/releases" "$case_dir/out" && [ -L "$case_dir/prebuild/nixos-result" ] && [ -L "$case_dir/tmp/go-choir-nixos-result" ] && [ -L "$case_dir/host-build/work/j1/result" ]'
for bad in "/" "rel" "/./" "//x"; do
  new_case "badroot-$(printf '%s' "$bad" | tr -c 'a-z' x)"
  run_script GO_CHOIR_NIX_RETENTION_MODE=enforce NIX_RETENTION_BUILDER_ROOT="$bad"
  check "R15 builder root \"$bad\" refused, nothing unrooted" 'grep -q "builder roots refused" "$case_dir/out" && [ -L "$case_dir/prebuild/nixos-result" ] && [ -L "$case_dir/tmp/go-choir-nixos-result" ] && [ -L "$case_dir/rel/c1/out/builder-result" ]'
done

# ---- R16 a failed stat keeps the link ------------------------------------------
new_case statfail
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce FAKE_STAT_FAIL="$root/case-statfail/rel/c1/out/builder-result"
check "R16 c1 kept when its stat fails" '[ -L "$case_dir/rel/c1/out/builder-result" ] && grep -q "cannot stat" "$case_dir/out"'
check "R16 c2 still unrooted" '[ ! -L "$case_dir/rel/c2/out/builder-result" ]'

# ---- R17 every release old: the default keep count is the guard ----------------
new_case allold
for c in 1 2 3 4 5; do touch -h -d "@$(( $(date +%s) - (40 - c) * 86400 ))" "$case_dir/rel/c$c/out/builder-result"; done
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "R17 the newest two (c5 c4) stay when all are over 7 days" '[ -L "$case_dir/rel/c5/out/builder-result" ] && [ -L "$case_dir/rel/c4/out/builder-result" ]'
check "R17 c3 c2 c1 unrooted" '[ ! -L "$case_dir/rel/c3/out/builder-result" ] && [ ! -L "$case_dir/rel/c2/out/builder-result" ] && [ ! -L "$case_dir/rel/c1/out/builder-result" ]'

# ---- R18 the shipped defaults ---------------------------------------------------
case_dir="$root"; : >"$root/out"
check "R18 default protect list names current, booted and guest" 'grep -qF "protect=\"\${NIX_RETENTION_PROTECT:-/run/current-system /run/booted-system /var/lib/go-choir/guest}\"" "$script"'
check "R18 default builder keep 2, min age 7, keep generations 3" 'grep -qF "NIX_RETENTION_BUILDER_KEEP:-2}" "$script" && grep -qF "NIX_RETENTION_BUILDER_MIN_AGE_DAYS:-7}" "$script" && grep -qF "NIX_RETENTION_KEEP_GENERATIONS:-3}" "$script"'

# ---- R9 a release whose store path a protected link uses stays ----------------
new_case protected
rm "$case_dir/run/guest"; ln -s "$case_dir/nix/store/aaaa-r1" "$case_dir/run/guest"
rm "$case_dir/run/booted-system"; ln -s "$case_dir/nix/store/aaaa-r2" "$case_dir/run/booted-system"
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "R9 c1 (the current guest image) kept" '[ -L "$case_dir/rel/c1/out/builder-result" ]'
check "R9 c2 (the booted system) kept" '[ -L "$case_dir/rel/c2/out/builder-result" ]'
check "R9 c3 still unrooted" '[ ! -L "$case_dir/rel/c3/out/builder-result" ]'

# ---- R8 non-symlinks and links outside the store stay; R11 ".." --------------
new_case shapes
rm "$case_dir/rel/c1/out/builder-result"; mkdir "$case_dir/rel/c1/out/builder-result"
rm "$case_dir/rel/c2/out/builder-result"; ln -s "$case_dir/tmp" "$case_dir/rel/c2/out/builder-result"
touch -h -d "@$(( $(date +%s) - 20 * 86400 ))" "$case_dir/rel/c2/out/builder-result"
ln -s "$case_dir/rel/../prebuild/nixos-result" "$case_dir/gcroots/auto/rootdotdot"
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "R8 a directory at the link path stays" '[ -d "$case_dir/rel/c1/out/builder-result" ]'
check "R8 a link outside the store stays" '[ -L "$case_dir/rel/c2/out/builder-result" ]'
check "R11 a .. path is not a builder release" '[ -L "$case_dir/prebuild/nixos-result" ]'
check "R8 c3 still unrooted" '[ ! -L "$case_dir/rel/c3/out/builder-result" ]'

# ---- missing builder root and gcroots dir are fine ----------------------------
new_case empty
rm -rf "$case_dir/rel" "$case_dir/gcroots"
run_script GO_CHOIR_NIX_RETENTION_MODE=enforce
check "no builder root: exits 0, generations still pruned" '[ "$(cat "$case_dir/rc")" = 0 ] && [ "$(deleted_gens)" = "+3" ]'

echo "node-b-nix-retention: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
