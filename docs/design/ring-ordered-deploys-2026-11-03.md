# Ring-ordered deploys and automatic code rollback (2026-11-03)

Station: `docs/definitions/choir-progressive-delivery-2026-11-03.md`, slices
P1 (design S5) and P2 (design S6, the guest-release half). Parent design:
`docs/design/progressive-delivery-2026-10-26.md` sections 3, 5, 6, 9 and the
owner rulings of 2026-11-03 (no switches; rollout is the deploy; the paper
refreshes automatically, last). Mutation class: red (vmctl, deploy routing,
VM lifecycle). Review: tier 2.

## What exists

- A deploy moves the guest link to a new guest release (NixOS activation).
  CI then asks vmctl for every active interactive computer
  (`GET /internal/vmctl/deploy-refresh`) and refreshes each one through the
  guest's planned-restart marker (`POST`), serially. A busy guest refuses and
  is recorded in the pending set; vmctl's retry sweep (every minute) asks it
  again until it takes the update. `critical_protected` and
  `public_platform` computers are asked with `allow_busy`; the guest still
  refuses mid self-development apply or with a live capsule.
- A cold boot launches from the current release (`bindCurrentRelease`); a
  snapshot wakes onto its own release and is enrolled `woke_behind`, so the
  sweep refreshes it at the user's next idle moment.
- Nothing orders the refreshes, and nothing reads health after them.

## What changes

One new piece of vmctl state, the **ring walk** (`ring-walk.json` beside the
pending set) and its append-only **history** (`ring-walk-history.jsonl`, the
artifact). No switch, no env, no flag: the walk runs on every deploy that
moves the guest release.

1. **Rings, by id** (design 3.1, review 340 F1). R0 d@e
   (`computer-59c902c5...`), R5 the paper (`computer-1355b83b...`), R2 the
   owner's computer (`computer-03335285...`), all three as constants in
   `internal/vmctl/ring_walk.go`; R1 every computer whose live class is
   `test`; R4 every `public_platform` computer; R3 every other computer,
   including a new one. Changing a membership is a commit.
2. **Start.** Each sweep tick (and each deploy-refresh GET or POST) reads the
   current single and layered release. When it differs from the walk's
   target, a new walk starts: `from` is the last release that finished a
   walk (`last_good`; at first start, the current release, with no walk),
   `to` is the new one. A walk in flight is superseded and recorded so.
3. **Containment by pin.** The manager asks the walk, at every cold boot and
   in every release rule, which release a computer launches from
   (`ReleasePin`). A computer whose ring has not opened is pinned to `from`;
   after a rollback every computer is pinned to `from`. A pinned release that
   does not load falls back to the current one (logged): a pin never fails a
   boot. Retention keeps every pinned release (`in_use` with `pinned_by`).
4. **Gate.** A deploy refresh of an active computer whose ring has not opened
   answers `skipped` ("ring R3 waits ...") unless its target is the commit
   of the release it is pinned to (a return). CI's `GET` lists such
   computers as `ring_waiting`, not as candidates, so the CI deploy refreshes
   only R0 and verifies it as before. A rescue of a down protected computer
   is never gated: it boots the release its ring is pinned to, and the plan
   names that commit (`ring_commit`) for CI's verification.
5. **Walk.** A ring opens: the walk records its members and enrolls each
   active one in the pending set with the target commit (once per ring), so
   the ordinary sweep refreshes it, marker-backed, busy computers at their
   next idle moment, the paper with `allow_busy` and never mid
   self-development apply or with a live capsule (the guest's refusals stay).
   The `woke_behind` path now compares a computer against its own resolved
   release, not the global current, so a pinned computer is not "behind".
6. **Soak** (design 3.2, code-deploy reading). A ring is clean when its time
   floor has passed (R0 10m, R1 15m since the cycle-time ruling below; R2
   4h, R3 4h, R4 2h, R5 1h) and every
   active, non-held, non-divergent member has been seen healthy on the
   target release, at least min(floor count, members) of them (R1 needs 3,
   R3 2, the rest 1). A member seen healthy on the release and since gone
   idle (hibernated) counts as exposed. A ring with no members is skipped and
   recorded. A ring whose members were all down for its floor is skipped and
   recorded, except R0, which holds: no release passes the proof ring
   unseen. d@e is usually hibernated between uses, so after the deploy's
   30-minute grace the walk wakes a down d@e once per walk, the way a request
   for it would (route check, then resolve of the existing computer).
   The next ring opens only on a clean ring. The paper opens last, after R4.
7. **Guards** (P2), per member on the target release, every tick:
   - readiness: `/health` not 200 or no answer, 3 consecutive samples, after
     a 5-minute initial delay from the first sample on the release (the
     paper is unreachable for minutes after boot, F1);
   - appender wedge: `projection_repair_required` in the `/health` body, 2
     consecutive samples (the guest already needs 3 refusals over 2
     minutes);
   - boot success: a `boot_failed` crash record after the ring opened;
   - crash restarts: any other crash record after the member was first seen
     on the release (zero tolerance, design 5.3).
   Per ring: one member's breach breaches the ring. Each guard has a replayed
   known-bad fixture (`internal/vmctl/testdata/ring_walk/`), design 9.2.
8. **Rollback.** A breach in ring k: the walk becomes `rolled_back`, every
   computer is pinned to `from`, every member of rings 0..k running the
   target is enrolled for a return refresh to `from`'s commit (marker-backed,
   planned, the same refusals), and the walk stops. One rollback per walk:
   a rolled-back walk evaluates no guards, so the return cannot loop. The
   next release starts a new walk from `last_good`. A crash-looping member
   is already held by the crash-loop bound; its next boot (owner start)
   launches `from` because the pin is read at boot.
9. **History.** One JSON line per event (walk started, ring opened, ring
   closed, breach, rollback, rollback complete, walk complete, superseded).
   A ring-closed line carries the release pair, the computers, the soak
   readings per computer, the outcome and `prediction_ids` (empty until
   releases carry them: the calibration bridge,
   `docs/design/calibration-seed-2026-11-03.md`). Settlement is not built.
   `GET /internal/vmctl/ring-walk` serves the state and the history tail.

## Cycle time (owner, 2026-11-03)

Owner: "for stuff related to an optimization loop it's important to speed
up the cycle time." A test computer should get a platform fix about 25 to
40 minutes after a deploy, not hours.

- **R1 is real.** On staging R1 (class test) held 2 computers, both down;
  the four self-development arms and every computer `scripts/test-computer-
  create` makes were class primary, so they walked in R3. The arms are now
  listed in `internal/computertiers/computer-tiers.json` `test_computers`,
  and `test-computer-create` adds each computer it makes there (the tier
  file is embedded in vmctl, so the entry takes effect when committed and
  deployed). Test tier is class test: ring R1, standing autonomy, and the
  pressure-reclaim rule that a test computer yields first. Retention deletes
  only ephemeral accounts (email domain or user-id prefix), not tier test
  computers.
- **Minutes for R0 and R1.** R0's floor is 10 minutes and R1's 15, from the
  ring opening; the deploy grace before the walk enrolls or wakes d@e is 7
  minutes (below the floor, so CI's refresh lands first); the readiness guard
  can breach from about 8 minutes on a one-minute tick, inside the floor; exposure and the guards are unchanged. R2 onward keep their
  floors. Expected: d@e refreshed by CI at the deploy, R0 closes at about 10
  minutes, R1 opens and enrolls the arms (an arm mid capsule or apply
  refuses and goes at its next idle moment), R1 closes at about 25 minutes.
- **Measured.** Each `ring_closed` line and the status carry `timing`:
  seconds from the walk start (the deploy moving the release) to the ring
  opening, to its closing, and to the last of its members seen on the
  release. R1's `last_reached_after_seconds` is deploy-to-last-arm-
  refreshed. The walk keeps `reached` (first time each computer was seen on
  the release, any ring), so a member refreshed after its ring closed still
  counts.

## Expedite (owner, 2026-11-03)

Policy (owner, 2026-11-03): "For these blocking fixes we should use expedited
delivery so changes propagate not blocking on wait. Not just now but as
policy." A fix that unblocks a loop (self-development rounds, the paper,
a deploy path) carries `Ring-Expedite: <the defect it unblocks>`; CLAUDE.md
carries the rule.

Owner: "we should have some way to force changes to all machines instantly.
Important for key fixes." An expedite skips the soak floors and the ring
order for one release; it does not skip safety.

- **Triggers.** (a) A commit trailer `Ring-Expedite: <reason>` on any commit
  in the deploy's range (the deployed commit in the receipt, exclusive, to
  the deploy commit). `node-b-ci-deploy` reads the range from the gate's
  checkout (read only, no gate change) and, before it takes the refresh
  plan, posts the expedite to vmctl with `by: "trailer <sha>"`. (b) An owner
  or operator act for a release already deployed:
  `POST /internal/vmctl/ring-walk/expedite {"reason", "by"}` (internal
  callers only; `by` is required and names who asked).
- **Effect.** vmctl first reads the current release (so a deploy's new walk
  exists), then marks that walk expedited: every remaining ring opens at
  once (the paper included), so no computer is pinned or gated, the CI plan
  lists every active computer as a candidate, and the walk enrolls every
  active computer off the release. An expedite with no walk (the release did
  not change) or on a rolled-back walk is refused (409) and recorded.
- **Unchanged.** Every refresh is the ordinary planned, marker-backed one;
  the guest still refuses mid self-development apply, with a live capsule,
  or on release skew, and a refused computer stays pending and goes at its
  next idle moment (the paper with allow_busy, never mid apply or with a
  capsule). The guards keep evaluating every member on the release in every
  ring, and a breach rolls back every computer that took it (all rings are
  open, so all are returned).
- **Closing.** Each ring closes once 10 minutes have passed since the
  expedite and every active, eligible member is seen healthy on the release;
  in R1 to R4 a member still pending (busy) does not hold its ring, and one
  whose refresh to this target failed never does. R0 and R5 are watched
  (review 3): R0 holds until d@e is seen healthy on the release (the walk
  wakes it at once, with the bounded retry), and R5 holds until the paper's
  refresh has landed and is seen healthy, so neither takes the release
  unguarded. The walk completes, and the baseline moves, only when every ring
  has closed. A rollback names the breached ring and closes every other open
  ring `stopped_by_rollback`.
- **Bound to a release.** The act may name the release (`release`: its
  commit or id); CI always does (the deploy commit) and retries the post. An
  expedite naming another release is refused, so it never lands on an older
  walk that is still in flight. A second expedite of the same walk is a
  no-op. Reason and `by` are stripped of control characters.
- **Record.** `expedite` history line with the reason, `by`, and the time;
  status shows `expedited: true`, `expedite_reason`, `expedited_by`; the
  CI plan's `ring_walk` carries `expedited` (and `open_ring: all`). The
  history is the source for naming every expedite in a report.

## Decisions after review 1 (Devin, 2026-11-03)

- **A release that returns to the baseline id ends the walk without rings.**
  When the current release becomes `last_good` again (a revert that rebuilds
  the identical image, or a host rollback), the walk ends, pins clear, and
  the deploy refreshes every active computer onto it as before. Decided, not
  an accident: that release already finished a walk, and holding computers
  on a release nothing proved (the rolled-back or superseded target) longer
  is the worse risk. A revert that produces a new release id walks the
  rings like any other release.
- R0 with no member observed holds (never `skipped_empty`); a walk enroll
  that was skipped (held, down) is re-enrolled at most every 10 minutes,
  never after a failed refresh to the same target; a crash after the last
  tick that saw the member off the release counts; walks compare release ids,
  not commits; a computer that woke behind keeps its user-idle gate.
- DeepSeek round 1: an empty observation set holds every ring; exposure is
  min(floor count, eligible members) seen healthy, so one busy member does
  not hold a large ring (its pending entry still refreshes it); the boot
  guard counts only members asked onto the release, never held or divergent
  ones; a layered computer is on target only on the layered release; an
  unobserved computer is pinned to `from`; health probes run 8 at a time
  under a 25-second tick deadline; a rescue's `ring_commit` is the commit
  the manager would boot; the state file keeps a `.bak` and loads it when
  torn; CI reports a walk whose proof computer it did not refresh as
  `ring_held_unproven`. Residual: retention reads `pinned_by` once per run,
  so a run that read usage just before a walk started could delete `from`
  (the baseline is normally also its "previous" keep); a pin that does not
  load falls back to the current release and logs it.

## Decisions after review 2 (Devin accept, DeepSeek no blocking)

- **Guards cover the open ring only (deliberate).** A ring that closed clean
  is not sampled again in this walk: its exposure was the evidence, and the
  outer rings carry their own guards. A late regression on an inner ring is
  caught by the crash-loop bound for the computer and by the next walk; a
  late-regression window over closed rings is a later slice (design 9.1).
- Taken: the readiness delay re-arms at every boot seen on the release; a
  crash counts against the release only while the member runs it; after a
  rollback, any computer found on the rolled-back release (an in-flight
  refresh that landed, a resumed snapshot) is enrolled for a return, and
  rollback completes only when none runs it.
- Residuals: CI's `ring_held_unproven` reads candidate membership, not the
  walk's own observation of d@e; a CI-time failed refresh of d@e holds R0
  (and the fleet on `from`) until the next deploy, by the no-kill-loop rule.

## Failure modes

- **A ring skipped silently.** Every skip writes a history line with its
  reason; R0 never skips for want of exposure.
- **Stuck computer.** The walk never holds a computer: an unopened ring runs
  `last_good`. A walk that holds (R0 asleep, a member that never refreshes)
  is visible in status and is superseded by the next deploy. A failed
  refresh is not retried by the walk (one enroll per member per ring), so no
  kill loop.
- **The paper at the wrong moment.** It refreshes only through the sweep
  once R5 opens (every other ring clean), marker-backed, refused by the
  guest mid apply or with a capsule. CI never refreshes it during a walk.
- **Rollback loop.** One rollback per walk; no guards after it; the return is
  a planned refresh refused while busy, retried by the existing bounded
  sweep.
- **Crash vs planned.** Forward and return refreshes are marker-backed
  planned updates (resume once). A crash boot of a pinned computer is
  markerless and closes work, as before; the pin only picks the release.
- **Starvation.** A walk takes about 13.5 hours end to end; deploys more
  often than that supersede it, so the outer rings and the paper can trail
  `last_good` for days. Recorded as a residual: the history shows it, and a
  later slice may let a clean ring prefix carry over.
- **Leak window.** A cold boot between the link moving and the walk's first
  read of it (at most one tick, and vmctl reads it at start before reattach)
  launches the new release outside its ring.
- **Drills on d@e.** A chaos drill that crashes d@e during R0's soak rolls
  the release back; that is the guard working, at the price of one return
  refresh of d@e.

## Proof plan

- Unit (written first): ring membership; containment pin; gate; soak and
  exposure; each guard against its known-bad fixture and a known-good one;
  rollback enrolls only target-running members and pins all; one rollback
  per walk; supersede; paper last; history lines; restart of vmctl keeps the
  walk.
- Repo gates, CI, the staging deploy.
- Staging acceptance: (1) the deploy that ships this records the baseline;
  (2) a planted bad release (guest `/health` reports an appender wedge on
  d@e only) walks to R0, breaches, rolls d@e back to `from`, and R1 never
  opens; (3) the next release with a new id walks R0 clean and opens R1 (a
  revert that rebuilds the baseline image ends the walk instead, as decided
  above). The history file is the artifact.
- Rollback of this change: revert the commits; deploys return to
  refresh-all; the state files are ignored by the old vmctl.
