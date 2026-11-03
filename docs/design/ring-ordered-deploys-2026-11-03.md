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
   floor has passed (R0 30m, R1 2h, R2 4h, R3 4h, R4 2h, R5 1h) and every
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
  opens; (3) the revert walks R0 clean and opens R1. The history file is the
  artifact.
- Rollback of this change: revert the commits; deploys return to
  refresh-all; the state files are ignored by the old vmctl.
