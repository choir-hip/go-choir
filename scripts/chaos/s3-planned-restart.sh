#!/usr/bin/env bash
# Scenario 3: planned restart under load. Sourced by scripts/chaos/run.
# vmctl's marker-backed refresh (POST /internal/vmctl/deploy-refresh; the
# guest writes <store>.planned-restart after draining telemetry and the
# run-memory window, vmctl restarts it, the next boot consumes the marker
# once). /internal/vmctl/refresh is the markerless one and is not used.
# d@e is class interactive: vmctl defers a busy guest (running runs), and its
# retry sweep refreshes it at the next idle moment, so the load here is file
# PUTs (no runs) and a deferral is waited out, not treated as a failure.
# Contract: drain completes (no drain-failure line), one planned boot and no
# crash boot, no pre-restart run crash-closed or passivated twice, every
# acknowledged write present, no duplicate, replay equivalent.
SCN_NAME=s3-planned-restart
SCN_FAULT="POST /internal/vmctl/deploy-refresh for d@e (planned, marker-backed restart) while it writes"

scn_plan() {
	cat <<EOF
pre:    pre-flight; map d@e (wake if needed); mark the console log; save open runs; start
        file PUT writes every 1s (no Texture creates: they start runs and a busy d@e defers)
fault:  POST 127.0.0.1:8083/internal/vmctl/deploy-refresh {user_id, desktop_id, target: d@e's build}
        X-Choir-Lifecycle-Caller: chaos.harness.s3
wait:   up to ${CHAOS_S3_TIMEOUT:-600}s for a planned boot line in d@e's console (a deferral is retried
        by vmctl's sweep); then up to 180s for d@e answering; 10s more writes
post:   one 'restart kind=planned', no crash boot, no 'treating boot as a crash'; no telemetry drain
        failure line; acknowledged writes read back; no pre-restart run crash-closed or passivated
        twice; replay-completeness equivalent
undo:   if no planned boot was seen: DELETE the guest marker (guest port, no token)
EOF
}

scn_pre() {
	chaos_target_up
	chaos_check "pre: d@e is active" eq active "$T_STATE"
	case "$T_URL" in http://10.[0-9]*:[0-9]*) ;; *) [ "$CHAOS_DRY" = 1 ] || chaos_die 3 "REFUSED: guest url '$T_URL' is not a guest tap address" ;; esac
	[ -n "$T_BUILD" ] || [ "$CHAOS_DRY" = 1 ] || chaos_die 1 "d@e reports no build commit to use as the refresh target"
	S_GC=$(chaos_setting tape.group_commit)
	chaos_event settings "tape.group_commit=$S_GC"
	chaos_console_mark
	chaos_save_open_runs
	chaos_load_start "file"
	chaos_nap 6
}

S3_PLANNED_SEEN=false
s3_undo() {
	[ "$S3_PLANNED_SEEN" = true ] && return 0
	# A guest port: the marker header only, never the internal-service token.
	chaos_remote_mut "clear d@e's planned-restart marker (no planned boot was seen)" "
curl -sS --max-time 15 -X DELETE -H 'X-Internal-Caller: true' '$T_URL/internal/runtime/planned-restart' || true" undo
}
s3_planned_boot() {
	local l
	l=$(chaos_boot_lines)
	[ "$(chaos_count "$l" 'restart kind=planned')" -ge 1 ]
}

scn_inject() {
	chaos_undo_push s3_undo "clear the guest marker if the planned boot never came"
	chaos_event fault "deploy-refresh target=$T_BUILD acked_file_writes=$(chaos_acked file | grep -c . || true)"
	body=$(jq -nc --arg u "$T_USER" --arg d "$T_DESKTOP" --arg t "$T_BUILD" '{user_id:$u,desktop_id:$d,target:$t}')
	case "$body" in *"'"*) chaos_die 2 "refusing a body with a single quote" ;; esac
	out=$(chaos_remote_mut "planned refresh of $T_VM (d@e) via vmctl deploy-refresh" "
row=\$(cr_row '$CHAOS_TARGET'); [ \"\$(printf '%s' \"\$row\" | jq -r .vm_id)\" = '$T_VM' ] || cr_refuse 'mapping changed'
[ \"\$(printf '%s' \"\$row\" | jq -r .user_id)\" = '$T_USER' ] || cr_refuse 'user changed'
choir_internal_curl 'http://127.0.0.1:8083/internal/vmctl/deploy-refresh' -sS --max-time 600 -X POST -H 'X-Choir-Lifecycle-Caller: chaos.harness.s3' -H 'Content-Type: application/json' -d '$body'" fault)
	case "$out" in REFUSED:*) chaos_die 3 "$out" ;; ABORT:*) chaos_abort "$out" ;; esac
	S_OUTCOME=$(printf '%s' "$out" | jq -r '.outcome // ""' 2>/dev/null)
	S_VMID=$(printf '%s' "$out" | jq -r '.vm_id // ""' 2>/dev/null)
	chaos_event fault-done "${out:-dry-run}"
}

scn_wait() {
	S_UP=false
	chaos_wait "${CHAOS_S3_TIMEOUT:-600}" 10 "a planned boot of d@e" s3_planned_boot && S3_PLANNED_SEEN=true
	chaos_wait 180 5 "d@e answering" chaos_ready && S_UP=true
	chaos_nap 10
	chaos_load_stop
}

scn_post() {
	local lines f n closed noted active repass restarted
	case "$S_OUTCOME" in refreshed | deferred) S_OUTCOME_OK=ok ;; *) S_OUTCOME_OK="outcome=${S_OUTCOME:-none}" ;; esac
	chaos_check "deploy-refresh outcome refreshed, or deferred and retried by the sweep" eq ok "$S_OUTCOME_OK"
	[ -z "$S_VMID" ] || chaos_check "deploy-refresh acted on d@e's vm" eq "$T_VM" "$S_VMID"
	chaos_check "planned boot seen" eq true "$S3_PLANNED_SEEN"
	chaos_check "d@e answering after the planned restart" eq true "$S_UP"
	lines=$(chaos_boot_lines)
	printf '%s\n' "$lines" >"$CHAOS_RUN_DIR/boot-lines.txt"
	chaos_check "planned-restart marker consumed once (one planned boot)" eq 1 "$(chaos_count "$lines" 'restart kind=planned')"
	chaos_check "no crash boot" eq 0 "$(chaos_count "$lines" 'restart kind=crash_or_stop')"
	chaos_check "no marker rejected as a crash" eq 0 "$(chaos_count "$lines" 'treating boot as a crash')"
	chaos_check "no run crash-closed" eq 0 "$(chaos_count "$lines" 'after a crash restart')"
	chaos_check "telemetry and run-memory drain did not fail" eq 0 "$(chaos_count "$lines" 'telemetry drain failed')"
	chaos_check "the guest took the host's planned-restart request" ge 1 "$(chaos_count "$lines" 'host requested a planned restart')"
	chaos_check_acked_present
	f=$(chaos_run_fates)
	read -r n closed noted active repass restarted <<EOF
$f
EOF
	chaos_check "no pre-restart run closed as interrupted_by_restart" eq 0 "$closed"
	chaos_check "no pre-restart run passivated more than once (resumed once)" eq 0 "$repass"
	chaos_check "replay-completeness of d@e" eq equivalent "$(chaos_replay_status)"
}
