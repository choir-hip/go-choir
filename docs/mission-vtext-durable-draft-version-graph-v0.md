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

- Existing runtime appears to have stale-head guards, user-authored revision
  creation, clean auto-follow, dirty no-clobber UI, autosave user revisions, and
  worker-update batching, but the durable draft/merge semantics are not yet
  specified or broadly proven.
- Working tree now contains an unlanded contract-preserving candidate fix:
  explicit `allow_rebase` user revision saves can rebase stale dirty drafts onto
  the current head while ordinary stale writes still return conflict. This must
  be committed, pushed, deployed, and verified before it counts as shipped.

what shipped:

- Nothing yet.

what was proven:

- Local focused runtime proof:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextCreateRevisionRejectsStaleHead|TestVTextCreateRevisionRebasesAllowedStaleUserDraft|TestVTextStaleAgentRevisionRejectsEditAfterUserEdit' -count=1`
  passed.
- Local product-path browser proof:
  `pnpm exec playwright test tests/vtext-document-stream.spec.js --project=chromium --grep 'auto-follows|rebases dirty|reopening the same file|restores on reload' --reporter=line`
  passed against the local service stack.

unproven or partial claims:

- durable cross-device draft sync before `Revise`;
- merge/rebase behavior when head changes while user is dirty;
- long-content/many-version/concurrent-worker robustness;
- whether autosave-as-canonical-user-version is the correct durable draft model.

belief-state changes:

- The smallest useful first cut is not WebSocket transport. It is explicit
  stale user draft rebase semantics on the existing product revision API, plus
  focused-editor refresh after a successful rebase response. WebSocket remains a
  possible later transport for lower-latency draft replication, but not the
  current load-bearing uncertainty.

remaining error field:

- Need commit separation, CI, staging deploy, staging product-path proof, and
  model-suite eval report for Kimi low, v4-flash medium, and GPT-5.5 low.
- Need broader long-content/many-version/concurrent-worker evaluation; current
  local proof only covers the first dirty-head rebase edge plus existing stream
  behavior.

highest-impact remaining uncertainty:

- Durable draft state model between keystrokes and canonical user-authored
  versions.

next executable probe:

- Commit this mission checkpoint before the behavior fix, then commit the
  rebase implementation and tests. Push through the staging landing loop and run
  deployed dirty-head proof before expanding to the model-suite report.

suggested resume goal string:

- Use the `/goal` text above.

evidence artifact refs:

- Local Playwright result: `frontend/test-results/vtext-document-stream-vtex-d34e2-instead-of-losing-the-draft-chromium/`
  captured the pre-refresh-gap failure.

rollback refs:

- Pre-mission code state: `cf446024a94cb1cd87afb4a593ba717c692ff5a6`.
