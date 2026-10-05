#!/usr/bin/env bash
# S2 station acceptance probe — runs ON NODE B (root SSH).
# Contract-class probes only: every release is choir-builder output from a
# real commit; every apply path is the real mint + signed offer + guest apply.
# Legs:
#   pre     snapshot disposable state (fc pid, boot_id, head, route, served commit)
#   neg:<case> base-digest | stale-head | content-mutation | provenance | absent-entrypoint | base-commit | schema-window — signed offers that must refuse pre-mutation
#   r2      build release at S2_COMMIT (default /opt/go-choir HEAD) and apply — no-reboot swap
#   panic   build release at a throwaway panic commit (created on Node B) and apply — must fail health, restore predecessor incl. exec
#   rollback re-apply predecessor release (2f0e2cac) — rollback through release authority
#   inspect guest layout receipt (boot timeline + console + updater-root listing via RO mount)
set -uo pipefail
OWNER="${OWNER:-a85b8fee-c7f5-4d04-b1f1-0e0ce8e4c335}"
COMPUTER="computer-6450a253b8b6ebc0866471973694f5be"
VM="vm-7bbcf74444aa90afff8acf4a378b72a6"
BUILDER=/var/lib/go-choir/services/choir-builder/bin/choir-builder
# MANIFEST must pin the BOOTED base image manifest, not the live deployed
# file — a deploy mid-run flips the file and mints mismatched base_commit.
MANIFEST="${MANIFEST:-/var/lib/go-choir/guest/guest-image-manifest}"
STOREDISK=/var/lib/go-choir/guest/storedisk.erofs
SRC=/opt/go-choir
S2_COMMIT="${S2_COMMIT:-$(git -C "$SRC" rev-parse HEAD)}"
# PRED_COMMIT must name a release built against the SAME base manifest the
# guest is booted on — set it explicitly per run (it is the release the
# rollback leg applies), not a constant.
PRED_COMMIT="${PRED_COMMIT:-}"
REL_ROOT=/var/lib/go-choir/builder/releases
IC=(-H 'X-Internal-Caller: true')
NOW=$(date -u +%Y%m%dT%H%M%SZ)

say(){ echo "## $*"; }
jlog(){ jq -nc "$@" ; }

fc_pid(){ pgrep -f "firecracker.*id ${VM}" | head -1; }
guest_health(){ curl -sS -m 8 "${IC[@]}" "http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${OWNER}/health?desktop=primary" 2>/dev/null; }
served_commit(){ guest_health | jq -r '.build.commit // empty'; }
guest_boot_id(){ curl -sS -m 8 "${IC[@]}" "http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${OWNER}/internal/boot/timeline?desktop=primary" 2>/dev/null | jq -r '.layout.boot_id // empty'; }
event_head(){ curl -sS "${IC[@]}" -H "X-Authenticated-User: ${OWNER}" "http://127.0.0.1:8086/internal/computers/events/head?computer_id=${COMPUTER}"; }
canonical_head(){ event_head | jq -r '.canonical_event_head // empty'; }
pending_ref(){ event_head | jq -r '.pending_transition_ref // empty'; }
tape_kinds(){ curl -sS "${IC[@]}" -H "X-Authenticated-User: ${OWNER}" "http://127.0.0.1:8086/internal/computers/events/replay?computer_id=${COMPUTER}&limit=60" | jq -r '.[].request.event.event_kind' 2>/dev/null; }
route_gen(){ curl -sS "${IC[@]}" "http://127.0.0.1:8083/internal/vmctl/computer-version-routes/resolve?route_slot_id=computer:${OWNER}:primary" | jq -r '.slot.generation // 0'; }
epoch_of(){ curl -sS "${IC[@]}" http://127.0.0.1:8083/internal/vmctl/list | jq -r --arg vm "$VM" '.ownerships[]|select(.vm_id==$vm)|.epoch'; }

wait_head_free(){ for i in $(seq 1 36); do p=$(pending_ref); [ -z "$p" ] || [ "$p" = "null" ] && return 0; sleep 5; done; return 1; }
wait_served(){ local want="$1" n=0; while [ $n -lt 90 ]; do c=$(served_commit); [ "$c" = "$want" ] || [[ "$want" == "$c"* ]] || [[ "$c" == "$want"* ]] && return 0; sleep 5; n=$((n+1)); done; return 1; }
wait_healthy(){ local n=0; while [ $n -lt 90 ]; do s=$(guest_health | jq -r '.status' 2>/dev/null); [ "$s" = "ready" ] || [ "$s" = "ok" ] && return 0; sleep 5; n=$((n+1)); done; return 1; }

build_release(){
  # $1 release name: commit SHA or synthetic dir. The out dir is base-keyed
  # via release_dir, so a nar built against one base is never reused after
  # a deploy moves the base. Synthetic dirs must already exist.
  local commit="$1"
  local out
  out="$(release_dir "$commit")/out"
  if [ ! -f "$out/app-layer-closure.nar" ]; then
    git -C "$SRC" cat-file -e "$commit^{commit}" 2>/dev/null || return 0
    local wt="/tmp/s2-build-$commit"
    git -C "$SRC" worktree remove --force "$wt" 2>/dev/null || true
    git -C "$SRC" fetch -q origin "$commit" 2>/dev/null || true
    git -C "$SRC" worktree add "$wt" "$commit" >/dev/null 2>&1 || git -C "$wt" checkout -q "$commit"
    (cd "$wt" && "$BUILDER" --installable '.#autoputer' --source-dir "$wt" \
      --base-manifest "$MANIFEST" --base-storedisk "$STOREDISK" --out "$out") || { git -C "$SRC" worktree remove --force "$wt" 2>/dev/null; return 1; }
    git -C "$SRC" worktree remove --force "$wt" 2>/dev/null || true
  fi
  [ -f "$out/app-layer-closure.nar" ] && [ -f "$out/builder-receipt.json" ]
}

mk_offer(){
  # $1 receipt, $2 update_id, $3 fields-overrides json (jq obj merged last)
  # S2_WITH_INLINE_SPA=1 restores the legacy harness-injected frontend file;
  # default omits it so the only servable frontend bytes are the staged
  # built tree (the frontend-join proof shape).
  local receipt="$1" uid="$2" over="${3:-"{}"}"
  local cc ep schema bc rd nar nar_sha head
  cc=$(jq -r .code_commit "$receipt")
  ep="$(jq -r .runtime_path "$receipt" | sed 's|^/nix/store/||')/bin/autoputer"
  schema=$(jq -r '.store_schema_version // 1' "$receipt")
  bc=$(awk -F= '$1=="build_commit"{print $2}' "$MANIFEST")
  rd=$(sha256sum "$receipt" | awk '{print $1}')
  nar=$(jq -r .exported_path "$receipt")
  if [ ! -f "${nar:-}" ]; then nar="$(dirname "$receipt")/app-layer-closure.nar"; fi
  nar_sha=$(sha256sum "$nar" | awk '{print $1}')
  curl -fsS "${IC[@]}" -T "$nar" "http://127.0.0.1:8086/internal/computers/platform-updates/blob/${nar_sha}" >/dev/null
  head=$(canonical_head)
  jq -n \
    --arg cid "$COMPUTER" --arg uid "$uid" --arg rid "${VM}-epoch-$(epoch_of)" \
    --arg beh "$head" --arg exp "$(date -u -d '+4 minutes' +%Y-%m-%dT%H:%M:%SZ)" \
    --arg mk "$uid" --arg cc "$cc" --arg bmd "$(sha256sum "$MANIFEST"|awk '{print $1}')" \
    --arg cd "$nar_sha" --arg ep "$ep" --argjson sv "$schema" --arg bc "$bc" \
    --arg rd "$rd" --arg ref "artifact+sha256://${nar_sha}/sha256/platform-update/${nar_sha}" \
    --arg vr "$(printf 'verify-%s' "$uid" | sha256sum | awk '{print $1}')" \
    --arg spa "$(printf '%s' '<!doctype html><title>s2-layered</title><div id=app></div>' | base64 -w0)" \
    --argjson over "$over" \
    '({computer_id:$cid,update_id:$uid,realization_id:$rid,base_event_head:$beh,expires_at:$exp,marker:$mk,code_commit:$cc,base_image_manifest_digest:$bmd,closure_digest:$cd,layering_entrypoint:$ep,store_schema_version:$sv,base_commit:$bc,builder_receipt_digest:$rd} + (if env.S2_WITH_INLINE_SPA == "1" then {files:[{path:"closure.nar",mode:292,ref:$ref},{path:"frontend/index.html",mode:420,bytes:$spa}]} else {files:[{path:"closure.nar",mode:420,ref:$ref}]} end) + {verifier_refs:[$vr],divergence_status:"tracking",platform_follow_policy:"auto"} + $over)'
}

mint(){ curl -sS -X POST -H 'Content-Type: application/json' "${IC[@]}" --data-binary @- \
  "http://127.0.0.1:8086/internal/computers/platform-updates/offer"; }
push(){ curl -sS -m 120 -X POST -H 'Content-Type: application/json' "${IC[@]}" --data-binary @- \
  "http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${OWNER}/internal/runtime/platform-update?desktop=primary"; }

RESULT=/var/lib/go-choir/deploy-failures/s2-acceptance-${NOW}.jsonl
note(){ jq -nc --arg leg "$1" --argjson d "${2:-null}" '{leg:$leg,at:(now|todateiso8601),data:$d}' | tee -a "$RESULT"; }

# release_dir resolves a release name to its base-keyed out dir, so every leg
# reads the nar built against the CURRENT base manifest — never a stale nar
# from a previous deploy. Synthetic dirs pass through for legs that manage
# their own out dir (panic). A missing manifest is fatal: falling back to a
# shared prefix would mix bases silently.
release_dir(){
  local name="$1" basekey
  case "$name" in
    */*) printf '%s' "$REL_ROOT/$name"; return 0;;
  esac
  [ -f "${MANIFEST:-}" ] || { echo "MANIFEST missing: $MANIFEST" >&2; return 1; }
  basekey="$(sha256sum "$MANIFEST" 2>/dev/null | awk '{print substr($1,1,12)}')"
  [ -n "$basekey" ] || { echo "MANIFEST digest failed: $MANIFEST" >&2; return 1; }
  printf '%s' "$REL_ROOT/$basekey-$name"
}

case "${1:-all}" in
pre)
  note pre "$(jlog \
    --arg fc "$(fc_pid)" --arg boot "$(guest_boot_id)" \
    --arg head "$(canonical_head)" --arg gen "$(route_gen)" \
    --arg served "$(served_commit)" --arg epoch "$(epoch_of)" \
    '{fc_pid:$fc,boot_id:$boot,canonical_head:$head,route_generation:$gen,served_commit:$served,epoch:$epoch}')"
  ;;
neg)
  [ -n "$PRED_COMMIT" ] || { echo "PRED_COMMIT required (release built against the booted base)"; exit 2; }
  receipt="$(release_dir "$PRED_COMMIT")/out/builder-receipt.json"
  case "$2" in
    base-digest)      over='{"base_image_manifest_digest":"0000000000000000000000000000000000000000000000000000000000000000"}' ;;
    stale-head)       over='{"base_event_head":"3333333333333333333333333333333333333333333333333333333333333333"}' ;;
    content-mutation) over='{"closure_digest":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}' ;;
    provenance)       over='{"code_commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}' ;;
    absent-entrypoint) over='{"layering_entrypoint":"ffffffffffffffffffffffffffffffffffff-autoputer-0.1.0/bin/autoputer"}' ;;
    base-commit)      over='{"base_commit":"2222222222222222222222222222222222222222"}' ;;
    schema-window)    over='{"store_schema_version":1,"min_store_schema_version":999}' ;;
    *) echo "unknown neg case $2"; exit 2 ;;
  esac
  pre=$(note "neg-$2-pre" "$(jlog --arg g "$(route_gen)" --arg h "$(canonical_head)" --arg s "$(served_commit)" --arg b "$(guest_boot_id)" --arg f "$(fc_pid)" '{route_gen:$g,head:$h,served:$s,boot:$b,fc:$f}')" | jq -c .data)
  req=$(mk_offer "$receipt" "s2neg-$2-${NOW}" "$over")
  offer=$(printf '%s' "$req" | mint)
  sig=$(printf '%s' "$offer" | jq -r '.authorization.signature // empty')
  resp=$(printf '%s' "$(jq -nc --argjson o "$offer" '{offer:$o}')" | push)
  sleep 10
  post=$(jlog --arg g "$(route_gen)" --arg h "$(canonical_head)" --arg p "$(pending_ref)" --arg s "$(served_commit)" --arg b "$(guest_boot_id)" --arg f "$(fc_pid)" '{route_gen:$g,head:$h,pending:$p,served:$s,boot:$b,fc:$f}')
  tail_kinds=$(tape_kinds | tail -5 | tr '\n' ' ')
  # Post-fix contract: a pre-mutation refusal must discharge the pending
  # transition — pending empty and materialization_failed on the tail.
  # The refusal reason must name the expected gate (no eyeballing): each
  # case declares its gate string, and the leg fails unless the push
  # response contains it. Boot/fc/route pins must be unchanged.
  case "$2" in
    base-digest)      want_reason="does not match booted base";;
    stale-head)       want_reason="base event head is stale";;
    content-mutation) want_reason="closure.nar digest";;
    provenance)       want_reason="code_commit";;
    absent-entrypoint) want_reason="not materialized";;
    base-commit)      want_reason="does not match booted base commit";;
    schema-window)    want_reason="store schema";;
  esac
  pend=$(printf '%s' "$post" | jq -r .pending)
  discharged=false
  if [ -z "$pend" ] || [ "$pend" = "null" ]; then discharged=true; fi
  reason_ok=false
  if printf '%s' "$resp" | grep -qF "$want_reason"; then reason_ok=true; fi
  pins_ok=false
  if [ "$(printf '%s' "$post" | jq -r .boot)" = "$(printf '%s' "$pre" | jq -r .boot)" ] && \
     [ "$(printf '%s' "$post" | jq -r .fc)" = "$(printf '%s' "$pre" | jq -r .fc)" ] && \
     [ "$(printf '%s' "$post" | jq -r .served)" = "$(printf '%s' "$pre" | jq -r .served)" ]; then pins_ok=true; fi
  note "neg-$2" "$(jlog --argjson pre "$pre" --argjson post "$post" --arg resp "${resp:0:1200}" --arg tape "$tail_kinds" --argjson disc "$discharged" --argjson rok "$reason_ok" --argjson pok "$pins_ok" --arg want "$want_reason" '{pre:$pre,post:$post,push_response:$resp,tape_tail:$tape,discharged:$disc,reason_ok:$rok,pins_ok:$pok,want_reason:$want}')"
  [ "$discharged" = "true" ] || { echo "neg-$2: pending transition not discharged"; exit 5; }
  [ "$reason_ok" = "true" ] || { echo "neg-$2: refusal reason missing (want: $want_reason)"; exit 6; }
  [ "$pins_ok" = "true" ] || { echo "neg-$2: boot/fc/served pins moved"; exit 7; }
  ;;
apply)
  # Generic apply leg: apply a release from an arbitrary release dir whose
  # entrypoint differs from the booted base. $2 = release dir under REL_ROOT
  # (a commit SHA, or a synthetic dir like idxmarker for patch-built releases).
  [ -n "${2:-}" ] || { echo "apply requires a release dir"; exit 2; }
  build_release "$2" || { note "apply-$2" '{"error":"build failed"}'; exit 4; }
  receipt="$(release_dir "$2")/out/builder-receipt.json"
  nar_sha=$(sha256sum "$(release_dir "$2")/out/app-layer-closure.nar"|awk '{print $1}')
  note "apply-$2-build" "$(jq -c '{code_commit:.code_commit,runtime_path:.runtime_path,closure_paths:(.closure_paths|length),nar_sha:"'"$nar_sha"'"}' "$receipt")"
  pre=$(jlog --arg s "$(served_commit)" --arg b "$(guest_boot_id)" --arg f "$(fc_pid)" --arg g "$(route_gen)" '{served:$s,boot:$b,fc:$f,route_gen:$g}')
  req=$(mk_offer "$receipt" "s2acc-apply-${NOW}" '{}')
  offer=$(printf '%s' "$req" | mint)
  resp=$(printf '%s' "$(jq -nc --argjson o "$offer" '{offer:$o}')" | push)
  note "apply-$2-push" "$(jlog --arg r "${resp:0:600}" '{response:$r}')"
  wait_served "$2" || true
  note "apply-$2" "$(jlog --argjson pre "$pre" --arg s "$(served_commit)" --arg b "$(guest_boot_id)" --arg f "$(fc_pid)" --arg g "$(route_gen)" --arg p "$(pending_ref)" '{pre:$pre,served:$s,boot:$b,fc:$f,route_gen:$g,pending:$p}')"
  ;;
r2)  # legacy alias → apply $S2_COMMIT
  S2_COMMIT="${S2_COMMIT:-$(git -C "$SRC" rev-parse HEAD)}"
  exec "$0" apply "$S2_COMMIT"
  ;;
panic)
  WT=/tmp/s2panic-wt
  git -C "$SRC" worktree remove --force "$WT" 2>/dev/null || true
  git -C "$SRC" worktree add --detach "$WT" HEAD >/dev/null 2>&1
  cd "$WT"
  python3 - <<'PY'
src = open('cmd/autoputer/main.go').read()
src = src.replace('autoputer.Run()',
  'if os.Getenv("CHOIR_UPDATER_ROOT") != "" { os.Exit(3) }\n\tautoputer.Run()', 1)
open('cmd/autoputer/main.go','w').write(src)
PY
  git -C "$WT" -c user.name=s2acc -c user.email=s2acc@localhost commit -qam "s2-acc panic release (throwaway)"
  PCOMMIT=$(git -C "$WT" rev-parse HEAD)
  out="$REL_ROOT/$PCOMMIT/out"; mkdir -p "$out"
  "$BUILDER" --installable '.#autoputer' --source-dir "$WT" \
    --base-manifest "$MANIFEST" --base-storedisk "$STOREDISK" --out "$out" || { note panic '{"error":"build failed"}'; exit 4; }
  receipt="$out/builder-receipt.json"
  pre_commit=$(served_commit)
  req=$(mk_offer "$receipt" "s2acc-panic-${NOW}" '{}')
  offer=$(printf '%s' "$req" | mint)
  resp=$(printf '%s' "$(jq -nc --argjson o "$offer" '{offer:$o}')" | push)
  note panic-push "$(jlog --arg r "${resp:0:600}" '{response:$r}')"
  # the apply must fail health and restore the predecessor incl. exec
  wait_served "$pre_commit" || true
  bootguard=$(curl -sS "${IC[@]}" "http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${OWNER}/internal/boot/timeline?desktop=primary" 2>/dev/null | jq -r '.layout.boot_id // empty')
  guardfile=$(ls /var/lib/go-choir/vm-state/$VM/ 2>/dev/null | wc -l)
  trip=$(find / -xdev -name "*.tripped" -path "*bootguard*" 2>/dev/null | head -3; ls /mnt/ 2>/dev/null)
  console=$(grep -a "bootguard\|layering release" /var/lib/go-choir/vm-state/$VM/console.log 2>/dev/null | tail -6 | tr '\n' ' ')
  note panic "$(jlog --arg s "$(served_commit)" --arg want "$pre_commit" --arg b "$(guest_boot_id)" --arg f "$(fc_pid)" --arg pc "$PCOMMIT" --arg con "$console" '{served:$s,restored:$want,boot:$b,fc:$f,panic_commit:$pc,console_tail:$con}')"
  ;;
rollback)
  # $2 = release dir under REL_ROOT to re-apply as the predecessor (a commit
  # SHA, or a synthetic dir like idxmarker for patch-built releases). It must
  # be built against the booted base and distinct from the serving one.
  # A fresh update_id mints a new operation — same release digest, real
  # apply path (journal replay would be vacuous).
  [ -n "${2:-}" ] || { echo "rollback requires the predecessor release dir"; exit 2; }
  receipt="$(release_dir "$2")/out/builder-receipt.json"
  pre=$(jlog --arg s "$(served_commit)" --arg b "$(guest_boot_id)" --arg f "$(fc_pid)" --arg g "$(route_gen)" '{served:$s,boot:$b,fc:$f,route_gen:$g}')
  req=$(mk_offer "$receipt" "s2acc-rb-${NOW}" '{}')
  offer=$(printf '%s' "$req" | mint)
  resp=$(printf '%s' "$(jq -nc --argjson o "$offer" '{offer:$o}')" | push)
  note rb-push "$(jlog --arg r "${resp:0:600}" '{response:$r}')"
  wait_served "$2" || true
  note rollback "$(jlog --argjson pre "$pre" --arg s "$(served_commit)" --arg b "$(guest_boot_id)" --arg f "$(fc_pid)" --arg g "$(route_gen)" '{pre:$pre,served:$s,boot:$b,fc:$f,route_gen:$g}')"
  ;;
inspect)
  note inspect-timeline "$(curl -sS -m 8 "${IC[@]}" "http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${OWNER}/internal/boot/timeline?desktop=primary" | jq -c '{layout:{boot_id:.layout.boot_id,mounts:.layout.mounts,nix_store_entries:.layout.nix_store_entries,persistent:.layout.persistent_bytes_total}}')"
  note inspect-console "$(grep -a "layering release\|overlay\|bootguard" /var/lib/go-choir/vm-state/$VM/console.log 2>/dev/null | tail -15 | jq -Rsc .)"
  mkdir -p /tmp/s2-data-ro
  mount -o ro,noload /var/lib/go-choir/vm-state/$VM/data.img /tmp/s2-data-ro 2>/dev/null && {
    find /tmp/s2-data-ro/mnt 2>/dev/null | head; 
    ls /tmp/s2-data-ro/choir-updater 2>/dev/null || find /tmp/s2-data-ro -maxdepth 2 -name "choir-updater" -o -name "current" | head
    UPD=$(find /tmp/s2-data-ro -maxdepth 3 -type d -name "choir-updater" | head -1)
    [ -n "$UPD" ] && {
      ls -la "$UPD"; echo "--- current:"; readlink "$UPD/current"; ls "$UPD"/releases 2>/dev/null | head
      echo "--- gc-roots:"; ls "$UPD"/gc-roots 2>/dev/null | head
      echo "--- entrypoint:"; cat "$UPD"/current/layering-entrypoint 2>/dev/null
      echo "--- store count:"; ls "$UPD"/store 2>/dev/null | wc -l
      echo "--- bootguard:"; ls "$UPD"/bootguard 2>/dev/null
      echo "--- processes check (nix daemon presence in guest fs):"; find /tmp/s2-data-ro -name "nix-daemon" 2>/dev/null | head -2
    }
    umount /tmp/s2-data-ro
  } || note inspect-mount '{"error":"ro mount refused"}'
  ;;
esac
