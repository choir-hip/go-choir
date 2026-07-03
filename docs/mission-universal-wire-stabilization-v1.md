# Mission: Universal Wire Stabilization v1

## Status

Paradoc v1, created 2026-07-03 03:00 EDT, Boston, MA.
Supersedes `docs/mission-universal-wire-agent-pipeline-v1.md` (prompt fix
settled, broader stabilization needed).

## Objective

Stabilize the Universal Wire pipeline end-to-end so that it is live,
functional, and publishing real LLM-synthesized articles on staging. This
requires four sub-objectives in order:

1. **Clean up bad tests** — fix the flaky race-detector test
   (`TestToolLoopEndToEndWithRuntime`) that blocks CI on every push.
2. **Improve CI** — ensure CI is green on main, the race detector shard is
   stable, and deploys are not blocked by flaky tests.
3. **Stabilize the deployed system** — recover staging VMs after the deploy
   failure, ensure the runtime (port 8085) is healthy, and verify the
   full service stack is operational on `choir.news`.
4. **Assure Universal Wire is live and publishing** — verify sourcecycled
   is cycling, processor runs are dispatching, Texture agents are
   synthesizing articles with source body text, and articles are appearing
   on the Wire feed.

This sets up tomorrow's scale-up: vastly increasing the number of sources
and stories processed into the knowledge graph and published.

## Context

### What v1.1 Settled

- Source body text (`excerpt_text`) is now included in the Texture agent
  prompt (`buildCoagentTextureRevisionPrompt`, commit `d38d3afd`).
- Processor agent has correct tool registry (`AllowCoAgentTools: true`,
  `AllowedDelegateTargets: [texture]`).
- Processor prompt instructs `spawn_agent` with `role=texture` for
  newsworthy items.
- Edition bootstrap is self-healing (`ensureUniversalWireEdition`).
- Handoff builder extracted to `internal/wire/processorkey/`.
- All local wire/coagent tests pass.

### Current Breakage (2026-07-03 02:20 UTC)

**CI failures (commit `660eee66`):**
- `TestToolLoopEndToEndWithRuntime` fails in race shard 2 — "missing
  expected event kind: loop.completed". This is a **test bug**, not a
  production data race: `waitForToolLoopTask` returns as soon as the run
  state is terminal, but `EventRunCompleted` may not be persisted yet
  (async event persistence). The test then immediately lists events and
  finds the completion event missing. This has failed on 2 consecutive
  pushes.
- All other 20 CI jobs pass (vet, build, non-runtime shards, non-runtime
  race, runtime shards 0-3 without race, runtime race shards 0,1,3, TLA+,
  docs truth, integration smoke, SBOM, FlakeHub).

**Staging deploy failure:**
- The deploy-impact classifier sees `internal/runtime/*` changes → marks
  `vmctl_restart` + `active_vm_refresh`.
- VM refresh timed out: "guest did not become healthy at
  http://10.202.58.2:8085 within 3m0s" and same for
  `vm-universal-wire-platform`.
- Host services DID deploy (proxy reports
  `deployed_commit: 660eee6633ec1d64b3321b394b31288dd5b165b8`).
- Staging is degraded: runtime (port 8085) connection refused, ollama
  connection refused. Sourcecycled and Qdrant are healthy.

**Root cause of VM refresh timeout:**
The VM refresh kills the old Firecracker process and boots a new one with
the updated guest image. The 3-minute timeout for guest health may be too
short for a cold boot, or the new guest image may have a startup issue
related to the OG migration changes. This needs investigation on Node B.

### The Flaky Test

`TestToolLoopEndToEndWithRuntime` in
`internal/runtime/toolloopvalidation_test.go:653`:

```go
// Wait for completion.
waitForToolLoopTask(t, s, rec.RunID, 5*time.Second)

// ... then immediately:
evts, err := s.ListEvents(context.Background(), rec.RunID, 100)
// checks for EventRunCompleted in evts
```

`waitForToolLoopTask` polls `s.GetRun` until `rec.State.Terminal()`. But
the run state becomes terminal *before* the `EventRunCompleted` event is
persisted (the runtime sets the state, then appends the event). Under the
race detector (which slows everything 2-10x), the gap between state update
and event persistence is large enough that the event list check races.

**Fix:** Add a retry/poll loop for the expected events, or wait for the
event list to contain `EventRunCompleted` before asserting.

## What Needs to Be Done

### Phase 1: Fix the Flaky Test (Green/Yellow)

Fix `TestToolLoopEndToEndWithRuntime` to poll for events rather than
checking once. The test should wait for `EventRunCompleted` to appear in
the event list, not just for the run state to be terminal.

Also audit other tests in `toolloopvalidation_test.go` for the same
pattern — any test that checks events immediately after
`waitForToolLoopTask` has the same race.

### Phase 2: Push, Monitor CI, Verify Green

Push the test fix. Monitor CI. All jobs including race detector shard 2
must pass. If the race detector still fails, investigate further — but
the expected fix is the event polling.

### Phase 3: Recover Staging

The staging VMs are down. Options:
- **Re-run the deploy workflow** via `workflow_dispatch` with
  `force_staging_deploy: true` — this will retry the VM refresh.
- **Investigate VM boot failure** — if the VMs consistently fail to boot
  with the new guest image, there may be a startup issue from the OG
  migration. Check vmctl logs on Node B.
- **Increase VM refresh timeout** — if the 3-minute timeout is too short
  for cold boots after a guest image change, increase it in the deploy
  script.

### Phase 4: Verify Universal Wire is Publishing

Once staging is healthy:
1. Verify sourcecycled is cycling (check sourcecycled health/logs).
2. Verify processor runs are being dispatched (check trajectories).
3. Verify Texture agents are producing article revisions (check texture
   documents).
4. Verify articles are appearing on the Wire feed
   (`choir wire stories` or `GET /api/universal-wire/stories`).
5. Verify at least one article has an event-grade headline, English body,
   and cited sources.

### Phase 5: Set Up for Scale-Up (Tomorrow)

Document the current source configuration, throughput, and any bottlenecks.
Identify what needs to change to vastly increase sources and stories:
- Source configuration (`configs/sources.json`)
- Processor dispatch concurrency limits
- Qdrant dedup capacity
- Edition/article storage
- Reconciler cadence

## Parallax State

status: working

mission conjecture: if the flaky race test is fixed, CI is green, staging
VMs are recovered, and Universal Wire is verified publishing real
LLM-synthesized articles on choir.news, then the pipeline is stable enough
to scale up sources and stories tomorrow.

deeper goal (G): the automatic newspaper at scale — broad multilingual
ingestion, event understanding, English synthesis, live article updates,
running reliably through the agent pipeline. Tomorrow's scale-up depends
on today's stabilization.

witness/spec (A/S): green CI on main, healthy staging (all services
operational), at least one real LLM-synthesized article on the Wire feed
with event-grade headline, English body, and cited sources.

invariants / qualities / domain ramp (I/Q/D):
- I: Do not reintroduce deterministic synthesis. Do not add story caps.
  Do not use source labels as headlines. Do not touch Texture core, O1-O3,
  or delete agent pipeline code. Do not weaken CI to make flaky tests
  pass. Do not skip the race detector.
- Q: CI must be genuinely green (not flaky-green). Staging must be
  genuinely healthy (not degraded). Articles must be genuinely
  LLM-synthesized (not template prose).
- D: local test fix → CI green → staging recovered → Wire publishing →
  scale-up ready. Each domain embeds in the next.

variant (conjecture descent) V: count conjectures about the pipeline's
stability and publishing capability.
Current: 5.
- C1: The flaky test is a test bug (event polling race), not a production
  data race (SUPPORTED — local tests pass without -race, failure is
  "missing event kind" not "DATA RACE detected")
- C2: Fixing the event polling in the test makes CI green (UNDICIDED —
  need to push and verify)
- C3: Staging VMs can be recovered by re-running the deploy (UNDICIDED —
  VM refresh timed out, may need investigation)
- C4: Sourcecycled is cycling and dispatching processor runs on staging
  (STRUCTURALLY SUPPORTED — sourcecycled health is ok, but runtime is
  down so dispatch may be failing)
- C5: The agent pipeline produces real LLM-synthesized articles on staging
  (UNDICIDED — requires healthy runtime + model calls)
Target: 0.

budget: 5-8 passes. Pass 0 spent (analysis and mission doc). 5-7 remaining.

authority / bounds: may modify test files, CI workflow, deploy scripts.
May push to origin/main. May trigger workflow_dispatch. May investigate
staging via API. May not touch Texture core, O1-O3, or delete agent
pipeline code. May not SSH to Node B directly (no access).

mutation class / protected surfaces:
- Green/Yellow: test fixes (no runtime behavior change)
- Orange: CI workflow changes, deploy script changes
- Red: staging deploy, VM lifecycle
- Protected: Texture revision creation, corpusd sync contract, source
  entity graph, agent pipeline code.

evidence packet:
- Flaky test failure logs (CI run 28642495032, shard 2)
- Local test pass without -race (this session)
- CI green status (pending)
- Staging health status (pending)
- Wire feed with real articles (pending)

heresy delta: `discovered` for the flaky test race (test bug, not
production race). `repaired` for H-WIRE-SOFT-GUARDRAIL if articles are
publishing. `discovered` for any VM boot issues from OG migration.

position / live conjectures / open edges:
The flaky test is the immediate blocker — it fails CI which blocks
clean deploys. The staging VMs are down from the last deploy attempt.
The pipeline code is structurally complete (v1.1 settled the prompt
fix), but cannot be verified end-to-end until staging is healthy. The
open edge is whether the VM refresh timeout is a transient issue or a
systemic boot failure from the OG migration.

next move: Fix the flaky test (Phase 1), push, monitor CI. In parallel,
trigger a workflow_dispatch to re-deploy and recover staging VMs.

ledger file: `docs/mission-universal-wire-stabilization-v1.ledger.md`

version / lineage: v1. Depends on
`docs/mission-universal-wire-agent-pipeline-v1.md` (settled, prompt fix
done). Successor: scale-up mission (to be created tomorrow).

learning state: retained here / promoted outward / successor links

settlement: settled when CI is green, staging is healthy, and at least
one real LLM-synthesized article is on the Wire feed. Open handoff if
VMs cannot be recovered (document the boot failure for operator
intervention).

## Suggested Goal String

```text
Use Parallax on docs/mission-universal-wire-stabilization-v1.md. Mission: stabilize the Universal Wire pipeline so it is live, functional, and publishing real LLM-synthesized articles on staging. Four phases: (1) Fix the flaky race-detector test TestToolLoopEndToEndWithRuntime — it checks for EventRunCompleted immediately after run state becomes terminal, but event persistence is async; add event polling. (2) Push, monitor CI, verify all jobs green including race detector. (3) Recover staging VMs — they timed out during deploy (guest did not become healthy in 3m); re-run deploy via workflow_dispatch or investigate VM boot failure. (4) Verify Universal Wire is publishing: sourcecycled cycling, processor runs dispatching, Texture agents synthesizing with source body text, articles on Wire feed with event-grade headlines. This sets up tomorrow's scale-up of sources and stories. Invariants: no deterministic synthesis, no story caps, no source-label headlines, do not touch Texture core or O1-O3, do not weaken CI or skip race detector. Budget: 5-8 passes. Exit: settled when CI green + staging healthy + one real article on Wire feed.
```
