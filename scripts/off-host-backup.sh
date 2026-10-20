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
# could be uploaded was, but an item was skipped or unreadable); 4 disk guard.
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

NODE_A_HOST="${OFFHOST_HOST:-}"
NODE_A_SSH_PORT="${OFFHOST_SSH_PORT:-22}"
NODE_A_SSH_USER="${OFFHOST_SSH_USER:-choir-backup}"
REMOTE_REST_PORT="${OFFHOST_REMOTE_REST_PORT:-8000}"
LOCAL_PORT="${OFFHOST_LOCAL_PORT:-18000}"
REST_USER="${OFFHOST_REST_USER:-nodeb}"
QUOTA_GIB="${OFFHOST_QUOTA_GIB:-450}"
UPLOAD_LIMIT_KIB="${OFFHOST_UPLOAD_LIMIT_KIB:-25000}" # about 200 Mbit/s; 0 = unlimited
MIN_FREE_GIB="${OFFHOST_MIN_FREE_GIB:-40}"
MAX_AGE_HOURS="${OFFHOST_MAX_AGE_HOURS:-36}"
MAX_VERIFY_AGE_HOURS="${OFFHOST_MAX_VERIFY_AGE_HOURS:-240}"
RESTIC_HOST_TAG="go-choir-b"

STATUS="$STATE/status.json"
STAGING="$STATE/staging"
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

# ---- status file ------------------------------------------------------
status_merge() { # JSON object on stdin merged into status.json atomically
  install -d -m 0700 "$STATE"
  local cur="{}"
  [ -s "$STATUS" ] && cur="$(cat "$STATUS")"
  printf '%s' "$cur" | jq -c --argjson new "$(cat)" '. * $new' > "$STATUS.tmp" && mv "$STATUS.tmp" "$STATUS"
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
  "$ROOT/mail/mail.db"            # captured consistently via sqlite3 .backup
  "$ROOT/mail/mail.db-wal"
  "$ROOT/mail/mail.db-shm"
)
resolve_paths() {
  PATHS=(); SKIPPED=(); REQUIRED_MISSING=0
  add_path "$ART" required
  add_path "$ROOT/platform-dolt/.choir-backup/platform" required
  add_path "$ROOT/corpus-dolt/.choir-backup/corpus" optional
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

# ---- the tunnel and restic environment ---------------------------------
CTL=""
tunnel_up() {
  [ -n "$NODE_A_HOST" ] || { err "OFFHOST_HOST is not set"; return 1; }
  install -d -m 0700 "$STATE/run"
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
# --limit-upload is a global restic flag; there is no environment variable.
R() { if [ "$UPLOAD_LIMIT_KIB" != 0 ]; then restic --limit-upload "$UPLOAD_LIMIT_KIB" "$@"; else restic "$@"; fi; }

# ---- staging: consistent copies of things that cannot be read live ------
dolt_sync() { # $1 port, $2 database, $3 target dir
  local port="$1" db="$2" dir="$3" pw home rc hash oldhome="${HOME:-}"
  install -d -m 0700 "$dir"
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
stage() {
  local ok=0 src
  install -d -m 0700 "$STAGING/sqlite" "$STAGING/vm-images" "$STATE/run"
  DOLT_HEADS=()
  dolt_sync 13306 platform "$ROOT/platform-dolt/.choir-backup/platform" || ok=1
  [ -d "$ROOT/corpus-dolt" ] && { dolt_sync 13307 corpus "$ROOT/corpus-dolt/.choir-backup/corpus" || ok=1; }
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
  for src in "$VM_STATE"/*/data.img; do
    [ -f "$src" ] || continue
    local id; id="$(basename "$(dirname "$src")")"
    install -d -m 0700 "$STAGING/vm-images/$id"
    if ! cp --reflink=always "$src" "$STAGING/vm-images/$id/data.img" 2>/dev/null; then
      err "reflink clone of a guest data image failed; it is not in this run"
      rm -rf "$STAGING/vm-images/$id"; ok=1
    fi
  done
  {
    printf '{"run_id":"%s","dolt_heads":"%s","restic":"%s","dolt":"%s"}\n' \
      "$RUN_ID" "${DOLT_HEADS[*]:-}" "$(restic version 2>/dev/null | head -1)" "$(dolt version 2>/dev/null | head -1)"
  } > "$STAGING/manifest.json"
  return "$ok"
}
cleanup_clones() { rm -rf "$STAGING/vm-images" 2>/dev/null || true; }

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
  resolve_paths
  echo "would back up:"; printf '  %s\n' "${PATHS[@]}"
  echo "excluded:";      printf '  %s\n' "${EXCLUDES[@]}"
  echo "absent:";        printf '  %s\n' "${SKIPPED[@]:-}"
  [ "$REQUIRED_MISSING" = 0 ] || { echo "a required path is absent"; return 3; }
}

cmd_backup() {
  local result=ok rc ex sid started added
  started="$(now)"
  trap 'cleanup_clones; tunnel_down' EXIT
  status_merge <<EOF
{"last_attempt_epoch":$started,"last_attempt_run":"$RUN_ID"}
EOF
  if ! disk_guard; then
    status_merge <<EOF
{"last_attempt_result":"disk-guard"}
EOF
    return 4
  fi
  tunnel_up || { status_merge <<<'{"last_attempt_result":"no-tunnel"}'; return 1; }
  restic_env daily
  R cat config >/dev/null 2>&1 || { err "repository is not initialised or unreachable (run: off-host-backup.sh init)"; status_merge <<<'{"last_attempt_result":"no-repo"}'; return 1; }
  stage || result=partial
  resolve_paths
  [ "$REQUIRED_MISSING" = 0 ] || { err "a required path is absent: ${SKIPPED[*]}"; result=partial; }
  local args=()
  for ex in "${EXCLUDES[@]}"; do args+=(--exclude "$ex"); done
  R backup --host "$RESTIC_HOST_TAG" --tag daily --tag "run-$RUN_ID" --json "${args[@]}" "${PATHS[@]}" \
    > "$STATE/run/backup.json" 2> "$STATE/run/backup.err"
  rc=$?
  if [ "$rc" -eq 3 ]; then result=partial; err "restic: some files were unreadable"
  elif [ "$rc" -ne 0 ]; then
    err "restic backup failed (exit $rc): $(tail -c 300 "$STATE/run/backup.err" | tr -d '\n')"
    status_merge <<<'{"last_attempt_result":"failed"}'
    return 1
  fi
  sid="$(jq -r 'select(.message_type=="summary") | .snapshot_id' "$STATE/run/backup.json" | tail -1)"
  added="$(jq -r 'select(.message_type=="summary") | .data_added' "$STATE/run/backup.json" | tail -1)"
  log "snapshot ${sid:-unknown} added ${added:-?} bytes"
  R check >/dev/null 2>"$STATE/run/check.err" || { err "restic check failed: $(tail -c 300 "$STATE/run/check.err" | tr -d '\n')"; status_merge <<<'{"last_attempt_result":"check-failed"}'; return 1; }
  local raw; raw="$(R stats --mode raw-data --json 2>/dev/null | jq -r '.total_size // 0')"
  local skipped_json; skipped_json="$(printf '%s\n' "${SKIPPED[@]:-}" | jq -R . | jq -sc 'map(select(length>0))')"
  status_merge <<EOF
{"last_attempt_result":"$result","last_snapshot_id":"${sid:-}","last_bytes_added":${added:-0},"repo_raw_bytes":${raw:-0},"skipped":$skipped_json,"duration_seconds":$(( $(now) - started ))}
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
  trap 'tunnel_down' EXIT
  tunnel_up || return 1
  restic_env daily
  week="$(date -u +%V)"; n=$(( (10#$week % 10) + 1 ))
  R check --read-data-subset="$n/10" >/dev/null 2>"$STATE/run/verify.err"; rc=$?
  if [ "$rc" -ne 0 ]; then
    err "verify $n/10 failed: $(tail -c 300 "$STATE/run/verify.err" | tr -d '\n')"
    status_merge <<<'{"last_verify_result":"failed"}'
    return 1
  fi
  log "verify $n/10 ok"
  status_merge <<EOF
{"last_verify_result":"ok","last_verify_epoch":$(now),"last_verify_subset":"$n/10"}
EOF
}

cmd_freshness() {
  local bad=0 t last first v raw
  [ -s "$STATUS" ] || { err "no status.json: the backup has never run"; return 1; }
  t="$(now)"
  last="$(jq -r '.last_success_epoch // 0' "$STATUS")"
  first="$(jq -r '.first_success_epoch // 0' "$STATUS")"
  v="$(jq -r '.last_verify_epoch // 0' "$STATUS")"
  raw="$(jq -r '.repo_raw_bytes // 0' "$STATUS")"
  if [ "$last" -eq 0 ] || [ $(( (t - last) / 3600 )) -gt "$MAX_AGE_HOURS" ]; then
    err "STALE: last successful backup $(( (t - last) / 3600 )) h ago (max $MAX_AGE_HOURS); last attempt: $(jq -r '.last_attempt_result // "none"' "$STATUS")"; bad=1
  fi
  case "$(jq -r '.last_attempt_result // "none"' "$STATUS")" in ok|none) ;; *) err "last attempt was '$(jq -r '.last_attempt_result' "$STATUS")'"; bad=1 ;; esac
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
  trap 'tunnel_down' EXIT
  tunnel_up || return 1
  restic_env archive
  R backup --host "$RESTIC_HOST_TAG" --tag archive --tag "$tag" "$@" || return 1
  R check --read-data || { err "archive verification failed; do NOT delete the local files"; return 1; }
  log "archive $tag stored and fully re-read; local deletion may proceed under the cleanup mission's own rules"
}

cmd_keygen() { # owner, interactive, once. Prints the restic password to the terminal only.
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
    printf '%s:%s\n' "$REST_USER" "$(openssl passwd -apr1 -stdin < "$SECRETS_DIR/rest-password")"
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
