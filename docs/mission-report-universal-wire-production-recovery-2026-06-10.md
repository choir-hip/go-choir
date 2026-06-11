# Mission Report: Universal Wire Production Recovery — 2026-06-10

## Goal

Fix Universal Wire production end-to-end: sourcecycled must not overload the platform computer, processor handoffs must complete into VText article revisions, platform publication must sync full VText revision history into platformd, the durable Wire edition must expose non-empty stories, and the authenticated Universal Wire app must render article cards on staging.

## Timeline

| Time (UTC) | Event |
|---|---|
| ~17:05 | Prior operator confirmed 32+32 processor runs submitted, concluded news was producing (false positive) |
| ~17:42+ | vmctl repeatedly marked platform VM unhealthy; sandbox health timing out |
| ~18:00 | Investigation: platformd has 0 VText docs/revisions; platform Firecracker at ~240% CPU |
| ~18:15 | Root cause doc written: submission ≠ completion; backpressure missing |
| ~18:30 | Cognitive transform review (state machine, backpressure, commutative diagram, contrapositive, prototype honesty) |
| ~18:40 | MissionGradient control doc written |
| ~18:45 | Committed documentation checkpoint (`ef1b41f0`) |
| ~19:00 | Implemented backpressure: sourcecycled in-flight tracking + runtime 429 guard (`27f4eaf8`) |
| ~19:02 | Pushed to origin/main, CI started, stopped wedged platform VM |

## What Shipped

### Commit `27f4eaf8` — fix: enforce processor backpressure

- **sourcecycled** (`cmd/sourcecycled/main.go`): 
  - Added `inFlightWindow` to dispatcher struct (configurable via `SOURCE_SERVICE_AGENT_DISPATCH_INFLIGHT_WINDOW_SECONDS`, default 15 min)
  - Before submission loop: count recently submitted processor requests via `CountRecentlySubmittedProcessorRequests`
  - Compute effective cap: `submitCap = maxProcessorRequests - inFlight`
  - Break submission loop when `result.ProcessorSubmitted >= submitCap`
- **cycle storage** (`internal/cycle/storage.go`):
  - Added `CountRecentlySubmittedProcessorRequests(ctx, since)` — counts processor_requests with status='submitted' and updated_at >= since
- **runtime** (`internal/runtime/api.go`):
  - `HandleInternalRunSubmission` now rejects processor profile submissions with `429 Too Many Requests` when `RunningCount() >= RUNTIME_MAX_PROCESSOR_RUNS` (default 32)

### Deploy Status

- Pushed `27f4eaf8` to origin/main
- CI in progress
- Platform VM stopped to clear wedge

## What Was Proven

- sourcecycled source fetch is not the blocker (198 successful fetches, 4241 items per cycle)
- Platform VM wedged under 64 concurrent processor runs
- Platformd had zero VText documents/revisions — no publication ever completed
- Missing backpressure is the causal root of the wedge
- `processor_submitted` ≠ article published

## Unproven or Partial

- Whether deployed backpressure prevents re-wedge (need production proof)
- Whether VText article creation and autonomous publication work once VM is healthy (need end-to-end proof)
- Whether Wire edition transclusion and platformd sync work after publication (need end-to-end proof)
- Whether the 64 submitted processor runs produced any VText docs inside the wedged guest (need disk inspection)

## Residual Risks

- Platform VM may still wedge if individual processor runs are very expensive (not just count, but resource per run)
- In-flight window estimation may be inaccurate if processor completion times vary widely
- No completion callback from runtime to sourcecycled — submitted requests stay "submitted" forever in sourcecycled DB, so in-flight count includes stale submissions
- Runtime overload guard (429) only helps if sourcecycled respects it and retries later; currently sourcecycled treats 429 as transient and retries, which could still hammer the sandbox if it recovers briefly

## Next Executable Probes

1. Wait for CI pass on `27f4eaf8`
2. Confirm Node B deploy of new commit
3. Resume platform VM
4. Verify sandbox healthy and backpressure prevents re-wedge
5. Verify processor → VText → publish → platformd sync → edition → stories chain
6. Verify authenticated Universal Wire app renders article cards
7. If articles still don't appear, inspect VM disk for processor run states

## Rollback Refs

- Prior deploy: `138ab79f` (periodic GC + 16GB disk)
- Backpressure fix: `27f4eaf8`
- Platform VM state: stopped via vmctl; data.img preserved


## Scientific Update — 2026-06-10 23:15 UTC

### Hypotheses ruled out

1. **Frontend-only rendering bug** — false. Empty/broken UI matched missing durable publication state.
2. **Source ingestion stalled** — false. sourcecycled kept fetching thousands of items and queuing processor work.
3. **Dispatch transport still blocked by stale direct TAP URL** — false after UDS proxy deployment.
4. **Platform VM disk exhaustion is the immediate blocker** — false after 16 GiB expansion and periodic GC.
5. **Guest lacks any path to host proxy** — false after tap firewall rule for host 8082 was added; guest-local curl to `10.203.176.1:8082/health` succeeded.
6. **Host publish route fundamentally broken** — false. Direct host replay of a sample article with full revision payload and full run metadata returned `201 Created` and a real publication record.

### Hypotheses confirmed

1. **Backpressure bug** — confirmed. Missing active-concurrency control let sourcecycled admit 32+32 processor runs and wedge the platform VM.
2. **Guest publish configuration bug** — confirmed and fixed. Runtime initially logged `wire publish is not configured`.
3. **Proxy desktop-resolution bug** — confirmed and fixed. Internal wire publish route resolved the wrong desktop before being pinned to the platform desktop.
4. **Tap firewall omission for host proxy 8082** — confirmed and fixed. Live guest probe timed out before the rule and succeeded immediately after adding the 8082 INPUT/DNAT rule.
5. **Publish-time self-read / synchronous reread fragility** — confirmed and reduced. Direct host replay with the full inline payload succeeds; relying on proxy rereads was part of the earlier failure surface.

### Current frontier

The remaining failing edge is **semantic publication eligibility / completion accounting**, not transport.

Observed current state:

- Fresh processor batches admit under the 4-run cap.
- Guest creates article revisions and attempts publish.
- Current guest logs now fail with `revision is not eligible for autonomous wire publish` instead of network timeout.
- platformd still shows `0` documents / `0` revisions.
- sourcecycled still has no completion feedback loop, so it keeps 4 requests marked `submitted` after the guest becomes idle.

### Best current theory

There are two live issues left:

1. **Eligibility mismatch**: live processor-created article revisions still fail the host-side publication policy even though older sampled revisions with durable lineage can be published successfully when replayed manually with full metadata.
2. **Completion accounting gap**: sourcecycled uses stale `submitted` rows as in-flight budget until manually reset or aged out; it does not reconcile against runtime completion state.

### Next probes

1. Inspect one current rejected revision's exact metadata and its correlated run metadata, then compare that against the successful manual replay payload.
2. Patch sourcecycled to poll submitted runtime run states and release in-flight budget on terminal states.
3. Once a fresh revision is publish-eligible, verify platformd rows increase, then verify Wire edition ordering and headline-open behavior from the UI.


## Scientific Update — 2026-06-11 03:30 UTC

### What changed

The investigation moved from infrastructure failures into the semantic decision frontier inside processor runs.

### Newly ruled out

1. **Ghost articles were manual mocks** — false. They were real platform-VM-local article revisions that leaked into the list before durable publication.
2. **The current blocker is still guest→host transport** — false. Guest-local curl to host TAP `:8082/health` succeeded after the tap firewall repair, and direct host replay of a full article payload returned `201 Created`.
3. **Queue bookkeeping alone explains the stall** — false. Sourcecycled queue accounting now self-heals much better; the dominant remaining failure is what processors decide to do after admission.

### Strongest current evidence

- One-at-a-time processor admission works.
- A clean admitted processor completed and explicitly returned: "No VText spawns required — all 10 items have existing article coverage."
- Another later processor returned: "VText spawning deferred — blocked on wire corpus search restoration for dedup verification."
- `search_wire_corpus` previously searched guest-local unpublished docs; that has now been cut over to published-only docs.
- Universal Wire list honesty was repaired so guest-local unpublished stories should no longer surface as canonical list items.
- platformd still remains at `0` docs / `0` revisions.

### Best current theory

The current bottleneck is semantic/agentic: the processor still carries stale continuity or stale coverage beliefs and suppresses the VText/article creation step, or it spawns child work that does not converge into durable publication.

### Next probes

1. Treat the processor channel/continuity as suspect and test one run on the new `processor-v2:*` channel.
2. If the stale belief persists, bypass continuity for one clean run.
3. Capture the live child-run topology of a single admitted processor.
4. Verify that the first truly clean processor run either creates one platformd row or yields a single exact semantic blocker.


## Conjecture Ledger Snapshot

- **C1 split-brain list/open:** confirmed and mitigated. List was exposing guest-local unpublished stories; click path expected durable docs.
- **C2 transport/publish path:** mostly ruled out as the primary blocker after publish URL, desktop resolution, and TAP 8082 fixes.
- **C3 queue bookkeeping:** improved enough that it is no longer the dominant blocker.
- **C4 unpublished corpus poisoning:** confirmed and fixed by constraining `search_wire_corpus` to published-only docs.
- **C5 stale processor continuity:** current leading conjecture. Completed processors still express old beliefs such as `wire corpus search restoration blocked`.
- **C6 single-run destabilization:** still active. One admitted processor can still drive the guest unhealthy before durable publish reaches platformd.
- **C7 primary mission frontier:** on a fresh guest and fresh processor identity, one admitted processor must either yield one durable platformd publication or expose the exact child-run / coverage-decision reason it does not.


## Architectural Conjecture: Realest Decoupled Pipeline

A new architectural conjecture emerged from the latest clean processor runs: even when transport, queue accounting, and list honesty are repaired, one processor run still tries to do too much semantic control work. The realest decoupled pipeline that preserves current topology is:

```text
source fetch
-> normalized source facts / source items
-> processor evidence pass
-> durable candidate story ledger on platform VM
-> coverage / dedup against published corpus only
-> publication-candidate selection
-> VText article spawn or revision
-> autonomous publish to platformd
-> durable Wire edition update
-> public stories list / headline open
```

This does **not** create a second article truth. It keeps:
- candidate ledger = pre-article planning state
- VText = article truth
- platformd = public durable publication truth

The conjecture is that the current monolithic processor run is over-coupled and can suppress the whole pipeline with a local semantic decision ("already covered", "need dedup verification", "need publication direction"). The decoupled version would make those states durable and explicit instead of burying them in one processor completion result.


## Scientific Update — 2026-06-11 04:40 UTC

### New decisive trace

A clean one-at-a-time processor run on a fresh guest produced the first precise child-run topology evidence:

```text
processor 9a2606ee...
-> vtext ffaa48da... (completed)
-> processor completes
-> super f653e2b6... continues on same document channel
   -> vtext 42eda5a8... (completed)
-> sourcecycled admits next processor dbc6478a... before the super/vtext continuation is finished
```

platformd still remained at `0 docs / 0 revs`.

### Updated best theory

The publication chain can escape the root processor run and continue through a super-owned branch on the same document channel. Root-run or direct-child accounting is therefore insufficient as the admission invariant. A durable publication-chain lock likely needs to be keyed by channel / trajectory / candidate state, not just by the root processor run id.


## Completion Branch Outcome — 2026-06-11 05:10 UTC

The narrowed semantic-frontier objective was satisfied on the **expose exact reason** branch, not on the **durable publication succeeded** branch.

### What was proven

On a fresh platform guest and fresh `processor-v2:*` identity, one admitted processor did not produce a durable platformd publication. The exact reason is now exposed:

```text
processor run completes
while a super-owned continuation on the same document channel remains active
which can spawn its own VText child work
so root-run completion is not a safe admission invariant for publication progress
```

This means the current orchestration model is using the wrong liveness boundary. Universal Wire publication is not a rooted tree that ends when the root processor run finishes; it behaves like a channel/trajectory-scoped coagent process.

### Consequence

The next architecture/protocol change should not be another transport patch. It should change the admission / liveness invariant from `root processor run is done` to something closer to `publication trajectory for this channel/candidate is settled`.


## Planned Refactor Program — 2026-06-11 discussion checkpoint

These are not part of the just-completed semantic-frontier proof branch. They are forward-looking architectural notes captured from discussion so tomorrow's design/doc revision starts from explicit intent rather than chat residue.

### 1. Remove parent/child as the primary causality/control abstraction

Current diagnosis:
- parent/child is still used as a control invariant in places where the real work is a shared coagent trajectory over a document/channel/publication object.
- this leaks badly in Universal Wire, where a root processor can complete while work continues through VText/super-owned continuations on the same channel.

Planned direction:
- treat parent/child, if retained temporarily, as **provenance/debugging only**;
- stop using parent completion as a publication-progress or admission invariant;
- move liveness/accounting toward channel / trajectory / candidate / artifact scoped coordination;
- redesign causality around explicit coagent coordination and durable artifact lineage rather than rooted trees.

### 2. VText should not route to co-super or vsuper

Current diagnosis:
- VText currently has too much latitude to request general continuation/orchestration through super pathways.
- regular research/article publication should remain VText-centered, with researcher evidence support when needed.

Planned direction:
- VText may call researcher for factual/current evidence work.
- VText may call **super only when actual coding / execution / privileged system work is required**.
- VText should not route to co-super or vsuper.
- remove fuzzy "general continuation/orchestration" language; continuation of what, orchestration of what, must be explicit and artifact-scoped.

### 3. Introduce nucleus sandboxes for ephemeral execution

Planned direction:
- supers will use nucleus sandboxes for ephemeral execution work.
- separate ephemeral execution environments from persistent user/platform computers.
- keep persistent-computer semantics for long-lived product state; use nucleus for throwaway execution/probing.

### 4. Rename `sandbox` to `autoputer`

Reason:
- what the product currently calls sandboxes are actually persistent computers, so `sandbox` is a misleading ontology.
- `autoputer` (automatic computer) is closer to the object we really mean.

Planned effect:
- rename product/runtime terminology from sandbox -> autoputer in docs, architecture, and later implementation.
- this should be done carefully to avoid conflating ephemeral nucleus sandboxes with persistent autoputers.

### 5. Rename `platformd` to `corpusd`

Reason:
- `platformd` is too easily confused with platform computers / platform VM ownership.
- `corpusd` makes the durable publication/corpus role explicit and distinct.

Planned effect:
- rename service/docs/API references from platformd -> corpusd.
- keep the semantic distinction clear: corpusd is durable publication state, not the platform computer itself.

### 6. Upgrade MissionGradient around conjectures

Current diagnosis:
- this mission only partially reflected the real conjecture workflow until a conjecture ledger was added manually.
- the useful control object was not a flat checklist but a conjecture program with falsifiers and branch outcomes.

Planned direction:
- upgrade MissionGradient so conjectures are first-class:
  - conjecture ledger,
  - current strongest evidence,
  - next falsifier,
  - branch outcomes,
  - explicit outside-the-envelope blind spots.
- move forward with conjecture learning as the central control model.

### 7. Realest decoupled news pipeline remains the architectural north star

Planned direction:
```text
source fetch
-> normalized source facts / source items
-> processor evidence pass
-> durable candidate story ledger on platform computer
-> coverage / dedup against published corpus only
-> publication-candidate selection
-> VText article spawn or revision
-> autonomous publish to corpusd
-> durable Wire edition update
-> public stories list / headline open
```

Constraint:
- no second article truth.
- candidate ledger is pre-article planning state only.
- VText remains article truth.
- corpusd remains durable public publication truth.
