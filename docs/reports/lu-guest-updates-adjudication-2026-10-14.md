# LU: Guest Updates, Panel Adjudication (round 1)

Date: 2026-10-14. Reviewed: the design note
([`lu-guest-updates-design-2026-10-14.md`](lu-guest-updates-design-2026-10-14.md))
and the station file
([`choir-ledger-lu-guest-updates-2026-10-14.md`](../definitions/choir-ledger-lu-guest-updates-2026-10-14.md))
at commit 346d968c (round 1). Mode: convergent. Raw outputs (session
diagnostics, outside the repo):
`/private/tmp/claude-501/-Users-wiz-go-choir/3631f12d-c64d-40d6-a462-dc5bc4bd8b0b/scratchpad/panel-lu/`.

## Verdicts

| Panelist | Verdict | Confidence |
| --- | --- | --- |
| claude (opus) | accept with repairs | medium-high |
| gemini-3.8-flash | accept with repairs | high |
| glm-5.3-flash | accept with repairs | medium |
| muse-spark | accept with repairs | high |
| deepseek-v4.1-flash | accept with repairs | high |
| gpt-6-luna | send_back | high |

Outcome: **send_back for round 1.** Five of six asked for repairs before
LU1 code, and the one send_back names blockers the others share
(fence-to-kill window, two update drivers) plus one only it found (the
bootstrap). Every blocking finding is adopted below and written into
round 2. All six said LU1 can start without the owner once the repairs
are in and a round-2 panel accepts the amended files. Round 2 has not run;
the station stays `drafted`.

Every finding was checked against the code before it was adopted. "Verified"
cites what was read.

## Blocking findings

| ID | Finding (panelists) | Verified | Disposition |
| --- | --- | --- | --- |
| R1 | **The drain blocks dispatcher slots.** Held occurrences wait inside `EnterWork`, each holding one of the dispatcher's 8 concurrent activations, so the continuation an executing run needs never gets a slot (claude; F15 also gemini, glm, muse, deepseek) | `internal/actor/dispatcher.go:159-161` (MaxConcurrent default 8); `hibernate.go:129-160` blocks in `enter`; `handler.go:171` | **Adopt.** The drain holds new work by returning `actor.ErrDeferUnprocessed` before `EnterWork`, as `applyHoldsWork` does (`handler.go:61-78,165-168`). Held work stays pending on the tape, uses no slot, and survives a planned boot |
| R2 | **The deferral budget bounds the drain.** 64 deferrals with backoff capped at 30 s is about 29.5 min; past it, held work goes to the poison sink (claude) | `adapter.go:137` (MaxDeferrals 64); `dispatcher.go:18-27` (0.5 s doubling to 30 s): 31.5 s + 58 × 30 s ≈ 29.5 min | **Adopt.** D plus the quiet-to-kill window must stay under the deferral budget; a unit test fails if it does not |
| R3 | **"Continuation" must be decided by run lineage, not by kind.** A child `initial_dispatch` from a run executing at drain start looks like new work (claude, glm, gemini, muse, deepseek) | `handler.go:171` calls `EnterWork(ctx)` with no occurrence identity; `hibernate.go:68-83` is an anonymous counter | **Adopt.** The drain snapshots the root runs and trajectories executing at its start; an occurrence whose root is in the set is admitted (including child spawns and results), plus cancels and deadline closures; all other `RestartWorkKinds` are deferred. Acceptance adds a child spawn |
| R4 | **One on/off flag cannot carry hibernate, quiet and drain.** Hibernate's lift or its 2-minute TTL would reopen a drain (claude, muse) | `hibernate.go:93-104` (`liftLocked` clears the one flag), `:132-137` (TTL lift), `:107-119` (`close` refuses while anything is inside) | **Adopt.** One gate with a set of reasons, each with its own deadline: `hibernate`, `update_drain`, `update_final`, later `selfdev_apply`; hibernate prepare refuses while a drain is active |
| R5 | **The 2-minute fence TTL races the 2-minute sweep and the kill.** A `drained` answer, a delayed poll and a ~38 s refresh can outlive the fence, so the host kills a guest that reopened and cleared its marker (gemini, glm, deepseek, luna, muse) | `hibernate.go:39`; `nix/node-b.nix:772`; `manager.go:1436-1459` | **Adopt.** `drained` holds new work under the drain hold (no marker, no short TTL). The host then makes one synchronous final call in the same vmctl goroutine: the guest closes the `update_final` reason, re-checks, syncs, writes the marker, and the runtime exits cleanly; vmctl waits up to 30 s for the runtime to stop answering, then kills. If the final call fails or the lease lapses, vmctl aborts and does not kill |
| R6 | **Two update drivers.** CI's deploy-time loop and the sweep can mark and refresh the same or different computers at once, defeating one-at-a-time (claude, gemini, glm, deepseek, luna) | `ci.yml:1427-1470` serializes only CI; `ownership.go:957-1004` has its own `sweepMu`; per-computer in-progress guard only (`ownership.go:2871-2884`) | **Adopt in LU1.** vmctl is the only update driver, one computer at a time host-wide, under a per-computer claim. CI sets the target, kicks one sweep, and reports. The forced dispatch input goes through vmctl and skips any computer with an active drain (glm) |
| R7 | **Bootstrap: the busy paper cannot use a drain it does not have yet.** The paper runs e3437f1c, which has no drain endpoint (luna) | `planned_restart.go:349-415` is the only restart route on today's guests; `deploy_refresh.go:276-293` | **Adopt.** LU1 splits: LU1a is host-only (the sweep retries the existing planned-restart endpoint every pass), which reaches today's guests at their quiet moments (the paper accepted markers at 05:48, 06:03 and 06:36 on 10-14). If the paper has no quiet moment before LU1b ships, one owner-approved refresh installs drain-capable code, recorded as the bootstrap |
| R8 | **The 60-minute acceptance contradicts "never force".** Q 30 + D 15 + backoff 30 already exceeds 60 min after one timeout (luna, muse) | design §4 LU1 numbers | **Adopt.** The acceptance becomes conditional: the paper updates within 60 min of a deploy when its work drains inside D; otherwise the state is visibly pending or stuck. The update-lag distribution is reported either way |
| R9 | **The quiet test misses desk mutations.** vmctl's idle test counts `desk_pending_mutations`; the guest's restart test does not (gemini) | `ownership.go:3108-3125` vs `planned_restart.go:321-336` | **Adopt.** The quiet and drained tests count executing runs, desk pending mutations, self-development transactions and occurrences inside the gate |
| R10 | **The sweep can hibernate a pending or draining computer** (`StopIdleVMs`, pressure reclaim run in the same sweep), so its wake is a crash boot (claude, glm, muse) | `ownership.go:982-1000` | **Adopt.** An idle pending computer is updated, not hibernated; a draining computer is excluded from idle stop and pressure reclaim; `/health` exposes the drain |
| R11 | **LU1 deploys with no enablement flag** and starts draining the paper under unruled defaults (muse) | design: no LU1 flag | **Adopt, amended.** `VM_UPDATE_SWEEP` with per-ring enablement; ring 0 at landing, the paper and the owner's rungs after ring 0's LU1 acceptance passes. The defaults are conservative, so this waits for evidence, not for the owner (No Blocking Asks) |

## Slice-gating findings (blocking their slice, not LU1)

| ID | Finding (panelists) | Verified | Disposition |
| --- | --- | --- | --- |
| R12 | **LU2 keyed on the marker misses markerless boots** (hibernate wakes, the post-L1c world) and a failed migration runs the new binary on unmigrated state (deepseek) | `store.go:678-682` persists the store-schema epoch with `storeschema.Write` | **Adopt.** The work-state epoch is persisted in the store the same way; migrations run on every boot where the persisted epoch is lower than the binary's, before any closure; planned-ness stays the marker's (later the host's). A failed migration holds the runtime (no admission, visible), never runs on unmigrated state |
| R13 | **LU2 is time-critical; move it up** (claude, gemini, glm; deepseek: a hard veto on the next bump) | C1e B2-B5 expected to bump the epoch | **Adopt.** New order: LU1a and LU2 first (different surfaces: vmctl and the guest), then LU1b |
| R14 | **Rollback must also refuse a store-schema change** (claude, muse, luna) | `updater.go:70-79,419-457` | **Adopt.** The image's `build.json` carries its store-schema version and work-state epoch; automatic rollback only when both equal the previous image's |
| R15 | **Split LU3**: rings, refusal records and halt need neither L1c nor L1b (claude) | design LU3 | **Adopt.** LU3a: rings, per-computer refusal, halt-and-alert, on the existing health wait. LU3b: probation hold and automatic rollback boot (needs L1c, L1b) |
| R16 | **The ring timeout lets an unvalidated ring advance** (deepseek, gemini, luna) | design LU3 | **Adopt.** A ring advances only when at least one of its computers validated and none failed; a timeout without a validated member halts and alerts. Luna's quarantine of late members is **not adopted**: a member that updates after later rings validated the build takes it under the same health gate; it no longer gates anything |
| R17 | **An `update` lifecycle receipt aborts older guests' receipt loop** (deepseek, luna) | `lifecycle_receipts.go:28-33` returns an error on an unknown action, which stops `start`, `stop` and credential receipts | **Adopt.** First a guest change that acknowledges and skips unknown actions; the host sends `update` receipts only to images that declare support |
| R18 | **Probation holds must not lose wakes on rollback** (muse) | follows from R1 | **Adopt.** Probation defers rather than blocks, so held wakes stay pending and are delivered after the planned rollback boot |
| R19 | **LU4 should not be built on spec** (claude, gemini, muse: cut or presumed cut; glm, deepseek: conditional; luna: keep the capability, never as a CI push) | a47122de; `updater.go:276-282` base join | **Adopt as conditional.** LU4 leaves the unconditional finish. It is built only if LU1's measured planned-refresh downtime matters, as the process-restart scope of ReleaseApply (luna), with three obligations first: a per-base layer builder (muse, gemini), pending derivation for layered computers (glm), and the S2-g bind gaps closed at take time. The owner asked for lighter restarts, so O5 stays the owner's |
| R20 | **LU5 needs the self-development effects gate**, and its restore proof must join event head, content witness, VM-local projection and route (luna, muse) | `docs/computer-ontology.md:74-78,100-119` | **Adopt.** Dependency and acceptance amended |
| R21 | **Retained images and moving targets need GC roots** (muse, gemini) | `node-b.nix:955-1000` one symlinked store output | **Adopt.** vmctl roots every image named by a computer's booted, last-good or target record; a cap (default 3 distinct) alerts rather than collects |

## Other findings

| ID | Finding (panelists) | Disposition |
| --- | --- | --- |
| R22 | "The hibernate fence is the only admission gate" is wrong: the self-development apply hold, the pre-boot gate and `RUNTIME_MAINTENANCE_HOLD` exist (claude, deepseek) | **Adopt**; verified `handler.go:61-119`, `runtime.go:810-818`. §1.4 rewritten; LU1 merges into one gate |
| R23 | "The paper is never idle" is overstated; it accepted markers at 05:48, 06:03, 06:36 (deepseek) | **Adopt.** Now a hypothesis for the period after intake.labels went on (06:50), tested by the quiet-gap sample |
| R24 | Executing activations can last up to 60 min, beyond D = 15 min (own check while adjudicating) | **Adopt**; `provideriface/config.go:24` (`DefaultActivationBudget = 60 * time.Minute`). D stays a default the sample calibrates |
| R25 | The paper acceptance must use a same-epoch build until LU2 lands (luna, muse) | **Adopt** |
| R26 | F6's guest verdict already exists on `/api/runtime/obligations` (glm) | **Adopt**: the host reads that; no new `/health` block |
| R27 | Owner activity cannot cancel a drain through the fenced actor path (luna) | **Adopt.** Cancellation happens at the API ingress for owner messages, before the actor path |
| R28 | A guest runtime restart mid-drain loses in-memory drain state (luna) | **Adopt.** That restart is a crash boot anyway; the host treats an unknown drain as aborted and records it |
| R29 | Booted-image identity must be written on every launch path, or a computer is refreshed every sweep (deepseek) | **Adopt.** Recorded at `launchFirecracker`, the single actuator in the restart design |
| R30 | A refused build needs a rule for re-cuts and for who may un-halt (deepseek) | **Adopt.** Refusal is keyed by image digest (any new commit is a new digest); un-halt and un-refuse are owner acts (new O9) |
| R31 | Interim attempt fields lack fsync (deepseek) | **Adopt as a named residual**; the restart design already moves them to L1b's ledger |
| R32 | Probation pauses the owner's messages for up to T_ready (muse) | **Adopt** as O3b |
| R33 | Owner-list hygiene: O4 is a constraint, O6 follows from restart semantics, Q is operational (muse, luna, claude) | **Adopt.** O4 becomes invariant U7; O6 moves to the dependency list; O7 keeps only the lag target |
| R34 | O8 (CLAUDE.md wording) is not LU1's to do (deepseek, glm) | **Adopt.** Deferred; LU1 satisfies the current sentence literally |
| R35 | The LU1 guest half must state how it reconciles with L1c (deepseek) | **Adopt.** After L1c the host's attestation replaces the marker; the drain and final call stay |
| R36 | Throughline: q9 names surfaces that do not exist yet; q7/q8 overstate current support; unknown worktrees without a reconciliation move; registry hygiene by precedent; realism axis wording (luna, deepseek) | **Adopt** all: q7-q9 rewritten as obligations with today's read paths; worktrees named as an accepted edge of this agent's isolation (touch forbidden); `docs/doc-authority-manifest.yaml` gains LU; the realism axis names the one path |
| R37 | Engineering may cut LU4 without the owner (gemini) | **Not adopted.** The owner named lighter restarts in the agreed direction, so cutting it is a scope change for the owner |
| R38 | The S3 note's snapshot invalidation is its failure 8, not 7 (gemini) | **Rejected.** The S3 note's item 7 ("Disk changed after capture") ends "A deploy that rewrites those files therefore invalidates every snapshot"; the citation stands |
| R39 | VM refresh took about 38 s, not 33 s (muse) | **Adopt**; kill 05:48:10, refreshed 05:48:48 |

## What changes in round 2

The design note and the station file carry a round-2 changelog naming
these IDs. The slice order is now LU1a (host-only retry) and LU2
(persisted epoch and migrations) first, then LU1b (drain with deferral,
lineage admission and a final call), LU3a (rings), LU3b (probation and
rollback), LU5, and LU4 only if measured.

# Round 2

Reviewed: the round-2 design note and station file at cca944f7 (landed on
`main` as 0a0ab4c6 and cca944f7). Same runner, mode and members. Raw
outputs: the session scratchpad, `panel-lu/round2/`.

## Verdicts

| Panelist | Verdict | Confidence |
| --- | --- | --- |
| claude (opus) | accept with repairs | medium |
| gemini-3.8-flash | accept with repairs | high |
| glm-5.3-flash | accept with repairs | medium |
| deepseek-v4.1-flash | accept with repairs | high on the repairs, medium-high overall |
| muse-spark | send_back (scoped: LU1b's final call and lineage predicate; LU1a and LU2 sound) | medium-high |
| gpt-6-luna | send_back | high |

All six found every round-1 blocker repaired in direction. Each round-2
blocker below is a reproducible, code-cited gap, so the minority
send_backs stand over the majority (throughline: "a reproducible minority
blocker outweighs an unsupported majority pass"). **Outcome: send_back
for round 2.** Every blocking finding is adopted and written into round 3.
The station stays `drafted` until a round-3 panel accepts it. All six said
LU1a needs no owner act.

## Findings

| ID | Finding (panelists) | Verified | Disposition |
| --- | --- | --- | --- |
| S1 | **The final call strands a computer if vmctl dies between the guest's 200 and the kill**: runtime down, marker written, and the sweep reads an unreachable guest as busy and defers forever (claude, deepseek, muse; gemini, luna) | `ownership.go:3100-3106` (transport error = busy); `deploy_refresh.go:292-293`; host deploys restart vmctl | **Adopt.** vmctl writes a durable final-call record before the call; the sweep completes kill and boot under the claim when the record exists, the runtime has not answered for 60 s and the record is younger than the marker's 15 min; otherwise it clears the record. New acceptance line |
| S2 | **The updater's restart bridge can start the runtime again after it exits** (claude, luna, muse) | `autoputer-vm.nix:483-493` runs `systemctl restart go-choir-autoputer.service` from a request file | **Adopt.** The final call refuses while an updater operation is in flight or a restart request file exists; once `update_final` is set the runtime asks for no restart. Residual: a request appearing in the last seconds |
| S3 | **No finalization barrier**: `Shutdown` waits for active handlers, the run path does not stop the adapter, and exiting before the 200 is flushed reads to vmctl as a failed call (luna, gemini) | `internal/server/server.go:274-286`; `internal/actorruntime/adapter.go:628-635` | **Adopt.** Order: stop the dispatcher and adapter, close the stores, write the marker, `sync`, send and flush the 200, then shut the server down asynchronously and exit 0 |
| S4 | **"Filesystems synced" overclaims**: signers, the updater and timers can write after the sync (claude, muse) | `autoputer-vm.nix:596,635,714` | **Adopt.** U2 narrowed: runtime stores closed, marker durable, a system-wide sync after the marker; other units' later writes are journal-recovered (named residual) |
| S5 | **Abort leaves `update_final` set**; no deadline and no cancelled state (deepseek, muse) | design text | **Adopt.** The guest clears `update_final` on an error return, on the host's abort, and at a 60 s deadline; poll states gain `cancelled` |
| S6 | **`update_drain` must set while work is inside**; only `hibernate` keeps refuse-while-inside (deepseek) | `hibernate.go:107-119` | **Adopt** |
| S7 | **Lineage admission is not implementable as written**: an update carries no root run; a child's trajectory is its parent's run id; `selfdev_materialization_retry` is outside `RestartWorkKinds` and can start an apply mid-drain; a scan of passivated runs can exhaust the guest (all six; luna and muse in detail) | `actor.go:48-60`; `runtime.go:4346-4363`; `handler.go:244-251`; `planned_restart.go:161-173`; `runtime.go:2515-2520` | **Adopt.** Set `update_drain` first, then snapshot the run ids of executing activations (in memory) and their ancestors by a bounded parent walk. Admit an occurrence when its run (from `Content` for `initial_dispatch`, the resume state for `coagent_result`, `TrajectoryID` only as a store-verified hint) or a bounded ancestor is in the set; admitted spawns join the set. Admit cancels and deadline closures. Admit `selfdev_materialization_retry` only for an operation already mid-transaction at drain start; defer it otherwise. Resolution errors defer (fail closed). Never `ListAllRunsByState`. Acceptance adds a grandchild and a cross-trajectory child |
| S8 | **Head-of-line**: a deferral `break`s the mailbox fold (claude); luna reads later folds as skipping not-due items | `dispatcher.go:333-337,394-433` | **Adopt as hardening.** Both readings hold: the block lasts until the head's next backoff. Update-hold deferrals `continue` the fold; a test queues a child result behind held work in one mailbox |
| S9 | **The deferral budget is cumulative**: `defer_count` is durable across boots and stacks across the drain, the final window, probation and the apply hold, reaching 64 (all six) | `log_sqlite.go:143-172`; `handler.go:61` | **Adopt.** Update holds (`update_drain`, `update_final`, `probation`) defer in a separate class that does not count toward `MaxDeferrals`; their live-lock bound is the hold's own deadline. The test sums the worst-case stack of the remaining counted holds |
| S10 | **LU2's epoch would be stamped before the migration runs**, so a crash mid-migration skips it next boot (claude, glm, deepseek, luna, muse) | `store.go:678-682` writes at every open | **Adopt.** The work-state epoch is read at open and never written there; it advances inside each step's transaction after the step; steps are idempotent; a boot with persisted < binary holds admission until they match |
| S11 | **First boot with no persisted epoch** (claude) | design gap | **Adopt.** Seed from the consumed marker's epoch if there is one, otherwise the binary's; the release that ships LU2 does not itself bump the epoch |
| S12 | **Migrations do not reach the actor log**, a separate SQLite database whose old-schema occurrences a planned boot would deliver (claude) | `adapter.go:100-115`; `handler.go:110` | **Adopt.** A step also closes or rewrites actor-log occurrences of changed kinds, idempotently, before the dispatcher starts; "one transaction" narrowed to "one per database, actor log first" |
| S13 | **A downgrade must fail closed**, not run as a crash boot on newer state; a revert after a migration ran is unsafe (luna, muse) | `planned_restart.go:133-141` only classifies | **Adopt.** A persisted epoch newer than the binary holds the runtime visibly for an owner act; reverting LU2 or later after a migration ran is forbidden (slice rollback says so) |
| S14 | **LU2 must not weaken the crash-closure barrier** (luna) | `runtime.go:2417-2469` best-effort today | **Adopt** as an acceptance line: a failed migration holds the runtime |
| S15 | **A moving target bypasses rings**: a drain started for A applies B before B passed earlier rings (luna) | design LU1b and LU3a | **Adopt.** Once rings exist, the final call applies only an image admitted to the computer's ring; otherwise it aborts and the computer re-gates |
| S16 | **LU1a should apply vmctl's idle test** (with desk mutations) before calling today's endpoint (claude) | `ownership.go:3108-3125` | **Adopt** |
| S17 | **LU1a's interfaces are undefined**: the kick route, where the target lives, how image identity reaches the ownership (`launchFirecracker` has no such parameter; boot and resume both call it), persisted `pending_since`, the canonical image identity (muse; deepseek on served commit) | `manager.go:866,1093,1888` | **Adopt.** `POST /internal/vmctl/update-sweep`; target = the resolved store path of `/var/lib/go-choir/guest`, the canonical identity (U6); identity passed through `bootVM` and `ResumeVM` to the ownership; `pending_since` and target-first-seen persisted; Q measured from first-seen and restarted on a target move |
| S18 | **Forced refresh vs drains** (muse) | design | **Adopt.** An owner's forced refresh may interrupt a drain (a crash boot) but never the window between the final call's 200 and the kill |
| S19 | **"Stuck" is not "failed"** for ring gating (muse) | design | **Adopt** |
| S20 | **The final call and kill-wait under `sweepMu` stall other passes** (claude, glm, muse) | `ownership.go:961-964` | **Adopt.** They run under the per-computer claim, off `sweepMu` |
| S21 | **The graph and spine make LU depend on L1**, contradicting LU1a and LU2 first (luna, deepseek) | `mission-graph.yaml` LU node; spine | **Adopt.** `depends_on: []`; LU3b's need for L1b and L1c stays in the station's dependencies |
| S22 | The spine still says "LU's O1-O8" (deepseek) | spine | **Adopt** |
| S23 | The now card bundled two moves (claude, luna) | station | **Adopt** |
| S24 | A refusal needs its own record on the computer's chain (glm) | design LU3a | **Adopt** |
| S25 | Owner activity racing `update_final` (luna) | design | **Adopt.** Once `update_final` is set, owner messages wait for the planned boot |
| S26 | A repeated drain request after a vmctl restart (luna) | design | **Adopt.** It reuses the active drain and its deadline; it never extends it |
| S27 | The skip-unknown-lifecycle-action change should ride the first guest image (claude) | `lifecycle_receipts.go:28-33` | **Adopt.** It ships with LU2 |
| S28 | LU2 lands through LU1a's sweep: parallel in development, not in landing (deepseek) | — | **Adopt** |
| S29 | The lag report should include the owner's computer (claude); O2 needs an owner opt-out (luna) | — | **Adopt** in O7 and O2 |
| S30 | Nix store exhaustion before the 3-image alert (gemini) | — | **Adopt.** The alert also fires on a disk floor |
| S31 | LU1a cannot start because the graph blocks LU on L1 (luna) | — | **Resolved by S21**, not by waiting for L1 |

## What changes in round 3

The design note and the station file carry a round-3 changelog naming
these IDs. LU1b's final call becomes a recoverable protocol (S1-S6, S25,
S26); its admission becomes a defined predicate (S7, S8) with its own
deferral class (S9). LU2 orders its epoch write and covers the actor log
and downgrades (S10-S14). LU1a's interfaces are named (S16, S17, S20).
Rings pin the image (S15, S19, S24). Next: a round-3 panel on the amended
files.
