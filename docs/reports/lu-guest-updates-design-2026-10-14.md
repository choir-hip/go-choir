# LU: Guest Updates (design note, round 2)

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
- **Round 2** (this file): a convergent panel (claude, gemini, glm, muse,
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

## 3. Invariants

- **U1 (pull).** A deploy changes only the target. vmctl's sweep is the
  only update driver: one computer at a time per host, under a
  per-computer claim. CI sets the target, kicks one sweep, and reports.
- **U2 (no work cut).** The host kills a guest for an update only after
  the guest's final call succeeded: new work held, nothing executing,
  no desk pending mutation, no self-development transaction, filesystems
  synced, marker written, runtime exited. Busy is never forced by an
  automatic path (CLAUDE.md "Deploys never restart a busy computer").
- **U3 (planned stays planned).** An update boot is planned unless a real
  crash happened. An epoch change is a migration, not a crash.
- **U4 (bounded).** Per computer and target image: at most one apply, one
  automatic rollback, and K drain attempts before a visible "update stuck"
  signal. A refused image is never re-applied automatically; un-refusing
  is an owner act; a re-cut fix is a new image digest.
- **U5 (recorded).** Every update attempt, outcome and rollback is a record
  on the computer's event chain, joined to the image identities.
- **U6 (one authority).** Pending is derived (booted image ≠ target image),
  never stored. The image identity wins over CI's served-commit reading.
  Attempt history lives in one host ledger (L1b's recovery ledger;
  interim fields on the ownership until it lands, as L1b-0 did).
- **U7 (forward-only across state).** No automatic rollback across a
  work-state epoch or a store-schema version (R14, R33). An update that
  crossed one fails forward: an L1 hold and a fix in a new build.

## 4. Slices

### LU1a: pending and retry (host only)

- **Booted image identity.** vmmanager records, at `launchFirecracker`
  (the single spawn actuator, restart design §9), the resolved store path
  of `/var/lib/go-choir/guest` and its `build.json` commit, store-schema
  version and work-state epoch. Every launch path passes through it, so
  none can skip it (F28). Pending = active computer whose booted image
  differs from the target. `vmctl list` shows `update: {target, booted,
  pending_since, last_attempt, outcome, guest_boot_kind}`.
- **One driver (F25).** The idle sweeper (`ownership.go:957-1004`, every
  2 min on staging, `node-b.nix:772`) gains an update pass, run before the
  idle and pressure passes. For each pending computer (not held, not
  divergent; platform tier first), it calls the existing
  `deployRefresh` (`deploy_refresh.go:254`) under a per-computer claim,
  at most one computer per host at a time. CI stops calling deploy-refresh
  per computer: it kicks one sweep and reports the update state. The
  forced dispatch input calls vmctl, which skips any computer with an
  active drain.
- **Bootstrap (F26).** LU1a works with today's guests: the existing
  planned-restart endpoint answers 200 at a quiet moment, and the sweep
  now asks every 2 minutes instead of once per deploy. That is how
  drain-capable code reaches the paper. If the paper has no quiet moment
  before LU1b ships, one owner-approved refresh installs it, recorded as
  the bootstrap.
- **No hibernating a pending computer (F27).** An idle pending computer is
  updated instead of hibernated.
- **Retained images (F30).** vmctl holds a GC root on every image named by
  a computer's booted, last-good or target record; above 3 distinct images
  it alerts rather than collects.
- **Guest verdict (F6).** After each update boot the sweep reads the
  guest's `restart.kind` from `/api/runtime/obligations` and records it.
- **Flag.** `VM_UPDATE_SWEEP=off|ring0|all`. Lands as `ring0`; `all` after
  LU1a's acceptance on ring 0.

Ways it could fail: a computer whose launch is not recorded is refreshed
every pass (covered by recording at the actuator); a sweep pass that
refreshes holds `sweepMu` for about a minute and delays warm passes (one
refresh per pass, logged); the guest's restart test misses desk
mutations (LU1b fixes the test; LU1a uses today's endpoint unchanged).

Rollback: revert; `VM_UPDATE_SWEEP=off`; CI's per-computer loop is a
revert away.

### LU2: persisted epoch and migrations (guest)

- The store persists the work-state epoch at open, beside the store-schema
  epoch (`storeschema.Write`, `store.go:678-682`) (R12).
- A registry: for each step `n → n+1`, a migration function or an
  explicit `crash` entry. A unit test fails when `CurrentWorkStateEpoch`
  changes without an entry, so every bump forces the author to choose.
- On **every** boot where the persisted epoch is lower than the binary's,
  the boot runs each step's migration before any closure and before the
  actor runtime starts, one store transaction per step where the store
  allows, and records `work_state_migrated {from, to, closed: [...]}`. A
  migration closes only the state whose schema changed (B1's would have
  closed engineering assignments opened under the packet schema), with
  the fate "closed by a work-state migration". The boot kind stays what
  the marker (later the host) says: a planned boot stays planned, and a
  markerless boot is still a crash boot, now on migrated state (F10).
- The marker's own epoch check (`planned_restart.go:133-136`) is replaced
  by the persisted check; the marker carries planned-ness only.
- A `crash` entry, a missing entry or a downgrade (persisted epoch higher
  than the binary's): the boot is a crash boot, as today. A failed
  migration holds the runtime (no admission, visible on `/health`), never
  runs on unmigrated state.
- Timing: the next epoch bump is expected in C1e B2-B5 (packets and work
  items deleted). LU2 lands before it; that bump does not land until it
  does.

Rollback: revert; epoch steps without migrations fall back to D7.

### LU1b: the drain (guest and host)

- **One gate with reasons (F23).** The hibernate fence becomes an
  admission gate with a set of reasons, each with its own deadline:
  `hibernate`, `update_drain`, `update_final`, later `selfdev_apply`. The
  gate is lifted only when its set is empty. Hibernate prepare refuses
  while a drain is active.
- **Defer, not block (F15, F22).** While `update_drain` is set, a held
  occurrence returns `actor.ErrDeferUnprocessed` before `EnterWork`, as
  the self-development apply hold does. It stays pending on the tape,
  uses no dispatcher slot, and is delivered after the planned boot. D
  plus the final-call window stays under the deferral budget (about 29.5
  min); a unit test fails otherwise.
- **Admission by lineage (R3).** At drain start the guest snapshots the
  root runs and trajectories with an executing activation or a parked run
  in flight. An occurrence whose root is in that set is admitted
  (child spawns and child results included), plus cancels and deadline
  closures. Every other occurrence of a `RestartWorkKinds` kind
  (`planned_restart.go:161-173`) is deferred. Owner messages are deferred
  too, unless O3 cancels the drain.
- **Start and poll.** After the quiet wait Q (default 30 min pending with
  no quiet moment), the sweep calls `POST /internal/runtime/update-drain
  {deadline}`, which answers 202. Each pass polls `GET` for `draining`,
  `drained` or `timed_out` (with the executing runs, their roots and
  ages). The drain is guest-timed: a vmctl restart loses nothing (F17); a
  guest runtime restart loses the drain, and the host reads an unknown
  drain as aborted (F31).
- **Drained.** The guest reports `drained` when nothing in the snapshot
  set is executing, no desk mutation is pending and no self-development
  transaction is open. It keeps deferring new work. No marker yet.
- **Final call (F4, F5, F9, F24).** On `drained`, the same vmctl goroutine
  calls the planned-restart endpoint with the current target (read now,
  F18). The guest sets `update_final` (which also stops admitting
  lineage occurrences), re-checks, syncs, writes the marker, answers 200,
  and the runtime exits cleanly (exit 0; the unit does not restart on a
  clean exit). vmctl waits up to 30 s for the runtime to stop answering,
  then kills and boots. If the call fails or the runtime does not stop,
  vmctl clears the marker and does not kill.
- **Deadline.** At D (default 15 min; calibrated by the quiet-gap sample,
  since one activation may run up to 60 min) the guest clears
  `update_drain` and reports `timed_out`. The host backs off (default 30
  min) and retries; after K = 3 timeouts for one target it raises "update
  stuck" on the owner's page and alerts. Never forced.
- **Owner activity (O3, F16).** On interactive tiers, an owner message
  arriving at the API ingress cancels the drain before it reaches the
  actor path (R27). On the platform tier it is deferred.
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
  under the same health wait; it no longer gates.
- **Failure.** A failed update halts the rollout of that image
  fleet-wide, refuses it for that computer, and alerts the owner. Un-halt
  and un-refuse are owner acts.
- **Record (U5, F29).** First a guest change: unknown lifecycle actions
  are acknowledged and skipped. Then vmctl issues a platform-signed
  lifecycle receipt with action `update` and `{from_image, to_image,
  outcome, reason}`, only to images that declare the action; the guest
  appends it as `lifecycle_observed`.

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
  LU3a need neither.
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
| O2 | Ring order | test → paper → owner → others; a failure halts the image fleet-wide and alerts the owner. Soak and timeouts are operational |
| O3 | Does the owner's activity cancel a drain on their own computer, and how long may new work wait there (D) | Cancel on interactive tiers; D = 15 min; on the platform tier, wait |
| O3b | During probation on the owner's computer (up to T_ready = 5 min): hold the owner's messages visibly, or admit them and give up automatic rollback for that update | Hold visibly ("updating") |
| O5 | Build LU4 (layered runtime swap) | Only if measured planned-refresh downtime matters; the owner asked for lighter restarts, so cutting it is the owner's call |
| O7 | Update-lag target for the paper | Within 60 min of a deploy when its work drains; otherwise visibly pending |
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
  call's 200 and the kill reads as planned until L1c.
