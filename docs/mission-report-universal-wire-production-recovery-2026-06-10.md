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
