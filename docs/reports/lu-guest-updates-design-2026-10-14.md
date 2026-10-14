# LU: Guest Updates (design note, round 3)

Date: 2026-10-14. Station: LU of the ledger metamission
([station file](../definitions/choir-ledger-lu-guest-updates-2026-10-14.md)).
Owner, in session, 2026-10-14: "we do have to just improve the updating
system in general. Like, I want to be pushing safe updates out to guest
computers. And similarly, I think this is going to relate to the
self-development system." Approved as a station: "Yes, this all makes
sense as a new station." This note is green (docs). The change it
specifies is **red**: deployment routing, vmctl, VM lifecycle, restart
semantics, run admission, self-development apply.

Facts below were read at `main` 97d7a573 (staging deploys 97d7a573,
deployed 10:40:40 UTC). Claims marked *hypothesis* are not yet confirmed by
a trace.

## Changelog

- **Round 1** (346d968c): first draft.
- **Round 2** (cca944f7): a convergent panel (claude, gemini, glm, muse,
  deepseek: accept with repairs; gpt-6-luna: send_back) found that the
  drain as written would block dispatcher slots, could not tell
  continuations from new work, shared one flag with hibernation, raced its
  own 2-minute fence, and had two update drivers; and that the busy paper
  could not use a drain it did not yet have. Adjudication:
  [`lu-guest-updates-adjudication-2026-10-14.md`](lu-guest-updates-adjudication-2026-10-14.md).
  Changes: the drain defers held work instead of blocking it (R1, R2),
  admits by run lineage (R3), uses a gate with reasons (R4), ends in one
  synchronous final call (R5); vmctl is the only update driver (R6); LU1
  splits into a host-only LU1a and the drain LU1b (R7); LU2 persists the
  epoch and moves up (R12, R13); LU3 splits into LU3a and LU3b (R15),
  refuses rollback across store schema (R14) and never advances an
  unvalidated ring (R16); LU4 becomes conditional (R19); smaller fixes
  R8-R11, R17, R18, R20-R39.
- **Round 3** (this file): the round-2 panel (claude, gemini, glm,
  deepseek: accept with repairs; muse, gpt-6-luna: send_back) found every
  round-1 blocker repaired and named new gaps, adjudicated as S1-S31 in
  the same document. Changes: the final call is a recoverable protocol
  (a durable host record and sweep completion, a finalization barrier, no
  updater restart, abort and deadline for `update_final`; S1-S6, S25,
  S26); lineage admission is a defined, fail-closed predicate with a
  bounded ancestor walk and a rule for self-development retries (S7, S8);
  update holds defer in their own class outside the poison budget (S9);
  LU2 reads the epoch at open and advances it only inside each step's
  transaction, covers the actor log, seeds the first boot and holds on a
  downgrade (S10-S14); LU1a names its interfaces and uses vmctl's idle
  test (S16, S17, S20); rings pin the image and treat stuck as not failed
  (S15, S19, S24); LU no longer depends on L1 as a whole (S21).

## 0. The short version

Today a deploy pushes: CI asks each running computer once, at deploy time,
whether it is idle; a busy computer is skipped until the next deploy, and
nothing retries it in between. The paper updated only through an owner
act on 10-14. When it did update by itself, a work-state epoch change
turned the planned restart into a crash. There is no health gate after an
image refresh and no automatic way back.

LU turns this into pull with a deadline:

1. **Pending, not pushed.** A deploy changes the target build. A computer
   whose booted image differs from the target is *pending*. vmctl's sweep,
   the only update driver, offers the update at every pass.
2. **Quiet moment, then drain.** The guest takes the update when it is
   quiet. If no quiet moment comes within a wait (default 30 min), the
   host asks the guest to drain: defer new work, let work already running
   finish within a deadline (default 15 min), then restart. Schedules are
   not closed. At the deadline the guest reopens and the update is
   deferred, never forced.
3. **Planned stays planned.** The work-state epoch is persisted; a change
   runs a registered migration at boot instead of reading as a crash.
4. **Rings, health, rollback.** Rollout goes test computers, then the
   paper, then the owner's computer, then everyone else, and stops on a
   failure. Later, the new build runs under a probation hold and the host
   boots the previous image once if it fails.
5. **Lighter restarts, if measured.** When only the guest runtime changed,
   the guest updater can swap the runtime and restart one unit. Built only
   if planned-refresh downtime turns out to matter.
6. **One apply path.** Platform updates and approved self-development go
   through one release-apply transaction.

Slice order: **LU1a** (host-only retry) and **LU2** (migrations) first, in
parallel; then **LU1b** (drain); **LU3a** (rings); **LU3b** (probation and
rollback, after L1c and L1b); **LU5** (one apply path); **LU4** only if
measured.

## 1. What happens today (code map)

### 1.1 The deploy path (host)

- CI's "active computer refresh" step lists candidates from
  `GET /internal/vmctl/deploy-refresh` (`.github/workflows/ci.yml:1404-1420`).
  The selection is every active interactive computer except held ones
  (`internal/vmctl/deploy_refresh.go:177-204`).
- CI refreshes them one at a time (`max_parallel_refresh=1`,
  `ci.yml:1427`), each with a 300 s timeout (`ci.yml:1451`). A computer
  already serving the target is "current" (`ci.yml:1439-1446`).
- Per computer, vmctl asks the guest to write its planned-restart marker
  (`deploy_refresh.go:276-277`). 409 means busy: **deferred to the next
  deploy** (`deploy_refresh.go:282-283`; `ci.yml:1466-1469`; summary line
  `ci.yml:1528`). A guest without the endpoint is refreshed only when its
  `/health` reports no work, as a crash boot (`deploy_refresh.go:284-291`).
  Anything else is deferred (`:292-293`).
- The refresh is `RefreshVMForDesktop` → vmmanager `RefreshVMWithConfig`,
  which SIGKILLs Firecracker (`internal/vmmanager/manager.go:1447`) and
  boots the current image. There is no guest quiesce or filesystem sync
  before the kill.
- The manual dispatch input `refresh_active_computers=true` force-refreshes
  deferred computers through the operator refresh, a crash boot
  (`ci.yml:11`, `:1470-1490`).
- Nothing retries a deferred computer between deploys. Docs-only pushes do
  not deploy at all.
- The guest image is one immutable Nix store output; `/var/lib/go-choir/guest`
  is a symlink to it, swapped at host activation
  (`nix/node-b.nix:955-1000`), and vmctl boots from the stable paths
  (`nix/node-b.nix:725-729`). The previous image survives only as long as
  an older system generation roots it. vmctl records no per-computer
  booted-image identity; CI infers currency from the guest's `/health`
  `build.commit` (`ci.yml:1439`).

### 1.2 Boot kinds and the marker (guest)

- `POST /internal/runtime/planned-restart` (commit bcc556de) writes the
  marker when the guest is not busy and answers 200 once it is durable
  (`internal/agentcore/planned_restart.go:361-416`). Busy means executing
  activations or a self-development operation mid-transaction
  (`plannedRestartBusy`, `:321-336`). It closes no admission gate and does
  not count desk pending mutations, which vmctl's own idle test does
  (`internal/vmctl/ownership.go:3108-3125`).
- `RunningCount` counts activations executing now (`runtime.go:2283`;
  added at `:414`, removed when the activation returns, `:848-855`). A
  parked run waiting for a child result is not counted. One activation may
  run up to the activation budget, 60 min by default
  (`internal/provideriface/config.go:24`).
- The marker lives 15 minutes (`planned_restart.go:26`), carries the
  binary's work-state epoch (`:64`), and is consumed once before the
  runtime starts (`:109-142`). A marker whose epoch differs from
  `CurrentWorkStateEpoch` (now 2, `:35`) makes the boot a crash
  (`:133-136`, decision D7). The epoch is not persisted anywhere but the
  marker.
- `Runtime.Start` branches on the boot kind (`runtime.go:768-774`):
  planned boots passivate activations; crash boots close pre-boot runs and
  open rounds. Source schedules survive both (ruling 6, ce59d406;
  `internal/agentcore/source_schedule.go:290-372`): a planned boot keeps
  the generation and re-arms the saved tick; a crash closes the poll in
  flight and pending arrivals and bumps the generation (`:371`).
- In-guest applies still write their own marker (`markPlannedRestart`,
  `planned_restart.go:291-311`), bounded at 3 starts per apply (`:215`,
  L1a-2). L1c moves planned-ness to the host.

### 1.3 The in-guest apply path (guest updater)

The root-owned `choir-updater` (`cmd/choir-updater/main_linux.go`,
`internal/updater/updater.go`) already does most of what LU asks for, for
layered releases:

- `Apply` (`updater.go:192`) checks the base join and the store-schema
  window before any mutation (`:270-292`, `:419-457`), stages and verifies
  the release, materializes its closure, writes a journal, swaps the
  `current` pointer (`:329-337`), publishes a restart of the runtime unit
  through a systemd path unit (`:338-364`; `nix/autoputer-vm.nix:474-510`),
  probes health (`:365-395`), signs health and materialization receipts,
  and on probe failure restores the prior release and restarts it
  (`restorePrior`, `:943-994`) with a signed recovery receipt.
- The store persists its schema epoch at open (`storeschema.Write`,
  `internal/store/store.go:678-682`) so the updater can refuse a release
  that cannot read it (the vm-3dc68688 class, `updater.go:70-79`).
- The boot wrapper serves a layer only on the base it was built for
  (`nix/autoputer-vm.nix:133-142`) and falls back to the base runtime after
  3 failed starts in 60 s (`:143-180`).
- Two callers: the self-development materializer
  (`internal/agentcore/self_development_materializer.go:315-453`, recorded
  as `materialization_started/applied/failed` events, then checkpoint and
  route) and the M9a platform-update handler
  (`internal/agentcore/platform_update.go:75`, served at `:774`).
- **CI no longer drives the platform-update offer.** The app-layer push
  was deleted in a47122de after it caused version skew (new realizations
  booted older code) and landed on 0 of 2 computers
  (`docs/problems/guest-runtime-fix-never-reaches-layered-computers-2026-10-09.md`;
  bind gaps in `docs/problems/s2-app-layer-offer-bind-gaps-2026-10-05.md`).

### 1.4 Admission holds today (guest)

There are four, each separate (R22):

- the **hibernate fence** (S3): every actor occurrence enters through
  `Runtime.EnterWork` (`internal/actorruntime/handler.go:171`;
  `internal/agentcore/hibernate.go:163-172`); the fence closes only while
  nothing is inside (`hibernate.go:107-119`), blocks new occurrences in
  `enter` until lifted or a 2-minute TTL (`:35-39`, `:129-160`), and one
  flag carries it (`:93-104`). Source polls enter through it
  (`handler.go:206`);
- the **self-development apply hold**: after a planned apply boot, restart
  work kinds return `actor.ErrDeferUnprocessed` while the apply is
  materializing (`handler.go:61-90,165-168`), so held work stays pending
  and uses no dispatcher slot;
- the **pre-boot gate**: crash boots refuse pre-boot work occurrences
  (`handler.go:109-119,160-164`);
- the **maintenance hold** (`RUNTIME_MAINTENANCE_HOLD=1`,
  `runtime.go:810-818`).

The dispatcher runs at most 8 activations at once (`internal/actor/dispatcher.go:159-161`)
and poisons an occurrence after 64 deferrals (`internal/actorruntime/adapter.go:137`),
with backoff doubling from 0.5 s to a 30 s cap (`dispatcher.go:18-27`):
about 29.5 minutes of deferral.

### 1.5 What is recorded

- Platform lifecycle receipts reach the computer's chain as
  `lifecycle_observed` with action `start`, `stop` or `restart`
  (`internal/agentcore/lifecycle_receipts.go:20-60`). An unknown action
  makes the guest return an error and stop processing the rest of the
  pending receipts (`:28-33`). They name no build.
- The deploy receipt (refreshed, current, deferred, skipped) lives on the
  host (`ci.yml:1526-1545`).
- The guest's own boot verdict is on `/api/runtime/obligations`
  (`restart.kind`); vmctl logs only that the guest accepted the marker.

### 1.6 Evidence from the last 36 hours

- 05:48 UTC 10-14: the paper's first refresh after two busy deferrals was
  an epoch-cutover boot (epoch 1 → 2, C1e B1); the guest read it as a
  crash and closed all 14 schedules; the paper was silent about an hour
  (`docs/problems/planned-refresh-closes-paper-schedules-2026-10-14.md`).
  Refreshes at 06:03 and 06:36 were planned: the paper did have quiet
  moments that night (R23).
- After intake.labels went on (06:50) and ce59d406 deployed (07:46), the
  paper was not refreshed by 08:28 (same doc). *Hypothesis:* it has had no
  quiet moment at deploy times since; the quiet-gap sample tests this.
- About 10:20 UTC 10-14: the paper took e3437f1c only through an
  owner-approved manual refresh, a crash boot that closed its in-flight
  runs; the owner's computer took it the same way, a crash boot by D7
  (`docs/evidence/owner-approved-refreshes-2026-10-14.json`).
- The owner's computer ran 761d94c5 for more than a day, deferred on every
  deploy (ledger metamission `blocker_or_risk`).

## 2. Failure modes

| # | Failure | Evidence | Slice |
| --- | --- | --- | --- |
| F1 | A busy computer misses every deploy | §1.6; `deploy_refresh.go:282-283` | LU1a, LU1b |
| F2 | Deferred computers wait for the next deploy; docs pushes never deploy | `ci.yml:1466-1469,1528` | LU1a |
| F3 | An epoch change turns a planned refresh into a crash | `planned_restart.go:133-136`; 05:48 console | LU2 |
| F4 | Race: work admitted between the marker's 200 and the kill | `planned_restart.go:395-415`; `deploy_refresh.go:276-296` | LU1b (final call) |
| F5 | The planned kill has no filesystem sync; timers and credential renewal can write after any sync | `manager.go:1447`; hibernate syncs (`hibernate.go:317`) | LU1b (runtime exits before the kill) |
| F6 | Host and guest can disagree about the boot kind | problem doc residual | LU1a (host reads the guest's verdict) |
| F7 | No health gate and no rollback after an image refresh; CI fails the deploy after the computer is already on the bad build | `ci.yml:1501,1515-1517` | LU3a, LU3b |
| F8 | In-guest applies self-mint their marker | `planned_restart.go:291-311`; L1 D1 | L1c (used by LU5) |
| F9 | A marker outlives a refresh that never happened: a crash within 15 min reads as planned | `planned_restart.go:26`; cleared only on refresh error, `deploy_refresh.go:296-300` | LU1b (final call clears on abort) |
| F10 | A deploy invalidates every snapshot, so every hibernated computer's next wake is a crash boot | S3 design, failure 7 and "Restart rule" | LU2 (migrations on any boot), L1c |
| F11 | Rolling back across a store-schema or work-state epoch runs an old binary on newer state | `updater.go:70-79`; D7 | U7 |
| F12 | One deploy exposes every idle computer, the owner's included, before any computer has proven the build | `ci.yml:1427-1517`, no ring gate | LU3a |
| F13 | Updates are not on the computer's event chain | `lifecycle_receipts.go:28-30`; host-only deploy receipt | LU3a, LU5 |
| F14 | A runtime swap that skips the base rebuild re-creates version skew; offers bound at deploy time go stale | a47122de; S2-g gaps | LU4 obligations |
| F15 | A drain that blocks held occurrences fills the 8 dispatcher slots, so the continuation an executing run needs never runs and every drain times out | `dispatcher.go:159-161`; `hibernate.go:129-160` (R1) | LU1b (defer, not block) |
| F16 | A drain delays the owner's own interaction | by construction | LU1b, O3 |
| F17 | vmctl restarts on every host deploy; a long synchronous call dies with it | host deploy restarts vmctl | LU1b (guest-timed, polled) |
| F18 | The target moves during a drain | deploys about hourly | LU1b (target read at the final call) |
| F19 | A rollback that fails, or a build that fails everywhere, loops | L1 design §1 | LU3b (one rollback; L1b grants) |
| F20 | A ring stalls behind a computer that never drains, or advances with no validated member | design R16 | LU3a |
| F21 | A planned restart with executing activations strands them | `runtime.go:768-770`; delegation design §8 | LU1b (drain waits for them) |
| F22 | Held work exceeds the deferral budget and is poisoned | `adapter.go:137`; `dispatcher.go:18-27` (R2) | LU1b (D bounded by test) |
| F23 | Hibernate's lift or TTL reopens a drain | `hibernate.go:93-104,132-137` (R4) | LU1b (gate with reasons) |
| F24 | The fence TTL expires between "drained" and the kill | `hibernate.go:39`; sweep 2 min (R5) | LU1b (final call) |
| F25 | Two update drivers on one host | `ci.yml:1427`; `ownership.go:957-1004` (R6) | LU1a |
| F26 | The paper runs a guest without the drain endpoint | e3437f1c (R7) | LU1a bootstrap |
| F27 | The idle or pressure sweep hibernates a pending or draining computer | `ownership.go:982-1000` (R10) | LU1a, LU1b |
| F28 | A launch path that skips the booted-image record leaves a computer pending forever, refreshed every pass | R29 | LU1a (record at `launchFirecracker`) |
| F29 | An `update` lifecycle receipt stops older guests' receipt processing | `lifecycle_receipts.go:28-33` (R17) | LU3a (skip unknown first) |
| F30 | Retained or target images collected by Nix GC mid-drain | `node-b.nix:955-1000` (R21) | LU1a (GC roots) |
| F31 | A guest runtime restart mid-drain forgets the drain | in-memory gate (R28) | LU1b (unknown drain = aborted) |
| F32 | An executing activation outlives D (budget 60 min) | `config.go:24` (R24) | LU1b (D calibrated by the sample) |
| F33 | vmctl dies between the final call's 200 and the kill: runtime down, marker written, and an unreachable guest reads as busy forever | `ownership.go:3100-3106`; `deploy_refresh.go:292-293` (S1) | LU1b (final-call record, sweep completes) |
| F34 | The updater's restart bridge starts the runtime again after it exited | `autoputer-vm.nix:483-493` (S2) | LU1b (final call refuses with an updater operation or request pending) |
| F35 | `Shutdown` waits for the handler that answers the final call; exiting before the flush looks like a failed call | `server.go:274-286` (S3) | LU1b (barrier, async shutdown) |
| F36 | A self-development retry starts an apply mid-drain (it is not a restart work kind) | `handler.go:244-251`; `planned_restart.go:161-173` (S7) | LU1b (admit only for an operation already mid-transaction) |
| F37 | LU2's epoch stamped before migrating, or old-schema actor-log occurrences delivered after a planned cross-epoch boot | `store.go:678-682`; `adapter.go:100-115` (S10, S12) | LU2 |
| F38 | A downgrade or a revert after a migration runs an old binary on newer state | `planned_restart.go:133-141` (S13) | LU2 (hold) |
| F39 | The target moves to an image that has not passed earlier rings | design (S15) | LU3a (pin the admitted image) |

## 3. Invariants

- **U1 (pull).** A deploy changes only the target. vmctl's sweep is the
  only update driver: one computer at a time per host, under a
  per-computer claim. CI sets the target, kicks one sweep, and reports.
- **U2 (no work cut).** The host kills a guest for an update only after
  the guest's final call succeeded: new work held, nothing executing,
  no desk pending mutation, no self-development transaction or updater
  operation, the runtime's dispatcher and stores closed, the marker
  durable, a system-wide sync after it, the runtime exited. Writes by
  other guest units after that sync are journal-recovered at boot (a
  named residual, S4). Busy is never forced by an automatic path
  (CLAUDE.md "Deploys never restart a busy computer").
- **U3 (planned stays planned).** An update boot is planned unless a real
  crash happened. An epoch change is a migration, not a crash.
- **U4 (bounded).** Per computer and target image: at most one apply, one
  automatic rollback, and K drain attempts before a visible "update stuck"
  signal. A refused image is never re-applied automatically; un-refusing
  is an owner act; a re-cut fix is a new image digest.
- **U5 (recorded).** Every update attempt, outcome and rollback is a record
  on the computer's event chain, joined to the image identities.
- **U6 (one authority).** Pending is derived (booted image ≠ target image),
  never stored. The canonical image identity is the resolved Nix store
  path of `/var/lib/go-choir/guest` (S17); it wins over CI's served-commit
  reading, and refusals and ring state are keyed by it.
  Attempt history lives in one host ledger (L1b's recovery ledger;
  interim fields on the ownership until it lands, as L1b-0 did).
- **U7 (forward-only across state).** No automatic rollback across a
  work-state epoch or a store-schema version (R14, R33). An update that
  crossed one fails forward: an L1 hold and a fix in a new build.

## 4. Slices

### LU1a: pending and retry (host only)

- **Booted image identity (F28, S17).** Both spawn paths, `bootVM` and
  `ResumeVM`, resolve the image (the store path behind
  `/var/lib/go-choir/guest`, and its `build.json` commit) and pass it with
  the computer id into `launchFirecracker` (`manager.go:866,1093,1888`),
  which returns it for vmctl to write on the ownership; a launch that
  cannot resolve it fails. Store-schema version and work-state epoch join
  the identity once the guest image's `build.json` carries them (produced
  by the guest image build in `flake.nix`; consumed by LU3b's U7 check).
  Pending = active computer whose booted image differs from the target.
  `vmctl list` shows `update: {target, booted, pending_since,
  target_first_seen, last_attempt, outcome, guest_boot_kind}`;
  `pending_since` and `target_first_seen` are persisted on the ownership.
- **Target.** The target is the resolved store path of
  `/var/lib/go-choir/guest`, read at each pass; nothing else stores it.
- **One driver (F25).** The idle sweeper (`ownership.go:957-1004`, every
  2 min on staging, `node-b.nix:772`) gains an update pass, run before the
  idle and pressure passes. For each pending computer (not held, not
  divergent; platform tier first) it first applies vmctl's own idle test
  (`guestBusy`, which counts desk pending mutations,
  `ownership.go:3108-3125`; S16), then calls the existing `deployRefresh`
  (`deploy_refresh.go:254`) under a per-computer claim, at most one
  computer per host at a time. The refresh and its wait run under the
  claim, off `sweepMu` (S20). CI stops calling deploy-refresh per
  computer: it calls `POST /internal/vmctl/update-sweep` once and reports
  the update state. An owner's forced refresh goes through vmctl; it may
  interrupt a drain (a crash boot, the owner's act) but never the window
  between a final call's 200 and the kill (S18).
- **Bootstrap (F26).** LU1a works with today's guests: the existing
  planned-restart endpoint answers 200 at a quiet moment, and the sweep
  now asks every 2 minutes instead of once per deploy. That is how
  drain-capable code reaches the paper. If the paper has no quiet moment
  before LU1b ships, one owner-approved refresh installs it, recorded as
  the bootstrap.
- **No hibernating a pending computer (F27).** An idle pending computer is
  updated instead of hibernated.
- **Retained images (F30).** vmctl holds a GC root on every image named by
  a computer's booted, last-good or target record; above 3 distinct images,
  or below a disk floor on the store volume, it alerts rather than
  collects (S30).
- **Guest verdict (F6).** After each update boot the sweep reads the
  guest's `restart.kind` from `/api/runtime/obligations` and records it.
- **Flag.** `VM_UPDATE_SWEEP=off|ring0|all`. Lands as `ring0`; `all` after
  LU1a's acceptance on ring 0.

Ways it could fail: a computer whose launch is not recorded is refreshed
every pass (covered: a launch without an identity fails); a refresh
stalls other sweep passes (covered: it runs off `sweepMu`); today's
endpoint misses desk mutations (covered: vmctl's idle test runs first);
the F4 race between the endpoint's 200 and the kill remains until LU1b,
as it does today.

Rollback: revert; `VM_UPDATE_SWEEP=off`; CI's per-computer loop is a
revert away.

### LU2: persisted epoch and migrations (guest)

- **Persisted, never stamped at open (S10).** The store keeps the
  work-state epoch beside the store-schema epoch, but unlike
  `storeschema.Write` (`store.go:678-682`, written at every open) the
  work-state epoch is only **read** at open. It advances inside each
  migration step's transaction, after the step's work. Steps are
  idempotent, so a crash between a step and its epoch write re-runs the
  step.
- **First boot (S11).** With no persisted epoch, the boot seeds it from the
  consumed marker's epoch if there is one, otherwise from the binary's.
  The release that ships LU2 does not itself bump the epoch.
- A registry: for each step `n → n+1`, a migration function or an
  explicit `crash` entry. A unit test fails when `CurrentWorkStateEpoch`
  changes without an entry, so every bump forces the author to choose.
- On **every** boot where the persisted epoch is lower than the binary's,
  the boot runs each step before any closure, before the dispatcher and
  before the actor runtime start, and records
  `work_state_migrated {from, to, closed: [...]}`. A step covers both
  databases (S12): first it closes or rewrites, idempotently, the
  actor-log occurrences (SQLite, `internal/actorruntime/adapter.go:100-115`)
  of kinds whose meaning changed, then it migrates the Dolt store and
  advances the epoch in that transaction. A migration closes only the
  state whose schema changed (B1's would have closed engineering
  assignments opened under the packet schema), with the fate "closed by a
  work-state migration". The boot kind stays what the marker (later the
  host) says: a planned boot stays planned, and a markerless boot is
  still a crash boot, now on migrated state (F10).
- The marker's own epoch check (`planned_restart.go:133-136`) is replaced
  by the persisted check; the marker carries planned-ness only.
- **Fail closed (S13, S14).** A `crash` entry or a missing entry: crash
  boot, as today. A persisted epoch **higher** than the binary's (a
  downgrade), or a failed migration: the runtime holds (no admission, no
  closure run on state it cannot read, visible on `/health`) for an owner
  act. A binary from before LU2 cannot read the persisted epoch, so
  reverting LU2 or a later release after a migration has run is
  forbidden; that is the slice rollback rule.
- The guest change that skips unknown lifecycle actions (LU3a, F29)
  ships in the same image (S27).
- Timing: the next epoch bump is expected in C1e B2-B5 (packets and work
  items deleted). LU2 lands before it; that bump does not land until it
  does. LU2 is built in parallel with LU1a but reaches computers through
  LU1a's sweep (S28).

Rollback: revert only before any migration has run; after that, fix
forward. Epoch steps without migrations fall back to D7.

### LU1b: the drain (guest and host)

- **One gate with reasons (F23, S6).** The hibernate fence becomes an
  admission gate with a set of reasons, each with its own deadline:
  `hibernate`, `update_drain`, `update_final`, `probation` (LU3b), later
  `selfdev_apply`. The gate is lifted only when its set is empty. Only
  `hibernate` keeps "set only while nothing is inside"; `update_drain`
  sets while work is executing (that is its purpose); `update_final` sets
  only after the drained re-check. Hibernate prepare refuses while a
  drain is active.
- **Defer, not block (F15, F22, S8, S9).** A held occurrence returns
  `actor.ErrDeferUnprocessed` before `EnterWork`, as the self-development
  apply hold does (`handler.go:165-168`), so it uses no dispatcher slot
  and stays pending. Update holds defer in their own class: the
  dispatcher does not count them toward `MaxDeferrals` (64,
  `adapter.go:137`), and their live-lock bound is the hold's own deadline,
  because `defer_count` is durable and would otherwise stack across the
  drain, the final window, probation and the apply hold
  (`log_sqlite.go:143-172`). An update-hold deferral `continue`s the
  mailbox fold instead of `break`ing it (`dispatcher.go:394-433`), so a
  continuation queued behind held work in the same mailbox is not
  delayed. A unit test sums the worst-case stack of the counted holds
  against the budget.
- **Admission predicate (S7).** In order: set `update_drain`; then
  snapshot the run ids of executing activations (the runtime's in-memory
  set, `runtime.go:2283`) and their ancestors, found by a bounded walk of
  each run's parent (`RequestedByRunID`). Never scan all runs
  (`ListAllRunsByState` can exhaust the guest, `runtime.go:2515-2520`).
  For each occurrence of a restart work kind
  (`planned_restart.go:161-173`), resolve its run: the run id in
  `Content` for `initial_dispatch`, the resume state for
  `coagent_result`, `TrajectoryID` only as a hint checked against the
  store (a child's trajectory is its parent's run id,
  `runtime.go:4346-4363`). Admit it when that run or a bounded ancestor is
  in the set; an admitted spawn joins the set. Admit cancels and deadline
  closures. Admit `selfdev_materialization_retry` only for an operation
  already mid-transaction when the drain began; defer it otherwise (it is
  not a restart work kind and can start an apply, `handler.go:244-251`;
  F36). Any resolution error defers (fail closed: a wrong deferral costs
  a timeout, never a cut). Owner messages are deferred unless O3 cancels
  the drain.
- **Start and poll.** After the quiet wait Q (default 30 min after
  `target_first_seen` with no quiet moment), the sweep calls
  `POST /internal/runtime/update-drain {deadline}`, which answers 202. A
  repeated request (for example after a vmctl restart) joins the active
  drain and never extends its deadline (S26). Each pass polls `GET` for
  `draining`, `drained`, `timed_out` or `cancelled` (with the executing
  runs, their roots and ages). The drain is guest-timed; a guest runtime
  restart loses it, and the host reads an unknown drain as aborted (F31).
- **Drained.** The guest reports `drained` when nothing in the snapshot
  set is executing, no desk mutation is pending, no self-development
  transaction is open and no updater operation is in flight. It keeps
  deferring new work. No marker yet.
- **Final call (F4, F5, F9, F24, F33-F35; S1-S5).**
  1. vmctl writes a durable final-call record on the ownership
     (`{target, at}`) before calling.
  2. It calls the planned-restart endpoint with the image the computer
     may take: the current target before rings exist, the image admitted
     to its ring after LU3a (F39). The call runs under the per-computer
     claim, off `sweepMu`.
  3. The guest sets `update_final` (60 s deadline), re-checks drained,
     and refuses if an updater operation is in flight or a restart request
     file exists (`autoputer-vm.nix:483-493`). Then the barrier: stop the
     dispatcher and the actor adapter, close the stores, write the marker,
     `sync`, send and flush the 200, shut the server down asynchronously
     (`Shutdown` waits for active handlers, `server.go:274-286`), exit 0.
     The unit is `Restart=on-failure`, so a clean exit stays down
     (`autoputer-vm.nix:842-843`).
  4. vmctl waits up to 30 s for the runtime to stop answering, then kills
     and boots, and clears the record after the boot.
  5. On any error before the 200, the guest clears `update_final` and the
     marker and returns to `update_drain`; vmctl clears the record and
     does not kill. The 60 s deadline clears `update_final` if vmctl never
     answers.
  6. **Recovery.** If vmctl restarts between the 200 and the kill, the
     next pass finds the record: if the runtime has not answered for 60 s
     and the record is younger than the marker's 15 minutes, it kills and
     boots (a planned boot, since the marker is written); otherwise it
     clears the record. Without this an unreachable guest reads as busy
     forever (`ownership.go:3100-3106`).
  7. Once `update_final` is set, owner messages wait for the planned boot
     (S25).
- **Deadline.** At D (default 15 min; calibrated by the quiet-gap sample,
  since one activation may run up to 60 min) the guest clears
  `update_drain` and reports `timed_out`. The host backs off (default 30
  min) and retries; after K = 3 timeouts for one target it raises "update
  stuck" on the owner's page and alerts. Never forced.
- **Owner activity (O3, F16).** On interactive tiers, an owner message
  arriving at the API ingress cancels the drain before it reaches the
  actor path (R27) and the poll reads `cancelled`; cancels are counted
  apart from timeouts. On the platform tier it is deferred.
- **No hibernating a draining computer (F27).** `/health` reports the
  drain, and the idle and pressure passes skip a draining computer.
- **After L1c (R35).** The host's attestation replaces the marker in the
  final call; the gate, the drain and the final call stay.

Rollback: revert; the drain endpoint is additive and unused when the sweep
flag is `ring0` without drains.

### LU3a: rings and refusal (host)

- **Rings.** Ring 0: test-tier computers (`WarmnessClassTest`,
  `internal/vmctl/warmness_policy.go:16-19`) and a minted canary when none
  is active. Ring 1: platform computers (the paper). Ring 2: paid accounts
  (the owner's computer). Ring 3: everyone else.
- **Gate (F12, F20).** Ring k+1 opens when at least one ring-k computer
  reached the target, booted planned and passed the health wait CI uses
  today, then stayed up through a soak S (default 10 min), and no ring-k
  computer failed. A ring timeout (default 2 h) without a validated member
  halts and alerts. A ring-k member that updates late takes the build
  under the same health wait; it no longer gates. "Update stuck" (drain
  timeouts) is not a failure and does not halt anything (S19).
- **Pinned image (F39, S15).** Rings and attempts are keyed by image
  identity. A computer takes only an image admitted to its ring; when the
  target moves, a drain or final call for the old image aborts and the
  computer re-gates on the new one.
- **Currency.** "Already current" is read from the booted-image record,
  never from `/health` `build.commit` (a layered computer reports its
  layer's commit).
- **Failure.** A failed update halts the rollout of that image
  fleet-wide, refuses it for that computer, and alerts the owner. Un-halt
  and un-refuse are owner acts.
- **Record (U5, F29, S24).** The guest change that acknowledges and skips
  unknown lifecycle actions ships first, with LU2. Then vmctl issues a
  platform-signed lifecycle receipt with action `update` and
  `{from_image, to_image, outcome, reason}` for every applied, refused or
  rolled-back update, only to images that declare the action; the guest
  appends it as `lifecycle_observed`. A refusal is recorded on the
  computer's own chain, not only as an alert.

Rollback: ring gating off returns to LU1 behavior.

### LU3b: probation and rollback (needs L1c and L1b)

- **Probation hold.** An update boot starts with the `probation` reason set
  (the host passes it with its planned-boot attestation, L1c). Held work
  is deferred, not blocked (R18), so it stays pending. The host probes:
  `/health` 200 within T_ready (default 5 min); served build equals the
  target; boot kind planned; no unit latched by its start limit; active
  schedule count not lower than before. Pass: the host clears the hold
  and records `update_applied`.
- **Rollback.** On a failed probe, vmctl boots the computer on its
  last-good image (vmmanager accepts per-boot kernel, initrd, rootfs and
  store-disk overrides, `manager.go:1462-1500`) as a host-attested planned
  boot under an L1b grant, records `update_rolled_back`, and refuses the
  image for that computer. Only when the previous image's work-state epoch
  and store-schema version equal the new image's (U7). One rollback per
  computer and image; a failed rollback becomes an L1 hold (F19). Held
  wakes are delivered after the rollback boot.

Rollback: `VM_UPDATE_HEALTH_GATE=off` returns to LU3a behavior.

### LU5: one apply path shared with self-development

- One `ReleaseApply` transaction for every change to what a computer
  runs: `kind` (platform_image, selfdev_bundle, restore; platform_runtime
  if LU4 is built), from and to identities, work-state epoch and
  store-schema version, admission (quiet then drain), restart scope,
  health contract, rollback target, and its records on the chain.
- Self-development runs Freeze → Verify → owner approval → materializer →
  `updater.Apply` → health → `materialization_applied` → checkpoint →
  route, with rollback through `rollback_requested/applied` and restore to
  a prior head (`docs/computer-ontology.md`, "Computer-local
  self-development"). A platform update enters the same path at the
  decision step: a platform-control-signed offer under the follow policy
  (M9a) instead of the owner's approval. After the decision everything is
  shared: the gate (the materializer's hold becomes the `selfdev_apply`
  reason), probation, rollback, the updater's journal, one record shape.
- Chain restore and image rollback stay different mechanisms that share a
  record shape. A restore proof joins event head, content witness,
  VM-local projection and route, within the ontology's restore-set
  boundary (Store B out) (R20).
- Divergent computers stay out: a platform update to a self-developed
  computer needs typed rebase (`docs/computer-ontology.md`, "Platform
  Updates To Divergent Computers"); it stays pending and visible.
- Identity: a computer's effective build can differ from the deploy SHA on
  purpose (pending, rolled back, restored). `vmctl list` and the owner's
  page show target, effective release, event head and route, as the
  Landing Loop's "Product restore vs platform deploy" requires.
- Depends on C1e's Freeze and Verify working on staging and on the
  self-development effects gate (`docs/computer-ontology.md:75-78`).

### LU4: binary-only swap (conditional)

Built only if LU1's measured planned-refresh downtime (about 38 s of VM
boot on the paper at 05:48, plus readiness) is shown to matter, and then
as the process-restart scope of ReleaseApply, never as a CI push (R19).
Obligations before it starts:

- a builder that produces a layer against each distinct booted base in
  use, or the rule that only computers on the previous deploy's base get
  a layer (`updater.go:276-282` refuses any other base);
- pending for a layered computer = target runtime commit ≠ served commit,
  or booted base ≠ target base, so a layer neither hides nor repeats an
  update;
- offers minted when the guest takes them, for its current realization
  and head (the S2-g bind gaps);
- the base still rebuilt on every runtime deploy (a47122de stands); the
  frontend stays out of the layer.

Rollback: flag off; a layer is dropped at the next VM boot by the base
binding (`autoputer-vm.nix:133-142`).

## 5. Relation to other stations

- **L1.** Update boots are external events in the restart design's sense
  (one episode per computer and target). LU3b needs L1c (host-asserted
  planned boots, for the probation flag and the rollback boot) and L1b
  (grants, and the ledger as LU's attempt history). LU1a, LU2, LU1b and
  LU3a need neither, so the station as a whole does not depend on L1
  (S21).
- **S3 snapshots.** A deploy invalidates every snapshot; the next wake is
  a cold boot without a marker, a crash boot (F10). With LU2 the boot at
  least runs on migrated state. Once L1c exists, the host can attest that
  cold boot as planned, because the snapshot fence guaranteed nothing was
  in flight; this follows from the restart design and needs no separate
  ruling (R33).
- **Management delegation.** Bounded tasks with a 10-minute deadline and
  the S1 host cell deadline are what make a drain finish on the paper; its
  O6 ("who requests the drain") is answered here: vmctl's update sweep,
  after the quiet wait.
- **C1e.** One shared-runtime cutover at a time: LU2 and LU1b are
  shared-runtime changes and land between C1e slices. LU2 lands before
  C1e's next epoch bump.

## 6. Open decisions for the owner

| # | Decision | Default in this note |
| --- | --- | --- |
| O1 | At the drain deadline: defer, or force after K failed drains | Defer; "update stuck" alert after K = 3; a forced refresh only by an owner act |
| O2 | Ring order, and whether the owner's computer may opt out of automatic updates | test → paper → owner → others; a failure halts the image fleet-wide and alerts the owner; no opt-out (the owner's computer follows ring 2). Soak and timeouts are operational |
| O3 | Does the owner's activity cancel a drain on their own computer, and how long may new work wait there (D) | Cancel on interactive tiers; D = 15 min; on the platform tier, wait |
| O3b | During probation on the owner's computer (up to T_ready = 5 min): hold the owner's messages visibly, or admit them and give up automatic rollback for that update | Hold visibly ("updating") |
| O5 | Build LU4 (layered runtime swap) | Only if measured planned-refresh downtime matters; the owner asked for lighter restarts, so cutting it is the owner's call |
| O7 | Update-lag target for the paper | Within 60 min of a deploy when its work drains; otherwise visibly pending. The lag is reported for the owner's computer too |
| O8 | CLAUDE.md wording ("Deploys never restart a busy computer") | Deferred; LU1 satisfies the current sentence literally. Later: "a deploy marks an update pending; each computer takes it at a quiet moment or after a bounded drain; busy is never forced" |
| O9 | Who may un-halt a rollout or un-refuse an image | The owner, by an explicit act |

Moved out of this list in round 2: O4 (forward-only across state) is now
invariant U7; O6 (hibernated computers) follows from the restart design;
the quiet wait Q is operational.

## 7. What this does not do

- It does not force a busy computer. A computer whose work never finishes
  inside the drain deadline stays on its build, visibly.
- It does not rebase platform updates onto self-developed computers.
- It does not replace L1's crash bounds; it reduces how often an update
  turns into a crash.
- It does not make deploys of host services zero-downtime (L1 D3, D4).
- Residuals: interim attempt fields on the ownership lack fsync until
  L1b's ledger (R31); a real crash in the seconds between the final
  call's 200 and the kill reads as planned until L1c; writes by other
  guest units after the final sync are journal-recovered (S4); a
  root-owned updater restart request created in the last seconds of the
  final call could still start the runtime (S2).
