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
- [ ] Coding benchmark/eval suite.
- [ ] Extraction pipeline.
- [ ] Citations/publication.
- [ ] Pretext editor rewrite.

## Recommended Next Sequence

1. Ship and verify the current UX patch on staging.
2. Fix any deployed regressions found by Playwright or mobile manual QA.
3. Return to `vmctl` motion before serious coding-agent benchmarks:
   background VM fork, worker execution, merge/promotion/rollback.
4. Add VText coding benchmark tasks only after mutable work can happen outside
   the live desktop VM.
5. Port extraction work after the VText product loop is stable enough to show
   retrieved content as first-class document evidence.
6. Add citation/publication flows after extraction produces durable source
   objects worth citing.
7. Replace the stopgap contenteditable Markdown surface with a Pretext-backed
   editor when document layout/transclusion/citation demands require it.

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

