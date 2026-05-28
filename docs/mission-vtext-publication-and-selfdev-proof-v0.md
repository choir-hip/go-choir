# MissionGradient: VText Publication + Self-Development Proof v0

Status: draft for owner review
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

## Suggested First Inner Prompt

Submit after P0 publish proof:

```text
Use Choir to safely build and verify one small research-backed code change in a candidate worker computer. Choose a CPU-only arXiv paper or comparable primary source with a compact algorithm, metric, or toy result that can be implemented with a tiny deterministic fixture and focused tests. Create a VText mission narrative, ask super to delegate durable repo/app work to a worker VM, and have vsuper coordinate one implementation co-super and one verifier co-super. The worker should implement the smallest useful library/CLI/demo slice and publish an AppChangePackage only if tests and human-readable proof exist. The verifier must independently inspect the source refs, command output, and proof artifact before approval. Record exact VText revisions, Trace ids, worker VM/vsuper/co-super ids, channel messages, commands, package id, rollback refs, blockers, and next safe probe. Do not use GPU, large datasets, manual deploys, internal/test success routes, fake fixtures, or canonical mutation without verified promotion.
```
