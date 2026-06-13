# Choir Doctrine Upgrade Report - 2026-06-13

## Executive Verdict

The doctrine upgrade is done at the documentation and operating-contract layer.
Choir now has a named apex doctrine, a doctrine-of-doctrine hierarchy, explicit
heresy accounting, upgraded high-read docs, updated mission portfolio framing,
and successor paradocs for the code-bearing residue discovered during the sweep.

The system is not "clean." That is the point of the doctrine upgrade. The work
made the remaining contradictions visible instead of hiding them behind a
coherent story. The remaining work is now named as code-bearing successor
missions, not silently folded into a docs-only doctrine pass.

Two commits carry the settled docs work:

- `79e98525` - `docs: land doctrine conformance sweep`
- `5f2ecec7` - `docs: add source intake routing paramission`

No runtime prompt defaults or behavior code were changed by the doctrine sweep
or the source-intake paramission checkpoint. The only non-doc doctrine surface
changed in the sweep was the local Parallax skill file, which is instruction
surface, not Choir runtime behavior.

## What Is Settled

The docs-level doctrine upgrade is settled.

Settled means:

- Choir Doctrine is the apex doctrine document.
- AGENTS, README, docs index, high-read architecture docs, platform app-state,
  mission portfolio, VText doctrine-adjacent docs, and Parallax instruction
  surface now inherit Choir Doctrine.
- Retired ontologies are marked as retired, historical evidence, transitional
  residue, or successor-scope code cleanup.
- Heresy accounting separates discovered, introduced, and repaired.
- New discoveries are epistemic progress, not regressions.
- Discovery is not counted as repair.
- Code-bearing cleanup is explicitly split into successor paradocs.

Not settled:

- runtime behavior;
- runtime prompt policy;
- BrowserApp/Web Lens implementation quarantine;
- Trace product stub removal;
- raw Terminal/Super Console code cleanup;
- continuation-level deletion;
- source-intake product implementation;
- executable heresy detector CI.

This is the right boundary. A doctrine mission that edits behavior-bearing
prompts, app registries, or runtime routing would stop being a docs-only doctrine
mission.

## Doctrine Delta

Before the upgrade, the repository had several competing root stories:

- personal writing system;
- publishing system;
- AI workspace;
- sandbox;
- workflow app;
- StoryGraph;
- Browser/Trace/Terminal app surface;
- MissionGradient checklist framing.

After the upgrade, the root story is:

> Choir is a self-improving mainframe made of persistent computers.

The product object is not a chat session, not a sandbox, and not a workflow app.
It is a persistent computer whose durable artifacts, evidence, provenance,
candidate worlds, promotion history, and rollback refs let the system learn and
improve from facts.

The doctrine now says the optimization target is:

1. truth from facts;
2. correct ontology;
3. recognition of heresies;
4. durable causality;
5. evidence-bounded claims;
6. deletion of heretical legacy control paths;
7. safe self-improvement by typed conjecture.

That hierarchy is now visible in the first-read docs instead of only implied by
mission history.

## Doctrine Of Doctrine

The most important correction is the doctrine-of-doctrine hierarchy.

First best: discover and name a real heresy from facts.

Second best: repair a named heresy by deleting, fixing, inverting, or replacing
the bad path.

Worst: preserve a clean story by hiding evidence, refusing to name the flaw, or
shipping around truth.

This matters because Choir agents were at risk of optimizing "the story looks
clean" rather than "the system has learned." A doctrine pass that finds more
heresies can make the inventory look worse while making the system smarter.

The accounting rule is now explicit:

- `discovered`: newly recognized flaws;
- `introduced`: new bad paths created by the current change;
- `repaired`: named heresies reduced or eliminated.

Discovery alone cannot claim repair progress. New discoveries are not
regressions unless the current change introduced them.

## Apex Hierarchy

The current doctrine stack is:

1. `docs/choir-doctrine.md`
2. `AGENTS.md`
3. relevant domain invariant docs
4. current mission paradoc
5. historical reports, reviews, specs, and ledgers as evidence

This resolves the old problem where "master spec," MissionGradient artifacts,
architecture reviews, README prose, and mission ledgers could all read like
parallel doctrine.

Historical documents are retained because deletion of evidence would be a
heresy. They now carry local notes or portfolio context that prevents older
terms from silently re-normalizing retired ontology.

## Parallax Upgrade

The Parallax skill was updated so broad missions run as conjecture circuits
rather than brittle checklist completion.

The upgraded doctrine-touch rule says that when agents touch doctrine,
operating contracts, mission portfolio, prompts, or high-read architecture docs,
they must reconcile framing and sentiment as well as facts. Choir Doctrine is
the apex.

The Parallax skill now also carries the H027-H029 guardrail:

- Trace means evidence and agentic tracing, not a user Trace app.
- Raw Terminal is replaced by Super Console/zot.
- Browser-as-source-gathering is replaced by Source Viewer/reader artifacts plus
  explicit Web Lens live/original inspection.

This matters because future mission agents read skills as live operating
surface. Updating only repository docs would not have fully repaired the
doctrine path.

## H027-H029 Cascade

The doctrine sweep promoted three concrete surface-ontology heresies:

### H027 - Trace App Residue

Trace is evidence, causal ledger, run bundle, and machine-readable diagnostic
substrate. It is not a normal user app direction.

Remaining code-bearing problem: a Features `Open Trace` product stub is still
truthfully recorded in platform state. It must be removed or replaced by one of:

- a trace-evidence/provenance action;
- a run-acceptance/evidence artifact link;
- a Super Console diagnosis action.

Non-goal: deleting trace evidence APIs, trace moments, run bundles, or
machine-readable evidence.

### H028 - Raw Terminal Residue

Raw Terminal is not the target user repair surface. Super Console backed by zot
is the repair direction.

Remaining code-bearing problem: any user-facing Terminal copy, registry entry,
test fixture, or default routing that presents raw Terminal as ordinary product
surface must be classified and either removed, renamed, or quarantined.

Non-goal: removing command execution capability from authorized repair,
candidate, verifier, or worker contexts.

### H029 - Browser Source-Gathering Residue

Browser-as-source-gathering is retired. The target source path is:

```text
source input
-> source artifact / reader artifact
-> VText transclusion
-> Source Viewer / owning app
-> explicit Web Lens live/original inspection when needed
```

Remaining code-bearing problem: `BrowserApp`, `browser_sessions`, stale browser
prompt routing, browser-named tests, and browser-shaped source surfaces still
exist as implementation residue. Some may need compatibility quarantine before
rename or deletion.

Non-goal: breaking Web Lens, source opening, iframe fallback, publication reader
snapshots, or media-specific opening.

## Source Intake Expansion

The last clarification expanded H029 from a narrow Conductor prompt cleanup into
a real product wedge: source intake into VText.

The newly created paradoc is
`docs/mission-conductor-url-source-routing-h029-v0.md`, now titled
"VText Source Intake And Conductor URL Routing H029."

The owner expectation is:

- paste a text URL, get durable source text transcluded into VText;
- paste a YouTube URL, get transcript segments as a source artifact with
  timestamp selectors, transcluded into VText;
- write review or commentary against exact source segments;
- let researcher agents process the transcript and the owner's commentary;
- later support podcasts by transcript discovery first, speech-to-text fallback
  second;
- support PDF and EPUB links/uploads by extracting text into source artifacts
  before VText transclusion while preserving full PDF/EPUB app opening.

This is not implemented yet. It is now documented as a product wedge and H029
successor, with YouTube transcript transclusion as the first concrete slice.

The mission explicitly defers:

- text-to-speech;
- podcast product redesign;
- vector index service;
- broad ingested-news/private-corpus search;
- complete PDF annotation UX.

## Files Changed By The Main Doctrine Sweep

Commit `79e98525` changed these files:

- `AGENTS.md`
- `README.md`
- `docs/README.md`
- `docs/choir-architecture-review-next-moves-2026-06-11.md`
- `docs/choir-deck-treatment-and-faq-2026-06-09.md`
- `docs/choir-doctrine.md`
- `docs/choir-master-spec-review-2026-06-13.md`
- `docs/choir-rearchitecture-durable-actors-2026-06-11.md`
- `docs/current-architecture.md`
- `docs/heresy-detectors.md`
- `docs/mission-agentic-debugging-vtext-stability-v0.md`
- `docs/mission-apps-and-changes-store-sweep-v0.md`
- `docs/mission-campaign-compiler-selfdev-v0.md`
- `docs/mission-choir-doctrine-upgrade-v0.md`
- `docs/mission-choir-grand-deformation-v0.md`
- `docs/mission-choir-in-choir-platform-pr-accelerator-v0.md`
- `docs/mission-doctrine-conformance-findings-2026-06-13.md`
- `docs/mission-global-wire-style-vtext-collaborative-storygraph-v0.md`
- `docs/mission-lifecycle-cutover-v0.ledger.md`
- `docs/mission-lifecycle-cutover-v0.md`
- `docs/mission-messaging-cutover-v0.ledger.md`
- `docs/mission-portfolio-2026-06-11.md`
- `docs/mission-source-system-simplify-secure-smart-v0.md`
- `docs/mission-surface-ontology-cleanup-h027-h029-v0.md`
- `docs/mission-vtext-client-ready-source-transclusion-pretext-v0.md`
- `docs/mission-vtext-source-viewer-reader-mode-hardening-v0.md`
- `docs/mission-web-surface-rationalization-v0.md`
- `docs/news-econ-publishing-synthesis-2026-06-04.md`
- `docs/overnight-vtext-super-console-zot-mega-report-2026-05-31.md`
- `docs/platform-os-app-state.md`
- `docs/project-goals.md`
- `docs/runtime-invariants.md`
- `docs/vtext-mission-current-system-hard-review-2026-06-06.md`
- `docs/vtext-regression-review-2026-05-31.md`
- `skills/parallax/SKILL.md`

The follow-up source-intake checkpoint, commit `5f2ecec7`, changed:

- `docs/mission-conductor-url-source-routing-h029-v0.md`
- `docs/mission-conductor-url-source-routing-h029-v0.ledger.md`
- `docs/mission-doctrine-conformance-findings-2026-06-13.md`
- `docs/mission-portfolio-2026-06-11.md`

## Validation Performed

The doctrine sweep ran grep/doc validation. The final sweep state recorded:

```text
rg --files -g '*.md' | wc -l => 186
post-edit doctrine term scan => 35 hit files / 151 no-hit files
```

The hit-file count was not intended to reach zero. Many hits are valid because
the documents preserve historical evidence, detector vocabulary, or explicitly
named successor work.

`git diff --check` passed before the source-intake checkpoint commit and before
this report generation.

No behavior tests were run for the docs-only doctrine sweep. That is correct:
no runtime behavior code was changed. Future prompt/default or app-surface
cleanup must run focused behavior tests and, if pushed as behavior change,
follow the staging landing loop.

## Heresy Accounting

### Discovered

The doctrine upgrade promoted these live issues into explicit inventory:

- H027 Trace app residue;
- H028 raw Terminal app residue;
- H029 Browser-as-source-gathering residue;
- stale Conductor prompt/default route that still says bare web URLs should
  open `browser`;
- runtime prompts using `.md`, which makes behavior-bearing policy easy to
  confuse with ordinary docs;
- source-intake product gap for YouTube transcripts, PDF/EPUB extraction,
  uploads, podcasts, and researcher access to ingested private sources;
- detector/process gap: heresy scans are not yet executable CI with typed
  allowlists;
- continuation-level transitional residue;
- old architecture residue around run-tree/continuation/parent-child control
  that still awaits M3/M4 deletion work.

### Introduced

No intentional heresy was introduced by the doctrine sweep or the source-intake
paramission checkpoint.

### Repaired

At the docs/instruction layer:

- Choir Doctrine is now apex doctrine.
- First-read docs inherit the self-improving-mainframe frame.
- Historical docs now mark stale ontology as historical evidence or successor
  residue.
- H027-H029 are named and carried into successor missions.
- Heresy accounting and settlement semantics are explicit.
- Parallax now carries doctrine-touch and H027-H029 guardrails.
- Mission portfolio now treats H027-H029 cleanup and source intake as successor
  work without claiming architecture-spine descent.

Not repaired:

- product code that still exposes retired surfaces;
- runtime prompt behavior;
- detector CI;
- continuation-level deletion;
- source-intake implementation.

## Open Questions

These are real open questions, but none blocks the comprehensive report.

1. Should runtime prompts be renamed from `.md` to `.prompt` or `.prompt.md`?

   Current answer: probably, but only through a code-bearing loader/test
   migration. The report should not claim that markdown prompts are harmless.
   They are behavior policy.

2. What is the smallest H029 behavior slice?

   Options:

   - fix Conductor prompt/default routing first, with focused tests only;
   - fix Conductor routing and YouTube transcript transclusion together if the
     existing source substrate can carry it cleanly.

   Conservative answer: inspect tests/loaders/source APIs first, then choose.

3. Which YouTube transcript adapter should Choir use?

   Unknown. The first product slice needs evidence on available transcript APIs,
   auth/rate policy, language support, timestamp fidelity, storage policy, and
   failure modes.

4. What speech-to-text system should replace the placeholder "Whisper"?

   Unknown. Podcast support needs a research mission that compares provider,
   cost, latency, privacy, diarization, timestamp alignment, and deployment
   policy.

5. How should researchers query ingested private/news/source corpora?

   Deferred. The source-intake wedge should not smuggle in the vector index or
   broad corpus search mission. It should preserve source artifacts and selectors
   so later search has the right substrate.

6. How much BrowserApp/browser_sessions naming is compatibility residue versus
   rename/delete surface?

   Unknown until the H029 code mission classifies app registry, store types,
   frontend components, tests, and schema/table names.

7. What is the exact replacement for the Features `Open Trace` stub?

   Candidates are trace evidence/provenance, run acceptance artifact, or Super
   Console diagnosis. The H027 code mission should decide from product evidence.

8. When should source-intake product work run relative to M3/M4?

   Portfolio answer: source intake is a side product wedge and future product
   falsifier. It should not consume architecture-spine attention unless it
   removes a concrete H029 blocker or becomes the chosen falsifier after durable
   actors stabilize.

## Successor Mission Inventory

### 1. M3.1 VText/Prompt/Tool Forcing Recovery

Purpose: continue the code-heavy recovery under upgraded doctrine. This is part
of the architecture spine.

Why next: the doctrine upgrade was originally preflight for returning to M3.1
without agents optimizing stale workflow/control ontology.

### 2. H027 Trace Stub Removal

Purpose: remove or replace product-facing Trace app residue.

Candidate files to audit:

- `frontend/src/lib/FeaturesApp.svelte`
- `frontend/src/lib/Desktop.svelte`
- Features tests
- copy that presents "Open Trace" or "Trace UI" as a user action

Settlement: product surface no longer presents Trace as a normal user app, while
trace evidence remains intact.

### 3. H028 Terminal/Super Console Cleanup

Purpose: quarantine or remove raw Terminal product residue and route repair
language toward Super Console/zot.

Settlement: raw Terminal is not taught as an ordinary app; command execution
survives only in authorized repair/candidate/verifier contexts.

### 4. H029 BrowserApp/Web Lens Quarantine

Purpose: classify and quarantine BrowserApp/browser_sessions implementation
residue.

Candidate files/surfaces:

- `frontend/src/lib/BrowserApp.svelte`
- `frontend/src/lib/apps/registry.ts`
- browser-named Playwright specs
- `internal/store/browser.go`
- `internal/types/browser.go`
- `browser_sessions` schema/table names
- routing/copy/tests that say Browser is the source workflow

Settlement: user-facing doctrine is Source Viewer/reader artifacts first, Web
Lens explicit live/original inspection second, with compatibility names either
hidden or renamed.

### 5. VText Source Intake And Conductor URL Routing H029

Purpose: repair stale Conductor prompt routing and build the first useful source
intake slice.

First product proof: a real YouTube URL becomes a durable transcript source
artifact with timestamp selectors and VText transclusion.

Path:

```text
YouTube URL
-> transcript acquisition
-> ContentItem/source item with segment selectors
-> VText source entity
-> inline transclusion
-> researcher-readable evidence
-> media/source expansion
```

### 6. Heresy Detectors CI

Purpose: convert detector vocabulary into executable checks with typed
allowlists and fail-on-unaccepted-increase semantics.

Settlement: new unaccepted introduced heresies fail, accepted discoveries are
recorded, historical evidence is allowlisted, and the system avoids fake clean
stories.

### 7. M4 Continuation Deletion

Purpose: finish removal/re-pointing so continuation-level language can leave
current acceptance doctrine.

Settlement: old continuation control paths are gone or formally superseded by
trajectory/work-item settlement evidence.

## Readiness To Resume Coding

The doctrine work is ready enough to resume coding if the next coding mission
starts from one of the successor paradocs and keeps the problem-documentation
first invariant.

The highest-value next code-bearing mission depends on what owner attention is
optimizing:

- for architecture spine: resume M3.1, then M4;
- for doctrine/code cleanup: H027/H028/H029 surface cleanup;
- for personal product value: VText source intake, starting with YouTube
  transcript transclusion;
- for process hardening: heresy detector CI.

The doctrine upgrade itself should not keep expanding unless new evidence shows
that a first-read doc still teaches a retired ontology as current doctrine.

## Final Report Verdict

Yes, the sprawling doctrine upgrade is complete at the layer it was allowed to
touch.

No, Choir is not free of doctrine heresies. The repaired part is the shared
learning surface: agents now have the right root doctrine, hierarchy, heresy
accounting, and successor missions.

The next work should be code-bearing and narrow. It should not reopen the whole
doctrine corpus unless a concrete contradiction is found. The upgrade succeeded
because it stopped hiding contradictions, not because it made the system look
clean.
