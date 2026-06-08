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

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T07:18Z source intake is no longer paltry, but
  processor dispatch is capped below the current source volume.
current artifact state:
  Commit cebb21c1ea63bdc23b855c6bee3370f1c64689ca is on origin/main as the
  latest docs checkpoint.
  Staging behavior code remains deployed at
  436ef202a385273bec5924d179b8e452c215562b. Source Service is running on
  Node B with the compiled processor dispatch default and no service
  environment override for `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS` or
  `SOURCECYCLED_AGENT_DISPATCH_MAX_PROCESSORS`.
new evidence:
  Source Service latest cycle `cycle_1d67b589d1aa22af1b539729` ran from
  2026-06-08T07:02:31Z to 2026-06-08T07:02:38Z. It fetched from 211 configured
  sources, had 198 successful fetches and 13 failed fetches, produced 611 new
  items for the cycle, and reported 3,873 stored source items. The runtime
  source-status API for a staging authenticated user reported 28 processor
  requests and 1 reconciler request for that cycle; 7 processor requests were
  submitted and 21 remained queued. Source Service's own latest summary also
  reported `processor_status_counts={queued:21, submitted:7}`.
code-path finding:
  `cmd/sourcecycled/main.go` sets
  `defaultSourceMaxxProcessorDispatchLimit = 7`. The dispatcher reads
  `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS` /
  `SOURCECYCLED_AGENT_DISPATCH_MAX_PROCESSORS`, then submits only the first
  `maxProcessorRequests` processor handoffs in the current cycle and increments
  `ProcessorSkipped` for the rest. Skipped requests remain queued; there is no
  observed follow-up drain loop in this pass.
belief-state changes:
  The earlier "16 sources is paltry" problem has materially moved: the system
  is now ingesting hundreds of items per cycle from RSS, GDELT, Telegram, and
  other configured source classes. The more immediate architecture question is
  how live processors should absorb source bursts without losing coverage,
  overloading LLM agents, or pretending that 7 processed chunks represent the
  whole source cycle.
remaining error field:
  Do not solve this by blindly raising the cap in tracked config. The next
  design/fix should decide whether queued processor requests need a drain
  worker, adaptive concurrency, per-processor continuity scheduling, backpressure
  metrics, or a different handoff topology. Acceptance evidence should show
  that queued processor requests either drain later or are explicitly
  superseded/merged with provenance, not silently stranded.
next executable probe:
  Inspect Source Service storage for queued processor requests across multiple
  cycles and recent runtime run durations. Then choose the scheduling topology
  that preserves processor continuity, drains or supersedes queued work
  explicitly, and avoids dispatching redundant stale chunks after newer cycles
  make them obsolete.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T07:36Z deployed queue drain is real but too weak
  for current source volume, and latest-cycle evidence hides global backlog.
current artifact state:
  Commit c7fa65114a3e238b6f1bfec7c5088b295632f837 is on origin/main and
  deployed to staging. It adds durable queued-processor draining and prevents a
  reconciler request from dispatching until its processor request handles have
  `submitted` status and non-empty runtime run IDs.
new evidence:
  GitHub Actions run 27122347993 passed and deployed commit
  c7fa65114a3e238b6f1bfec7c5088b295632f837. Staging `/health` reported that
  commit for proxy and sandbox at 2026-06-08T07:26:39Z.

  Node B Source Service first post-deploy cycle
  `cycle_6d17c95662e648f06b65dde0` ran from 2026-06-08T07:26:43Z to
  2026-06-08T07:26:50Z. It fetched 211 configured sources, produced 4,975
  source items, queued 124 processor requests, and queued 1 reconciler request.
  The latest-cycle endpoint showed all 124 processors still queued and the
  reconciler queued.

  Service logs explain part of the transition: the first dispatch attempt after
  sourcecycled restart saw `Post "http://127.0.0.1:8085/internal/runtime/runs":
  dial tcp 127.0.0.1:8085: connect: connection refused`. Runtime health later
  reported ready on commit c7fa6511, so this was startup ordering / readiness
  timing, not a permanently missing runtime.

  The sourcecycled process environment on Node B still pins
  `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS=7`, so the code default of 32
  cannot take effect on staging. A read-only copy of
  `/var/lib/go-choir/source-service/sourcecycled.db` showed global request
  state after deploy:

  - processor requests: 7 `dispatch_failed`, 532 `submitted`, 1,902 `queued`
  - reconciler requests: 1 `dispatch_failed`, 75 `submitted`, 6 `queued`
  - the post-deploy dispatch submitted 7 older queued processor requests from
    `cycle_8a3fd397a071c7d2b1f27b05`, not the latest cycle.
code-path finding:
  The queue drain patch does select queued processors globally by oldest
  created-at order, then selects queued reconcilers and submits a reconciler
  only if its processor request IDs are already submitted. That explains why
  the latest cycle still looked untouched: the durable queue had a large older
  backlog and staging was capped at 7 submissions per cycle.
belief-state changes:
  Reconciler gating is now closer to the desired topology: it should not
  summarize a cycle before processor work is at least dispatched. The remaining
  failure is source-pressure scheduling. Current staging intake can create
  hundreds of processor chunks in minutes, while the deployed dispatcher only
  drains seven per cycle and has no readiness-independent drain job. The
  latest-cycle UI/API is also an insufficient operations view because it cannot
  distinguish "this cycle is stranded" from "the daemon is draining older
  queued work first."
remaining error field:
  Fixing this requires more than raising one constant. The next code change
  should remove or update the tracked deployed cap, make dispatch resilient to
  runtime readiness/startup ordering, and expose enough queue/backlog evidence
  that staging proof can show whether processors drain, fail, or are
  intentionally superseded. Do not claim processor scheduling solved while
  1,902 queued requests remain invisible behind a latest-cycle-only summary.
next executable probe:
  Find the tracked source of `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS=7`
  and decide whether to remove it, raise it, or make it adaptive. Then add a
  readiness/backlog test that proves a large durable queue can drain after the
  runtime becomes available without prematurely submitting reconcilers.
```

```text
status: shipped_partial
last checkpoint: 2026-06-08T07:49Z Source Service queue drain now runs between
  ingestion cycles and staging no longer pins processor dispatch at 7.
current artifact state:
  Commit e1b177d6796e52474810488e42ede347c247211f is on origin/main and
  deployed to staging. It follows documentation checkpoint
  d79fda97d49b8f9d8ef633c8bb39ac28e899952ce, which recorded the backlog
  problem before the fix.
what shipped:
  - `cmd/sourcecycled/main.go` now runs a queued SourceMaxx dispatch drain every
    minute between the 15-minute source ingestion cycles.
  - Transient runtime submission failures leave processor/reconciler requests
    queued for a later drain instead of marking them permanently
    `dispatch_failed`.
  - `nix/node-b.nix` now deploys
    `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS=32` and
    `SOURCE_SERVICE_AGENT_DISPATCH_DRAIN_INTERVAL_SECONDS=60`.
  - `cmd/sourcecycled/main_test.go` adds regression coverage proving transient
    runtime unavailability keeps a processor request queued with no runtime run
    ID.
what was proven:
  - Local `git diff --check` passed.
  - Local `nix develop -c go test ./cmd/sourcecycled` passed.
  - Local `nix develop -c go test ./internal/cycle` passed.
  - CI run 27123001146 passed for commit
    e1b177d6796e52474810488e42ede347c247211f, including Go gates and staging
    deploy.
  - FlakeHub run 27123001150 passed for the same commit.
  - Public staging `/health` reported proxy and sandbox deployed commit
    e1b177d6796e52474810488e42ede347c247211f with
    `deployed_at=2026-06-08T07:41:14Z`.
  - Node B process env for sourcecycled reported
    `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS=32`,
    `SOURCE_SERVICE_AGENT_DISPATCH_DRAIN_INTERVAL_SECONDS=60`, and
    `SOURCE_SERVICE_RUNTIME_BASE_URL=http://127.0.0.1:8085`.
  - Source Service health reported `status=ok`, `item_count=69108`,
    `fetch_count=12969` at 2026-06-08T07:44:58Z.
  - Post-deploy sourcecycled logs show repeated drain ticks:
    at 07:45:22Z, 32 processors submitted, 0 failed, 6 reconcilers skipped;
    at 07:46:23Z, 32 processors submitted, 1 reconciler submitted, 5 skipped;
    at 07:47:22Z, 32 processors submitted, 0 failed, 5 skipped.
  - A read-only copy of the deployed DB after those drains showed processor
    status counts of 7 `dispatch_failed`, 667 `submitted`, and 2,045 `queued`;
    reconciler status counts of 1 `dispatch_failed`, 79 `submitted`, and
    5 `queued`.
unproven or partial claims:
  This proves that queue drain now repeats and can submit processors/reconcilers
  after deployment. It does not prove the backlog is solved. The queue remains
  large because prior cycles created thousands of processor requests, and the
  system still needs an agentic scheduling decision about whether older queued
  chunks should be compacted, superseded, merged by continuity ref, or drained
  exactly as historical work.
belief-state changes:
  The source system is now visibly ingesting thousands of items and dispatching
  durable processor/reconciler work, but the processor topology remains too
  literal: every historical source chunk is treated as work to submit unless a
  future scheduler says otherwise. For the news product, the higher-value next
  move is not only more concurrency; it is continuity-aware processor state,
  compaction/supersession rules, and real article/VText ownership.
remaining error field:
  Do not call Global Wire source processing ship-worthy while 2,045 queued
  processor requests remain and latest-cycle APIs still obscure global backlog.
  The next realism axis should expose queue/backlog health productively and add
  supersession/compaction semantics so processors do not spend all capacity on
  stale chunks when newer source context has already arrived.
rollback refs:
  Revert e1b177d to restore the previous sourcecycled behavior: dispatch only
  during source cycles, tracked staging cap of 7, and transient runtime failures
  marked as dispatch failures.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T07:53Z queue drain works, but the backlog is
  repeated continuity work rather than a clean FIFO truth object.
current artifact state:
  Commit e8383a98ad8fb47233e315d97c8ac04f16819e756 is on origin/main as the
  latest mission checkpoint. Staging behavior code remains
  e1b177d6796e52474810488e42ede347c247211f.
new evidence:
  Sourcecycled logs after the previous landing continued to show one-minute
  drains: 32 processor submissions at 07:48:23Z, 32 at 07:49:22Z with one
  reconciler submitted, and 32 at 07:50:22Z. A current read-only Node B DB copy
  still showed processor counts of 7 `dispatch_failed`, 763 `submitted`, and
  1,949 `queued`; reconciler counts were 1 `dispatch_failed`, 80 `submitted`,
  and 4 `queued`.

  The queued processors are heavily concentrated by continuity:

  - `processor:global_firehose:global:gdelt`: 543 queued
  - `processor:technology:global:rss`: 139 queued
  - `processor:conflict:europe:telegram`: 97 queued
  - `processor:finance:global:rss`: 72 queued
  - `processor:conflict:europe:rss`: 68 queued
  - `processor:technology:europe:rss`: 67 queued

  The latest-cycle endpoint still reports only latest-cycle status
  (`cycle_c49482d8c6899c36eb7d4b55` has 125 queued processors and 1 queued
  reconciler), while global storage shows the active drain is processing older
  cycles first. That endpoint is not a sufficient operations truth surface.
belief-state changes:
  The queue is not merely "large." It is preserving stale snapshots for the
  same long-running processor continuity refs. That is misaligned with the
  intended processor topology: processors should update a durable
  representation of their world slice and compact as context moves forward,
  not process every old burst in exact FIFO order after newer source cycles
  have arrived.
remaining error field:
  Add explicit supersession semantics before more dispatch tuning. New queued
  processor work for a continuity ref should mark older queued processor work
  for that same continuity as `superseded`, preserving the historical record
  but preventing stale chunks from consuming drain capacity. Reconciler
  requests that depend on superseded processor request IDs should also become
  superseded rather than sitting queued forever or summarizing incomplete old
  cycles.
next executable probe:
  Implement and test storage-level supersession before dispatch. Then deploy
  and prove on staging that queued counts drop by superseding older same-
  continuity work, latest cycles can still dispatch current processors, and
  reconcilers only submit when their active processor request IDs have real
  runtime run IDs.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T08:04Z processor supersession landed and proved on
  staging.
current artifact state:
  Commit 37c914e2448d747fa0bf4f91387c07b5df2e70c7 is on origin/main and
  deployed to staging. It follows documentation checkpoint
  b9a92997988420cd9d0280177330287b56dc0bdb, which recorded the repeated
  continuity backlog problem before the fix.
what shipped:
  - Queued processor requests now supersede older queued requests with the same
    continuity ref before dispatch.
  - Queued reconcilers whose dependent processor requests were superseded are
    also marked `superseded`.
  - Submitted processor requests are preserved; supersession only removes stale
    queued snapshots from active dispatch pressure.
  - Source handoff events now record `processor_continuity_refresh`,
    `superseded_processor_count`, and `superseded_reconciler_count`.
what was proven:
  - Local `gofmt`, `git diff --check`, `nix develop -c go test ./internal/cycle`,
    and `nix develop -c go test ./cmd/sourcecycled` passed before push.
  - CI run 27123778280 passed for commit
    37c914e2448d747fa0bf4f91387c07b5df2e70c7, including Go gates and staging
    deploy.
  - FlakeHub run 27123778183 passed for the same commit.
  - Public staging `/health` reported proxy and sandbox deployed commit
    37c914e2448d747fa0bf4f91387c07b5df2e70c7 with
    `deployed_at=2026-06-08T07:58:16Z`.
  - Node B sourcecycled env reported
    `SOURCE_SERVICE_AGENT_DISPATCH_MAX_PROCESSORS=32`,
    `SOURCE_SERVICE_AGENT_DISPATCH_DRAIN_INTERVAL_SECONDS=60`, and
    `SOURCE_SERVICE_RUNTIME_BASE_URL=http://127.0.0.1:8085`.
  - Source Service health reported `status=ok`, `item_count=69664`, and
    `fetch_count=13180` at 2026-06-08T08:00:31Z.
  - The first post-deploy source cycle fetched 5,026 deduped new items and
    queued 125 processor requests plus 1 reconciler request.
  - The handoff event for that cycle recorded
    `superseded_processor_count=1711` and
    `superseded_reconciler_count=4`.
  - A read-only deployed DB copy after two drain ticks showed processor status
    counts of 7 `dispatch_failed`, 32 `queued`, 1,094 `submitted`, and
    1,711 `superseded`; reconciler status counts of 1 `dispatch_failed`,
    1 `queued`, 80 `submitted`, and 4 `superseded`.
  - Drain logs after deploy showed `processor_skipped` collapse from the old
    backlog range of roughly 1,700 queued requests to 64 and then 32 active
    queued requests, while still submitting 32 processors per minute.
belief-state changes:
  Processor dispatch is no longer a stale FIFO backlog. It now behaves closer
  to long-running processor continuity: newest queued context for a processor
  continuity ref is active, older unsubmitted snapshots become historical
  evidence, and submitted historical work remains intact.
retrieval note:
  The user wants the old Global Wire Sources Chronology/search/source-ledger
  surface deleted, not cautiously retained behind "maybe" language. The same is
  true of bespoke Style.vtext controls such as radio buttons, `S`, Compose,
  Replace, and Ask. Styles are VTexts/sources and should be visible through
  article-quality examples and transclusion, not a separate control panel. Do
  not restore those detritus surfaces while claiming source exploration or style
  handling has improved.
remaining error field:
  This does not make Global Wire ship-worthy. The source system is ingesting
  large volumes, but product quality still needs real full-article source
  bodies where available, front-page ranking by prominence/importance/novelty,
  VText-agent-owned publication-quality articles with native source
  transclusion, and a cleaner product status surface that explains processor
  and reconciler work without exposing detritus.
rollback refs:
  Revert 37c914e2 to remove supersession behavior while keeping the prior
  queue-drain behavior from e1b177d6. Revert e1b177d6 as well to return to the
  original dispatch-only-during-cycle behavior.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T08:12Z source-body integrity problem documented
  before the next behavior change.
current artifact state:
  Staging still serves behavior commit 37c914e2448d747fa0bf4f91387c07b5df2e70c7.
  The latest docs checkpoint on origin/main is
  ec8e0cd4458cff3aa13f2a97feb56ec23d60becf.
new evidence:
  A current read-only copy of Node B
  `/var/lib/go-choir/source-service/sourcecycled.db` showed 69,664 Source
  Service items. The `items` schema has one `body` column but no separate
  reader snapshot, extraction state, body kind, body length, or source-body
  availability field.

  Current body-length distribution by source type:

  - `gdelt`: 61,578 items, average body length 798.1, 3,735 bodies at least
    2,000 chars, 15,406 bodies under 280 chars, 0 empty.
  - `rss`: 5,979 items, average body length 743.2, 203 bodies at least
    2,000 chars, 2,829 bodies under 280 chars, 343 empty.
  - `telegram`: 2,107 items, average body length 356.8, 31 bodies at least
    2,000 chars, 1,329 bodies under 280 chars, 0 empty.

  Code inspection matched the DB evidence:

  - `internal/sources/rss.go` sets `Item.Body` from RSS description or Atom
    summary/content. It does not fetch the article URL and does not create a
    readability/full-article snapshot.
  - `internal/sources/gdelt.go` sets `Item.Body` to GDELT metadata strings
    such as themes, organizations, and locations. That is useful signal, but
    it is not the source article body at `DOCUMENTIDENTIFIER`.
  - `internal/sources/telegram.go` sets `Item.Body` to Telegram post text,
    which can be the whole source for a post but is not equivalent to an
    article body.
  - `cmd/sourcecycled/main.go` maps Source Service items to API results with
    only `body` and `evidence_level`; every live poller currently uses the
    generic `source_feed` evidence level.
belief-state changes:
  The product is now ingesting many sources, but the source-body object is too
  coarse. Downstream VText/source-reader code cannot tell whether a SourceItem
  is a full article snapshot, a feed summary, a GDELT metadata packet, a
  Telegram post, or an empty body. That explains why source readers can look
  vacant or overclaim evidence even after source volume increased.
remaining error field:
  The next behavior change should make source-body availability explicit at
  the Source Service boundary. A small honest fix is to classify each SourceItem
  body and expose fields such as body kind, body length, and whether the body is
  a reader/full snapshot. This will not fetch full article bodies yet, but it
  prevents the system from Goodharting "many sources" while hiding that most RSS
  records are feed summaries and GDELT records are metadata packets.
next executable probe:
  Add source-body classification to `sources.Item`, persisted storage, Source
  Service API results, and source-item resolution metadata. Classify RSS as
  feed summary/content, GDELT as metadata packet, and Telegram as post text;
  include tests and staging proof that API-resolved source items expose the
  classification.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T08:33Z source-body availability classification
  landed and was proved on staging.
current artifact state:
  Commit 55ddce3f1c61fdac8b6d50840f7e24a03f8bb501 is on origin/main and
  deployed to staging. It follows documentation checkpoint
  042c35f1a54d118d0c31e4024ead188985bb0926, which recorded the source-body
  integrity problem before the behavior change.
what shipped:
  - `sources.Item` now carries `body_kind`, `body_length`, and
    `reader_snapshot` classification.
  - RSS items are classified as `feed_summary` when they contain feed body text
    and `empty` when body text is absent.
  - GDELT items are classified as `metadata_packet`.
  - Telegram items are classified as `social_post`.
  - Source Service storage persists the new fields and derives them on read for
    legacy rows whose classification columns are empty.
  - Source Service search/resolve API results expose `body_kind`,
    `body_length`, and explicit `reader_snapshot: false` when no reader/full
    snapshot exists.
  - Researcher source-search projections and Global Wire Source Service
    content-item metadata carry the same classification.
  - VText source entities enriched from Source Service items now include body
    classification and an uncertainty note when the body is only a feed
    summary, metadata packet, social post, or empty.
what was proven:
  - Local `git diff --check` passed.
  - Local `nix develop -c go test ./internal/sourceapi ./internal/sourcecontract ./internal/sources ./internal/cycle ./cmd/sourcecycled`
    passed.
  - Local `nix develop -c go test ./internal/runtime -run 'TestResearcherSourceSearchCallsSourceServiceAPI|TestVText|TestGlobalWire'`
    passed.
  - Follow-up local `nix develop -c go test ./cmd/sourcecycled ./internal/sourceapi`
    and `nix develop -c go test ./internal/runtime -run 'TestResearcherSourceSearchCallsSourceServiceAPI'`
    passed after making false/zero classification fields explicit in JSON.
  - CI run 27125359353 passed for commit
    55ddce3f1c61fdac8b6d50840f7e24a03f8bb501, including Go gates and staging
    deploy.
  - FlakeHub run 27125359369 passed for the same commit.
  - Public staging `/health` reported proxy and sandbox deployed commit
    55ddce3f1c61fdac8b6d50840f7e24a03f8bb501 with
    `deployed_at=2026-06-08T08:30:39Z`.
  - Node B Source Service search for an existing RSS item
    `srcitem_02e62eff64001f85b3a33963` returned
    `body_kind=feed_summary`, `body_length=164`, and
    `reader_snapshot=false`.
  - Node B Source Service search for an existing RSS item with no body
    `srcitem_9b7830af5a2c40378af4bbb7` returned `body_kind=empty`,
    `body_length=0`, and `reader_snapshot=false`.
  - Node B Source Service search for a GDELT result returned
    `body_kind=metadata_packet`, `body_length=418`, and
    `reader_snapshot=false`.
belief-state changes:
  The product can now honestly distinguish source volume from source-body
  richness at the Source Service boundary. This does not ingest full article
  bodies yet, but it prevents the current source cards/transclusions from
  silently treating feed summaries and metadata packets as equivalent to
  reader snapshots.
remaining error field:
  Global Wire is still not ship-worthy. The next source-body realism axis is a
  second-stage reader/full-article extraction path for URLs where policy allows
  it, with clear failure states and no dead mobile deep links. Separate
  remaining axes are front-page ranking by prominence/novelty/freshness and
  VText-agent-owned publication-quality article prose.
rollback refs:
  Revert 55ddce3f and 8433aa50 to remove body availability fields from the
  Source Service API/storage path. Revert 042c35f1 only if the problem
  checkpoint should be removed from the mission history.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T09:02Z reader-snapshot enrichment blocker
  documented before the next behavior change.
current artifact state:
  The latest pushed commit is docs-only
  594da5dddf4a08b3c4c573ea95666c74f04d780a, which records the owner's
  deletion-bias eval note. Staging behavior is still the source-body
  classification slice from 55ddce3f1c61fdac8b6d50840f7e24a03f8bb501 unless a
  newer deploy has landed externally.
new evidence:
  Code inspection found that `ensureGlobalWireSourceServiceContentItem` in
  `internal/runtime/global_wire.go` still creates the article-attached source
  ContentItem from `result["body"]` alone. When a Source Service result is an
  RSS feed summary, empty feed item, or GDELT metadata packet, Global Wire
  stores that weak body as the source item text even though the source may have
  an HTTP URL that could yield a reader snapshot.

  The runtime already has a reusable URL reader path:
  `Runtime.ImportURLContent` in `internal/runtime/content.go`. It validates
  HTTP URLs, fetches/extracts readable text, stores a ContentItem, and marks
  HTML imports with `reader_artifact_kind=cleaned_reader_markdown`. Reusing this
  path is preferable to adding a new crawler.

  The Source Service storage schema already stores source policy on the
  `sources` table (`tos_class`, `robots_policy`, `auth_policy`,
  `store_body_policy`). However `SearchItems`, `GetItem`, and
  `sourceAPIItemResult` currently return item body classification but not the
  source policy fields. As a result, Global Wire cannot make a policy-aware
  decision about whether a second-stage reader import is allowed for a given
  source result.
belief-state changes:
  The next source realism step is not "fetch everything." The safe topology is
  to expose source policy at the Source Service boundary, then let Global Wire
  attempt bounded reader-snapshot import only for policy-compatible URL source
  results and only when the existing body is not already a reader/source body.
remaining error field:
  Full article/source bodies are still partial. Source volume is high, but
  article-attached sources can remain feed summaries or metadata packets unless
  the policy-aware reader-snapshot enrichment path lands. The fix must avoid
  increasing background load blindly and must record failure/skip state rather
  than silently overclaiming source richness.
next executable probe:
  Add source policy fields to Source Service item results by joining item rows
  to their source rows. Then add bounded Global Wire source conversion logic
  that reuses `ImportURLContent` for allowed URL sources, records
  `reader_snapshot_status`, and preserves explicit skip/failure metadata for
  disallowed or failed imports.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T09:03Z policy-aware reader snapshot enrichment
  landed and was proved on staging.
current artifact state:
  Commit 3c432e125b0ffad4534bddddbfbef91686672d08 is on origin/main and
  deployed to staging. It follows documentation checkpoint
  36106c595157897c65b0eb788ed066f5a631ff8a, which recorded the reader-snapshot
  enrichment problem before the behavior change.
what shipped:
  - Source Service item search/resolve now joins `items` to `sources` and
    exposes `source_tos_class`, `source_robots_policy`,
    `source_auth_policy`, and `store_body_policy`.
  - Researcher/runtime source-search projections carry the same policy fields.
  - Global Wire source-service ContentItem conversion now records source policy
    fields in metadata.
  - When a Source Service result is not already a reader snapshot and has
    weak body kind (`empty` or `feed_summary`) plus an allowed store-body
    policy (`bounded_text` or `bounded_release_text`), Global Wire attempts a
    direct, bounded `ImportURLContent` reader import with a 12 second context
    timeout and no SearXNG fallback query.
  - Successful reader imports replace the deterministic Global Wire
    source-service item body with extracted reader text and record
    `reader_snapshot=true`, `body_kind=reader_snapshot`,
    `reader_snapshot_status=imported`, and the imported reader content id.
  - Disallowed, unsupported, failed, or low-content imports preserve the
    original source-service body and record explicit `reader_snapshot_status`
    metadata such as `skipped_store_body_policy`, `skipped_body_kind`,
    `fetch_failed`, or `low_content`.
what was proven:
  - Local `git diff --check` passed.
  - Local `nix develop -c go test ./internal/cycle ./cmd/sourcecycled ./internal/sourceapi ./internal/sources`
    passed.
  - Local `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWireSourceSearchImports|TestHandleGlobalWireSourceSearchImportsAllowedReaderSnapshot|TestResearcherSourceSearchCallsSourceServiceAPI'`
    passed.
  - Local `nix develop -c go test ./internal/runtime -run 'TestGlobalWire|TestHandleGlobalWire|TestVTextSourceService'`
    passed.
  - Local `nix develop -c go test ./cmd/sourcecycled ./internal/cycle` passed.
  - CI run 27126653731 passed for commit
    3c432e125b0ffad4534bddddbfbef91686672d08, including Go gates and staging
    deploy.
  - FlakeHub run 27126653950 passed for the same commit.
  - Public staging `/health` reported proxy and sandbox deployed commit
    3c432e125b0ffad4534bddddbfbef91686672d08 with
    `deployed_at=2026-06-08T08:56:51Z`.
  - Node B Source Service health reported `status=ok`, `item_count=71470`,
    and `fetch_count=14024` at 2026-06-08T08:59:09Z.
  - Node B Source Service search for `technology` returned live policy fields:
    RSS result `srcitem_92f12eebfead4be3a08c68a9` from `rss:finextra` had
    `body_kind=feed_summary`, `reader_snapshot=false`,
    `store_body_policy=bounded_text`, `source_auth_policy=none`, and
    `source_robots_policy=feed_allowed`; GDELT results had
    `store_body_policy=bounded_metadata` and `source_robots_policy=dataset_feed`.
  - Authenticated deployed product-path call to
    `POST https://choir.news/api/global-wire/source-search` with query
    `technology` and `max_results=1` returned status 200 and converted the
    live `rss:finextra` result into ContentItem
    `global-wire-source-service-3d83dde8-9a75-5429-8dcf-db6b7fd36087`.
    Because the original article URL returned HTTP 403, the stored metadata
    recorded `reader_snapshot_status=fetch_failed`,
    `reader_snapshot_error="URL import failed: direct_http returned status 403 Forbidden"`,
    `body_kind=feed_summary`, `reader_snapshot=false`,
    `store_body_policy=bounded_text`, and
    `source_robots_policy=feed_allowed`. This proves the deployed product path
    attempts eligible imports and preserves honest failure state rather than
    silently overclaiming a feed summary as a full source body.
belief-state changes:
  Global Wire source conversion is now policy-aware and has explicit
  reader-snapshot attempt/skip/failure state. The system still does not
  guarantee full article bodies for all source types, but it no longer has to
  collapse source-body weakness into an unlabelled text item.
unproven or partial claims:
  - Staging proof saw an eligible live source fail with HTTP 403; successful
    live reader import was not found in the quick deployed probe. Successful
    import is covered by local runtime test using a real HTTP reader server.
  - Existing deterministic Global Wire source-service ContentItems are not
    backfilled or updated if they were created before this commit; the new path
    affects new source conversions.
  - GDELT article URLs remain metadata packets because the GDELT source policy
    is `bounded_metadata`; a separate policy/design decision is required before
    fetching article URLs discovered through a dataset source.
remaining error field:
  The next realism axes remain front-page ranking by prominence/importance/
  novelty/freshness, VText-agent-owned publication-quality article prose with
  native source transclusion, and eventually a redesigned source exploration
  workflow if one is actually needed. Do not restore the deleted source
  chronology/search ledger or bespoke Style.vtext controls.
rollback refs:
  Revert 3c432e12 to remove policy-aware reader snapshot enrichment and source
  policy fields from the Source Service result path. Revert 36106c59 only if
  the problem checkpoint should be removed from mission history.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T09:15Z front-page freshness/ranking problem
  documented before behavior changes.
current artifact state:
  Staging serves behavior commit 3c432e125b0ffad4534bddddbfbef91686672d08.
  Latest origin/main is docs checkpoint
  8ff6f049c92c2a1612b88616e9721f4067453b3d.
new evidence:
  Code inspection found that the seeded Global Wire stories in
  `internal/store/global_wire.go` still store literal freshness copy such as
  `updated 18 min ago`, `updated 41 min ago`, and `updated 1 hr ago`.
  The frontend fallback data in `frontend/src/lib/GlobalWireApp.svelte` repeats
  the same hardcoded strings. This matches the user-observed failure: the top
  stories can sit on screen for hours while still claiming minute-scale updates.

  `ListGlobalWireStories` orders durable stories by `prominence DESC,
  updated_at DESC`. That is not a full importance/novelty ranking model, but it
  is at least a stable current ordering primitive. The more immediate product
  bug is that seed/story freshness is copy, not state. Runtime source-network
  VText stories already use `sourceMaxxFreshness(doc.UpdatedAt)`, and graph
  candidate promotion sets `story.UpdatedAt = now`, so real promoted updates can
  use relative time honestly.
belief-state changes:
  The first honest ranking/freshness slice should separate seed source
  neighborhoods from live updates. Seed stories should say they are seeded
  source neighborhoods, not recently updated news. VText-derived and promoted
  stories can use relative updated time from their actual `UpdatedAt`.
remaining error field:
  This will not implement a complete prominence/importance/novelty ranking
  model. It only removes a misleading time signal and prevents fallback UI from
  showing stale minute-copy. The larger ranking problem remains open.
next executable probe:
  Normalize Global Wire story presentation at the API boundary: for seeded
  stories, replace hardcoded `updated ... ago` freshness with an explicit seed
  status; for non-seeded stories whose freshness is empty or update-like,
  derive relative freshness from `UpdatedAt`. Update frontend fallback copy and
  focused tests.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T09:18Z honest seed freshness landed and was proved
  on staging.
current artifact state:
  Commit fca07803a56606fa00500a76c36dffaa78d27fbb is on origin/main and
  deployed to staging. It follows documentation checkpoint
  9fef8eb163835927b3e8d6f1a32708a6b525cd7, which recorded the stale/fake
  freshness problem before the behavior change.
what shipped:
  - Seed Global Wire stories no longer store hardcoded freshness strings like
    `updated 18 min ago`, `updated 41 min ago`, or `updated 1 hr ago`.
  - Frontend fallback story data no longer repeats those fake minute labels.
  - `/api/global-wire/stories` now normalizes story presentation before
    response: seeded source neighborhoods with auto/update-like freshness show
    `seed source neighborhood`, while non-seeded stories with update-like
    freshness derive relative freshness from actual `UpdatedAt`.
what was proven:
  - Local `git diff --check` passed.
  - Local `nix develop -c go test ./internal/runtime -run 'TestHandleGlobalWireStories'`
    passed.
  - Local `nix develop -c go test ./internal/store -run 'TestGlobalWire'`
    passed.
  - Local `cd frontend && npm run build` passed.
  - CI run 27127523355 passed for commit
    fca07803a56606fa00500a76c36dffaa78d27fbb, including Go gates, frontend
    build, and staging deploy.
  - FlakeHub run 27127523420 passed for the same commit.
  - Public staging `/health` reported proxy and sandbox deployed commit
    fca07803a56606fa00500a76c36dffaa78d27fbb with
    `deployed_at=2026-06-08T09:13:29Z`.
  - Authenticated deployed product-path call to
    `GET https://choir.news/api/global-wire/stories` returned status 200 and
    the three seeded stories with `freshness=seed source neighborhood`,
    `source_state=seeded-source-neighborhood`, and prominence values 82, 74,
    and 63. No `updated 18 min ago`, `updated 41 min ago`, or `updated 1 hr
    ago` labels remained in the API response.
belief-state changes:
  Global Wire no longer falsely claims that seeded placeholder stories were
  updated minutes ago. The front page is still not a real editorial ranking
  engine, but its freshness signal is now honest about seed state versus live
  update state.
unproven or partial claims:
  - This does not solve ordering by prominence, importance, novelty, source
    volume, or source diversity.
  - It does not replace seeded story copy with current top world news.
remaining error field:
  The next major product axis is ranking and article ownership: current source
  cycles/processors should produce VText-agent-owned articles whose front-page
  order is based on actual source-network prominence, novelty, freshness,
  contradiction, and update pressure rather than static seed prominence.
rollback refs:
  Revert fca07803 to restore the previous hardcoded seed freshness behavior.
  Revert 9fef8eb only if the problem checkpoint should be removed.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T09:35Z product-path continuation submitted and
  front-page population problem narrowed before behavior changes.
current artifact state:
  Staging still reports deployed commit
  fca07803a56606fa00500a76c36dffaa78d27fbb. Codex submitted a new
  authenticated prompt-bar/VText continuation through Comet, asking Choir to
  run this mission as a MissionGradient continuation focused on the remaining
  Global Wire front-page article population/ranking axis. The visible mission
  VText id in the app was `1bd1038e-b...d733c3`, and the VText run shown in
  the UI was `296c079b-ac0d-4781-8d8c-d905a3e0f50b`. The VText created a
  structured problem statement and called `request_super_execution`.
new evidence:
  Comet/browser product-path observation before the raw-API navigation showed
  the signed-in Global Wire app rendering `3 articles`, with the deleted
  Sources Chronology/search surface and bespoke Style.vtext controls still
  absent. The three visible articles were the seeded source-neighborhood
  stories.

  A Node B internal diagnostic against the active sandbox service at
  `http://127.0.0.1:8085/api/global-wire/stories`, with the same visible owner
  id injected by the proxy header, returned 15 stories:

  - 12 `source-network-vtext-*` live articles owned by `global-wire-platform`,
    including Iran regional infrastructure risk, Iran missile interceptions,
    Xi in Pyongyang, ERCOT/data-center grid risk, Kashmir clashes,
    Israel/Iran escalation, Ukraine/Crimea supply lines, DRC Ebola, ISS leak,
    HN AI-career anxiety, DRC Ebola, and Nigerian hostage rescue;
  - followed by the 3 seeded owner stories with `seed source neighborhood`.

  That diagnostic means the deployed backend source-network VText index is
  capable of producing live front-page article candidates. The stronger
  current hypothesis is no longer "the VText article index is empty." It is
  one of:

  - the public/Svelte app session or candidate route is rendering from a
    preview/stale route even while showing owner-looking labels;
  - the public proxy/session path used by the SPA differs from the active
    sandbox service queried internally;
  - the Global Wire component loaded before the live stories were available
    and does not refresh after source-network updates;
  - the app window remained mounted with stale state while the backend had
    already advanced.

  Direct Comet navigation to `https://choir.news/api/global-wire/stories`
  returned `401 authentication required`, and navigating back degraded the
  tab to signed-out preview. Treat this as a browser/session diagnostic, not
  proof that the SPA fetch itself is unauthenticated.
belief-state changes:
  The immediate front-page population bug appears to be a product-path
  visibility/refresh/session-route issue, not absence of live VText-owned
  source-network articles. The backend already has enough live article objects
  to exceed the three seeded stories; the shipped UI and/or public product
  route is not reliably surfacing them to the owner.
remaining error field:
  The mission still needs a reviewable AppChangePackage or precise blocker
  from the product-path worker run. Any fix must preserve the deletion of the
  detritus source ledger and bespoke Style.vtext controls, must not replace
  VText-owned articles with static story fixtures, and must distinguish
  internal diagnostics from public authenticated product proof.
next executable probe:
  Inspect the product-path run that started from VText
  `1bd1038e-b...d733c3` and run `296c079b-ac0d-4781-8d8c-d905a3e0f50b`. If it
  publishes an AppChangePackage, review the package diff and evidence before
  landing. If it does not, root-cause whether `request_super_execution`
  failed, worker lease/delegation failed, or the worker reported only a VText
  checkpoint. In parallel, inspect the Global Wire frontend load/refresh logic
  and candidate-route/proxy session behavior for why a backend response with
  15 stories can render as a 3-story front page.
rollback refs:
  No behavior change in this checkpoint. Staging rollback remains
  fca07803a56606fa00500a76c36dffaa78d27fbb.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T09:57Z Global Wire mounted-window story-load retry
  fix landed and was proved on staging.
current artifact state:
  Commit 24c589ca88fb3ab715b6a616bf5092e357868a29 is on origin/main and
  deployed to staging. It follows documentation checkpoint
  32601389f63d0c267900f8d4f4796d3a55038bd6, which narrowed the front-page
  population problem before the behavior change.
what shipped:
  - `GlobalWireApp.svelte` no longer treats an authenticated Global Wire story
    load as complete before the fetch succeeds.
  - A transient authenticated `/api/global-wire/stories` failure no longer
    pins the mounted window to the three-story preview/seed state forever.
  - Authenticated Global Wire windows retry after transient failure, refresh
    periodically, and refresh when the window regains focus or the page becomes
    visible.
  - The old Sources Chronology/search ledger and bespoke Style.vtext controls
    remain deleted.
what was proven:
  - Local `git diff --check` passed.
  - Local `cd frontend && npm run build` passed.
  - Local `cd frontend && PLAYWRIGHT_BASE_URL=http://localhost:4173 npm run e2e
    -- global-wire-app.spec.js` passed after running the local service harness
    in the repo Nix dev shell. The new regression proves an authenticated
    Global Wire window whose first `/api/global-wire/stories` request returns
    503 retries and renders four live source-network stories instead of
    staying on the three preview stories.
  - CI run 27129723868 passed for commit
    24c589ca88fb3ab715b6a616bf5092e357868a29, including Go shards, non-runtime
    tests, frontend build, and staging deploy.
  - FlakeHub run 27129723860 passed for the same commit.
  - Public staging `/health` reported proxy and sandbox deployed commit
    24c589ca88fb3ab715b6a616bf5092e357868a29 with
    `deployed_at=2026-06-08T09:54:49Z`.
  - Deployed frontend index referenced `GlobalWireApp-D9OMYV56.js`, and that
    deployed chunk contained the new authenticated retry/refresh code
    (`setInterval(... force: true, silent: true ...)` and
    `visibilitychange`).
  - Authenticated deployed sandbox API proof for owner
    `5bd6de97-3b58-408c-bf89-c42c81b083de` returned 15 Global Wire stories
    from `durable-storygraph+source-network-vtexts`; the first three were live
    `source-network-vtext-*` platform-owned VText articles with real headlines
    and `updated just now` freshness.
belief-state changes:
  The specific 3-story visible state is now plausibly explained by a mounted
  Global Wire window caching preview/failure state after a transient
  authenticated route failure. The backend article index was not empty; it was
  already capable of returning live source-network VText articles. The shipped
  frontend now keeps the mounted window connected to that advancing backend
  state instead of treating the first authenticated attempt as final.
unproven or partial claims:
  - This does not prove the exact earlier Comet window refreshed in place after
    the user-visible failure; it proves the deployed code path and the local
    product regression for the same failure class.
  - This does not solve ranking quality, source prominence scoring, processor
    overload, or publication-quality article prose.
  - No product-path Choir-in-Choir AppChangePackage appeared for this probe;
    Codex landed the frontend retry fix directly after the prior docs
    checkpoint.
remaining error field:
  Global Wire still needs the larger news mission: processors and reconcilers
  should keep producing publication-quality VText-agent-owned articles from
  many source bodies, front-page ordering should reflect source-network
  prominence/novelty/freshness/contradiction/update pressure rather than flat
  source-network prominence scores, and the Choir-in-Choir platform PR
  accelerator still needs to prove a worker can produce a reviewable
  AppChangePackage or a precise blocker for this class of platform work.
highest-impact remaining uncertainty:
  Whether the current Choir-in-Choir request path reliably converts a VText
  mission continuation into a Super/vsuper worker run with durable
  AppChangePackage evidence. The observed UI run called
  `request_super_execution`, but no package was visible through the active
  deployed API before this Codex-landed fix.
next executable probe:
  Resume the mission on the Choir-in-Choir accelerator axis: trace the
  prompt-bar/VText/Super handoff for the `1bd1038e-b...d733c3` mission window
  and `296c079b-ac0d-4781-8d8c-d905a3e0f50b` visible run. Determine whether
  the product run is stored under a different owner/candidate route, whether
  `request_super_execution` failed to lease a worker, or whether the worker
  produced only a VText checkpoint. Then either fix the handoff/package path
  with documentation-first discipline or record the exact blocker.
rollback refs:
  Revert 24c589ca to restore the previous one-shot authenticated Global Wire
  load behavior. Revert 32601389 only if the problem checkpoint should be
  removed from mission history.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T10:08Z Choir-in-Choir continuation reached Super
  but failed before worker delegation or package/blocker publication.
current artifact state:
  Staging behavior commit is 24c589ca88fb3ab715b6a616bf5092e357868a29. The
  owner-routed desktop is active at VM vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19
  with sandbox URL http://10.202.180.2:8085. No new Global Wire
  AppChangePackage is visible in that routed desktop; the only listed package
  is the old unrelated RBO package 84c12250-2d0b-43f3-b5ed-90f8e051634e.
new evidence:
  The product-path continuation is trajectory
  cff02946-bcc0-46e0-bacf-f3641750a250. It is stored in the owner-routed
  sandbox, not the active platform sandbox. Its agents were conductor
  completed, vtext completed, and super failed. The VText agent for document
  296c079b-ac0d-4781-8d8c-d905a3e0f50b created a mission revision, invoked
  `request_super_execution`, emitted a channel message to
  `super:5b`, and received `request_super_execution returned`.

  Super then started its loop and performed many repo/file search operations
  (`glob`, `read_file`, `bash`, `grep`). It compacted context from 177,714
  tokens to 31,200 tokens, then continued reading/searching. The terminal trace
  event is `loop.failed` with summary
  `tool loop: model stopped at max_tokens (iteration 30)`.
belief-state changes:
  This is not a VText-to-Super routing failure and not an AppChangePackage
  review failure. The handoff to persistent Super succeeded, but Super never
  turned the mission into a worker-medium/vsuper delegation or a precise
  owner-readable blocker before hitting the tool-loop model/iteration limit.
  This is a supervision-handoff discipline/runtime substrate problem: long
  Super investigations can consume their whole loop budget reading context and
  then fail silently from the product point of view, leaving no reviewable
  package and no mission VText blocker.
remaining error field:
  The Choir-in-Choir accelerator still cannot be called reliable for platform
  work. A product-path mission continuation can reach Super, but Super needs a
  deterministic obligation to checkpoint, delegate, or publish a precise
  blocker before loop exhaustion. The next code/design probe should find where
  max-token/iteration-stop failures are handled and ensure this failure mode
  creates durable owner-readable state rather than only a failed Trace.
next executable probe:
  Inspect the agent loop termination path for `model stopped at max_tokens`
  and the Super profile/tool policy around MissionGradient work. Prefer a
  bounded fix that forces mission Super runs to emit a checkpoint/blocker VText
  or deterministic delegation request before terminal budget exhaustion. Add
  regression coverage for loop exhaustion preserving a durable checkpoint or
  explicit blocker, then rerun a product-path mission continuation.
rollback refs:
  No behavior change in this checkpoint. Staging rollback remains
  24c589ca88fb3ab715b6a616bf5092e357868a29.
```

```text
status: shipped_partial
last checkpoint: 2026-06-08T10:26Z Super pre-delegation failure fallback
  landed and was proved through CI/staging identity.
current artifact state:
  Documentation checkpoint 591da77be7874adff6087f1e5375a978ed8475d9 records
  the observed Super failure before the behavior change. Behavior commit
  8b32ebfae80274cd5a69d5303330b3a66da0abbe is on origin/main and deployed to
  staging.
what shipped:
  - `handleExecutionError` now invokes a Super failure fallback after the
    existing delegate-worker fallback.
  - When persistent Super was requested by a VText and fails without
    `submit_coagent_update` or delegate-worker evidence, runtime synthesizes a
    structured VText-addressed blocker update instead of leaving only a failed
    Trace.
  - The synthesized update records the Super run id, trajectory id, error,
    successful tool names, and whether worker lease/delegation evidence was
    missing.
  - Existing delegate-worker fallback behavior is preserved and takes
    precedence when `delegate_worker_vm` evidence exists.
what was proven:
  - Local `git diff --check` passed.
  - Local focused ordinary test passed:
    `nix develop -c go test ./internal/runtime -run 'TestRuntimeSynthesizesVTextBlockerWhenSuperFailsBeforeDelegation'`.
  - Local focused comprehensive test passed:
    `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestSuperFailureAfterDelegateSynthesizesWorkerUpdate'`.
  - CI run 27131235293 passed for commit
    8b32ebfae80274cd5a69d5303330b3a66da0abbe, including Go vet/build,
    all runtime shards, non-runtime tests, integration-tagged smoke, and
    staging deploy.
  - FlakeHub run 27131235269 passed for the same commit.
  - Public staging `/health` reported proxy and sandbox deployed commit
    8b32ebfae80274cd5a69d5303330b3a66da0abbe with
    `deployed_at=2026-06-08T10:23:37Z`.
belief-state changes:
  The particular failure mode from trajectory
  cff02946-bcc0-46e0-bacf-f3641750a250 should no longer disappear into a
  failed Super trace with no owner-readable blocker. If a VText-requested
  Super again spends its loop budget before delegation, VText should receive a
  blocker update that can drive a follow-up revision or resumed Super request.
unproven or partial claims:
  This does not prove a new product-path mission continuation now reaches
  worker-medium or publishes an AppChangePackage. It only proves the runtime
  fallback for the pre-delegation failure class.
remaining error field:
  The accelerator still needs a fresh product-path rerun: VText mission
  continuation -> Super -> worker-medium/vsuper delegation -> package or
  precise blocker -> VText narrative. The next run should either make progress
  to a worker artifact or, if Super fails again before delegation, surface the
  new synthesized blocker in the mission VText channel.
rollback refs:
  Revert 8b32ebfa to remove the Super pre-delegation blocker fallback. Revert
  591da77b only if the problem checkpoint should be removed from mission
  history.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T10:40Z product-path rerun reached worker-medium,
  but terminal worker reporting failed after cancellation.
current artifact state:
  Behavior commit 8b32ebfae80274cd5a69d5303330b3a66da0abbe remains deployed
  to staging and active on the owner-routed desktop sandbox at
  http://10.202.180.2:8085. Product-path rerun trajectory
  ea8013ff-ca9b-4ecb-99b4-e7dd0431bf40 created VText document
  a6948b75-5a92-40f1-8cb4-08522f0ba4a3 and delegated to worker sandbox
  http://10.202.207.2:8085 on worker VM
  vm-a8d52e96d64e6dc1d46ad6542439019f.
new evidence:
  The rerun improved over the prior failure. VText sent Super a concrete
  instruction to lease worker-medium and call `start_worker_delegation`.
  Super did so. The worker run id was
  61574de4-578f-4608-8ddc-85e7b232b606, and the worker repo bootstrap cloned
  `https://github.com/choir-hip/go-choir.git` into Source/platform and
  Source/candidate at commit 8b32ebfae80274cd5a69d5303330b3a66da0abbe.

  The worker did not publish an AppChangePackage. It entered broad source
  reading, spawned co-super child
  4cb584d5-7aff-4a06-bbd7-34c9c1e98de9, which failed with Fireworks
  `503 Service Unavailable`, then spawned co-super child
  e745acec-f96c-482d-8849-d54aef88efba, which began editing but did not
  produce a package before the parent cancellation. Super observed the worker
  repeatedly, redirected it twice, then cancelled the worker delegation.

  The cancellation certificate recorded this reason: the worker was stuck in
  a read-only loop across 156 events, two redirects were ignored, one child
  failed with Fireworks 503, the second child was repeating reads/edits, and
  zero code changes, commits, or AppChangePackages were produced. After
  cancellation, Super attempted `submit_coagent_update` three times, but each
  attempt failed with:
  `tool_error: resolve delivery target lookup: record not found`.

  The previously shipped fallback did fire earlier in the same trajectory:
  when Super hit a Fireworks 503 before terminal reporting, the runtime
  synthesized a VText-visible blocker update. That proves the
  pre-delegation/failed-Super fallback works for one class, but does not cover
  this later explicit `submit_coagent_update` target-resolution failure.
belief-state changes:
  Choir-in-Choir can now reach worker-medium/vsuper through the product path,
  clone the candidate repository, spawn worker-local helpers, and expose
  worker progress to the mission VText. The next reliability blocker is no
  longer initial Super delegation. It is terminal coordination under failure:
  after cancellation or continuation handoff, Super may be unable to deliver
  the final blocker/cancellation certificate because `submit_coagent_update`
  cannot resolve its target.
remaining error field:
  The platform PR accelerator is not ready for overnight unattended platform
  work. It can begin the work, but it still lacks a reliable terminal action
  path. A worker that fails to produce a package must always leave a
  VText-visible blocker or cancellation certificate. The observed target
  lookup failure means the mission can still strand decisive evidence in Trace
  instead of returning it to the owner-readable VText narrative.
next executable probe:
  Document and fix the `submit_coagent_update` delivery-target resolution
  failure after Super continuation/cancellation. The fix should preserve
  normal addressed updates, but fall back to the VText channel/document when
  the explicit delivery target lookup fails and the run has an owner-scoped
  channel id. Add regression coverage for a Super continuation that cancels a
  worker and then submits a blocker update to the originating VText.
rollback refs:
  No code change in this checkpoint. Revert 8b32ebfa to remove the earlier
  Super pre-delegation blocker fallback if it causes regressions.
```

```text
status: shipped_partial
last checkpoint: 2026-06-08T10:56Z stale coagent update target fallback
  landed and was proved on staging.
current artifact state:
  Documentation commit 0e4f38cccd0be2af1637de41b2d12b4d9694232d records
  the terminal-reporting blocker before the fix. Behavior commit
  d487b2090f543af7fda6531c14a203af4a82d808 is on origin/main and deployed to
  staging.
what shipped:
  `submit_coagent_update` target resolution now preserves normal parent,
  requester, and existing explicit-agent routing, but if an explicit target
  agent is stale/missing and the run still has a VText channel/document
  context, it falls back to `vtext:<channel_id>` instead of failing with
  `resolve delivery target lookup: record not found`.
what was proven:
  - Local `git diff --check` passed.
  - Local focused ordinary runtime test passed:
    `nix develop -c go test ./internal/runtime -run 'TestRuntimeSynthesizesVTextBlockerWhenSuperFailsBeforeDelegation'`.
  - Local focused comprehensive tests passed:
    `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestSubmitWorkerUpdate(FallsBackToVTextChannelWhenExplicitTargetMissing|UsesVTextRequesterMetadataWhenAgentMissing)$'`.
  - CI run 27132481605 passed for commit
    d487b2090f543af7fda6531c14a203af4a82d808, including Go vet/build,
    runtime shards, non-runtime tests, integration-tagged smoke, and staging
    deploy.
  - FlakeHub run 27132481602 passed for the same commit.
  - Public staging `/health` reported proxy and sandbox deployed commit
    d487b2090f543af7fda6531c14a203af4a82d808 with
    `deployed_at=2026-06-08T10:48:51Z`.
  - Owner-routed sandbox http://10.202.180.2:8085 also reported deployed
    commit d487b2090f543af7fda6531c14a203af4a82d808.
  - Deployed product-path acceptance probe used prompt-bar plus
    `POST /api/vtext/documents/46ce1a2f-c5ba-46a9-ad9e-aa3036a4c955/revise`.
    VText requested Super execution; Super called `submit_coagent_update`
    with stale `agent_id=5bd6de97-3b58-408c-bf89-c42c81b083de` and update id
    `deployed-stale-target-fallback-20260608T1050Z`.
  - The deployed tool result was successful:
    `status=submitted`, `agent_id=vtext:46ce1a2f-c5ba-46a9-ad9e-aa3036a4c955`,
    `channel_id=46ce1a2f-c5ba-46a9-ad9e-aa3036a4c955`, `cursor=2`,
    `trajectory_id=c0ee94d8-786f-49ae-a540-2af370bf8c30`.
  - VText consumed that worker update into revision
    ee7adeb5-376f-45a1-9a4b-a798a80223f7; revision metadata includes
    `worker_updates_consumed` with the Super channel message.
residual risk:
  The acceptance trajectory later failed on an unrelated Fireworks
  `412 Precondition Failed` during extra VText/Super model turns. That does
  not invalidate the routing fix, but it remains a separate provider/runtime
  reliability issue for unattended overnight runs.
remaining error field:
  Choir-in-Choir is now better at returning terminal blockers to VText after
  stale target ids, but the larger platform PR accelerator still needs a new
  overnight rerun that reaches package/adoption evidence or a product-level
  blocker. The worker itself still showed a tendency toward broad read loops,
  co-super provider failures, and cancellation without product code output.
next executable probe:
  Rerun the overnight mission with a tighter initial worker objective that
  names one implementation target and terminal artifact, then inspect whether
  the worker deletes the Global Wire detritus source ledger surface as recorded
  in `docs/choir-in-choir-deletion-bias-eval-note-2026-06-08.md`.
rollback refs:
  Revert d487b2090f543af7fda6531c14a203af4a82d808 to remove the stale-target
  VText channel fallback. Revert 0e4f38cc only if the problem checkpoint
  should be removed from mission history.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T11:10Z product-path mission continuation failed
  in VText before Super delegation.
current artifact state:
  Behavior commit d487b2090f543af7fda6531c14a203af4a82d808 remains deployed
  to staging and active on the owner-routed desktop sandbox at
  http://10.202.180.2:8085. Documentation commit
  5f9e0c95d8b5d3132186e16d4ee5d2723a98c8a67 records the prior routing proof.
new evidence:
  A focused product-path continuation was submitted through the owner-routed
  prompt bar with trajectory id 7be84ab9-0d79-4f95-9046-1e70decbd540. It
  created VText document e9c1e6e5-1235-4c8f-922d-8877c53e0071 and initial
  revision 805f52a0-8b83-444e-a04c-d55d8695f020.

  The continuation asked VText to create/update the mission narrative and then
  ask Super to lease worker-medium for the first implementation target:
  deleting the current Global Wire Sources Chronology/search/source-ledger
  surface, while preserving article-attached source transclusion/source-reader
  access.

  The run failed before any Super request or worker delegation. Trace moment
  ae6db819-2c39-478e-b90e-ab3be7515387 records:
  `tool loop iteration 0: gateway call failed: gateway client: fireworks:
  status 412 Precondition Failed (sanitized)`. The run log showed the VText
  provider path using Fireworks model
  `accounts/fireworks/models/deepseek-v4-flash`, prompt length about 11052,
  and forced tool choice `function:edit_vtext`.

  The VText document remained at its initial revision and did not receive a
  mission-state revision or a Super handoff. The error was posted to the
  trajectory channel, not converted into a useful owner-readable mission
  narrative revision.
belief-state changes:
  The stale `submit_coagent_update` target fallback is still useful, but the
  next reliability bottleneck for unattended Choir-in-Choir runs has shifted
  earlier: VText can fail on the initial model call before it has a chance to
  write a mission checkpoint or request Super. This makes an overnight run
  fragile even when downstream worker reporting is repaired.
remaining error field:
  Investigate the Fireworks 412 root cause in the VText provider/tool-choice
  path. Determine whether the request shape, forced tool choice, model policy,
  context size, provider adapter, or retry/fallback semantics are responsible.
  Do not treat this as a generic transient unless evidence shows repeated
  successful retry behavior under the same request shape. A VText initial-turn
  failure must either recover through an appropriate alternate provider/model
  path or leave a clear owner-readable blocker in the VText narrative.
next executable probe:
  Inspect gateway/provider request construction for VText forced tool calls,
  the model policy active on staging, and existing fallback behavior for
  Fireworks 412. Add focused regression coverage for a VText initial-turn
  gateway failure so the mission does not silently stop before Super.
rollback refs:
  No code change in this checkpoint. Revert only this checkpoint if the
  evidence is superseded by a more precise provider-root-cause document before
  any behavior fix lands.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T11:18Z deployed exact-tool-choice relaxation was
  proved, but VText still failed before Super delegation.
current artifact state:
  Documentation commit 28f17aee3a7f0e2dcfcbf8f3db2c2a30e5cda4ba records the
  initial VText Fireworks 412 failure. Behavior commit
  3c3521c02a08dfec3df15b29b6a4a34f15faf3a7 is on origin/main, passed CI run
  27133734082, passed FlakeHub run 27133734079, and is deployed to public
  staging plus the owner-routed sandbox. Public `/health` and
  http://10.202.180.2:8085/health both reported deployed commit
  3c3521c02a08dfec3df15b29b6a4a34f15faf3a7 with
  `deployed_at=2026-06-08T11:13:52Z`.
what shipped:
  The runtime tool loop now retries one initial exact tool-choice provider
  precondition failure by relaxing `tool_choice=function:<name>` to
  `tool_choice=required`. This keeps the tool-call requirement but avoids the
  exact OpenAI-compatible function-choice object when a provider rejects it.
what was proven:
  - Local `git diff --check` passed.
  - Local focused tests passed:
    `nix develop -c go test ./internal/runtime -run
    'TestRunToolLoop(RelaxesExactInitialToolChoiceAfterProviderPrecondition|RetriesProviderRateLimit|TerminalToolSuccessStopsWithoutExtraProviderTurn)$'`.
  - Local broader tool-loop tests passed:
    `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop'`.
  - CI run 27133734082 passed for commit
    3c3521c02a08dfec3df15b29b6a4a34f15faf3a7, including runtime shards,
    non-runtime tests, Go vet/build, integration-tagged smoke, and staging
    deploy.
  - FlakeHub run 27133734079 passed for the same commit.
  - Deployed product-path rerun trajectory
    fb765cc1-a72a-4ca8-9436-02d3e03f20f6 created VText document
    544b8868-a0ab-49cc-a17c-b73f2aa4998a and VText run
    1f2f4442-f2f0-4fda-ac8a-5eb07f1d1dec.
  - The deployed VText trace emitted `loop.retry` moment
    08567a2d-9b0e-463a-806a-fbaa5deea6e3 with summary
    `retry after exact_initial_tool_choice_precondition`, proving the new
    relaxation path fired on staging for the same failure class.
new failure evidence:
  After the retry, the VText run made a second provider call and failed again:
  `tool loop iteration 1: gateway call failed: gateway client: fireworks:
  status 412 Precondition Failed (sanitized)`. The second failure occurred
  after the exact-choice relaxation, so the root cause is not limited to the
  exact `function:edit_vtext` tool-choice object.

  The trace moment detail route returned `detail:null` for moments whose
  summaries reported `has_detail=true`, so the public trace API did not expose
  the full provider-call payload for this run. The available summaries still
  prove the sequence: first provider call, exact-choice retry, second provider
  call, second Fireworks 412.
belief-state changes:
  The deployed retry improved diagnosis and ruled out one narrow hypothesis,
  but it did not unblock the overnight mission. The failure now points toward a
  broader Fireworks/DeepSeek V4 Flash request-shape incompatibility for VText
  initial tool-calling turns, possibly involving the VText tool catalog size,
  prompt/system size, reasoning setting, or Fireworks handling of tool calls
  under this model path. It may also justify routing VText initial tool turns
  to a different configured model/provider when Fireworks returns this
  precondition class.
remaining error field:
  Continue root-cause investigation from the new evidence. The next fix should
  not merely retry the same Fireworks request again. Either reduce/alter the
  VText initial tool-call request shape in a principled way, or introduce a
  policy-respecting alternate model/provider fallback for this provider
  precondition class. A successful fix must be proved by a product-path rerun
  that reaches at least VText mission narrative creation and Super delegation,
  or by a VText-visible blocker if provider execution remains impossible.
next executable probe:
  Reproduce or narrow the 412 with a live provider-shaped request: compare
  Fireworks VText initial call with full VText tool catalog versus only
  `edit_vtext`, `request_super_execution`, and the terminal handoff tools; also
  compare DeepSeek V4 Flash to the configured Super/Pro model if policy allows.
  If the provider only fails with the full catalog, add a VText initial-turn
  tool-scope reduction that preserves needed tools while reducing provider
  request complexity. If it fails even with the small catalog, use a model
  fallback rather than repeated same-request retries.
rollback refs:
  Revert 3c3521c02a08dfec3df15b29b6a4a34f15faf3a7 to remove the exact-choice
  relaxation if it causes regressions. Revert 28f17aee only if the prior
  problem checkpoint should be removed from mission history.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T11:30Z scoped initial tool definitions shipped,
  but VText still failed before Super delegation.
current artifact state:
  Behavior commit cce5dd840abe06418bee93ed7753589a84a28d91 is on origin/main,
  passed CI run 27134314374, passed FlakeHub run 27134314437, and is deployed
  to public staging plus the owner-routed sandbox. Public `/health` and
  http://10.202.180.2:8085/health both reported deployed commit
  cce5dd840abe06418bee93ed7753589a84a28d91 with
  `deployed_at=2026-06-08T11:25:39Z`.
what shipped:
  Exact initial tool-choice turns now send only the exact requested tool
  definition in the provider request. For VText's first turn this scopes the
  initial provider-visible `tools` array to `edit_vtext`, while preserving the
  full runtime registry for executing the tool and later turns.
what was proven:
  - Local `git diff --check` passed.
  - Local focused tests passed:
    `nix develop -c go test ./internal/runtime -run
    'TestRunToolLoop(RelaxesExactInitialToolChoiceAfterProviderPrecondition|RetriesProviderRateLimit|TerminalToolSuccessStopsWithoutExtraProviderTurn)$'`.
  - Local broader tool-loop tests passed:
    `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop'`.
  - CI run 27134314374 passed for commit
    cce5dd840abe06418bee93ed7753589a84a28d91, including runtime shards,
    non-runtime tests, Go vet/build, integration-tagged smoke, and staging
    deploy.
  - FlakeHub run 27134314437 passed for the same commit.
new failure evidence:
  Deployed product-path rerun trajectory
  e2710f4c-d0fe-46f3-8b06-441f681f24ed created VText document
  55cddb8f-ea65-4629-9e9f-ea344338a848 and VText run
  0afb950d-35c2-4b78-8667-941fb06177be. The VText trace again emitted a retry
  after `exact_initial_tool_choice_precondition`, then failed on the second
  provider call:
  `tool loop iteration 1: gateway call failed: gateway client: fireworks:
  status 412 Precondition Failed (sanitized)`.

  Because cce5dd84 scoped the provider-visible tool definitions for that
  initial exact tool turn, this rules out the full VText tool catalog in the
  API `tools` array as the sole cause. The public trace detail route still
  returned `detail:null` for moments marked `has_detail=true`, so the exact
  tool-count payload was not inspectable through the deployed trace API.
belief-state changes:
  The root cause now appears broader than exact tool choice or tool-array
  breadth. The likely remaining axes are Fireworks DeepSeek V4 Flash rejecting
  the VText initial request shape even with a single tool, reasoning effort
  `medium` with tool calls, long VText system/prompt context, or the provider
  model itself for this appagent turn.
remaining error field:
  The next behavior route should stop retrying Fireworks Flash with minor
  request-shape changes and instead use a policy-respecting alternate
  configured model/provider for VText initial tool-call precondition failures,
  or a direct model policy change if owner-controlled policy is the intended
  product path. The fallback must preserve the same tool loop and role
  semantics; it should not hard-code VText as a special harness branch beyond
  using normal role/model selection and provider capability/error handling.
next executable probe:
  Add a provider-precondition fallback for initial tool-call turns that switches
  from the current Fireworks Flash selection to a configured alternate such as
  the role/fallback policy model or Fireworks Pro when available, then rerun
  the product path. If the alternate model writes the mission VText and reaches
  Super delegation, record the provider-specific failure as a model-policy
  tuning issue and continue the mission. If alternate model fallback also fails,
  synthesize a VText-visible blocker instead of stranding the owner on a failed
  trace.
rollback refs:
  Revert cce5dd840abe06418bee93ed7753589a84a28d91 to remove initial tool
  definition scoping. Revert 3c3521c02a08dfec3df15b29b6a4a34f15faf3a7 to
  remove exact-choice precondition retry.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T11:43Z provider-precondition model fallback
  shipped, but VText still failed before Super delegation.
current artifact state:
  Documentation checkpoint 9c9811d4635db3dba28284af20108e662491d2ec and
  behavior commit ef6121c7d7225de67045a67b0ba1a9fc92b44444 are on origin/main.
  CI run 27134954662 passed, including runtime shards, non-runtime tests, Go
  vet/build, integration-tagged smoke, and staging deploy. FlakeHub run
  27134954653 passed. Public `https://choir.news/health` and owner-routed
  `http://10.202.180.2:8085/health` both reported deployed commit
  ef6121c7d7225de67045a67b0ba1a9fc92b44444 with
  `deployed_at=2026-06-08T11:38:46Z`.
what shipped:
  Initial tool-call provider precondition failures now preserve the same VText
  tool obligation, first relaxing exact `function:edit_vtext` to `required`,
  then falling back from the active Fireworks Flash selection to the configured
  Fireworks Pro selection. The fallback uses normal LLM selection fields and
  the shared tool loop; it does not add a VText-only harness branch.
what was proven:
  - Local `git diff --check` passed.
  - Local focused tests passed:
    `nix develop -c go test ./internal/runtime -run
    'TestRunToolLoop(RelaxesExactInitialToolChoiceAfterProviderPrecondition|FallsBackModelAfterRelaxedInitialToolChoicePrecondition|RetriesProviderRateLimit|TerminalToolSuccessStopsWithoutExtraProviderTurn)$|TestProviderPreconditionFallbackSelectionsUseFireworksProForFlash'`.
  - Local broader focused runtime tests passed:
    `nix develop -c go test ./internal/runtime -run
    'TestRunToolLoop|Test.*ModelPolicy|TestProviderPreconditionFallbackSelections'`.
  - Product-path rerun trajectory d1d91874-778f-4152-8c17-bf167d6954e5
    created VText document 7b4ffe0f-1859-4943-8684-b57b299c7d68 and VText run
    5f485219-7609-45ed-b26e-323b5a82b8a1.
  - The deployed trace showed the exact intended sequence:
    Fireworks Flash exact initial tool call, retry after
    `exact_initial_tool_choice_precondition`, Fireworks Flash `required`, retry
    after `provider_precondition_fallback`, then Fireworks Pro `required`.
new failure evidence:
  The Fireworks Pro fallback also failed with
  `tool loop iteration 2: gateway call failed: gateway client: fireworks:
  status 412 Precondition Failed (sanitized)`. VText therefore still failed
  before creating an owner-readable mission narrative or requesting Super
  execution. The failure now rules out three narrower causes as sufficient
  explanations: exact tool choice, full provider-visible VText tool catalog,
  and Fireworks Flash specifically.
belief-state changes:
  This is now a Fireworks/provider-family incompatibility for the deployed
  VText initial tool-call request shape, or a provider-adapter bug shared
  across Fireworks Flash and Pro for tool calls. Continuing to adjust the same
  Fireworks request risks another shallow retry. The mission needs either an
  alternate provider/model family for the VText initial tool-mediated turn, or
  a product-path blocker synthesis that writes the failure into the VText
  narrative without requiring a failing Fireworks tool-call turn.
remaining error field:
  The next fix must not claim success by adding another Fireworks retry. It
  should use an existing policy-respecting non-Fireworks configured model path
  for initial VText tool calls, if available, or add an owner-visible
  terminal-blocker path that updates the VText through runtime-owned failure
  handling when the appagent cannot make its first provider turn. The desired
  continuation proof remains: VText mission narrative exists, Super delegation
  is requested, and the worker either deletes the Global Wire Sources
  Chronology/search/source-ledger surface or reports the exact product-path
  blocker and code path.
next executable probe:
  Inspect the deployed model catalog/policy for a non-Fireworks model already
  authorized for text/tool use. If one exists, extend provider-precondition
  fallback selection to that model and prove it through the owner prompt-bar
  path. If no such model exists, implement a narrowly scoped VText failure
  continuation that creates a normal VText revision recording the precise
  provider blocker and then requests Super only if the product authority model
  permits bypassing VText appagent authorship for the mission narrative.
rollback refs:
  Revert ef6121c7d7225de67045a67b0ba1a9fc92b44444 to remove model fallback.
  Revert cce5dd840abe06418bee93ed7753589a84a28d91 to remove initial tool
  definition scoping. Revert 3c3521c02a08dfec3df15b29b6a4a34f15faf3a7 to
  remove exact-choice precondition retry.
```

```text
status: checkpoint_incomplete
last checkpoint: 2026-06-08T11:56Z platform fallback commit deployed, but
  deployed VText did not attempt the platform fallback.
current artifact state:
  Documentation checkpoint ab84250df9bfa79d48dbfb7769490adcaa99e208 and
  behavior commit 427b3e178eafd964aa17b48a737a248c48e74539 are on origin/main.
  CI run 27135573511 passed, including runtime shards, non-runtime tests, Go
  vet/build, integration-tagged smoke, and staging deploy. FlakeHub run
  27135573490 passed. Public `https://choir.news/health` and owner-routed
  `http://10.202.180.2:8085/health` both reported deployed commit
  427b3e178eafd964aa17b48a737a248c48e74539 with
  `deployed_at=2026-06-08T11:51:09Z`.
what shipped:
  The provider-precondition fallback builder now appends the explicit runtime
  platform fallback provider/model after Fireworks Pro when the active
  selection is Fireworks and the platform fallback is non-empty. On staging,
  the sandbox runtime process exposes `RUNTIME_LLM_PROVIDER=chatgpt`,
  `RUNTIME_LLM_MODEL=gpt-5.5`, and `RUNTIME_LLM_REASONING_EFFORT=low`.
what was proven:
  - Local `git diff --check` passed.
  - Local focused tests passed:
    `nix develop -c go test ./internal/runtime -run
    'TestRunToolLoop(RelaxesExactInitialToolChoiceAfterProviderPrecondition|FallsBackModelAfterRelaxedInitialToolChoicePrecondition|TriesMultipleProviderPreconditionFallbacks|RetriesProviderRateLimit|TerminalToolSuccessStopsWithoutExtraProviderTurn)$|TestProviderPreconditionFallbackSelectionsUseFireworksProForFlash'`.
  - Local broader focused runtime tests passed:
    `nix develop -c go test ./internal/runtime -run
    'TestRunToolLoop|Test.*ModelPolicy|TestProviderPreconditionFallbackSelections'`.
  - Product-path rerun trajectory 61a2818b-ad93-4c1e-8776-526f982d3d39
    created VText document d0b1ac1e-7119-48db-a4c4-b6bcdec11aac and VText run
    245c2021-dc64-4e55-ab27-8110790a0490.
new failure evidence:
  The deployed trace showed Fireworks Flash exact, retry after
  `exact_initial_tool_choice_precondition`, Fireworks Flash `required`, retry
  after `provider_precondition_fallback`, and Fireworks Pro `required`. It then
  failed at `tool loop iteration 2` with Fireworks 412. It did not emit a
  second `provider_precondition_fallback` retry or a provider call for
  `chatgpt/gpt-5.5`.
belief-state changes:
  The latest code path works in local unit tests but not in the deployed VText
  execution path. The most likely causes are that `executeWithToolLoop` is not
  receiving `rt.cfg.LLMProvider/LLMModel` for this deployed VText run, the
  deployed VText path is using an older or separate runtime configuration
  object than the sandbox process environment suggests, or the fallback list is
  being truncated before reaching `RunToolLoop`.
remaining error field:
  The next fix should identify why the platform fallback is absent from the
  deployed VText run rather than adding another retry. Add trace-visible
  fallback-list metadata on provider-call start or retry, or add a focused
  runtime test that constructs a Runtime with `LLMProvider=chatgpt` and proves
  `executeWithToolLoop` passes both fallbacks into `RunToolLoop`.
next executable probe:
  Inspect runtime initialization and provider bridge setup for whether
  `runtime.LoadConfig()` and the `Runtime` object used by VText share the same
  config. Add test coverage at the `executeWithToolLoop`/runtime boundary if
  feasible. If the config is empty because runtime config is not preserved in
  the `Runtime`, fix that plumbing and rerun the owner prompt-bar path. If the
  config is present but still not visible, emit a bounded fallback-count/detail
  trace payload before the provider call.
rollback refs:
  Revert 427b3e178eafd964aa17b48a737a248c48e74539 to remove platform fallback
  ordering. Revert ef6121c7d7225de67045a67b0ba1a9fc92b44444 to remove model
  fallback. Revert cce5dd840abe06418bee93ed7753589a84a28d91 to remove initial
  tool definition scoping.
```
