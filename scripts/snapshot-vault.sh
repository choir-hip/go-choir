#!/usr/bin/env bash
# The snapshot vault's host script (SDC NET-06, review F12; goal file
# docs/archive/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md).
#
# A volume whose key is read from /dev/urandom when the host boots and is
# never written down: a sparse file, a loop device, a dm-crypt PLAIN mapping
# (no header, no key slot), ext4, mounted at one path with mode 0700.
# vmctl writes hibernate snapshots (the guest's RAM) only there when
# VM_SNAPSHOT_VAULT_DIR names that path, and refuses to write them anywhere
# else (internal/vmmanager/snapshot_vault.go). A reboot or `down` makes the
# volume unreadable even to root; every hibernated computer then cold boots.
#
# Usage (nix/snapshot-vault.nix runs up and down; status and scan are for the
# operator and the deployed proof):
#   snapshot-vault.sh up                 create and mount the vault; a vault
#                                        that is already up is left alone
#   snapshot-vault.sh down               unmount and destroy it; refuses while
#                                        anything holds it open
#   snapshot-vault.sh status             verify a running vault; exit 0 only
#                                        if every check passes
#   snapshot-vault.sh scan FILE PATH...  count files and bytes under PATH that
#                                        contain the text in FILE (a canary);
#                                        prints counts, never the text
# Exit status: 0 ok; 1 failed or refused; 2 usage; 5 a tool is missing
# (nothing was done).
#
# Environment (the unit sets these): VAULT_IMG (backing file), VAULT_MOUNT,
# VAULT_NAME (device-mapper name), VAULT_SIZE_GIB, VAULT_DEV_DIR
# (/dev/mapper), VAULT_SYS_DIR (/sys), VAULT_OWNER (root; tests only). No secret is read, written, logged or
# put on a command line. There is no `set -x`. Strict mode (bunch 31): an
# unguarded failing step stops the script; every step whose failure is an
# answer (a check, a probe) is guarded explicitly.
set -euo pipefail

IMG="${VAULT_IMG:-/var/lib/go-choir/snapshot-vault.img}"
MNT="${VAULT_MOUNT:-/var/lib/go-choir/snapshot-vault}"
NAME="${VAULT_NAME:-choir-snapshot-vault}"
SIZE_GIB="${VAULT_SIZE_GIB:-56}"
DEVDIR="${VAULT_DEV_DIR:-/dev/mapper}"
SYSDIR="${VAULT_SYS_DIR:-/sys}"
OWNER="${VAULT_OWNER:-root}"
PLAIN_PREFIX="CRYPT-PLAIN-"

log() { printf '%s snapshot-vault: %s\n' "$(date -u +%FT%TZ)" "$*"; }
err() { printf '%s snapshot-vault: ERROR: %s\n' "$(date -u +%FT%TZ)" "$*" >&2; }

need() { # tools...: refuse before doing anything if one is missing
  local t missing=""
  for t in "$@"; do command -v "$t" >/dev/null 2>&1 || missing="$missing $t"; done
  if [ -n "$missing" ]; then
    err "missing tools:$missing"
    exit 5
  fi
}

# --- facts about the mapping ------------------------------------------------
dm_node() { # the kernel's name for our mapping (dm-N), empty if unmapped
  [ -e "$DEVDIR/$NAME" ] || return 0
  basename "$(readlink -f "$DEVDIR/$NAME")"
}
dm_uuid() { # empty when unmapped or unreadable
  local node
  node="$(dm_node)"
  if [ -n "$node" ]; then cat "$SYSDIR/class/block/$node/dm/uuid" 2>/dev/null || true; fi
}
dm_dev() { # major:minor of the mapping; empty when unmapped or unreadable
  local node
  node="$(dm_node)"
  if [ -n "$node" ]; then cat "$SYSDIR/class/block/$node/dev" 2>/dev/null || true; fi
}
mapping_is_plain() {
  case "$(dm_uuid)" in "$PLAIN_PREFIX"*) return 0 ;; esac
  return 1
}
mounted_is_ours() { # the mount at MNT is on our mapping's device
  mountpoint -q "$MNT" 2>/dev/null || return 1
  local want got
  want="$(dm_dev)"
  got="$(mountpoint -d "$MNT" 2>/dev/null || true)"
  [ -n "$want" ] && [ "$want" = "$got" ]
}
detach_loops() {
  local l
  for l in $(losetup -j "$IMG" -O NAME --noheadings 2>/dev/null); do
    losetup -d "$l" || return 1
  done
  return 0
}

# --- up ---------------------------------------------------------------------
ROLLBACK_LOOP=""
ROLLBACK_MAPPED=0
ROLLBACK_MOUNTED=0
ROLLBACK_IMG=0
ROLLBACK_OK=0
rollback() {
  [ "$ROLLBACK_OK" = 1 ] && return 0
  # Best effort, every step attempted: a failure here must not stop the rest.
  if [ "$ROLLBACK_MOUNTED" = 1 ]; then umount "$MNT" >/dev/null 2>&1 || true; fi
  if [ "$ROLLBACK_MAPPED" = 1 ]; then cryptsetup close "$NAME" >/dev/null 2>&1 || true; fi
  if [ -n "$ROLLBACK_LOOP" ]; then losetup -d "$ROLLBACK_LOOP" >/dev/null 2>&1 || true; fi
  if [ "$ROLLBACK_IMG" = 1 ]; then rm -f "$IMG" || true; fi
  return 0
}
fail() { err "$*"; exit 1; }

cmd_up() {
  need cryptsetup losetup mkfs.ext4 mount umount mountpoint truncate chmod mkdir rm readlink basename cat date
  if mountpoint -q "$MNT" 2>/dev/null; then
    if mapping_is_plain && mounted_is_ours; then
      log "already up: $MNT is the plain dm-crypt vault; left alone"
      return 0
    fi
    fail "$MNT is a mount point that is not this vault; left alone"
  fi
  # A directory with files under it would be hidden by the mount: those files
  # would be a plaintext copy of what the vault is meant to hold.
  if [ -d "$MNT" ] && [ -n "$(ls -A "$MNT" 2>/dev/null)" ]; then
    fail "$MNT is not empty ($(ls -A "$MNT" | wc -l | tr -d ' ') entries); refusing to mount over it"
  fi
  # A mapping or loop device left by a failed run. Closing is refused, not
  # forced, if anything still uses it.
  if cryptsetup status "$NAME" >/dev/null 2>&1; then
    cryptsetup close "$NAME" || fail "a stale mapping $NAME cannot be closed (in use?); nothing created"
  fi
  detach_loops || fail "a stale loop device on $IMG cannot be detached; nothing created"

  trap rollback EXIT
  umask 077
  rm -f "$IMG" || fail "cannot remove a stale backing file $IMG"
  truncate -s "${SIZE_GIB}G" "$IMG" || fail "cannot create the ${SIZE_GIB}G backing file $IMG"
  ROLLBACK_IMG=1
  chmod 0600 "$IMG" || fail "cannot set $IMG to 0600"
  ROLLBACK_LOOP="$(losetup --find --show "$IMG")" || fail "no loop device for $IMG"
  # The key is read from /dev/urandom by cryptsetup and goes to the kernel;
  # it is not in a file, a variable or an argument. Plain mode has no header
  # and no key slot, so once the mapping is closed or the host reboots nothing
  # on disk can reopen it.
  cryptsetup open --type plain --cipher aes-xts-plain64 --key-size 512 \
    --key-file /dev/urandom --keyfile-size 64 --batch-mode "$ROLLBACK_LOOP" "$NAME" \
    || fail "cryptsetup could not open the plain mapping"
  ROLLBACK_MAPPED=1
  mapping_is_plain || fail "the mapping is not a CRYPT-PLAIN- target (uuid '$(dm_uuid)'); torn down"
  mkfs.ext4 -q -F -m 0 -T largefile4 -O ^has_journal -L choir-snap-vault "$DEVDIR/$NAME" \
    || fail "mkfs.ext4 failed"
  mkdir -p "$MNT" || fail "cannot create $MNT"
  mount -o noatime,nodev,nosuid,noexec "$DEVDIR/$NAME" "$MNT" || fail "mount failed"
  ROLLBACK_MOUNTED=1
  chmod 0700 "$MNT" || fail "cannot set $MNT to 0700"
  ROLLBACK_OK=1
  log "up: $MNT on a plain dm-crypt mapping with an ephemeral key (${SIZE_GIB}G sparse backing file $IMG)"
}

# --- down -------------------------------------------------------------------
cmd_down() {
  need cryptsetup losetup umount mountpoint rm readlink basename cat date
  if mountpoint -q "$MNT" 2>/dev/null; then
    umount "$MNT" || fail "$MNT is busy (a VMM still maps a snapshot); the vault is left up"
  fi
  if cryptsetup status "$NAME" >/dev/null 2>&1; then
    cryptsetup close "$NAME" || fail "mapping $NAME cannot be closed"
  fi
  detach_loops || fail "a loop device on $IMG cannot be detached"
  # The mapping is closed, so the file is keyless ciphertext; still, down
  # reports success only when it is gone (bunch 31, low).
  rm -f "$IMG" 2>/dev/null || true
  [ ! -e "$IMG" ] || fail "the mapping is closed but the backing file $IMG could not be removed"
  log "down: the vault key is gone and the backing file removed"
}

# --- status -----------------------------------------------------------------
STATUS_FAILS=0
st() { # description, result (0 = pass)
  if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; STATUS_FAILS=$((STATUS_FAILS + 1)); fi
}
chk() { # description, command...: one status line; a failing check is an answer
  local d="$1"; shift
  if "$@"; then st "$d" 0; else st "$d" 1; fi
}
perm_is() { [ -n "$(find "$1" -maxdepth 0 -perm "$2" 2>/dev/null || true)" ]; }
owned_by_owner() { [ -n "$(find "$1" -maxdepth 0 -user "$OWNER" 2>/dev/null || true)" ]; }
is_mount() { mountpoint -q "$MNT" 2>/dev/null; }
has_mapping() { [ -n "$(dm_node)" ]; }
# A here-string, not a pipe into grep -q: under pipefail an early grep exit
# could fail the pipeline although the line was found.
crypt_status_has() { local out; out="$(cryptsetup status "$NAME" 2>/dev/null || true)"; grep -q "$1" <<< "$out"; }
root_mode_ok() { perm_is "$MNT" 700 && owned_by_owner "$MNT"; }
img_mode_ok() { [ -e "$IMG" ] && perm_is "$IMG" 600; }

cmd_status() {
  need mountpoint cryptsetup readlink basename cat find grep
  chk "$MNT is a mount point" is_mount
  chk "mapping $NAME exists" has_mapping
  local uuid
  uuid="$(dm_uuid)"
  chk "the mapping's device-mapper uuid starts with $PLAIN_PREFIX (is '$uuid')" mapping_is_plain
  chk "the mount is on that mapping" mounted_is_ours
  chk "cryptsetup reports type PLAIN" crypt_status_has '^ *type: *PLAIN'
  chk "cipher aes-xts-plain64" crypt_status_has 'cipher: *aes-xts-plain64'
  chk "the mount root is $OWNER's, mode 0700" root_mode_ok
  chk "the backing file exists and is 0600" img_mode_ok
  if [ "$STATUS_FAILS" = 0 ]; then echo "snapshot vault: all checks passed"; return 0; fi
  echo "snapshot vault: $STATUS_FAILS check(s) failed"
  return 1
}

# --- scan -------------------------------------------------------------------
# Counts only, and zero only when every requested byte was read (finding B4):
# grep's status 2, a subtree find cannot enter, and a device node under a
# directory (not read) are read errors; any read error exits 1. A socket, fifo
# or symlink has no bytes of its own and is counted as skipped. An explicit
# path may be a regular file or a block device (the raw backing device).
SCAN_ERRORS=0
scan_file() { # canary, file -> 0 hit, 1 none; counts a read error
  local rc=0
  grep -a -q -F -f "$1" -- "$2" 2>/dev/null || rc=$?
  case $rc in
    0) return 0 ;;
    1) return 1 ;;
    *) err "cannot read $2"; SCAN_ERRORS=$((SCAN_ERRORS + 1)); return 1 ;;
  esac
}
cmd_scan() { # canary-file, paths...
  need grep find wc tr
  local canary="${1:-}"; shift || true
  [ -n "$canary" ] && [ -s "$canary" ] && [ "$#" -gt 0 ] || { err "usage: scan CANARY_FILE PATH..."; exit 2; }
  local path hits total=0 skipped=0 f listing rc n
  local before
  for path in "$@"; do
    hits=0
    before=$SCAN_ERRORS
    if [ -d "$path" ]; then
      # find's own status is the last line, so a traversal error is seen.
      listing="$(find "$path" -type f 2>/dev/null; echo "find-rc=$?")"
      rc="${listing##*find-rc=}"
      listing="${listing%find-rc=*}"
      if [ "$rc" != 0 ]; then
        err "find could not traverse all of $path"; SCAN_ERRORS=$((SCAN_ERRORS + 1))
      fi
      while IFS= read -r f; do
        [ -n "$f" ] || continue
        if scan_file "$canary" "$f"; then hits=$((hits + 1)); fi
      done <<< "$listing"
      n="$( { find "$path" \( -type b -o -type c \) 2>/dev/null || true; } | wc -l | tr -d ' ')"
      if [ "${n:-0}" -gt 0 ]; then
        err "$path holds $n device node(s) that were not read"; SCAN_ERRORS=$((SCAN_ERRORS + n))
      fi
      n="$( { find "$path" \( -type l -o -type p -o -type s \) 2>/dev/null || true; } | wc -l | tr -d ' ')"
      skipped=$((skipped + ${n:-0}))
    elif [ -f "$path" ] || [ -b "$path" ]; then
      if scan_file "$canary" "$path"; then hits=1; fi
    else
      err "$path is neither a file, a block device nor a directory"
      exit 1
    fi
    if [ "$SCAN_ERRORS" = "$before" ]; then
      echo "$path: hit: $hits"
    else
      echo "$path: incomplete, $((SCAN_ERRORS - before)) read error(s), hits among what was read: $hits"
    fi
    total=$((total + hits))
  done
  echo "files with a hit: $total"
  echo "skipped non-file entries: $skipped"
  echo "read errors: $SCAN_ERRORS"
  [ "$SCAN_ERRORS" = 0 ] || { err "the scan did not read every requested byte; its counts are not evidence"; exit 1; }
}

case "${1:-}" in
  up) cmd_up ;;
  down) cmd_down ;;
  status) cmd_status ;;
  scan) shift; cmd_scan "$@" ;;
  *) echo "usage: snapshot-vault.sh up|down|status|scan FILE PATH..." >&2; exit 2 ;;
esac
