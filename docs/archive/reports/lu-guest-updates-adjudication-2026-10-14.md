# LU: Guest Updates, Panel Adjudication (round 1)

Date: 2026-10-14. Reviewed: the design note
([`lu-guest-updates-design-2026-10-14.md`](../../reports/lu-guest-updates-design-2026-10-14.md))
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
| S29 | The lag report should include the owner's computer (claude); O2 needs an owner opt-out (luna) | — | **Adopt** in O7; the opt-out is **posed** in O2 as an open owner question with the default "no opt-out" (corrected in round 3, T22: not an adoption of the opt-out) |
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

# Round 3

Reviewed: the round-3 design note and station file at 7ccc5ded. Same
runner, mode and members. Raw outputs: the session scratchpad,
`panel-lu/round3/`. The prompt allowed a finding that only matters for a
later slice to be recorded as an implementation-gate obligation instead of
a send_back, when it can be named precisely now.

## Verdicts

| Panelist | Verdict | Confidence |
| --- | --- | --- |
| claude (opus) | accept with repairs (gates G1-G6 for later slices; R1, R2 for LU1a) | medium-high |
| gemini-3.8-flash | accept with repairs | high |
| glm-5.3-flash | accept with repairs (R-a, R-b gate LU1b; R-c LU2; R-d LU1a) | medium |
| deepseek-v4.1-flash | send_back (N1 in LU2, N2 in LU1b) | high on code, medium-high on verdict |
| gpt-6-luna | send_back | high |
| muse-spark | timed out (1200 s), no output | — |

All five who answered found the round-2 blockers repaired in direction.
**Outcome: send_back for round 3.** Two findings are mechanism defects in
text that must ship correct, both verified: LU2's first-boot seeding can
skip migrations (T1; LU2 is a first build slice, so it cannot be deferred
to an implementation gate), and the final call's record and abort rules
can still strand a computer (T2; three of the accepting panelists named
the same hole as their own LU1b gate). Every finding below is adopted or
answered and written into round 4. The station stays `drafted`.

## Findings

| ID | Finding (panelists) | Verified | Disposition |
| --- | --- | --- | --- |
| T1 | **LU2 seeding skips migrations**: an absent epoch with no marker seeds from the binary, so a computer that never ran an LU2-era binary (a hibernated computer's markerless cold wake, or one deferred across the LU2 and bump releases) takes the bump image, seeds the new epoch, and runs no migration (deepseek, luna; claude as a residual) | The epoch exists only in markers today (`planned_restart.go:35,64,133-136`); a deploy invalidates every snapshot (S3 note, failure 7) | **Adopt.** No persisted epoch and a marker: seed from the marker. No persisted epoch and no marker: seed from 1, the first tracked epoch (Cut 1), so every registered step runs. A `crash` step on a markerless boot is satisfied by the crash closure that boot runs anyway (today's D7 behavior); the epoch advances past it after closure. LU2's acceptance adds a store that never booted an LU2-era binary, booting markerless onto a fixture bump |
| T2 | **The final call can still strand a computer**: the host clears its record on a call error (a lost 200 or a 20 s client timeout against a longer guest barrier), and clears a record older than 15 min while the guest is unreachable; the guest's abort after it has stopped the dispatcher and closed stores cannot be built (deepseek, luna, glm R-a, gemini, claude G1) | `deploy_refresh.go:72` (20 s client), `:87-101` (any transport error is "unavailable"); `ownership.go:3100-3106`; `planned_restart.go:26` | **Adopt.** The marker's durable write is the guest's commit point: refusals and re-checks run first and may abort; after the marker, no abort; a later failure exits non-zero (a crash boot, recorded). The host resolves its record only by reachability, never by the call's result: guest answering with no update in progress, clear; unreachable for 60 s, kill and boot whatever the record's age (a fresh marker boots planned, a stale one crash-boots and alerts); never clear while unreachable. Each final call carries an idempotency id the guest reports on `GET update-drain`. Timeouts ordered: guest barrier budget < host call timeout < host stop-wait |
| T3 | **The runtime cannot see the updater's request files**: `/run/choir-updater-control` is inaccessible to the runtime unit; there are two bridges, `restart` and `recover` (glm R-b, deepseek N3, claude) | `autoputer-vm.nix:483-493,497-511,851` | **Adopt.** The final call refuses while any updater operation journal is non-terminal (`/mnt/persistent/choir-updater/operations/*.json`, readable to the runtime, `autoputer-vm.nix:850`); the runtime's restart-handoff endpoint, which the updater calls before a restart (`autoputer-vm.nix:713`), refuses while `update_final` is set. That covers both bridges |
| T4 | **Direct HTTP mutators bypass the actor gate**: the source schedule routes write the store outside actor admission, and other handlers could hit closed stores (luna, claude) | `source_schedule.go:73-145,444-487` | **Adopt.** The barrier first closes mutating API admission (those routes answer 503 with Retry-After) and waits for in-flight mutators, then stops the dispatcher and closes stores |
| T5 | **The pinned image is not bound to the boot**: launches read the stable symlink paths, so a host activation between pinning and launch boots a different image than the record says (luna, claude R2) | `node-b.nix:725-729,1006-1013`; refresh rebuilds from config, `manager.go:1436-1459`; overrides exist, `:1462-1500` | **Adopt in LU1a.** Each launch resolves the image root once and passes every artifact path from it through the per-boot overrides; the recorded identity is the one booted. A launch that cannot resolve an identity boots from the stable paths, records `unknown` and alerts, rather than failing the boot (claude, glm) |
| T6 | **LU1a's desk-mutation check is a time-of-check race**: vmctl's idle read and the guest's marker write are separate (luna) | `ownership.go:3088-3125`; `planned_restart.go:321-336,395-415` | **Adopt by narrowing.** LU1a claims only what today's endpoint guarantees (the F4 race remains until LU1b's guest-side final call, which checks desk mutations under the gate); the vmctl idle read is a filter that lowers the odds, not a guarantee. LU1a's acceptance wording changed accordingly |
| T7 | **The per-computer claim has no exclusion set**: reconcile, warm, idle stop, pressure reclaim and the operator refresh can act on a computer mid-refresh once the refresh runs off `sweepMu` (claude R1; glm on persistence) | `ownership.go:965-990` | **Adopt.** Every one of those paths checks the claim and skips a claimed computer. Claims are in memory; the final-call record is the only durable part. Acceptance line added |
| T8 | **The drain snapshot is not atomic with activation completion** (luna) | `runtime.go:406-420,848-855` | **Adopt.** The gate reason and the snapshot are taken under the runtime's running-set lock. A run that finishes first leaves its queued work deferred, which is safe (a timeout, never a cut). Race test added |
| T9 | **The reconcile trigger bypasses the gate**: post-commit observers reach `TriggerSelfDevelopmentReconcile` directly, so an approval committed mid-drain can start an apply; an admitted retry reconciles globally (claude G2, deepseek) | `self_development_materializer.go:45-69`; `handler.go:244-251` | **Adopt.** While an update hold is set, the reconcile trigger starts no new apply (it may advance an operation that was already mid-transaction when the drain began) and re-arms when the hold lifts |
| T10 | **The uncounted deferral class hot-loops**: backoff is computed from `defer_count`, so a class that does not count it stays at 500 ms (claude G3) | `dispatcher.go:18-27,394-406`; `log_sqlite.go:143-172` | **Adopt.** Update-hold deferrals use a fixed 10 s backoff, capped at the hold's deadline; the test asserts no 500 ms churn |
| T11 | **`continue` gives up FIFO in a mailbox**: an admitted cancel can run before a deferred dispatch it cancels (claude G4) | `dispatcher.go:433` | **Adopt.** Handlers re-check run state when a deferred occurrence finally runs (a dispatch for a cancelled run is a no-op); acceptance case added |
| T12 | **Starvation under hourly deploys**: aborting a drain on every target move and restarting Q can keep a busy paper from ever updating (claude G5) | design | **Adopt.** A drain or final call keeps the image it started with while that image is still admitted to the computer's ring; Q is measured from `pending_since`, not from `target_first_seen`. The record's image is GC-rooted and booted through the overrides (T5) |
| T13 | **`restorePrior` checks store schema but not the work-state epoch** (claude G6) | `updater.go:943-994` | **Adopt** as an LU3b/LU5 gate: the updater refuses a restore across a work-state epoch (U7) |
| T14 | **Probation has no guest-side backstop** if vmctl dies (deepseek) | `hibernate.go:39` precedent | **Adopt** as an LU3b gate: probation has a guest TTL; on expiry the guest lifts the hold and reports `probation_expired`, and the host treats the update as unvalidated (no automatic rollback after work ran) |
| T15 | **Mixed fleet during LU1b's rollout**: guests without the drain endpoint answer 404 (deepseek) | — | **Adopt.** A 404 falls back to LU1a's quiet-moment path and never counts toward "stuck" |
| T16 | **Two-database migration crash** after the actor-log step commits (luna, gemini) | `adapter.go:100-115` | **Adopt.** Acceptance injects a failure after the actor-log step; the re-run is idempotent |
| T17 | **Where the epoch lives**: the store-schema epoch is a workspace file, which cannot join a Dolt transaction (glm R-c) | `store.go:678-682` | **Adopt.** The work-state epoch is a row in the Dolt store, advanced in the step's Dolt transaction |
| T18 | **The kick and the deploy receipt**: whether the sweep route is synchronous, and what the deploy receipt says for pending computers (glm R-d) | `ci.yml:1520-1545`; `docs/archive/problems/deploy-refresh-all-deferred-fails-deploy-2026-10-13.md` | **Adopt.** The route is a kick (202); pending is not a deploy failure; q3 names `.github/scripts/deploy-workflow-contract-test` and the receipt mapping |
| T19 | **Refused or stuck computers would never hibernate** under the F27 rule (glm) | design | **Adopt.** F27 applies only while an attempt is in progress; an idle refused or stuck computer hibernates normally |
| T20 | **Ring 0 canary minting can fail** (gemini) | design | **Adopt.** A failed mint halts ring 0 and alerts; it never advances |
| T21 | **A target collected by Nix GC before vmctl roots it** (gemini) | `node-b.nix:1006-1013` | **Rejected.** The guest image is part of the active system closure (the activation script interpolates `${guestImage}`), so the system profile roots the target before the symlink moves |
| T22 | **O2's opt-out vs S29** (luna, deepseek, glm) | adjudication S29 | **Adopt.** S29's wording corrected above: the opt-out is posed, not adopted |
| T23 | **Missing owner decision: may the sweep wake a hibernated pending computer to update it?** (deepseek) | — | **Adopt** as O10, default no (it updates at its next wake) |
| T24 | **The paper waiting behind a drain while the owner posts** (claude) | — | **Answered** under O3: on the platform tier the owner's messages are deferred up to D; no new decision |
| T25 | **LU1b accepted against a state table** (deepseek) | — | **Adopt** as an LU1b gate: its acceptance enumerates every final-call state against each failure edge |
| T26 | **Small fixes**: the record carries the image identity (claude); `update_final`'s deadline applies only up to the commit point (glm); the S2 residual should say a phantom restart consumes the marker (glm); `now.source_ref` (claude); a host crash differs from a vmctl restart for record durability (luna, R31) | — | **Adopt** |

## What changes in round 4

LU2's seeding (T1, T16, T17) and the final call's commit point and
record resolution (T2-T4, T8, T9) are rewritten; LU1a binds the booted
image and narrows its claim (T5-T7); the deferral class gets a fixed
backoff (T10, T11); starvation is closed (T12); later-slice gates T13,
T14, T25 are named. Next: a round-4 panel on the amended files.

# Round 4 (per slice)

Reviewed: the round-4 files at b158b954. Same runner, mode and members,
timeout raised to 1800 s. This round asked for a verdict per slice as well
as overall, so that accepted slices can proceed while the rest iterate.
Raw outputs: the session scratchpad, `panel-lu/round4/`. muse-spark timed
out again (1800 s, no output); five answered.

## Per-slice verdicts

| Slice | claude | gemini | glm | deepseek | luna | Adjudicated |
| --- | --- | --- | --- | --- | --- | --- |
| LU1a | accept w/ repairs | accept | accept | accept | accept w/ repairs | **reviewed** (repairs V1-V6 adopted) |
| LU2 | accept w/ repairs | accept | accept | accept w/ repairs | send_back | **reviewed** (repairs V7-V10 adopted, V7 is luna's blocker) |
| LU1b | send_back | send_back | send_back | send_back | send_back | **drafted**; structural assessment instead of a fifth patch |
| LU3a | accept w/ repairs | accept | accept w/ repairs | accept | accept w/ repairs | **reviewed** (repairs V11-V15 adopted) |
| LU3b | accept w/ repairs | accept | accept | accept | accept w/ repairs | **reviewed**, gated on L1c, L1b and LU1b's gate (V16, V17) |
| LU4 | accept | accept | accept | accept | accept | **reviewed**, conditional on O5 |
| LU5 | send_back | accept | accept | accept w/ repairs | accept | **drafted**: needs its own design round (V18) |

Overall (station structure and invariants): claude accept the structure,
gemini accept with repairs, glm accept, deepseek accept scoped to LU1a and
LU2, luna send_back (on LU2 and LU1b). Adjudicated: **reviewed, scoped**.
LU1a, LU2, LU3a, LU3b and LU4 are reviewed; LU1b and LU5 are drafted and
must not be built until a panel accepts them.

## Repairs adopted (all verified)

| ID | Slice | Finding (panelists) | Verified | Repair |
| --- | --- | --- | --- | --- |
| V1 | LU1a | A pinned launch would stick: `bootVM` stores its config as `inst.Config` and `ResumeVM` launches from it, so one resolved-path boot pins every later resume to the old image (claude) | `manager.go:840-846,1086-1093` | Resolved artifact paths are per boot only; the configured paths stay the stable paths. An explicit image root from a caller (a final-call record, a rollback) takes precedence; `ResumeVM` gains the overrides parameter |
| V2 | LU1a | An `unknown` computer would restart every pass (claude) | design | A computer that booted `unknown` from an attempt for target X is not attempted again for X; it alerts once (U4) |
| V3 | LU1a | With the flag at `ring0`, the CI change leaves every other computer with no update path (claude) | design | While `VM_UPDATE_SWEEP` is not `all`, CI keeps its per-computer loop for computers outside the sweep's scope; the kick replaces the loop only at `all` |
| V4 | LU1a | `ring0` needs a definition before LU3a (claude) | `warmness_policy.go:16-19` | Before LU3a, `ring0` means `WarmnessClassTest` computers |
| V5 | LU1a | Retention pruning is missing from the claim's exclusion set (claude) | `ownership.go:983` | Add `PruneRetention` |
| V6 | LU1a | Do not enable `all` before LU1b, because today's endpoint keeps the F4 race (luna) | `planned_restart.go:395-415` | **Not adopted as stated.** LU1a does not add exposures: each pending computer is refreshed once per target, as CI does today; only the timing changes (at a quiet moment instead of at deploy time). `all` stays gated on LU1a's ring-0 acceptance. The design now says so, and says LU1a never claims U2 (T6) |
| V7 | LU2 | **The epoch check must come before `Store.Open` mutates anything**: open runs DDL, drops retired tables, backfills and rewrites the store-schema receipt before returning (luna) | `store.go:627-693,711-770` | A non-mutating epoch preflight reads the persisted epoch before bootstrap; an absent epoch is the seed case; a newer or unreadable epoch holds before any schema write |
| V8 | LU2 | What a `crash` step does is ambiguous, including on a planned boot (claude, deepseek) | design | A `crash` step advances the epoch in its own transaction and makes the boot a crash boot even when a marker was consumed; all steps run, then the crash closure every crash boot runs; `work_state_migrated` names the step; the boot is recorded crash-kind with an alert. U3 gains the qualifier "unless the registry declares that step a `crash` entry" |
| V9 | LU2 | A fresh store needs a rule (claude) | `store.go:684` (`freshStore`) | A fresh store seeds at the binary's epoch |
| V10 | LU2 | The producer of `work_state_epoch` and `store_schema` in the guest image's `build.json` has no owner (claude) | — | LU2's release adds both fields to `build.json` (`flake.nix`); LU1a's identity and LU3b's U7 check read them |
| V11 | LU3a | A ring timeout with no validated member "halts", which contradicts S19 for the paper, a ring of one (claude) | design | A timeout pauses advancement and alerts; it refuses nothing and resumes when a member validates. Only a failure halts |
| V12 | LU3a | Where halt state lives (claude) | — | Derived from refusal records; un-halt is an owner record; both survive a vmctl restart |
| V13 | LU3a | Who runs the health wait once CI no longer refreshes per computer (glm) | — | The sweep runs it after each update boot |
| V14 | LU3a | The owner's un-halt and un-refuse need a no-SSH surface and an acceptance line; O2 must be ratified before the rollout goes past the paper's ring (luna) | — | Acceptance added; ring 2 waits for O2 |
| V15 | LU3a | Small: images declare the `update` action in `build.json`; the rollback line says LU1a, not LU1; the mid-drain target case moves behind LU1b (claude, glm) | — | Adopted |
| V16 | LU3b | Probation with an owner message needs an acceptance case (luna) | — | Adopted |
| V17 | LU3b | LU3b uses LU1b's gate with reasons | design | Named as a dependency |
| V18 | LU5 | **Not specified enough to build**: whether `platform_image` is driven by vmctl (an image refresh) or by the guest updater (an M9a offer, whose only producer a47122de deleted) is undefined, and "one transaction" across a host-driven boot and a guest-driven swap has no driver or commit point; U1 says vmctl is the only driver (claude; deepseek on the offer producer) | `platform_update.go:75,774`; q4 | **LU5 drafted.** It needs its own design round after C1e Freeze and Verify; U1 is scoped to platform image updates. It blocks nothing else |
| V19 | station | `must_preserve` says no automatic path restarts a guest before its final call, which LU1a itself does through today's endpoint (deepseek); stale round labels (gemini, glm) | — | Reworded: "no automatic path forces a busy guest; from LU1b on, no automatic path restarts a guest before the final call's commit"; labels fixed |

## LU1b: structural assessment instead of a fifth patch

All five panelists sent LU1b back, and the final call has drawn new
blocking findings in every round since round 2 (S1-S6, T2-T4, and now
V20-V25 below). CLAUDE.md "Escalate on dead ends" applies: three or more
iterations without convergence means stop patching and assess the
structure.

Round-4 findings, all verified, which any LU1b design must satisfy:

| ID | Finding (panelists) | Verified |
| --- | --- | --- |
| V20 | The abort path still runs after the dispatcher stops and the stores close; nothing reopens a live runtime's stores or restarts its kernel (gemini, glm, luna, deepseek) | `store.go:913-946`; `adapter.go:628-635`; `runtime.go:845-859` |
| V21 | A pending desk mutation blocks every drain: the activation that clears it has no executing ancestor, so lineage admission defers it, and "drained" waits for it (claude) | `runtime.go:2326-2335` |
| V22 | The recover bridge publishes its request with no restart-handoff call, so the handoff refusal covers only one bridge (claude, deepseek) | `internal/updater/runtime.go:37-77` |
| V23 | The journal check misses an apply that is still staging: `Apply` writes its journal after staging and closure materialization; and the check runs before the routes that start an apply are closed (luna, claude) | `updater.go:295-324`; `platform_update.go:774` |
| V24 | A guest that commits and then hangs while still answering has no fate; a failure after the commit point is not a crash boot, because `Restart=on-failure` restarts the runtime in place and it consumes the fresh marker as a planned boot on the old image (deepseek, claude) | `autoputer-vm.nix:842-843`; `planned_restart.go:109-142` |
| V25 | A stale non-terminal journal from a spent apply refuses every later final call (deepseek) | `planned_restart.go:215` |

**The common cause.** Every one of these lives in the same place: two
processes (the guest runtime and the host's vmctl) must agree, across a
network call that can be lost, on the moment a guest may be killed. The
guest decides it is safe; a different process, later, does the killing.
Every round has closed one interleaving of "decide" and "kill" and exposed
another. That is a substrate problem (who ends the guest's life), not a
wording problem.

**A substrate change that removes the class (hypothesis, to verify).**
Let the guest end its own life. After draining and writing its marker,
the guest reboots itself. The guest kernel runs with `reboot=k`
(`manager.go:1642,1674`), so a guest reboot resets the CPU through the
keyboard controller, and Firecracker exits on that reset (to verify
against Firecracker 1.15.1 and vmctl's process reconcile). The host then
needs no final-call record, no kill timing and no reachability rule: it
sees a Firecracker process that exited after the guest accepted an update,
and it boots the target image. The boot consumes the marker as planned. A
guest that never reboots was never committed, and an unreachable one is
handled by the existing wedge and recovery paths and by L1's bounds.
Findings V20, V24 and most of the S1-S5 and T2 machinery disappear; V21,
V22, V23 and V25 remain as guest-side drain rules (admit desk-mutation
activations snapshotted at drain start; an in-process count of updater
calls in flight, closed by the final step, instead of journal scanning;
treat a journal whose writer is gone as settled by the materializer).

**Escalated for a decision** (coordinator or owner): build LU1b on the
guest-initiated reboot (recommended: it removes the failure class), or
patch the host-kill protocol a fifth time. Either way LU1b's next text goes
to a panel before any LU1b code. Nothing else waits on it: LU1a, LU2 and
LU3a proceed now.
