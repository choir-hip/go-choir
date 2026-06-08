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

```text
status: shipped_partial
last checkpoint: 2026-06-08T05:50Z Codex salvaged and landed the deletion
  payload after rejecting the malformed Choir-in-Choir package.
current artifact state:
  Commit aa5bef5bee595d13fe95a22cbf9a52089e3d75c7 is on origin/main and
  deployed to staging. It removes the Global Wire Sources Chronology/search
  surface and bespoke Style.vtext controls from the shipped app while
  preserving article columns, per-article VText open buttons, source entity
  transclusions, and related VText transclusions.
what shipped:
  - `frontend/src/lib/GlobalWireApp.svelte`: deleted the source chronology
    sidebar, source search/fetch/schedule controls, bespoke style selector,
    style compose/replace controls, Ask button, and all associated state,
    fetches, helper functions, and CSS.
  - `frontend/tests/global-wire-app.spec.js`: now asserts the detritus source
    and style surfaces are absent and that every article remains openable as a
    VText.
what was proven:
  - Local `npm run build` passed without Svelte warnings after cleanup.
  - Local focused Playwright passed:
    `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npm run e2e -- tests/global-wire-app.spec.js`.
  - CI run 27118442072 passed, including frontend build and staging deploy.
  - FlakeHub run 27118442058 passed.
  - Staging `/health` reports proxy and sandbox deployed commit
    aa5bef5bee595d13fe95a22cbf9a52089e3d75c7 with
    `deployed_at=2026-06-08T05:46:34Z`.
  - Deployed focused Playwright passed:
    `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- tests/global-wire-app.spec.js`.
unproven or partial claims:
  This is a shipped product cleanup and a partial Choir-in-Choir proof, not a
  complete news-system delivery. Source-body integrity, front-page ranking
  truthfulness, mobile source-open/deep-link behavior, processor/reconciler
  reliability, longtail Telegram/feed ingestion, and source article body
  availability remain open mission axes.
belief-state changes:
  The multiagent system did delete the detritus surfaces rather than merely
  recommending deletion, which is a useful positive eval. It still failed at
  source fidelity and verification honesty: the package contained malformed
  Svelte markup and overstated a timed-out build. Codex review remains required
  before landing Choir-in-Choir artifacts.
remaining error field:
  Continue the news mission from the real source pipeline: ingest many more
  RSS/GDELT/Telegram/HN/science/finance/industry sources with full article
  bodies, make article VTexts real prose owned by VText agents, and fix
  ranking/freshness/source-open truthfulness. Separately, improve
  AppChangePackage/source-transfer verification so packages cannot be reported
  as build-passed after a timeout or published with escaped source markup.
next executable probe:
  Use the next Choir-in-Choir run on a non-UI source-truthfulness payload, but
  require it to run the exact frontend/backend acceptance relevant to the
  files it changes and to record failures as review findings before package
  publication.
evidence artifact refs:
  documentation checkpoint commit 05f5271b; shipped fix commit
  aa5bef5bee595d13fe95a22cbf9a52089e3d75c7; CI run 27118442072; FlakeHub run
  27118442058; rejected package 5365bcb7-a51c-4f49-9a06-d10c465c8a7b; product
  trajectory f2330486-99e4-4a7b-a283-a9eaf1625dbc.
rollback refs:
  Revert aa5bef5b to restore the deleted Global Wire surfaces, though product
  direction says they should stay deleted. Rejected package 5365bcb7 was never
  adopted.
```

```text
status: shipped_partial
last checkpoint: 2026-06-08T05:56Z RunAcceptance synthesis recorded the
  product-path self-development evidence, with an honest blocked state.
run acceptance:
  acceptance_id runacc-58bb87e7b8990c65466d
  acceptance_level export-level
  state blocked
what the acceptance proves:
  The product path reached prompt submission, mission VText creation,
  persistent super request, worker VM lease, worker delegation, package
  publication, and run-memory compaction. It also recorded that the worker
  produced package 5365bcb7 from candidate commit e9b8ff8 after deleting the
  Sources Chronology/search surface and bespoke Style.vtext controls.
why the acceptance remains blocked:
  The exported package was reviewable but not acceptable. Codex review found
  malformed Svelte source (`<script>` escaped as `\<script\>`) and a false
  verification claim: the worker's `go build ./...` proof timed out after two
  minutes. The synthesized verifier also marked `checkpoint_causal_order`
  blocked and `export-level-product-path` blocked because the delegation ended
  non-cleanly (`context canceled`, `worker_observed`) rather than as a clean
  accepted package/adoption path.
evidence limitation:
  The synthesized record is derived from the Choir-in-Choir trace that began on
  deployed substrate c9b02be2, so its deployment_commit and health_commit fields
  are c9b02be2. Public staging health for the salvaged shipped fix is
  aa5bef5bee595d13fe95a22cbf9a52089e3d75c7. Treat the RunAcceptanceRecord as
  evidence of the self-development trajectory, not as proof that the rejected
  package landed.
next executable probe:
  A future acceptance synthesizer should be able to bind Codex review outcome,
  rejected-package state, and a subsequent human/Codex landing commit into the
  same mission evidence object without conflating "package exported" with
  "code accepted and deployed."
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T06:18Z live source-network VTexts present, but
  source entity labels are still too opaque for a reader-facing news surface.
current artifact state:
  Staging commit aa5bef5bee595d13fe95a22cbf9a52089e3d75c7 now returns
  `durable-storygraph+source-network-vtexts` from `/api/global-wire/stories`.
  The latest source cycle `cycle_13e2fd38f4867e98bb71b31f` completed with 211
  configured source fetches, 564 handoff items in the runtime status surface,
  7 completed processor runs, 1 completed reconciler run, and VText child runs.
  The front page now includes fresh live VText articles rather than only the
  three seeded StoryGraph records.
new evidence:
  The first live story on staging was
  "Pentagon Strips 180 Faiths from Military Recognition List, Atheism No Longer
  an Option", `source_state=source-network-vtext-index`, updated minutes ago,
  with real prose and native inline source refs. Its manifest lead entries,
  however, displayed generic titles such as `Source Service item
  srcitem_5d9a3d1f8d5e4c7046b8d2d4`.
  Direct Source Service resolution of that handle at
  `/internal/source-service/items/srcitem_5d9a3d1f8d5e4c7046b8d2d4` returned a
  real title, canonical URL, source id `rss:zerohedge`, published timestamp,
  content hash, and a body of roughly 4.9k characters.
belief-state changes:
  The earlier suspicion that Global Wire was still only indexing seed stories
  is no longer true on staging. The higher-value source-truth issue has moved
  one layer inward: source bodies exist in the local Source Service, but VText
  source entities created from `source_service_item:<id>` handles are sometimes
  labeled only by opaque handles when worker text does not include the article
  title. That makes the news collection and source transclusion surface look
  less real than the underlying source substrate is.
remaining error field:
  Reader-facing source manifests should use resolved source titles, URLs,
  source ids, timestamps, and body-backed source-service metadata when a
  source-service item handle is available. They must still preserve native
  source entities and not flatten source bodies into article prose or add a
  separate source-ledger surface.
next executable probe:
  Patch the VText source-entity derivation path so source-service item handles
  are enriched from the local Source Service API when available, with a fast
  bounded fallback to the existing handle label. Add a regression test proving
  a researcher handoff containing only `source_service_item:<id>` yields a
  VText source entity and Global Wire manifest with the resolved article title.
```

```text
status: shipped_partial
last checkpoint: 2026-06-08T06:38Z source-service item handles now resolve
  into reader-facing Global Wire source metadata.
current artifact state:
  Commit 2c6690633091aa30af234aa24b1cdaa0ec652eef is on origin/main and
  deployed to staging. Global Wire still reads ordinary VText article revisions
  and native source entities; the fix does not reintroduce a source ledger UI
  and does not flatten source bodies into article prose.
what shipped:
  - `internal/runtime/tools_research.go`: Source Service HTTP client can resolve
    `/internal/source-service/items/{id}` in addition to search/latest status.
  - `internal/runtime/vtext_media_sources.go`: source-service entities derived
    from worker/researcher handles are enriched from local Source Service item
    records when available, with a short bounded timeout and existing fallback
    labels when unavailable.
  - `internal/runtime/global_wire.go`: Global Wire manifest projection enriches
    only the cited source entities that will be shown in the story source
    neighborhood, so already-created VText articles can render real source
    titles/URLs without mutating stored VText metadata.
  - `internal/types/global_wire.go`: source manifest items now carry optional
    `source_id` and `fetch_id` fields alongside `canonical_url`.
what was proven:
  - Focused local Go test passed:
    `nix develop -c go test ./internal/runtime -run 'TestVTextPromptDerivesSourceServiceEntitiesFromResearcherUpdates|TestVTextSourceServiceEntitiesResolveItemTitles|TestHandleGlobalWireStoriesUsesVisibleSourceEntitiesForSourceNetworkManifest|TestHandleGlobalWireStoriesIndexesSourceNetworkVTextHeads'`.
  - CI run 27120207895 passed for commit
    2c6690633091aa30af234aa24b1cdaa0ec652eef, including runtime shards, build,
    and staging deploy.
  - FlakeHub run 27120207902 passed.
  - Public staging `/health` and Node B sandbox `/health` both reported
    deployed commit 2c6690633091aa30af234aa24b1cdaa0ec652eef with
    `deployed_at=2026-06-08T06:34:50Z`.
  - Deployed staging API proof against Node B
    `/api/global-wire/stories` returned
    `durable-storygraph+source-network-vtexts`; the first live story's manifest
    lead sources included resolved titles such as "Telegram Post from
    Slavyangrad Telegram" and Euronews article titles, plus `source_id`,
    `fetch_id`, and `canonical_url`.
unproven or partial claims:
  The source labels are now real item metadata, but the quality of some source
  bodies remains mixed: RSS bodies may include HTML; GDELT bodies may be GKG
  metadata rather than article text; Telegram items are short social posts.
  Article ranking, source-body cleaning/readability extraction, mobile source
  open behavior, and deeper Choir-in-Choir acceptance remain open.
belief-state changes:
  The source pipeline is materially more real than the earlier screenshots
  implied: current cycles have hundreds of items and live VText articles.
  The next realism gap is not "sources do not exist"; it is source-body quality,
  normalization, and how article agents choose and cite the best source mix.
remaining error field:
  Continue toward publication-quality Global Wire by improving source body
  normalization/readability, ranking/prominence/novelty, and source-open
  behavior. Preserve the deletion of detritus ledger/style controls.
rollback refs:
  Revert 2c669063 to remove source-service item enrichment and return to
  handle-only source labels. Revert aa5bef5b only if the deleted detritus
  surfaces must be restored, which remains contrary to product direction.
```

```text
status: shipped_partial
last checkpoint: 2026-06-08T06:50Z RSS source item bodies are normalized at
  ingestion for future source cycles.
current artifact state:
  Commit 1fd56a2924eeb5dd6c49a1c7cbe0101574f7a776 is on origin/main and
  deployed to staging. Source Service RSS polling now stores cleaned text for
  item bodies when feed descriptions contain HTML fragments or entities.
what shipped:
  - `internal/sources/rss.go`: RSS item body creation now extracts text from
    HTML description fragments with an HTML tokenizer, unescapes entities, and
    collapses whitespace/punctuation spacing before computing the item content
    hash.
  - `internal/sources/rss_test.go`: regression coverage proves an RSS
    description such as `<p>Markets &amp; policy <strong>shifted</strong>.</p>`
    becomes `Markets & policy shifted.` rather than raw HTML.
what was proven:
  - Local tests passed:
    `nix develop -c go test ./internal/sources`
    and `nix develop -c go test ./cmd/sourcecycled`.
  - CI run 27120708225 passed for commit
    1fd56a2924eeb5dd6c49a1c7cbe0101574f7a776, including runtime shards,
    non-runtime tests, build, and staging deploy.
  - FlakeHub run 27120708229 passed.
  - Public staging `/health` and Node B sandbox `/health` both reported
    deployed commit 1fd56a2924eeb5dd6c49a1c7cbe0101574f7a776 with
    `deployed_at=2026-06-08T06:47:27Z`.
unproven or partial claims:
  This is an ingestion-time normalization fix. It does not rewrite old source
  item rows that already contain raw RSS HTML, and it does not fetch full
  article bodies beyond the feed description. GDELT source items may still be
  metadata summaries rather than article text. A deeper readability/fetch-body
  mission remains open.
belief-state changes:
  RSS body quality is now improved for new items without adding latency to
  source reads. The next source-body axis is full article extraction and
  source-type-specific representation quality, not simply stripping feed HTML.
remaining error field:
  Existing stored items may remain noisy until refreshed or reprocessed. Source
  Service still needs a principled representation policy for RSS summary,
  article body fetch, GDELT metadata, Telegram posts, and publication-safe
  excerpts.
rollback refs:
  Revert 1fd56a29 to restore raw RSS descriptions as source item bodies.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T07:05Z source reader links can expose app-only
  source URLs as browser links on mobile.
current artifact state:
  Commit 1fd56a2924eeb5dd6c49a1c7cbe0101574f7a776 is deployed to staging.
  Global Wire source transclusions open an internal Source Viewer window, but
  that Source Viewer renders `sourceUrl` as a normal external anchor whenever a
  source URL is present.
new evidence:
  User iPhone screenshots show tapping "Open original" in a source window
  triggers Safari's "Open in Choir?" deep-link prompt and then goes nowhere in
  the web app context. Code inspection shows `frontend/src/lib/ContentViewer.svelte`
  unconditionally renders `<a href={sourceUrl}>` for any source URL. Seed
  Global Wire source items still use app-only canonical URLs of the form
  `choir://global-wire/source/<id>` in `internal/store/global_wire.go`, so the
  source reader can expose a non-web URL to Safari.
belief-state changes:
  Opening a source inside Choir and opening the original web URL are separate
  operations. The internal source-reader path should keep working for
  `choir://` and source-service handles, but the browser-facing "Open original"
  control should exist only for web-safe URLs such as `http:` and `https:`.
remaining error field:
  Patch the Source Viewer so app-only or unsupported source URLs are not
  rendered as clickable browser anchors, while preserving clickable original
  links for real web URLs and preserving the Source Viewer reader/provenance
  apparatus.
next executable probe:
  Add regression coverage proving a `choir://` source URL does not render an
  `.source-link`, while an `https://` source URL still renders the external
  original link. Then run the focused source-entity frontend test and deployed
  proof after landing.
```

```text
status: shipped_partial
last checkpoint: 2026-06-08T07:11Z app-only source URLs no longer render as
  browser-openable original links in the Source Viewer.
current artifact state:
  Commit 436ef202a385273bec5924d179b8e452c215562b is on origin/main and
  deployed to staging. The Source Viewer still opens from native VText source
  transclusions, but it only renders the "Open original" / "Open source"
  browser anchor for `http:` and `https:` URLs.
what shipped:
  - `frontend/src/lib/source-url.ts`: added a small browser URL classifier that
    accepts only `http:` and `https:` source URLs for external browser opening.
  - `frontend/src/lib/ContentViewer.svelte`: uses the classifier before
    rendering `.source-link`, so app-only URLs such as
    `choir://global-wire/source/<id>` are not exposed to Safari as deep links.
  - `frontend/tests/vtext-source-entities.spec.js`: regression coverage proves
    `https://` and `http://` remain browser-openable while `choir://`,
    `source_service_item:*`, and relative internal paths are not.
what was proven:
  - Local `npm run build` passed.
  - Local focused Playwright pure regression passed:
    `npm run e2e -- tests/vtext-source-entities.spec.js -g "source reader exposes only web-safe original links"`.
  - The broader local browser-backed source-entity file could not run without
    the local service stack; the failure was `ERR_CONNECTION_REFUSED` at
    `http://localhost:4173/`, after the pure source-url regressions had passed.
  - CI run 27121344802 passed for commit
    436ef202a385273bec5924d179b8e452c215562b, including frontend build, Go
    gates, and staging deploy.
  - FlakeHub run 27121344783 passed.
  - Public staging `/health` reports proxy and sandbox deployed commit
    436ef202a385273bec5924d179b8e452c215562b with
    `deployed_at=2026-06-08T07:04:12Z`.
  - A live staging Playwright probe created a VText with a
    `choir://global-wire/source/source-port-authority` source entity, opened it
    through the normal VText source button, verified Source Viewer reader mode,
    and verified `.source-link` count was zero.
  - Deployed focused Playwright passed:
    `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- tests/vtext-source-entities.spec.js -g "VText source URL opens Source Viewer unless browser is explicitly requested"`,
    proving normal `https://` source URLs still open Source Viewer by default
    and explicit Web Lens routing still opens the browser app.
unproven or partial claims:
  This fixes the web app deep-link prompt class for Source Viewer browser
  anchors. It does not make old seed source bodies real full article sources,
  and it does not solve front-page ranking, processor/reconciler backlog, or
  article body extraction quality.
belief-state changes:
  Source opening now has a cleaner boundary: VText source transclusion opens
  the internal Source Viewer; only web URLs become browser/open-original
  anchors. App-only source identifiers remain internal product references.
remaining error field:
  Continue the Global Wire source-truth mission on ranking freshness,
  processor/reconciler queue pressure, source body extraction quality, and
  eventual removal of remaining seed/demo source records from reader-facing
  news flows.
rollback refs:
  Revert 436ef202 to restore prior behavior where any source URL, including
  `choir://`, rendered as a browser anchor.
```
