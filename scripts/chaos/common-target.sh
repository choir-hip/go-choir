#!/usr/bin/env bash
# Shared scenario steps (sourced by the scenarios after lib.sh).

# chaos_target_up: map d@e; wake it through its own API when it is not
# active (the product path: the proxy asks vmctl), then map it with its pid.
chaos_target_up() {
	chaos_resolve 0
	if [ "$CHAOS_DRY" = 1 ]; then
		[ "$T_STATE" = active ] && chaos_resolve 1 || info "(dry-run) d@e is $T_STATE; would wake it through GET /api/compute/status, then map its firecracker pid"
		return 0
	fi
	if [ "$T_STATE" != active ]; then
		info "d@e is $T_STATE; waking it through its API"
		chaos_wait 240 5 "d@e active and answering" chaos_ready || chaos_die 1 "d@e did not come up"
	fi
	chaos_resolve 1
}

# chaos_console_mark: remember where d@e's console log ends (S_MARK "INODE SIZE").
chaos_console_mark() {
	S_MARK=$(chaos_remote "cr_console_mark '$T_VM'")
	[ -n "$S_MARK" ] || S_MARK="0 0"
	chaos_event console-mark "$S_MARK"
}

# chaos_boot_lines: the restart lines d@e logged after the mark.
chaos_boot_lines() {
	local ino=${S_MARK%% *} size=${S_MARK#* }
	chaos_isnum "$ino" && chaos_isnum "$size" || { ino=0 size=0; }
	chaos_remote "cr_console_since '$T_VM' '$ino' '$size'"
}
chaos_count() { printf '%s\n' "$1" | grep -c -- "$2" || true; }

# chaos_save_open_runs: the runs open before the fault (S_OPEN, one per line).
chaos_save_open_runs() {
	S_OPEN=$(chaos_open_runs)
	printf '%s\n' "$S_OPEN" | grep . >"$CHAOS_RUN_DIR/open-runs.txt" || true
	chaos_event open-runs "$(printf '%s' "$S_OPEN" | grep -c . || true)"
}

# chaos_run_fates: for the saved open runs, counts as "N CLOSED NOTED ACTIVE
# REPASSIVATED RESTARTED": closed with interrupted_by_restart, of those the
# ones whose close event carries the "may be missing" note, still active
# (pending/running/passivated/blocked), passivated more than once, and
# started again after their crash close.
chaos_run_fates() {
	local n=0 closed=0 noted=0 active=0 repass=0 restarted=0 id run ev st
	[ "$CHAOS_DRY" = 1 ] && { echo "0 0 0 0 0 0"; return 0; }
	while IFS= read -r id; do
		[ -n "$id" ] || continue
		n=$((n + 1))
		run=$(chaos_body "$(chaos_api GET "/api/runs/$id")")
		ev=$(chaos_run_events "$id")
		st=$(printf '%s' "$run" | jq -r '.state // ""' 2>/dev/null)
		if [ "$st" = cancelled ] && [ "$(printf '%s' "$run" | jq -r '.error // ""')" = interrupted_by_restart ]; then
			closed=$((closed + 1))
			printf '%s' "$ev" | jq -e '[.. | strings | select(test("may be missing"))] | length > 0' >/dev/null 2>&1 && noted=$((noted + 1))
			# a loop.started after the last loop.cancelled is a resume
			printf '%s' "$ev" | jq -e '[.. | objects | select(has("kind")) | .kind] as $k | ($k | rindex("loop.cancelled")) as $c | $c != null and ([$k[($c + 1):][] | select(. == "loop.started")] | length > 0)' >/dev/null 2>&1 && restarted=$((restarted + 1))
		fi
		case "$st" in pending | running | passivated | blocked) active=$((active + 1)) ;; esac
		[ "$(printf '%s' "$ev" | jq '[.. | objects | select(.kind? == "activation.passivated")] | length' 2>/dev/null || echo 0)" -gt 1 ] && repass=$((repass + 1))
	done <"$CHAOS_RUN_DIR/open-runs.txt"
	echo "$n $closed $noted $active $repass $restarted"
}
