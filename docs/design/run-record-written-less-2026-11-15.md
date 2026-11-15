# The run record written less (T2, first slice)

Date: 2026-11-15. Class of this document: green (design only). The change it
designs is orange to red: `choir.run` is a kernel kind, and the run
acceptance and evidence surfaces read it.

Station: T2 of `docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md`,
first slice, the `choir.run` object. Background:
`docs/design/store-once-2026-11-14.md` (R2). Counters: `a21558f9b`.

Reading rule used here: every claim about code carries the file and line I
read in this worktree. Where I did not read a function, the table says so.
Nothing here was run; no number below is a measurement unless it says
"measured", and the only measured figures are the owner's (22.6% of object
bytes, 4.7 writes per run, about 18.7 KB a write, from the ledger entry C74).

## 0. Decision in one paragraph

Mechanism A: drop the one run write that changes no field but `updated_at`,
the "persist actor initial mailbox metadata" write at
`internal/agentcore/runtime.go:4214-4217`, and nothing else in this slice.
B (move the unchanging fields out into a write-once object) loses because it
changes the stored shape that about twenty decode sites read and has a silent
failure (an empty prompt) and a rollback hazard. C (a patch op) loses because
an old binary cannot replay a tape that holds an op kind it does not know
(`internal/store/project.go:115-116` refuses the batch), which makes every
rollback a replay failure. A's expected saving is smaller than B's best case
and I say so in section 3; its failure surface is the smallest, and it needs
no format change, no replay change and no new tape op. A step 0 (read-only,
on the test computer) must confirm the premise before any code.

## 1. What the 18.7 KB is, and today's writes

### 1.1 The bytes

A tape op for an object is `json.Marshal(obj)` of the whole
`objectgraph.Object` (`internal/store/projection_tape.go:110-119`). The
counter measures `len(op.Body)` of that (`internal/store/tape_write_counters.go:151-153`).
`Object.Body` is a `[]byte` (`internal/objectgraph/object.go:30`), which
`encoding/json` writes as base64: the run body is carried at 4/3 of its size
on the tape. This is from the code, not measured. A second inflation
(not part of this slice, section 7).

The run body is `types.RunRecord` (`internal/types/task.go:89-155`):

| Field group | Fields | Changes after create? |
| --- | --- | --- |
| identity scalars (about 0.7 KB raw, my estimate: six UUIDs with keys, profile, role, three timestamps) | `loop_id`, `agent_id`, `channel_id`, `requested_by_run_id`, `trajectory_id`, `agent_profile`, `agent_role`, `owner_id`, `computer_id`, `created_at` | no |
| `state`, `updated_at`, `finished_at` | | every transition |
| `prompt` | the submitted input | at create; rewritten only by a control-delivery refresh (`internal/store/lifecycle_control_delivery.go:578`) and kept by `preserveLifecycleControlRunAuthority` (`internal/store/lifecycle.go:3270`) |
| `result` | final text | set at the terminal write (`internal/agentcore/runtime.go:4345`); kept by every later write (`lifecycle.go:5545-5547`) |
| `error` | | terminal or passivation writes |
| `metadata` (a `map[string]any`) | llm selection keys, `lifecycle_*` keys, `work_item_ids`, token counts at completion (`runtime.go:4355-4357`), `lifecycle_control_bindings` (a list that only grows, `lifecycle_control_delivery.go:597-638`), reactivation markers | a few keys per transition; most of the map is fixed at create |
| `labels` | provenance label set, union-only | each growth (`internal/store/store.go:2448`) |

Beside the body, every write carries an 11-key metadata object
(`lifecycle.go:3468-3474`, my estimate about 0.55 KB) and the object
envelope (ids, hash, three timestamps; my estimate about 0.45 KB).

So of 18.7 KB, roughly 1.9 KB is identity, envelope and metadata that is
unavoidable per write (estimate: 0.7 KB x 4/3 + 0.55 + 0.45). The other
roughly 16.8 KB, base64, is `prompt` + `result` + `error` + `metadata` map +
`labels`, about 12.6 KB raw. **I cannot split that 12.6 KB from code.** No
file I read states the mean prompt size or the metadata map size. Step 0
measures it.

### 1.2 Every write site of a `choir.run` object

All sites funnel into one of five object writers. "Reads of the state" is
who looks at that state afterwards (section 1.3).

| # | Site | Transition | Fields that change | Writer |
| --- | --- | --- | --- | --- |
| W1 | `internal/agentcore/runtime.go:1563` (`persistLifecycleSubmittedRun`; function body not read), `:1651`, `:1911`, `:1914`; `runtime_persistence.go:39`; `store.go:474` -> `persistLifecycleRun` or `CreateRunOG` (`graph_store.go:575-597`) | create, state pending (the conductor-decision run at `:1634-1651` is created already completed) | all; the whole body is new | `projectLifecycleRun` (`lifecycle.go:3293`, write at `:3528`) or `ogPut` |
| W2 | `runtime.go:3966-3968` -> `persistActivationStateAndEmit` (`:2427`) -> `persistActivationStateWithEvent` (`:2368`) -> `management_controller.go:907` -> `Store.UpdateRunWithEvent` (`store.go:507`, `updateRunWithEvent` `:536`) -> `persistLifecycleRunWithEvent` (`store.go:424`) | pending -> running, with the `loop.started` event in the same batch | `state`, `updated_at` | `projectLifecycleRun` |
| W3 | `runtime.go:4214-4217` | none (state stays running) | `updated_at` only, as far as I could read: I found no assignment to `rec` in the injectors, but did not read all of them (`coagentUpdateTurnInjectorWithDelivered`, `textureFrameInjector`, `calibrationDigestInjector`, `shotImageInjector`) | `projectLifecycleRun` |
| W4 | `Store.UnionRunLabels` (`store.go:2448`, write at `:2501`), called from `agentcore/run_labels.go:145` (`flushRunLabels`, before an act is stamped) and `agentcore/run_memory.go:190` (labels carried from the previous run, at run start) | none | `labels`; the body is re-marshalled whole | `PutBatchConditional` on a body patched in memory |
| W5 | `runtime.go:4344-4383` (tool-loop path), `:4609-4633` (provider path) | running -> completed, with `loop.completed` in the same batch | `state`, `result`, `finished_at`, `updated_at`, three token keys in `metadata` | `projectLifecycleRun` |
| W6 | `runtime.go:2293` `terminalizeRunCanonical` -> `Store.TerminalizeRun` (`lifecycle.go:5520-5575`); callers: cancel (`:2276-2284`), crash-boot close (`:3068-3083`), planned-restart task close (`:3110`) | active -> cancelled / failed | `state`, `error`, `result`, `finished_at`, `updated_at` | `TerminalizeRun` |
| W7 | `runtime.go:2330` `ReactivateRunCanonical` -> `reactivateRunOnce` (`lifecycle.go:5628`, write at `:5690-5725`); callers `actorruntime/handler.go:639`, `:686` | passivated / interrupted -> pending / running | `state`, `error`, `result`, `finished_at`, `updated_at`, a metadata patch | `reactivateRunOnce` |
| W8 | passivation: `runtime.go:3130` (planned restart), `:3923` (provider admission), `:4443`, `:4485` (injection failure; not read past the state set) | running -> passivated | `state`, `error`, `updated_at`, `metadata["passivated_reason"]` | `UpdateRun` -> `projectLifecycleRun` |
| W9 | `internal/store/lifecycle_control_delivery.go:569-646` | none | `prompt` (refresh), `metadata` (`lifecycle_control_bindings` and others), `updated_at` | one `PutBatchConditional` with the trajectory and receipt |
| W10 | `internal/store/engineering_assignments.go:1639` (bind), `:1910` (terminal projection) | engineering run created / ended | all (create), `state`, `result`/`error`, `finished_at` | `lifecycleObject` |
| W11 | `runtime.go:2718` (`CancelRunTrajectory` stamps trajectory identity), `actorruntime/handler.go:1208`, `textureowner/texture_controller.go:1093`, `textureowner/texture_handoff.go:336`, `management_controller.go:398`, `:512`, `:582`, `:1426`, `:3004` | not read; named only so no site is missed | | `UpdateRun` |

W1, W2 and W5 are the floor of a lifecycle run (three writes). The measured
mean of 4.7 means 1.7 more per run come from W3, W4, W6 to W11. **I could not
split the 1.7 between those sites from code.** The counter is per kind, not
per site. Step 0 splits it from the tape.

One more fact from the code: `projectLifecycleRun` already skips a write
whose body and metadata hash equal the stored object
(`lifecycle.go:3491-3496`, "Replay: true"). W3 is not skipped, because
`updated_at` differs.

### 1.3 Who reads the intermediate states

| State or field | Reader | Evidence |
| --- | --- | --- |
| pending | the actor wake: a pending run owes its agent an `initial_dispatch` wake derived from the run object in the same batch | `lifecycle.go:3520-3524`; `graph_store.go:600-612` |
| pending and running | crash boot: closes both, and passivated and blocked, the same way; creates no work | `runtime.go:3066-3083` (`closePreBootRunsAfterCrash`) |
| pending and running | planned restart: parks both | `runtime.go:3095-3098` |
| running only | admission capacity: `RunningCountByProfile` counts running runs only | `runtime.go:2897-2933` |
| running only | dead-loop detection: a running run unchanged for `deadRunGrace` (2 min) with no live loop | `runtime.go:5463-5480` |
| `updated_at` of an active run | the cell terminal deadline binds the activation's `updated_at` and terminalizes only if it is unchanged | `agentcore/continuation_schedule.go:84-93`, `:169-174` |
| `updated_at` | resume-dispatch deadline of persistent management; oldest-open surface | `management_controller.go:549-552`, `:609-612`, `:638-639`; `obligations_surface.go:285-293` |
| `labels` | every act stamps the labels the record holds; persisted before the stamp so an act never carries a label the record lacks | `run_labels.go:127-143` (`flushRunLabels`); `store.go:2439-2447` |
| terminal state, `result` | run acceptance (reads `root.Prompt` and run records, red surface), outcome binding, settlement | `agentcore/run_acceptance.go:109`; `research_checkpoint_fallback.go:27-66`; `lifecycle.go:464-468` |
| whole body | about 20 decode sites of `RunRecord` in `internal/store` (`decodeLifecycleObject[types.RunRecord]`, `ogDecode`) and the list and metadata-filter readers | grep `RunRecord` with `decode`; `lifecycle.go:1925-1937`, `graph_store.go:874-` |

What this says about dropping intermediate writes: pending, running and the
terminal states each have a distinct reader. The `updated_at`-only touch W3
has no reader of its own: the readers of `updated_at` need it to change at
state transitions, and W2 changes it (section 4, F3).

## 2. The mechanism

**Chosen: A.** Delete W3. The write at `runtime.go:4214-4217` stays out; the
`rec.UpdatedAt = time.Now().UTC()` line at `:4216` goes with it, so the
record in memory and the stored record agree.

Why it is a deletion and not a new thing: no state is added, no op kind, no
field. The tape, replay and every reader see the same sequence minus one
write per run that carried it.

### 2.1 Why not B

B names a write-once object (content-hash identity, like
`choir.run_memory_entry` at `internal/objectgraph/registry.go:92`) that holds
`prompt` and, in a later step, the create-time `metadata`, and the run names
it. Costs I found in code:

- Every reader of `RunRecord` must hydrate. There are about twenty decode
  sites (section 1.3) and several list loops that decode many runs
  (`lifecycle.go:1925-1937`), so hydrating adds one read per run to a list.
- Every writer must keep the reference. The read-modify-write writers
  (`TerminalizeRun`, `reactivateRunOnce`, `projectLifecycleRun`,
  `UpdateRunOG`) decode, change and re-marshal; if a writer decodes a
  hydrated record and marshals it, the prompt is inlined again (harmless but
  a silent loss of the saving); if a reader forgets to hydrate, it sees an
  empty prompt and nothing fails (the prompt feeds `openingText` at
  `runtime.go:4088` and the acceptance source objective at `run_acceptance.go:109`).
- `prompt` is not write-once: `lifecycle_control_delivery.go:578` replaces
  it. It would need a new object per refresh. `metadata` has deletions
  (`runtime.go:3922`), so a split by key needs tombstones.
- A binary from before B reading a tape from after B decodes a run with an
  empty prompt and, on its next write of that run, stores that. A rollback
  with an active run loses the prompt for good. The way around it is a
  reader-first release followed by a writer release, which is two deploys
  for one saving.
- The saving of B depends on a number I do not have (the prompt and metadata
  share of the 16.8 KB).
- B saves tape bytes only. The live `og_bodies` row holds one body per run
  either way.

B stays the follow-on if step 0 shows that W3 plus the per-site split cannot
reach the stated number (section 5).

### 2.2 Why not C

A patch op needs a new `ProjectionOp` kind. The projector refuses an unknown
kind (`internal/store/project.go:115-116`, `ErrProjectionBatchInvalid`), and
the replay path calls the same `projectOp`. An older binary replaying a tape
that holds a patch op does not skip it: it fails. C also needs the live write
and the replay to read the stored body inside the transaction and reproduce
one hash, over a batch that writes the run beside the trajectory, receipt and
event (`lifecycle.go:5578-5589`), and a conditional write guards on the
content hash of the object it read (`graph_store.go:786`), so the patch
must restate its base hash. The run object's hash is not even computed one
way: `lifecycle_control_delivery.go:645` uses `objectgraph.SHA256(runBody)`
while every other writer uses `objectgraph.ContentHash(kind, body, metadata)`
(`lifecycle.go:3488`, `:5575`). A witness "the patched content hash equals a
whole write" is false today for objects written at that site. C is the
largest change on a red surface for the same bytes B removes.

## 3. Expected bytes saved per run (estimate)

Baseline: 4.7 writes x 18.7 KB = 87.9 KB per run (measured, C74).

A removes W3 on every run that takes it. Let f be the fraction of runs that
reach `runtime.go:4217`. The condition at `:4202` is `initialTurns != nil`,
which holds when a texture frame injector, the calibration digest or the
mailbox injector returns turns, and the write needs `len(injected) > 0`
(`:4214`). I did not establish f from code. A run that took W3 saves one
whole write.

| Case | f | Writes per run | Saved per run |
| --- | --- | --- | --- |
| low (my assumption) | 0.3 | 4.4 | 5.6 KB, 6% |
| central (my assumption) | 0.7 | 4.0 | 13.1 KB, 15% |
| high (every run) | 1.0 | 3.7 | 18.7 KB, 21% |

Saving as a share of the tape's object bytes: choir.run is 22.6% of them, so
the central case is about 3.4% of object bytes and the ceiling about 4.8%.
On disk the saving is smaller than the plaintext count, because payloads are
compressed singly before sealing
(`docs/design/store-once-2026-11-14.md` item 3 context; not re-read here),
and a repeated body inside one write compresses less than across writes.

This is a small saving. It is what the code supports without a format
change. The reasons it is still the right first slice: it deletes a write
(CLAUDE.md "prefer deleting"), it needs no replay change, and step 0 turns
"1.7 unexplained writes" into named sites that are each a fix or a reason.

## 4. Failure list

Each entry: the failure, then how the design prevents or detects it.

**F1. A crash between writes leaves a run with no closing state.**
A removes a write and adds no state, so it adds no crash window. The states a
crash can leave a run in are still pending, running, passivated and blocked,
and the markerless boot closes all four to cancelled with
`interrupted_by_restart` (`runtime.go:3066-3083`). Detected by: after the
deployed proof, a kill -9 of the runtime mid-run on the test computer (d@e
only), then the boot closes the run and the obligations count falls to zero
(`obligations_surface.go:285`).

**F2. A reader that depended on the dropped intermediate state.**
W3 produces no state: the record's `state` is running before and after. The
readers of running (admission, dead-loop, crash close) read `state` and
`updated_at`, both set by W2. Prevented by: step 0 diffs every pair of
consecutive writes of one run on the test computer and refuses the change
unless every W3 diff is `updated_at` alone. A diff with any other field
means W3 carries a real change (a rec mutation I did not read in the
injectors) and the design stops.

**F3. A deadline or grace that reads `updated_at`.**
Three readers use it (section 1.3). (a) The cell terminal deadline binds the
`updated_at` the activation had when the cell was scheduled
(`continuation_schedule.go:90-92`) and terminalizes only if it is still equal
(`:169`). Cells are scheduled after the injection at `runtime.go:4202-4221`,
so the bound value becomes W2's timestamp instead of W3's; a later state
transition still changes it, so the stale-deadline guard still tells
activations apart. (b) `runHasNoLiveLoop` uses a two-minute grace from
`updated_at` and asks `rt.running`. Without W3 the clock starts at W2, before
`memory.initialize` (`:4185`), which can take time. The loop is registered
before the activation body (`runtime.go:3941-3945` deletes it on exit), but
I did not read where it is set, so this stays unverified. Detected by: a test
that holds `memory.initialize` for longer than `deadRunGrace` and asserts the
run is not closed as dead. (c) The persistent-management resume deadline
(`management_controller.go:549-552`) measures from `updated_at` of a parked
run, not a running one; unaffected.

**F4. Replay of an old tape that holds whole writes (and W3 writes).**
The tape is never rewritten and old events stay readable. A's replay is
unchanged: every op is still a whole object written by `projectObject`
(`project.go:241-275`). Old W3 writes replay as before. Nothing to detect
beyond the whole-tape replay in section 5.

**F5. Reverting to the old binary on a tape written by A.**
The tape holds fewer run writes and nothing new. The old binary reads it. It
writes W3 again. No format changed. This is the reason A beats B and C.

**F6. Content-hash witnesses.**
A changes no hash computation. The proof compares, for every `choir.run`
canonical id, the content hash of the last write on the tape before and
after replay (section 5). Pre-existing finding recorded here: the hash at
`lifecycle_control_delivery.go:645` is `SHA256(body)` while the other writers
use `ContentHash(kind, body, metadata)`. A conditional write that expects one
scheme against the other conflicts and retries, which is why `UpdateRunOG`
re-reads (`graph_store.go:713-722`). Not fixed here (section 7).

**F7. Run acceptance and the evidence surfaces (red, protected).**
Acceptance reads terminal run records and `root.Prompt`
(`run_acceptance.go:109`); the terminal write W5 and W6 are untouched, and
`prompt`, `result` and `metadata` are unchanged. The change is in a
mid-run write acceptance does not read. The red ceremony still applies to
the landing (conjecture delta, protected surfaces, rollback); this document
is the design half. Detected by: an accepted run on the test computer before
and after, comparing the acceptance record's fields.

**F8. Concurrent writers.**
W3 runs inside the activation, while W4 (`UnionRunLabels`), `ReplaceLifecycleActivation`
paths and cancels write the same object under hash conditions
(`graph_store.go:786`, `store.go:2501`). Removing a writer cannot add a
conflict. One consequence: W3 today loses to a concurrent label union and
retries through `UpdateRunOG` (`runWriteAttempts`, `graph_store.go:728`);
without it, one retry loop fewer. Detected by: the existing tests that hold
the read-to-write window (`runUpdateAfterRead`, `runLabelsBeforeUnionWrite`)
must pass uncached.

**F9. A write I called "touch" does something I did not see.**
`rt.store.UpdateRun` runs, besides the object write, `noteRunTerminal` for a
non-active state (`store.go:507-534`; W3 is active, so not) and
`rejectActiveRunOnTerminalTrajectory` for the non-lifecycle path
(`store.go:575-583`); on the lifecycle path it reaches `projectLifecycleRun`
and its agent projection (`lifecycle.go:3454-3466`: `ActiveRunID` changes
only if it differs). Step 0 also compares the `choir.agent` and trajectory
writes in the same batch as each W3. If W3 is the only writer that sets
`agent.ActiveRunID` for some profile, the design stops.

**F10. Retirement.**
No kind is added, no object is orphaned, nothing needs retiring. Run records
are not retired by the daily jobs I read (`telemetry_retire.go`,
`run_memory_retire.go` act on telemetry and run memory; not re-read here).
Fewer ops simply means fewer tape bytes to retain.

**F11. The saving is a smaller share than stated.**
If f is low or W3 is a different write than I think, the counter does not
fall to the stated number. Detected by: the stated number in section 5 and a
rule in section 7 that a miss opens B, not a second patch.

## 5. Proof

Step 0, before any code, read-only, on the deployed test computer (d@e only):
walk the tape for 20 runs of the profiles that reach `runtime.go:4202`
(`internal/ledgerwitness/walk.go` already walks object ops; the diff is a
script, class yellow). For each run list its `choir.run` writes in order with
`state`, `updated_at`, and the names of the changed fields (body keys,
metadata keys). Output the per-site split of the 4.7 writes, the mean
`prompt`, `result` and `metadata` sizes, and f. This replaces my assumptions
in section 3. Record it in the calibration ledger before building (CLAUDE.md
"Precommit, then score"): the claim is "W3 is an `updated_at`-only write on
at least 60% of runs", probability 0.6, settled by that tape walk.

Repo tests (yellow), written as failing first:
1. A run through the activation path with injected turns: the stored run's
   write count for that run equals the count without injection (3 plus
   labels), and the stored body equals the in-memory record.
2. `continuation_schedule` test: a deadline scheduled after the injection
   still terminalizes the activation it was scheduled for and does not
   terminalize a later one.
3. The F3(b) test: initialization slower than `deadRunGrace` does not close
   the run as dead.
4. The whole uncached suites of `internal/agentcore`, `internal/store` and
   every importer (`scripts/go-test-runtime-shards`).

Deployed proof, on the test computer after the staging deploy:
- Run N = 20 runs of one profile with injection and no new labels. The tape
  walk shows exactly one write fewer per run than the step-0 walk of the same
  profile (3 writes where it showed 4). The kind counter
  `object_rewrites["choir.run"]` (served from
  `internal/store/tape_write_counters.go:255`) shows ops per distinct object
  falling from the step-0 reading to that reading minus 1.0 for these runs.
  Stated number for the paper's mixed work: `choir.run` ops per run fall
  from 4.7 to at most 4.0 on the central assumption (f = 0.7), and to 3.7 if
  every run takes W3. A reading above 4.4 after a day of ordinary work
  falsifies A as enough (section 7).
- Whole-tape replay: replay the test computer's tape into a fresh store
  (the existing replay command) and compare, for every canonical id of kind
  `choir.run`, `content_hash` of the replayed row against the live row.
  Required: all equal, zero missing.
- An accepted run (acceptance record present) before and after the deploy,
  same fields.
- The F1 crash drill on d@e.

The landing carries the `Ring-Expedite` trailer (the ledger entry C74 and
this document name the defect: tape bytes are mostly unchanged content);
`internal/agentcore/` is an expedited path in `.github/scripts/node-b-ci-deploy`.

## 6. Rollback

Revert the commit. The old binary writes W3 again; nothing on the tape or in
the store needs repair (F5). Class: orange for the code change (runtime
behavior, one write removed); the protected surfaces it touches are the run
record and, indirectly, run acceptance (red read paths), so it lands with the
red ceremony: conjecture delta (section 5's ledger claim), protected
surfaces (this section), evidence class (tape walk and replay equality),
rollback (this section), heresy delta (none: no switch, no limit added).

## 7. Open questions, each with a conservative default

1. **Is W3 really `updated_at`-only?** Default: do not build until step 0
   shows it on a majority of runs. If a diff shows another field, stop and
   treat that field as the state it records.
2. **If A alone leaves choir.run above 4.4 writes per run, what next?**
   Default: B for `prompt` only (a content-hash object the run names, reader
   release first, writer release second, no metadata split). Not combined
   with A in one change, and not started before the step-0 numbers say the
   prompt is a large share. The rule that follows from a miss is a
   structural assessment, not a second patch (CLAUDE.md, three rounds).
3. **Carried labels at run start (W4 from `run_memory.go:190`).** Folding
   them into W2 would remove one write, but the source run is chosen inside
   `memory.initialize`, after W2, and the labels invariant (persisted before
   an act is stamped) is a provenance rule. Default: leave W4 alone.
4. **Base64 on every object op (`Object.Body []byte`).** A third of every
   object op's bytes, for every kind, not only runs. It is a format
   change to what the tape holds, so it is not in this slice. Default:
   record it as a separate problem with its own design; do not touch here.
5. **The inconsistent hash at `lifecycle_control_delivery.go:645`.**
   Default: record it as a finding with the F6 evidence and fix it
   in its own change; it is not needed for A.
6. **`lifecycle_control_bindings` grows inside a mutable run.** It is the
   run's own growing list, the case the third clause of T2 (a gate for a
   kind that embeds a growing list) is about. Default: out of scope for this
   slice; named for the gate's design.
7. **Where does `owner_email`-style data sit in run metadata?** The key name
   `owner_email` is assigned at two places in agentcore (found by grep, not
   read). Default: not in this slice; named for the owner as a secrets
   hygiene question, no value quoted.

## 8. What I could not verify

- The mean sizes of `prompt`, `result` and `metadata`: no source in the
  repo; step 0 measures them.
- The fraction f of runs that take W3 and the per-site split of the 1.7
  extra writes.
- Whether any injector mutates `rec` (read only the `shouldAppendInitial...`
  and metadata greps).
- Where `rt.running[runID]` is registered relative to the running write
  (F3 b).
- The bodies of `persistLifecycleSubmittedRun`, `runtime_persistence.go:39`,
  `handleExecutionError`, and the W11 sites; the table names them by call
  line only.
- The compressed on-disk effect of the saving.
- Nothing here was compiled or run.

## 9. What the first measurement showed, and the counter needed first

Measured 2026-11-15 on d@e (the test computer) through the read-only API,
GET only, one request every 21 seconds: `GET /api/runtime/observability` once,
`GET /api/runs?limit=20` once, `GET /api/runs/{id}` for the 20 newest runs.
Raw responses are in the session scratchpad, not in the repo.

### 9.1 Read paths that exist, and what they cannot show

| Path | Shows | Cannot show |
| --- | --- | --- |
| `GET /api/runtime/observability`, block `telemetry_batching.object_rewrites["choir.run"]` (`internal/store/telemetry_stats.go:303`, `object_rewrite_counters.go:110`) | ops, bytes, mean bytes per op, distinct objects, writes per object, since boot | any split by site, state or field |
| `GET /api/runs/{id}` (`internal/agentcore/api.go:610-631`) | the run's current record only | any earlier write |
| `GET /api/runs/{id}/events` | the run's `loop.*` events with times | the object writes |
| `cmd/ledger-witness` (`cmd/ledger-witness/main.go`) | where named inbox records sit on the host's artifacts | run writes; needs the host's artifact files, which I may not read |

No existing path lists the `choir.run` writes of one run with time, size and
changed fields. So the per-write walk the design's step 0 names, the split of
writes by site (W1 to W11), the share of runs that take W3 and whether W3
changes only `updated_at` **could not be done**. I did not build one.

### 9.2 What was measured

Counter, d@e since its last boot: `choir.run` 10 ops, 151,219 bytes, mean
15,122 bytes per op, 2 distinct objects, 5.0 writes per object. Two runs
only: a sample too small to say anything about the paper's 4.7.

Final bodies of the 20 newest runs (JSON length of each field in the current
record; 6 management runs, 14 texture runs; 9 completed, 11 passivated):

| Field | Mean | Max | Notes |
| --- | --- | --- | --- |
| `prompt` | 6,157 | 16,215 | texture runs 2.4 to 16.2 KB, management 873 B |
| `result` | 149 | 741 | zero on 15 of 20 |
| `error` | 0 | 0 | |
| `metadata` | 16,404 | 81,159 | the largest field by far |
| `labels` | 40 | 115 | |
| whole record | 23,414 | 86,842 | before base64 (x 4/3 on the tape) |

Inside `metadata`, one key decides it: `texture_available_source_entities`
is 1.4 KB in a passivated texture run and 40 to 79 KB in a completed one
(runs 13, 14, 15, 18, 20); a completed texture run's metadata is 42 to 81 KB.
Without it, metadata is about 1 to 2 KB (`llm_fallbacks` 291 B, `llm_policy_source`
48 B, ids). That key is a list merged into the run as evidence arrives
(`internal/textureowner/texture_evidence_sources.go:459-465`,
`texture_agent_revision.go:675-683`); it only grows during a run. It is the
growing-list-in-a-mutable-object case the third clause of T2 names, and it
lives inside `choir.run` itself.

These are end-of-run sizes, not the sizes at each write: a run's earlier
writes carry less of the list. The paper's 18.7 KB per write is lower than
this sample's mean 23.4 KB x 4/3, so d@e's texture-heavy runs are heavier
than the paper's mix (this is the sample, not the paper).

### 9.3 What this does to the decision

It weakens section 3 and section 0 as written:

- The bulk of a run write is `metadata`, mean 70% of the body, nearly all of
  it one list in texture runs. `prompt` is 26%. Everything else is under 3%.
- Dropping W3 (A) removes one whole write per run that takes it; the size
  of that saving is still unknown, and it does not touch the growth of the
  list inside the other writes.
- B for `prompt` only (open question 2) would remove at most about 26% of
  the body per write after the first; B for the one list key would remove
  more on texture runs and is smaller to build than a split of the whole map
  (one key, read in `texture_evidence_sources.go:454` and written at the two
  sites above), but it is still a stored-shape change with the rollback
  hazard of section 2.1.

I do not re-decide here. I state that the case for A rests on one write the
tape has not been shown to contain, while the measured weight is in a list
this document did not name when it chose. The design should be re-taken after
the counter below has run for a day. Until then no code on this slice.

### 9.4 The counter needed first (yellow, no code written)

The smallest change that answers every question above is one new counter
beside `object_rewrites`, filled where a run write is built, because the
interceptor at `internal/store/projection_tape.go:105-120` sees only an
object and `tallyTapeParts` is deliberately free of JSON decoding
(`tape_write_counters.go:128-133`). A label cannot be recovered there.

What it counts, per `choir.run` write, under the key `site/state/change`:

- `site`: a fixed string at each writer, from this list of file:line:
  `create` (`internal/store/graph_store.go:597`, and the create branch of
  `internal/store/lifecycle.go:3528`), `activate` and `activation_update`
  (`lifecycle.go:3528`, told apart by the stored state read at `:3333-3351`),
  `terminalize` (`lifecycle.go:5573-5589`), `reactivate`
  (`lifecycle.go:5690-5725`), `labels` (`internal/store/store.go:2501`),
  `control_delivery` (`internal/store/lifecycle_control_delivery.go:646`),
  `ogupdate` (`internal/store/graph_store.go:786-790`), `engineering_bind`
  and `engineering_terminal` (`internal/store/engineering_assignments.go:1639`,
  `:1910`). The `W3` write arrives through `activate`/`activation_update` and
  is told apart by `change`, not by site.
- `state`: the run's new `state`.
- `change`: `updated_at_only` when the new record equals the stored one in
  every field but `updated_at`, otherwise the sorted names of the changed
  body keys and `metadata:<key>` for changed metadata keys. Every writer
  except the label writer and `graph_store.go:786` already holds both the
  stored record and the new one (`storedRun` at `lifecycle.go:3336`,
  `existingRec` at `graph_store.go:739`, `run` after decode at
  `lifecycle.go:5528`), so the comparison costs no extra read.
- Bytes by field of the written body: `prompt`, `result`, `error`,
  `metadata` (and the largest metadata key's name and size), `labels`,
  `other`.

Where it goes: a `runWrites map[string]RunWriteStat` in `tapeWriteMaps`
(`internal/store/object_rewrite_counters.go:122-150`, next to
`addRewrite`), a method on the store called by the writers above just
before their conditional put, and the served field next to
`ObjectRewrites` (`internal/store/telemetry_stats.go:303`). Keys are a closed
set (sites x states x change names); the change name is capped to a bounded
number of keys with an `other` bucket, as `bumpKey` does
(`tape_write_counters.go:55-60`). It counts the writes the store accepted, so
it must run after the conditional put succeeds, not before.

This is class yellow (a counter, no behavior). It is the "counter that
answers the premise" CLAUDE.md asks for before a fix. It answers: writes per
run by site, the share of runs that take W3, whether W3 is `updated_at`
only, and the field-by-field bytes at each write, from the paper's own work.

### 9.5 What I could not see

- Any individual write, its time or its diff (section 9.1).
- W1 to W11 split, f, and whether W3 is `updated_at`-only.
- Sizes at each write: only the end-of-run sizes above.
- Whether `GET /api/runs/{id}` returns the stored body unprojected: I read
  `HandleRunResource` (`api.go:626-631`) which returns `rt.GetRun`; I did not
  read `GetRun`. The returned keys are the `RunRecord` fields, so I treat it
  as the record.
- The paper's mix: d@e is a test computer with a texture-heavy history.
