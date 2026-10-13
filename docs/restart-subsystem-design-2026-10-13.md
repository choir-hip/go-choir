# Restart subsystem: bounded recovery, no crash loops (design draft, 2026-10-13)

Status: **v3**: v2 after a lateral review (seven panelists, §8), amended by
the convergent round (six answered, five ready with changes, §9). Where §9
and the text above it differ, §9 governs. Requested by the owner
("Another thing we have to build is the restarting subsystem ... is there a
way we can guarantee we never get into a crash loop?"; "design, verify this
with other agents, and then slot it into the roadmap"). Mutation class when
built: **red** (vmctl, VM lifecycle, run acceptance, deploy routing).

## 1. The claim (v2)

We cannot guarantee a computer never crashes. We can guarantee it never
**loops**: between two **external events** (an owner act, or a deploy that
brings a different build), the number of automatic boots and automatic
re-drives of work is bounded by a small fixed number, and after that the
computer is **held** (latched) and visible until an external event releases
it.

**Invariant R (v2).** Every Firecracker spawn requires a **grant** recorded
durably (fsync) in the computer's recovery ledger before the spawn. Grants
come from two sources only:

1. an **external event** (owner act, owner opening the computer, a deploy
   refresh carrying a new build, a planned transition the host initiated):
   each opens a new episode with a fresh allowance;
2. the episode's **automatic allowance** (proposed: one automatic retry,
   plus one fallback boot when the failure follows an image change).

Health does **not** refill the allowance (v1 did, and the panel showed the
result is a rate bound: a crash every `T_healthy + 1` minutes loops forever).
Running the same code on the same state without an external change is not
recovery, it is a loop waiting to happen. A long-healthy computer that
crashes again simply opens a new episode only if an external event occurred
in between; otherwise it spends the remaining allowance and then holds.

Why it is a guarantee: the gate sits at the one call every Firecracker
spawn passes through (vmmanager `launchFirecracker`, reached from both
`bootVM` and `ResumeVM`; §9 corrects v2, which named `bootVM`), so no caller
can skip it; grants are keyed by a per-computer spawn sequence, so a resume
that reuses an epoch cannot reuse a grant; the ledger is written before the
spawn and fails closed (missing, corrupt or unwritable means hold); and the
allowance is only replenished by events that change the inputs. Any number
of callers (resolve, warm sweeps, proxy retries, a browser's auto-refresh)
can only ask; they cannot add boots. Inside a running VM, systemd start
limits bound restarts of each unit, so the total is at most
(1 + allowance) spawns times each unit's start-limit burst per external
event.

**Invariant W (work).** A crash boot is side-effect free: it loads state,
closes every open obligation as `interrupted_by_restart` (as a completion
barrier: if closure fails, the runtime stays quiescent rather than
continuing), and starts **no** re-driver of pre-boot work. Re-drivers (the
wake projector's pre-boot wakes, self-development reconcile and retry,
platform-update resume, engineering recast, settlement repair) run only on a
boot whose planned-ness the **host** attests (below).

**Invariant P (planned).** A boot is planned only when the guest's marker
and the initiator's transition record agree on a nonce. The guest can ask
for a planned restart; it cannot declare one. A marker without a matching
host record is ignored (the boot is a crash boot), so a crash inside an
apply cannot masquerade as planned.

## 2. What exists today (code map, 2026-10-13)

Facts from a code survey; file:line references are in the survey notes and
the cited problem records.

| Path | What it does | Bounded? |
| --- | --- | --- |
| vmctl resolve recovery (`ownership.go` `ensureActiveVMReady` → `recoverOrRestartActiveVM`) | a `failed` guest is rebooted on the next resolve, including the "starting" page's auto-refresh every 5-30 s | **no**: no count, no backoff; `RecoveryAdmission` only sees typed pre-readiness boot refusals |
| Always-on warm (`WarmAlwaysOnDesktops`, idle sweep every 2 min) | resumes any stopped always-on primary, including `stopped_by=wedged` | **no** |
| Wedge watchdog (`wedge.go`) | stops a guest refusing TCP for 5 min | stop is bounded; the reboot after it is not |
| Guest systemd (`nix/autoputer-vm.nix`) | `Restart=on-failure`, `RestartSec=1`, no StartLimit | **no**; every in-VM restart is a crash boot |
| Layering boot guard (`autoputer-vm.nix`) | 3 starts in 60 s of a layered release falls back to the base runtime | yes: the only boot counter and last-good fallback |
| Planned-restart marker (`agentcore/planned_restart.go`) | written fsync+rename, 15-minute TTL, consumed once before the actor runtime starts | yes (one token), but **race**: a run can be admitted between the busy check (200) and the kill |
| Platform-update resume (`platform_update.go`) | runs on every boot, crash or planned; re-drives an accepted update and writes a new marker each time | **no**: if the apply kills the guest, each boot reads as planned and retries |
| Crash-boot closure (`runtime.go` `closePreBootRunsAfterCrash`, rounds, schedules, selfdev) | closes runs, rounds, schedules, pre-decision selfdev ops as `interrupted_by_restart` | yes for those; work items may stay open (problem 2026-10-12) |
| Actor dispatcher | attempt recorded before the handler; 8 attempts, 64 deferrals, then the poison sink | yes |
| Engineering recast | 3 attempts | yes |
| Hibernate (`manager.go`) | kills the process, no snapshot; wake is a fresh, markerless boot | wake is a **crash boot**: an idle hibernation closes schedules |
| Deploy refresh (`deploy_refresh.go`, CI) | marker via guest, deferred when busy, crash boot for guests predating the route | bounded per deploy |

Durable history today: `ownerships.json` (state, `stopped_by`, epoch; no
crash count), the epoch file (a boot counter), `boot-timeline.json` (last
boot only), `receipts/destructive.jsonl` (append-only kill causes),
`recovery-conditions.json` (typed refusals).

## 3. Design (v2)

### 3.1 The recovery ledger (host, single authority for boots)

- One append-only ledger per **ComputerID** (not per VM directory, so
  realization replacement, restore or reconstruction cannot reset it):
  `VM_STATE_DIR/recovery/<computer_id>.jsonl`, fsync per entry and on the
  directory. Entries: `episode_open {cause: external event ref}`,
  `grant {kind: external|automatic|fallback, boot_epoch, caller}`,
  `boot_result {epoch, outcome}`, `hold {reason, cause_chain}`,
  `release {owner act or deploy ref}`.
- Gate: `bootVM` refuses to spawn without a grant written for that epoch.
  Grants are idempotent by epoch, so a retried request cannot spend twice
  and a crash after the grant but before the spawn loses that grant
  (conservative: the computer may hold one boot early).
- Allowance per episode: 1 automatic + 1 fallback (only if the episode
  began with an image change). Exhausted: `hold`, typed refusal on resolve
  ("stopped after repeated failures", cause chain, Retry-After none), the
  starting page stops auto-refreshing, the proxy never retries into a boot.
- Warm sweeps (always-on, platform computer) may request a grant only for a
  computer whose last stop was a clean, host-attested planned stop (idle
  hibernation with marker). They never warm `failed`, `wedged`,
  `recovery_failed` or held computers.

### 3.2 Process supervision (host and guest)

- Every systemd unit that can restart (guest runtime, updater, the
  updater's restart bridges; host vmctl, proxy, corpusd, gateway, auth,
  maild, checkpointd) gets `StartLimitIntervalSec`/`StartLimitBurst` (for
  example 3 in 10 minutes) and an `OnFailure=` that records the failure; a
  failed guest runtime makes `/health` report `runtime_failed`, which the
  host treats as a failed boot (spends the allowance; it does not route
  around it via the wedge path).
- The guest boot guard is rewritten durable (fsync, monotonic window,
  write errors fail closed).

### 3.3 Planned transitions (host-attested)

- The initiator (vmctl for deploy refresh, operator refresh and idle
  hibernation; the updater for in-guest applies) writes a transition record
  with a nonce before asking the guest to write its marker with the same
  nonce; the boot consumes the marker only if the nonce matches the host
  record, and the host consumes its record once.
- Admission fence: from the moment the guest accepts a planned restart it
  refuses new run admission until the restart or the record expires; to
  avoid starving never-idle computers, the host requests the restart only
  at a turn boundary (the guest answers 409 while a turn is open, and the
  host retries within a bounded window).
- Hibernation markers carry no short TTL (the host record bounds them), so
  an overnight wake is planned.
- An apply that crashes: the boot sees no host-attested record (the updater
  records `restart_requested` only after the apply returns), so it is a
  crash boot; platform-update and self-development re-drive spend the
  episode allowance and record `update_failed_after_restart` when it is
  gone.

### 3.4 Work after a crash (owner-owned for now)

- Crash boots close everything (owner rule) and write one
  `restart_report {epoch, closed refs, in-flight suspects}` record. Nothing
  reopens automatically. The owner (or, later, management under a ratified
  policy) reopens.
- **Standing intents versus occurrences** (owner decision needed for the
  paper): a source schedule is a standing owner intent; a poll is an
  occurrence. Option A (today): a crash closes the schedule, the owner act
  restarts it. Option B: a crash closes the in-flight occurrence only; the
  schedule's next tick is new work, on a computer that booted under a
  grant, and a schedule whose tick crashes the computer twice is closed
  with that cause. B keeps the paper publishing unattended without any
  resume path.
- Management-owned reopen waits until crash attribution is trustworthy
  (suspects today cannot always be named: a poisoned durable input can be
  reread by fresh work).

### 3.5 Host-wide faults

- A mass-hold detector: M computers held within T minutes is a host
  incident, not M computer failures. It records the incident, pages the
  owner, and suppresses image blacklisting; per-computer holds stay, so
  nothing loops, and a deploy carrying a fix is an external event that
  releases them (deploy refresh may boot held computers when the build
  changed).

### 3.6 Images (later)

- A/B slots with a trial boot are kept as a later step. A failed trial
  blacklists a build only for guest-attributable failures (boot timeline
  reached userspace, guest refusal), never host provisioning faults, and a
  fallback must be state-compatible (a persistent computer's data may have
  changed under the new image).

### 3.7 Owner visibility

- vmctl list and the product show `recovery {episode, allowance left,
  held_reason, cause_chain, last N entries}`; guest `/health` shows
  `boot {epoch, kind: crash|planned, nonce}`; Autopaper gets its "paper
  stopped" signal from the hold.

## 4. What this does not do

- It does not prevent crashes; it bounds their repetition to a fixed number
  per external event and makes the stop visible.
- It does not keep a computer available through repeated faults: a fault
  that recurs on the same build holds it until an owner act or a new build.
  The owner's 2026-10-10 ruling ("users shouldn't ever have to do this")
  is honoured by making holds rare (one automatic retry absorbs transient
  faults) and by deploys releasing holds, not by unbounded retry.
- It adds no runtime budgets, scoring or round caps on model work.

## 5. Acceptance (fault-injection matrix on a minted test computer)

Faults: crash at boot; crash 30 s after ready; crash just after a long
healthy window; crash inside an apply after the marker; a poisoned record
that crashes management's cell; vmctl crash-looping; a host-wide tap fault
on two test computers. Drivers for each: resolve, browser auto-refresh,
proxy retries, always-on warm, platform warm, wedge, idle hibernate and
wake, deploy refresh (same build and new build), operator refresh, vmctl
restart, updater recovery restart. Crash points: before and after the grant
write, the spawn, the boot result and the closure write; plus a corrupt or
missing ledger and a clock jump.

Pass, per cell: Firecracker spawns per episode ≤ the allowance (counted from
the ledger and `receipts/destructive.jsonl`); the final state is `held`
with the cause chain; no spawn for 60 minutes after the hold under all
drivers; a crash boot starts no re-driver (no pre-boot wake dispatched, no
reconcile, no update resume); an apply crash is a crash boot; an owner act
or a new-build deploy releases exactly one new episode; the mass-hold
detector fires once for the host fault.

## 6. Order of work (v3, per §9)

1. **L1a, the self-sustaining loops** (small; guest and Nix): start limits
   and `OnFailure` on every restarting unit, guest and host; a markerless or
   guest-self-marked boot runs no re-driver (platform-update resume,
   engineering capsule reconcile, self-development reconcile and
   post-commit, the wake projector's pre-boot sweep, the
   `selfdev_materialization_retry` exemption), and crash closure settles
   those transitions to terminal fates instead; warm sweeps never warm a
   computer stopped as wedged, failed or `recovery_failed`.
2. **L1b, the boot ledger**: grants at `launchFirecracker` keyed by spawn
   sequence, no ungranted epoch auto-increment, exhaustion sets the existing
   hold with a `recovery-ledger` class, owner acts are explicit mutations
   only, the held page does not refresh, the proxy treats the hold refusal
   as final, deploy refresh releases a recovery hold only for a different
   build.
3. **L1c, host-asserted planned boots**: the host passes planned-ness and
   its transition record at boot (boot parameter or credential disk); the
   guest writes no marker of its own; crash closure becomes a barrier.
4. Visibility (the ledger's tail in `vmctl list` and the owner's computer
   page; an alert on any hold).
5. Schedules option B, once the owner ratifies it.
6. A/B image slots (later).

## 7. Open questions for the convergent round (answered in §9)

- One automatic retry per episode, or two?
- Should a deploy of the *same* build release a hold (it is an owner-
  initiated transition, but the inputs did not change)?
- Is "turn boundary" restart feasible for the paper, or does it need a
  bounded wait-then-force policy?
- Option A or B for schedules (an owner decision; what should the default
  be?).

## 8. Lateral review (2026-10-13), adopted

Seven panelists (claude opus, codex, gpt-6.1-sol, gemini-3.8-flash,
deepseek-v4.1-flash, muse-spark, opencode); outputs under
`.agentic-consensus/restart-lateral-2026-10-13/`. Findings adopted into v2:

- v1's token refill on health was a rate bound, not a loop bound (claude,
  gpt-6.1-sol, deepseek); health is self-reported by the judged party
  (muse).
- Enforce at the actuator, not per caller; boot-cause classification is
  lossy (vmctl load rewrites states to `vmctl-restart`) (claude, deepseek,
  codex).
- The guest self-mints planned markers before applies
  (`platform_update.go:263`, `self_development_materializer.go:315,390`),
  so an apply crash boots as planned and re-drives (deepseek, codex).
- Missed paths: vmctl and every Node B unit restart without StartLimit
  (`node-b.nix`); the updater's restart bridges and recovery restart
  (`autoputer-vm.nix:474-510`, `updater.go:338-365,958-975`); post-decision
  self-development reconcile and retry wakes; the wake projector's redrive
  with fresh occurrence ids and process-local retry counts
  (`actor_wake_projector.go:17-34`, `adapter.go:373-386`); the platform
  warmer calling `ResumeVM` directly (`platform_computer.go:75`); proxy
  retries and the starting page's meta-refresh; hibernate and warm
  oscillation; repeated deploy refreshes.
- Persistence: ownership and recovery-condition writes lack fsync and log
  errors; the boot guard truncates and resets on wall-clock windows
  (gpt-6.1-sol, codex, claude).
- Crash closure is best-effort, not a barrier (`runtime.go:2399-2450`)
  (gpt-6.1-sol, codex).
- A host-wide fault would hold everything and block the fix's deploy
  (deepseek).
- A/B fallback needs state compatibility (gpt-6.1-sol, codex); image
  blacklisting must not follow host faults (deepseek).
- Reopen: owner-only for now (all seven); the paper needs an explicit
  standing policy (codex, claude, muse).

## 9. Convergent round (2026-10-13), adjudicated

Panel: convergent, seven selected plus glm-5.3. Answered: claude opus,
gemini-3.8-flash, muse-spark, deepseek-v4.1-flash, glm-5.3-flash (**ready
with changes**), opencode (**not ready**; three blocking findings that the
others share). codex and gpt-6.1-sol failed on an OpenAI usage limit.
Raw outputs: `.agentic-consensus/restart-convergent-2026-10-13/` and
`-glm/` (session diagnostics).

### Blocking findings, adopted

1. **`bootVM` is not the single actuator** (all six). `ResumeVM`
   (`internal/vmmanager/manager.go:966`) copies the launch sequence and calls
   `launchFirecracker` itself (`:1036`) without `ReserveBootEpoch`; its live
   caller is the platform warmer (`internal/vmctl/platform_computer.go:75`).
   The gate moves to `launchFirecracker` (`manager.go:1827`), and
   `consumeBootEpoch`'s auto-increment for an unsupplied epoch
   (`manager.go:628-654`) must not mint a spawn without a grant.
2. **Grants keyed by epoch are replayable** (claude, glm, deepseek): resume
   deliberately keeps the epoch (`manager.go:1027-1031`). Grants are keyed by
   a per-computer spawn sequence; the epoch is a recorded field.
3. **"Owner opening the computer" is a refill** (all six). The starting page
   meta-refreshes every 5-30 s (`internal/proxy/computer_surface_page.go:66`),
   resolve of a failed computer recovers it (`ownership.go:1150-1155`), and
   for the platform computer every proxied request re-runs the ensure path
   (`handlers.go:1040-1045`). An owner act is an explicit authenticated
   mutation only: the lifecycle start, restart or recover routes
   (`computer_lifecycle.go`), unhold (`handlers.go:555`), or a schedule
   restart. Resolve, page loads, probes and proxy retries never open an
   episode; a held computer's page does not auto-refresh and offers a retry
   button; the proxy treats the hold refusal as final.
4. **A hold already exists** (claude, deepseek, glm). `HoldStatus`,
   `SetHold` and `ClearHold` (`ownership.go:157-200`) are durable and
   host-owned, honoured by resolve, both warmers, idle candidates and deploy
   refresh. Exhaustion sets that hold with `held_by=recovery-ledger`; the
   ledger only counts and records the cause chain (one state authority).
   Deploy refresh skips held computers today; it changes to "release a
   recovery-ledger hold only when the build differs from the build recorded
   at hold time".
5. **The fastest loop never spawns Firecracker** (all six). Guest units
   restart after 1 s with no start limit (`nix/autoputer-vm.nix:590,626,702,
   827`), and every boot runs re-drivers (`internal/agentcore/runtime.go:
   761-770,783-788`). An apply that crashes the runtime restarts it in a
   second, the guest re-mints its marker (`platform_update.go:263`;
   `self_development_materializer.go:315,390`), the boot reads planned and
   re-applies. The boot ledger never sees it. Start limits and crash-boot
   re-driver gating move to the first slice (L1a), and §1 now counts the
   in-guest bound.
6. **Invariant W and v2 §3.3 contradicted each other** (deepseek, muse). W
   governs: a crash boot never re-drives. Crash closure settles each
   transition to a terminal fate (`update_failed_after_restart`; a
   self-development op failed by restart) and discharges the
   pending-transition reference (`platform_update.go:829-834`) so later
   updates are not refused forever.
7. **Wakes**: closure does not touch unprojected actor wakes; the projector
   sweeps immediately on any boot, redrive mints salted generations with a
   fresh `CreatedAt` (`runtime.go:2527-2547`) that pass the handler's
   wall-clock pre-boot check, and retry counts are process-local
   (`actor_wake_projector.go:26-34`). The sweep is gated on boot kind (crash
   boots mark pre-boot wakes interrupted), and
   `selfdev_materialization_retry` loses its exemption on crash boots
   (`internal/actorruntime/handler.go:98-113`).
8. **Warm sweeps warm wedged computers** (muse, deepseek):
   `WarmAlwaysOnDesktops` filters state but not `stopped_by=wedged`
   (`ownership.go:3051-3053`). Filter it in L1a; it closes the wedge-warm
   cycle on its own.

### Non-blocking, adopted

- The guest boot guard's 60 s window resets on a loop just over a minute
  (`autoputer-vm.nix:162-178`); the start limits replace it as the bound.
- Ownership and epoch writes lack fsync (`ownership.go:565-570,598`;
  `manager.go:2476-2486`); the ledger is the durable authority, and holds
  must not depend on `ownerships.json` alone.
- Maintenance recovery bypasses admission (`ownership.go:2661-2665`): it is
  an operator act, so it passes the gate as a recorded external grant.
- Crash closure skips production computers without an owner id and logs and
  continues on errors (`runtime.go:2400-2421,2438-2460`); L1c makes it a
  barrier.
- Check that the proxy's transient-error test excludes the hold refusal.

### Answers to §7

- **Retries: one** automatic retry per episode (all six), at least 60 s
  after the failure, plus the fallback boot only for an episode that began
  with an image change and a failure that never reached userspace.
- **Same-build deploy: never releases a hold** (all six). Only a different
  build (recorded in the ledger at hold time) or an explicit owner act.
- **Turn boundary: wait, then defer** (claude, glm, deepseek; muse, gemini
  and opencode preferred wait-then-force). Adopted: defer, because CLAUDE.md
  says deploys never restart a busy computer and the paper idles between
  polls. A recorded forced-refresh kind is added only if a never-idle
  computer blocks a needed fix.
- **Schedules: option B as the default** (all six), pending the owner's
  ratification: a crash closes the occurrence in flight; the next tick is
  new work on a granted boot; a durable per-schedule count closes a
  schedule whose ticks crash the computer twice, with a visible
  "paper stopped" record. Every idle hibernate wake today is a crash boot
  that closes schedules (glm), so option A silences the paper routinely.

### Over-built, trimmed

- The nonce handshake and admission fence (§3.3) give way to a host-asserted
  planned boot (muse): the host already injects per-boot material, so it
  passes planned-ness and its transition record at boot and the guest
  writes nothing. Self-minting becomes impossible by construction. The
  guest's busy endpoint stays for deferral.
- The mass-hold detector becomes an alert on any hold (two computers).
- The `recovery {...}` view and `restart_report` start as the ledger's tail
  beside `HoldStatus` in `vmctl list`.

### Acceptance additions

Matrix cells added: platform-warmer resume, guest unit start limit, a
resolve storm after a hold (no spawn for 60 minutes), a same-build deploy
(no release), page open (no release), resume with a reused epoch (no
reused grant), and an apply that crashes the runtime (crash boot, settled,
no re-apply).
