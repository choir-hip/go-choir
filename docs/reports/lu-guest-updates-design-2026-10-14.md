# LU: Guest Updates (design note, round 1)

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

## 0. The short version

Today a deploy pushes: CI asks each running computer once, at deploy time,
whether it is idle; a busy computer is skipped until the next deploy. The
paper is never idle, so it never updates by itself. When it finally does,
a work-state epoch change can turn the planned restart into a crash. There
is no health gate after an image refresh and no automatic way back.

LU turns this into pull with a deadline:

1. **Pending, not pushed.** A deploy changes the target build. A computer
   whose booted build differs from the target is *pending*. vmctl's sweep
   offers the update to each pending computer at every pass, not only at
   deploy time.
2. **Quiet moment, then drain.** The guest takes the update when nothing
   is inside its admission fence. If no quiet moment comes within a wait
   (default 30 min), the host asks the guest to drain: stop admitting new
   work, let running work finish within a deadline (default 15 min), then
   restart. Schedules are not closed; their next tick runs after the boot.
   At the deadline the guest reopens and the update is deferred, never
   forced.
3. **Planned stays planned.** A work-state epoch change runs a registered
   migration at the planned boot instead of reading the boot as a crash.
4. **Health, rollback, rings.** The new build must pass a health contract
   under a probation fence. On failure the host boots the previous image
   (kept on disk) once. Rollout goes test computers, then the paper, then
   the owner's computer, then everyone else; a rollback stops the line.
5. **Lighter restarts.** When only the guest runtime changed, the guest
   updater swaps the runtime binary and restarts one unit, not the VM.
6. **One apply path.** Platform updates and approved self-development go
   through one release-apply transaction (admission, apply, health,
   rollback, record on the computer's event chain).

Slice order: LU1 pending + quiet/drain; LU2 migrations; LU3 health,
rollback and rings; LU4 binary swap; LU5 one apply path.

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
- The refresh itself is `RefreshVMForDesktop` → vmmanager
  `RefreshVMWithConfig`, which SIGKILLs Firecracker
  (`internal/vmmanager/manager.go:1447`) and boots the current image. There
  is no guest quiesce or filesystem sync before the kill.
- The manual dispatch input `refresh_active_computers=true` force-refreshes
  deferred computers through the operator refresh, a crash boot
  (`ci.yml:11`, `:1470-1490`).
- Nothing retries a deferred computer between deploys. Docs-only pushes do
  not deploy at all, so a busy computer can lag for many hours.
- The guest image is one immutable Nix store output; `/var/lib/go-choir/guest`
  is a symlink to it, swapped at host activation
  (`nix/node-b.nix:955-1000`), and vmctl boots from the stable paths
  (`nix/node-b.nix:725-729`). The previous image survives only as long as
  an older system generation roots it. vmctl records no per-computer
  "booted image" identity; CI infers it from the guest's `/health`
  `build.commit` (`ci.yml:1439`).

### 1.2 Boot kinds and the marker (guest)

- `POST /internal/runtime/planned-restart` (commit bcc556de) writes the
  marker when the guest is not busy and answers 200 once it is durable
  (`internal/agentcore/planned_restart.go:361-416`). Busy means executing
  runs or a self-development operation mid-transaction
  (`plannedRestartBusy`, `:321-336`). It does **not** close any admission
  gate, and it does not count source polls, which call models outside a
  run.
- The marker lives 15 minutes (`:26`), carries the binary's work-state
  epoch (`:64`), and is consumed once before the runtime starts (`:109-142`).
  A marker whose epoch differs from `CurrentWorkStateEpoch` (now 2, `:35`)
  makes the boot a crash (`:133-136`, decision D7).
- `Runtime.Start` branches on the boot kind (`internal/agentcore/runtime.go:768-774`):
  planned boots passivate activations; crash boots close pre-boot runs and
  open rounds. Source schedules survive both (ruling 6, ce59d406;
  `internal/agentcore/source_schedule.go:290-336`); a crash still closes
  pending arrivals and the poll in flight.
- In-guest applies (platform-update offers, self-development applies and
  rollbacks) still write their own marker (`markPlannedRestart`,
  `planned_restart.go:291-311`), bounded at 3 starts per apply (`:215`,
  L1a-2). L1c moves planned-ness to the host.

### 1.3 The in-guest apply path (guest updater)

The root-owned `choir-updater` (`cmd/choir-updater/main_linux.go`,
`internal/updater/updater.go`) already does most of what LU asks for, for
layered releases:

- `Apply` (`updater.go:192`) checks the base join and the store-schema
  window before any mutation (`:270-292`), stages and verifies the release,
  materializes its closure, writes a journal, swaps the `current` pointer
  (`:329-337`), publishes a restart of the runtime unit through a systemd
  path unit (`:338-364`; `nix/autoputer-vm.nix:474-510`), probes health
  (`:365-395`), signs a health receipt and a materialization receipt, and
  on probe failure restores the prior release and restarts it
  (`restorePrior`, `:943-994`) with a signed recovery receipt.
- The boot wrapper serves a layer only on the base it was built for
  (`nix/autoputer-vm.nix:133-142`) and falls back to the base runtime after
  3 failed starts in 60 s (`:143-180`).
- Two callers: the self-development materializer
  (`internal/agentcore/self_development_materializer.go:315-453`, recorded
  as `materialization_started/applied/failed` events, then checkpoint and
  route) and the M9a platform-update offer (`internal/agentcore/platform_update.go:75`,
  `POST /internal/runtime/platform-update`).
- **CI no longer drives the platform-update offer.** The app-layer push
  was deleted on purpose in a47122de after it caused version skew (new
  realizations booted older code) and landed on 0 of 2 computers
  (`docs/problems/guest-runtime-fix-never-reaches-layered-computers-2026-10-09.md`;
  bind gaps in `docs/problems/s2-app-layer-offer-bind-gaps-2026-10-05.md`:
  head not initialized, realization stale after a resume).

### 1.4 The one admission fence (guest)

S3 snapshot hibernation added the only work admission gate: the actor
handler enters every occurrence through `Runtime.EnterWork`
(`internal/actorruntime/handler.go:171`; `internal/agentcore/hibernate.go:163-172`).
The fence closes only while nothing is inside (`hibernate.go:107-119`),
holds new occurrences until lifted or until a 2-minute TTL (`:35-39`,
`:129-160`), and `prepareHibernate` then checks executing runs and
self-development operations and syncs filesystems (`:281-319`). The
planned-restart endpoint does not use it.

### 1.5 What is recorded

- Platform lifecycle receipts reach the computer's event chain as
  `lifecycle_observed` with action `start`, `stop` or `restart`
  (`internal/agentcore/lifecycle_receipts.go:20-60`). They name no build.
- The deploy receipt (refreshed, current, deferred, skipped) lives on the
  host (`ci.yml:1526-1545`), not on any computer's chain.
- vmctl logs `planned_restart=true` when the guest accepted the marker; it
  cannot see the guest's own boot verdict
  (`docs/problems/planned-refresh-closes-paper-schedules-2026-10-14.md`).

### 1.6 Evidence from the last 36 hours

- 05:48 UTC 10-14: the paper's first refresh after two busy deferrals was
  an epoch-cutover boot (epoch 1 → 2, C1e B1); the guest read it as a
  crash and closed all 14 schedules; the paper was silent about an hour
  (problem doc above).
- After ce59d406 (07:46) the paper was not refreshed: "it is never idle,
  and deploys never restart a busy computer" (same doc, last section).
- ~10:20 UTC 10-14: the paper took e3437f1c only through an owner-approved
  manual refresh, a crash boot that closed its in-flight runs; the owner's
  computer took it the same way, a crash boot by D7
  (`docs/evidence/owner-approved-refreshes-2026-10-14.json`).
- The owner's computer ran 761d94c5 for more than a day, deferred on every
  deploy (ledger metamission `blocker_or_risk`).

## 2. Failure modes

Each is evidenced or read in code unless marked *hypothesis*.

| # | Failure | Evidence | Slice |
| --- | --- | --- | --- |
| F1 | A never-idle computer never updates | §1.6; `deploy_refresh.go:282-283` | LU1 |
| F2 | Deferred computers wait for the next deploy; docs pushes never deploy | `ci.yml:1466-1469,1528` | LU1 |
| F3 | An epoch change turns a planned refresh into a crash | `planned_restart.go:133-136`; 05:48 console | LU2 |
| F4 | Race: work admitted between the marker's 200 and the kill; source polls are not counted as busy | `planned_restart.go:321-336,395-415`; `deploy_refresh.go:276-296` | LU1 |
| F5 | The planned kill has no filesystem sync | `manager.go:1447`; hibernate syncs (`hibernate.go:317`), planned restart does not | LU1 |
| F6 | Host and guest can disagree about the boot kind | problem doc residual | LU1 (reported), LU3 (gated) |
| F7 | No health gate and no rollback after an image refresh; CI fails the deploy after the computer is already on the bad build | `ci.yml:1501,1515-1517`; no per-computer prior image | LU3 |
| F8 | In-guest applies self-mint their marker | `planned_restart.go:291-311`; L1 D1 | L1c (consumed by LU5) |
| F9 | A marker outlives a refresh that never happened (vmctl dies between mark and kill): a crash within 15 min reads as planned | `planned_restart.go:26`; clear only on refresh error, `deploy_refresh.go:296-300` | LU1 |
| F10 | A deploy invalidates every snapshot, so every hibernated computer's next wake is a crash boot | S3 design failure 7 and "Restart rule" | LU1 (pending), O6 |
| F11 | Rolling back across a store-schema or work-state epoch runs an old binary on newer state | `updater.go:70-79` (vm-3dc68688 class); D7 | LU2, LU3 |
| F12 | One deploy exposes every idle computer, the owner's included, before any computer has proven the build | `ci.yml:1427-1517` serial, no ring gate | LU3 |
| F13 | Updates are not on the computer's event chain | `lifecycle_receipts.go:28-30`; host-only deploy receipt | LU3, LU5 |
| F14 | A runtime swap that skips the base rebuild re-creates version skew; offers bound at deploy time go stale | a47122de; S2-g gaps | LU4 |
| F15 | *Hypothesis:* drain deadlock — an executing run waits on an occurrence the fence holds, so it never finishes and every drain times out | not yet traced | LU1 |
| F16 | A drain delays the owner's own interaction by up to the deadline | by construction | LU1, O3 |
| F17 | vmctl restarts on every host deploy; a synchronous drain call dies with it | `ci.yml` host restart; vmctl `KillMode=process` | LU1 |
| F18 | The target moves during a drain (deploys about hourly) | deploy cadence 10-13/14 | LU1 |
| F19 | A rollback that fails, or a build that fails on every computer, loops | L1 design §1 | LU3 (one rollback per computer and build; L1b grants) |
| F20 | Ring gating stalls behind a computer that never drains | by construction | LU3 |
| F21 | A planned restart with executing runs strands them: a planned boot passivates and nothing re-drives them | `runtime.go:768-770`; delegation design §8 | LU1 (drain waits for them) |

## 3. Invariants

- **U1 (pull).** A deploy changes only the target. Every attempt to move a
  computer is a sweep decision on that computer's current facts.
- **U2 (no work cut).** The host kills a guest for an update only after
  the guest has answered "drained": admission closed, nothing inside the
  fence, no executing run, no self-development transaction, filesystems
  synced. Busy is never forced by an automatic path (CLAUDE.md "Deploys
  never restart a busy computer").
- **U3 (planned stays planned).** An update boot is planned unless a real
  crash happened. An epoch change is a migration, not a crash.
- **U4 (bounded).** Per computer and target build: at most one apply, one
  automatic rollback, and K drain attempts before a visible "update stuck"
  signal. Same build is never re-applied automatically after a rollback.
- **U5 (recorded).** Every update attempt, outcome and rollback is a record
  on the computer's event chain, joined to the build identities.
- **U6 (one authority).** "Pending" is derived (booted image ≠ target
  image), never stored as a second truth. Attempt history lives in one
  host ledger (L1b's recovery ledger once it exists; interim fields on the
  ownership until then, as L1b-0 did with `FailureWarmedAt`).

## 4. Slices

### LU1: pending, quiet moment, drain

Mechanism:

- **Booted image identity.** vmctl records, at every launch, the guest
  image identity it booted (the resolved store path of
  `/var/lib/go-choir/guest` and its `build.json` commit; vmmanager already
  takes artifact identities for snapshots, `internal/vmmanager/snapshot.go:385-395`).
  Pending = active computer whose booted image differs from the current
  target. `vmctl list` shows `update: {target, booted, pending_since,
  last_attempt, outcome}`.
- **Sweep.** The idle sweeper (`internal/vmctl/ownership.go:957-1004`,
  every 2 min on staging, `nix/node-b.nix:772`) gains an update pass, run
  before the idle stop pass, at most one computer at a time per host (as
  CI does today). For each pending computer, not held, not divergent:
  ask for a quiet restart; if the guest is quiet, refresh now.
- **Quiet restart.** The planned-restart endpoint closes the hibernate
  fence first (generalized to one admission gate with a reason), then
  checks executing runs and self-development transactions, syncs
  filesystems, writes the marker, and answers 200 with the fence held
  (TTL 2 min). Busy lifts the fence and answers 409. This closes F4 and F5.
  If the fence TTL expires without a kill, the guest lifts it **and clears
  the marker** (F9).
- **Drain.** If a computer stays pending past the quiet wait Q (default
  30 min after the target changed), the sweep starts a drain:
  `POST /internal/runtime/update-drain {target, deadline}` answers 202 at
  once. The guest closes admission to **new** work and keeps admitting
  continuations of work already executing (an occurrence for a run that
  was executing when the drain began, plus cancels and deadline closures;
  F15). Each sweep polls `GET /internal/runtime/update-drain`; when the
  guest reports `drained` (same test as the quiet restart, then the marker
  and sync), the host refreshes within the fence TTL. At the deadline D
  (default 15 min, enough for the 10-minute task deadline in the delegation
  design §8) the guest reopens admission, keeps no marker, and reports
  `timed_out` with what was still running. The host backs off (default 30
  min) and retries; after K = 3 timed-out drains for one target it raises
  "update stuck" (owner page and alert). Never forced. The drain is
  guest-timed, so a vmctl restart mid-drain loses nothing (F17).
- **Moving target.** The marker's target is read at kill time; a drain
  started for build B and finished after build C landed applies C (F18).
- **Schedules continue.** A schedule tick that arrives during a drain
  waits at the fence and runs after the planned boot (planned boots keep
  the schedule's generation, `source_schedule.go:333-345`). No schedule
  is closed.
- **Owner activity** (O3): on an interactive-tier computer, an owner
  message during a drain cancels it and defers the update (default); on
  the platform tier it waits.
- **CI.** The deploy step stops refreshing; it sets the target (the host
  activation already does), kicks one sweep, and waits for the test ring
  to reach the target (LU3 adds the gate). Until LU3, CI keeps today's
  refresh loop as the first attempt, and the sweep handles what it defers.
- **Report the guest's verdict (F6).** The guest's `/health` exposes
  `restart: {kind, reason, target}` for the current boot; the sweep records
  it after each update boot, so a planned request that booted as a crash
  is visible on the host.

Ways LU1 could fail (written before code): the drain never drains (F15,
F21) → the timed-out report names the executing runs, and the first move
of the slice measures the paper's quiet gaps; the fence blocks a
liveness-critical internal path (credential renewal, health) → only actor
occurrences pass through the fence, as today; an owner message waits up
to D → O3; the sweep refreshes two computers at once and exhausts the host
→ one at a time; a hibernated computer is pending → it boots the current
image on wake, and F10 stays open until O6.

Rollback: revert. Interim ownership fields are ignored by older vmctl;
the guest's drain endpoint is additive; CI's deploy-time refresh is
unchanged until LU3.

### LU2: work-state migrations

- Replace D7's blanket rule with a registry: for each step
  `n → n+1`, either a migration function or an explicit `crash` entry.
  A unit test fails when `CurrentWorkStateEpoch` changes without a
  registry entry, so every epoch bump forces the author to choose.
- At a planned boot whose marker epoch is lower than the binary's, the
  boot runs each step's migration before the actor runtime starts, in one
  store transaction per step where the store allows, and records a
  `work_state_migrated {from, to, closed: [...]}` record. A migration
  closes only the state whose schema changed (B1's would have closed
  engineering assignments opened under the packet schema), with the fate
  "closed by a work-state migration", and leaves everything else to the
  planned resume. The boot stays planned.
- A step with a `crash` entry, a missing entry, a marker epoch higher than
  the binary's (downgrade), or a migration error: crash boot, as today.
  A crash during a migration leaves the marker consumed, so the next boot
  is a crash boot; migrations must therefore leave state the crash path
  can close.
- **Rollback across an epoch is forbidden** (F11): an update that ran a
  migration is forward-only; LU3 does not roll it back automatically
  (O4).
- Timing: the next epoch bump is expected in C1e B2-B5 (packets and work
  items deleted). LU2 should land before it, or the paper's next refresh
  is again a crash boot.

Rollback: revert; steps without migrations fall back to D7.

### LU3: health, rollback, rings

- **Probation fence.** An update boot starts with admission closed (the
  host passes a probation flag with the planned-boot attestation, L1c).
  The host probes a health contract: `/health` 200 within T_ready
  (default 5 min); served build equals the target; boot kind planned; no
  unit latched by its start limit; active schedule count not lower than
  before. Pass: the host lifts the fence and records `update_applied`.
  Because nothing was admitted on the new build, a rollback loses nothing.
- **Rollback.** vmctl keeps the previous guest image (a GC root on its
  store path, one per distinct booted image still in use) and records each
  computer's last good image. On a failed probe it boots the computer on
  the last good image (vmmanager already accepts per-boot kernel, initrd,
  rootfs and store-disk overrides, `manager.go:1462-1500`) as a planned,
  host-attested boot, records `update_rolled_back`, and marks the target
  build refused for that computer. One rollback per computer and build;
  a failed rollback becomes an L1 hold (F19). Same-epoch only (LU2).
- **Soak.** After the fence lifts, the computer is watched for S (default
  10 min): a crash boot, a start-limit latch or a hold in that window
  counts as a failed update for ring gating (it does not roll back a
  computer that has already run work on the new build; the L1 bounds
  apply).
- **Rings.** Ring 0: test-tier computers (`WarmnessClassTest`,
  `internal/vmctl/warmness_policy.go:16-19`) and a minted canary when none
  is active. Ring 1: platform computers (the paper). Ring 2: paid accounts
  (the owner's computer). Ring 3: everyone else. Ring k+1 opens when every
  ring-k computer that updated passed probation and soak; a computer that
  is still draining or deferred does not block the next ring after the
  ring timeout (default 2 h; F20). A rollback anywhere halts the rollout
  for that build fleet-wide and alerts (O2).
- **Record (U5).** vmctl issues a platform-signed lifecycle receipt with
  action `update` and `{from_image, to_image, outcome, reason}`; the guest
  appends it as `lifecycle_observed` (`lifecycle_receipts.go` gains the
  action). The ring state is a host projection of these receipts.
- Depends on L1c (host-asserted planned boots: the rollback boot and the
  probation flag must come from the host, since the failed guest cannot
  write a marker) and on L1b (grants for the rollback boot).

Rollback: `VM_UPDATE_HEALTH_GATE=off` returns to LU1 behavior; the
retained image GC root is harmless.

### LU4: binary-only swap

- When the only difference between a computer's booted image and the
  target is the autoputer runtime closure (kernel, initrd, rootfs, kernel
  parameters and unit files equal), the sweep offers a **layered release**
  built against that computer's booted base, through the existing updater
  (§1.3). The guest stages, swaps `current`, restarts one unit, probes, and
  restores the prior release on failure, with signed receipts.
- What is different from the deleted push (F14): the base image is still
  rebuilt on every runtime deploy (a47122de stays), so new realizations
  and every later VM boot run the new base, and the boot wrapper drops the
  layer on the new base (`autoputer-vm.nix:133-142`); the offer is minted
  when the guest takes it, for its current realization and head, not at
  deploy time (the S2-g bind gaps); the drain and the planned marker come
  from LU1; the frontend stays out of the layer (layered SPA residual).
- Gain: a process restart (seconds) instead of a VM boot (about 33 s on
  the paper at 05:48 plus readiness), no Firecracker spawn (no L1b grant
  spent), page cache and balloon state kept. Cost: the layered path
  becomes a platform path again (O5).

Rollback: the classifier flag off sends every update down the VM path;
a layer is dropped at the next boot by the base binding.

### LU5: one apply path shared with self-development

- One `ReleaseApply` transaction for every change to what a computer
  runs: `kind` (platform_image, platform_runtime, selfdev_bundle, restore),
  from and to identities (image digest or release digest, CodeRef),
  work-state epoch and store-schema window, admission (quiet then drain),
  restart scope (process or VM), health contract, rollback target, and
  its records on the chain.
- Self-development today runs Freeze → Verify → owner approval →
  materializer → `updater.Apply` → health → `materialization_applied` →
  checkpoint → route, with rollback through `rollback_requested/applied`
  and restore to a prior head (`docs/computer-ontology.md`, "Computer-local
  self-development"). A platform update enters the same path at the
  decision step: a platform-control-signed offer under the follow policy
  (M9a) replaces the owner's approval. Everything after the decision is
  shared: LU1's admission, LU3's probation and rollback, the updater's
  journal, one record shape.
- The materializer's soft hold (`SelfDevelopmentApplyMaterializing`,
  `self_development_materializer.go:186-196`) becomes the same admission
  gate with reason `selfdev_apply`.
- Divergent computers stay out: a platform update to a self-developed
  computer needs typed rebase (`docs/computer-ontology.md`, "Platform
  Updates To Divergent Computers"); it stays pending and visible.
- Identity: after LU5 a computer's effective build can differ from the
  deploy SHA on purpose (pending, rolled back, restored). `vmctl list` and
  the owner's page show target, effective release, event head and route,
  as the Landing Loop's "Product restore vs platform deploy" requires.

## 5. Relation to other stations

- **L1.** LU uses L1's bounds and must not weaken them: update boots are
  external events in the restart design's sense (one episode per
  computer and target); LU3 needs L1c (host-asserted planned boots) and
  L1b (grants, the ledger as LU's attempt history). LU1 needs neither.
- **S3 snapshots.** A deploy invalidates every snapshot; the next wake is
  a cold boot without a marker, so a crash boot (F10). The snapshot fence
  already guarantees nothing was in flight, so once L1c exists the host
  can attest that cold boot as planned. Alternative: keep the previous
  image until its snapshots are consumed (O6).
- **Management delegation.** Bounded tasks with a 10-minute deadline and
  the S1 host cell deadline are what make a drain finish on the paper;
  O6 of that design ("who requests the drain") is answered here: the
  host's update sweep, after the quiet wait.
- **C1e.** One shared-runtime cutover at a time: LU1's guest half (fence
  and drain) is a shared-runtime change and lands between C1e slices, not
  beside one.

## 6. Open decisions for the owner

| # | Decision | Default in this note |
| --- | --- | --- |
| O1 | At the drain deadline: defer, or force after K failed drains | Defer; "update stuck" alert after K = 3. A forced kind only by an owner act |
| O2 | Ring order and gates | test → paper → owner → others; soak 10 min; ring timeout 2 h; a rollback halts the build fleet-wide |
| O3 | Does the owner's activity cancel a drain on their own computer | Yes on interactive tiers; no on the platform tier |
| O4 | Epoch-changing updates | Forward-only, no automatic rollback; ring 0 soak extended to 1 h |
| O5 | Build LU4 (layered runtime swap for platform updates) | After LU3, only if measured planned-refresh downtime warrants it |
| O6 | Hibernated computers after a deploy | Host-attested planned cold boot after a clean snapshot fence (after L1c) |
| O7 | Quiet wait Q and the update-lag target | Q = 30 min; the paper updated within 60 min of a deploy |
| O8 | CLAUDE.md wording ("Deploys never restart a busy computer") | Amend to "a deploy marks an update pending; each computer takes it at a quiet moment or after a bounded drain; busy is never forced" |

## 7. What this does not do

- It does not force a busy computer. A computer whose work never finishes
  inside the drain deadline stays on its build, visibly.
- It does not rebase platform updates onto self-developed computers.
- It does not replace L1's crash bounds; it reduces how often an update
  turns into a crash.
- It does not make deploys of host services zero-downtime (L1 D3, D4).
