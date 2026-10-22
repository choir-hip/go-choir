#!/usr/bin/env bash
# Off-host backup client for Node B (the node-a stopgap).
#
# Design and ruling: docs/problems/no-off-host-backup-2026-10-20.md
# (owner-notes-2026-10-14 section 27 item 17). Everything is encrypted by
# restic on this host with a repository password that never leaves it; the
# remote (node-a) holds ciphertext only, behind an append-only rest-server
# reached through a restricted SSH port forward.
#
# Usage:
#   off-host-backup.sh keygen      owner, interactive: create the secrets once
#   off-host-backup.sh init        owner: create the two restic repositories
#   off-host-backup.sh plan        print what would be backed up; no network
#   off-host-backup.sh backup      the daily run (systemd service)
#   off-host-backup.sh verify      weekly: restic check of a rotating subset
#   off-host-backup.sh freshness   every 6 h: fail if the backup is stale
#   off-host-backup.sh archive TAG PATH...   tape-cleanup archive (never pruned)
#
# Exit status: 0 ok; 1 failed or stale; 2 usage; 3 partial (everything that
# could be uploaded was, but an item was skipped or unreadable); 4 disk guard;
# 5 a tool the subcommand needs is not on PATH (nothing was done).
#
# Credentials: files named restic-password, rest-password, ssh-key, known-hosts
# and (for backup) dolt-root-password come from $CREDENTIALS_DIRECTORY
# (systemd LoadCredential=) or, for an owner-run shell, from
# $OFFHOST_SECRETS_DIR. They reach tools only through files and environment
# variables, never argv; nothing secret is echoed. There is no `set -x`.
set -u
set -o pipefail

ROOT="${OFFHOST_DATA_ROOT:-/var/lib/go-choir}"
STATE="${OFFHOST_STATE_DIR:-$ROOT/offhost-backup}"
SECRETS_DIR="${OFFHOST_SECRETS_DIR:-$ROOT/secrets/offhost-backup}"
DOLT_ROOT_PW_FILE="${OFFHOST_DOLT_ROOT_PASSWORD_FILE:-$ROOT/secrets/dolt-root-password}"
ART="$ROOT/platform-artifacts"
VM_STATE="$ROOT/vm-state"
# Computers whose guest data.img is cloned into the run, comma or space
# separated (the unit passes commas: systemd splits Environment= on spaces).
# Empty: every image. A computer rebuilds from its tape by design, so
# images are a recovery-time hedge for the computers that matter, not the
# tracers' test computers (problem record section 13 item 10).
IMAGE_COMPUTERS="${OFFHOST_IMAGE_COMPUTERS:-}"; IMAGE_COMPUTERS="${IMAGE_COMPUTERS//,/ }"

NODE_A_HOST="${OFFHOST_HOST:-}"
NODE_A_SSH_PORT="${OFFHOST_SSH_PORT:-22}"
NODE_A_SSH_USER="${OFFHOST_SSH_USER:-choir-backup}"
REMOTE_REST_PORT="${OFFHOST_REMOTE_REST_PORT:-8000}"
LOCAL_PORT="${OFFHOST_LOCAL_PORT:-18000}"
REST_USER="${OFFHOST_REST_USER:-nodeb}"
QUOTA_GIB="${OFFHOST_QUOTA_GIB:-450}"
UPLOAD_LIMIT_KIB="${OFFHOST_UPLOAD_LIMIT_KIB:-25000}" # about 200 Mbit/s; 0 = unlimited
MIN_FREE_GIB="${OFFHOST_MIN_FREE_GIB:-40}"
# Store A's memory line while its sync runs. DOLT_BACKUP runs in the
# server, and at the unit's own 3G line the sync stalled the server until
# the liveness probe aborted it (problem record section 13 item 15).
DOLT_SYNC_MEMORY_HIGH="${OFFHOST_DOLT_SYNC_MEMORY_HIGH:-8G}"
PLATFORM_DOLT_UNIT="${OFFHOST_PLATFORM_DOLT_UNIT:-go-choir-platform-dolt.service}"
# The Dolt servers' data directories (their working directories). When one
# is its own btrfs subvolume the store is captured as a read-only snapshot
# of it, atomic across the directory (what a power cut would leave, which
# Dolt's journal recovers), with no work in the server; otherwise the
# server-side sync above is the fallback (problem record section 13 item
# 16; scripts/dolt-data-to-subvolume.sh makes the subvolume).
PLATFORM_DOLT_DATA="${OFFHOST_PLATFORM_DOLT_DATA:-$ROOT/platform-dolt/platform}"
CORPUS_DOLT_DATA="${OFFHOST_CORPUS_DOLT_DATA:-$ROOT/corpus-dolt}"
MAX_AGE_HOURS="${OFFHOST_MAX_AGE_HOURS:-36}"
MAX_VERIFY_AGE_HOURS="${OFFHOST_MAX_VERIFY_AGE_HOURS:-240}"
RESTIC_HOST_TAG="go-choir-b"

STATUS="$STATE/status.json"
STAGING="$STATE/staging"
DOLT_SNAPS="$STAGING/dolt"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
now() { date -u +%s; }
log() { printf '%s offhost-backup: %s\n' "$(date -u +%FT%TZ)" "$*"; }
err() { printf '%s offhost-backup: ERROR: %s\n' "$(date -u +%FT%TZ)" "$*" >&2; }

cred() { # path of credential file $1
  if [ -n "${CREDENTIALS_DIRECTORY:-}" ] && [ -r "$CREDENTIALS_DIRECTORY/$1" ]; then
    printf '%s/%s' "$CREDENTIALS_DIRECTORY" "$1"
  elif [ "$1" = dolt-root-password ]; then
    printf '%s' "$DOLT_ROOT_PW_FILE"
  else
    printf '%s/%s' "$SECRETS_DIR" "$1"
  fi
}

# ---- tools -------------------------------------------------------------
# The systemd units carry every tool on PATH (offHostBackupEnv in
# nix/node-b.nix); a root shell on Node B does not. Each subcommand checks
# the tools it uses before it touches anything, so a hand run refuses
# instead of ending partial an hour later (problem record section 13 item 5).
NIX_SHELL_LINE="nix-shell -p restic jq openssh sqlite dolt apacheHttpd util-linux --run '...'"
need() { # tools...
  local t missing=()
  for t in "$@"; do command -v "$t" >/dev/null 2>&1 || missing+=("$t"); done
  [ "${#missing[@]}" = 0 ] && return 0
  err "not on PATH: ${missing[*]}; nothing was done. On Node B run it under: $NIX_SHELL_LINE"
  return 5
}

# ---- status file ------------------------------------------------------
# The daily backup and the weekly verify are separate units and can overlap,
# so every write is a read-modify-write under a lock beside the file, held
# only for the merge (never across restic), through a temp of its own. The
# wait has no timeout: the holder runs one jq on a small file, and the
# unit's TimeoutStartSec bounds a stuck one. With flock present a write is
# never unlocked.
status_merge() { # JSON object on stdin merged into status.json atomically
  install -d -m 0700 "$STATE"
  local new
  new="$(cat)" # read before the pipeline: its stages do not share this stdin
  if command -v flock >/dev/null 2>&1; then
    ( flock 9 || { err "could not take the status.json lock; status not written"; exit 1; }
      # Temps left by a writer killed mid-merge (SIGKILL runs no cleanup).
      find "$STATE" -maxdepth 1 -name "$(basename "$STATUS").??????" -mmin +60 -delete 2>/dev/null
      status_write "$new" ) 9>>"$STATUS.lock"
  else
    err "flock (util-linux) is not on PATH: status.json is written unlocked, and a backup and a verify that overlap can lose each other's marks"
    status_write "$new"
  fi
}
status_write() { # $1 JSON object
  local cur="{}" tmp
  [ -s "$STATUS" ] && cur="$(cat "$STATUS")"
  tmp="$(mktemp "$STATUS.XXXXXX")" || return 1
  if printf '%s' "$cur" | jq -c --argjson new "$1" '. * $new' > "$tmp" && mv "$tmp" "$STATUS"; then
    return 0
  fi
  rm -f "$tmp"; return 1
}

# ---- attempt marks -----------------------------------------------------
# A run is marked running after its tool preflight and before the disk
# guard, the tunnel or any restic call, and every way out of it
# leaves a result: the normal paths write theirs, the EXIT trap turns a
# running mark of this run into failed (a signal, or set -u), and a SIGKILL
# of bash itself leaves running, which the freshness check reads against
# the recorded pid. With no swap a cgroup OOM kill is a designed failure,
# so it must read as one (problem record offhost-backup-reads-terabytes-
# and-needs-swap-2026-10-22, review round 2).
ATTEMPT_KIND="" # attempt (the daily backup) or verify
SIGNALLED=""
attempt_start() { # $1 attempt|verify; extra JSON fields may follow in $2
  ATTEMPT_KIND="$1"
  trap 'SIGNALLED=TERM; exit 143' TERM
  trap 'SIGNALLED=INT; exit 130' INT
  trap 'SIGNALLED=HUP; exit 129' HUP
  status_merge <<EOF
{"last_${1}_result":"running","last_${1}_run":"$RUN_ID","last_${1}_pid":$$,"last_${1}_cause":""${2:+,$2}}
EOF
}
attempt_end() { # $1 result, $2 cause (may be empty)
  [ -n "$ATTEMPT_KIND" ] || return 0
  local cause; cause="$(printf '%s' "${2:-}" | jq -R .)"
  status_merge <<EOF
{"last_${ATTEMPT_KIND}_result":"$1","last_${ATTEMPT_KIND}_cause":$cause}
EOF
}
attempt_interrupted() { # EXIT trap: a running mark of this run becomes failed
  [ -n "$ATTEMPT_KIND" ] && [ -s "$STATUS" ] || return 0
  local k="$ATTEMPT_KIND" cur run
  cur="$(jq -r --arg k "last_${k}_result" '.[$k] // ""' "$STATUS" 2>/dev/null)"
  run="$(jq -r --arg k "last_${k}_run" '.[$k] // ""' "$STATUS" 2>/dev/null)"
  [ "$cur" = running ] && [ "$run" = "$RUN_ID" ] || return 0
  attempt_end failed "interrupted${SIGNALLED:+ by SIG$SIGNALLED} before a result was written"
}
exit_cause() { # $1 exit status of a tool, $2 tool name
  if [ "$1" -gt 128 ]; then
    printf '%s killed by signal %s%s' "$2" "$(( $1 - 128 ))" "$([ "$1" = 137 ] && echo ' (KILL: a cgroup OOM kill sends this)')"
  else
    printf '%s exit %s' "$2" "$1"
  fi
}

# ---- the set of paths -------------------------------------------------
PATHS=()
SKIPPED=()
REQUIRED_MISSING=0
add_path() { # $1 path, $2 required|optional
  if [ -e "$1" ]; then
    PATHS+=("$1")
  else
    SKIPPED+=("$1")
    [ "${2:-optional}" = required ] && REQUIRED_MISSING=1
  fi
}
EXCLUDES=(
  "$ART/sha256/projection-base"   # 47 GB, rebuilt from the tape
  "$ART/checkpoint-scratch"       # regenerable scratch
  "$ROOT/secrets/offhost-backup"  # holds this backup's own key
  "key-escrow-kek*"                # opens the sealed escrow key; the owner keeps it offline (FP9)
  "$ROOT/mail/mail.db"            # captured consistently via sqlite3 .backup
  "$ROOT/mail/mail.db-wal"
  "$ROOT/mail/mail.db-shm"
  "$DOLT_SNAPS/corpus/.choir-backup" # Store B's old sync copy, inside its data directory
  "$DOLT_SNAPS/**/sql-server.info"   # the live server's lock note; a restore starts without it
)
# The escrow KEK the units actually load (finding B3): the source file of a
# key-escrow-kek* credential and an absolute CHOIR_KEY_ESCROW_KEK_PATH, read
# from the units, are excluded whatever their file names. The name pattern
# above covers the documented layout; this covers the configured one. A
# host without systemctl adds nothing (development).
KEK_UNITS="${OFFHOST_KEK_UNITS:-go-choir-corpusd.service go-choir-checkpointd.service}"
kek_sources() { # prints one path per line
  command -v systemctl >/dev/null 2>&1 || return 0
  local unit line tok
  for unit in $KEK_UNITS; do
    systemctl show -p LoadCredential -p LoadCredentialEncrypted -p Environment "$unit" 2>/dev/null |
    while IFS= read -r line; do
      case "$line" in
        LoadCredential=key-escrow-kek*:/*|LoadCredentialEncrypted=key-escrow-kek*:/*)
          printf '%s\n' "${line#*:}" ;;
        Environment=*)
          for tok in ${line#Environment=}; do
            case "$tok" in CHOIR_KEY_ESCROW_KEK_PATH=/*) printf '%s\n' "${tok#*=}" ;; esac
          done ;;
      esac
    done
  done
}

resolve_paths() {
  PATHS=(); SKIPPED=(); REQUIRED_MISSING=0
  local kek
  while IFS= read -r kek; do
    [ -n "$kek" ] && EXCLUDES+=("$kek")
  done < <(kek_sources)
  add_path "$ART" required
  # A snapshotted store travels in $STAGING; its old sync copy is not read.
  is_subvolume "$PLATFORM_DOLT_DATA" || add_path "$ROOT/platform-dolt/.choir-backup/platform" required
  is_subvolume "$CORPUS_DOLT_DATA" || add_path "$ROOT/corpus-dolt/.choir-backup/corpus" optional
  add_path "$ROOT/secrets" required
  add_path "$ROOT/auth-signing" optional
  add_path "$ROOT/gateway-provider.env" optional
  add_path "$ROOT/codex-auth.json" optional
  add_path "$ROOT/maild.env" optional
  add_path "$ROOT/searxng.env" optional
  add_path "$ROOT/gateway-identities.json" optional
  add_path "$ROOT/corpus-dsn.env" optional
  add_path "$ROOT/deploy.env" optional
  add_path "$ROOT/sandbox-gateway-token.env" optional
  add_path "$ROOT/mail" optional
  add_path "$ROOT/files" optional
  add_path "$VM_STATE/ownerships.json" optional
  add_path "$STAGING" optional # sqlite copies, reflink image clones, manifest
}

# ---- guest images ------------------------------------------------------
IMAGES=()
IMAGE_MISSING=()
IMAGE_ERR=""
select_images() { # fills IMAGES (data.img paths), IMAGE_MISSING, IMAGE_ERR
  IMAGES=(); IMAGE_MISSING=(); IMAGE_ERR=""
  local src c vm found vms
  if [ -z "$IMAGE_COMPUTERS" ]; then
    for src in "$VM_STATE"/*/data.img; do [ -f "$src" ] && IMAGES+=("$src"); done
    return 0
  fi
  if ! vms="$(jq -r '.ownerships[] | [.computer_id, .vm_id] | @tsv' "$VM_STATE/ownerships.json" 2>/dev/null)"; then
    IMAGE_ERR="$VM_STATE/ownerships.json is unreadable; no guest image selected"
    for c in $IMAGE_COMPUTERS; do IMAGE_MISSING+=("$c"); done
    return 1
  fi
  for c in $IMAGE_COMPUTERS; do
    found=0
    while IFS=$'\t' read -r cid vm; do
      [ "$cid" = "$c" ] || continue
      case "$vm" in ""|*/*|.*) continue ;; esac
      [ -f "$VM_STATE/$vm/data.img" ] && { IMAGES+=("$VM_STATE/$vm/data.img"); found=1; }
    done <<<"$vms"
    [ "$found" = 1 ] || IMAGE_MISSING+=("$c")
  done
  [ "${#IMAGE_MISSING[@]}" = 0 ]
}

# ---- the tunnel and restic environment ---------------------------------
CTL=""
tunnel_up() {
  install -d -m 0700 "$STATE/run"
  # Test hook: a rest-server already listening on OFFHOST_LOCAL_PORT (the
  # local drill and the harness use it; production never sets this).
  [ "${OFFHOST_NO_TUNNEL:-}" = 1 ] && return 0
  [ -n "$NODE_A_HOST" ] || { err "OFFHOST_HOST is not set"; return 1; }
  CTL="$STATE/run/ssh.ctl"
  ssh -i "$(cred ssh-key)" -o IdentitiesOnly=yes -o BatchMode=yes \
    -o UserKnownHostsFile="$(cred known-hosts)" -o StrictHostKeyChecking=yes \
    -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=6 \
    -o ControlMaster=yes -o ControlPath="$CTL" \
    -p "$NODE_A_SSH_PORT" -f -N \
    -L "127.0.0.1:$LOCAL_PORT:127.0.0.1:$REMOTE_REST_PORT" \
    "$NODE_A_SSH_USER@$NODE_A_HOST" || { err "ssh tunnel to node-a failed"; return 1; }
  local i
  for i in $(seq 1 20); do
    (exec 3<>"/dev/tcp/127.0.0.1/$LOCAL_PORT") 2>/dev/null && return 0
    sleep 1
  done
  err "tunnel is up but 127.0.0.1:$LOCAL_PORT does not answer"
  return 1
}
tunnel_down() {
  [ -n "$CTL" ] && ssh -O exit -o ControlPath="$CTL" "$NODE_A_SSH_USER@$NODE_A_HOST" >/dev/null 2>&1
  CTL=""
}
restic_env() { # $1 = daily | archive
  export RESTIC_PASSWORD_FILE
  RESTIC_PASSWORD_FILE="$(cred restic-password)"
  export RESTIC_REST_USERNAME="$REST_USER"
  RESTIC_REST_PASSWORD="$(cat "$(cred rest-password)")"; export RESTIC_REST_PASSWORD
  export RESTIC_REPOSITORY="rest:http://127.0.0.1:$LOCAL_PORT/$REST_USER/$1"
  export RESTIC_CACHE_DIR="$STATE/restic-cache"
  export RESTIC_COMPRESSION=auto RESTIC_PACK_SIZE=64 RESTIC_READ_CONCURRENCY=2
  export GOGC=50
}
# The repository's stored bytes, as the sum of the pack sizes the
# rest-server lists (REST API v2). `restic stats --mode raw-data` walks every
# snapshot's trees and held the unit at its memory line (problem doc section
# 13 item 19). The password goes to curl on stdin, never on its command line.
repo_pack_bytes() { # $1 = daily | archive
  printf 'user = "%s:%s"\n' "$REST_USER" "$RESTIC_REST_PASSWORD" |
    curl -fsS --max-time 120 -K - -H 'Accept: application/vnd.x.restic.rest.v2' \
      "http://127.0.0.1:$LOCAL_PORT/$REST_USER/$1/data/" 2>/dev/null |
    jq -r '[.[].size] | add // 0' 2>/dev/null || echo 0
}
# --limit-upload is a global restic flag; there is no environment variable.
R() { if [ "$UPLOAD_LIMIT_KIB" != 0 ]; then restic --limit-upload "$UPLOAD_LIMIT_KIB" "$@"; else restic "$@"; fi; }

# ---- staging: consistent copies of things that cannot be read live ------
dolt_sync() { # $1 port, $2 database, $3 target dir
  local port="$1" db="$2" dir="$3" pw home rc hash oldhome="${HOME:-}"
  install -d -m 0700 "$dir"
  record_rows "$db" "$port" "$db"
  home="$(mktemp -d)"; export HOME="$home"
  pw="$(cat "$(cred dolt-root-password)" 2>/dev/null || true)"
  case "$pw" in *[!A-Za-z0-9]*) pw="" ;; esac
  # Same rule as go-choir-dolt-ensure-users: root with its password, or
  # passwordless root while the staged rollout still allows it. The app
  # user is refused DOLT_BACKUP ("command denied"), so root is required.
  if [ -n "$pw" ] && printf 'select 1;\n' | DOLT_CLI_PASSWORD="$pw" dolt --host 127.0.0.1 --port "$port" --no-tls --user root sql >/dev/null 2>&1; then
    DOLT_CLI_PASSWORD="$pw" dolt --host 127.0.0.1 --port "$port" --no-tls --user root --use-db "$db" \
      sql -q "CALL DOLT_BACKUP('sync-url','file://$dir')" >/dev/null 2>"$STATE/run/dolt-$db.err"
    rc=$?
    hash="$(printf 'select commit_hash from dolt_log limit 1;\n' | DOLT_CLI_PASSWORD="$pw" dolt --host 127.0.0.1 --port "$port" --no-tls --user root --use-db "$db" sql -r csv 2>/dev/null | tail -1)"
  else
    dolt --host 127.0.0.1 --port "$port" --no-tls --user root --password "" --use-db "$db" \
      sql -q "CALL DOLT_BACKUP('sync-url','file://$dir')" >/dev/null 2>"$STATE/run/dolt-$db.err"
    rc=$?
    hash="$(printf 'select commit_hash from dolt_log limit 1;\n' | dolt --host 127.0.0.1 --port "$port" --no-tls --user root --password "" --use-db "$db" sql -r csv 2>/dev/null | tail -1)"
  fi
  rm -rf "$home"; export HOME="$oldhome"
  if [ "$rc" -ne 0 ]; then
    err "dolt backup of $db failed (client stderr class: $(head -c 120 "$STATE/run/dolt-$db.err" | tr -d '\n' | sed -e 's/password[^ ]*/password=?/Ig'))"
    return 1
  fi
  DOLT_HEADS+=("$db=$hash")
  log "dolt backup $db synced (head $hash)"
}
is_subvolume() { command -v btrfs >/dev/null 2>&1 && btrfs subvolume show "$1" >/dev/null 2>&1; }
drop_snapshot() { # $1 path; a read-only snapshot needs btrfs to delete it
  [ -e "$1" ] || return 0
  btrfs subvolume delete "$1" >/dev/null 2>&1 || { err "could not delete snapshot $1"; return 1; }
}
dolt_scalar() { # $1 port, $2 database, $3 query: one value, as root with its password or passwordless
  local port="$1" db="$2" q="$3" pw home oldhome="${HOME:-}" out
  home="$(mktemp -d)"; export HOME="$home"
  pw="$(cat "$(cred dolt-root-password)" 2>/dev/null || true)"
  case "$pw" in *[!A-Za-z0-9]*) pw="" ;; esac
  if [ -n "$pw" ] && printf 'select 1;\n' | DOLT_CLI_PASSWORD="$pw" dolt --host 127.0.0.1 --port "$port" --no-tls --user root sql >/dev/null 2>&1; then
    out="$(DOLT_CLI_PASSWORD="$pw" dolt --host 127.0.0.1 --port "$port" --no-tls --user root --use-db "$db" sql -r csv -q "$q" 2>/dev/null | tail -1)"
  else
    out="$(dolt --host 127.0.0.1 --port "$port" --no-tls --user root --password "" --use-db "$db" sql -r csv -q "$q" 2>/dev/null | tail -1)"
  fi
  rm -rf "$home"; export HOME="$oldhome"
  printf '%s' "$out"
}
dolt_head() { dolt_scalar "$1" "$2" 'select commit_hash from dolt_log limit 1'; } # the newest commit
# Store A's freshness marker. With PLATFORM_DOLT_SNAPSHOTS=off its newest
# commit no longer moves, so the head alone cannot show a restore is recent;
# the event receipt count only grows. Read before the capture, so the capture
# holds at least this many (docs/problems/platform-dolt-oldgen-rate-and-
# squash-window-2026-10-22.md, review round 1 R2).
record_rows() { # $1 name, $2 port, $3 database
  [ "$3" = platform ] || return 0
  local n; n="$(dolt_scalar "$2" "$3" 'select count(*) from computer_event_append_receipts')"
  case "$n" in ''|*[!0-9]*) log "dolt $1: receipt count unread"; return 0 ;; esac
  DOLT_ROWS+=("$1=$n")
}
snapshot_store() { # $1 name, $2 port, $3 database, $4 data directory (a subvolume)
  local name="$1" dst="$DOLT_SNAPS/$1" hash
  install -d -m 0700 "$DOLT_SNAPS" "$STATE/run"
  drop_snapshot "$dst" || return 1
  # The head and the receipt count are read first: the snapshot holds them
  # or later state.
  hash="$(dolt_head "$2" "$3")"
  record_rows "$name" "$2" "$3"
  if ! btrfs subvolume snapshot -r "$4" "$dst" >/dev/null 2>"$STATE/run/snapshot-$name.err"; then
    err "btrfs snapshot of $name failed: $(head -c 200 "$STATE/run/snapshot-$name.err" | tr -d '\n')"
    drop_snapshot "$dst"; return 1
  fi
  DOLT_HEADS+=("$name=$hash")
  log "dolt $name captured as a read-only snapshot (head before it: ${hash:-unread})"
}
cleanup_snapshots() { local s; for s in "$DOLT_SNAPS"/*; do [ -e "$s" ] && drop_snapshot "$s"; done; return 0; }

RAISED_UNIT=""
SAVED_MEMORY_HIGH=""
raise_dolt_memory() { # $1 unit: raise its MemoryHigh for the sync, remembering the old value
  local cur
  cur="$(systemctl show -p MemoryHigh --value "$1" 2>/dev/null)" || return 1
  [ -n "$cur" ] || return 1
  systemctl set-property --runtime "$1" "MemoryHigh=$DOLT_SYNC_MEMORY_HIGH" || return 1
  RAISED_UNIT="$1"; SAVED_MEMORY_HIGH="$cur"
  log "raised $1 MemoryHigh from $cur to $DOLT_SYNC_MEMORY_HIGH for the sync"
}
restore_dolt_memory() { # idempotent; also called from the exit trap
  [ -n "$RAISED_UNIT" ] || return 0
  if systemctl set-property --runtime "$RAISED_UNIT" "MemoryHigh=$SAVED_MEMORY_HIGH"; then
    log "restored $RAISED_UNIT MemoryHigh to $SAVED_MEMORY_HIGH"
  else
    err "could not restore $RAISED_UNIT MemoryHigh to $SAVED_MEMORY_HIGH; set it by hand (systemctl set-property --runtime)"
  fi
  RAISED_UNIT=""
}
stage() {
  local ok=0 src
  install -d -m 0700 "$STAGING/sqlite" "$STAGING/vm-images" "$STATE/run"
  DOLT_HEADS=()
  DOLT_ROWS=()
  if is_subvolume "$PLATFORM_DOLT_DATA"; then
    snapshot_store platform 13306 platform "$PLATFORM_DOLT_DATA" || ok=1
  elif raise_dolt_memory "$PLATFORM_DOLT_UNIT"; then
    dolt_sync 13306 platform "$ROOT/platform-dolt/.choir-backup/platform" || ok=1
    restore_dolt_memory
  else
    err "could not raise $PLATFORM_DOLT_UNIT MemoryHigh; platform Dolt sync skipped (at the unit's own line the sync stalled Store A)"; ok=1
  fi
  if is_subvolume "$CORPUS_DOLT_DATA"; then
    snapshot_store corpus 13307 corpus "$CORPUS_DOLT_DATA" || ok=1
  elif [ -d "$ROOT/corpus-dolt" ]; then
    dolt_sync 13307 corpus "$ROOT/corpus-dolt/.choir-backup/corpus" || ok=1
  fi
  for src in "$ROOT/auth/auth.db" "$ROOT/mail/mail.db"; do
    [ -f "$src" ] || continue
    if sqlite3 "$src" ".backup '$STAGING/sqlite/$(basename "$src")'"; then
      log "sqlite backup $(basename "$src") ok"
    else
      err "sqlite backup of $(basename "$src") failed"; ok=1
    fi
  done
  # Reflink clones are atomic per file: a crash-consistent image. They pin
  # copy-on-write extents while they exist; the EXIT trap removes them.
  rm -rf "$STAGING/vm-images"; install -d -m 0700 "$STAGING/vm-images"
  if ! select_images; then
    [ -n "$IMAGE_ERR" ] && err "$IMAGE_ERR"
    err "guest image missing for: ${IMAGE_MISSING[*]}"; ok=1
  fi
  for src in "${IMAGES[@]}"; do
    local id; id="$(basename "$(dirname "$src")")"
    install -d -m 0700 "$STAGING/vm-images/$id"
    if ! cp --reflink=always "$src" "$STAGING/vm-images/$id/data.img" 2>/dev/null; then
      err "reflink clone of a guest data image failed; it is not in this run"
      rm -rf "$STAGING/vm-images/$id"; ok=1
    fi
  done
  {
    printf '{"run_id":"%s","dolt_heads":"%s","dolt_rows":"%s","restic":"%s","dolt":"%s"}\n' \
      "$RUN_ID" "${DOLT_HEADS[*]:-}" "${DOLT_ROWS[*]:-}" "$(restic version 2>/dev/null | head -1)" "$(dolt version 2>/dev/null | head -1)"
  } > "$STAGING/manifest.json"
  return "$ok"
}
cleanup_clones() { rm -rf "$STAGING/vm-images" 2>/dev/null || true; cleanup_snapshots; }

disk_guard() {
  local free_kib
  free_kib="$(df -Pk "$ROOT" | awk 'NR==2{print $4}')"
  if [ "$((free_kib / 1048576))" -lt "$MIN_FREE_GIB" ]; then
    err "disk guard: $((free_kib / 1048576)) GiB free on $ROOT, need $MIN_FREE_GIB; the Dolt backup copies and image clones need room"
    return 1
  fi
}

# ---- subcommands -------------------------------------------------------
cmd_plan() {
  [ -z "$IMAGE_COMPUTERS" ] || need jq || return
  local rc=0
  resolve_paths
  echo "would back up:"; printf '  %s\n' "${PATHS[@]}"
  echo "excluded:";      printf '  %s\n' "${EXCLUDES[@]}"
  echo "absent:";        printf '  %s\n' "${SKIPPED[@]:-}"
  select_images || rc=3
  echo "guest images: ${IMAGE_COMPUTERS:-every computer}"; printf '  %s\n' "${IMAGES[@]:-}"
  echo "missing images:"; printf '  %s\n' "${IMAGE_MISSING[@]:-}"
  [ -z "$IMAGE_ERR" ] || echo "$IMAGE_ERR"
  [ "$REQUIRED_MISSING" = 0 ] || { echo "a required path is absent"; return 3; }
  return "$rc"
}

cmd_backup() {
  local result=ok rc ex sid started added
  need ssh restic jq sqlite3 dolt systemctl || return
  started="$(now)"
  trap 'restore_dolt_memory; cleanup_clones; tunnel_down; attempt_interrupted' EXIT
  attempt_start attempt "\"last_attempt_epoch\":$started"
  if ! disk_guard; then
    attempt_end disk-guard ""
    return 4
  fi
  tunnel_up || { attempt_end no-tunnel ""; return 1; }
  restic_env daily
  # A run killed mid-command loses its tunnel in the same instant, so restic
  # cannot delete its lock and the next run is refused (item 18, run 5).
  # unlock removes only locks whose process is gone.
  R unlock >/dev/null 2>&1 || true
  R cat config >/dev/null 2>&1 || { err "repository is not initialised or unreachable (run: off-host-backup.sh init)"; attempt_end no-repo ""; return 1; }
  stage || result=partial
  resolve_paths
  [ "$REQUIRED_MISSING" = 0 ] || { err "a required path is absent: ${SKIPPED[*]}"; result=partial; }
  local args=()
  for ex in "${EXCLUDES[@]}"; do args+=(--exclude "$ex"); done
  # --no-scan: the scanner only feeds progress totals, and walks and
  # lstats all 9.7M files a second time beside the backup's own walk.
  R backup --host "$RESTIC_HOST_TAG" --tag daily --tag "run-$RUN_ID" --json --no-scan "${args[@]}" "${PATHS[@]}" \
    > "$STATE/run/backup.json" 2> "$STATE/run/backup.err"
  rc=$?
  if [ "$rc" -eq 3 ]; then result=partial; err "restic: some files were unreadable"
  elif [ "$rc" -ne 0 ]; then
    err "restic backup failed ($(exit_cause "$rc" restic)): $(tail -c 300 "$STATE/run/backup.err" | tr -d '\n')"
    attempt_end failed "$(exit_cause "$rc" restic)"
    return 1
  fi
  sid="$(jq -r 'select(.message_type=="summary") | .snapshot_id' "$STATE/run/backup.json" | tail -1)"
  added="$(jq -r 'select(.message_type=="summary") | .data_added' "$STATE/run/backup.json" | tail -1)"
  log "snapshot ${sid:-unknown} added ${added:-?} bytes"
  # No `restic check` here: walking every tree of a 9.7 million file
  # source runs past the unit's memory line for hours (problem doc section
  # 13 item 18). The weekly verify carries the structural check.
  local raw; raw="$(repo_pack_bytes daily)"
  local skipped_json; skipped_json="$(printf '%s\n' "${SKIPPED[@]:-}" | jq -R . | jq -sc 'map(select(length>0))')"
  status_merge <<EOF
{"last_attempt_result":"$result","last_attempt_cause":"","last_snapshot_id":"${sid:-}","last_bytes_added":${added:-0},"repo_raw_bytes":${raw:-0},"skipped":$skipped_json,"duration_seconds":$(( $(now) - started ))}
EOF
  if [ "$result" = ok ]; then
    local first; first="$(jq -r '.first_success_epoch // empty' "$STATUS" 2>/dev/null)"
    [ -n "$first" ] || first="$(now)"
    status_merge <<EOF
{"last_success_epoch":$(now),"first_success_epoch":$first}
EOF
    return 0
  fi
  return 3
}

cmd_verify() {
  local week n rc
  need ssh restic jq || return
  trap 'tunnel_down; attempt_interrupted' EXIT
  install -d -m 0700 "$STATE/run"
  attempt_start verify
  tunnel_up || { attempt_end failed "no tunnel"; return 1; }
  restic_env daily
  week="$(date -u +%V)"; n=$(( (10#$week % 10) + 1 ))
  # Two connections: restic's tree loaders are connections + GOMAXPROCS
  # + 1 (the unit sets GOMAXPROCS=2), each holding a decoded tree; trees
  # over 50 MiB go to one loader of their own (restic 0.19.1
  # internal/data/tree_stream.go, StreamTrees).
  R -o rest.connections=2 check --with-cache --read-data-subset="$n/10" >/dev/null 2>"$STATE/run/verify.err"; rc=$?
  if [ "$rc" -ne 0 ]; then
    err "verify $n/10 failed ($(exit_cause "$rc" restic)): $(tail -c 300 "$STATE/run/verify.err" | tr -d '\n')"
    attempt_end failed "$(exit_cause "$rc" restic)"
    return 1
  fi
  log "verify $n/10 ok"
  status_merge <<EOF
{"last_verify_result":"ok","last_verify_cause":"","last_verify_epoch":$(now),"last_verify_subset":"$n/10"}
EOF
}

result_fresh() { # $1 attempt|verify, $2 label: 1 when the last result is not good
  local r cause pid
  r="$(jq -r --arg k "last_${1}_result" '.[$k] // "none"' "$STATUS")"
  cause="$(jq -r --arg k "last_${1}_cause" '.[$k] // ""' "$STATUS")"
  case "$r" in
    ok|none) return 0 ;;
    running)
      pid="$(jq -r --arg k "last_${1}_pid" '.[$k] // 0' "$STATUS")"
      if [ "$pid" -gt 0 ] 2>/dev/null && { tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || ps -p "$pid" -o args= 2>/dev/null; } | grep -q 'off-\?host-backup'; then
        log "a $2 run is in progress (pid $pid)"; return 0
      fi
      err "last $2 was interrupted: marked running, and its process ($pid) is gone"; return 1 ;;
    *) err "last $2 was '$r'${cause:+ ($cause)}"; return 1 ;;
  esac
}

cmd_freshness() {
  local bad=0 t last first v raw
  need jq || return
  [ -s "$STATUS" ] || { err "no status.json: the backup has never run"; return 1; }
  t="$(now)"
  last="$(jq -r '.last_success_epoch // 0' "$STATUS")"
  first="$(jq -r '.first_success_epoch // 0' "$STATUS")"
  v="$(jq -r '.last_verify_epoch // 0' "$STATUS")"
  raw="$(jq -r '.repo_raw_bytes // 0' "$STATUS")"
  if [ "$last" -eq 0 ] || [ $(( (t - last) / 3600 )) -gt "$MAX_AGE_HOURS" ]; then
    err "STALE: last successful backup $(( (t - last) / 3600 )) h ago (max $MAX_AGE_HOURS); last attempt: $(jq -r '.last_attempt_result // "none"' "$STATUS")"; bad=1
  fi
  result_fresh attempt backup || bad=1
  result_fresh verify verify || bad=1
  if [ "$first" -gt 0 ] && [ $(( (t - first) / 3600 )) -gt "$MAX_VERIFY_AGE_HOURS" ] && { [ "$v" -eq 0 ] || [ $(( (t - v) / 3600 )) -gt "$MAX_VERIFY_AGE_HOURS" ]; }; then
    err "no passing verify in $MAX_VERIFY_AGE_HOURS h"; bad=1
  fi
  if [ "$raw" -gt $(( QUOTA_GIB * 1073741824 / 100 * 80 )) ]; then
    err "QUOTA: repository is $(( raw / 1073741824 )) GiB, over 80% of the $QUOTA_GIB GiB quota on node-a"; bad=1
  fi
  [ "$bad" = 0 ] && log "fresh: last success $(( (t - last) / 60 )) min ago, repo $(( raw / 1073741824 )) GiB of $QUOTA_GIB"
  return "$bad"
}

cmd_init() {
  need ssh restic || return
  trap 'tunnel_down' EXIT
  tunnel_up || return 1
  local repo
  for repo in daily archive; do
    restic_env "$repo"
    if R cat config >/dev/null 2>&1; then log "$repo repository already initialised"
    else R init --repository-version 2 && log "$repo repository initialised"; fi
  done
}

cmd_archive() { # TAG PATH...: tape-cleanup archive; verifies by re-reading everything
  local tag="${1:-}"; shift || true
  [ -n "$tag" ] && [ "$#" -gt 0 ] || { echo "usage: archive TAG PATH..." >&2; return 2; }
  need ssh restic || return
  trap 'tunnel_down' EXIT
  tunnel_up || return 1
  restic_env archive
  R backup --host "$RESTIC_HOST_TAG" --tag archive --tag "$tag" "$@" || return 1
  R check --read-data || { err "archive verification failed; do NOT delete the local files"; return 1; }
  log "archive $tag stored and fully re-read; local deletion may proceed under the cleanup mission's own rules"
}

cmd_keygen() { # owner, interactive, once. Prints the restic password to the terminal only.
  # htpasswd is not installed on Node B; nix-shell supplies it. Checked
  # before any key is written: a key without its htpasswd line makes a
  # rerun refuse (the keys exist) while node-a still cannot authenticate.
  need ssh-keygen base64 || return
  command -v htpasswd >/dev/null 2>&1 || command -v nix-shell >/dev/null 2>&1 || need htpasswd || return
  [ -t 1 ] && [ -e /dev/tty ] || { err "keygen prints the password once and needs a terminal"; return 2; }
  install -d -m 0700 "$SECRETS_DIR"
  umask 077
  local f
  for f in restic-password rest-password ssh-key; do
    [ -e "$SECRETS_DIR/$f" ] && { err "$SECRETS_DIR/$f exists; refusing to overwrite a key"; return 2; }
  done
  head -c 32 /dev/urandom | base64 | tr -d '\n' > "$SECRETS_DIR/restic-password"
  head -c 32 /dev/urandom | base64 | tr -d '\n=/+' > "$SECRETS_DIR/rest-password"
  ssh-keygen -q -t ed25519 -N "" -C "node-b-offhost-backup" -f "$SECRETS_DIR/ssh-key"
  {
    echo "== COPY THIS TO THE OWNER'S OFFLINE MEDIUM NOW. It is shown once. =="
    echo "restic repository password:"
    cat "$SECRETS_DIR/restic-password"; echo
    echo
    echo "== Give node-a's agent these two lines (neither is secret) =="
    echo "ssh public key:"; cat "$SECRETS_DIR/ssh-key.pub"
    echo "htpasswd line:"
    # bcrypt (apache htpasswd -B): rest-server did not accept apr1/md5 hashes
    # when tested; bcrypt and {SHA} entries authenticate. -i reads stdin.
    if command -v htpasswd >/dev/null 2>&1; then
      htpasswd -niB "$REST_USER" < "$SECRETS_DIR/rest-password"
    else
      REST_USER="$REST_USER" nix-shell -p apacheHttpd --run 'htpasswd -niB "$REST_USER"' < "$SECRETS_DIR/rest-password"
    fi
    echo
    echo "Still to place by hand: $SECRETS_DIR/known-hosts (node-a's host key, verified out of band)."
  } > /dev/tty
}

case "${1:-}" in
  keygen) shift; cmd_keygen "$@" ;;
  init) shift; cmd_init "$@" ;;
  plan) shift; cmd_plan "$@" ;;
  backup) shift; cmd_backup "$@" ;;
  verify) shift; cmd_verify "$@" ;;
  freshness) shift; cmd_freshness "$@" ;;
  archive) shift; cmd_archive "$@" ;;
  *) sed -n '2,24p' "$0" >&2; exit 2 ;;
esac
