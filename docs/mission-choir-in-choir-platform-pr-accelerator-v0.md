# Mission: Choir-in-Choir Platform PR Accelerator

**Status:** draft MissionGradient mission  
**Created:** 2026-06-08  
**Primary payload:** Global Wire/news-system reliability and source truthfulness
**Meta-goal:** prove Choir-in-Choir can accelerate platform work without owning
platform promotion.

## Goal String

```text
/goal Run docs/mission-choir-in-choir-platform-pr-accelerator-v0.md: use Choir-in-Choir to produce a reviewable platform PR/package for Global Wire reliability, with VText narrative, worker evidence, Playwright proof where useful, and Codex final review/landing authority.
```

## Mission Identity

This is not primarily a news mission and not primarily a self-development demo.

The real mission is to establish a useful near-term platform development loop:

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

## Why This Matters

Choir needs to become useful for its own development before it can accelerate
the whole project. The current Codex-only loop is effective but expensive in
attention and token budget. Choir-in-Choir should absorb more investigation,
candidate implementation, QA, and evidence packaging while keeping platform
merge authority outside the candidate system until the promotion path earns
trust.

The news system is a good payload because it has live complexity, real source
cycles, processor/reconciler failures, VText articles, browser-visible UI, and
high demand for repeated iteration.

## Current Belief State

- Choir has product concepts for foreground `super`, background/candidate
  `vsuper`, subordinate `cosuper`, `worker-medium`, and `worker-playwright`.
- The current intended source-transfer object is `AppChangePackage`; old
  patchset promotion is deprecated.
- Prior Choir-in-Choir work proved some package/adoption/rollback substrate,
  but the human-proof loop remained incomplete.
- `worker-playwright` exists conceptually as the heavy browser-proof worker
  class, but it must be reproven before relying on it.
- Candidate computers can mutate speculative state; canonical platform state
  changes only through external review and promotion.
- Platform-public changes still need the normal landing loop:
  `commit/PR -> CI -> deploy -> staging identity -> product proof`.

Highest-impact uncertainty:

```text
Can a visible product-path Choir run produce a reviewable platform candidate
artifact for a real news-system problem, with enough evidence that Codex can
review rather than rediscover the work?
```

## Cognitive Transform Set

### 1. Authority Transform

Do not ask "can Choir develop Choir?" in the abstract. Ask what authority the
candidate system is allowed to hold today.

Changed route: Choir-in-Choir can author, investigate, package, test, and open
review artifacts. Codex/human still owns platform merge/deploy until a
promotion-level path is separately proven.

### 2. Relief-Valve Transform

The point is not to replace Codex immediately. The point is to move token-heavy
search work out of Codex: source-cycle triage, trace reading, failure
classification, candidate tests, visual/browser QA, and first-pass fixes.

Changed route: optimize for Codex-reviewable evidence and diffs, not for
autonomous completion theater.

### 3. PR As Boundary Object

A PR is a social and technical boundary object: source diff, tests, review
comments, CI, deploy eligibility, and rollback context all attach to it.

Changed route: for platform work, the candidate output should be a branch/PR
or an AppChangePackage that can become a PR. If no PR can be created, the
mission must record exactly which source-transfer capability is missing.

### 4. News-System Payload Transform

Use Global Wire not as a side quest but as the stressor. A useful
self-development loop should improve a real live system, not only build a
toy Chyron feature.

Changed route: the first payload is Global Wire front-page truthfulness:
ranking freshness, source-body integrity, source opening behavior, and
processor/reconciler failure classification. These are narrow enough to review
but important enough to expose whether the platform is actually ingesting and
using live news.

### 5. Human-Proof Transform

The VText narrative is not a trace dump. It is the owner-readable story of
what was attempted, what changed, what failed review, and what the next worker
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
-> worker inspects source, traces, API evidence, and tests
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

## Payload: Global Wire Reliability

The first platform payload should be bounded:

```text
Investigate and improve Global Wire front-page truthfulness and reliability.
```

The candidate should start from the latest deployed Global Wire behavior and
answer:

- Why do the same top stories remain visible for hours while claiming
  recent update ages such as "18 min ago" or "41 min ago"?
- What ordering function chooses front-page stories, and does it account for
  prominence, importance, novelty, freshness, and source volume rather than
  only seed/update timestamps?
- Are source cards and article source transclusions backed by full ingested
  article bodies, or are seed placeholders/headline-level records still
  dominating the product path?
- When a source is opened on mobile web, why can an "Open source/original"
  action trigger an iOS app/deep-link prompt that goes nowhere?
- Which source cycle has recent processor/reconciler failures?
- Which run ids failed?
- What failure classes are present: provider/search/runtime/tool-contract,
  source-batch overload, VText edit failure, researcher handoff failure,
  timeout, cancellation, or bad source handle?
- Are failures retriable, resumable, or expected degradation?
- What owner-facing status should Global Wire show?
- What code/test change would make the system more reliable or more honest?

Do not let the first payload sprawl into article-quality, UI redesign, or
learned source track record unless failure root-cause directly requires it.
However, do not treat source expansion as optional polish: if the product is
still showing placeholder sources instead of full ingested articles, that is a
truthfulness defect and must be investigated.

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
- The desktop theme appeared to switch between Futuristic Noir and Carbon Fiber
  Kintsugi. The current root-cause hypothesis is an authenticated theme
  hydration race: the app applies `DEFAULT_THEME` on mount before loading the
  owner-saved server theme. The preferred fix is a non-authoritative last-good
  local boot cache for first paint, followed by server preference
  reconciliation; do not add authenticated startup latency just to avoid a
  theme flash.

Immediate pre-mission fix candidate:

```text
Fix theme hydration instability first if the code confirms the race, preserving
immediate shell paint, then re-run responsive/product proof. Fold stale
ranking, vacant sources, and source open behavior into the Choir-in-Choir
Global Wire payload unless they can be fixed narrowly without delaying the
mission.
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
- super/vsuper/worker run ids;
- worker-medium lease/class evidence;
- candidate workspace or package/branch identity;
- VText mission narrative document id and current revision;
- Trace/run refs;
- test commands and results;
- if browser proof is needed, worker-playwright lease and screenshot/video refs;
- final source artifact: GitHub PR, branch, or AppChangePackage;
- Codex review result.

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

1. **Readiness probe:** verify product-path Choir-in-Choir can start the run,
   create/update mission VText, lease `worker-medium`, and return a structured
   result.
2. **Source-transfer probe:** verify the worker can produce a reviewable source
   artifact: AppChangePackage, branch, or PR. If this fails, root-cause before
   attempting the news fix.
3. **News failure probe:** inspect latest Global Wire source-status and failed
   processor/reconciler runs.
4. **Candidate fix probe:** if root cause is inside scope, worker produces a
   candidate fix plus tests.
5. **Verifier probe:** independent verifier or Codex checks evidence and
   rejects/accepts.
6. **Review loop:** failed review revises mission VText and requeues worker;
   passed review may be landed by Codex.

Do not continue into broad news architecture work until the platform PR loop
itself is proven or precisely blocked.

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
- "failure fixed" when failures are merely hidden from UI.

## Stopping Conditions

### Complete

The mission is complete only if:

- Choir-in-Choir produces a reviewable platform artifact for the Global Wire
  reliability payload;
- Codex can review it without reconstructing the investigation from scratch;
- the mission VText records evidence and review state;
- either:
  - Codex lands the accepted fix through CI/deploy/staging proof, or
  - Codex rejects the artifact and the mission VText records actionable
    findings for the next worker revision.

### Checkpoint Incomplete

Use `checkpoint_incomplete` if the loop advances but does not finish, for
example:

- worker-medium lease works but source-transfer fails;
- AppChangePackage exists but PR creation is missing;
- PR exists but verifier proof is incomplete;
- news failure is documented but candidate fix is not ready.

### Blocked Incomplete

Use `blocked_incomplete` only after root-cause probes and cognitive transforms
if a blocker prevents progress, such as:

- staging auth/session prevents product-path run start;
- worker VM lease is broken;
- worker cannot access repo/source context;
- package/branch/PR source transfer is impossible with current product tools;
- worker-playwright class is unavailable or cannot run browser proof.

## Expected Overnight Route

The overnight run should bias toward proving the self-development loop before
deep news changes.

Likely route:

1. Confirm staging identity and current deployed SHA.
2. Start a product-path self-development prompt.
3. Have `super` create/update a mission VText.
4. Lease `worker-medium`.
5. Ask worker to inspect latest Global Wire failed processor/reconciler runs.
6. Require worker to produce either:
   - a small PR/package with tests for failure classification/status, or
   - a precise blocker explaining why it cannot.
7. If a UI/browser change is involved, lease `worker-playwright` for proof.
8. Codex reviews the artifact.
9. If acceptable, Codex lands it through normal platform loop.
10. If rejected, Codex records review findings in mission VText and requeues
    the worker if time remains.

## Run Checkpoint & Resumption State

```text
status: draft
last checkpoint: mission authored, not yet executed.
current artifact state:
  Choir-in-Choir has prior package/adoption substrate but no recent readiness
  proof for platform PR acceleration.
what shipped:
  Nothing from this mission yet.
what was proven:
  Nothing from this mission yet.
unproven or partial claims:
  worker-medium availability, worker-playwright availability, PR/package
  source transfer for platform work, VText narrative quality, Codex review loop
  integration, and Global Wire failure root-cause.
belief-state changes:
  None yet.
remaining error field:
  The platform can currently be changed reliably by Codex through git/CI/deploy,
  but Choir-in-Choir has not recently proven it can produce reviewable platform
  candidates that reduce Codex work.
highest-impact remaining uncertainty:
  Can a worker produce a reviewable source artifact for a live platform issue
  without direct main/deploy authority?
next executable probe:
  Start a staging product-path run that leases worker-medium for Global Wire
  failure investigation and asks for a PR/package or precise blocker.
suggested resume goal string:
  /goal Run docs/mission-choir-in-choir-platform-pr-accelerator-v0.md
evidence artifact refs:
  none yet.
rollback refs:
  platform rollback is current deployed main before any accepted fix lands.
```
