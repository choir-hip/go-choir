# MissionGradient: Choir-in-Choir Deformation v0

Status: runnable mission geometry; v0 executable slice completed 2026-05-13
Date: 2026-05-13

## Real Artifact

Choir-in-Choir self-development loop: Choir uses its own desktop, super/vsuper/cosuper authority geometry, run memory, background VMs, verifier contracts, candidate-world promotion, and product UI to improve Choir through verified deltas.

The goal is not to make a better Codex wrapper. Codex stops when a goal completes. Choir should keep optimizing: when a verified goal finishes, the system compacts the run, selects or synthesizes the next objective from durable evidence, starts the next bounded run in the right execution world, and continues until stopped by invariant-level risk, exhausted authority, or explicit human pause.

## Current State

Already present or recently added:

- Durable run memory v0: persisted tool-loop messages, compaction entries, manual compaction, threshold compaction, context-overflow retry, blocked overflow state, and child-run memory proof.
- Candidate-world promotion v0: candidate identity, branch-per-candidate import, verifier contracts, divergence blocking, explicit promotion, and rollback evidence.
- Background worker VM primitives: `request_worker_vm`, `delegate_worker_vm`, and worker-side `export_patchset`.
- Product surfaces that can become dogfood targets: Files, Launcher, Settings/theme preview, ContentViewer podcast display, VText, Trace, Browser, Terminal.
- Playwright coverage and auth helpers in `frontend/tests`, plus frontend build scripts.

Important current gaps:

- No promotion queue product/runtime bridge yet: exported patchsets do not become first-class reviewable promotions in the live product path.
- No automatic next-goal continuation: a completed run does not synthesize and start the next run from durable memory/evidence.
- Run recovery after process restart is still not full continuation; interrupted running tasks can be marked failed.
- The `Launcher.svelte` component exists, but shell tests currently assert no launcher toggle in the desktop path.
- Files lacks upload UI/API; tests explicitly work around the missing upload API.
- Theme validation exists, but Settings says theme editing is not user-facing.
- Podcast support exists as feed rendering/audio playback in `ContentViewer`; it is not yet a full podcast/radio app.
- Super/cosuper exist; vsuper is not yet a first-class runtime role tied to a candidate VM sovereign worker.

## Value Criterion

Maximize verified Choir self-improvement per unit of human monitoring while minimizing canonical-state corruption, unreviewed mutation, hidden context loss, verifier Goodharting, deadlock, authority leakage, product regressions, and dead-end work that cannot be promoted.

The optimization target is:

```text
J = verified_product_and_architecture_improvement
    - canonical_corruption_risk
    - hidden_state_or_context_loss
    - human_review_burden
    - verifier_bypass
    - authority_leakage
    - rollback_cost
```

The run should select work from the error field: where the self-development loop cannot yet continue, verify, promote, recover, or expose product value.

## Invariants

- Foreground desktop and canonical repo state are not speculatively mutated.
- Candidate worlds mutate only inside background VM or integration-branch boundaries.
- Every candidate world has owner, parent run, candidate run, VM, base SHA, worker head or patchset, verifier contracts, evidence, promotion decision, and rollback point.
- Super owns orchestration and promotion. Vsuper owns one candidate VM. Cosupers are subordinate helpers and cannot mint authority upward.
- Appagents own semantic artifact changes. VText remains the semantic substrate for documents/radio.
- Researchers write epistemic state, not filesystem patches.
- Workers produce candidates. Super integrates and verifies. Canonical state changes only by explicit promotion.
- Every loop has one owner, one bounded lease, durable messages, idempotent events, and no circular synchronous wait.
- A completed run compacts into operational memory before any automatic continuation decision.
- Automatic continuation can start only bounded next runs with explicit authority profiles and stop conditions.
- Playwright/Codex bootstrapping may drive the product at low resolution, but the proof path must be continuously deformable into Choir driving itself.
- Product work must land through the same candidate/promotion path as architecture work.

## Homotopy

This is one system at increasing resolution, not a ladder of unrelated MVPs.

### Lambda 0: Codex Drives Choir As External Hands

Codex uses local tools and Playwright to run Choir, submit prompts, inspect Trace, and verify UI. This is acceptable only as a bootstrap proxy for a future vsuper/browser capability.

Required topology:

- prompts enter through the Choir product path where possible;
- generated work is exported as candidate patchsets or local integration candidates;
- verification evidence is saved in repo docs/tests, not only chat;
- no direct product success is claimed from a manually edited artifact alone.

### Lambda 0.2: Promotion Queue v0

Connect candidate-world promotion v0 to a runtime/product queue:

- background VM export results become candidate promotion records;
- users can inspect candidate metadata, changed files, verifier contracts, verification results, and rollback command;
- super/platform can run verification on an integration branch;
- only verified candidates expose promotion;
- divergence blocks direct promotion and asks for integration/reverify.

This turns candidate-world promotion from a library proof into a usable bridge.

### Lambda 0.35: Vsuper As Candidate-World Sovereign

Add vsuper as a first-class role/record:

- one vsuper per background VM by default;
- vsuper has candidate-world mutation authority only inside its VM;
- vsuper can spawn VM-local cosupers;
- vsuper cannot promote or mutate foreground canonical state;
- vsuper exports patchsets, verification artifacts, and run compactions.

This should mostly refine current worker VM/co-super behavior, not replace it.

### Lambda 0.5: Automatic Next-Goal Continuation

Build run-continuation control:

- every completed run writes an operational compaction;
- the system derives a next objective from mission docs, run memory, verifier failures, product gaps, and promotion queue state;
- continuation starts only if the next objective fits an allowed authority profile and bounded lease;
- continuation emits a durable "goal selected" record with reasons, constraints, and stop condition;
- if no safe next goal exists, Choir stops with a review packet instead of drifting.

This is the key Codex-surpassing behavior: completing a goal does not end inference; it updates the control state.

### Lambda 0.65: Product Work As Self-Development Pressure

Dogfood one narrow product patch through the full loop. The preferred pressure source is launcher/uploads/themes because it is visible, bounded, and currently half-built:

- make bottom-left control an actual start/app launcher path;
- add desktop app icons where missing;
- add file upload UI/API to Files;
- expose theme creation/editing as validated config, not arbitrary CSS edits;
- add a small set of theme presets only as examples of the user-editable theme system.

The system should choose one thin vertical slice first, then continue automatically into the next slice if verification passes.

### Lambda 0.8: Podcast/Radio As Semantic Product Target

Use the podcast app/radio direction as the second product pressure source:

- podcast feeds become durable content artifacts, not just display rendering;
- VText/radio can reference episodes, clips, narration beats, sources, and listen paths;
- appagent ownership and researcher updates feed podcast/radio state;
- UI remains a projection of vtext semantics, not a standalone media toy.

This should follow promotion queue and continuation, because it has higher semantic scope.

### Lambda 1: Choir Develops Choir

Choir starts with a high-level mission, decomposes it into owned runs, uses background VMs for speculative mutation, runs verifier contracts, promotes verified deltas, compacts learning, synthesizes the next mission, and continues across O(8h) leaps without relying on Codex to pick up the thread.

Codex remains a bootstrap/verifier/operator, not the center of cognition.

## Dense Feedback Channels

- Go tests for runtime, store, promotion, shipper, vmctl, and vmmanager.
- Frontend build: `cd frontend && npm run build`.
- Focused Playwright tests for the touched product surface.
- Product-path Playwright dogfood: prompt bar -> conductor/super -> background VM -> export -> promotion queue -> verified promotion.
- Trace assertions for roles, tool calls, candidate-world records, verifier events, and promotion events.
- Git assertions: base SHA, worker head, integration branch, destination branch, dirty state, rollback command.
- Run memory assertions: compaction before continuation and durable next-goal selection records.
- Promotion queue assertions: created, verified, blocked-on-divergence, promoted, rejected.
- Screenshots/video only where they verify real UI state, not as cosmetic proof.

## Forbidden Shortcuts

- Do not replace Choir-in-Choir with Codex directly editing files and writing a success report.
- Do not mark a goal complete merely because a local test passed; require durable evidence and next-state decision.
- Do not let workers push, promote, or mutate canonical foreground state.
- Do not create a verifier-agent caste when a verifier contract and capability profile suffice.
- Do not make app launcher, uploads, themes, or podcasting one-off UI patches that bypass promotion.
- Do not seed promotion records manually in tests if the product path can create them.
- Do not treat Playwright-only UI interaction as self-development unless it produces candidate artifacts and verifier evidence.
- Do not let automatic continuation run without bounded authority, stop conditions, and rollback.
- Do not collapse vsuper into generic co-super if the candidate VM boundary matters.
- Do not hide failed candidates; failed worlds should leave diagnostics, failed-route records, and learning.

## Rollback Policy

Git:

- every candidate records base SHA and worker head;
- every integration branch is `agent/<run>/<slug>`;
- promotion is fast-forward or explicit merge only after verification;
- divergence blocks promotion;
- rollback command is saved in the promotion report.

VM:

- candidate VM leases expire;
- failed worlds can be discarded without foreground mutation;
- successful worlds export patchsets and reports before teardown.

Database/runtime:

- migrations must be additive and covered by tests;
- promotion queue records are append/update state machines, not destructive rewrites;
- run memory entries are append-only.

Product:

- UI/product dogfood patches must have focused Playwright or component tests;
- theme/user-generated config must validate before application;
- file upload paths must enforce per-user file-root boundaries.

## Learning Side-Channel

Every long run must update durable learning artifacts:

- mission doc completion notes;
- dogfood proof report;
- next-frontier report;
- run memory compaction;
- promotion reports for every candidate;
- failed-candidate report when applicable.

Learning classification:

- Tactical learning: update implementation route/tests immediately.
- Target-level learning: update the mission doc or next-frontier report.
- Invariant-level learning: stop and ask before changing authority, trust boundary, promotion semantics, or canonical mutation rules.

## 8-24 Hour Run Shape

The next long run should not try to build every product feature. It should build the continuation machinery that lets Choir keep going, then use one product slice as live pressure.

Minimum ambitious target:

- promotion queue v0 wired to exported background VM patchsets;
- vsuper role/record sketched or implemented where it clarifies worker VM ownership;
- automatic next-goal selection record after a verified run;
- one launcher/uploads/themes product patch dogfooded through candidate-world promotion;
- Playwright proof that Codex can bootstrap Choir through the product path for the first loop;
- final report explaining what the system would choose next without Codex intervention.

Stretch target:

- after first verified product patch, automatic continuation starts a second bounded candidate run;
- second run tackles the next product slice, likely file upload UI/API or theme editor scaffolding;
- failed or blocked second run still leaves compaction and next-goal evidence.

## Suggested First Product Slice

Use launcher/uploads/themes, not podcast/radio, for the first full self-development dogfood:

- Launcher is visible and already partially present.
- Files upload is concrete and currently missing.
- Theme editing has a schema and validation but no user-facing editor.
- These features exercise desktop UX, APIs, tests, and promotion without requiring the full semantic radio architecture.

Podcast/radio should become the next major product target once continuation and promotion queue are real.

## Stopping Condition

Stop only when either:

- Choir has a documented and tested deformation from Codex-driven bootstrap to Choir-controlled self-development, with promotion queue, continuation records, vsuper/candidate-world authority, and one verified product patch through the promotion path; or
- the run is blocked with the failed invariant, rollback point, evidence, and next smallest safe probe written in a repo doc.

Completion requires a next-state decision. If a goal completes and the system can safely continue, the artifact should include the next objective record instead of ending as a dead stop.

## v0 Execution Evidence

The 2026-05-13 run completed the first executable deformation slice:

- promotion queue runtime/store bridge;
- `vsuper` as a first-class candidate-world profile;
- automatic continuation records and metadata-driven auto-start;
- delegate-worker export results queued as promotion candidates;
- launcher/uploads/themes dogfood candidate patch verified and promoted through the runtime queue test.

Evidence docs:

- `docs/choir-in-choir-deformation-v0-dogfood-2026-05-13.md`
- `docs/choir-in-choir-next-frontier-2026-05-13.md`

Verification command:

```sh
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' \
go test ./internal/store ./internal/runtime ./internal/promotion
```

Residual risks are explicit: promotion queue UI, full objective synthesis, real launcher/uploads/themes/files implementation, live VM rollback proof, and Playwright prompt-bar bootstrap remain for the next mission.

## One-Line Goal

`/goal Use MissionGradient to execute docs/mission-choir-in-choir-deformation-v0.md for an 8-24h run, wiring promotion queue and automatic continuation toward Choir-in-Choir, then dogfooding one launcher/uploads/themes patch through verified candidate-world promotion.`
