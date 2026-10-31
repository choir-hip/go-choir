#!/usr/bin/env bash
# Scenario 2: host service crash. Sourced by scripts/chaos/run.
# go-choir-corpusd (the tape and receipt service the guest appends to over its
# tap, port 8086) is SIGKILLed while d@e writes. systemd restarts it
# (Restart=on-failure); a dead-man timer started BEFORE the kill starts it in
# any case. Contract: the guest's appends retry under their idempotency key
# without duplicates; no acknowledged write is lost; the host receipt chain
# of d@e (computer_event_append_receipts) has no gap; the rows written in the
# drill are compact (receipt compaction is active on d@e) and expand on
# replay; the tape segments read back.
SCN_NAME=s2-corpusd-crash
SCN_FAULT="systemctl kill -s KILL go-choir-corpusd while d@e appends (dead-man restart timer first)"
S2_UNIT=go-choir-corpusd

scn_plan() {
	cat <<EOF
pre:    pre-flight; map d@e (wake if needed); corpusd active; receipt chain stats of d@e
        (count, min, max sequence) from PostgreSQL; start writes (file PUT every 1s, Texture creates)
fault:  systemd-run --on-active=${CHAOS_S2_DEADMAN:-90}s --unit=choir-chaos-corpusd-deadman-<run> systemctl start $S2_UNIT;
        then systemctl kill -s KILL $S2_UNIT
wait:   up to 120s for corpusd active, 15s more writes, up to 180s for d@e answering
post:   acknowledged writes read back, Texture replays answer the same doc_id; chain count = max-min+1;
        new rows >= 1; new rows' request_commitment distinct; new rows compact ('c1:'); replay of the
        new rows (127.0.0.1:8086) returns them all with expanded receipts; corpusd tape counters
        segment_fallbacks and reader.segment_error_fallbacks 0; d@e's newest segment reads whole
EOF
}

s2_pg() { # SQL: one psql read on Node B, capped
	chaos_remote "cr_capped runuser -u postgres -- \"\$(command -v psql)\" -d control -XAtc \"$1\""
}
s2_chain() { s2_pg "select count(*)||' '||coalesce(min(sequence),0)||' '||coalesce(max(sequence),0) from computer_event_append_receipts where computer_id='$CHAOS_TARGET'"; }
s2_active() { [ "$(chaos_remote "systemctl is-active $S2_UNIT || true")" = active ]; }

scn_pre() {
	chaos_target_up
	chaos_check "pre: d@e is active" eq active "$T_STATE"
	chaos_check "pre: $S2_UNIT active" eq active "$(chaos_remote "systemctl is-active $S2_UNIT || true")"
	S_CHAIN0=$(s2_chain)
	chaos_event chain-before "${S_CHAIN0:-dry-run}"
	S_MAX0=$(printf '%s' "$S_CHAIN0" | awk '{print $3}')
	chaos_isnum "${S_MAX0:-}" || S_MAX0=0
	chaos_load_start "file texture"
	chaos_nap 6
}

S2_DEADMAN_UNIT=""
s2_undo() {
	chaos_remote_mut "start $S2_UNIT if it is not active; stop the dead-man timer" "
systemctl is-active --quiet $S2_UNIT || systemctl start $S2_UNIT
systemctl stop '$S2_DEADMAN_UNIT.timer' 2>/dev/null || true
systemctl is-active $S2_UNIT" undo
}

scn_inject() {
	S2_DEADMAN_UNIT="choir-chaos-corpusd-deadman-$CHAOS_RUN_ID"
	S2_DEADMAN_UNIT=$(printf '%s' "$S2_DEADMAN_UNIT" | tr -c 'A-Za-z0-9-\n' '-')
	chaos_undo_push s2_undo "start $S2_UNIT and stop the dead-man timer"
	chaos_event fault "systemctl kill -s KILL $S2_UNIT acked_file_writes=$(chaos_acked file | grep -c . || true)"
	out=$(chaos_remote_mut "dead-man restart timer, then SIGKILL $S2_UNIT" "
systemd-run --quiet --on-active=${CHAOS_S2_DEADMAN:-90}s --unit='$S2_DEADMAN_UNIT' --description='chaos: restart $S2_UNIT' systemctl start $S2_UNIT || { echo 'REFUSED: dead-man timer not scheduled'; exit 3; }
systemctl kill -s KILL $S2_UNIT && echo killed" fault)
	case "$out" in REFUSED:*) chaos_die 3 "$out" ;; ABORT:*) chaos_abort "$out" ;; esac
	[ "$CHAOS_DRY" = 1 ] || [ "$(printf '%s' "$out" | tail -1)" = killed ] || chaos_die 1 "kill did not report success: $out"
	chaos_event fault-done "${out:-dry-run}"
}

scn_wait() {
	S_CORPUS_UP=false S_UP=false
	chaos_nap 2
	chaos_wait 120 3 "$S2_UNIT active again" s2_active && S_CORPUS_UP=true
	chaos_nap 15
	chaos_load_stop
	chaos_wait 180 5 "d@e answering" chaos_ready && S_UP=true
}

scn_post() {
	local chain1 cnt mn mx new distinct compact rep rcount rexp rcontig health
	chaos_check "$S2_UNIT active after the kill" eq true "$S_CORPUS_UP"
	chaos_check "d@e answering after the host crash" eq true "$S_UP"
	chaos_check_acked_present
	chain1=$(s2_chain)
	read -r cnt mn mx <<EOF
$chain1
EOF
	chaos_check "receipt chain of d@e has no gap (count = max - min + 1)" eq "$((${mx:-0} - ${mn:-0} + 1))" "${cnt:-}"
	new=$(( ${mx:-0} - S_MAX0 ))
	chaos_check "new receipts during the drill" ge 1 "$new"
	distinct=$(s2_pg "select count(distinct request_commitment)||' '||count(*) from computer_event_append_receipts where computer_id='$CHAOS_TARGET' and sequence > $S_MAX0")
	chaos_check "new receipts have distinct request commitments (no duplicate append)" eq "${distinct#* }" "${distinct%% *}"
	compact=$(s2_pg "select count(*) from computer_event_append_receipts where computer_id='$CHAOS_TARGET' and sequence > $S_MAX0 and left(event_head_receipt_json, 3) = 'c1:'")
	chaos_check "new receipts stored compact (compaction active on d@e)" eq "$new" "$compact"
	rep=$(chaos_remote "choir_internal_curl 'http://127.0.0.1:8086/internal/computers/events/replay?computer_id=$CHAOS_TARGET&after_sequence=$S_MAX0&limit=2048' -fsS --max-time 60 -H 'X-Authenticated-User: $T_USER' | jq -c '{n: length, expanded: ([.[] | select((.event_head_receipt.receipt_kind // \"\") != \"\" and ((.event_head_receipt.signature_set // []) | length) > 0)] | length), seqs: [.[] | .request.sequence // .event_head_receipt.sequence]}'")
	rcount=$(printf '%s' "$rep" | jq -r '.n // ""' 2>/dev/null)
	rexp=$(printf '%s' "$rep" | jq -r '.expanded // ""' 2>/dev/null)
	rcontig=$(printf '%s' "$rep" | jq -r --argjson s "$S_MAX0" '.seqs as $q | [range(0; $q | length) | select($q[.] != ($s + 1 + .))] | length' 2>/dev/null)
	chaos_check "replay returns every new receipt" eq "$new" "$rcount"
	chaos_check "replayed receipts are expanded (kind and signatures present)" eq "$new" "$rexp"
	chaos_check "replayed sequences are contiguous" eq 0 "$rcontig"
	health=$(chaos_remote "choir_internal_curl 'http://127.0.0.1:8086/health' -fsS --max-time 15 | jq -c '{f: .tape.segment_fallbacks, r: .tape.reader.segment_error_fallbacks}'")
	chaos_check "corpusd tape segment_fallbacks since restart" eq 0 "$(printf '%s' "$health" | jq -r '.f // ""' 2>/dev/null)"
	chaos_check "corpusd tape reader segment_error_fallbacks since restart" eq 0 "$(printf '%s' "$health" | jq -r '.r // ""' 2>/dev/null)"
	chaos_check "d@e's newest tape segment reads whole" eq ok "$(chaos_remote "f=\$(ls -1 \"\$CR_ARTIFACTS\"/tape-seg/$CHAOS_TARGET/*.seg | tail -1); cr_capped cat \"\$f\" >/dev/null && echo ok")"
}
