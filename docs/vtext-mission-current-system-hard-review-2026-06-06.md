# VText Mission / Current System Hard Review

Date: 2026-06-06

Reviewed scope:

- Mission document: `docs/mission-vtext-client-ready-source-transclusion-pretext-v0.md`
- Current deployed behavior commit: `e2603c1c0a7d8eef0dff82787fa3d95b1ab4197a`
- Latest pushed docs evidence commit: `85a98d01`
- Primary source UI/code paths:
  - `frontend/src/lib/VTextEditor.svelte`
  - `frontend/src/lib/vtext-source-flow.ts`
  - `frontend/src/lib/vtext-source-renderer.ts`
  - `frontend/src/lib/BrowserApp.svelte`
  - `internal/proxy/platform_publish.go`
- Doctrine note (2026-06-13): `BrowserApp` is quoted here as the then-current
  implementation name. Current source doctrine is Source Viewer/reader
  artifacts first, with explicit Web Lens inspection only when the original page
  itself must be inspected.
- Deployed proof target: `https://choir.news`
- Owner artifact: `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`

This is a hard review of the whole mission and current system state. It is not
a completion claim. The system has materially improved, but the mission remains
active because several explicit requirements are still partial or unproven.

## Findings

### P1: Source work is still centralized in `VTextEditor.svelte`

`VTextEditor.svelte` is still 4,156 lines and owns too many source-related
responsibilities: source review, source diagnosis, source artifact import and
attachment, source-window dispatch, source-flow mounting, edit evidence, and
the source panel template.

Evidence:

- Source-review payload callsite: `frontend/src/lib/VTextEditor.svelte:276`.
- Source artifact payload construction: `frontend/src/lib/VTextEditor.svelte:307`.
- Diagnosis control and timeout path: `frontend/src/lib/VTextEditor.svelte:1585`.
- Source review submit path: `frontend/src/lib/VTextEditor.svelte:1666`.
- Source window dispatch: `frontend/src/lib/VTextEditor.svelte:2023`.
- Pretext source-flow interaction handling: `frontend/src/lib/VTextEditor.svelte:2058`.
- Source panel template: `frontend/src/lib/VTextEditor.svelte:2475`.

Risk:

- Source repair, source import, diagnosis, and source display share a large
  mutable component rather than a small explicit boundary.
- The next feature, researched source acquisition, will be tempted to add more
  local state and more UI branching to the same component.

Recommended simplification:

- Extract `VTextSourcePanel.svelte` for source review, artifact attachment, and
  diagnosis controls.
- Keep source-flow rendering separate from source-review form state.
- Move source open dispatch into a small source-surface adapter so VText can
  pass a source entity and not know every window/app detail.

### P1: Manual source review still marks owner-entered evidence as confirmed

The owner source-review path is useful for repairing citation gaps, but it is
not yet the researched-source system the mission asks for. It builds a source
entity from form fields and sets `evidence.research_state` to `confirmed`.

Evidence:

- `buildSourceReviewPayload()` sets `research_state: 'confirmed'` in
  `frontend/src/lib/vtext-source-review.js:63`.
- The same helper sets `created_by: 'source_review_panel'` and
  `rights_scope: 'public_source'` at
  `frontend/src/lib/vtext-source-review.js:67`.
- The UI labels the excerpt field as confirming evidence at
  `frontend/src/lib/VTextEditor.svelte:2603`.

Risk:

- A pasted owner excerpt is not the same as a researched, verified source.
- The system can overstate evidence quality and create future confusion between
  manual citation repair and source research.

Recommended simplification:

- Use a distinct metadata state for owner-entered/manual source review, such as
  `research_state: owner_supplied` or `manual_reviewed`.
- Reserve `confirmed` for source-service/researcher-verified evidence.
- Make the UI language support confirming, refuting, or intentionally omitting
  a source instead of implying every gap is a confirmation task.

### P1: Source windows still feel like app chrome before source content

Article-side source notes are now closer to a journal/marginal note, but opened
source windows still expose operational chrome and metadata too early. In Comet,
the ABA Formal Opinion source window showed `FILES CONTENT`, source text,
then `Source evidence`, `Source entity`, and `Provenance` accordions. That is
acceptable for inspection, but it is not yet the quiet reader-mode source
experience requested for client-facing reading.

Evidence:

- Comet owner proof on 2026-06-06 opened the legal-cloud source window after
  expanding the first citation.
- `BrowserApp.svelte` has a good source-snapshot path, but the source window
  still includes Web Lens/page controls and snapshot chrome around the reader
  surface: `frontend/src/lib/BrowserApp.svelte:568` and
  `frontend/src/lib/BrowserApp.svelte:693`.
- `renderSourceTransclusionBody()` still carries facts/metadata for source
  bodies outside the simplified journal note path:
  `frontend/src/lib/vtext-source-renderer.ts:251`.

Risk:

- A client sees the product apparatus instead of a reader-first source.
- The mission could regress into "source cards and metadata" instead of
  content-forward source evidence.

Recommended simplification:

- Source windows should default to a cleaned reader article view with minimal
  source title, publication/source URL, and an unobtrusive details section.
- Keep provenance/metadata accordions, but secondary and collapsed.
- Treat iframe/page preview as fallback, not the default proof of source access.

### P1: Publication source snapshots are heuristic and need quality state

Publication now preserves source snapshots for public/authorized sources, and
the noisy HTML reader cleanup substantially improved real URL imports. But the
extractor is heuristic and the current metadata can still make a snapshot look
"ready" without expressing quality or confidence.

Evidence:

- Publication metadata enrichment is in
  `internal/proxy/platform_publish.go:178`.
- Public-safe source snapshots are attached at
  `internal/proxy/platform_publish.go:218`.
- Public rights checks are implemented at
  `internal/proxy/platform_publish.go:317`.
- Deployed proof for the Hetzner datacenter source showed an 8,239 character
  cleaned snapshot with no cookie/location/login noise.

Risk:

- A source can be publication-safe but still low-quality as reader evidence.
- Future arbitrary web sources may need better classification: full reader
  snapshot, bounded excerpt, curated source artifact, import failed, or
  low-confidence cleanup.

Recommended simplification:

- Add an explicit reader quality/status field separate from access rights.
- Surface source-kind language in the source window: full reader snapshot,
  curated source artifact, bounded excerpt, or unavailable.
- Add more real-world fixtures before broadening source acquisition.

### P2: Pretext flow is now real, but still depends on synthetic DOM projection

The article/source flow now uses the right Pretext primitives and no longer
clones the old source card into the journal note. It still reconstructs article
lines as an absolute-positioned noncanonical DOM projection and hides original
paragraphs.

Evidence:

- Pretext imports: `frontend/src/lib/vtext-source-flow.ts:1`.
- Rich inline routing: `frontend/src/lib/vtext-source-flow.ts:124`.
- Plain text routing: `frontend/src/lib/vtext-source-flow.ts:150`.
- Journal note construction: `frontend/src/lib/vtext-source-flow.ts:258`.
- Original paragraph hiding: `frontend/src/lib/vtext-source-flow.ts:419`.
- Deployed Playwright asserted routed source lines and nested citation behavior.
- Comet owner proof showed the legal-cloud article text flowing beside the
  source note.

Risk:

- Any future editable in-flow interaction can break if code treats the
  projection as canonical document structure.
- The collapsed hover popover remains as a fallback path and should be pruned or
  reduced once accessibility behavior is preserved.

Recommended simplification:

- Keep the projection boundary explicit: Pretext output is display-only.
- Add small unit coverage around the projection/hiding lifecycle.
- Decide whether hover popovers are still needed now that expanded source flow
  is the primary interaction.

### P2: The source panel still contains operator/debug controls

The `Diagnostic JSON repair` disclosure is less prominent than before, but it
still lives in the owner-facing source panel alongside source review and source
artifact workflows.

Evidence:

- Diagnostic JSON repair template starts at
  `frontend/src/lib/VTextEditor.svelte:2702`.

Risk:

- Debug controls can drift back into primary product UX.
- Owner workflows and operator recovery stay coupled in tests and component
  state.

Recommended simplification:

- Move diagnostic JSON repair behind a developer/operator flag or a separate
  diagnostics surface.
- Keep typed source review as the normal owner path.

### P2: Deployed owner proof still uses mixed proof modes

Computer Use/Comet is available and has proven the deployed legal-cloud reader
and source windows. Some workflows remain proven by Playwright/API backup
because the live owner publication no longer has unresolved source markers and
because reversible owner-account mutation is still awkward.

Evidence:

- Deployed Comet proof covered the real legal-cloud publication, Pretext source
  note, and source window.
- Deployed Playwright covered source-flow geometry and guest/public source
  snapshot behavior.
- Earlier source-gap repair proof used staging Playwright on a fixture.

Risk:

- The mission asks for owner-account proof on the actual owner document. We have
  strong proof for reading and source opening, but not every mutation axis is
  proven directly through Comet on that owner document.

Recommended simplification:

- Add a reversible owner-safe candidate/private-copy path for Comet QA.
- Keep owner document proof for read/source/publish behavior; use candidate
  copies for mutation proof that should not alter the client-facing artifact.

## What Is Working

- The legal-cloud publication resolves on staging and contains seven source
  entities and seven transclusions.
- Markdown export is a canonical publication export, not rendered DOM scraping.
- Deployed Markdown export for the legal-cloud route is 38,398 characters,
  preserves `source:` markers, preserves the glossary table, and contains no
  `missing source` prose.
- The legal-cloud proposal is now long-form and equivalent in shape to the
  legacy Markdown proposal, not a short demo article.
- The published article uses source markers as transclusion points.
- Expanded article-side sources use Pretext line routing for magazine/journal
  flow.
- Source notes no longer clone the old popover/card DOM into the journal note.
- Public/guest readers can open publication-carried source snapshots where
  policy allows.
- Noisy URL reader cleanup is deployed and proven on a real Hetzner source.
- `choir_private_legal_cloud_proposal.md` is presented and published as
  `choir_private_legal_cloud_proposal.vtext`; Markdown export is a projection.
- The owner account proof path through Comet is available.

## Evidence Ledger

- Current deployed behavior commit:
  `e2603c1c0a7d8eef0dff82787fa3d95b1ab4197a`.
- Latest docs evidence commit: `85a98d01`.
- CI: GitHub Actions run `27047989903` passed, including frontend build, Go
  tests, runtime shards, integration smoke, and Node B deploy.
- FlakeHub: run `27047989896` passed.
- Staging health: proxy and sandbox report
  `e2603c1c0a7d8eef0dff82787fa3d95b1ab4197a`, deployed at
  `2026-06-06T00:58:46Z`.
- Deployed source-flow test:
  `BASE_URL=https://choir.news ... pnpm --dir frontend exec playwright test
  frontend/tests/vtext-source-entities.spec.js -g "VText lays out expanded text
  sources as noncanonical journal flow" --project=chromium --timeout=120000`
  passed.
- Publication resolve:
  `/api/platform/publications/resolve?route=/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`
  returned seven source entities and seven transclusions.
- Publication export:
  `/api/platform/publications/export?route=/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6&format=md`
  returned `choir-private-legal-cloud-proposal-vtext-pub270a62fb6.md` with
  38,398 content characters and content hash
  `4e6f3f9888c7ed41fe2b386620445985290285001bd0d3c16dfb02ad600f81bc`.
- Comet owner proof: the legal-cloud publication loaded under the owner account,
  the first citation expanded to a simplified Pretext journal note, article text
  routed beside it, and `Open source` opened the ABA Formal Opinion source
  window.

## Simplification Backlog

1. Extract `VTextSourcePanel.svelte`.
2. Extract a source-surface launcher adapter from `VTextEditor.svelte`.
3. Split manual source review from researched/verified source acquisition.
4. Replace owner-facing `confirmed` metadata for pasted excerpts with a manual
   review state.
5. Move diagnostic JSON repair out of the owner source panel.
6. Make source windows reader-first and metadata-secondary.
7. Add reader snapshot quality/status beyond access/policy status.
8. Add more real noisy-source fixtures to reader cleanup.
9. Decide whether collapsed hover popovers are still necessary.
10. Add a reversible Comet-friendly owner/candidate mutation proof path.

## Completion Audit

Not complete yet.

- Client-ready article: partially true. The article is long-form, source-backed,
  and published, but source windows and source-kind language still need polish.
- Legacy Markdown to `.vtext`: partially proven. The owner-facing artifact is
  `.vtext` and Markdown export works as a projection, but a fresh v0-to-v1
  migration path for arbitrary imported Markdown/text still deserves a direct
  current-state proof before this mission can close.
- Source research: incomplete. Manual source repair exists; researched
  confirm/refute/omit workflow is not yet implemented as a first-class path.
- Publication source access: materially working for public snapshots, but needs
  quality/status hardening.
- Pretext wrapping: deployed and proven for the article-side source note.
- Hard review/PDF: this report is the current hard review and should be
  regenerated to PDF after commit.
- Simplification pass: started, but not complete. The largest remaining target
  is source workflow extraction from `VTextEditor.svelte`.

## Residual Risk

The system has crossed from demo into usable staging behavior for the
legal-cloud proposal, but source acquisition and source workflow architecture
are still evolved paths. The next code pass should reduce component surface area
and make source-quality states explicit before adding broader source research
automation.
