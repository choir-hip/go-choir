# Mission: Choir-in-Choir Platform PR Accelerator

**Status:** ready MissionGradient mission  
**Created:** 2026-06-08  
**Primary payload:** Global Wire source truthfulness  
**Meta-goal:** prove Choir-in-Choir can accelerate platform work by producing
reviewable PR/package artifacts without owning platform promotion.

## Goal String

```text
/goal Run docs/mission-choir-in-choir-platform-pr-accelerator-v0.md
```

## Mission Identity

This is not primarily a news mission and not a self-development demo.

The mission has one real object with two inseparable faces:

1. **Self-development face:** Choir-in-Choir must produce a reviewable platform
   artifact through the product path.
2. **News-system face:** the artifact must reduce uncertainty or improve
   behavior for Global Wire source truthfulness.

The intended loop is:

```text
owner intent
-> mission VText
-> super/vsuper worker orchestration
-> worker-medium investigation/implementation candidate
-> worker-playwright proof when browser evidence is needed
-> AppChangePackage and/or GitHub PR as reviewable source artifact
-> verifier evidence
-> mission VText revision with review result and next action
-> Codex/human reviews and lands through normal platform CI/deploy
```

Choir-in-Choir may produce candidate platform work. It does not get to silently
merge `main`, deploy Node B, or claim platform promotion.

If the run improves Global Wire only through Codex edits, it did useful work
but did not satisfy this mission. If the run proves Choir-in-Choir only on a
toy change, it did not satisfy this mission. The value is in the combined
artifact: a real news-system candidate produced through the self-development
loop.

## Why This Matters

Choir needs to become useful for its own development before it can accelerate
the whole project. The Codex-only loop is effective but expensive in attention
and token budget. Choir-in-Choir should absorb more investigation, candidate
implementation, QA, and evidence packaging while keeping platform merge
authority outside the candidate system until the promotion path earns trust.

Global Wire is a good payload because it has live complexity, real source
cycles, processor/reconciler failures, VText articles, browser-visible UI, and
high demand for repeated iteration.

## Current Belief State

- Commit `0610a87a` is on `origin/main` and contains the pre-mission theme
  boot-cache fix and VText toolbar responsive layout fix.
- Staging `/health` reports deployed commit
  `0610a87aa5a3e05ccedee4c0d34c4c6250625513` with
  `deployed_at=2026-06-08T04:13:21Z`.
- Choir has product concepts for foreground `super`, background/candidate
  `vsuper`, subordinate `cosuper`, `worker-medium`, and `worker-playwright`.
- The current intended source-transfer object is `AppChangePackage`; old
  patchset promotion is deprecated.
- Prior Choir-in-Choir work proved some package/adoption/rollback substrate,
  but the human-proof platform PR loop remained incomplete.
- `worker-playwright` exists conceptually as the heavy browser-proof worker
  class, but it must be reproven before relying on it.
- Candidate computers can mutate speculative state; canonical platform state
  changes only through external review and promotion.
- Platform-public changes still need the normal landing loop:
  `commit/PR -> CI -> deploy -> staging identity -> product proof`.

Highest-impact uncertainty:

```text
Can a visible product-path Choir run produce a reviewable platform candidate
artifact for Global Wire source truthfulness, with enough evidence that Codex
can review instead of rediscovering the work?
```

## Cognitive Transform Set

### 1. Authority Transform

Do not ask "can Choir develop Choir?" in the abstract. Ask what authority the
candidate system is allowed to hold today.

Changed route: Choir-in-Choir can author, investigate, package, test, and open
review artifacts. Codex/human still owns platform merge/deploy until a
promotion-level path is separately proven.

### 2. Artifact-First Transform

Do not optimize for a worker doing lots of activity. Optimize for the artifact
that survives handoff.

Changed route: the worker may read traces, inspect code, run tests, and reason
freely, but the overnight proof is a branch, PR, AppChangePackage, or precise
source-transfer blocker with evidence. A long Trace without a reviewable source
artifact is not enough.

### 3. Relief-Valve Transform

The point is not to replace Codex immediately. The point is to move token-heavy
search work out of Codex: source-cycle triage, trace reading, failure
classification, candidate tests, visual/browser QA, and first-pass fixes.

Changed route: optimize for Codex-reviewable evidence and diffs, not for
autonomous completion theater.

### 4. News-Truthfulness Transform

The visible defect is not "Global Wire needs polish." The defect is that the
product may be presenting placeholders and stale story ordering as if they were
live source-grounded news.

Changed route: start from product truthfulness questions. Are sources full
article bodies? Are top stories genuinely fresh, prominent, and novel? Do
source actions open real evidence? Only after those are observed should the run
choose between ingestion, ranking, processor/reconciler, or UI fixes.

### 5. PR As Boundary Object

A PR is a social and technical boundary object: source diff, tests, review
comments, CI, deploy eligibility, and rollback context all attach to it.

Changed route: for platform work, the candidate output should be a branch/PR
or an AppChangePackage that can become a PR. If no PR can be created, the
mission must record exactly which source-transfer capability is missing.

### 6. Human-Proof Transform

The VText narrative is not a trace dump. It is the owner-readable story of what
was attempted, what changed, what failed review, and what the next worker
should do.

Changed route: failed PR review must revise the mission VText with actionable
findings, not disappear into GitHub comments or raw logs.

## Real Artifact

The artifact is a working platform PR accelerator loop:

```text
Prompt bar mission
-> super starts self-development run
-> mission VText created or updated
-> worker-medium receives scoped platform task
-> worker inspects product behavior, source records, traces, code, and tests
-> worker either:
   A. publishes AppChangePackage with candidate source delta, or
   B. opens/pushes a GitHub branch/PR, or
   C. records a precise source-transfer blocker
-> verifier worker or Codex runs independent checks
-> worker-playwright provides browser proof if UI/product path changed
-> mission VText summarizes the whole state
-> Codex performs final PR review
-> if accepted, Codex merges/lands through CI/deploy/staging proof
-> if rejected, mission VText records review findings and next worker action
```

## Payload: Global Wire Source Truthfulness

The candidate should start from the latest deployed Global Wire behavior and
answer these in order.

### A. Source-Body Integrity

- Are source cards and article source transclusions backed by full ingested
  article bodies, or are seed placeholders/headline-level records still
  dominating the product path?
- Where does the live ingestion path store article body, source URL, language,
  fetch timestamp, and reader snapshot?
- Why do source readers show seed copy such as "This normalized SourceItem..."
  after the UI reports hundreds of live sources/items?
- What smallest source-body integrity fix or blocker would make the product
  more honest?

### B. Front-Page Ranking Truthfulness

- Why do the same top stories remain visible for hours while claiming recent
  update ages such as "18 min ago" or "41 min ago"?
- What ordering function chooses front-page stories, and does it account for
  prominence, importance, novelty, freshness, source volume, and source
  diversity rather than only seed/update timestamps?
- If a real importance ranking is out of scope, what honest interim ordering
  and status should the product show?

### C. Source Opening Behavior

- When a source is opened on mobile web, why can an "Open source/original"
  action trigger an iOS app/deep-link prompt that goes nowhere?
- Are source actions using web URLs, internal source-reader windows, or
  `choir://`/app-intent links?
- What change prevents dead mobile deep links while preserving source
  transclusion semantics?

### D. Delete Detritus Source Ledger Surfaces

- The Global Wire "Sources Chronology" list and search surface currently
  appears to be detritus: seed-looking repeated records, unclear user purpose,
  and a search box that does not provide an intelligible source exploration
  workflow.
- Treat this surface as a deletion candidate, not a polish candidate. If it is
  not backed by full article bodies and a clear reader/research workflow, remove
  it from the news app slice.
- Preserve source access through VText transclusions, source-reader windows,
  and source evidence attached to articles. Do not keep a noisy chronology list
  merely because the underlying source records exist.
- If a source exploration surface is later needed, it should be redesigned from
  the real job: browse/filter full ingested articles by time, language, topic,
  feed/channel, and source body availability. That is not the current surface.

### E. Delete Bespoke Style.vtext Controls From Global Wire

- The Global Wire Style.vtext panel with radio-button-like choices, "S",
  "Compose", "Replace", and "Ask" is not a coherent news app surface.
- Style.vtexts are ordinary VTexts and citeable source artifacts. They should
  be selected, composed, replaced, or customized through the VText agent/source
  workflow, not through a special Global Wire control panel.
- Styles should be example-rich VTexts written in their own style. Examples
  matter more than rule bullets. A style artifact should demonstrate the voice,
  structure, pacing, citation treatment, and editorial stance it wants future
  articles to learn from.
- Global Wire may show that an article cites or was influenced by a style
  source when relevant, but it should not expose bespoke style-selection UI in
  the news collection view.
- Treat the current Style.vtext Global Wire section as a deletion candidate.

### F. Processor/Reconciler Failure Context

- Which source cycle has recent processor/reconciler failures?
- Which run ids failed?
- What failure classes are present: provider/search/runtime/tool-contract,
  source-batch overload, VText edit failure, researcher handoff failure,
  timeout, cancellation, or bad source handle?
- Are failures retriable, resumable, or expected degradation?
- What owner-facing status should Global Wire show?

Allowed candidate fixes include:

- replacing placeholder source-reader content with real ingested article body
  when available;
- marking placeholder/seed sources honestly when live body is missing;
- changing front-page ordering to stop stale seed stories from dominating;
- fixing mobile source-open behavior to use web-safe routes;
- deleting the current Sources Chronology/search surface if it is not a real
  full-article source exploration workflow;
- deleting the current bespoke Style.vtext controls from Global Wire;
- improving owner-facing status for failed/degraded source cycles;
- adding tests around any of the above.

Disallowed overnight detours:

- building Autoradio;
- redesigning Global Wire UI broadly;
- preserving the current source chronology/search surface by default;
- preserving the current bespoke Style.vtext panel by default;
- introducing embeddings/clustering;
- hard-coding source trust tiers;
- trying to solve article prose quality before source-body truthfulness;
- mutating `main` or Node B directly from the Choir-in-Choir run.

## Pre-Mission Product Defects Observed

Owner-observed staging/mobile behavior on 2026-06-08 before mission start:

- Global Wire showed "211 live sources" and "543 source items", but front-page
  stories still looked like old seed stories and did not obviously reflect the
  most prominent or novel current events.
- The first stories claimed recent update ages while appearing unchanged for
  multiple hours.
- Source reader windows for items such as "Port authority throughput bulletin"
  and "Carrier service advisory" displayed seed placeholder copy rather than
  full ingested article bodies.
- Story VText source transclusions opened source cards, but the cards were
  effectively vacant as news evidence.
- iPhone Safari treated at least one "Open source/original" action as a request
  to open the native Choir app, then the action went nowhere. In web mode,
  source actions must either open an internal source reader or an ordinary
  browser URL, never a dead mobile deep link.
- The "Sources Chronology" list and search UI appeared to be detritus: repeated
  seed/source-neighborhood records, unclear user purpose, and no useful search
  workflow. The mission should allow deleting this surface rather than
  improving it incrementally.
- The Global Wire Style.vtext panel appeared incoherent: radio-button-like
  choices, "S", "Compose", "Replace", and "Ask" controls inside the news app.
  Style.vtexts should be ordinary VTexts/sources, and style artifacts should be
  example-rich articles in their own style rather than bullet-point rule cards.

Pre-mission fix state:

```text
Theme hydration and VText toolbar responsive fixes landed in commit 0610a87a.
If staging evidence shows those problems persist, document the regression and
decide whether it blocks the run. Otherwise, keep the overnight mission focused
on Choir-in-Choir plus Global Wire source truthfulness.
```

## Authority Boundaries

Hard rules:

- Choir-in-Choir must not push directly to `main`.
- Choir-in-Choir must not deploy Node B.
- Choir-in-Choir must not mutate canonical platform state outside a candidate
  branch/package/adoption route.
- Worker-local commits are not sufficient evidence unless exported as an
  AppChangePackage or pushed to a review branch.
- A proof worker must not verify only an unreachable local SHA.
- `worker-playwright` is leased only when browser/media proof is needed.
- Codex or the human owner performs final platform review before merge.
- Codex performs the final landing loop until promotion-level self-development
  is separately proven.

Allowed:

- worker-medium may inspect repo code, docs, Trace/API evidence, and tests;
- worker-medium may create candidate source edits in its candidate state;
- worker-medium may run focused tests and package the result;
- worker-playwright may run browser proof in authorized scratch/candidate
  state;
- verifier workers may run read-only or scratch verification;
- mission VText may be revised with narrative state and review findings;
- Codex may patch substrate bugs that block the self-development loop, but must
  document the problem first.

## Quality Bar

Target: **solid**.

Success is not "a worker did something." Success is a reviewable artifact that
reduces Codex rediscovery work.

The candidate output should include:

- the exact problem statement;
- source-cycle/run ids or a precise reason they could not be retrieved;
- root-cause hypothesis and evidence;
- code diff or package ref when a fix is attempted;
- focused tests;
- browser proof if UI changed;
- rollback notes;
- residual risks;
- mission VText revision written in plain owner-readable prose.

## Evidence Ledger

Required evidence for readiness:

- staging health/build identity before the run;
- visible prompt-bar or product-path initiation;
- exact prompt text used to start the run;
- super/vsuper/worker run ids;
- worker-medium lease/class evidence;
- candidate workspace or package/branch identity;
- VText mission narrative document id and current revision;
- Trace/run refs;
- test commands and results;
- if browser proof is needed, worker-playwright lease and screenshot/video refs;
- final source artifact: GitHub PR, branch, or AppChangePackage;
- Codex review result.

Minimum acceptable source-truthfulness evidence:

- at least one current staging screenshot or API observation of Global Wire
  front page;
- at least one source-reader observation proving whether a displayed source has
  real body text or placeholder seed text;
- code path identifying where displayed source cards/VText transclusions are
  populated;
- code path identifying the front-page story ordering function;
- code path identifying source-open URL/action generation;
- code path identifying the Sources Chronology/search surface, with a deletion
  recommendation if it is not backed by full article source exploration;
- code path identifying the bespoke Style.vtext Global Wire controls, with a
  deletion recommendation unless they are required for a real VText
  source-selection workflow;
- test or precise blocker for each attempted fix.

Required evidence for a landed platform fix:

- PR or commit SHA;
- CI run;
- deploy job;
- staging health/build identity;
- deployed acceptance proof;
- rollback ref;
- mission VText final update.

## Receding-Horizon Control

Operate in short intervals:

1. **Staging identity probe:** verify staging is on `0610a87a` or a later
   expected commit. If not, wait for deploy or record deploy identity blocker.
2. **Readiness probe:** verify product-path Choir-in-Choir can start the run,
   create/update mission VText, lease `worker-medium`, and return a structured
   result.
3. **Source-transfer probe:** verify the worker can produce a reviewable source
   artifact: AppChangePackage, branch, or PR. If this fails, root-cause before
   attempting the news fix.
4. **News truthfulness probe:** inspect Global Wire source-body integrity,
   front-page ranking, mobile source opening, source chronology/search deletion,
   Style.vtext panel deletion, and processor/reconciler failures in that order.
5. **Candidate fix probe:** if root cause is inside scope, worker produces a
   candidate fix plus tests. If multiple roots appear, prefer source-body
   integrity over ranking, and ranking over UI polish.
6. **Verifier probe:** independent verifier or Codex checks evidence and
   rejects/accepts.
7. **Review loop:** failed review revises mission VText and requeues worker;
   passed review may be landed by Codex.

If the self-development loop fails before source-transfer, do not silently fall
back to direct Codex implementation. Root-cause the loop failure if it is
inside mission authority. Direct Codex fixes may be used only for substrate
blockers after documenting the problem first.

## Anti-Goodhart Constraints

Do not accept:

- a pretty VText with no source artifact;
- a package with no evidence;
- a PR with no root-cause explanation;
- a build receipt instead of product proof;
- a worker-local commit that another worker cannot inspect;
- a screenshot disconnected from a package/PR;
- a Codex-written fix falsely labeled Choir-in-Choir;
- a bypass through internal/test-only routes;
- direct `main` mutation by worker;
- "failure fixed" when failures are merely hidden from UI;
- "source integrated" when only a title/headline/seed manifest is displayed;
- "fresh" when a timestamp is recomputed while story content is unchanged;
- "source chronology fixed" when the noisy surface remains without a clear
  full-article browsing job;
- "Style.vtext integrated" when Global Wire keeps bespoke style controls
  instead of treating styles as ordinary VText source artifacts;
- "Choir-in-Choir succeeded" when Codex had to rediscover the investigation.

## Stopping Conditions

### Complete

The mission is complete only if:

- Choir-in-Choir produces a reviewable platform artifact for the Global Wire
  source-truthfulness payload;
- Codex can review it without reconstructing the investigation from scratch;
- the mission VText records evidence and review state;
- either:
  - Codex lands the accepted fix through CI/deploy/staging proof, or
  - Codex rejects the artifact and the mission VText records actionable
    findings for the next worker revision.

An investigation-only artifact can satisfy completion if it is reviewable,
evidence-rich, and Codex rejects or accepts it without rediscovering the same
source/ranking/opening facts. A code fix is valuable but not mandatory if the
run proves that the correct next change requires a larger architecture move.

### Checkpoint Incomplete

Use `checkpoint_incomplete` if the loop advances but does not finish, for
example:

- worker-medium lease works but source-transfer fails;
- AppChangePackage exists but PR creation is missing;
- PR exists but verifier proof is incomplete;
- news truthfulness defect is documented but candidate fix is not ready.

### Blocked Incomplete

Use `blocked_incomplete` only after root-cause probes and cognitive transforms
if a blocker prevents progress, such as:

- staging auth/session prevents product-path run start;
- worker VM lease is broken;
- worker cannot access repo/source context;
- package/branch/PR source transfer is impossible with current product tools;
- worker-playwright class is unavailable or cannot run browser proof.

## Expected Overnight Route

The overnight run should prove the self-development loop while using Global
Wire truthfulness as the payload. It should not spend the night on broad news
architecture if it cannot first produce a reviewable artifact.

Likely route:

1. Confirm staging identity and current deployed SHA.
2. Start a product-path self-development prompt.
3. Have `super` create/update a mission VText.
4. Lease `worker-medium`.
5. Ask worker to inspect Global Wire in this order:
   - source cards and article transclusions for full body vs placeholder text;
   - front-page story ordering and stale timestamp behavior;
   - source-open behavior on mobile/web routes;
   - whether the Sources Chronology/search surface should be deleted;
   - whether the bespoke Style.vtext panel should be deleted;
   - latest processor/reconciler failures for context.
6. Require worker to produce either:
   - a small PR/package with tests for source-body, ranking, source-open, or
     status honesty, or
   - a precise blocker explaining why it cannot.
7. If a UI/browser change is involved, lease `worker-playwright` for proof.
8. Codex reviews the artifact.
9. If acceptable, Codex lands it through normal platform loop.
10. If rejected, Codex records review findings in mission VText and requeues
    the worker if time remains.

## Run Checkpoint & Resumption State

```text
status: blocked_incomplete
last checkpoint: 2026-06-08T04:58Z product-path run reached Super
  worker-medium lease twice, then failed before start_worker_delegation.
current artifact state:
  The staging-facing pre-mission UI fixes are on origin/main and deployed at
  commit 0610a87a. The Choir-in-Choir mission created owner-readable VText
  mission narratives and proved that Super can request worker-medium leases,
  but the delegated worker run never started. No AppChangePackage, branch, PR,
  candidate tests, worker proof, or Global Wire source-truthfulness fix exists.
what shipped:
  Pre-mission fixes in 0610a87a: local theme boot cache, VText toolbar
  right-aligned Revise action, no R/S/P narrow fallback, responsive toolbar
  regression tests. No mission payload fix has shipped.
what was proven:
  - Staging `/health` reports deployed commit
    0610a87aa5a3e05ccedee4c0d34c4c6250625513.
  - Product-path prompt-bar trajectory
    da92802e-00e3-4644-9138-321dc3fcf43d created mission VText
    a354b950-fea1-41a9-a399-ea9bfd0c18da and Super run
    b1a434e3-6f18-41fb-90d3-56240a743c71.
  - That Super run inspected repo context, corrected the false assumption that
    the repo was private `choir-ai/go-choir`, identified the public repo
    `https://github.com/choir-hip/go-choir.git`, read the mission doc, found
    relevant Global Wire code paths, and requested worker-medium
    worker-438463b747b3d37e / vm-5a3a26451d862c67cd2c228ae3373555.
  - A second product-path prompt-bar trajectory
    8243f21a-d37b-4b9f-a580-b71a55b001ca created concise mission VText
    17756e4e-8746-483f-84b7-0cd949fdb029 and reproduced the same failure
    after intentionally avoiding full mission-doc loading before delegation.
  - In both attempts, `request_worker_vm` returned
    `delegation_required=true`, `next_tool=start_worker_delegation`, and
    `start_args`, but the runtime/provider loop timed out twice before the
    forced `start_worker_delegation` tool call.
unproven or partial claims:
  worker-medium leasing is proven, but worker delegation start is not.
  worker-playwright availability, PR/package source transfer for platform work,
  Codex review loop integration, Global Wire source-body integrity, front-page
  ranking truthfulness, mobile source-open behavior, source chronology/search
  deletion, Style.vtext panel deletion, and processor/reconciler failure
  root-cause remain unproven.
belief-state changes:
  The first assumed root cause, "Super read too much mission context before the
  forced start tool," is probably incomplete or wrong. The repeated failure on
  the smaller route points to the required-next-tool/provider interaction. Trace
  events for the forced calls show `tool_choice=function:start_worker_delegation`
  and `max_tokens=0` before provider timeouts, making the required-tool retry
  path the highest-value substrate suspect.
remaining error field:
  The platform can currently be changed reliably by Codex through git/CI/deploy,
  but Choir-in-Choir cannot yet start the delegated worker after a successful
  worker lease through the product path. Global Wire may still be presenting
  seed placeholders and stale ordering as live news. The owner preference to
  delete the Sources Chronology/search surface is recorded separately in
  docs/choir-in-choir-deletion-bias-eval-note-2026-06-08.md.
highest-impact remaining uncertainty:
  Why does the required-next-tool path fail to produce
  `start_worker_delegation` after `request_worker_vm` returns start_args? Is
  the provider receiving an impossible zero-token request, mishandling forced
  tool choice, or blocked by another adapter/runtime constraint?
next executable probe:
  Document this substrate failure first, then inspect and patch the
  required-next-tool retry path so the forced `start_worker_delegation` call has
  a valid provider request or a deterministic non-LLM handoff. Add a regression
  test that fails if a required tool retry is sent with an impossible token
  budget or silently times out before the tool call. After deploy, rerun the
  same product-path mission and verify that a worker run actually starts.
suggested resume goal string:
  /goal Run docs/mission-choir-in-choir-platform-pr-accelerator-v0.md
evidence artifact refs:
  commit 0610a87a on origin/main for pre-mission theme/toolbar fixes; staging
  health confirmed deployed commit
  0610a87aa5a3e05ccedee4c0d34c4c6250625513 at 2026-06-08T04:13:21Z;
  failed trajectory da92802e-00e3-4644-9138-321dc3fcf43d;
  failed Super run b1a434e3-6f18-41fb-90d3-56240a743c71;
  retry trajectory 8243f21a-d37b-4b9f-a580-b71a55b001ca;
  retry VText doc 17756e4e-8746-483f-84b7-0cd949fdb029.
rollback refs:
  platform rollback is current deployed main before any accepted fix lands; no
  mission payload code changed during the failed product-path attempts.
```

```text
status: review_failed_requeue
last checkpoint: 2026-06-08T05:30Z product-path worker produced a deletion
  AppChangePackage, but Codex review rejected it before landing.
current artifact state:
  The required-next-tool substrate blocker was fixed in commit c9b02be2 and
  deployed to staging. The rerun product trajectory
  f2330486-99e4-4a7b-a283-a9eaf1625dbc successfully reached worker-medium,
  started worker/vsuper loop 1ee7a2a7-107f-46b9-b4a2-763911d21592, spawned an
  implementation co-super loop 592f9bce-b4ed-4190-bb5d-2b10a3ddcc11, and
  produced unlisted AppChangePackage 5365bcb7-a51c-4f49-9a06-d10c465c8a7b.
what the worker did well:
  The worker passed the deletion-bias eval at the intent level. It did not
  merely recommend deletion; it removed the Sources Chronology/search surface,
  removed the bespoke Style.vtext control panel, updated Global Wire tests to
  assert their absence, committed candidate SHA
  e9b8ff8e0cc216c9d6d478c72ab4671339e50470, and published an AppChangePackage.
review failure:
  Codex rejected the package before landing. The package patch appears to
  change the first line of `frontend/src/lib/GlobalWireApp.svelte` from
  `<script>` to `\<script\>`, which would break Svelte parsing if applied as
  source. The worker also claimed "build passed", but trace event 66 for the
  implementation co-super shows `go build ./...` timed out after two minutes.
  No frontend `npm run build` or Playwright acceptance proof was present in
  the package evidence at review time.
belief-state changes:
  Choir-in-Choir can now start a delegated worker after a lease, read the
  mission docs, make candidate code edits, commit them, and publish an
  AppChangePackage through the product path. The source-transfer/review loop is
  real but not yet trustworthy: package generation can carry malformed source,
  and the mission VText can overstate verification when a worker reports
  success after a timed-out command.
remaining error field:
  The candidate must be corrected or requeued with explicit review findings:
  preserve the deletion intent, repair malformed Svelte markup, run the real
  frontend build/test path, and provide browser or Playwright evidence for
  Global Wire after the deletion. Do not land package 5365bcb7 as-is.
next executable probe:
  Feed the Codex review failure back into the mission VText/Super loop or have
  Codex salvage the intended deletion in a normal branch. Any accepted fix must
  include frontend build proof and staging/browser proof after deployment.
evidence artifact refs:
  deployed substrate fix c9b02be2200764ad809935472c8073fa4694eb05; product
  trajectory f2330486-99e4-4a7b-a283-a9eaf1625dbc; mission VText
  861848eb-3310-46d1-b4c0-777a38e46686; worker run
  1ee7a2a7-107f-46b9-b4a2-763911d21592; implementation co-super run
  592f9bce-b4ed-4190-bb5d-2b10a3ddcc11; rejected package
  5365bcb7-a51c-4f49-9a06-d10c465c8a7b; rejected candidate commit e9b8ff8.
rollback refs:
  no rejected package has been adopted or deployed. Staging remains at
  c9b02be2.
```
