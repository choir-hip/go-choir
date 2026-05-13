# MissionGradient: Run Acceptance Verification v0

Status: proposed verification mission
Date: 2026-05-13

## Real Artifact

A durable Run Acceptance System for Choir self-development runs.

The artifact is not a Playwright test, a Trace screen, a VText summary, or a set of Go unit tests. Those are projections. The real artifact is an auditable acceptance record that proves a long-running Choir run moved through the intended production state machine with bounded authority, observable evidence, rollback semantics, and no product-path bypasses.

The acceptance system should make this question answerable:

```text
Did Choir actually perform this run through the intended product/control path, and can a skeptical reviewer replay enough evidence to trust the result?
```

The target state is a verifier that can prove, at increasing realism:

```text
prompt/objective -> VText/appagent -> super -> vmctl/candidate world -> worker export
-> promotion candidate -> verifier contract -> owner decision -> promotion/rollback
-> compaction/continuation
```

The initial low-resolution slice may prove only a prefix of that chain, but it must preserve the same event semantics, state ownership, trust boundaries, and evidence model as the full system.

## Staging-First Execution Contract

From this mission forward, staging is the default execution environment for meaningful Choir self-development verification.

Local development is allowed only for:

- fast frontend visual/design iteration;
- narrow unit tests that shape code before deployment;
- local reproduction of a deployed failure when staging evidence already identifies the failing transition.

Local development is not acceptance evidence for:

- vmctl behavior;
- live background/candidate VMs;
- gateway credential flow;
- live language model calls;
- search/provider integrations;
- promotion, rollback, or deployed auth/session behavior;
- Choir-in-Choir product-path claims.

Every mission that changes behavior must include the landing loop as part of the mission identity:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

A run is not complete merely because local tests pass or files changed. It is complete only when the deployed staging system at `draft.choir-ip.com` is running the intended commit and the relevant deployed acceptance proof passes, or when an invariant-level blocker is documented with evidence and rollback status.

## Invariants

- Verification observes the product/control path; it does not create a parallel test-only success path.
- Staging is the acceptance environment. Local evidence can support implementation, but cannot satisfy VM/gateway/model/promotion claims.
- Every behavior-changing mission commits, pushes to `origin/main`, monitors CI, monitors the Node B staging deploy, and verifies the deployed commit before claiming success.
- Browser tests may use the public authenticated product API, but must not call browser-public internal orchestration routes such as `/api/agent/*`, `/api/prompts`, `/api/test/*`, or raw event mutation endpoints.
- The foreground desktop and canonical repo are not mutated by worker/vsuper actions.
- Candidate mutation happens in a bounded worker world. Staging acceptance requires live deployed vmctl/candidate behavior unless the mission explicitly documents a staging blocker and proves only a fallback-labeled diagnostic.
- Every accepted run has a durable run id, trajectory id, owner/user id, source objective, authority profile, lease/timeout, state-machine checkpoints, evidence references, verifier contracts, and final decision.
- Trace events and acceptance checkpoints must agree on causal order. UI text alone is not proof.
- VText may present progress, but the verifier must not trust unconstrained model prose unless it matches structured trace/export/promotion evidence.
- Long-running verification must renew auth/session state through the normal product auth path.
- Duplicate worker requests, exports, or agent messages must be represented as recurrence/portfolio evidence, not hidden by permissive assertions.
- Failures are first-class accepted outcomes when they preserve diagnostics, rollback state, and the next safe probe.
- Deployment proof is required before claiming Node B readiness.

## Value Criterion

Maximize trustworthy evidence that Choir can develop Choir through its own product path while minimizing:

- false positives from LLM summaries or generic UI text;
- bypasses around appagent/super/vmctl/promotion boundaries;
- unobserved state transitions;
- brittle timing assertions;
- hidden auth/session expiry failures;
- duplicate-work noise;
- human log-reading burden;
- canonical-state corruption risk;
- verifier Goodharting.

Better verification means lower divergence between the intended run state machine and the observed trace/store/UI/deploy evidence.

## Acceptance Record

Introduce or synthesize a durable `RunAcceptanceRecord` as the canonical verifier object.

Suggested shape:

```text
acceptance_id
target_mission_id
source_prompt_or_objective
user_id
desktop_id
trajectory_id
run_id
authority_profile
base_sha
deployment_commit
state
checkpoints[]
invariant_checks[]
verifier_contracts[]
evidence_refs[]
rollback_refs[]
failure_or_residual_risks
created_at
updated_at
```

Suggested checkpoint kinds:

```text
submitted
vtext_opened
super_requested
worker_leased
worker_delegated
export_observed
promotion_candidate_queued
verification_started
verification_passed
owner_reviewed
promoted
rollback_available
compacted
continued
failed_recoverably
```

Each checkpoint should cite evidence: trace moment id, event id, API response, patchset manifest, git SHA, VM id, screenshot, or verifier output.

## Homotopy Parameters

Increase realism continuously along these axes:

- Frontend visual local loop -> deployed Node B product acceptance.
- Local unit shaping -> CI proof -> staging deploy proof -> deployed acceptance proof.
- Stub/local-worktree diagnostic fallback -> live deployed vmctl/background VM worker.
- Single worker -> duplicate-request control -> explicit parallel worker portfolio.
- Marker patch -> product-visible patch -> semantic app patch.
- Prompt-driven run -> mission-driven continuation -> controller-selected next objective.
- VText summary proof -> VText dashboard backed by structured acceptance records.
- Trace-only proof -> Trace plus store plus artifact plus UI proof.
- Export-only proof -> promotion queue -> verifier contract -> explicit owner decision -> promotion/rollback.
- Happy path -> auth expiry -> worker failure -> verifier failure -> rollback/retry.
- Single run -> chained runs with compaction and continuation.

At every lower-resolution point, preserve the same state-machine labels, ownership boundaries, evidence references, and failure semantics used by the higher-resolution system.

## Dense Feedback Channels

- Go tests for acceptance record state transitions, idempotency, checkpoint causal order, and invariant checks.
- Store tests proving acceptance/checkpoint records are durable, owner-scoped, append-only where appropriate, and queryable by trajectory/run/mission.
- Runtime tests that synthesize acceptance records from existing Trace/tool events without requiring a separate success path.
- Playwright tests that submit through the visible prompt bar and verify visible VText/Trace progress against structured acceptance records.
- GitHub Actions status for the pushed `origin/main` commit, including frontend build, Go vet/test/build, and staging deploy jobs.
- Staging health/build identity checks proving proxy and sandbox are running the pushed commit.
- Deployed Playwright tests that prove auth renewal, Node B build identity, prompt/VText/gateway path, vmctl worker lease, worker export, and concrete patchset evidence.
- Trace assertions for roles, tool calls, worker ids, export paths, base SHA, worker head SHA, verifier results, promotion events, and recurrence controls.
- Git assertions for foreground base SHA, worker head SHA, dirty-state blockers, integration branch/promotion state, and rollback refs.
- VM assertions for worker lease identity, sandbox URL, live/fallback mode, expiry, and disposal/recovery.
- UI screenshots only where they show dashboard state that is backed by structured evidence.

## Forbidden Shortcuts

- Do not stop after local tests when the mission touches runtime, vmctl, gateway, model calls, worker/candidate worlds, promotion, auth, or deployed UX.
- Do not defer commit/push/CI/deploy verification as a follow-up for behavior-changing work.
- Do not prove success by checking that VText contains generic words like "verified", "worker", or "patchset".
- Do not manually seed acceptance records in product-path tests.
- Do not call internal browser-public agent/test APIs from Playwright to skip the prompt/appagent/super path.
- Do not replace vmctl/worker/export behavior with mocks that cannot deform into live background VM behavior.
- Do not let Codex direct edits count as Choir-in-Choir work unless the same delta is observed through candidate-world/promotion evidence.
- Do not mark a timeout as success because some partial UI appeared.
- Do not hide duplicate worker requests by selecting the first convenient export; record and explain recurrence.
- Do not make the dashboard a freeform prose summary with no structured evidence links.
- Do not claim deployed readiness from local-only proof.
- Do not use local dev server behavior to infer staging VM/gateway/model behavior.

## Rollback Policy

Git:

- Acceptance records must capture base SHA, worker head SHA, destination commit when promoted, and rollback command/ref.
- The mission's own implementation commits must land on `origin/main` and record the pushed SHA used for staging verification.
- Promotion remains explicit and blocks dirty/diverged canonical state.

VM:

- Worker lease identity and expiry are recorded.
- Failed candidate worlds are discardable or archived with diagnostics.
- Live VM rollback remains unclaimed until proven on Node B.

Database/runtime:

- Acceptance records and checkpoints are append-only or state-machine constrained.
- Repeated checkpoints are idempotent by deterministic identity.
- Auth/session renewal failures become verifier failures with evidence, not silent flake.

Product:

- VText/Trace dashboards render acceptance state from structured records.
- UI actions never mutate acceptance state except through owner-mediated review/promotion flows.
- `draft.choir-ip.com/health` must match the pushed commit before deployed product-path verification is considered valid.

## Learning Side-Channel

Every run should emit a verifier learning packet when it discovers one of:

- missing checkpoint visibility;
- ambiguous trace causality;
- duplicate work recurrence;
- auth/session renewal gap;
- worker/provisioning failure;
- promotion/rollback uncertainty;
- dashboard evidence mismatch;
- verifier assertion that was too permissive or too brittle.
- local-only success that fails or becomes ambiguous on staging;
- CI/deploy latency or failure mode that changes what the acceptance system must observe.

Classify discoveries:

- Tactical learning: patch the verifier/test/runtime directly.
- Target-level learning: update this mission doc or the acceptance record schema.
- Invariant-level learning: stop and ask before changing authority boundaries, promotion semantics, rollback meaning, or what counts as proof.

## Stopping Condition

Stop only when one of these is true:

- The implementation is committed and pushed to `origin/main`, CI and Node B deploy are green, `draft.choir-ip.com/health` reports the pushed commit, and a deployed Node B acceptance test proves at least one real product-path coding run from prompt/objective through VText, super, vmctl worker, export, concrete dashboard evidence, and recoverable final state; or
- the run reaches an invariant-level blocker and records the failed transition, evidence, rollback status, and next smallest safe probe.

Completion requires a final report naming:

- accepted run id / trajectory id;
- deployment commit;
- pushed commit and CI/deploy run id;
- verifier contracts run;
- evidence refs;
- residual risks;
- the next realism axis to increase.

## One-Line Goal

`/goal Use MissionGradient. Complete docs/mission-run-acceptance-verification-v0.md by building and landing a durable staging-first run acceptance verifier for Choir self-development runs, committing and pushing to origin/main, monitoring CI/deploy, proving the deployed product path with structured evidence instead of brittle UI/prose assertions, and stopping only on verified staging acceptance or an invariant-level blocker.`
