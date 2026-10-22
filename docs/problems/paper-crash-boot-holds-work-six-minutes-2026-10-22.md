# A crash boot holds the paper's work for six minutes (2026-10-22)

Date: 2026-10-22, 20:15 UTC. Found by: FP0, during the KF host reboot.
Status: open, problem record before any fix. Mutation class of a fix: red
(boot path, crash-restart closure, the paper's availability).

## Evidence

Node B rebooted at 20:05:19 for kernel 6.18.57. The paper
(vm-1669020a…, critical_protected) was busy, so it refused a planned-restart
marker and booted as a crash (`restart kind=crash_or_stop`, 20:07:08). Its
`/health` answered 200 from 20:08:33, but its boot phases ran in order, and
the two crash closures came before the source schedules:

| phase | start | duration |
|---|---|---|
| `close_pre_boot_runs_after_crash` | 20:07:12 | 1m50.4s |
| `close_open_rounds_after_crash` | 20:09:03 | 4m36.6s |
| `source_schedules_after_restart` | 20:13:39 | 3.3s |

So no source schedule was armed, and the gateway saw no inference call from
the paper, for about six and a half minutes after its boot while `/health`
said ok. During the second phase the console was silent; from the host the
Firecracker vCPUs used about 0.8 CPU, and the process had read 12 GB from
its disks by 20:13.

Then the schedules wait one interval. At 20:13:39 each schedule logged
"crash boot: … stays active; next tick 2026-10-22T20:28:39Z", as designed
(a crash boot arms the next tick one interval after boot). Meanwhile the
dispatcher dead-lettered every source-poll wake recorded before the boot
(`durably invalid occurrence: interrupted_by_restart: work occurrence
recorded before this boot`; 42 between 20:16:07 and 20:22:20), also as
designed. So the paper did no source work from 20:05 to at least 20:28,
about 23 minutes, of which the closures were six and a half.

## Cause (read from the code)

`closeOpenRoundsAfterCrash` (`internal/agentcore/inbox_records.go:1133`)
calls `store.RebuildInboxHeads` (`internal/store/inbox.go:1088`), which
lists every inbox record of the computer (the record plane; the paper's
store held 46,381 `commitment_record` rows at 18:30, an upper bound), folds every inbox head and every
conversation, and only then picks the open conversations. The store already
keeps the conversation head as a cached object written with each record
(round, open flag, caster; `inbox.go:15-28`), so the open set is available
without the full fold. The fold's cost grows with the paper's whole
history, so every crash boot gets slower.

`close_pre_boot_runs_after_crash` closed each pre-boot run one at a time
(about 150 ms per run in the console).

## Why it matters

- Owner rule (2026-10-14): "we need paper publishing 24/7, even after
  crashes". A standing intent survives a crash, but here its next tick waits
  for a closure whose cost is unbounded.
- `/health` says ok through the whole window, so the runbook's "paper
  healthy" test (health ok AND schedules not overdue AND inference in the
  last 30 minutes) is the only check that catches it, and only after 30
  minutes.
- It is the same family as the post-refresh windows
  ([paper-post-refresh-health-windows-2026-10-22.md](paper-post-refresh-health-windows-2026-10-22.md))
  and the derived-keys validation pass: whole-history work on the boot path
  or right after it.

## Open

1. Can the crash closure read the open set from the cached conversation
   heads (and leave the full fold to `VerifyInboxHeads`, off the boot path)?
   What must hold for the cache to be trusted at a crash boot (the heads are
   written in the same transaction as their records?).
2. Should the source schedules arm before the crash closures? They are
   standing intents, independent of the closed runs, but a schedule tick
   must not race a closure of the same work.
3. The host reboot script never asks a busy critical computer for a
   planned restart with `allow_busy`, which the guest supports
   (`"allow_busy":false` in its refusals at 20:04:16-20:05:19). With it, the
   paper would have booted planned and skipped both closures. **Answered
   in the script** (R1 below, `lane/fp0-reboot-script-r2`); questions 1
   and 2 stay open, since any crash boot of the paper still pays them.

## Findings: the host reboot script (open question 3), 2026-10-22

FP0 review round on `scripts/node_b_host_reboot.py` (lane
`lane/fp0-reboot-script-r2`). Problem record before any fix. Mutation
class of the fix: red (the script decides how busy computers restart).

R1. **The script never asks with `allow_busy`.** `phase_mark` sends
`POST <computer_url>/internal/runtime/planned-restart` with no query, so
the guest applies its idle-only rule to every computer. The 20:04:16 to
20:05:19 refusals in `_runs/deployKF/reboot-20261022.log` all say
`"allow_busy":false,"running_runs":7-8,"selfdev_active_operations":0`:
the paper was busy only with runs, which the guest lets a host cut when
it asks with `allow_busy` (`HandleInternalPlannedRestart`,
`internal/agentcore/planned_restart.go`: the runs park for the planned
boot's one resume; a self-development operation mid-transaction or a
live capsule still refuses). vmctl's deploy refresh already asks with
`allow_busy` for exactly the always-busy tiers
(`deployRefreshAllowsBusy`, `internal/vmctl/deploy_refresh.go`:
`critical_protected` and `public_platform`), as CLAUDE.md requires. The
reboot script had a second, stricter rule, so the paper rebooted
unmarked under `--allow-crash-close` and booted as a crash: about 23
minutes without source work (Evidence above). The script's docstring and
the runbook's marker policy still say markers go only to idle guests.

R2. **The script does not say why a guest refused.** It logs the first
160 characters of each 409 and treats every refusal alike, so a refusal
for runs (which `--allow-crash-close` may cut) and a refusal for a
self-development apply or a live capsule (which the operating contract
says these computers are never restarted through) look the same, and
`--allow-crash-close` would crash both.

R3. **The post-check "FORWARD jumps vm-+ to CHOIR-GUEST-FWD first" is
stale.** After the reboot it failed (the only post-check FAIL) because
FORWARD's first rule is `-o vm-+ … -j CHOIR-GUEST-RET` (ESTABLISHED
accept, else DROP), then `-i vm-+ … -j CHOIR-GUEST-FWD`. That is the
order vmctl installs at start: `ensureBaseLocked`
(`internal/vmmanager/hostfirewall.go`) inserts the CHOIR-GUEST-FWD jump
at position 1 and then the CHOIR-GUEST-RET jump at position 1, so on a
fresh boot RET is rule 1 and FWD rule 2. The check predates the return
chain (SEC-03, 9256d6368). The firewall is right; the check is wrong, and
a check that fails on the right state teaches operators to ignore it.

### Fix (lane `lane/fp0-reboot-script-r2`)

- R1 fixed. `phase_mark` sends `?allow_busy=1` (a query parameter, as
  `httpGuestMarker.mark` does) exactly for `marker_allows_busy`:
  `critical_protected` and `public_platform`, vmctl's
  `deployRefreshAllowsBusy` set. The vmctl list's `warmness_class` is
  the registry-resolved class (`WarmnessClassForOwnership`), the input
  vmctl's rule reads. `scripts/test_node_b_host_reboot.py` reads the case
  list from the Go source, so a change to vmctl's rule fails the test
  instead of drifting. `--no-allow-busy` keeps the old rule. The
  docstring and the runbook's marker policy say so.
- R2 fixed. `classify_marker_answer` turns each answer into written,
  busy (runs only), unsafe (an apply mid-transaction or a live capsule,
  or a 409 the script cannot read: fail closed), unsupported or
  unavailable. Each round and a final table log every computer's outcome,
  class, `allow_busy` and the guest's counts; the receipt keeps them
  under `marks.outcomes`. Choice, stated: an unsafe refusal aborts the
  reboot and clears the markers even under `--allow-crash-close`, which
  now covers only runs. A crash cuts an apply or a capsule just as a
  planned restart would, and the contract says these computers are never
  restarted through one; the operator waits (`--busy-wait`) or reruns.
- R3 fixed. The facts carry FORWARD's raw rules, and
  `forward_guest_jumps` passes only when rule 1 is `-o vm-+ … -j
  CHOIR-GUEST-RET` and rule 2 `-i vm-+ … -j CHOIR-GUEST-FWD`; a missing
  rule, the reverse order, a rule ahead of them or an unreadable chain
  fails. Pre-check stays a warning; post-check stays blocking.

Residuals:

- The order vmctl produces depends on history: `ensureBaseLocked`
  re-inserts a missing jump at position 1, so if only the CHOIR-GUEST-FWD
  jump were lost and restored, FWD would come first. Both chains end in
  DROP and match opposite directions, so that order is not less safe, but
  the check would fail on it. Fail closed is the chosen side; a reboot
  re-creates the fresh order.
- `--no-markers` still crash-boots every computer without asking.
  Narrowed by round 2 (F5 below): it now aborts on a self-development
  operation, but live capsules stay unchecked.
- Not run against any host. The next reboot is the first live proof that
  the paper boots `planned` with its runs resumed once.

## Findings: review round 2 on the reboot script (bunch 114), 2026-10-22

Sonnet review of lane tip 4d1a201a7 (`_runs/bunch114/sonnet.out`).
Recorded before any fix. Class of the fix: red, as before.

F1. **"Cannot tell" is rebooted through.** `classify_marker_answer` files
a 503, a timeout or connection error (status 0) and any other 5xx under
`unavailable`, and `--allow-crash-close` reboots through every
non-unsafe outcome. A 503 is the guest saying it cannot tell:
`plannedRestartBusy` fails closed with "cannot read self-development
operations" or "cannot read engineering assignments"
(`internal/agentcore/planned_restart.go`). So an operator who passes
`--allow-crash-close` for an ordinary busy computer also cuts an apply
or a capsule on any guest whose store is contended or whose answer timed
out. A 404/405 (`unsupported`) guest is in the same position: it never
said whether an apply runs.

F2. **Each retry round overwrites the last outcome.** `outcomes[vm]` is
replaced every round, and the abort reads only the last one. An `unsafe`
refusal in round 1 followed by a timeout in round 2 becomes
`unavailable`, which `--allow-crash-close` reboots through.

F3. **Markers are not refreshed before the reboot.** A marker written in
round 1 is never re-sent. With `--busy-wait 600`, a marker written at
t=0 while another computer stays busy is 10 to 13 minutes old when the
loop ends, before `systemctl reboot`, the host's POST and boot, vmctl
start and the guest's start; `plannedRestartMaxAge` is 15 minutes, so
the paper very likely boots as a crash (safe, but the patch's purpose is
lost while the log says "marked"). An apply or capsule that starts after
round 1 is also never seen. `marks.marked_at` is stamped after the loop,
not at each write, so the post-check understates the age.

F4. **A failure inside the mark phase leaves markers behind.** (a) An
ssh error (`RuntimeError`) or `subprocess.TimeoutExpired` from
`remote()` (180 s, while each guest may take 25 s: more than seven hung
guests exceed it after the guests have written) propagates to `main()`
with no unmark and no `marks` entry in the receipt. (b) The abort unmarks
only the computers whose 200 came back; a POST whose answer was lost
(status 0) may have written a marker and is not cleared. (c) The unmark
answers are never read, so a failed DELETE still logs "Clearing markers"
and sets `cleared: true`. A stale marker lasts up to 15 minutes, during
which a real crash would boot as planned and resume work, against the
"crash never resumes" rule.

F5. **Some computers reboot with no apply or capsule check.** (a)
`--no-markers` sends nothing, so nothing is checked. (b) Held computers
are not asked. (c) Only active, degraded and booting computers are
listed; the pre-check prints run counts only. Yet the runbook says "No
computer is rebooted through a self-development apply or a live
capsule", which is too broad.

F6 (low). The `--allow-crash-close` line logs only a count, so a
`busy` and an `unavailable` computer look alike there.

F7 (low, tests). No test for unsafe-then-unavailable across rounds, a
503 or timeout under `--allow-crash-close`, retry progress with
`busy_wait > 0`, a `remote()` exception, marker refresh, held computers,
or `--no-markers`.

F8 (low). An older guest whose 409 body lacks `live_capsules` is read as
unsafe (fail closed), so such a guest can never be marked while busy.
Undocumented.

F9 (nit). The tier test reads only the first `case ...: return true` in
`deployRefreshAllowsBusy`.

### Fix, round 2 (lane `lane/fp0-reboot-script-r2`)

- F1 fixed. `--allow-crash-close` now covers only `busy` (runs). An
  `unavailable` (no answer, timeout, 503, other 5xx) or `unsupported`
  (404/405) guest aborts the reboot unless the new
  `--allow-crash-unverified` names that risk. Choice, stated: a separate
  flag rather than calling these `unsafe`, so a guest that is truly dead
  can still be rebooted through on purpose; fail closed stays the
  default. `unsafe` has no flag.
- F2 fixed. `marks.worst` keeps each computer's worst outcome across
  rounds (written < busy < unsupported/unavailable < unsafe) and the
  abort reads it. An unsafe refusal ends the rounds at once: waiting
  cannot change the decision, so the operator reruns after the apply or
  capsule settles. A computer that was busy and then wrote its marker
  counts as written (the purpose of `--busy-wait`).
- F3 fixed. After the rounds pass, a final pass re-sends every marker
  immediately before `systemctl reboot`: the guest replaces its marker
  (`writeHostPlannedRestartMarker`), so the 15-minute TTL starts then, and
  it re-checks for an apply or capsule. A computer written in the rounds
  that does not write again (any outcome) aborts whatever the flags; a
  busy one that answers worse is judged by its new worst. `marked_at`
  is the time the final pass was sent, and each written outcome carries
  its own `written_at`; both are taken when the request goes out, so
  post's age is never smaller than the marker's true age.
- F4 fixed. Every computer is recorded in `marks.sent` before its POST.
  An abort, or any exception in the mark phase (an ssh drop,
  `subprocess.TimeoutExpired`, an interrupt), DELETEs the marker on every
  computer sent, whatever it answered, then re-raises. Each DELETE answer
  is read: a non-200, a missing answer or a failed call is logged as a
  warning naming the computer and kept in `marks.unmark_failed`, and
  `cleared` is true only when every clear answered 200. The ssh timeout
  for a mark or unmark call is 60 s plus 30 s per guest, above the
  guests' 25 s each.
- F5 fixed by a check plus a narrower claim. The remote computer list
  now reads `selfdev.busy` from each running guest's obligations surface
  (the same states the marker refusal counts; `null` when the section is
  missing or reports an error). Computers the script does not ask (held
  ones; every one under `--no-markers`) abort the reboot on a nonzero
  count, before any marker is sent, and need `--allow-crash-unverified`
  when the count is unreadable. The pre-check prints the count for every
  running computer, held ones included. Live capsules have no read-only
  surface, so for unasked computers they are not checked, and computers
  that are not running are not checked at all; the docstring and the
  runbook now say exactly that instead of "no computer is rebooted
  through an apply or a capsule".
- F6 fixed. The proceed log names each unmarked computer, its worst
  outcome and the flag that accepted it, with the apply/capsule risk
  spelled out for unverified ones; the abort log names each blocking
  computer and the flag that would accept it, or none.
- F7 fixed. `scripts/test_node_b_host_reboot.py` adds 20 tests: cannot-
  tell answers under `--allow-crash-close`, the unverified flag and its
  limit, unsafe-then-timeout, timeout-then-busy, busy-then-written with
  `busy_wait > 0` on a fake clock, the final pass (refresh after a long
  wait, an apply that starts after marking, a marker lost in the final
  pass), the scaled ssh timeout, ssh drop and ssh timeout, a lost answer
  cleared on abort, a failed clear reported, and held and `--no-markers`
  computers.
- F8 documented (runbook marker policy): an older guest without
  `live_capsules` in its 409 is read as unsafe and cannot be marked while
  busy. Fail closed, kept.
- F9 fixed. The tier test reads every `case ...: return true` in
  `deployRefreshAllowsBusy`.

Residuals after round 2:

- Live capsules on unasked (held, `--no-markers`) computers are not
  checked; reading them would need a read-only capsule count on the
  obligations surface (`plannedRestartBusy` has one; the surface does
  not expose it).
- The final pass narrows, but cannot close, the window between the
  guest's check and the reboot: an apply admitted in the seconds after
  the final pass is cut. The same window exists for vmctl's deploy
  refresh.
- A computer that boots more than 15 minutes after the final pass still
  boots as a crash (safe fallback); the final pass only moves the start
  of the TTL to the last moment.
- Still not run against any host.

## Findings: review round 3 on the reboot script (bunch 118), 2026-10-22

Sonnet review of lane tip 6b4e1b76e (`_runs/bunch118/sonnet.out`): F1 to
F9 closed. New findings, recorded before any fix. Class: red.

N1. **The computer list is read once, before the busy-wait.** The
unasked check (held computers; every computer under `--no-markers`)
and the set of computers to ask are taken at t=0 and never refreshed.
(A) A held computer, or any computer under `--no-markers`, checked
clean at t=0 can start a self-development apply during the up-to-600 s
wait, and the reboot cuts it. (B) A hibernated or stopped computer that
a user request wakes during the wait is neither asked nor checked, and
the reboot crashes it unexamined.

N2. **The reboot command's exit status is ignored, and nothing clears
the markers after `phase_mark` succeeds.** If `systemctl reboot` is
refused (an inhibitor, a bus error), ssh exits non-zero with the host
up; the script then waits `--boot-timeout` (20 minutes) while the old
boot_id answers, and exits 4 "HOST DID NOT RETURN" though the host never
went down. The markers stay armed for up to 15 minutes, so a crash in
that window boots as planned and resumes work. An exception between
`phase_mark` returning and the reboot (a failed receipt save, an
interrupt) leaves the same armed markers.

N3 (low). In the final pass a computer that was written and now
answers `busy` aborts even under `--allow-crash-close`. Fail closed;
the operator reruns.

N4 (low). The `computers` remote call keeps the fixed 180 s ssh
timeout though it makes an 8 s health and a 15 s obligations request
per running guest in turn; about eight hung guests exceed it.

N5 (low). Live capsules on unasked computers are not checked (an
accepted residual), but the only notice is in each computer's own log
line; there is no summary before the reboot.

N6 (nit). A 404/405 (`unsupported`) guest is asked again every 15 s for
the whole `--busy-wait` though its answer cannot change.

### Fix, round 3 (lane `lane/fp0-reboot-script-r2`)

- N1 fixed. After the rounds pass, the script lists the computers again
  immediately before the final pass (`split_computers`). The unasked set
  (held computers; every one under `--no-markers`) is rebuilt and
  re-checked from the fresh list, so an apply that started during the
  wait aborts; a running, non-held computer not asked before (woken
  during the wait) joins the final pass and is judged like the others;
  an unreadable list aborts and clears. Under `--no-markers` the second
  list is the whole re-check. A computer both asked and now held is
  judged by the worse of its two outcomes.
- N2 fixed. `phase_reboot` reads ssh's exit status: anything but 0, 255
  (ssh's own error, the connection dropping) or an ssh timeout means
  systemctl refused, so every marker in `marks.sent_items` is cleared
  and the run exits 3. If the old boot_id still answers after the new
  `--down-wait` (300 s), the host did not go down: the markers are
  cleared and the run exits 3. A host that never answers keeps the
  markers and exits 4, as before. `reboot_after_mark` wraps the receipt
  save and the reboot: any exception clears the markers and re-raises
  (clearing a computer that already rebooted is harmless: its marker is
  consumed or gone).
- N3 declined, documented. A computer that wrote in the rounds and then
  answers `busy` (or a passing 503) in the final pass aborts even under
  `--allow-crash-close`. Fail closed kept: the earlier marker would be
  armed against a state the guest no longer confirms, and the operator
  reruns. The runbook says so.
- N4 fixed. The `computers` call's ssh timeout is `max(180, 60 + 25 n)`
  for n running guests (from the pre-check list, then from the first
  list), in the mark and post phases. The pre-check's own first call
  keeps 180 s; a timeout there stops the run before anything is written.
- N5 fixed. One `WARNING` line before the reboot names every unasked
  computer whose live capsules were not checked.
- N6 fixed. A 404/405 guest is not asked again in later rounds; the
  final pass asks it once more.
- Tests: 14 more in `scripts/test_node_b_host_reboot.py` (53 in all),
  with a fake host whose list changes between calls and a fake reboot
  command and boot_id.

Gemini review (bunch 118): ACCEPT, no required findings. Its risks are
recorded as residuals:

- The guest requests in one mark call run one after another (25 s
  each), so with many hung guests a final pass takes minutes and the
  first markers age while it runs. `marked_at` is stamped before the
  pass, so post's age is never understated; a marker that ages out
  makes a crash boot, the safe side. Concurrent requests in the remote
  program would shorten it; not done in this round.
- A clear that fails because ssh to the host is gone leaves markers for
  up to 15 minutes; the script logs each one and the runbook says to
  clear them by hand.

Residuals after round 3: as after round 2 (unasked live capsules; the
seconds between the final pass and the reboot; no live proof), plus
the two above.

## Findings: DeepSeek review of round 2 (bunch 118), 2026-10-22

DeepSeek review of 6b4e1b76e, received after the round-3 fix landed
(`_runs/bunch118/out/omp-deepseek-v41-flash.out`). Recorded before any
fix. Class: red.

D1 (medium, a regression of the F4 fix). **Clearing deletes the guest's
own apply marker.** `clear_markers` DELETEs every computer POSTed to,
including ones whose answer proves the host wrote nothing (a readable
409, 400, 403, 404, 405: the handler refuses before
`writeHostPlannedRestartMarker`). The marker path is shared:
`markPlannedRestart` (`internal/agentcore/planned_restart.go`) writes the
same `<store>.planned-restart` file for the guest's own self-development
apply (`self_development_materializer.go`), and the DELETE handler clears
it unconditionally. So when the script aborts on `unsafe`, the very case
in which an apply is running, the clear can remove the apply's marker:
its restart becomes a crash boot and spends one of the apply's durable
attempts (`guestApplyAttemptLimit`).

D2 = N1 (the unasked check is a single snapshot), already fixed in
round 3 (e7540b0a6).

D3 (low). A computer whose marker the final pass did not renew is logged
as "<outcome> in the final pass (was written) (no flag accepts it)"; the
reason it stops the reboot (a marker written earlier was not renewed;
rerun) is not said.

D4 (low). The `--allow-crash-unverified` help does not say that a
computer that wrote earlier and then cannot answer in the final pass
aborts anyway.

D5 (nit). `marks.marked_at` is set even when the final pass aborts and
the markers are cleared.

### Fix, DeepSeek round (lane `lane/fp0-reboot-script-r2`)

- D1 fixed. The receipt keeps `marks.may_hold_marker`: every computer
  that answered 200 in any round, gave no answer (status 0), answered
  5xx (`writeHostPlannedRestartMarker` can fail after the rename), or was
  in a mark call that failed in flight (`marks.in_flight`). Every clear
  (an abort, a failure in the mark phase, a refused reboot, a host that
  did not go down, a failure after marking) DELETEs only those, and logs
  the ones it skips. A computer that refused before writing (a readable
  409, 400, 403, 404, 405) is not cleared, so the guest's own apply
  marker survives. Choice, stated: a computer that wrote earlier and then
  refuses (for example `unsafe` in the final pass) is still cleared. Its
  file may now hold the apply's marker rather than the host's, but it
  may also still hold the host's, and leaving that would let a crash in
  the next 15 minutes boot as planned. The crash-never-resumes rule wins;
  the cost is that the apply's restart may become a crash boot.
- D2 is N1, fixed in round 3.
- D3 fixed. A computer the final pass did not renew is logged as
  "marker not renewed in the final pass (answered <outcome>)", with "a
  computer marked earlier must write again; rerun once it answers". An
  `unsafe` final answer keeps its own message.
- D4 fixed. The `--allow-crash-unverified` help says it does not cover a
  computer that wrote earlier and does not write again in the final
  pass; the runbook already said so.
- D5 fixed. `marks.marked_at` is set only when the final pass succeeds.
- Tests: six more (59 in all). Four earlier expectations that a refusing
  computer is cleared now expect that it is not.

Residual: a guest cannot tell the host's marker from its apply's
(one file, one unconditional DELETE). Clearing only the host's own
marker would need the DELETE to match the reason or target it was
written with, which is a guest change, not this script's.

## Findings: review round 4 on the reboot script (bunch 120), 2026-10-22

Sonnet review of lane tip 69c2d2515 (`_runs/bunch120/sonnet.out`):
accept; the earlier findings are closed. New findings, recorded before
any fix. Class: red.

R1 (low-medium). **A 503 the guest sends before writing is cleared like
a write.** `may_hold_marker` takes every status >= 500. The guest's two
503s both come before `writeHostPlannedRestartMarker`
(`HandleInternalPlannedRestart`, `internal/agentcore/planned_restart.go`):
`{"error":"planned restart surface unavailable"}` and
`{"error":"planned restart: <err>"}` when `plannedRestartBusy` cannot
read its self-development operations or engineering assignments. That
second 503 is the answer of a guest whose store is contended during an
apply, so the clear can delete the apply's own marker (D1's case
again). Only the handler's 500 follows a write attempt.

R2 (low-medium). **An ssh exit 255 or timeout is always read as "the
host went down".** ssh exits 255 both when the connection drops as the
host goes down and when it never connected or authenticated
(connection refused or timed out, MaxStartups, a key refused). In the
second case the reboot was never sent. If `ssh_ok` then fails for the
whole `--boot-timeout`, the run exits 4, which keeps the markers, so
they stay armed for up to 15 minutes against a host that never
rebooted. The runbook does not say that exit 4 leaves markers armed.

R3 (low). **SIGHUP and SIGTERM skip every clear.** Only Python
exceptions (KeyboardInterrupt included) reach the clear paths. If the
operator's terminal or ssh session dies during the busy-wait or the
final pass, the default SIGHUP action ends the process at once and the
markers stay for 15 minutes with no reboot.

R4 (low). A host still shutting down (VMs stopping, units waiting out
their stop timeout, sshd still up) can answer with the old boot_id at
`--down-wait` (300 s); the run then clears the markers and exits 3 while
the host goes down, so every computer boots as a crash.

R5 (low). The re-list comes before the final ask, which takes up to
25 s per guest, so a computer that wakes or a held one that starts an
apply during the ask is not seen. An asked computer that stopped during
the wait answers `unavailable` in the final pass and blocks (fail
closed, noisy).

R6 (nit). A POST that timed out (status 0) can still reach the guest
after the DELETE that follows it.

### Fix, round 4 (lane `lane/fp0-reboot-script-r2`)

- R1 fixed. `answer_may_have_written` reads a 503's body: the guest's
  own two pre-write 503s, `{"error":"planned restart surface
  unavailable"}` and `{"error":"planned restart: <err>"}` (matched
  exactly against `HandleInternalPlannedRestart`), count as refusals and
  are not cleared. Any other 503 body (a proxy, an older guest), a 500
  or another 5xx, and a lost answer still count as a possible write.
- R2 fixed. On an ssh exit 255 or an ssh timeout, the script reads ssh's
  stderr. A connect or authentication failure (`connect to host`,
  `Permission denied`, `Could not resolve hostname`, `Host key
  verification failed`, `kex_exchange_identification`, `Too many
  authentication failures`) means the command was never sent: the
  markers are cleared and the run exits 3 (`reboot.not_sent`). Bare
  `Connection timed out` and `No route to host` are left out on purpose:
  ssh also prints them after "Read from remote host" when the session
  drops as the host goes down; a connect failure carries the "connect to
  host" prefix. A drop after connecting still waits for the new boot. The
  runbook now says exit 4 keeps the markers armed and how to clear them
  by hand.
- R3 fixed. From the mark phase on, SIGHUP and SIGTERM raise
  `Interrupted` (a `KeyboardInterrupt`), so the mark-phase clear and
  `reboot_after_mark`'s clear both run. SIGKILL cannot be caught; the
  runbook says so.
- R4 recorded, default kept. A shutdown slower than `--down-wait`
  (300 s) clears the markers and exits 3 while the host goes down, so
  every computer boots as a crash: the safe side, which costs the
  planned resume. The flag stays at 300 s; the runbook says to raise it
  if shutdowns run long and to run `--phase post` after a late reboot.
  Reading `systemctl is-system-running` ("stopping") first would tell
  the two apart; not done.
- R5 recorded as a residual. The re-list comes before the final ask, so
  the unasked check is as old as the ask (up to 25 s per guest) plus the
  reboot; a computer woken, or a held one that starts an apply, in that
  time is not seen. An asked computer stopped during the wait answers
  `unavailable` in the final pass and blocks (fail closed); the operator
  reruns.
- R6 recorded as a residual. A POST that timed out (status 0) can still
  reach the guest after the DELETE that follows it, leaving a marker for
  up to 15 minutes. Inherent in a request whose answer is lost.
- Tests: 7 more (66 in all), plus guard cases that a mid-session drop
  keeps the markers.

Gemini review of 69c2d2515 (bunch 120): ACCEPT, no required findings;
its risks (serial guest requests, the second between the final pass and
the reboot, unasked live capsules) are residuals already recorded. The
DeepSeek review had not arrived when this round landed.

## Findings: DeepSeek review of round 4 (bunch 124), 2026-10-22

Verdict ACCEPT, conditional on F1. Read against `d1e5d5819`; 66 tests
pass there. Its three round-4 checks held (the two 503 literals match the
guest and come before the write; the ssh phrases cannot match a drop after
the session exists; the handlers are installed after `--execute` and
`--console-verified`, before the mark phase).

F1 (medium-low, fail-open, required). `mark_rounds.ask` sets
`m["in_flight"] = []` right after `remote()` returns, before the loop that
fills `may_hold_marker`. A SIGHUP or SIGTERM (now an `Interrupted`), or
any exception, inside that loop reaches `phase_mark`'s `except
BaseException`, which clears `may_hold_marker` united with `in_flight`:
both empty. `clear_markers` then skips every sent computer while it logs
that it clears every marker that may exist. The host marker stays armed
for 15 minutes with no reboot, so a crash in that window boots as
planned, the state the red rule forbids. Reproduced by the reviewer on
the FakeHost harness (guest answers 200, a signal at the first
`answer_may_have_written`, `unmarked == []`).

F2 (low). `main` catches `Exception` around the phases, which misses
`Interrupted` and `KeyboardInterrupt`. Two gaps: (a) between `phase_mark`
returning True and the `try` in `reboot_after_mark`, an interrupt leaves
the markers armed with no clear; (b) an interrupt that `phase_mark` or
`reboot_after_mark` clears and re-raises escapes `main` without `save()`,
so the receipt that records what was sent is lost.

F3 (low, doc). The runbook says an exit 4 keeps the markers armed and a
crash in the next 15 minutes boots as planned. Exit 4 comes about 1230 s
after the final pass (30 s sleep plus `--boot-timeout` 1200), past
`plannedRestartMaxAge` (15 minutes, `internal/agentcore/planned_restart.go`),
so at the defaults every marker is already stale and boots as a crash. The
claim only holds when `--boot-timeout` is under 900 s.

F4 (low). OpenSSH localizes its error messages. Under a non-C
`LC_MESSAGES` none of the `SSH_NOT_SENT` phrases match, and R2 quietly
degrades to waiting and exit 4 (fail-safe, but the exit-3 path is dead).

F5 (nit, carried over). `--down-wait >= --boot-timeout` is not
validated; the down-wait branch is then unreachable and a host that is
still up exits 4.

F6 (nit). `answer_may_have_written` treats every other status below 500
as a refusal before the write. An unknown 4xx (408 or 429 from something
in front of the guest) is then never cleared. Unreachable with today's
guest, whose pre-write answers are 400, 403, 404, 405, 409 and the two
503s.
