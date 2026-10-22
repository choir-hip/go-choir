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
- `--no-markers` still crash-boots every computer without asking, so it
  also crashes through an apply or a live capsule. Unchanged: it is the
  operator's explicit choice of pure crash semantics.
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
