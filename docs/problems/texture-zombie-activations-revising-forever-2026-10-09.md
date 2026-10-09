# Texture documents stuck in "Revising…": 68 runs re-passivated on every boot

Date: 2026-10-09. Owner report (2026-10-08): Texture is slow to load
recent documents; once a document loads, the revision chevrons are slow; a
document can load into "Revising…"; in that state Cancel is not instant.
Operational invariant O1 (station SL: every durable obligation has one
driver and a recorded terminal fate). Mutation class of a fix: red
(Texture lifecycle, run state).

## Evidence (owner computer `computer-03335285…`, read-only, 2026-10-09 ~02:50Z)

Timed from Node B against the owner guest, the way the proxy calls it.
Response bodies were discarded; only status, size, IDs and counts were
kept.

- **Reads are fast when the host is quiet:**
  - `GET /api/texture/documents` (50 docs): 1.20 s cold, 0.15 s warm;
  - one revision (each chevron click): 15–25 ms;
  - revision list: 24 ms;
  - trajectory snapshot: 0.37 s.

  The reported slowness is load-dependent; see "Load" below.
- **"Revising…" on load.** The newest document `3b12d89a…` reports
  `agent_revision_pending: true`, but the list reports 0 pending documents.
  Its trajectory `4f0311fd…` has `activation.state = pending` for run
  `36f5e4e9…` (lifecycle version 10, last updated 2026-10-08 20:02:28,
  when that run authored a Texture turn).
- **The run cycles and never terminates.**
  - The owner console logs `passivated run 36f5e4e9… (was pending) after
    restart` on 7 boots: 23:08, 00:10, 00:36, 01:02, 01:33, 02:18, 02:34.
  - Every boot passivates the same 68 runs (`boot passivation owner-scoped
    candidates=68`; 65 on 2026-10-08).
  - A boot soon after another finds 0 candidates (00:10:46, 01:03:07), so
    passivation does commit. Something later returns the runs to
    `pending`.
  - The dispatcher logs `deferred … texture:… deferrals=35 cause=actor:
    defer unprocessed occurrence: actorruntime: pending Texture occurrence
    produced no exact run` throughout.
- **Passivation does not touch the Texture lifecycle.**
  `passivateInterruptedActivations` (`internal/agentcore/runtime.go:2336`)
  sets the run to `passivated` and marks the agent mutation stale. It
  appends no lifecycle event, so the document's activation stays `pending`
  and the editor shows "Revising…" (`TextureEditor.svelte:518,542`).
- **Cancel is three sequential round trips** (`frontend/src/lib/texture.js:429`):
  get document, trajectory snapshot (0.37 s quiet), then the cancel POST.
  So Cancel can never feel instant. On a pending activation with no live
  run, it may wait on the same stuck machinery (not yet measured).

## Load (hypothesis, to measure)

The quiet-host reads above are fast; the owner saw slowness yesterday.

- Node B at 02:46: one VM (`candidate-fleet-e15cb89f…`, the owner) holds
  14.7 GB RSS of 31 GB; swap 12 MB free; load average 11 at 02:42.
- The owner guest store is 11 GiB live, and `dolt gc` is skipped every
  boot ("exceeds safe bounded-guest GC size (5 GiB)").
- Every boot also spends ~13 s passivating the 68 zombie runs and ~9 s
  reconciling terminal outcomes.

Slowness under load is a hypothesis until it is measured during an
active run on a large-history computer.

## Same session: signup computer failed on a credential timeout

QA account `qa-owner-20261009@example.com` (persistent, virtual passkey):
at 02:42:32 its first boot failed. vmctl logged `computer credential
request failed … /internal/computers/credentials/issue: context deadline
exceeded` → `realization credential unavailable` → state `failed`. A
retry 4 minutes later booted in 17 s. A brand-new user saw a failed
computer during host load.

## Next

1. Find what returns passivated runs to `pending` (trace one run across
   boots: run row updates, dispatcher deferrals, rewarm).
2. Every passivated or abandoned Texture activation needs a terminal
   lifecycle event, so the document leaves "Revising…".
3. Make Cancel one request, and make it work when no live run exists.
4. Measure Texture reads under load on a large-history computer.
5. Credential issuance under load: a refusal or retry, never a `failed`
   computer.

## Owner-approved disposal, and why Cancel cannot finish (2026-10-09 ~06:30Z)

The owner approved cancelling/disposing the stuck activations. Everything
below went through the product cancel path (`GET` document, `GET`
trajectory snapshot, `POST /api/trajectories/{id}/cancel`). Only states,
counts and timings were read.

- **`3b12d89a` (newest, "Revising…"):** run `36f5e4e9` pending since
  10-08 20:02. Cancel returned 200 `cancelled`; the document now reports
  no pending revision.
- **`d50af120` and `bda20d6e` (September documents):**
  - These are the four Texture occurrences deferred 43–49 times
    ("pending Texture occurrence produced no exact run").
  - Their activations are `passivated`, with lifecycle versions ~1,900
    and 217 work items.
  - **Trajectory snapshot: 9–18 s each.** That is the second of the
    three Cancel round trips.
  - **Cancel is refused:** HTTP 400 in 12–30 s with
    `prepare trajectory assignment fate: co-super assignment invalid transition`.

**Cause (verified in source and data):**
- Each trajectory holds about 90 co-super assignments with:
  `disposition=cancelled`, `capsule_disposition=unbound`, and no bound
  run. These were cancelled before ever binding a capsule.
- `prepareEngineeringTrajectoryCancellation`
  (`internal/agentcore/engineering_assignment_fate.go:252`) requests a
  capsule revoke for **every** non-revoked assignment.
- `SetEngineeringCapsuleDisposition` refuses any transition on an
  unbound, non-open assignment (`internal/store/engineering_assignments.go:2397`).

So one never-bound assignment makes owner Cancel impossible for the
whole trajectory. The occurrences then have no terminal fate and retry
forever (O1).

**A terminal `unbound` assignment never had an executor.** Capsule
disposition never returns to `unbound`, and `Bound` requires a
non-`unbound` capsule (`internal/types/engineering_assignment.go:323`).
So skipping it loses no executor fate.

**Residue left by the failed attempts:** a durable cancellation intent
on each trajectory. A retry with the same command identity resumes it.

## Resolved on the owner computer (2026-10-09 08:10–08:15 UTC)

**Fixes, deployed to staging:**
- 3083c6dd: cancel skips never-bound terminal assignments.
- cd7e8cfe: the Cancel UI reads `?view=summary`.
- 8fd243ba: boot refuses a stale layer.

The owner computer then ran 8fd243ba itself; before that it had stayed on
the 39c0d991 layer. See
[`guest-runtime-fix-never-reaches-layered-computers`](guest-runtime-fix-never-reaches-layered-computers-2026-10-09.md).

| Measure | Before | After |
|---|---|---|
| `GET /api/trajectories/{id}?view=summary` | full snapshot: 9–18 s | 2–360 ms |
| Cancel `d50af120` (resumed original intent, V=1901) | 400 in 12–30 s | **200 `cancelled`**, 1 run cancelled, 32.8 s |
| Cancel `bda20d6e` (V=1987) | 400 | **200 `cancelled`**, 2 runs cancelled, 40.8 s |
| `3b12d89a` | "Revising…" | `cancelled` (v13) |
| Management "reconcile lifecycle cancellation" deferrals | every 25–50 s | none since the 8fd243ba boot |
| `GET /api/texture/documents` | timed out (>120 s) at 4 GiB | 0.14–0.36 s warm at 8 GiB; 2.7 s first call after boot |

**Residuals:**
- **Cancel latency is still 33–41 s on trajectories with ~200
  assignments.** Prepare/finish write per assignment under the store
  mutex. Bound or batch them.
- **Owner-visible failure:** a failed cancel leaves a durable intent. A
  retry from the UI mints a new command id (from version and head) and
  conflicts with that intent. The UI should resume the stored intent.
- **Zombie cycle:** the original 68-run re-passivation cycle has not
  recurred since the 05:38 boot (0 candidates). Its trigger is still
  untraced; next steps 1, 2 and 5 above remain open.

## Correction: the zombie cycle never stopped; boot replay is the trigger (2026-10-09 09:45Z)

The residual above ("has not recurred since the 05:38 boot") was wrong.
The owner console shows the cycle on every boot that starts after the
previous boot's replay finished:

| Boot (server start) | Passivation candidates | Previous replay complete |
|---|---|---|
| 05:37:52 | 0 | 05:33:54, but that boot was the compaction window's `maintenance-serve` under hold, where Texture reconcile is deferred (likely; not traced) |
| 06:06:49 | 68 | 05:48:18 |
| 06:44:04 | 67 | 06:16:19 |
| 07:15:02 | 67 | 07:09:17 |
| 07:42:27 | 67 | 07:24:03 |
| 08:10:07 | 67 | 07:51:23 |
| 08:49:28 | 67 | 08:20:55 |
| 08:56:25 | **0** | 08:49 boot's replay had not finished |
| 09:33:33 | 67 | 09:05:19 |

**Mechanism (trace, owner computer on c01bc2f9; states and metadata
flags only):**
1. Boot passivates the 67 Texture runs (`passivated_reason =
   runtime_restarted`), 11.3 s.
2. The replay phase (`runReplayPhase` → `actorruntime.Adapter.Start` →
   `textureowner.Handler.Start`) reactivates them. Three sampled runs went
   back to `pending` at 09:36:50, 09:37:05 and 09:38:16; "replay complete"
   was logged at 09:38:16. Their metadata reads `request_source =
   update_coagent`, `request_intent = integrate_execution_findings`,
   `actor_reactivated_from_passivated = true`, and `passivated_reason`
   still `runtime_restarted`.
3. The reactivated runs never execute: the dispatcher logs
   `pending Texture occurrence produced no exact run` and defers.
4. The next boot passivates them again.

A boot that starts before the previous replay finished finds the runs
still `passivated` and reports 0, which is why 05:38 and 08:56 looked
clean.

The reactivation writes are silent: no log line or lifecycle event
records them, so the cycle was visible only by comparing boots.

**Replay duration with the snapshot fixes:** 9.0–10.8 min per boot before;
**4.7 min** on c01bc2f9 (09:33:33 → 09:38:16).

Next: find why a run reactivated by boot replay never produces the
"exact run" the deferred occurrence waits for, and give each of these
runs a terminal fate (O1) instead of re-arming it every boot.

## Root cause: boot re-arms open-work runs with no executor (d1d875a0) (2026-10-09 10:05Z)

**Trace (code plus owner computer states):**
- `textureowner.Handler.Start` ends by calling `ReconcileActorWake` for
  every activation-eligible Texture subject, and discards the result.
- `reconcileAgentWakeLocked` arms `initialWorkWake` whenever the
  trajectory has an open Texture work item and no **active** run.
  d1d875a0 (2026-09-30, "runtime_restarted texture run suppressed its own
  re-wake") narrowed the suppression from "any run" to "an active run".
- With only that arm, `reactivatePassivatedTextureRun` sets the passivated
  run to `pending` and its mutation back to pending, then returns. Its own
  comment: "the current coagent_result occurrence is the execution
  authority". At boot there is no occurrence: boot dispatches occurrences
  only for producer reports and owner-input heads. **Nothing executes the
  re-armed run.**
- The next boot passivates it (`runtime_restarted`), and the cycle repeats.

**Owner-visible effect:** the re-armed mutation makes
`agent_revision_pending: true` (`internal/textureowner/texture.go:1049`).
So **every** affected document opens into "Revising…" and stays there.
This is the owner's 2026-10-08 report "a document can load into
Revising…". Two sampled zombie documents (last edited 09-30 and 10-02)
return `agent_revision_pending: true` now.

**Scale on the owner computer (read-only dry run, 09:58Z):**
- 67 runs, on 67 distinct live trajectories, all `pending`;
- 28 at lifecycle version 2–3 (initial work, no committed turn);
- 39 with history, up to version 775;
- runs created 2026-08-20 … 10-08; **66 of 67 on or after 09-30**, the
  day d1d875a0 landed.

**Why tests did not catch it:** both regression tests from d1d875a0
(`TestTextureOwnerStartReactivates{RuntimeRestartedPassivatedRun,PassivatedRunOnOpenWork}`)
assert only `state == pending` after `Start`. Neither checks that the run
ever executes. That violates the artifact-verified-success standing
question: "re-armed" was taken as "recovered".

**Clustering note (3+ Texture activation bugs this week):** never-bound
assignments blocking cancel; the cancellation-intent retry loop; this.
All three are obligations with no driver or no terminal fate (O1). This
one is substrate-level: a wake that changes state without an executor.

**Fix direction (decision recorded; No Blocking Asks):**
- **Chosen (conservative):** boot does not re-arm a run on the open-work
  arm alone. A re-arm requires an occurrence that will execute it:
  - a producer report;
  - an owner-input head;
  - an occurrence-driven wake.
  The run stays `passivated` (honest: not running, not "Revising…"). The
  next real occurrence re-arms it through the occurrence path, which does
  execute.
- **Not chosen:** dispatch a recovery occurrence at boot so the run
  executes. On the owner computer that would start 67 LLM turns at the
  next boot, many on weeks-old obligations, and write agent revisions
  into owner documents without a fresh owner request.
- **Residual (owner decision):** should an interrupted Texture turn
  resume automatically after a restart, and within what age? Until
  decided, it resumes on the next occurrence for that document.
- **Disposal of the 67:** the owner approved disposing stuck activations.
  Cancelling the 28 initial-only trajectories was attempted and blocked
  by the session permission classifier. With the fix, the next boot
  leaves all 67 passivated and the "Revising…" state clears, so no
  cancel is needed for the owner-visible symptom.
