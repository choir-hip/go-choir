# MissionGradient: VText Publication + Self-Development Proof v0

Status: checkpoint_incomplete — export-level reached, promotion blocked by review-proof linkage
Date: 2026-05-28

## Goal String

```text
/goal Run docs/mission-vtext-publication-and-selfdev-proof-v0.md as a Codex-operated MissionGradient mission: make Choir's VText and self-development substrate visibly real before expanding product surface. Start by verifying and repairing plain VText publication so clicking Publish opens or presents a shareable https://choir.news/pub/vtext/... permalink that renders the published VText for logged-out users. Then use the visible prompt bar on choir.news to run one complex but bounded research+coding workload that requires super -> worker VM -> vsuper -> implementation co-super + verifier co-super, preferably implementing a small CPU-only algorithm from an arXiv paper or comparable primary source, with research evidence, code changes, tests, human-readable VText narrative, Trace evidence, and an AppChangePackage. If export-level evidence works, prove the package is reviewable in the Apps/Changes surface and, only if verifier evidence and rollback refs are present, proceed to recipient adoption/promotion proof. Do not use manual deploy shortcuts, internal/test success routes, fake fixtures, host-only proof, or direct canonical mutation by workers. Preserve VText as the only canonical document writer, keep candidate work in candidate/worker boundaries until verified, use Computer Use plus product APIs for deployed proof, and perform deletion-first cleanup only after live proof identifies dead paths. Stop at the highest honest evidence level reached with exact IDs, screenshots/Computer Use observations, CI/deploy identity for platform changes, rollback refs, residual risks, and the next mission string.
```

## Current Evidence Read

Recent evidence changed the mission shape:

- VText is no longer simply "stuck at v1" for the current demo path. Deployed commit `408884a113a02ea69a388ade83daeadd1a27f51e` proved default-policy factual and bounded coding prompts can reach worker evidence and VText v2.
- Computer Use in Comet confirmed the live `choir.news` UI can submit a prompt, open a VText window, and display a v2 containing exact shell command output.
- The same Computer Use pass exposed a remaining UI-state defect: the document content and Chyron showed completion while the VText toolbar still displayed `Revising...` with Cancel visible.
- VText publication already has backend/proxy/platform code (`/api/platform/vtext/publications`, `/pub/vtext/...`, logged-out VText reader), but the product behavior is not acceptable yet: Publish should yield an immediately usable public permalink, preferably by opening it or presenting a copyable/openable link.
- AppChangePackage/adoption surfaces exist (`AppsChangesApp`, `/api/app-change-packages`, `/api/adoptions`, run acceptance synthesis), but the next proof must show they work through a real worker/candidate trajectory, not by treating their existence as proof.

The next mission should therefore be a proof-and-convergence mission, not a broad architecture rewrite.

## Execution Checkpoint: 2026-05-28

This mission reached the highest honest evidence level currently available:
`export-level`. It did not reach promotion/adoption-level.

### Platform Repair And Deploy

VText publication was repaired and deployed.

- Problem documentation commit: `bf7a4b0` (`docs: record VText publication proof gap`).
- Code repair commit: `41991763b0ea61fc9442bd14aa6c3353252811c6` (`fix: make VText publish links usable`).
- CI run: `26552436491`, passed.
- FlakeHub run: `26552436506`, passed.
- Deployed proxy commit: `41991763b0ea61fc9442bd14aa6c3353252811c6`.
- Deployed sandbox commit: `41991763b0ea61fc9442bd14aa6c3353252811c6`.
- Deploy timestamp: `2026-05-28T03:16:51Z`.

Supporting local checks:

```text
pnpm --dir frontend build
nix develop -c go test ./internal/store -run TestVTextAgentMutationMarkStaleClearsPending
nix develop -c go test ./internal/runtime -run TestRunToolLoopBoundsRequiredNextToolProviderCall
nix develop -c go test ./internal/store -run 'TestVTextAgentMutation(Complete|MarkStaleClearsPending)'
```

Known unrelated local blocker:

```text
nix develop -c go test ./internal/runtime -run ...
```

failed before mission-specific execution because `strconvQuote` is redeclared in
`internal/runtime/content_test.go` and
`internal/runtime/researcher_checkpoint_fallback.go`.

### Public VText Proof

Computer Use and a no-storage browser proof verified that publishing now yields
a usable public permalink:

```text
https://choir.news/pub/vtext/computer-use-verification-write-and-run-a-tiny-shell-pub20e2c0b75
authState: signed_out
reader: Published VText
content: Computer Use Verification
evidence id: a3e93e9a-e5ca-5972-8182-6b7bcc56dadc
```

The owner VText toolbar also recovered from stale `Revising...` state on the
completed shell-command document: `Latest` was visible and `Publish v2` was
enabled.

### Self-Development Prompt

The complex workload was submitted through the visible prompt bar on
`https://choir.news`, not through an internal/test route.

Prompt summary:

```text
Create a VText mission narrative. Research Webber, Moffat, Zobel 2010
Rank-Biased Overlap, use arXiv:2406.07121 only as tie-handling context, lease a
background candidate worker VM, delegate to vsuper, coordinate separate
implementation and verifier co-super agents, implement a tie-free finite RBO Go
package, verify it, publish an AppChangePackage, and do not claim adoption or
promotion unless Apps/Changes and rollback evidence are present.
```

Key IDs and evidence:

- Trajectory/submission: `50ec2a07-1b41-4d45-8fa8-3693941ffc73`.
- VText doc/channel: `7b7437b1-15f8-4042-a322-af0ed3c62e4d`.
- Final observed VText revision: `d52fc738` after worker completion.
- Researcher run: `8cfb9c6b-d0ec-44ea-ae6d-f6f4c5ef97ea`.
- Super run: `e00bb7eb-9030-420f-a8fb-2a5a8da57d67`.
- Worker VM: `worker-10d6d2f2dcadd342`.
- Worker vm id: `vm-b11826f8405cbb5904312d484881f3c5`.
- Vsuper/worker run: `81968656-c2f3-441b-967a-d0cc65908dcc`.
- Implementation co-super agent/run:
  `016693a3-716c-4c20-9b39-45e59e0d0dc9` /
  `03bd50ae-223d-42c7-be05-2af619cc3f66`.
- Verifier co-super agent/run:
  `03dcbb76-ffc9-43bc-995f-3c4d7a3f89e3` /
  `240629d6-b5fb-4d57-b4cd-2b83118a0945`.

Research evidence:

- Primary source identified as Webber, Moffat, Zobel (2010), "A Similarity
  Measure for Indefinite Rankings", DOI `10.1145/1852102.1852106`.
- Tie-handling paper `arXiv:2406.07121` was treated as context only and out of
  scope for the implemented tie-free algorithm.
- Foreground sandbox had no Go repo, so super correctly leased a worker VM.

Worker/candidate evidence:

- Candidate repo base SHA: `41991763b0ea61fc9442bd14aa6c3353252811c6`.
- Candidate commit: `a25de9c1192e8ea4f7d3b17bd9d9ffda7f6d874e`.
- Candidate ref: `refs/computers/primary/candidates/81968656-c2f3-441b-967a-d0cc65908dcc`.
- Implemented files:
  - `internal/rbo/rbo.go` (73 lines).
  - `internal/rbo/rbo_test.go` (272 lines).
- Verified formula: `RBO(S,T,p) = (1-p)/p * sum_{d=1..k}(X_d * p^d) + X_k * p^k`.
- Verifier result: PASS, 100.0% statement coverage, independent Python
  cross-validation matched all 18 expected values within `1e-15`.

Package evidence:

- AppChangePackage: `84c12250-2d0b-43f3-b5ed-90f8e051634e`.
- Status: `published_unlisted`.
- Human proof state: `evidence_pending`.
- Package publish evidence:
  - `worker_tool_invoked_event:9c6858db-219a-4cb4-a8e0-215e900cc51e`
  - `worker_tool_result_event:7b1f3795-9921-43e0-a205-e47acd2cda36`
- Rollback base/ref:
  - platform base `41991763b0ea61fc9442bd14aa6c3353252811c6`.
  - candidate ref `refs/computers/primary/candidates/81968656-c2f3-441b-967a-d0cc65908dcc`.

### Apps/Changes Proof And Blocker

Computer Use verified that Apps & Changes shows the package, but not as a
reviewable/adoptable package:

```text
Review changes before they enter this computer
0 ready, 1 pending
## Tie-Free Finite Rank-Biased Overlap (RBO) — Go Implementation
EVIDENCE PENDING
Needs human proof
Missing: causal VText narrative, screenshot, video, or benchmark evidence
Try in candidate: disabled
Verify build: disabled
Install: disabled
Rollback: disabled
Open Trace evidence: disabled
Trajectory: not created
Acceptance: not synthesized
State: available
Evidence: pending
```

Clicking `Open VText narrative` produced the product message:

```text
This change does not include a VText narrative yet.
```

Trace also exposed a lifecycle/convergence problem:

```text
trajectory: 50ec2a07-1b41-4d45-8fa8-3693941ffc73
visible state: failed
agents: 4
delegations: 4
moments: 519
messages: 38
compactions: 2
retries: 12
latest activity: 2026-05-28T03:36:04Z
```

The worker path completed and exported a package, but the foreground trajectory
failed after completion and the package did not carry enough human-proof
metadata for Apps/Changes to make it reviewable. This blocks adoption/promotion
and run-acceptance synthesis through the product surface.

### Current Diagnosis

The self-development substrate is real enough to prove:

```text
visible prompt bar
  -> VText narrative
  -> researcher
  -> super
  -> worker VM
  -> vsuper
  -> implementation co-super
  -> verifier co-super
  -> candidate commit
  -> AppChangePackage visible in Apps/Changes
```

The substrate is not yet converged enough to prove:

```text
AppChangePackage visible
  -> human-reviewable package
  -> candidate try/build verification
  -> owner install/adoption
  -> rollback evidence
  -> run acceptance synthesis
```

The likely product gap is review-proof linkage and finalization sequencing:
the VText narrative revision exists after worker completion, but the package
was published as `evidence_pending` without linked narrative refs or another
accepted human-proof artifact. Apps/Changes therefore correctly blocks install
and recovery actions.

### Cleanup Decision

Deletion-first cleanup is deferred. Live proof did identify a real dead-end
state, but it is on the package review/adoption boundary, not on stale UI or
obvious dead code. Cleaning before repairing this linkage risks deleting paths
needed to understand the failed export-to-review transition.

## P0 Live Evidence: Publication Permalink

Observed on `https://choir.news` with deployed proxy/sandbox commit
`408884a113a02ea69a388ade83daeadd1a27f51e`.

Computer Use observations:

- Existing completed document `Computer Use verification: write and run a tiny
  shell` shows `v2` content with exact command output and completed Chyron
  events, but the VText toolbar remains in `Revising...`; `Publish v2` is
  disabled and `Cancel` remains visible.
- Older document `What does "elan" mean` shows `Latest` and an enabled
  `Publish v1` button.
- Clicking `Publish v1` succeeds and renders an in-window publication result:
  `/pub/vtext/what-does-elan-mean-pub18e0fa2ba`, content hash prefix
  `5c79a1006e...0a7ca0`, publication version prefix
  `pubver-733...520b37`.
- The result route is rendered as a heading/text panel, not as an obvious
  absolute `https://choir.news/...` link, copy control, or open action. Clicking
  the route text did not navigate in Comet.

Logged-out route proof:

```text
https://choir.news/pub/vtext/what-does-elan-mean-pub18e0fa2ba
authState: signed_out
content visible: Meaning of "elan"
mutation affordance: Edit visible, expected to require auth before mutation
```

Named P0 problems before code changes:

1. Publish succeeds and the public route resolves logged out, but the owner UI
   does not make the permalink immediately usable enough. It should open the
   published route or present an absolute copyable/openable link.
2. Completed VText runs can leave the toolbar in stale `Revising...` state,
   disabling Publish even when the latest document content and Chyron show the
   run completed.

## Cognitive Transforms

Current uncertainty or obstacle:

```text
Choir has many substrate pieces that appear to exist: VText, publication, worker VMs, vsuper/co-super, AppChangePackage, adoption, Trace, run acceptance. The risk is mistaking code presence or local/API proof for a live self-development loop that a user can see, share, review, adopt, and roll back.
```

Selected transforms:

1. **Depth extraction** - the deep object is not "an agent made a patch"; it is a verified state transition from owner intent to candidate work to reviewable/public artifact.
2. **Homotopy boundary** - start with low-resolution real objects only: a real public VText permalink and one real AppChangePackage. Avoid demo islands that cannot deform into publication/adoption.
3. **Evidence topology** - every claim must map to a causal chain: prompt -> VText -> super -> worker VM -> vsuper -> co-super -> evidence -> package/adoption -> VText/Trace/public UI.
4. **Anti-accretion** - after proof, prefer deleting stale controls, docs, helpers, and UI panels over adding compensating surfaces.
5. **Audience translation** - the owner-facing demo should be simple: publish a document, share a link, ask Choir to build a bounded change safely, inspect it, and adopt or reject it.

Route-changing insights:

- Fixing VText publication first is higher leverage than immediately running a complex worker mission, because public VText is the human-readable proof artifact for everything downstream.
- The complex workload should be a substrate test, not a product grab bag. The chosen paper/algorithm must be small enough to verify in CPU-only worker VMs.
- Promotion is a separate reality-changing act. Export-level proof is valuable; adoption/promotion-level proof requires verifier evidence and rollback refs.
- Cleanup should be gated by live proof. Before proof, "dead code" is a hypothesis; after proof, stale paths become deletable.

Changed plan:

- implementation: first a small VText publish/permalink repair if the current UI fails; then one candidate-world research+coding proof; then adoption/promotion if evidence supports it.
- verifier/evidence: use Computer Use for visible live UI proof and product APIs/Trace for exact IDs; local tests are supporting evidence only.
- scope: no broad App Store redesign, no full package registry, no newsletter/email work, no VText architecture rewrite unless a reproduced transition failure demands it.
- stopping condition: highest honest evidence level, not "everything done overnight."

Next high-information action:

```text
Use Computer Use on choir.news to publish a simple VText and test the logged-out permalink behavior before any self-development workload.
```

## Real Artifact

A deployed, owner-visible proof path:

```text
VText document
  -> Publish
  -> public choir.news permalink readable logged out
  -> visible prompt-bar self-development request
  -> VText mission narrative
  -> super delegates risky durable work
  -> worker VM / vsuper candidate world
  -> implementation co-super + verifier co-super
  -> tested source delta or precise blocker
  -> AppChangePackage with human proof
  -> Apps/Changes review surface
  -> optional recipient adoption/promotion with rollback
```

## Value Criterion

Maximize verified self-development capacity per unit of overnight mutation while minimizing fake progress, hidden state, stale UI, unreviewed canonical mutation, orphan candidate work, and cleanup debt.

The mission moves uphill when:

- Publish produces a permalink an unauthenticated visitor can open.
- VText status accurately reflects completed runs.
- A complex research+coding request crosses the worker VM/vsuper boundary.
- Vsuper coordinates separate implementation and verifier co-super agents.
- The candidate publishes an AppChangePackage with source refs, tests, human-readable proof, and rollback refs.
- Trace, VText, Apps/Changes, and run acceptance all point to the same causal evidence.
- Cleanup deletes obsolete paths after proof instead of adding more structure.

## Hard Invariants

- Staging/live acceptance is `https://choir.news` unless the mission explicitly documents why `https://draft.choir-ip.com` is the correct acceptance surface.
- Platform behavior changes follow `commit -> push origin main -> monitor CI -> monitor deploy -> verify deployed commit -> deployed proof`.
- No manual deploy shortcuts and no direct edits to Node B tracked files.
- Product proof may use visible UI, Computer Use, Playwright, and public authenticated product APIs. Do not use `/internal/*`, `/api/test/*`, `/api/agent/*`, raw event mutation, direct service ports, or manually seeded success records for acceptance.
- VText is the only canonical document writer.
- Researcher, super, vsuper, and co-super send evidence/checkpoints; they do not directly edit canonical VText.
- Candidate/worker code changes remain in candidate boundaries until packaged, verified, reviewed, and promoted/adopted.
- Worker co-super does not verify its own work.
- Public/shared artifacts must not leak private user state or secrets.
- A public VText permalink is a selected projection, not exposure of the entire private computer.

## Control Intervals

### P0 - Publish/Permalink Proof

Use Computer Use on `https://choir.news`:

1. Create or open a simple VText.
2. Click Publish.
3. Verify the UI either opens the public route or presents a copyable/openable absolute `https://choir.news/pub/vtext/...` link.
4. Open the permalink in a logged-out context or private/incognito-equivalent browser session.
5. Confirm the published content is visible without login and mutation actions require auth.

If this fails, document the failure first, then repair the smallest layer:

- likely UI behavior: publish result panel is not a real link/open action;
- possible route/proxy issue: public route does not resolve logged out;
- possible state issue: publication succeeded but desktop did not open reader.

Acceptance:

- public permalink visible/clickable/opened;
- logged-out reader renders content;
- mutation from logged-out reader prompts for auth;
- VText completed state does not remain stuck at `Revising...` after run completion.

### P1 - Complex Research+Coding Workload Selection

Choose a bounded workload that genuinely requires research and coding but fits small CPU-only worker VMs.

Preferred shape:

```text
Find a primary-source arXiv paper with a compact algorithm or metric that can
be implemented in under ~200 lines with deterministic fixtures and no GPU.
Implement a tiny CLI/library/demo plus tests that prove the algorithm on a
small fixture. Do not train models, download large datasets, or require
accelerated compute.
```

Good examples:

- a paper's scoring metric, parser, ranking heuristic, or toy algorithm;
- a small benchmark/fixture reproducing one table row or toy result;
- a deterministic implementation of a known mathematical method described in the paper.

Bad examples:

- model training;
- GPU inference;
- broad "implement the paper" claims;
- UI polish as the main payload;
- static fake demo with no algorithmic/test evidence.

### P2 - VSuper / MicroVM Self-Development Proof

Submit the selected workload through the visible prompt bar.

Required evidence:

- prompt submission id;
- VText doc id and revision ids;
- Trace trajectory/run ids;
- super request to worker VM;
- worker VM id or precise vmctl blocker;
- vsuper run id;
- implementation co-super run id;
- verifier co-super run id or precise reason verifier could not start;
- worker/verifier channel messages;
- source refs and candidate head SHA;
- tests/commands run in candidate context;
- AppChangePackage id or precise blocker;
- human proof refs: screenshot, terminal output, benchmark, or VText narrative as appropriate.

If the flow does not reach vsuper/co-super, investigate and repair the routing/substrate layer before trying a different prompt.

### P3 - AppChangePackage Review And Optional Adoption

If an AppChangePackage is published:

1. Open Apps/Changes in the product UI.
2. Confirm the package is visible and reviewable.
3. Confirm it names source lineage, tests, proof artifacts, and rollback profile.
4. Verify candidate preview or record the precise preview blocker.
5. Run recipient build/adoption verification if the product path supports it.
6. Promote/adopt only when verifier evidence and rollback refs are present.

Do not claim promotion-level evidence for export-only proof.

### P4 - Deletion-First Convergence

Only after P0-P2 proof:

- identify stale docs that contradict current evidence;
- remove or mark obsolete prompt/test scaffolding made unnecessary by proven transitions;
- simplify VText publish UI if duplicate panels/buttons exist;
- remove dead setup/demo cruft around old promotion queues if AppChangePackage/adoption is the active path;
- keep diff near-even or deletion-heavy unless a failing verifier requires code.

If additions exceed deletions by more than 500 lines during cleanup, stop and explain why.

## Dense Feedback

- Computer Use screenshots/observations for publish, logged-out reader, Apps/Changes, and mobile if UI is touched.
- Product APIs: `/api/vtext/*`, `/api/trace/*`, `/api/app-change-packages/*`, `/api/adoptions/*`, `/api/run-acceptances/*`.
- GitHub Actions and deploy identity for behavior-changing platform commits.
- Focused Go tests for runtime, platform publish, app promotion, and worker delegation changes.
- Frontend build and focused browser proof for any UI change.
- Trace raw JSON and run acceptance synthesis for the self-development workload.

## Rollback Policy

- Platform rollback: revert or fix-forward through git, CI, and deploy.
- Publication rollback: use recorded public-route disable rollback ref or publish a corrected version; do not delete provenance.
- Candidate rollback: discard/archive candidate package if verifier fails.
- Adoption rollback: use recorded source rollback refs.
- If a public artifact leaks unintended private data, disable the route first, then preserve a private incident record.

## Stopping Conditions

Stop with `complete` only if:

- public VText publish/permalink works and is verified logged out;
- a complex research+coding workload reaches at least export-level through super -> worker VM -> vsuper -> co-super evidence;
- AppChangePackage review evidence is visible in product UI;
- run acceptance records the honest level reached;
- any behavior-changing commits are deployed and verified;
- cleanup pass completed or explicitly deferred with evidence-based reason.

Stop with `checkpoint_incomplete` if:

- VText publish/permalink is fixed and proven, but worker/candidate proof reaches a precise substrate blocker;
- the complex workload reaches worker evidence but not package export;
- export-level works but adoption/promotion is not yet safe.

Stop with `blocked_incomplete` only after root-cause probes and route-changing cognitive transforms show that continuing requires external authority or would violate an invariant.

## Follow-On Mission String

```text
/goal Run a Codex-operated MissionGradient mission to repair the export-to-review boundary for Choir self-development packages. Start from live evidence: trajectory 50ec2a07-1b41-4d45-8fa8-3693941ffc73 reached worker VM worker-10d6d2f2dcadd342, vsuper run 81968656-c2f3-441b-967a-d0cc65908dcc, candidate commit a25de9c1192e8ea4f7d3b17bd9d9ffda7f6d874e, and AppChangePackage 84c12250-2d0b-43f3-b5ed-90f8e051634e, but Apps/Changes shows it as evidence_pending with no linked VText narrative, disabled Try/Verify/Install/Rollback controls, no trajectory/acceptance synthesis, and the foreground Trace row failed after worker completion. Investigate and document root cause first. Then make the smallest platform/product repair so a verified worker package can carry or attach causal VText narrative refs, verifier evidence refs, candidate refs, rollback refs, and run/trace refs into the Apps/Changes human-proof contract without manually seeding success. Preserve authority boundaries: workers/candidates do not mutate canonical state directly, VText remains canonical document writer, AppChangePackage remains export not adoption, and owner install/promotion requires product review plus rollback evidence. Verify on choir.news through visible prompt bar or an existing package replay path that does not use internal/test routes: package becomes human_reviewable, Open VText narrative works, Try in candidate and Verify build become available or produce precise blockers, run acceptance can synthesize the honest level, and adoption/promotion is attempted only if verifier evidence and rollback refs are present. No manual deploy shortcuts, no fake fixtures, no broad product expansion, and deletion-first cleanup only after the export-to-review transition is live-proven.
```

## Suggested First Inner Prompt

Submit after P0 publish proof:

```text
Use Choir to safely build and verify one small research-backed code change in a candidate worker computer. Choose a CPU-only arXiv paper or comparable primary source with a compact algorithm, metric, or toy result that can be implemented with a tiny deterministic fixture and focused tests. Create a VText mission narrative, ask super to delegate durable repo/app work to a worker VM, and have vsuper coordinate one implementation co-super and one verifier co-super. The worker should implement the smallest useful library/CLI/demo slice and publish an AppChangePackage only if tests and human-readable proof exist. The verifier must independently inspect the source refs, command output, and proof artifact before approval. Record exact VText revisions, Trace ids, worker VM/vsuper/co-super ids, channel messages, commands, package id, rollback refs, blockers, and next safe probe. Do not use GPU, large datasets, manual deploys, internal/test success routes, fake fixtures, or canonical mutation without verified promotion.
```
