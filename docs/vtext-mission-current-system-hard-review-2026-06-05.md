# VText Mission / Current System Hard Review

Date: 2026-06-05

Reviewed scope:

- Mission document: `docs/mission-vtext-client-ready-source-transclusion-pretext-v0.md`
- Current landed slice: `40fb36b7..4c107fef`
- Primary code paths: `frontend/src/lib/VTextEditor.svelte`,
  `frontend/tests/vtext-markdown-lineage.spec.js`
- Deployed proof target: `https://choir.news`, Node B commit
  `ed1835ff4a3b5dafd448b68d2596b35303903f84`

## Findings

### P1: VText editor source workflow has outgrown `VTextEditor.svelte`

`frontend/src/lib/VTextEditor.svelte` now owns document rendering, revision
selection, compare/merge, publishing, source diagnosis, source repair, source
artifact attachment, Pretext source flow, stream handling, and edit evidence.
The new source-review code is correct enough to ship, but it adds another
state cluster and another form/payload builder to an already overloaded
component.

Evidence:

- Source-review payload construction lives inline at
  `frontend/src/lib/VTextEditor.svelte:257`.
- Source-review submit logic lives inline at
  `frontend/src/lib/VTextEditor.svelte:1672`.
- Source-review, source-artifact, and diagnostic JSON panels are all rendered
  in one template block beginning at `frontend/src/lib/VTextEditor.svelte:2480`.

Risk:

- Future source work will keep adding local state, reactive resets, and
  duplicated error/status handling.
- The component makes it hard to reason about whether source repair, source
  attachment, diagnosis, and Pretext rendering share invariants or merely share
  a file.

Recommended simplification:

- Extract a `VTextSourcePanel.svelte` component that receives `currentDoc`,
  `currentRevision`, `sourceEntities`, `sourceCandidates`, `editEvidence`, and
  action callbacks.
- Extract source-review payload construction into a small pure helper module
  with unit tests.

### P1: Diagnosis remains a known weak path

This checkpoint correctly stops diagnosis from blocking source review, but the
explicit Diagnosis button remains under-reviewed. The mission observed a panel
that stayed in `Loading...` long enough to block a test. The root cause was
partly stale local windows/streams, but diagnosis still scans revisions/runs and
should have bounded behavior, cancellation, and clear UI failure modes.

Evidence:

- Diagnosis starts at `frontend/src/lib/VTextEditor.svelte:1649` and calls
  `getVTextDiagnosis(currentDoc.doc_id, 80)`.
- The mission doc records the observed loading hang and later stream-pressure
  evidence.

Risk:

- A user can still click Diagnosis and wait without knowing whether work is
  happening, blocked, or stale.
- Future agents may mistake the explicit Diagnosis button for a required source
  repair precondition again.

Recommended simplification:

- Add timeout/cancellation semantics to the diagnosis client path.
- Label diagnosis as debug/evidence refresh, not source review.
- Add a focused deployed test for diagnosis response shape and bounded latency.

### P1: Per-window VText streams can starve local browser proof

The source-review test failure exposed a real current-system pressure point:
the desktop can retain many VText windows, and each loaded VText document opens
an EventSource stream. Local HTTP/1.1-like environments can then queue ordinary
API requests behind persistent streams.

Evidence:

- `openDocumentStream()` creates one `EventSource` per loaded document in
  `frontend/src/lib/vtext.js:405`.
- `VTextEditor.svelte` closes a stream on component destroy, but persistent
  desktop windows keep components alive.
- The E2E harness now closes stale VText/source-reader windows before source
  review proof in `frontend/tests/vtext-markdown-lineage.spec.js:24`.

Risk:

- Long-lived owner desktops can accumulate streams, windows, and source readers.
- Local/staging QA can fail for connection-pool reasons rather than product
  behavior, which wastes debugging effort.

Recommended simplification:

- Multiplex VText document updates through one desktop-level stream or pause
  streams for background/minimized windows.
- Add a lightweight stream budget metric to `/health` or trace evidence.
- Keep test cleanup, but treat it as harness hygiene, not the product fix.

### P2: Typed source review still creates source entities from free text only

The new owner form is a major improvement over raw JSON, but it is still a
manual title/excerpt/URL adapter. It does not yet research, confirm, refute, or
omit a source for a claim.

Evidence:

- `sourceReviewPayload()` constructs one source entity directly from marker,
  title, URL, and excerpt fields at `frontend/src/lib/VTextEditor.svelte:286`.
- Provenance is fixed as `created_by: "source_review_panel"` and
  `rights_scope: "public_source"`.

Risk:

- The owner can paste low-quality or untrusted evidence and the UI will mark
  the source as `confirmed`.
- This is acceptable for the current repair workflow, but not enough for the
  broader "research and find confirming/refuting sources" requirement.

Recommended simplification:

- Separate manual source review from researched source acquisition in the UI
  and metadata.
- Use a distinct evidence state for owner-entered excerpts until a researcher
  or source import verifies them.

### P2: Diagnostic JSON repair remains in the owner panel

Raw repair JSON has been demoted to a `Diagnostic JSON repair` disclosure, but
it is still present in the same owner panel. That is useful for operator
recovery, yet it is still close enough to the primary workflow that future UI
changes may expose it again.

Evidence:

- The diagnostic disclosure starts at
  `frontend/src/lib/VTextEditor.svelte:2707`.
- The tests assert the raw textarea is not visible during normal source review
  at `frontend/tests/vtext-markdown-lineage.spec.js:519`.

Risk:

- Owner-facing product surfaces can drift back into operator/debug controls.

Recommended simplification:

- Gate diagnostic JSON behind a developer/operator mode flag or move it into a
  separate diagnostics panel.

### P2: Deployed owner proof is split across Comet and Playwright

Computer Use/Comet proved the deployed legal-cloud proposal and source windows.
The exact typed source-review form was verified on staging with authenticated
Playwright because the legal-cloud document no longer had unresolved markers.

Evidence:

- Comet showed the deployed published proposal, source markers, Pretext journal
  note, source reader windows, and `Edit my version` private VText.
- Playwright staging proof passed:
  `frontend/tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies source-gap repair"`.

Risk:

- The owner-account proof is adequate for deployed source UX, but not a single
  end-to-end owner legal-cloud source-gap repair because the owner document is
  already source-resolved.

Recommended simplification:

- Add a durable owner-safe source-gap fixture or a reversible candidate/private
  copy path for Comet QA.

## What Is Working

- Markdown imports advance to canonical `.vtext` at first VText revision.
- Markdown export remains a projection of canonical VText.
- The legal-cloud proposal has full-length equivalent content, source markers,
  source windows, and a preserved Markdown glossary table.
- Pretext is used for magazine/journal source flow, not merely card styling.
- Published source metadata and reader-mode source windows are available to
  authorized publication readers.
- The typed source-review panel creates canonical source-repair revisions
  without asking the owner to write JSON.
- Local and deployed tests prove source refs expand, source notes open source
  windows, and structured edit metadata remains visible without raw prompts.

## Evidence Ledger

- Local: `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-markdown-lineage.spec.js --project=chromium
  --timeout=120000` passed 6/6.
- Local: `pnpm --dir frontend build` passed.
- CI: GitHub Actions run `27045898828` passed, including Node B deploy.
- FlakeHub: run `27045898824` passed.
- Staging health: proxy and sandbox deployed commit
  `ed1835ff4a3b5dafd448b68d2596b35303903f84` at
  `2026-06-05T23:43:32Z`.
- Deployed source-review backup: staging Playwright source-gap repair test
  passed in 14.2s.
- Comet: deployed legal-cloud publication opened, source marker expansion and
  source reader windows observed, and private editable VText copy created.

## Simplification Backlog

1. Extract source panel UI and actions from `VTextEditor.svelte`.
2. Extract source-review payload construction into a pure helper with tests.
3. Bound or cancel source diagnosis requests.
4. Move diagnostic JSON repair behind operator/developer affordance.
5. Add stream budgeting or multiplexing for VText document updates.
6. Add a Comet-friendly reversible source-gap fixture for deployed owner QA.
7. Separate manual source review metadata from researched/confirmed evidence.
8. Continue the source-reader cleanup axis: cleaned Markdown reader mode first,
   iframe/web preview only as fallback.

## Residual Risk

The system is now materially better, but the source subsystem is still an
evolved set of working paths rather than a small architecture. The next code
pass should reduce surface area before adding source research automation.
