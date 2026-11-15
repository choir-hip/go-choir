#!/usr/bin/env bash
# Scenario 1: guest crash mid-write. Sourced by scripts/chaos/run.
# d@e writes (file PUTs and Texture creates) and runs a prompt-bar run; its
# firecracker process is SIGKILLed by exact pid; vmctl brings it back on the
# next request. Contract (CLAUDE.md restarts; tape group commit design 9):
# every acknowledged write survives, run-memory messages included (they are
# acknowledged only after their CAS; the run-memory window is deleted); only
# queued telemetry may be missing, and the crash-closed run says so; open runs
# end "interrupted_by_restart" and nothing resumes; replay is equivalent. The
# drill has no cheap read of a run's memory log, so an acknowledged run-memory
# message surviving the kill is proved by the store tests
# (TestRunMemoryMessageIsAcknowledgedOnlyAfterItsCAS) and by replay-completeness
# below, not by a check of its own.
SCN_NAME=s1-guest-crash
SCN_FAULT="SIGKILL d@e's firecracker process (exact pid, mapped through ownerships.json) mid-write"

scn_plan() {
	cat <<EOF
pre:    pre-flight; map d@e -> vm -> firecracker pid (wake it first if needed); read telemetry.batch;
        mark the console log; start writes (file PUT every 1s, Texture create
        every 4th, at most 3); start one prompt-bar run and wait for it to run; save open run ids
fault:  re-map on Node B and kill -KILL <pid> only if the mapping is unchanged
wait:   up to ${CHAOS_S1_TIMEOUT:-300}s for d@e active and /api/runtime/observability 200 (the API wakes it)
post:   acknowledged writes read back; Texture replays answer the same doc_id; the prompt-bar
        replay answers the same submission; open runs cancelled with interrupted_by_restart and
        the 'may be missing' note (telemetry, when batching is on); none active or restarted; one
        crash boot and no planned boot in the console; replay-completeness equivalent
EOF
}

scn_pre() {
	chaos_target_up
	S_TB=$(chaos_setting telemetry.batch)
	chaos_event settings "telemetry.batch=$S_TB"
	chaos_check "pre: d@e is active" eq active "$T_STATE"
	chaos_console_mark
	chaos_load_start "file texture"
	chaos_start_run
	chaos_nap 8
	chaos_save_open_runs
	chaos_check "pre: open runs on d@e before the fault" ge 1 "$(grep -c . "$CHAOS_RUN_DIR/open-runs.txt" 2>/dev/null || echo 0)"
	S_ACK_AT_KILL=$(chaos_acked file | grep -c . || true)
}

s1_wake() { chaos_api GET /api/compute/status >/dev/null; }

scn_inject() {
	chaos_undo_push s1_wake "ask for d@e through its API so vmctl starts it again"
	chaos_event fault "kill -KILL pid=$T_PID vm=$T_VM acked_file_writes=${S_ACK_AT_KILL:-0}"
	out=$(chaos_remote_mut "SIGKILL firecracker pid $T_PID of $T_VM (d@e)" "
now=\$(cr_map '$CHAOS_TARGET')
[ \"\$now\" = '$T_VM $T_PID' ] || cr_refuse \"mapping changed since the check: \$now\"
case \"\$(tr '\\000' ' ' <\"\$CR_PROC/$T_PID/cmdline\")\" in *$CHAOS_PAPER_HEX*|*$CHAOS_OWNER_HEX*|*$CHAOS_OWNER_VM_PREFIX*|*$CHAOS_PAPER_VM*) cr_refuse 'pid cmdline names a protected computer' ;; esac
\$CR_KILL -KILL '$T_PID' && echo killed" fault)
	case "$out" in REFUSED:*) chaos_die 3 "$out" ;; ABORT:*) chaos_abort "$out" ;; esac
	[ "$CHAOS_DRY" = 1 ] || [ "$out" = killed ] || chaos_die 1 "kill did not report success: $out"
	chaos_event fault-done "${out:-dry-run}"
}

scn_wait() {
	chaos_nap 5
	chaos_load_stop
	S_UP=false
	chaos_wait "${CHAOS_S1_TIMEOUT:-300}" 5 "d@e back after the kill" chaos_ready && S_UP=true
}

scn_post() {
	local f n closed noted active repass restarted lines sub0 sub1
	chaos_check "d@e active and answering after the crash" eq true "$S_UP"
	chaos_check_acked_present
	sub0=$(jq -r '.submission_id // ""' <(chaos_body "$(cat "$CHAOS_RUN_DIR/prompt-bar.out" 2>/dev/null)") 2>/dev/null)
	sub1=$(chaos_body "$(chaos_api POST /api/prompt-bar "$(jq -nc --arg c "chaos-$CHAOS_RUN_ID-run" '{text:"Chaos drill on the test computer: write the numbers 1 to 40, one per line, each with one sentence about it. Take your time.",command_id:$c}')")" | jq -r '.submission_id // ""' 2>/dev/null)
	chaos_check "prompt-bar replay of the same command_id answers the same submission" eq "$sub0" "$sub1"
	f=$(chaos_run_fates)
	read -r n closed noted active repass restarted <<EOF
$f
EOF
	chaos_check "open runs closed with fate interrupted_by_restart" eq "$n" "$closed"
	if [ "$S_TB" = true ] || [ "$CHAOS_DRY" = 1 ]; then
		chaos_check "crash-closed runs carry the 'may be missing' note" eq "$closed" "$noted"
	fi
	chaos_check "no pre-crash run left active (nothing auto-resumed)" eq 0 "$active"
	chaos_check "no crash-closed run started again" eq 0 "$restarted"
	lines=$(chaos_boot_lines)
	printf '%s\n' "$lines" >"$CHAOS_RUN_DIR/boot-lines.txt"
	chaos_check "one crash boot after the kill" eq 1 "$(chaos_count "$lines" 'restart kind=crash_or_stop')"
	chaos_check "no planned boot after the kill" eq 0 "$(chaos_count "$lines" 'restart kind=planned')"
	chaos_check "replay-completeness of d@e" eq equivalent "$(chaos_replay_status)"
}
