#!/usr/bin/env bash
# The snapshot vault's host script (scripts/snapshot-vault.sh) against stubbed
# tools. No root, no devices, no network, no secrets: cryptsetup, losetup,
# mkfs.ext4, mount, umount, mountpoint and truncate are stubs that record
# their arguments and keep a tiny state machine in a temp directory.
#
# Failure modes (written before the script; FP9 goal file, "Host module and
# script", H1 to H6, plus the ones found while writing them):
#
#  H1 The unit runs twice and recreates a live vault: never. A vault that is
#     up (mounted, our mapping, plain) is left alone and the second run says so.
#  H2 A stale mapping or loop device from a failed run blocks the next boot:
#     it is closed and detached first. A stale mapping that cannot be closed
#     stops the run (exit 1) before anything is created.
#  H3 The backing file is readable by others, or the mount root is left 0755.
#  H4 The key comes from anywhere but /dev/urandom, or the mapping is not
#     plain mode: the arguments are checked.
#  H5 The mapping opened is not a CRYPT-PLAIN- target (a wrong cipher string,
#     a LUKS mapping from an earlier run): the run tears down and exits 1.
#  H6 status passes a vault that is not dm-crypt plain, not mounted, or loose.
#  H7 Files are under the mount point before the mount (a plaintext copy the
#     mount would hide): refuse before creating anything, counts only.
#  H8 A tool is missing: exit 5 before anything is created.
#  H9 mkfs fails halfway: everything done so far is undone.
#  H10 down while a VMM still has the vault busy: nothing is closed.
#  H11 down on a free vault leaves no mapping, loop device or backing file.
#  H12 The mount point holds some other file system: left alone, exit 1.
#  H13 The size reaches truncate as given.
#  H14 scan reports counts, never the canary.
#
# Run: bash scripts/snapshot-vault-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/snapshot-vault.sh"
BASH_BIN="$(command -v bash)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0

# stubs NAME: a toolbox with real basic tools and the stubbed vault tools.
make_box() { # name
  local box="$WORK/$1" t p
  mkdir -p "$box/bin" "$box/state" "$box/dev" "$box/sys" "$box/mnt-parent"
  for t in env date cat head tr mkdir rm basename dirname awk sed grep tail mv cp ls sort wc readlink find chmod touch ln printf sleep id; do
    p="$(command -v "$t" 2>/dev/null)"
    case "$p" in /*) ln -sf "$p" "$box/bin/$t" ;; esac
  done
  cat > "$box/bin/cryptsetup" <<'EOF'
#!/bin/sh
echo "cryptsetup $*" >> "$STATE/calls.log"
case "$1" in
status) [ -f "$STATE/mapped" ] || exit 4
  printf 'type:    PLAIN\ncipher:  aes-xts-plain64\nkeysize: 512 bits\n'; exit 0 ;;
open) [ -f "$STATE/fail_open" ] && exit 1
  : > "$STATE/mapped"; : > "$DEVDIR/dm-7"; ln -sf dm-7 "$DEVDIR/$VAULT_NAME"
  mkdir -p "$SYSDIR/class/block/dm-7/dm"
  printf '%s\n' "${UUID_VALUE:-CRYPT-PLAIN-$VAULT_NAME}" > "$SYSDIR/class/block/dm-7/dm/uuid"
  printf '254:7\n' > "$SYSDIR/class/block/dm-7/dev"; exit 0 ;;
close) [ -f "$STATE/fail_close" ] && exit 1
  rm -f "$STATE/mapped" "$DEVDIR/$VAULT_NAME" "$DEVDIR/dm-7"; exit 0 ;;
esac
exit 2
EOF
  cat > "$box/bin/losetup" <<'EOF'
#!/bin/sh
echo "losetup $*" >> "$STATE/calls.log"
case "$1" in
--find) : > "$STATE/loop"; echo /dev/loop9 ;;
-j) [ -f "$STATE/loop" ] && echo /dev/loop9; exit 0 ;;
-d) rm -f "$STATE/loop" ;;
esac
exit 0
EOF
  cat > "$box/bin/mkfs.ext4" <<'EOF'
#!/bin/sh
echo "mkfs.ext4 $*" >> "$STATE/calls.log"
[ -f "$STATE/fail_mkfs" ] && exit 1
exit 0
EOF
  cat > "$box/bin/mount" <<'EOF'
#!/bin/sh
echo "mount $*" >> "$STATE/calls.log"
[ -f "$STATE/fail_mount" ] && exit 1
# A fresh ext4's root directory is 0755: the mount replaces the directory the
# script prepared, as the kernel does.
for last; do :; done
chmod 755 "$last"
: > "$STATE/mounted"; exit 0
EOF
  cat > "$box/bin/umount" <<'EOF'
#!/bin/sh
echo "umount $*" >> "$STATE/calls.log"
[ -f "$STATE/busy" ] && exit 32
rm -f "$STATE/mounted"; exit 0
EOF
  cat > "$box/bin/mountpoint" <<'EOF'
#!/bin/sh
[ -f "$STATE/mounted" ] || [ -f "$STATE/foreign_mount" ] || exit 1
[ "$1" = "-d" ] && { if [ -f "$STATE/foreign_mount" ]; then echo 8:1; else echo 254:7; fi; }
exit 0
EOF
  cat > "$box/bin/truncate" <<'EOF'
#!/bin/sh
echo "truncate $*" >> "$STATE/calls.log"
# Created with whatever mode the caller's umask gives; make it loose so the
# script's own chmod is what the test sees.
: > "$3"; chmod 644 "$3"; exit 0
EOF
  for t in cryptsetup losetup mkfs.ext4 mount umount mountpoint truncate; do chmod +x "$box/bin/$t"; done
}

# run NAME ARGS... ; env for the script is the box's. Sets RC and OUT.
run() {
  local name="$1"; shift
  local box="$WORK/$name"
  OUT="$(cd "$box" && env -i PATH="$box/bin" STATE="$box/state" DEVDIR="$box/dev" SYSDIR="$box/sys" \
    VAULT_IMG="$box/vault.img" VAULT_MOUNT="$box/mnt" VAULT_NAME="${VAULT_NAME:-choir-snapshot-vault}" \
    VAULT_DEV_DIR="$box/dev" VAULT_SYS_DIR="$box/sys" VAULT_SIZE_GIB="${VSIZE:-7}" VAULT_OWNER="$(id -un)" UUID_VALUE="${UUID_VALUE:-}" \
    "$BASH_BIN" "$SCRIPT" "$@" 2>&1 </dev/null)"
  RC=$?
  BOX="$box"
}
check() { # description, result (0 = pass)
  if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC out=$OUT"; fails=$((fails + 1)); fi
}
calls() { cat "$BOX/state/calls.log" 2>/dev/null; }
count() { calls | grep -c -- "$1"; }
nothing_created() { [ ! -e "$BOX/vault.img" ] && [ ! -e "$BOX/state/mapped" ] && [ ! -e "$BOX/state/loop" ] && [ ! -e "$BOX/state/mounted" ]; }

if [ ! -f "$SCRIPT" ]; then
  echo "FAIL scripts/snapshot-vault.sh does not exist"
  exit 1
fi

# --- a fresh boot ---------------------------------------------------------
make_box fresh
run fresh up
check "up on a fresh host exits 0" "$([ "$RC" = 0 ]; echo $?)"
check "up mapped, formatted and mounted the vault" "$([ -f "$BOX/state/mapped" ] && [ -f "$BOX/state/mounted" ] && [ "$(count '^mkfs.ext4')" = 1 ]; echo $?)"
# H3
check "H3 the backing file is 0600" "$(find "$BOX/vault.img" -maxdepth 0 -perm 600 | grep -q .; echo $?)"
check "H3 the mount root is 0700" "$(find "$BOX/mnt" -maxdepth 0 -perm 700 | grep -q .; echo $?)"
# H4
OPEN="$(calls | grep '^cryptsetup open')"
check "H4 plain mode, aes-xts, 512 bit key" "$(printf '%s' "$OPEN" | grep -q -- '--type plain' && printf '%s' "$OPEN" | grep -q -- '--cipher aes-xts-plain64' && printf '%s' "$OPEN" | grep -q -- '--key-size 512'; echo $?)"
check "H4 the key comes from /dev/urandom and from nowhere else" "$([ "$(printf '%s' "$OPEN" | grep -o -- '--key-file [^ ]*' | tr '\n' ' ')" = '--key-file /dev/urandom ' ]; echo $?)"
check "H4 no hash, passphrase or key slot options" "$(! printf '%s' "$OPEN" | grep -q -e '--hash' -e '--key-slot' -e 'luks' -e ' -d '; echo $?)"
# H13
check "H13 the size reaches truncate" "$(calls | grep -q '^truncate -s 7G '; echo $?)"
# H6
run fresh status
check "H6 status passes a good vault" "$([ "$RC" = 0 ]; echo $?)"
# H1
before="$(count '^cryptsetup open')$(count '^truncate')$(count '^mkfs')"
run fresh up
check "H1 a second up exits 0 and says it is already up" "$([ "$RC" = 0 ] && printf '%s' "$OUT" | grep -q 'already up'; echo $?)"
check "H1 a second up creates nothing" "$([ "$(count '^cryptsetup open')$(count '^truncate')$(count '^mkfs')" = "$before" ]; echo $?)"

# H6: status of a vault whose mapping is not plain
make_box luks
UUID_VALUE=CRYPT-LUKS2-0123-choir run luks up
check "H5 a mapping that is not CRYPT-PLAIN- is torn down, exit 1" "$([ "$RC" = 1 ] && nothing_created; echo $?)"
unset UUID_VALUE
make_box notplain
run notplain up
printf 'CRYPT-LUKS2-0123-choir\n' > "$BOX/sys/class/block/dm-7/dm/uuid"
run notplain status
check "H6 status fails a LUKS mapping" "$([ "$RC" != 0 ]; echo $?)"
printf 'CRYPT-PLAIN-choir\n' > "$BOX/sys/class/block/dm-7/dm/uuid"
chmod 755 "$BOX/mnt"
run notplain status
check "H6 status fails a loose mount root" "$([ "$RC" != 0 ]; echo $?)"
chmod 700 "$BOX/mnt"; rm -f "$BOX/state/mounted"
run notplain status
check "H6 status fails a vault that is not mounted" "$([ "$RC" != 0 ]; echo $?)"

# H2: stale mapping and loop from a failed run
make_box stale
: > "$WORK/stale/state/mapped"; : > "$WORK/stale/dev/dm-7"; ln -sf dm-7 "$WORK/stale/dev/choir-snapshot-vault"
: > "$WORK/stale/state/loop"; : > "$WORK/stale/vault.img"
run stale up
check "H2 a stale mapping is closed, a stale loop detached, then the vault comes up" "$([ "$RC" = 0 ] && calls | grep -q '^cryptsetup close' && calls | grep -q '^losetup -d' && [ -f "$BOX/state/mounted" ]; echo $?)"
make_box stuck
: > "$WORK/stuck/state/mapped"; : > "$WORK/stuck/state/fail_close"; : > "$WORK/stuck/vault.img"
run stuck up
check "H2 a stale mapping that will not close stops the run before anything is created" "$([ "$RC" = 1 ] && [ "$(count '^truncate')" = 0 ] && [ "$(count '^cryptsetup open')" = 0 ]; echo $?)"

# H7: files under the mount point
make_box hidden
mkdir -p "$WORK/hidden/mnt/vm-a/snapshot"; : > "$WORK/hidden/mnt/vm-a/snapshot/mem"
run hidden up
check "H7 files under the mount point refuse the mount, exit 1, nothing created" "$([ "$RC" = 1 ] && nothing_created && printf '%s' "$OUT" | grep -q 'not empty'; echo $?)"

# H8: a missing tool
make_box notool
rm -f "$WORK/notool/bin/cryptsetup"
run notool up
check "H8 a missing tool exits 5 and creates nothing" "$([ "$RC" = 5 ] && nothing_created && printf '%s' "$OUT" | grep -q cryptsetup; echo $?)"

# H9: mkfs fails
make_box mkfsfail
: > "$WORK/mkfsfail/state/fail_mkfs"
run mkfsfail up
check "H9 a failed mkfs undoes the mapping, the loop device and the file" "$([ "$RC" = 1 ] && nothing_created; echo $?)"
make_box mountfail
: > "$WORK/mountfail/state/fail_mount"
run mountfail up
check "H9 a failed mount undoes everything too" "$([ "$RC" = 1 ] && nothing_created; echo $?)"

# H12: some other file system is mounted at the mount point
make_box foreign
: > "$WORK/foreign/state/foreign_mount"
run foreign up
check "H12 a foreign mount is left alone: exit 1, nothing created" "$([ "$RC" = 1 ] && [ "$(count '^cryptsetup open')" = 0 ] && [ "$(count '^truncate')" = 0 ]; echo $?)"

# H10 and H11
make_box downbusy
run downbusy up
: > "$BOX/state/busy"
run downbusy down
check "H10 down with a busy vault closes nothing and exits 1" "$([ "$RC" = 1 ] && [ -f "$BOX/state/mapped" ] && [ -f "$BOX/vault.img" ]; echo $?)"
rm -f "$BOX/state/busy"
run downbusy down
check "H11 down on a free vault removes mapping, loop device and file" "$([ "$RC" = 0 ] && nothing_created; echo $?)"

# H14: scan
make_box scan
CANARY="canary-$(date +%s)-aabbccddeeff00112233445566778899"
printf '%s' "$CANARY" > "$WORK/scan/canary"
mkdir -p "$WORK/scan/tree/a"
{ echo before; printf '%s' "$CANARY"; echo after; } > "$WORK/scan/tree/a/hit"
echo nothing > "$WORK/scan/tree/miss"
run scan scan "$WORK/scan/canary" "$WORK/scan/tree"
check "H14 scan counts the files with a hit under a directory" "$([ "$RC" = 0 ] && printf '%s' "$OUT" | grep -q 'tree: hit: 1' && printf '%s' "$OUT" | grep -q 'files with a hit: 1'; echo $?)"
run scan scan "$WORK/scan/canary" "$WORK/scan/tree/a/hit"
check "H14 scan counts a single file with a hit" "$([ "$RC" = 0 ] && printf '%s' "$OUT" | grep -q 'hit: 1' && printf '%s' "$OUT" | grep -q 'files with a hit: 1'; echo $?)"
check "H14 scan never prints the canary" "$(! printf '%s' "$OUT" | grep -q -F "$CANARY"; echo $?)"
run scan scan "$WORK/scan/canary" "$WORK/scan/tree/miss"
check "H14 scan of a clean file reports zero" "$([ "$RC" = 0 ] && printf '%s' "$OUT" | grep -q 'hit: 0'; echo $?)"

if [ "$fails" = 0 ]; then echo "all passed"; else echo "$fails failed"; exit 1; fi
