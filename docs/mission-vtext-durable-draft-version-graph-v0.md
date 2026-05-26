# MissionGradient: VText Durable Draft Version Graph

**Status:** draft
**Date:** 2026-05-25
**Target environment:** staging, `https://draft.choir-ip.com`

## Goal

Make VText direct editing, multi-device sync, autosave, worker-update synthesis,
and agent-authored revisions converge on a clear version-graph contract: user
text becomes durable before `Revise`, VText remains the single agent-side
canonical writer, no local user draft is silently lost, and head-move
concurrency produces explicit merge/rebase behavior instead of accidental
overwrite or opaque conflict failure.

## Key Deliverables

The mission deliverable is not only a design or test harness. It is:

- working product behavior deployed to staging;
- a staged acceptance/eval report over the full scenario matrix;
- a model-suite comparison report for VText/researcher/super coordination using:
  - `fireworks-kimi-k2p6-low`;
  - `fireworks-deepseek-v4-flash-medium`;
  - `chatgpt-gpt-5-5-low`;
- enough Trace, revision, stream, and screenshot/video evidence for a skeptical
  reviewer to inspect failures and compare model behavior;
- a concise final recommendation on which model-policy shape is acceptable for
  this VText durable-draft workload, including residual risks.

## Real Artifact

The artifact is the production VText version graph and live editing runtime:

- immutable user-authored and agent-authored document versions;
- durable user draft sync from browser to the user's computer VM;
- document stream semantics across devices;
- VText appagent synthesis over known base revisions;
- worker/researcher/super updates as queued synthesis inputs;
- conflict, merge, and rebase behavior when canonical head changes while a
  user draft or VText run is in flight.

This is not primarily an editor-widget task, an autosave task, a WebSocket task,
or a model-comparison task. Those are implementation and evaluation dimensions
inside the version-graph artifact.

## Invariants

- User text is never silently overwritten by an incoming VText version, stream
  event, autosave response, reload, or cross-device update.
- Direct user editing syncs durably to the user's computer VM before `Revise`.
  `Revise` is a semantic instruction to engage VText, not the first durable save.
- Every durable user draft or user-authored version records its base revision,
  actor, timestamp, content or edit payload, and enough metadata to explain its
  relationship to canonical head.
- VText is the only agent-side canonical text writer. Researchers, supers,
  vsupers, and cosupers produce updates, evidence, artifacts, and findings, not
  canonical document patches.
- VText agent writes are based on an explicit base revision. A stale VText write
  must fail safely, re-run against the newer head, or create an explicit
  merge/rebase path; it must not overwrite newer user work.
- Worker updates are queued and consumed through the VText controller checkpoint.
  Concurrent worker deliveries may be batched, but their consumption/pending
  status must be visible in revision metadata or Trace.
- Clean editor views may auto-follow a new canonical head. Dirty editor views
  must preserve local text and enter an explicit "new version available" or
  merge/rebase state.
- Cross-device state must use product paths and persistent computer state, not
  browser-only local state as the source of truth.
- Browser-public tests may use authenticated product APIs such as `/api/vtext/*`,
  `/api/trace/*`, and `/api/prompt-bar/*`, but must not bypass behavior through
  `/api/agent/*`, `/api/test/*`, `/internal/*`, or raw event mutation endpoints
  for acceptance proof.
- Platform behavior changes require the landing loop: commit, push `origin/main`,
  monitor CI, monitor staging deploy, verify deployed commit identity, and run
  deployed acceptance proof.

## Value Criterion

Minimize divergence between the observed VText event/version graph and the
intended graph while preserving the invariants.

Better states have:

- lower probability of lost or hidden user text;
- lower probability of stale agent output materializing after newer user edits;
- clearer version lineage for user drafts, user versions, agent versions, and
  merged/rebased versions;
- lower cross-device divergence between durable VM state and open editors;
- denser evidence for worker-update batching, pending status, and consumption;
- fewer ambiguous 409/conflict dead ends in real product workflows;
- lower coordination noise from redundant tools, stale writes, and hidden retries.

Penalize:

- bypasses that prove behavior through non-product APIs;
- local-only proof for VM persistence, multi-device sync, auth/session renewal,
  model calls, or staging behavior;
- UI copy that hides a lost-draft or stale-head bug;
- tests that assert only eventual text presence without proving causality,
  lineage, and actor authority;
- transport-driven design that picks WebSocket, SSE, or POST before defining the
  state contract.

## Quality Gradient

Expected quality level: **solid**.

A solid result has:

- a documented version-graph contract for direct editing, durable drafts,
  canonical versions, and VText synthesis;
- working implementation deployed to staging for the accepted contract;
- focused runtime/frontend tests for the contract;
- staging Playwright/API acceptance over real product paths;
- a model-suite eval report across Kimi low, v4-flash medium, and GPT-5.5 low;
- clear Trace/revision metadata evidence for concurrency cases;
- no hidden parallel write path for appagent text;
- no known local draft loss path in covered workflows;
- a resumable mission checkpoint if full merge/rebase polish is not reached.

Substandard work:

- only adding a "New version available" label without durable draft semantics;
- treating backend `409` as the product solution for normal editing races;
- autosaving only to browser local storage;
- adding WebSocket plumbing without explicit base-revision and merge semantics;
- proving with dry-run or test-only routes when staging product proof is required;
- expanding to a full collaborative editor without first protecting the VText
  single-writer/version graph invariants.

## Homotopy Parameters

Increase realism along these axes while preserving the same product topology:

- content length: short note -> long multi-section document -> many-version
  research/coding narrative;
- edit entropy: one local edit -> edits in distant sections -> overlapping edits
  over agent-changed text;
- version count: v0/v1 -> dozens of user and agent versions;
- actor concurrency: one browser -> two browser sessions/devices -> user edit
  plus VText in-flight plus worker updates;
- worker concurrency: one researcher update -> several researcher/super updates
  before VText wake -> updates arriving while VText is active;
- transport realism: current POST/SSE flow -> durable draft sync protocol ->
  optional WebSocket if it simplifies live draft replication without changing
  semantics;
- proof realism: unit tests -> local product integration -> deployed staging
  product-path Playwright/API acceptance;
- failure realism: happy path -> stale head -> auth renewal during long run ->
  reload/reconnect while dirty.

## Starting Belief State

Current believed state:

- User revision creation is author-locked to authenticated user identity.
- Store writes reject stale parent revisions.
- Stale VText `edit_vtext` after a user edit fails safely and does not create a
  canonical appagent revision.
- The frontend currently autosaves direct edits after a short delay and creates
  user revisions before explicit `Revise`.
- Clean editors auto-follow new heads; dirty editors preserve local text and
  show "New version available".
- Worker updates are serialized through a VText controller checkpoint and can be
  batched into one VText synthesis run.

Evidence for this belief:

- Runtime/store tests around user revision authorship, stale head rejection, and
  stale agent write rejection.
- Frontend tests for clean auto-follow and dirty no-clobber behavior.
- Runtime tests for worker-update batching, pending late updates, and checkpoint
  advancement.
- Recent staging acceptance for VText/researcher/super cadence and terminal
  tool-loop behavior.

Main uncertainties:

- Whether autosave creates too many canonical user versions rather than durable
  draft records with intentional version promotion.
- Whether a dirty editor based on revision `A` can survive and merge/rebase when
  VText creates head `B`.
- Whether "New version available" currently lets users accidentally discard a
  dirty local draft.
- Whether cross-device sessions observe durable draft state before `Revise`.
- Whether long documents with distant edits and concurrent worker updates remain
  coherent under VText synthesis.
- Whether WebSocket is necessary, or whether the right first cut is a durable
  draft resource plus existing stream events.

Highest-impact uncertainty:

What should be the durable state model between keystrokes and canonical
user-authored versions: canonical autosave revisions, separate draft records, or
branch-like user draft heads that can be promoted/rebased?

Next observation that reduces uncertainty:

Run product-path probes that create a dirty draft on one browser session, create
a newer canonical VText/user head from another actor while the draft is dirty,
then observe whether the original draft is persisted, visible on another device,
and safely mergeable or recoverable without text loss.

## Investigation & Cognitive Reframing

Before accepting a blocker, run a root-cause loop:

1. Identify which edge failed: draft persistence, stream delivery, head update,
   stale-write guard, merge/rebase, VText synthesis, worker checkpoint, or UI
   state transition.
2. Inspect product evidence: revision history, document stream events, Trace,
   frontend console, network responses, revision metadata, and staging health.
3. Form a narrow hypothesis and add the smallest focused test or instrumentation
   that distinguishes it.
4. Patch only the implicated layer if the root cause is clear.
5. Re-run the focused proof, then the broader staging acceptance that covers the
   changed topology.

Tactical blockers that should trigger another autonomous probe:

- a stale-head 409 with recoverable draft content;
- missing stream event after a persisted version;
- duplicate or skipped worker-update metadata;
- UI state mismatch between dirty flag and current revision;
- auth expiry during a long Playwright observer, if renewal can be integrated.

Invariant-level or external blockers requiring escalation:

- a proposed fix lets non-VText agents write canonical text;
- a proposed fix makes browser local state the only copy of user text;
- a proposed fix bypasses product APIs for acceptance;
- a deploy/auth/provider issue prevents staging proof and cannot be isolated
  through existing product health and logs.

Apply these cognitive transforms before declaring a hard blocker:

- **Real Object Transform:** reframe the bug as version graph divergence, not UI
  awkwardness.
- **Transport/Semantics Split:** prove the state contract before selecting POST,
  SSE, WebSocket, or patch streaming.
- **Branch Instead Of Conflict Error:** treat dirty user text over a moved head
  as a branch needing rebase/merge, not an invalid request.
- **Value Of Information:** choose the next eval that distinguishes text loss,
  lineage loss, stale-write safety, and UI affordance failures.

If a blocker defines an executable next probe inside the current authority
boundary, run that probe instead of ending.

## Receding-Horizon Control

Work in short control intervals:

1. Read the current contract and implicated tests.
2. Pick one narrow concurrency edge.
3. Predict the expected version graph and stream/Trace evidence.
4. Run or write a focused test/probe.
5. Compare observed graph to expected graph.
6. Update the mission doc belief state if the observation changes the model.
7. Implement the smallest contract-preserving change.
8. Verify locally where appropriate, then on staging for behavior-changing work.

Mutation radius:

- Documentation/eval-only passes may touch mission docs and test probes.
- Runtime/frontend changes must stay scoped to VText draft/version/stream
  behavior and directly required tests.
- Do not redesign the whole editor or add general collaborative editing unless
  evidence shows the narrower version-graph contract cannot support the mission.

## Dense Feedback Channels

Use feedback that reveals local error:

- Go tests for `CreateRevision`, stale parent rejection, VText stale edit
  rejection, worker-update consumed/pending metadata, and any new draft/merge
  store contract.
- Frontend tests for dirty editor preservation, clean auto-follow, draft sync,
  reload/reconnect, and "show latest" behavior.
- Playwright product-path probes against staging for:
  - direct edit persists before `Revise`;
  - cross-device/session sees durable draft or canonical saved state;
  - VText in-flight head change while user is dirty;
  - multiple worker updates before and during VText run;
  - long document with distant edits.
- Revision history assertions: actor, parent revision, metadata, content hash,
  version count, and current head.
- Stream/Trace assertions: snapshot, head_changed, synth_started/completed,
  revision_created, worker update pending/consumed.
- Browser screenshots/video only when UI affordance or no-clobber behavior needs
  visual proof.

## Evidence Ledger

For every nontrivial claim, record:

```text
claim:
evidence source:
command or observation:
artifact path:
result:
uncertainty/caveat:
promotion relevance:
```

Claims requiring ledger entries include:

- user draft persists before `Revise`;
- dirty local text survives an incoming VText head;
- stale VText write cannot overwrite a newer user edit;
- cross-device state converges;
- worker updates are batched or marked pending/consumed correctly;
- long-content multi-section edits remain coherent;
- each model in the suite was run over the same scenario matrix, with model
  identity, reasoning level, latency, revision count, tool errors, duplicate
  side effects, stale-write failures, consumed/pending worker updates, and final
  content-quality notes recorded;
- staging is running the commit under test.

## Forbidden Shortcuts

- Do not use `/api/test/*`, `/api/agent/*`, `/internal/*`, or raw event mutation
  endpoints for acceptance proof.
- Do not manually seed success records, revision metadata, worker checkpoints, or
  run acceptance records.
- Do not claim VM/cross-device persistence from browser local storage.
- Do not patch Node B tracked files or environment variables as a substitute for
  product/runtime policy.
- Do not let researchers, supers, or test harnesses write canonical VText text.
- Do not turn stale-head errors into success by suppressing errors in the UI.
- Do not add a WebSocket path that bypasses the same auth, owner scoping,
  base-revision, and event semantics used by product APIs.
- Do not call a checkpoint complete if merge/rebase or dirty-draft preservation
  remains unproven.

## Rollback Policy

- Use normal git rollback for code changes; never revert unrelated user or
  other-agent changes.
- For behavior-changing commits, record the pre-change commit SHA and the pushed
  commit SHA.
- Push through `origin/main`, monitor CI, monitor staging deploy, verify staging
  build identity, and run deployed acceptance proof.
- Preserve created VText documents/revisions as evidence artifacts; do not
  delete staging state to hide failed attempts.
- If a deployed change risks user text loss, stop and rollback before continuing
  speculative improvements.

## Learning Side-Channel

Tactical learning:

- update tests, mission checkpoint, and implementation notes.

Target-level learning:

- update this mission doc's belief state, homotopy parameters, and stopping
  condition; propose a reparameterization before continuing.

Invariant-level learning:

- stop and escalate before changing writer authority, product-path proof,
  persistence ownership, or deployment boundaries.

Canonical docs to update only when behavior changes:

- `docs/architecture.md` for current VText draft/version contract;
- `docs/runtime-invariants.md` for agent authority or worker-update invariants;
- `docs/glossary.md` for durable draft, user-authored version, merge/rebase, or
  Revise terminology.

Run details belong in this mission doc, test artifacts, Trace, and final reports.

## Stopping Condition

The mission is complete only when staging evidence shows:

- direct editing creates durable VM-backed state before `Revise`;
- the durable state survives reload and is observable from another authenticated
  browser session/device path;
- a new VText/user head arriving while the editor is dirty does not silently
  clobber local text;
- the dirty draft has an explicit recover, merge, or rebase path over the newer
  head;
- stale VText output cannot overwrite newer user work;
- concurrent worker updates before and during VText runs are marked consumed or
  pending and integrated without duplicate mutation noise;
- long-content, many-version, multi-section evals pass over product paths;
- the accepted eval suite has been run on `fireworks-kimi-k2p6-low`,
  `fireworks-deepseek-v4-flash-medium`, and `chatgpt-gpt-5-5-low`;
- the final report compares those models on latency, revision cadence,
  coordination noise, tool errors, stale-head handling, worker-update
  integration, long-document edit quality, and residual failure modes;
- docs describe the resulting contract clearly enough for future edge-case work;
- CI and staging deploy for the behavior commit are green and identity-verified.

If only the state contract and first proof cases land, report
`checkpoint_incomplete`, not complete.

If a blocker remains after root-cause probes and cognitive transforms, report
`blocked_incomplete` with exact evidence, rollback state, and the smallest safe
next probe.

## Initial Eval Matrix

Seed the mission with these product-path scenarios:

1. **Clean Auto-Follow:** clean editor on head `A`; external user/VText creates
   `B`; editor auto-follows `B`; no update pill remains.
2. **Dirty No-Clobber:** dirty editor on `A`; external user/VText creates `B`;
   editor still shows local dirty text and exposes latest-head state.
3. **Dirty Draft Persistence:** dirty editor on `A`; wait for sync; reload or
   open second session; user draft is recoverable before `Revise`.
4. **Dirty Over Moved Head:** dirty editor on `A`; VText creates `B`; user
   continues typing; save/revise produces explicit merge/rebase/recover behavior,
   not silent discard or opaque terminal failure.
5. **Stale Agent Guard:** VText starts from `A`; user creates `B`; stale VText
   output from `A` cannot become canonical head.
6. **Concurrent Worker Batch:** several researcher/super updates arrive before
   VText wakes; one VText revision consumes the batch or records precise pending
   state.
7. **Late Worker While Dirty:** user is dirty, VText is active, worker update
   arrives late; final versions preserve user draft and worker evidence.
8. **Long Multi-Section:** long document, many prior versions, user edits distant
   sections, workers report disjoint changes, VText integrates without dropping
   sections or losing lineage.

Run the full accepted matrix across:

- `fireworks-kimi-k2p6-low`;
- `fireworks-deepseek-v4-flash-medium`;
- `chatgpt-gpt-5-5-low`.

For every model/scenario row, report:

- model and reasoning level;
- v1/v2/v3 timing and total wall time;
- number of user, autosave/draft, and appagent versions;
- worker updates posted, consumed, skipped, and pending;
- stale-head conflicts and whether recovery was successful;
- duplicate tool calls or side-effect attempts;
- final content quality notes for long and multi-section cases;
- evidence refs: doc id, trajectory id, trace refs, revision ids, screenshots or
  videos where relevant.

## Suggested `/goal`

```text
/goal Use MissionGradient. Complete docs/mission-vtext-durable-draft-version-graph-v0.md by optimizing the real VText durable draft/version graph under its invariants, belief-state updates, investigation loop, cognitive reframing, quality gradient, and verification criteria. Preserve the VText single-writer authority boundary, product-path proof, no-lost-user-text invariant, and staging landing loop. The key deliverable is working staging behavior plus an eval report over the accepted scenario matrix for fireworks-kimi-k2p6-low, fireworks-deepseek-v4-flash-medium, and chatgpt-gpt-5-5-low. Establish the documented contract and eval probes for direct editing, durable draft sync, dirty-head concurrency, stale VText writes, worker-update batching, and long multi-section VTexts; implement the smallest contract-preserving changes needed to make staging pass; then run and report the model suite with latency, revision cadence, coordination noise, tool errors, stale-head handling, worker-update integration, and content-quality evidence. If the stopping condition is not reached, do not call the mission complete: update the Run Checkpoint & Resumption State with checkpoint_incomplete or blocked_incomplete, record evidence and rollback refs, and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint:

- 2026-05-26 follow-up design decision: normalize coagent reporting into one
  structured non-canonical update API rather than adding wrapper tools. The
  right shape is Go-like: one `submit_coagent_update` primitive used by
  researcher, super, vsuper, and co-super. Role differences belong in system
  prompts and authority policy. The API should carry findings, evidence,
  artifacts, questions, proposals, blockers, and typed `capability_requests`
  symmetrically; those requests are signals for the owning supervisor/VText, not
  deterministic auto-routing.
- 2026-05-26 staging product-path long-section rubric probe failed across the
  accepted model set on deployed commit `2cf7253`. All rows preserved the dirty
  user marker and used researcher/search evidence, but none produced a super
  agent in Trace for the required command-evidence step. Kimi low completed the
  text rubric prematurely without actual super execution; v4-flash medium and
  GPT-5.5 low stayed more conservative and left command evidence pending. This
  exposes a mixed research-plus-execution coordination gap: researcher-first
  VText loops can keep integrating researcher deliveries without a deterministic
  later `request_super_execution` call.
- 2026-05-26 staging product-path multi-worker storm probe passed against the
  deployed behavior commit. Using Kimi low for conductor/VText/researcher, VText
  spawned two researcher agents on distinct branches, a dirty user marker was
  inserted after v1, and later VText revisions preserved the marker while
  recording two consumed worker updates from two distinct researcher senders.
  A stricter rerun waited for pending late updates to drain: the final head had
  no pending worker updates, four consumed worker updates across revisions, two
  distinct researcher senders, five appagent revisions, and the exact dirty
  marker still present.
- 2026-05-26 deployed commit `2cf7253954aa5f67f7251fd22f4946ed0adb40ec`
  completed the current deliverable: staging has the durable dirty-draft rebase
  behavior, cross-session pre-`Revise` autosave visibility, a passing dirty user
  edit plus researcher worker-follow-up proof, and a final model-suite rerun
  across v4-flash medium, Kimi low, and GPT-5.5 low. This is not the end of all
  VText version-graph work; remaining realism axes are many-version long
  documents, multi-worker storms, and source-grounded content quality.
- 2026-05-25 staging product-path worker-concurrency probe exposed a separate
  coordination failure before worker-update integration could be evaluated:
  v4-flash medium wrote a useful VText v1, the injected user revision preserved
  the exact marker text, then VText attempted `spawn_agent` and the call errored.
  The document stayed at one appagent revision plus the user marker revision for
  the full observation window. This matches the earlier v4 long-row malformed
  delegation noise and must be treated as a tool-argument normalization problem,
  not as proof that worker updates can be integrated over dirty user edits.
- 2026-05-25 local product-path probe identified the first concrete dirty-head
  failure surface: current autosave creates durable user revisions when the head
  is stable, but a dirty editor based on revision `A` can race with a newer
  head `B`. The existing backend stale-head guard correctly rejects the old
  parent, but the product still needs an explicit recover/merge/rebase path so
  the dirty user draft does not remain browser-local or die as an opaque `409`.
- A second local probe after the first rebase implementation found a frontend
  refresh gap: the backend could persist a rebased revision containing both the
  incoming head and dirty draft, while the focused editor continued displaying
  only the pre-rebase dirty buffer. This is a UI synchronization problem, not a
  persistence failure.

current artifact state:

- Staging now includes the first contract-preserving durable-draft fix:
  explicit `allow_rebase` user revision saves can rebase stale dirty drafts onto
  the current head while ordinary stale writes still return conflict.
- The behavior is deployed at
  `2cf7253954aa5f67f7251fd22f4946ed0adb40ec` and product-path proven for the
  dirty-over-moved-head API path, cross-session pre-`Revise` autosave
  visibility, one-researcher dirty worker follow-up, and two-researcher
  consumed/pending worker-update metadata.

what shipped:

- `b5d72c1` `Checkpoint VText durable draft version graph`
- `b2252fe` `Rebase stale VText user drafts`
- `cbed84d` `Document VText worker concurrency probe failure`
- `2cf7253` `Tolerate noisy delegate role values`

what was proven:

- Local focused runtime proof:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextCreateRevisionRejectsStaleHead|TestVTextCreateRevisionRebasesAllowedStaleUserDraft|TestVTextStaleAgentRevisionRejectsEditAfterUserEdit' -count=1`
  passed.
- Local product-path browser proof:
  `pnpm exec playwright test tests/vtext-document-stream.spec.js --project=chromium --grep 'auto-follows|rebases dirty|reopening the same file|restores on reload' --reporter=line`
  passed against the local service stack.
- Local frontend build:
  `pnpm build` passed.
- CI run `26423509844` passed, including runtime shards, non-runtime tests,
  integration smoke, Go vet/build, frontend build, and staging deploy.
- FlakeHub publish run `26423509853` passed.
- Staging `/health` reported proxy and sandbox deployed at
  `b2252fe4ecc9f05f827ca3c86e2703ada68d4820`.
- Deployed dirty-draft proof:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DURABLE_DRAFT_EVIDENCE_DIR=../test-results/vtext-durable-draft-staging-b2252fe-20260525T232239Z pnpm exec playwright test tests/vtext-durable-draft-version-graph.tmp.spec.js --project=chromium --reporter=line`
  passed.
- Deployed worker-concurrency probe:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DURABLE_DRAFT_EVIDENCE_DIR=../test-results/vtext-durable-draft-worker-concurrency-staging-b2252fe-20260525T235603Z pnpm exec playwright test tests/vtext-durable-draft-version-graph.tmp.spec.js --project=chromium --grep 'worker-driven VText follow-up' --reporter=line`
  failed: the exact user marker persisted in a durable user revision, but
  `spawn_agent` errored and no second appagent revision appeared within the
  observation window. Evidence: doc
  `b3cdb2d6-3ac8-4fad-8632-65ce5418072b`, submission
  `8ad567dd-cd0f-4dda-b6dd-ee3f8eebf50a`, failed trace artifact
  `frontend/test-results/vtext-durable-draft-versio-0f3ac-rker-driven-VText-follow-up-chromium/trace.zip`.
- Model-suite eval report created:
  `docs/vtext-durable-draft-version-graph-eval-report-2026-05-25.md`.
- Noisy-delegation fix local proof:
  `nix develop -c go test ./internal/runtime -run TestNormalizeDelegateTargetValueAllowsSingleNoisyAllowedTarget -count=1`
  passed.
- Focused VText regression proof:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextCreateRevisionRejectsStaleHead|TestVTextCreateRevisionRebasesAllowedStaleUserDraft|TestVTextStaleAgentRevisionRejectsEditAfterUserEdit|TestVTextInitialEditToolResultRequiresResearchContinuation' -count=1`
  passed.
- CI run `26424849935` passed, including runtime shards, non-runtime tests,
  integration smoke, Go vet/build, and staging deploy.
- FlakeHub publish run `26424849948` passed.
- Staging `/health` reported proxy and sandbox deployed at
  `2cf7253954aa5f67f7251fd22f4946ed0adb40ec`.
- Deployed dirty user edit plus worker-follow-up proof:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DURABLE_DRAFT_EVIDENCE_DIR=../test-results/vtext-durable-draft-worker-concurrency-staging-2cf7253-20260526T000659Z pnpm exec playwright test tests/vtext-durable-draft-version-graph.tmp.spec.js --project=chromium --grep 'worker-driven VText follow-up' --reporter=line`
  passed. Evidence:
  `test-results/vtext-durable-draft-worker-concurrency-staging-2cf7253-20260526T000659Z/dirty-user-edit-worker-followup.json`.
- Final deployed model-suite rerun:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-kimi-k2p6-low,fireworks-deepseek-v4-flash-medium,chatgpt-gpt-5-5-low VTEXT_MODEL_PROMPTS=deep-research,coding-super,long-multi-section VTEXT_MODEL_CADENCE_EVIDENCE_DIR=../test-results/vtext-model-suite-durable-draft-2cf7253-20260526T000948Z pnpm exec playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`
  passed. Evidence:
  `test-results/vtext-model-suite-durable-draft-2cf7253-20260526T000948Z/`.
- Deployed two-researcher dirty worker-update storm proof:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DURABLE_DRAFT_EVIDENCE_DIR=../test-results/vtext-durable-draft-multi-worker-staging-a2fe62f-20260526T003647Z pnpm exec playwright test tests/vtext-durable-draft-version-graph.tmp.spec.js --project=chromium --grep 'two researcher worker updates' --reporter=line`
  passed. Evidence:
  `test-results/vtext-durable-draft-multi-worker-staging-a2fe62f-20260526T003647Z/dirty-user-edit-two-worker-updates.json`.
- Deployed two-researcher pending-drain proof:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DURABLE_DRAFT_EVIDENCE_DIR=../test-results/vtext-durable-draft-multi-worker-drain-staging-a2fe62f-20260526T005412Z pnpm exec playwright test tests/vtext-durable-draft-version-graph.tmp.spec.js --project=chromium --grep 'two researcher worker updates' --reporter=line`
  passed. Evidence:
  `test-results/vtext-durable-draft-multi-worker-drain-staging-a2fe62f-20260526T005412Z/dirty-user-edit-two-worker-updates.json`.
- Deployed long-section rubric probe:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-2cf7253-full-20260526T010650Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`
  failed across Kimi low, v4-flash medium, and GPT-5.5 low. Evidence:
  `test-results/vtext-long-section-rubric-staging-2cf7253-full-20260526T010650Z/`.

residual or partial claims:

- two-browser cross-session draft visibility before `Revise` is proven; two
  physical devices are not separately proven;
- one researcher worker update arriving after a dirty user revision is proven;
  a two-researcher storm is now proven for consumed/pending metadata and dirty
  marker preservation, including eventual pending-drain to an empty latest
  pending set in the stricter rerun;
- long-content model rows are covered, but not yet with a strict
  section-obligation/content-quality rubric that passes;
- whether autosave-as-canonical-user-version is the final durable draft model
  remains open.

belief-state changes:

- The smallest useful first cut is not WebSocket transport. It is explicit
  stale user draft rebase semantics on the existing product revision API, plus
  focused-editor refresh after a successful rebase response. WebSocket remains a
  possible later transport for lower-latency draft replication, but not the
  current load-bearing uncertainty.
- Mixed research-plus-execution prompts need an explicit continuation guard.
  The existing model contract can route the first turn to researcher and later
  wake VText from researcher findings, but the observed models do not reliably
  remember that super execution is still required once research evidence begins
  arriving.

remaining error field:

- No accepted model reliably completes the strict long-section
  research-plus-super rubric. Trace showed zero super agents for all three rows.
- v4-flash medium no longer fails noisy researcher delegation, but its final
  deep-research row still did not produce a second VText revision inside the
  observation window.
- GPT-5.5 low still produced duplicate side-effect attempts; runtime guards
  skipped or contained them.
- Kimi low remains the cleanest coordination row, but model/content quality still
  needs a stricter rubric than revision cadence.
- Current-events content quality remains a real risk: the Artemis worker proof
  preserved lineage and consumed worker evidence, but the final content included
  contradictory launch-status claims that need source-truth gates.

highest-impact remaining uncertainty:

- Whether the runtime should enforce a pending-super continuation for mixed
  research-plus-execution VText documents after the first researcher delivery,
  rather than relying on the model to remember and call `request_super_execution`.

next executable probe:

- Add the smallest runtime/prompt guard that makes a VText document whose
  original request needs both research and execution call
  `request_super_execution` once research grounding exists and no super request
  has been recorded, then rerun the long-section rubric across the accepted
  model set.

checkpoint update, 2026-05-26 02:13 UTC:

- Landed normalized coagent update direction through code commit
  `92896a2ae8069f58f42e60c7587f97ec7a808913` after docs checkpoint
  `776dc94`.
- Local proof passed:
  `nix develop -c scripts/go-test-runtime-shards`.
- CI run `26427881056` passed, including runtime shards, non-runtime tests,
  integration smoke, Go vet/build, and staging deploy.
- Staging `/health` reported proxy and sandbox deployed at
  `92896a2ae8069f58f42e60c7587f97ec7a808913`, deployed at
  `2026-05-26T01:57:16Z`, with `vmctl_status=ok`.
- Deployed long-section rubric rerun command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-92896a2-full-20260526T015800Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Kimi low row failed the strict final-rubric timeout. Evidence:
  `test-results/vtext-long-section-rubric-staging-92896a2-full-20260526T015800Z/fireworks-kimi-k2p6-low.json`.
- Important positive signal: the normalized primitive worked for researcher
  cadence. Trace shows repeated successful `web_search` and
  `submit_coagent_update` calls; VText consumed `Coagent update ready` channel
  messages and produced multiple app revisions from those updates.
- New problem evidence: VText wrote that super execution was pending/requested
  and carried a `[CMD]` ledger row, but Trace showed no `super` agent and no
  `request_super_execution` tool result. The document advanced through
  researcher-grounded revisions while the execution obligation remained only a
  narrative placeholder.

cognitive transform pass:

- Depth extraction: the load-bearing variable is not "can researchers report
  findings" anymore; it is whether VText treats unmet capability obligations as
  durable workflow debt instead of prose state.
- Boundary inversion: a capability request or prompt obligation must be treated
  as "not done until a role-appropriate update arrives"; writing "pending" is
  useful only if paired with the side-effect that creates the pending work.
- Evidence-substitution lens: the expected command hash in the prompt is a
  verifier bait. A passing document must distinguish target/expected values
  from evidence returned by super.
- State-machine lens: the minimal prompt-level invariant is "if the user asked
  for command/code/browser/verification evidence and no successful super update
  exists, the next VText turn must either call `request_super_execution` or
  explicitly say the execution obligation is still blocked without claiming the
  evidence."

belief-state changes:

- Normalizing researcher/super/vsuper/co-super updates into
  `submit_coagent_update` reduced API surface noise without requiring wrapper
  tools, and it preserved VText wakeup from incoming worker messages.
- The long-rubric failure has narrowed from "research plus execution
  coordination is generally noisy" to "VText does not reliably convert an
  outstanding execution obligation into `request_super_execution` after
  researcher updates arrive."

remaining error field:

- The system can still create polished long VText revisions with a false
  workflow state: command execution described as pending/requested even though
  no super request has been made.
- Strict model-suite pass remains incomplete until Kimi low, v4-flash medium,
  and GPT-5.5 low each show researcher grounding plus successful super evidence
  on the long-section rubric.

next executable probe:

- Tighten only VText prompt/policy language so it must call
  `request_super_execution` when an unmet execution/code/browser/verification
  obligation is present, and must not use `[CMD]` as evidence unless a super
  update returned it. Do not add deterministic wrapper/control-flow tools.

checkpoint update, 2026-05-26 02:25 UTC:

- Prompt-only fix `d7921ec86258a08da97fde0242a6f2f371614a2a` landed and
  deployed. CI run `26428490820` passed; staging `/health` reported proxy and
  sandbox deployed at `d7921ec86258a08da97fde0242a6f2f371614a2a`, deployed at
  `2026-05-26T02:19:04Z`, with `vmctl_status=ok`.
- Focused local prompt proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestVTextPrompt|TestInitialVTextToolChoiceUsesExactTools|TestVTextPromptStoryWithCurrentFactsRequiresGrounding' -count=1`.
- Post-fix long-section Kimi row command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-d7921ec-full-20260526T021939Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Kimi low still failed `traceHasSuper=false`, but it now reached final rubric
  shape quickly: marker preserved, all 12 sections present, source ledger and
  section update sentences present. Evidence:
  `test-results/vtext-long-section-rubric-staging-d7921ec-full-20260526T021939Z/fireworks-kimi-k2p6-low.json`.

new problem evidence:

- The static VText prompt now says not to claim super was requested without the
  tool result, but the dynamic revision prompt still creates a conflicting
  incentive: after a researcher packet arrives, it says to update the document
  as soon as the packet can improve it.
- In the Kimi row, VText consumed researcher updates, wrote a complete final
  shape, and left command execution as pending, but did not call
  `request_super_execution`.

belief-state changes:

- The next fix should still avoid wrapper tools and new routing APIs, but it
  must remove the prompt conflict inside the dynamic VText revision prompt:
  researcher evidence is enough to improve source sections, not enough to
  advance command/evidence sections when a super obligation is open.

remaining error field:

- Prompt-only static wording is insufficient while dynamic revision instructions
  prioritize immediate `edit_vtext` after worker findings.

next executable probe:

- Strengthen the dynamic VText revision prompt and role prompt so an open
  command/code/browser/verification obligation has priority over another
  source-grounded edit: call `request_super_execution` first, or write only an
  explicit blocked state that does not satisfy `[CMD]`.

checkpoint update, 2026-05-26 02:35 UTC:

- Dynamic prompt fix `fe00a14` landed and deployed. CI run `26428732241`
  passed; staging `/health` reported proxy and sandbox deployed at
  `fe00a1412313869018457eb828620d6521d5ab2f`, deployed at
  `2026-05-26T02:27:31Z`, with `vmctl_status=ok`.
- Clean post-deploy Kimi row still failed `traceHasSuper=false`. Evidence:
  `test-results/vtext-long-section-rubric-staging-fe00a14-full-20260526T022928Z/fireworks-kimi-k2p6-low.json`.

new problem evidence:

- Kimi reached the final content shape quickly but still never opened super.
  Trace showed only two VText runs: initial `edit_vtext` -> `spawn_agent`, then
  worker-wake `edit_vtext`. No `request_super_execution` was available in the
  observed trace.
- Code inspection shows why prompt-only pressure is weak here:
  `initialVTextToolChoice` returns exact `edit_vtext` whenever a VText run does
  not require first worker grounding. A researcher delivery makes the document
  have grounded history, so the worker-wake VText run is biased toward the
  document-write path even when the original prompt still has an unmet super
  obligation.

cognitive transform update:

- Constraint inversion: the problem is not lack of semantic instruction. The
  instruction exists, but the tool-choice prior narrows the action surface at
  the moment when VText must decide between writing and opening super.
- Minimalism lens: the fix should not add wrapper tools or route execution
  automatically. It should remove the over-specific exact tool choice for
  worker-wake VText runs, letting the existing VText prompt and existing
  `request_super_execution` tool compete normally.

next executable probe:

- For VText runs woken by addressed worker messages, stop forcing exact
  `edit_vtext` as the first tool choice. Keep exact tool choice for initial
  first-version shaping where it protects prompt-to-v1 cadence. Rerun the long
  rubric on the three accepted models after deploy.

checkpoint update, 2026-05-26 03:24 UTC:

- Tool-choice fix `f3d48e7` later landed, followed by default-policy and
  temporal-grounding fixes through deployed commit
  `4e3080ba3dbb3f7dc5c64396f0fd597e22ea1488`. CI run `26429507417`
  passed; staging `/health` reported proxy and sandbox deployed at
  `4e3080ba3dbb3f7dc5c64396f0fd597e22ea1488`, deployed at
  `2026-05-26T02:53:48Z`.
- Manual mobile QA before these fixes showed two distinct staging problems:
  bootstrap route instability with repeated `502` responses around the
  15-20 second mark, and simple current-events VText runs that stalled or used
  stale/irrelevant baseball information.
- Default-policy proof after `4e3080b` passed for the simple current-events
  case. Evidence:
  `test-results/vtext-default-policy-staging-4e3080b-20260526T025423Z/default-policy-proof.json`.
  The generated policy used DeepSeek v4 Flash at `medium` reasoning for both
  VText and researcher, and the document anchored "last night" to Monday,
  2026-05-25 instead of the earlier stale May 12-13 range.
- Full long-section rubric rerun command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-4e3080b-full-20260526T025808Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Kimi low passed the strict long-section rubric. Evidence:
  `test-results/vtext-long-section-rubric-staging-4e3080b-full-20260526T025808Z/fireworks-kimi-k2p6-low.json`.
- DeepSeek v4 Flash medium failed the strict final-rubric timeout, but not with
  the old no-super symptom. Trace showed 262 moments, two researcher agents,
  one super agent, 15 search queries, successful `request_super_execution`, one
  `super:bash`, one super `submit_coagent_update`, and five VText edits. The
  final content incorporated the command hash and researcher/super evidence but
  abandoned the exact requested numbered-section shape and omitted the required
  `SECTION 1 UPDATE:`, `SECTION 7 UPDATE:`, and `SECTION 12 UPDATE:` sentences.
  Evidence:
  `test-results/vtext-long-section-rubric-staging-4e3080b-full-20260526T025808Z/fireworks-deepseek-v4-flash-medium.json`.
- GPT-5.5 low failed earlier in the pipeline. Trace showed only one VText
  agent run, no researcher, no super, no search, and two `edit_vtext` tool
  calls at the same timestamp. The current document head was the user marker
  revision over the initial appagent v1, so no source-grounded follow-up
  arrived after the dirty user edit. Evidence:
  `test-results/vtext-long-section-rubric-staging-4e3080b-full-20260526T025808Z/chatgpt-gpt-5-5-low.json`.
- Staging health after the long run still reported deployed `4e3080b`.
  Bootstrap counters increased in total volume during the run, but the recorded
  bootstrap error counts did not increase from the previously observed
  `http_502=8` and `resolve_error=15`, so the long-rubric failures should be
  treated as VText coordination/content-shape failures rather than fresh VM
  boot failures.

belief-state changes:

- The worker-wake exact-tool-choice fix did unblock the old execution bridge
  for at least Kimi low and v4 Flash medium: VText can now request super and
  consume returned super evidence on the long rubric.
- The remaining v4 Flash problem is a long-document obligation-retention
  problem. Once enough worker evidence arrives, VText can replace the document
  with a coherent research brief while losing the user's exact section and
  update-sentence contract.
- The remaining GPT-5.5 problem appears to be a continuation/action-selection
  problem before delegation: the model produced an initial working draft and
  repeated `edit_vtext`, but did not open researcher or super work before the
  dirty user marker became the head.
- VM pressure remains an operational risk, but this evidence does not show new
  bootstrap errors as the direct cause of the long-rubric failures.

remaining error field:

- VText still does not robustly preserve explicit long-document obligations
  across worker-update integration for every accepted model.
- GPT-5.5 low can still satisfy the first-draft surface while failing to launch
  the requested researcher/super pipeline.
- The accepted suite is not complete until Kimi low, v4 Flash medium, and
  GPT-5.5 low all pass the long-section mixed researcher/super/user-edit
  rubric with durable marker preservation and exact obligation satisfaction.

next executable probe:

- Root-cause the GPT duplicate-edit/no-delegation trace and the v4
  obligation-retention trace against the VText prompts and first-tool policy.
  Prefer prompt and tool-choice surface shaping over deterministic role-specific
  control flow; keep the agent harness uniform.

checkpoint update, 2026-05-26 04:01 UTC:

- Prompt/contract fix `9a7e533b4b4b9d95044fa30ccb062bd83db7cf02`
  landed and deployed. CI run `26430553860` passed, FlakeHub run
  `26430553844` passed, and staging `/health` reported proxy and sandbox on
  deployed commit `9a7e533b4b4b9d95044fa30ccb062bd83db7cf02`, deployed at
  `2026-05-26T03:30:21Z`.
- Local focused proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestVTextPromptPreservesExplicitHardConstraints|TestVTextPromptCurrentEventsRequiresResearcher|TestInitialVTextToolChoiceUsesExactTools|TestVTextInitialEditContinuationClassifiesPrompts|TestVTextExplicitResearchWinsFirstContinuationForMixedPrompt|TestVTextInitialEditRequiresContinuationButSpawnDoesNotForceSecondEdit' -count=1`.
- Local comprehensive continuation proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestEditVTextInitialWorkingRevisionRequiresActualContinuation' -count=1`.
- Full long-section rubric rerun command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-9a7e533-full-20260526T033108Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Kimi low passed again. Evidence:
  `test-results/vtext-long-section-rubric-staging-9a7e533-full-20260526T033108Z/fireworks-kimi-k2p6-low.json`.
- DeepSeek v4 Flash medium failed, but the failure narrowed materially.
  Trace showed researcher and super work completed, command hash present,
  source ledger present, all 12 sections present, and marker preserved. The
  only strict content assertion that failed was the required update sentence
  prefixes: `SECTION 1 UPDATE:`, `SECTION 7 UPDATE:`, and
  `SECTION 12 UPDATE:` were absent from the final replacement. Evidence:
  `test-results/vtext-long-section-rubric-staging-9a7e533-full-20260526T033108Z/fireworks-deepseek-v4-flash-medium.json`.
- GPT-5.5 low also changed failure shape. It no longer failed at "no workers":
  Trace showed two researchers, one super, searches, `spawn_agent`, repeated
  `request_super_execution`, and `super:bash`. However, no post-marker
  appagent revision was written. VText produced the initial working draft,
  the user marker became the head, and subsequent VText turns repeatedly
  requested super instead of writing the final grounded revision. Super ran
  `bash`, including one error result, but did not deliver a usable
  `submit_coagent_update` before the strict timeout. Evidence:
  `test-results/vtext-long-section-rubric-staging-9a7e533-full-20260526T033108Z/chatgpt-gpt-5-5-low.json`.

cognitive transform update:

- Checklist-to-prompt transform: "Preserve hard constraints" as prose is too
  diffuse for v4 Flash. The next prompt should materialize a compact checklist
  of exact required prefixes/markers already visible in the document/request,
  so replace-all generation has nearby anchors.
- Negative-space transform: GPT did perform the missing actions after the
  previous fix. The remaining empty space is not "can GPT delegate" but "can
  the super request lifecycle produce an addressed evidence packet and stop
  VText from re-requesting instead of revising."
- Signal routing transform: repeated `request_super_execution` calls are a
  valuable signal for VText and the mission; they show an unresolved execution
  obligation, but after a request exists they should not monopolize every
  subsequent VText turn.

remaining error field:

- v4 Flash still needs stronger exact-prefix retention for long structured
  replacements.
- GPT-5.5 low still needs a clearer super-result contract and VText behavior
  for "super already requested / evidence not yet delivered" states.

next executable probe:

- Add dynamic prompt materialization for exact hard requirements already present
  in the document/request, especially `SECTION n UPDATE:` prefixes and
  `USER_*MARKER*` strings, and strengthen the super continuation objective so
  bounded command work must report through `submit_coagent_update` whether the
  command succeeds or fails.

checkpoint update, 2026-05-26 04:11 UTC:

- Dynamic hard-requirement checklist fix
  `679115abbbfdbc2027996bc593c147573b7bddee` landed and deployed. CI run
  `26431436850` passed, FlakeHub run `26431436856` passed, and staging
  `/health` reported proxy and sandbox on deployed commit
  `679115abbbfdbc2027996bc593c147573b7bddee`, deployed at
  `2026-05-26T04:00:32Z`.
- Full long-section rubric command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-679115a-full-20260526T040111Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Playwright reported `3 passed (8.1m)`. Evidence:
  `test-results/vtext-long-section-rubric-staging-679115a-full-20260526T040111Z/`.
- Important stricter audit: the rubric pass is not yet sufficient acceptance.
  GPT-5.5 low included real super command evidence: `[CMD] Super command
  evidence: ... exited 0 ... printed ...`.
- Kimi low and v4 Flash medium passed the current rubric while still writing
  `[CMD]` as pending/target state rather than actual command evidence:
  Kimi wrote `[CMD] Pending super execution...`; v4 wrote
  `[CMD] — pending (super execution requested)`.
- Therefore the current eval rubric is too weak: it treats the presence of the
  expected hash and `[CMD]` label as command evidence, even when the document
  explicitly says the command is still pending.

belief-state changes:

- The dynamic hard-requirement checklist fixed the v4 exact-prefix loss:
  v4 preserved all sections and the three `SECTION n UPDATE:` prefixes.
- The super reporting contract fixed GPT's prior no-final-revision behavior in
  this run: GPT received enough super evidence to write a real `[CMD]` row.
- The remaining problem is evidence-label discipline. VText must not use the
  `[CMD]` evidence label for pending command state, and the eval must reject
  pending `[CMD]` rows.

remaining error field:

- Kimi low and v4 Flash medium can still produce a rubric-shaped document that
  looks accepted while command evidence is pending.
- The accepted matrix should not be considered complete until all three rows
  satisfy a stricter command-evidence check: `[CMD]` must be backed by a super
  delivery or explicitly successful command output, not pending text or a
  user-supplied target hash.

next executable probe:

- Strengthen VText prompt language and hard-requirement hints so `[CMD]` is a
  final evidence label only after super delivery. Pending command state should
  be written without `[CMD]`. Tighten the long-section eval/audit to reject
  `[CMD]` rows containing pending/requested/target-only wording.

checkpoint update, 2026-05-26 04:50 UTC:

- Evidence-label fix `fdcfaad42257703ad33e01d9adedb5249b67b790` landed and
  deployed. CI run `26431842186` passed, FlakeHub run `26431842166` passed,
  and staging `/health` reported proxy deployed at
  `fdcfaad42257703ad33e01d9adedb5249b67b790`, deployed at
  `2026-05-26T04:14:37Z`, with `vmctl_status=ok`. Bootstrap counters still
  showed the earlier pressure signature (`http_502=8`, `resolve_error=15`),
  but the strict eval failures below are coordination/content-shape failures,
  not fresh bootstrap-route failures.
- The long-section eval was tightened so the command-evidence assertion rejects
  pending/requested/target-only `[CMD]` rows. A loose strict audit before the
  wait change showed GPT-5.5 low could satisfy this stronger command evidence
  shape, while Kimi low and v4 Flash medium could still write pending `[CMD]`
  rows.
- Strict-wait staging command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-fdcfaad-strictwait-20260526T042507Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Result: GPT-5.5 low passed; Kimi low and DeepSeek v4 Flash medium failed.
  Evidence:
  `test-results/vtext-long-section-rubric-staging-fdcfaad-strictwait-20260526T042507Z/`.
- GPT-5.5 low pass shape: 2 app revisions, researcher and super present,
  `commandEvidenceSatisfied=true`, 4 searches, 8 researcher
  `submit_coagent_update` calls, 8 VText `request_super_execution` calls,
  4 super `bash` calls, and 4 super `submit_coagent_update` calls. The final
  revision consumed researcher and super updates and preserved all exact long
  rubric obligations.
- Kimi low failure shape: 3 appagent revisions after the user marker, all via
  source-only `edit_vtext` calls consuming researcher updates; no super agent
  appeared in Trace, and the final document said command evidence was pending
  without using a fake `[CMD]` label. This proves the label fix worked while
  exposing a remaining execution-delegation priority failure.
- v4 Flash medium failure shape: only the first appagent revision appeared
  within the strict wait. Trace showed one researcher and no super. The initial
  revision still used `[CMD]` for a pending ledger row, so v4 needs the
  evidence-label discipline to apply to first-draft structure as well as final
  revisions.

cognitive transform update:

- Label/authority transform: `[CMD]` is not a formatting token. It is an
  authority claim that a super delivery exists. Pending command state must use
  unlabelled prose or a non-evidence status row.
- Obligation-debt transform: after a document has source grounding, an unmet
  execution requirement is workflow debt with higher priority than another
  source-only revision. VText should not converge to `completed` while that
  obligation has no super request and no explicit blocker.
- First-draft/final-draft split: a useful v1 may mention command evidence is
  pending, but it must not pre-allocate the final `[CMD]` evidence label to that
  pending state. The prompt needs to separate scaffold placeholders from final
  evidence labels.
- Model-contrast transform: GPT passing with the same tools proves the runtime
  path is available; Kimi/v4 failures are robustness failures in prompt/tool
  selection pressure, not missing capability.

belief-state changes:

- The stronger `[CMD]` prompt removed the fake pending label for Kimi's later
  revisions, but not for v4's initial ledger scaffold.
- The current strongest remaining loss is not search quality. It is VText's
  ordering policy when both source-grounding and execution evidence are
  required: weaker rows can keep improving source text while never opening or
  completing the execution side channel.
- GPT-5.5 low is now the positive control for the accepted mixed
  researcher/super long-rubric path.

remaining error field:

- Kimi low can still finish its VText loop with no super agent when researcher
  evidence has arrived and command evidence is still required.
- v4 Flash medium can still use `[CMD]` as a pending scaffold in v1 and can
  fail to advance beyond that v1 under strict wait.
- The accepted matrix remains incomplete until Kimi low, v4 Flash medium, and
  GPT-5.5 low all pass the stricter command-evidence rubric.

next executable probe:

- Strengthen the VText prompt and dynamic checklist so an open command/code/
  browser/verification obligation with no super request has priority over
  source-only edits after the first working draft, and so the initial draft can
  describe pending command evidence only without using the final `[CMD]`
  evidence label. Keep this as prompt/tool-surface shaping, not wrapper tools
  or role-specific harness branching.

checkpoint update, 2026-05-26 05:18 UTC:

- Prompt/tool-surface fix `14431b525aded4b2faabad1394b21d786c439daa`
  landed and deployed. CI run `26433038844` passed, FlakeHub run
  `26433038861` passed, and staging `/health` reported proxy deployed at
  `14431b525aded4b2faabad1394b21d786c439daa`, deployed at
  `2026-05-26T04:54:48Z`, with `vmctl_status=ok`. Bootstrap counters remained
  unchanged from the prior pressure signature (`http_502=8`,
  `resolve_error=15`).
- Local focused proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestVTextPromptPreservesExplicitHardConstraints|TestVTextPromptPrioritizesSuperAfterResearchForMixedObligation|TestVTextSuperContinuationObjectiveRequiresCoagentUpdate|TestVTextPromptCurrentEventsRequiresResearcher|TestInitialVTextToolChoiceUsesExactTools|TestVTextInitialEditContinuationClassifiesPrompts|TestVTextExplicitResearchWinsFirstContinuationForMixedPrompt' -count=1`.
- Local comprehensive continuation proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestEditVTextInitialWorkingRevisionRequiresActualContinuation' -count=1`.
- Strict-wait staging command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-14431b5-strictwait-20260526T045536Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Result: Kimi low passed, GPT-5.5 low passed, v4 Flash medium failed.
  Evidence:
  `test-results/vtext-long-section-rubric-staging-14431b5-strictwait-20260526T045536Z/`.
- Kimi low pass shape: 4 app revisions, 1 researcher, 1 super, 3 search
  queries, one VText `request_super_execution`, one super `bash`, one super
  `submit_coagent_update`, all exact section/update/marker obligations
  preserved, and `commandEvidenceSatisfied=true`.
- GPT-5.5 low pass shape: 2 app revisions, 2 researchers, 1 super, 4 search
  queries, repeated VText `request_super_execution`, 4 super `bash` calls, 4
  super `submit_coagent_update` calls, all exact obligations preserved, and
  `commandEvidenceSatisfied=true`.
- v4 Flash medium failure shape: only one appagent revision was created, the
  VText agent ended in `failed` state after a single successful `edit_vtext`,
  and Trace showed no researcher, no super, and no search. The initial v1 still
  included `[CMD]` as a pending Source Ledger row:
  `[CMD] | printf ... | Command evidence pending — super execution requested`.
  This means v4 is still over-literal about final ledger requirements in the
  initial scaffold, and it also did not perform the post-edit required
  continuation.

belief-state changes:

- The same deployed runtime can now pass the strict mixed researcher/super
  long-rubric path with Kimi low and GPT-5.5 low.
- The remaining accepted-matrix blocker is narrowed to v4 Flash medium. Its
  failure is no longer "cannot do the whole workflow" in all cases; on this
  run it failed before delegation by treating the initial scaffold as if it
  should already contain final `[CMD]` ledger shape and then stopping after
  the first edit.
- The prompt still exposes the literal `[CMD]` hard requirement too close to
  the initial-v1 generation surface. For v4, "final-only" prose is weaker than
  a prompt shape that withholds final-only evidence labels from the initial
  scaffold acceptance checklist until super evidence exists.

remaining error field:

- v4 Flash medium can still write a pending `[CMD]` scaffold in v1 and fail to
  continue to researcher/super after the first edit.
- The accepted matrix remains incomplete until v4 Flash medium also passes the
  strict command-evidence rubric.

next executable probe:

- Change hard-requirement materialization so `[CMD]` is not presented as an
  immediate checklist item when no super delivery is present. Instead present a
  pending-command rule for v1/interim revisions and reintroduce the literal
  `[CMD]` requirement only once a super delivery or execution blocker exists.
  Keep the existing post-edit continuation guard intact and rerun the v4 row
  first before spending another full matrix run.

checkpoint update, 2026-05-26 05:35 UTC:

- V4-focused prompt fix `29186b6177ca2d73ae1c8e91e98c99d68ca2764a` landed and
  deployed. CI run `26433850464` passed, FlakeHub run `26433850461` passed,
  and staging `/health` reported proxy deployed at
  `29186b6177ca2d73ae1c8e91e98c99d68ca2764a`, deployed at
  `2026-05-26T05:21:09Z`, with `vmctl_status=ok`.
- Local focused proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestVTextPromptPreservesExplicitHardConstraints|TestVTextPromptRestoresFinalCommandEvidenceRequirementAfterSuperDelivery|TestVTextPromptPrioritizesSuperAfterResearchForMixedObligation|TestVTextSuperContinuationObjectiveRequiresCoagentUpdate|TestVTextPromptCurrentEventsRequiresResearcher|TestInitialVTextToolChoiceUsesExactTools|TestVTextInitialEditContinuationClassifiesPrompts|TestVTextExplicitResearchWinsFirstContinuationForMixedPrompt' -count=1`.
- Local comprehensive continuation proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestEditVTextInitialWorkingRevisionRequiresActualContinuation' -count=1`.
- V4-only strict-wait staging command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_VARIANTS=fireworks-deepseek-v4-flash-medium VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-29186b6-v4-strictwait-20260526T052154Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Result: v4 Flash medium failed, but the failure moved. Evidence:
  `test-results/vtext-long-section-rubric-staging-29186b6-v4-strictwait-20260526T052154Z/fireworks-deepseek-v4-flash-medium.json`.
- Positive movement: the initial v1 no longer used `[CMD]` as a pending
  Source Ledger label. It used `Command evidence` for the pending command row,
  preserving the final-only evidence-label distinction.
- New failure shape: Trace showed 554 moments, two researcher agents, one
  super agent, search activity, 19 VText `request_super_execution` calls, and
  28 super `bash` calls. There were zero super `submit_coagent_update` calls.
  VText therefore never received consumable command evidence and kept
  re-requesting execution.

belief-state changes:

- V4 can now get past the initial scaffold label problem and open the
  researcher/super topology.
- The remaining v4 blocker is the super reporting contract: bounded command
  execution happens, but the evidence does not cross the coagent update
  boundary back to VText.
- Prompt-only reporting language in the super role prompt and VText objective
  is not strong enough for v4 Flash medium under this rubric.

remaining error field:

- V4 Flash medium can still run the requested command repeatedly without
  delivering `submit_coagent_update`, leaving VText no authoritative `[CMD]`
  evidence to consume.
- The full accepted matrix is still incomplete because v4 has not passed the
  strict command-evidence rubric.

next executable probe:

- Add the smallest uniform tool-result continuation hint for super bounded
  `bash` results when the super run is serving a VText channel: after a bash
  result, require or strongly name `submit_coagent_update` as the next tool so
  command output crosses the existing coagent-update boundary. This should not
  add a wrapper API or let super write canonical text; it only reinforces the
  existing one-way evidence-reporting contract.

checkpoint update, 2026-05-26 05:51 UTC:

- Super reporting continuation fix `34941109e76c681fd8d5ef81ea1bf766cf16f34d`
  landed and deployed. CI run `26434389749` passed, FlakeHub run
  `26434389729` passed, and staging `/health` reported proxy deployed at
  `34941109e76c681fd8d5ef81ea1bf766cf16f34d`, deployed at
  `2026-05-26T05:37:21Z`, with `vmctl_status=ok`.
- Local proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestPersistentSuperInboxBashRequiresCoagentUpdate|TestEditVTextInitialWorkingRevisionRequiresActualContinuation' -count=1`.
- Local focused prompt proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestVTextPromptPreservesExplicitHardConstraints|TestVTextPromptRestoresFinalCommandEvidenceRequirementAfterSuperDelivery|TestVTextPromptPrioritizesSuperAfterResearchForMixedObligation|TestVTextSuperContinuationObjectiveRequiresCoagentUpdate|TestVTextPromptCurrentEventsRequiresResearcher|TestInitialVTextToolChoiceUsesExactTools|TestVTextInitialEditContinuationClassifiesPrompts|TestVTextExplicitResearchWinsFirstContinuationForMixedPrompt' -count=1`.
- V4-only strict-wait staging command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_VARIANTS=fireworks-deepseek-v4-flash-medium VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-3494110-v4-strictwait-20260526T053808Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Result: v4 Flash medium still failed. Evidence:
  `test-results/vtext-long-section-rubric-staging-3494110-v4-strictwait-20260526T053808Z/fireworks-deepseek-v4-flash-medium.json`.
- Failure shape changed again: Trace showed one researcher, no super, search
  activity, one researcher `submit_coagent_update`, and only one appagent
  revision. VText spawned research but never requested super or wrote a
  post-research revision before timeout.
- The final visible document also regressed to using `[CMD]` as a target
  placeholder in the v1 ledger, despite the previous prompt-shaping fix. The
  stricter artifact analysis still rejects it because no source entries and no
  super evidence reached a final revision.

belief-state changes:

- V4 Flash medium at `medium` is unstable across adjacent runs of the same
  staging rubric: one run opened super repeatedly without evidence delivery;
  the next opened only researcher and never requested super.
- The current failure is no longer explained by one missing prompt sentence.
  The model is failing to preserve the role topology across the multi-turn
  VText loop under long-document hard requirements.
- Kimi low and GPT-5.5 low remain the positive controls for this exact strict
  rubric after `14431b5`; v4 remains the only model blocking the accepted
  matrix.

remaining error field:

- V4 Flash medium can still stop after v1/researcher without opening super or
  producing a source-grounded final revision.
- Prompt-only coordination may be below the reliability threshold for v4 as a
  VText owner on this long mixed research-plus-execution workload.

next executable probe:

- Before adding stronger deterministic control flow, run one final v4-focused
  cognitive transform / root-cause pass over the trace: determine whether the
  acceptable solution is (a) declare v4 medium unsuitable for VText owner on
  this workload while still usable for narrower conductor/researcher roles, or
  (b) introduce a narrowly scoped VText next-tool policy for open execution
  obligations despite the preference for prompt-only coordination. If choosing
  (b), document why prompt-only alternatives were falsified first.

checkpoint update, 2026-05-26 06:08 UTC:

- Manual QA reproduced two visible product failures: mobile bootstrap sometimes
  waits about 20s and reports `VM route returned 502`, while simple baseball
  prompts can remain stuck in weak v1/v2 research-in-progress states instead of
  converging to a useful next version.
- Staging `/health` during the investigation still reported `vmctl_status=ok`,
  but lifecycle counters showed real route pressure: `bootstrap.total` had 23
  errors out of 238 attempts, with `http_502=8` and `resolve_error=15`;
  `bootstrap.resolve` max duration was 15098ms.
- Fresh-account default-policy proof command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DEFAULT_POLICY_EVIDENCE_DIR=../test-results/vtext-default-policy-current-20260526T055414Z npx playwright test tests/vtext-default-policy-proof.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- The default-policy proof produced evidence despite a Playwright artifact
  timeout: VText and researcher both resolved to
  `fireworks/accounts/fireworks/models/deepseek-v4-flash` with
  `reasoning=medium`, and the document reached v2 after about 176s. Evidence:
  `test-results/vtext-default-policy-current-20260526T055414Z/default-policy-proof.json`.
- The v2 was still weak: it incorporated only partial baseball findings and
  explicitly said final scores were still being gathered. This falsifies the
  simple theory that the manual problem is only stale `low`/`none` model policy.
- Explicit low-control command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-low VTEXT_MODEL_PROMPTS=baseball VTEXT_MODEL_CADENCE_EVIDENCE_DIR=../test-results/vtext-model-cadence-current-low-control-20260526T055414Z npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Low-control result: v1 at about 3.4s, a researcher was spawned, one
  `web_search` ran, but no findings reached VText within the observation
  window. Evidence:
  `test-results/vtext-model-cadence-current-low-control-20260526T055414Z/fireworks-deepseek-v4-flash-low.json`.
- Explicit medium-control command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-medium VTEXT_MODEL_PROMPTS=baseball VTEXT_MODEL_CADENCE_EVIDENCE_DIR=../test-results/vtext-model-cadence-current-medium-control-20260526T055912Z npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Medium-control result: v1 at about 6.8s, first researcher update at about
  19.8s, v2 at about 29.8s, then the researcher kept issuing search/fetch calls
  through the rest of the 90s observation window without another
  `submit_coagent_update`, so no v3 arrived. Evidence:
  `test-results/vtext-model-cadence-current-medium-control-20260526T055912Z/fireworks-deepseek-v4-flash-medium.json`.

belief-state changes:

- New computers are no longer stuck on generated V4 Flash `low`/`none` policy;
  the deployed default path is using `medium` for VText/researcher.
- V4 Flash `low` remains unsuitable for this product path because it can spawn
  research and still fail to deliver the first findings update.
- V4 Flash `medium` is better, but still starves the revision cadence after the
  first partial checkpoint. The visible v2 may promise follow-up research or
  comprehensive completion even when no new VText-driving update is actually
  delivered.
- The next prompt-only fix should tighten the coagent communication contract:
  researchers that continue past the first checkpoint must send incremental
  updates before another search batch, and VText must not write "follow-up
  researcher dispatched" or equivalent future-tense coordination claims unless
  it actually uses `spawn_agent` in that turn.

remaining error field:

- Bootstrap route pressure is real and should be tracked separately from model
  quality; manual mobile UX can look broken even when vmctl aggregate health is
  `ok`.
- V4 `medium` simple sports cadence still lacks reliable v3 convergence.
- Prompt-only coordination has not yet been exhausted for this simple sports
  failure; unlike the long mixed command rubric, this can still plausibly be
  improved without a role-specific harness branch.

next executable probe:

- Strengthen only role/tool prompt contracts for researcher incremental
  checkpoints and VText truthful coordination language. Then rerun the two
  focused controls before returning to the strict long-section matrix.

checkpoint update, 2026-05-26 06:18 UTC:

- Prompt-only cadence fix `3896d22424e9347a0eaa3768f40dfe0e4af3c145`
  landed and deployed. CI run `26435347929` passed, FlakeHub run
  `26435347931` passed, and staging `/health` reported proxy and sandbox
  deployed at `3896d22424e9347a0eaa3768f40dfe0e4af3c145`, deployed at
  `2026-05-26T06:06:06Z`.
- Local focused proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestVTextPromptForFactualFirstRevisionForbidsUngroundedContent|TestVTextPromptForPartialFindingsForbidsFalseFollowupClaims|TestVTextResearchContinuationObjectiveRequiresFastCheckpoint|TestSystemPromptForResearcherForcesEarlyHandoff|TestVTextPromptCurrentEventsRequiresResearcher|TestInitialVTextToolChoiceUsesExactTools|TestVTextInitialEditContinuationClassifiesPrompts|TestCompactWebSearchProjectionCanRequireResearchFindingsCheckpoint|TestShouldRequireResearchFindingsAfterSearchOnlyForFirstResearcherSearch' -count=1`.
- Focused deployed V4 medium command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-medium VTEXT_MODEL_PROMPTS=baseball VTEXT_MODEL_CADENCE_EVIDENCE_DIR=../test-results/vtext-model-cadence-3896d22-medium-control-20260526T060706Z npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Result: the prompt-only fix did not solve the simple sports v3 starvation.
  Evidence:
  `test-results/vtext-model-cadence-3896d22-medium-control-20260526T060706Z/fireworks-deepseek-v4-flash-medium.json`.
- Observed transition: V4 medium produced v1 at about 4.2s, a researcher was
  spawned, the first researcher update arrived at about 38.2s, and v2 arrived
  at about 49.2s. The researcher then continued search/fetch calls through the
  rest of the observation window without sending another
  `submit_coagent_update`, and no v3 arrived.
- Improvement: the v2 no longer claimed that a follow-up researcher had already
  been dispatched. It stayed closer to honest partial status.
- Remaining failure: prompt guidance alone did not make the researcher produce
  incremental updates after the first checkpoint, even while it continued
  issuing additional search/fetch calls.

belief-state changes:

- For simple sports cadence, prompt-only is now falsified for the post-first
  checkpoint researcher starvation transition on V4 Flash medium.
- The existing `next_required_tool` mechanism already works for the first
  researcher checkpoint and for super bash evidence. Extending that same
  existing tool-result contract to later researcher search/fetch batches is now
  the smallest plausible non-wrapper change. It does not grant researchers new
  authority or let them write canonical text; it only requires them to report
  the evidence they just gathered before continuing.

remaining error field:

- The next code change should use the existing uniform tool-loop
  `next_required_tool` contract, not a new wrapper or role-specific core-loop
  branch: after a researcher has already checkpointed once, any later successful
  search/fetch batch that occurs after the latest `submit_coagent_update`
  should require another `submit_coagent_update` before more searching.

next executable probe:

- Implement the narrow researcher post-checkpoint reporting contract through
  existing search/fetch tool result metadata. Rerun the V4 medium baseball
  control and inspect whether a second researcher update and v3 appear without
  a search loop.

checkpoint update, 2026-05-26 06:26 UTC:

- Researcher follow-up reporting contract `52e199737acb003f5840617cab3aaec146b6962d`
  landed and deployed. CI run `26435606338` passed, FlakeHub run
  `26435606320` passed, and staging `/health` reported proxy and sandbox
  deployed at `52e199737acb003f5840617cab3aaec146b6962d`, deployed at
  `2026-05-26T06:14:07Z`.
- Local proof passed:
  `nix develop -c go test ./internal/runtime -run 'TestCompactWebSearchProjectionCanRequireResearchFindingsCheckpoint|TestShouldRequireResearchFindingsAfterResearchToolBatches|TestVTextPromptForPartialFindingsForbidsFalseFollowupClaims|TestPersistentSuperInboxBashRequiresCoagentUpdate' -count=1`.
- Focused deployed V4 medium command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-medium VTEXT_MODEL_PROMPTS=baseball VTEXT_MODEL_CADENCE_EVIDENCE_DIR=../test-results/vtext-model-cadence-52e1997-medium-control-20260526T061509Z npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
- Result: the revision cadence failure is fixed for this focused V4 medium
  sports row. Evidence:
  `test-results/vtext-model-cadence-52e1997-medium-control-20260526T061509Z/fireworks-deepseek-v4-flash-medium.json`.
- Observed transition: VText produced 4 appagent revisions instead of stalling
  after v2. Researcher produced 5 coagent updates and only 3 search queries,
  instead of running a long silent search/fetch loop. There were zero
  duplicate-tool, stale-VText, mutation-state, or edit_vtext errors.
- Remaining quality issue: the row still did not reach final baseball scores.
  Researcher reported that ESPN and MLB.com scoreboard fetches were blocked
  with 403 responses and CBS Sports returned 404, then shifted to recap search.
  The final observed v4 document was honest but incomplete:
  "Scoreboard sources blocked; searching news recaps for final scores and
  storylines."

belief-state changes:

- The manual "stuck on v1/v2 and fails to get to v3" symptom was a real
  researcher-reporting cadence bug, not just a reasoning-level setting. The
  existing `next_required_tool` continuation contract fixed that shape without
  adding wrapper tools or allowing researchers to write canonical VText.
- V4 Flash medium can now keep VText moving through multiple sports revisions,
  but source acquisition quality remains a separate frontier. Current search
  and fetch paths may not reliably recover live scoreboard data when
  authoritative scoreboard pages block extraction.

remaining error field:

- Need a source-quality probe across V4 medium, Kimi low, and GPT-5.5 low:
  determine whether final-score failure is model-specific, provider/search
  substrate-specific, or a prompt/source-strategy issue.
- VM/bootstrap lifecycle counters remain noisy even while aggregate
  `vmctl_status=ok`: latest health still showed bootstrap `http_502=8` and
  `resolve_error=15` cumulatively.

next executable probe:

- Run the accepted comparison set on the focused sports row, then return to the
  strict long-section research-plus-super rubric only if the sports row shows
  stable v3+ cadence across Kimi low, V4 medium, and GPT-5.5 low.

suggested resume goal string:

- Use the `/goal` text above.

evidence artifact refs:

- Local Playwright result: `frontend/test-results/vtext-document-stream-vtex-d34e2-instead-of-losing-the-draft-chromium/`
  captured the pre-refresh-gap failure.
- Deployed dirty-draft evidence:
  `test-results/vtext-durable-draft-staging-b2252fe-20260525T232239Z/dirty-rebase-product-path.json`.
- Model-suite evidence:
  `test-results/vtext-model-suite-durable-draft-b2252fe-20260525T232318Z/`.
- Model-suite long rerun evidence:
  `test-results/vtext-model-suite-durable-draft-b2252fe-long-rerun-20260525T233946Z/`.
- v4 long 180s rerun evidence:
  `test-results/vtext-model-suite-durable-draft-b2252fe-v4-long-180s-20260525T234456Z/`.
- Eval report:
  `docs/vtext-durable-draft-version-graph-eval-report-2026-05-25.md`.
- Deployed worker-follow-up evidence:
  `test-results/vtext-durable-draft-worker-concurrency-staging-2cf7253-20260526T000659Z/dirty-user-edit-worker-followup.json`.
- Final deployed model-suite evidence:
  `test-results/vtext-model-suite-durable-draft-2cf7253-20260526T000948Z/`.
- Deployed two-researcher storm evidence:
  `test-results/vtext-durable-draft-multi-worker-staging-a2fe62f-20260526T003647Z/dirty-user-edit-two-worker-updates.json`.
- Deployed two-researcher pending-drain evidence:
  `test-results/vtext-durable-draft-multi-worker-drain-staging-a2fe62f-20260526T005412Z/dirty-user-edit-two-worker-updates.json`.
- Deployed long-section rubric evidence:
  `test-results/vtext-long-section-rubric-staging-2cf7253-full-20260526T010650Z/`.

rollback refs:

- Pre-mission code state: `cf446024a94cb1cd87afb4a593ba717c692ff5a6`.
