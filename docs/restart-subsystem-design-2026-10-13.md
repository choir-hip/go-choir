# Restart subsystem: bounded recovery, no crash loops (design draft, 2026-10-13)

Status: draft for an agentic-consensus review requested by the owner
("Another thing we have to build is the restarting subsystem ... is there a
way we can guarantee we never get into a crash loop?"; "design, verify this
with other agents, and then slot it into the roadmap"). Mutation class when
built: **red** (vmctl, VM lifecycle, run acceptance, deploy routing).

## 1. The claim

We cannot guarantee a computer never crashes. We can guarantee it never
**loops**: the number of automatic restarts, resumes or re-dispatches caused
by failures is bounded between two demonstrations of health, and the bound
is enforced by durable write ordering, not by a heuristic.

**Invariant R.** Every automatic action taken because of a failure
(reboot, image retry, resume of work, re-dispatch of a wake) first spends a
token from a durable ledger for its scope, with the spend committed before
the action starts. Tokens are minted only by a recorded **health proof** for
that scope. With a cap of one token, the worst case between health proofs
is: failure, one automatic attempt, failure, stop with an owner-visible
reason.

Why it holds whatever the cause (bad build, poisoned record, model-written
code, a bug in the recovery path itself): an attempt that crashes has
already spent its token, and a computer that never proves health never
mints another. The only way to exceed the bound is an action that skips
the ledger, so the review question for every recovery path is "does it
spend a token first?"

This generalises the owner's 2026-10-09 rule (CLAUDE.md, "Restarts End Work
(Crash) Or Resume It (Planned Update)"): the planned-restart marker is
already a one-token ledger (written by the host before the restart, consumed
before the resume, so a crashing resume leads to a markerless boot that
closes work).

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

## 3. Design

### 3.1 Scopes and their single authority

Each scope has exactly one ledger and one writer. No second counter in
memory or in another service that could disagree.

| Scope | Covers | Ledger (single authority) | Writer |
| --- | --- | --- | --- |
| **S-host**: computer boot | reboots by vmctl after a guest fails, wedges or dies; always-on warm after a failure | `VM_STATE_DIR/<vm_id>/recovery-ledger.jsonl`, append-only, fsync per entry | vmctl |
| **S-image**: build trial | first boots of a new guest image or layered release | same ledger, entries of kind `image_trial`; extends the layering boot guard to the base image | vmctl (host), boot guard (guest) reports |
| **S-runtime**: in-guest process | systemd restarts of the guest runtime | guest-local `bootguard` counter (exists for layered releases), generalised to every boot | guest boot guard |
| **S-work**: resumption of work | anything reopened or re-dispatched because of a restart: schedules, runs, rounds, platform-update resume, wakes | commitment records on the tape (one `restart_tokens` fold per computer) | the guest runtime, from management's acts |

### 3.2 Health proof (mints tokens)

A scope's health proof is a ledger entry written only when:

- S-host and S-image: the guest answered `/health` ok continuously for
  `T_healthy` (proposed 10 minutes) since its boot, with `runtime_health`
  ok and no boot-guard trip.
- S-runtime: the runtime stayed up `T_healthy` and committed at least one
  durable write (any record, turn or capture) after boot.
- S-work: a turn of the desk that reopened work committed after the
  reopen, and `T_healthy` passed without a crash boot.

A health proof mints one token (cap 1 per scope; S-host may use cap 2 for
transient host faults, decided by review). Tokens never accumulate beyond
the cap, so a long healthy period cannot fund a later loop.

### 3.3 Policies per scope

**S-host (vmctl).**

- Every reboot vmctl starts because of a failure (resolve recovery,
  always-on warm after a non-idle stop, wedge recovery) appends
  `{kind: recovery_attempt, cause, epoch, at}` and spends the token
  **before** booting. No token: the ownership moves to
  `held_after_failures` with the cause chain, and resolve returns a typed
  refusal ("this computer stopped after repeated failures") instead of a
  boot. The "starting" page shows that state and stops auto-refreshing.
- Owner act (or operator) `POST /internal/vmctl/recover` with a reason
  re-arms one token; the act is itself recorded.
- Deliberate boots (owner opens a hibernated computer, deploy refresh,
  operator refresh) do not spend tokens: they are not failure-caused. They
  do count for S-image when the image is new.

**S-image (build trial).**

- A computer's first boot on a new image is a trial. If the guest does not
  reach the health proof within `T_trial` (proposed 5 minutes) or crashes
  before it, vmctl boots the computer's last-good image and records
  `image_trial_failed` for that build. Further deploys skip that build for
  every computer until a newer build arrives.
- This needs per-computer image paths (today one global set of paths at
  `/var/lib/go-choir/guest/*`): keep the previous image beside the current
  one, as A/B slots.

**S-runtime (guest systemd).**

- Add a StartLimit to `go-choir-autoputer` matching the boot guard (for
  example 3 starts in 10 minutes), after which the unit stays failed, guest
  `/health` reports `runtime_failed`, and S-host sees a failed guest
  (which then spends an S-host token, not a fresh loop).

**S-work (management-owned resumption).**

- A crash boot keeps closing every obligation (runs, rounds, schedules,
  wakes, work items) with the visible fate `interrupted_by_restart`. The
  runtime never resumes work (owner rule).
- The crash boot writes **one** system record to management:
  `restart_report {boot_epoch, cause: crash|wedge|refresh|hibernate_wake,
  closed: [obligation refs], suspect: [refs in flight at the crash],
  tokens_left}`. It wakes management only if an S-work token is available;
  otherwise it is recorded and the owner sees "stopped after a crash".
- Management decides what to reopen with ordinary acts (for example
  `choir.Reopen(ref, reason)` or a fresh schedule start). A reopen spends
  the S-work token first and carries `reopened_from` lineage. The decision
  stays in a model, not in a runtime resumer (doctrine: intelligence in
  models).
- **Suspects** (obligations, cells or records in flight at the crash) cannot
  be reopened by management; only an owner act reopens them. A poisoned
  capture cannot cause a loop and does not block the healthy schedules.
- An obligation already reopened once and then closed by another crash
  needs an owner act.

**Planned restarts and updates.**

- The marker write also sets an **admission fence**: the guest refuses new
  run admission (typed 409 to the caller, retried later) from the moment it
  answers 200 until the restart or marker expiry. This closes the
  busy-check race.
- Platform-update resume spends an S-work token before re-driving an
  accepted update; a resume whose boot crashes finds no token and records
  `update_failed_after_restart` instead of re-arming the marker.

**Hibernate and wake.**

- An idle hibernation is planned, not a crash: vmctl asks the guest for a
  marker with reason `hibernate` before the stop (same route as deploy
  refresh). A wake with that marker resumes (schedules keep their state); a
  wake without one is a crash boot as today. Real memory snapshots stay
  later work.

### 3.4 Owner visibility

- vmctl ownership gains `recovery {tokens, last_attempt, held_reason,
  history (last N ledger entries)}` in `/internal/vmctl/list`, and the
  product shows a computer "stopped after repeated failures" with the cause
  chain and a "try again" owner act.
- Guest `/health` gains `boot {epoch, kind: crash|planned|hibernate_wake,
  restart_tokens}`.
- Autopaper gets its "paper stopped" signal from the S-work state.

## 4. What this does not do

- It does not prevent crashes or make them cheap; it bounds their repetition.
- It does not resume model state mid-turn; reopened work starts a new turn.
- It does not add runtime budgets, scoring or round caps on model work
  (2026-10-12 decisions): tokens bound failure-caused automation only.

## 5. Acceptance (tracer scenarios that try to loop)

Each runs on a minted test computer through product and internal routes the
tracer already uses; each must be able to fail.

1. **Poisoned record.** Plant a record that makes management's cell crash
   the guest runtime (a test-only crash hook behind a build flag).
   Pass: at most one automatic reopen; the second crash ends with
   `held`/"stopped after a crash"; zero further failure-caused boots over
   30 minutes; the suspect record is named in the restart report.
2. **Post-ready crash.** A test hook that kills the runtime 30 s after
   readiness, on every boot. Pass: S-runtime StartLimit trips, S-host spends
   its token(s) and holds; the "starting" page stops refreshing; total boots
   ≤ cap + 1 over 30 minutes.
3. **Bad image.** Deploy a guest build that crashes at boot to one test
   computer. Pass: one trial boot, fallback to last-good, `image_trial_failed`
   recorded, the next deploy skips that build.
4. **Planned restart race.** Request a planned restart and submit a prompt
   in the same second. Pass: the prompt is refused with a typed retry, never
   killed mid-run.
5. **Hibernate wake.** An idle paper-like computer with schedules is
   hibernated and woken. Pass: schedules resume (not closed).
6. **Health re-arm.** After a held computer is recovered by owner act and
   stays healthy `T_healthy`, a later single crash is again recovered once.

## 6. Order of work (proposed)

1. S-host ledger and hold (closes the unbounded resolve and always-on loops;
   host only, no guest change).
2. S-runtime StartLimit and boot guard generalised (guest image change).
3. Admission fence and platform-update token (guest).
4. S-work restart report and management reopen (after C1e moves
   engineering and work items onto records; the reopen act is a records
   act).
5. Hibernate marker.
6. S-image A/B slots (host image layout change; largest).

## 7. Open questions for review

- Is one token per scope the right cap, or does S-host need two for
  transient host faults (OOM under pressure, a host reboot)?
- Is `T_healthy` a duration, a committed write, or both, per scope?
- Should S-work reopen be management's decision at all, or should crash
  reopen always need the owner until self-development is trusted?
- Where exactly does a poisoned *capture* (not a cell) get attributed as a
  suspect?
- Does the admission fence risk a deploy starving a computer that is never
  idle (the paper), and should a planned restart instead wait for a turn
  boundary?
- Is there any recovery path in the map above that this design misses?
