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

## Substrate Definitions (Owner-Provided, Releveled from Conjecture)

These are not hypotheses to test. They are definitions about the current
state of the system, established by observation of the architecture's
history. They inform the approach from the start, not conditionally.

### D1: The race-detector CI model is from a prior architecture

The race detector runs the full runtime test suite (sharded across 4
jobs, 30-minute timeout each) under -race, which slows execution 2-10x.
The runtime has since undergone: OG migration (SQLite → Dolt), actor
runtime migration (mutexes → Go-channel mailboxes), and wire pipeline
rewrite (deterministic → agent pipeline). The tests were not rewritten
for the actor runtime model. Tests that check event ordering by polling
run state then immediately listing events race against async event
persistence — this is a structural pattern, not a one-off bug. The race
detector is surfacing test-infrastructure bugs from a prior concurrency
model, not production data races.

**Implication:** The first-pass fix (event polling) may quiet this one
test, but the pattern is likely systemic. The constructive approach is
to review what the race detector should be testing in the current
architecture, not to patch each test individually. Simplify: focused
concurrency tests on the actor runtime's actual invariants, not
full-suite race sharding that finds test bugs.

### D2: The TLA+ specs are from a prior architecture

The TLA+ specs in `specs/` were written before the OG migration, actor
runtime migration, and wire pipeline rewrite. The probability that they
are well-formed, stable, and well-designed for the current architecture
is drawn from a sparse base distribution. They are checking invariants
for a system that no longer exists in the form modeled.

**Implication:** The TLA+ model check in CI is either (a) passing
because the specs are abstract enough to still hold, (b) passing because
the specs don't exercise the changed surfaces, or (c) not actually
checking anything meaningful. Without a review, it's noise with a
green-checkmark. The constructive approach is to review what the specs
actually model, whether those invariants still hold, and whether TLA+ is
the right tool for the current system's complexity level. If not,
remove it from CI rather than carrying stale verification theater.

### D3: The documentation and systematic description need constructive critique

The project's documentation — mission docs, architecture docs, doctrine,
agent operating contracts — has accumulated through multiple major
migrations without a composition and clarity pass. Docs may describe a
prior architecture, use vocabulary from a prior model, or carry
assumptions that no longer hold. The systematic description of the
system (how components connect, what the data flow is, what the
invariants are) may be stale in the same way the TLA+ specs are.

**Implication:** As part of stabilization, review the documentation for
composition quality and clarity of communication. This is not a refactor
— it's a releveling of how the system is described, matching the current
architecture. The docs are the handoff artifact for tomorrow's scale-up;
if they describe a prior system, the scale-up will be built on stale
understanding.

### Approach

All three definitions point toward **simplify**. The system has been
through major migrations. The verification infrastructure (race
detector, TLA+) and the documentation (mission docs, architecture docs)
are carrying assumptions from prior architectures. The constructive
approach is to review and relevel them to match the current system, not
to patch symptoms or carry stale verification.

The first-pass fix (event polling for the flaky test) is still worth
attempting — it's cheap and may unblock CI immediately. But if it fails
or another race test flakes, the next move is D1 (review the race
detector model), not patching the next test. And the documentation
critique (D3) should run in parallel with the stabilization work, not
after it.

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
  pass. Do not skip the race detector without an explicit releveling of
  what it should test.
- Q: CI must be genuinely green (not flaky-green). Staging must be
  genuinely healthy (not degraded). Articles must be genuinely
  LLM-synthesized (not template prose). Documentation must describe the
  current architecture, not a prior one.
- D: local test fix → CI green → staging recovered → Wire publishing →
  scale-up ready. Each domain embeds in the next. D1/D2/D3 inform the
  approach throughout, not conditionally.

substrate definitions (releveled from conjecture, established by
observation of architecture history):
- D1: The race-detector CI model is from a prior architecture. The full
  runtime test suite under -race surfaces test-infrastructure bugs from
  the prior concurrency model, not production data races. The event-
  polling pattern is likely systemic, not a one-off.
- D2: The TLA+ specs are from a prior architecture. They model a system
  that has since undergone OG migration, actor runtime migration, and
  wire pipeline rewrite. Without review, the TLA+ check in CI is stale
  verification theater.
- D3: The documentation and systematic description need constructive
  critique. Docs have accumulated through multiple migrations without a
  composition and clarity pass. They may describe a prior architecture
  and carry stale vocabulary and assumptions.

variant (conjecture descent) V: count conjectures about the pipeline's
stability and publishing capability. D1/D2/D3 are definitions, not
conjectures — they don't count toward V but they inform move selection.
Current: 3.
- C1: The flaky test is a test bug (event polling race), not a production
  data race (SUPPORTED — D1 audit confirmed 8 instances across 4 files;
  local -race tests pass with fix)
- C2: Fixing the event polling in the test makes CI green (SUPPORTED —
  CI run 28644141785: all 21 jobs passed including race detector shard 2)
- C3: Staging VMs can be recovered by re-running the deploy (UNDICIDED —
  workflow_dispatch deploy triggered, monitoring)
- C4: Sourcecycled is cycling and dispatching processor runs on staging
  (STRUCTURALLY SUPPORTED — sourcecycled health is ok, but runtime is
  down so dispatch may be failing)
- C5: The agent pipeline produces real LLM-synthesized articles on staging
  (UNDICIDED — requires healthy runtime + model calls)
Target: 0.

budget: 15-25 passes. Pass 2 spent. 13-23 remaining. Solvent.

D3 findings: canonical docs are current; historical mission docs have
appropriate stale vocabulary. No HIGH-severity documentation issues. D3
is effectively scoped — no releveling needed for canonical docs.

authority / bounds: may modify test files, CI workflow, deploy scripts,
TLA+ specs, documentation. May push to origin/main. May trigger
workflow_dispatch. May investigate staging via API. May not touch
Texture core, O1-O3, or delete agent pipeline code. May not SSH to
Node B directly (no access).

mutation class / protected surfaces:
- Green/Yellow: test fixes, documentation critique (no runtime behavior
  change)
- Orange: CI workflow changes, deploy script changes, TLA+ spec changes
- Red: staging deploy, VM lifecycle
- Protected: Texture revision creation, corpusd sync contract, source
  entity graph, agent pipeline code.

evidence packet:
- Flaky test failure logs (CI run 28642495032, shard 2)
- Local test pass without -race (this session)
- CI green status (pending)
- Staging health status (pending)
- Wire feed with real articles (pending)
- Documentation critique findings (pending)

heresy delta: `discovered` for the flaky test race (test bug, not
production race). `repaired` for H-WIRE-SOFT-GUARDRAIL if articles are
publishing. `discovered` for any VM boot issues from OG migration.
`discovered` for D1/D2/D3 (substrate-level CI/verification/documentation
debt — these are definitions, not new findings, but recording them is
epistemic progress).

position / live conjectures / open edges:
Pass 2 complete. CI is fully green — all 21 jobs passed including race
detector shard 2 (the previous blocker). C2 is SUPPORTED. The test fix
(commit 2dcee27e) used a shared `waitForEvents` helper applied to all
8 HIGH-risk instances. D1 confirmed the pattern was systemic; the fix
addresses it structurally rather than patching individually. D3 found
canonical docs are current. A workflow_dispatch has been triggered to
force staging deploy (the test-only commit correctly skipped deploy).
The open edge is C3: will staging VMs recover?

next move: Monitor the workflow_dispatch deploy. If staging VMs recover,
C3 is SUPPORTED and we move to C4/C5 (Wire publishing verification).
If VMs fail again, investigate the boot failure — may need to increase
the VM refresh timeout or check for OG migration startup issues.

ledger file: `docs/mission-universal-wire-stabilization-v1.ledger.md`

version / lineage: v1. Depends on
`docs/mission-universal-wire-agent-pipeline-v1.md` (settled, prompt fix
done). Successor: scale-up mission (to be created tomorrow).

learning state: retained here / promoted outward / successor links

settlement: settled when CI is green, staging is healthy, at least one
real LLM-synthesized article is on the Wire feed, and the documentation
critique (D3) has been started or scoped. Open handoff if VMs cannot be
recovered (document the boot failure for operator intervention). If D1
or D2 reveal that the CI/verification infrastructure needs releveling,
the mission may split into a CI/verification-infrastructure mission
before continuing.

## Suggested Goal String

```text
Use Parallax on docs/mission-universal-wire-stabilization-v1.md. Mission: stabilize the Universal Wire pipeline so it is live, functional, and publishing real LLM-synthesized articles on staging. Four phases: (1) Fix the flaky race-detector test TestToolLoopEndToEndWithRuntime — it checks for EventRunCompleted immediately after run state becomes terminal, but event persistence is async; add event polling. (2) Push, monitor CI, verify all jobs green including race detector. (3) Recover staging VMs — they timed out during deploy (guest did not become healthy in 3m); re-run deploy via workflow_dispatch or investigate VM boot failure. (4) Verify Universal Wire is publishing: sourcecycled cycling, processor runs dispatching, Texture agents synthesizing with source body text, articles on Wire feed with event-grade headlines. This sets up tomorrow's scale-up of sources and stories. Substrate definitions (releveled from conjecture, established by observation): D1 — the race-detector CI model is from a prior architecture (full runtime suite under -race surfaces test-infrastructure bugs from the prior concurrency model, not production data races; the event-polling pattern is likely systemic); D2 — the TLA+ specs are from a prior architecture (written before OG migration, actor runtime migration, and wire pipeline rewrite; without review they are stale verification theater); D3 — the documentation and systematic description need constructive critique (docs have accumulated through multiple migrations without a composition and clarity pass; they may describe a prior architecture and carry stale vocabulary). These are definitions, not hypotheses — they inform the approach from the start. If C2 is falsified (another race test flakes), shift to D1: review the race-detector CI model rather than patching the next test. Run D3 documentation critique in parallel with stabilization. Default direction: simplify, not patch. Invariants: no deterministic synthesis, no story caps, no source-label headlines, do not touch Texture core or O1-O3, do not weaken CI or skip race detector without explicit releveling. Budget: 5-8 passes. Exit: settled when CI green + staging healthy + one real article on Wire feed + documentation critique started or scoped.
```
