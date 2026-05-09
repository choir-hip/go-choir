# VText Next Planning Checklist — 2026-05-09

This is the pre-push planning checkpoint after the mobile VText UX patch
(`f08b77a Improve mobile vtext editing UX`).

## Current State

- [x] VText mobile window sizing is substantially better.
- [x] The VText document surface renders Markdown and is directly editable.
- [x] Read/Edit mode split was removed.
- [x] Prompt bar is a growing textarea.
- [x] Local build passes.
- [x] Focused local Playwright checks pass against a stub runtime:
  - `VText recent landing can open a Markdown document without control overlap`
  - `VText opens near full mobile workspace and clears the prompt bar`
  - `bottom bar prompt input with placeholder`

## Push And Staging Verification Gate

After pushing, verify the deployed commit before doing more feature work.

- [ ] Push `main`.
- [ ] Wait for GitHub Actions and Node B deployment.
- [ ] Verify deployed build identity matches `HEAD`.
- [ ] Run deployed origin auth/cache smoke tests.
- [ ] Run deployed prompt-bar to VText Playwright coverage.
- [ ] Run deployed live-search VText coverage when provider/search capacity is available.
- [ ] Manually check mobile Safari for:
  - no viewport zoom on prompt focus
  - keyboard slides the desktop rather than zooming the page
  - prompt bar grows for multiline input
  - VText toolbar stays one row and fades while scrolling
  - VText document remains editable while rendered

## Do Not Mix These Scopes

Keep these as separate commits/goals. Mixing them is how the system keeps
getting hard to reason about.

- [ ] Mobile/editor UX polish.
- [ ] Staging/deploy verification.
- [ ] VM/background-work architecture.
- [ ] Server-side Browser app / Obscura integration.
- [ ] Coding benchmark/eval suite.
- [ ] Extraction pipeline.
- [ ] Citations/publication.
- [ ] Pretext editor rewrite.

## Recommended Next Sequence

1. Ship and verify the current UX patch on staging.
2. Fix any deployed regressions found by Playwright or mobile manual QA.
3. Prototype the server-side Browser app with Obscura if it can be kept
   narrowly scoped:
   backend-controlled browsing, extraction, and future background-VM preview
   surfaces; not a broad rewrite of QA or desktop.
4. Return to `vmctl` motion before serious coding-agent benchmarks:
   background VM fork, worker execution, merge/promotion/rollback.
5. Add VText coding benchmark tasks only after mutable work can happen outside
   the live desktop VM.
6. Port extraction work after the VText product loop is stable enough to show
   retrieved content as first-class document evidence.
7. Add citation/publication flows after extraction produces durable source
   objects worth citing.
8. Replace the stopgap contenteditable Markdown surface with a Pretext-backed
   editor when document layout/transclusion/citation demands require it.

## Server-Side Browser / Obscura Track

Motivation:

- The current Browser app is iframe-like client-side browsing, which runs into
  frame blockers and is the wrong substrate for agentic browsing.
- Background VMs need visible previews. A background VM should be openable as a
  Browser app page, then later promoted to the active desktop with an explicit
  transition.
- Browser-derived content should feed extraction and transclusion, not just
  display pixels.

Obscura is a candidate substrate because it is a Rust headless browser engine
with V8, Chrome DevTools Protocol compatibility, Puppeteer/Playwright
connection support, and built-in DOM-to-Markdown (`LP.getMarkdown`) according
to its upstream README.

Initial integration shape:

- [ ] Add an internal browser service or sandbox-side browser controller that
  starts Obscura as a managed process.
- [ ] Expose product endpoints for browser sessions, navigation, screenshots or
  DOM snapshots, and extracted text/Markdown.
- [ ] Keep the browser app server-side. The frontend should render a controlled
  view and send navigation/input intents; it should not iframe arbitrary sites.
- [ ] Record browser navigation/extraction events into Trace when an agent uses
  the browser.
- [ ] Treat Obscura as extraction/browser substrate first, not as a replacement
  for product Playwright QA.
- [ ] Add capability checks for binary availability, version, CDP health, and
  failure modes.
- [ ] Preserve provider independence: Obscura should be one browser backend
  behind an internal interface, not a global dependency that breaks the desktop
  if unavailable.

Open risks:

- [ ] Obscura is young and may not implement enough CDP surface for all sites.
- [ ] Anti-detection/stealth features need policy review before defaulting on.
- [ ] Server-side browsing has SSRF and network-access risks; URL allow/deny,
  per-user isolation, and resource limits are required.
- [ ] Screenshots/video streaming can become expensive; start with static
  snapshots and extraction before live remote-browser streaming.

## VText Coding Benchmark Shape

These benchmarks should prove the product path, not agent plumbing.

Required path:

`prompt bar -> conductor -> VText -> researcher/super -> artifact + verification -> VText revisions -> Trace audit`

Rules:

- [ ] No browser-public internal orchestration APIs.
- [ ] No test harness manually spawning researcher/super/cosuper.
- [ ] No coding in the live desktop VM once background VM motion exists.
- [ ] Trace must prove tool-backed causality:
  - researcher search/fetch events
  - super/cosuper file writes
  - verification command execution and result
  - VText consuming worker updates
  - VText creating canonical revisions by document edit
- [ ] Final artifact includes a Playwright video or equivalent product-facing
  proof, not just backend assertions.

Candidate medium-difficulty programming prompts:

- [ ] Implement a Gray-Scott reaction-diffusion simulation with deterministic
  parameters, tests, and an interactive visualization.
- [ ] Implement a small cellular automata model of biological evolution with
  explicit assumptions, deterministic seed control, and verification.
- [ ] Implement PageRank or HITS from the original paper on a small graph
  corpus, with tests against hand-computed examples.
- [ ] Implement Needleman-Wunsch or Smith-Waterman sequence alignment from the
  original algorithm description, with property tests and examples.
- [ ] Implement a Kalman filter tutorial/paper example with numeric validation.
- [ ] Implement Black-Scholes pricing and Greeks with tests against known
  formula outputs and an explanatory VText report.
- [ ] Implement Markowitz portfolio optimization on a toy dataset with
  constraints, tests, and visualization.
- [ ] Implement a small Boids or flocking simulation from source material with
  measurable invariants.

Scoring dimensions:

- [ ] Research grounding: sources are actually searched/fetched and cited in
  worker findings.
- [ ] Artifact correctness: generated files are visible in Files/Browser.
- [ ] Verification quality: tests/checks are run and fail meaningfully if the
  artifact is broken.
- [ ] VText quality: revisions are complete current-state documents, not status
  chatter.
- [ ] Trace quality: the trajectory is inspectable and unambiguous.
- [ ] Safety: mutable work happens in the right VM boundary.

## Extraction Track

Extraction should become a source-object pipeline, not just paste text into a
prompt.

- [ ] URL to cleaned text/Markdown.
- [ ] PDF extraction with page/source anchors.
- [ ] EPUB extraction with chapter/source anchors.
- [ ] YouTube transcript extraction.
- [ ] File upload ingestion for text/Markdown/PDF/EPUB.
- [ ] Display retrieved/uploaded source objects in appropriate apps.
- [ ] Make source objects transcludable into VText.
- [ ] Preserve enough provenance for later citation/publication.

## Pretext Track

The current contenteditable Markdown surface is a pragmatic stopgap.

Use Pretext when we need:

- [ ] stable text measurement/layout
- [ ] inline citation markers
- [ ] transclusion blocks
- [ ] mobile reading/editing polish beyond basic Markdown rendering
- [ ] richer document-native interactions without becoming a Google Docs clone

Do not block current VText usability on Pretext.
