# MissionGradient: Run Acceptance Verification v0

Status: proposed documentation-and-verification mission
Date: 2026-05-13

## Real Artifact

A coherent operational documentation set plus a durable Run Acceptance System for Choir self-development runs.

The artifact is not a Playwright test, a Trace screen, a VText summary, a README, or a set of Go unit tests. Those are projections. The real artifact is a repo that teaches future agents and humans how Choir is actually operated, and an auditable acceptance record that proves a long-running Choir run moved through the intended staging state machine with bounded authority, observable evidence, rollback semantics, and no product-path bypasses.

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

## Mission Phases

This is one mission with four coupled phases. They should be executed in order unless evidence shows a different order preserves the same invariants better.

1. Documentation review, update, and cleanup:
   - inventory `README.md`, `AGENTS.md` if present, top-level docs, mission docs, architecture docs, proof reports, and stale milestone docs;
   - identify canonical docs versus historical/proof docs;
   - update stale claims about architecture, deployment, staging, local dev, vmctl, gateway, VText, Trace, super/vsuper/cosuper, promotion, and Choir-in-Choir;
   - avoid deleting historical proof artifacts unless they are clearly junk or duplicated; prefer index/label/redirect over destructive cleanup.

2. README rewrite:
   - rewrite `README.md` as the current human entrypoint;
   - describe what Choir is now, what runs in staging, what local dev is for, what the services do, how deployment works, and how to verify behavior;
   - remove outdated milestone narration or move it behind links to historical docs;
   - make the staging-first operating model obvious to a new contributor or agent in the first screen.

3. Agent operating contract:
   - create or update repository-level `AGENTS.md`;
   - encode staging-first rules, docs-only CI exception, behavior-changing landing loop, MissionGradient usage, local-dev limits, no-bypass rules, and required final evidence;
   - make it clear that docs-only commits may be pushed without automatic CI/deploy, while behavior-changing work must be committed, pushed, deployed to staging, and verified there.

4. Run acceptance verification:
   - build the durable acceptance verifier described below;
   - prove it on staging against a real product-path Choir self-development run;
   - use the updated README and `AGENTS.md` as operational inputs, not side notes.

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

Documentation-only changes are different:

- docs-only commits should not trigger automatic CI/deploy;
- docs-only commits may still be committed and pushed to `origin/main`;
- if a docs-only change must be verified by CI, use an explicit manual workflow dispatch when available or run the specific check directly;
- do not remove the docs-only `paths-ignore` behavior just to force CI;
- when docs become runtime-operational inputs, the next behavior-changing deploy must verify that staging has consumed the expected docs/commit state.

## Invariants

- Verification observes the product/control path; it does not create a parallel test-only success path.
- Staging is the acceptance environment. Local evidence can support implementation, but cannot satisfy VM/gateway/model/promotion claims.
- Every behavior-changing mission commits, pushes to `origin/main`, monitors CI, monitors the Node B staging deploy, and verifies the deployed commit before claiming success.
- Documentation-only commits are exempt from automatic CI/deploy, but the resulting docs must still be coherent enough to steer future behavior-changing staging missions.
- `README.md` and `AGENTS.md` must not contradict the mission docs, current staging reality, or CI/deploy behavior.
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
ci_run_id
deploy_run_id
staging_url
health_commit
acceptance_level
vm_mode
gateway_provider_evidence
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

## Acceptance Levels

Use explicit acceptance levels so the system does not overclaim.

```text
docs-level
  Documentation, README, and AGENTS.md are coherent and committed.

staging-smoke-level
  CI/deploy is green, staging health reports the pushed commit, auth shell and prompt/VText smoke pass.

export-level
  A deployed product-path run reaches VText -> super -> vmctl worker -> worker export,
  and structured evidence links the final dashboard/document state to concrete VM/export/git facts.

promotion-level
  Export-level plus promotion candidate, verifier contract, owner decision, promotion or rollback evidence.

continuation-level
  Promotion-level plus compaction/run-memory and bounded next objective continuation evidence.
```

The v0 verifier may stop at `export-level` if promotion is not yet product-ready, but it must label the result as export-level acceptance and record the missing transition to promotion-level acceptance.

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
- Documentation review evidence: list of docs reviewed, docs updated, docs marked historical, and unresolved contradictions.
- README review evidence: README matches current staging architecture, local-dev limits, deploy flow, and verification contract.
- `AGENTS.md` review evidence: agent rules match the staging-first operating model and docs-only CI exception.
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
- Do not force CI/deploy for docs-only commits by weakening the workflow path filters.
- Do not rewrite README as marketing copy; it should be an operational entrypoint.
- Do not create `AGENTS.md` as generic Codex boilerplate; it must encode Choir-specific staging, authority, verification, and deployment rules.
- Do not hide stale docs by ignoring them; label, update, index, or explicitly defer them.
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
- Documentation-only commits may land without CI/deploy, but behavior-changing commits must include CI/deploy evidence.
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

- stale or contradictory documentation;
- README instructions that do not match staging reality;
- agent instructions that would cause local-only verification or delayed pushing;
- missing checkpoint visibility;
- ambiguous trace causality;
- duplicate work recurrence;
- auth/session renewal gap;
- worker/provisioning failure;
- promotion/rollback uncertainty;
- dashboard evidence mismatch;
- verifier assertion that was too permissive or too brittle;
- local-only success that fails or becomes ambiguous on staging;
- CI/deploy latency or failure mode that changes what the acceptance system must observe.

Classify discoveries:

- Tactical learning: patch the verifier/test/runtime directly.
- Target-level learning: update this mission doc or the acceptance record schema.
- Invariant-level learning: stop and ask before changing authority boundaries, promotion semantics, rollback meaning, or what counts as proof.

## Stopping Condition

Stop only when one of these is true:

- Documentation review/cleanup is complete enough that canonical docs, historical docs, and unresolved contradictions are distinguishable; `README.md` has been rewritten as the current operational entrypoint; repository-level `AGENTS.md` exists and encodes the staging-first operating contract; the behavior-changing verifier implementation is committed and pushed to `origin/main`; CI and Node B deploy are green for that behavior-changing commit; `draft.choir-ip.com/health` reports the pushed commit; and a deployed Node B acceptance test proves at least `export-level` acceptance for one real product-path coding run from prompt/objective through VText, super, vmctl worker, export, concrete dashboard evidence, and recoverable final state, with any missing promotion/continuation transitions explicitly labeled; or
- the run reaches an invariant-level blocker and records the failed transition, evidence, rollback status, and next smallest safe probe.

Completion requires a final report naming:

- accepted run id / trajectory id;
- acceptance level reached;
- docs reviewed and docs updated;
- README rewrite summary;
- `AGENTS.md` operational rules added;
- deployment commit;
- pushed commit and CI/deploy run id;
- verifier contracts run;
- evidence refs;
- residual risks;
- the next realism axis to increase.

## One-Line Goal

`/goal Use MissionGradient. Complete docs/mission-run-acceptance-verification-v0.md as one staging-first mission: review and clean up the docs, rewrite README.md as the operational entrypoint, create/update AGENTS.md with Choir's agent operating contract, then build and land a durable run acceptance verifier, committing and pushing behavior changes to origin/main, monitoring CI/deploy, proving the deployed product path with structured evidence, and stopping only on verified staging acceptance or an invariant-level blocker.`
