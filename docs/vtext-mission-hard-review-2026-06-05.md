# VText Mission Hard Review - Current State - 2026-06-05

This is a hard review of the whole VText legal-cloud mission and current
system state, not a review of only the latest patch. It covers the path from
checkpoint `f05b4c92` through current `main`.

Review point in time:

- Local branch: `main`
- Local HEAD and `origin/main`: `21cb34f0`
- Latest behavior-changing deployed commit: `24cb3cd1fb98ce720bb64befe64fef28bbc56ec7`
- Staging health: `https://choir.news/health` returned `status: ok`,
  `upstream: ok`, `vmctl_status: ok`
- Staging deployed identity: proxy and sandbox upstream both reported
  `24cb3cd1fb98ce720bb64befe64fef28bbc56ec7`, deployed at
  `2026-06-05T22:36:03Z`
- Current docs-only checkpoints after deploy: `7c283c0f` and `21cb34f0`
- Primary owner/public route:
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`
- Review scope: 32 tracked files changed since `f05b4c92`, about 12.8k
  insertions and 602 deletions

## Findings

### P0 - Mission Incomplete: The Review And Simplification Loop Has Not Landed

The product path is now much closer to the desired client-ready proposal, but
the mission stopping condition is not satisfied. The requested hard review was
not current until this report, the PDF export still needs to be produced from
this report, and the simplification/dead-code pass has not been run.

Evidence:

- `docs/mission-vtext-client-ready-source-transclusion-pretext-v0.md` still
  records the hard review, iCloud PDF, and simplification pass as pending before
  this report.
- `git diff --stat f05b4c92..HEAD` shows substantial code growth across
  `VTextEditor.svelte`, source rendering, publication, source materialization,
  and VText runtime paths.

Acceptance implication: do not call the mission complete until this report is
exported to PDF, simplification lands, CI/deploy pass again, and staging proof
is refreshed after simplification.

### P1 - Source UX Still Has Too Much Product Chrome For The Magazine/Journal Target

Pretext is now used for article-side source-note wrapping, and nested source
cards were fixed. That is real progress. But the visible source apparatus still
leans toward product controls, rounded panels, buttons, and metadata labels.
The user clarified that the point of Pretext is the wrapping and magazine or
academic journal UX: prose should flow around source apparatus, while opened
sources should read as evidence, not as a dashboard.

Evidence:

- `frontend/src/lib/vtext-source-flow.ts` implements the current routed source
  note flow using `data-vtext-source-flow`.
- `frontend/src/lib/VTextEditor.svelte` still owns large CSS blocks for source
  notes, source-flow close buttons, mounted markers, source artifacts, and
  source repair controls.
- The latest Comet proof shows the source content window is reader-mode, but
  the in-article source note still needs visual reduction and typography work.

Recommended direction: keep the current Pretext flow as the behavioral
foundation, then redesign the source note as a quiet marginal/inline evidence
apparatus. Remove visual layers that communicate "card stack" rather than
"citation note".

### P1 - Source Acquisition Is Still Too Manual And Not Yet A Durable Reader Pipeline

The publication/source graph now proves that source entities, transclusions,
reader snapshots, source attachment, and opened source windows can work. But
the legal-cloud source artifacts were repaired through manual attachment and
diagnostic workflows. The durable target is source acquisition that can fetch,
clean to Markdown, store a reader artifact, and fall back from live preview
without hand-pasted source text.

Evidence:

- `frontend/src/lib/VTextEditor.svelte` exposes a `SOURCE ARTIFACT` panel with
  title, URL, text, import, and attach controls.
- `internal/runtime/browser.go` and `internal/runtime/vtext_media_sources.go`
  contain the source snapshot/materialization path, including fallback states.
- Mission evidence records iframe/Web Lens brittleness and the need to treat
  cleaned Markdown as the primary reader path, with iframe preview as optional.

Recommended direction: make "clean public source into Markdown reader artifact"
the default source pipeline. Keep iframe/live preview as a secondary tab or
fallback, not the authoritative proof of source readability.

### P1 - Publication Source Policy Needs A Focused Privacy/Licensing Review

The current public legal-cloud route exposes seven source entities and seven
transclusions, and Markdown export reports `private_material_omitted: true`.
That is encouraging. It is not enough policy proof for broad publication of
private, licensed, or subscription-derived source artifacts.

Evidence:

- Public resolve for the route returned `7` source entities and `7`
  transclusions under public access policy.
- Public Markdown export returned 38,398 bytes, source markers, and
  `private_material_omitted: true`.
- Mission contract requires users authorized to access a publication to inspect
  permitted sources, which implies source policy must distinguish public,
  owner-private, licensed, excerpt-only, and no-publish cases.

Recommended direction: add a source publication policy review before expanding
source import beyond public sources. The export path should continue to prove
what was omitted and why.

### P1 - Source Repair Is Still Exposed As Raw JSON In The Owner Surface

The backend source repair contract works, and it helped bootstrap the real
source graph. But the UI still exposes `Repair JSON` in the main VText editor.
That is an agent/operator tool, not an owner-grade source workflow.

Evidence:

- `frontend/src/lib/VTextEditor.svelte` still renders `Repair JSON`.
- `defaultSourceRepairPayload` still exists near the top of the component.
- The source artifact panel is useful but still reads like a control surface
  rather than a citation review workflow.

Recommended direction: replace raw repair JSON with typed claim/source rows:
claim, current marker, candidate source, confirming/refuting/omit decision,
bounded selector, and source-window preview. Keep raw JSON behind diagnostics
only if needed.

### P2 - `VTextEditor.svelte` Is Too Large And Owns Too Many Domains

The component is now 4,141 lines. It owns document rendering, autosave,
revision state, title/manifest behavior, publishing/export, compare/merge,
source entities, source artifacts, source repair, edit evidence, Pretext flow
integration, and large CSS blocks. This size is now a correctness risk.

Evidence:

- `frontend/src/lib/VTextEditor.svelte`: 4,141 lines.
- Source-flow integration alone touches event handling, source marker cloning,
  mounted marker state, and hundreds of CSS lines.
- New helper files exist (`vtext-source-flow.ts`, `vtext-source-renderer.ts`,
  `vtext-markdown-renderer.ts`), but the editor still centralizes too much
  state and UI.

Recommended direction: extract source panel state, source artifact actions,
repair draft construction, edit evidence extraction, and publication/export
actions into smaller modules or components before adding more UX.

### P2 - `internal/runtime/vtext.go` Is Becoming A Second Monolith

The runtime VText API file is now 5,214 lines and contains file import,
Markdown lineage import, export, diagnosis, compare/merge, source repair,
source attachment, source prompt construction, and source artifact plumbing.
Some of this belongs together, but the current file size makes review and
future change isolation harder.

Evidence:

- `internal/runtime/vtext.go`: 5,214 lines.
- Related source materialization logic also exists in
  `internal/runtime/vtext_media_sources.go`,
  `internal/runtime/browser.go`, and `internal/proxy/platform_publish.go`.

Recommended direction: split by contract boundary, not by convenience:
import/export, source graph/source attachment, diagnosis/debug, compare/merge,
and prompt/context construction.

### P2 - Legacy Write-Through Still Looks Like Dead Or Dangerous Compatibility Debt

`writeThroughToFile` remains in `VTextEditor.svelte` and is called from save
paths, although it skips `.vtext` and any current VText document. Under the
current invariant, canonical writes should go through VText revisions, and
Markdown should be an export projection.

Evidence:

- `frontend/src/lib/VTextEditor.svelte` still defines and calls
  `writeThroughToFile`.
- The current canonical import tests and deployed owner proof support `.vtext`
  convergence rather than source-file write-through.

Recommended direction: prove whether any current noncanonical editor path still
needs this function. If not, delete it. If yes, fence it explicitly as legacy
noncanonical file editing and keep it out of VText document flows.

### P2 - Structural Preservation Is Better, But Still Too Text-Recovery-Centered

The table regression was repaired without a glossary-specific hardcode, and
tests cover untouched and bounded table edits. But the underlying strategy
still includes Markdown-structure stabilization after content edits. Durable
VText should preserve table, list, citation, and hidden metadata structure as
first-class document structure through render/edit/save/revise, not recover
them from text shape after damage.

Evidence:

- Mission evidence identified v74 -> v75 as the first owner transition that
  collapsed the glossary table into `TermDefinition`.
- Current tests cover bounded table preservation and source-aware roundtrip.
- The mission still names structured VText preservation as a core invariant.

Recommended direction: keep stabilization as a guardrail, but drive future work
toward explicit VText block/inline structure and structured edit operations.

### P2 - Test Coverage Is Valuable But Too Scattered And Setup-Heavy

The mission added good regression coverage, especially around source entities,
publication source service, Markdown lineage, and desktop state. The tests now
need consolidation before the next expansion.

Evidence:

- `frontend/tests/vtext-source-entities.spec.js`: 525 lines.
- `frontend/tests/vtext-source-service-publication.spec.js`: 251 lines.
- `frontend/tests/vtext-markdown-lineage.spec.js`: 633 lines.
- Focused comprehensive runtime tests are behind `-tags comprehensive`, so a
  normal `go test ./internal/runtime -run ...` can report no tests to run.

Recommended direction: extract shared product API helpers and fixtures for
VText documents, publications, source entities, source artifacts, and desktop
state reset. Document comprehensive-tag test invocation near the tests or in
the mission report.

### P3 - Mission Docs Are Valuable But Too Large For Fast Resumption

The mission documentation discipline is a strength: newly found problems were
documented before code, and evidence was preserved. The downside is size. The
current client-ready mission doc is 4,697 lines, and the earlier fluid-editing
mission doc is 3,182 lines.

Recommended direction: keep the append-only evidence ledger, but add or update
a short "Run Checkpoint & Resumption State" section near the top with latest
artifact state, proven behavior, unproven behavior, next executable probe, and
links to this review.

## What Works Now

- Computer Use is available and was used against authenticated Comet.
- The real owner legal-cloud proposal has a canonical `.vtext` path in the
  current proof chain.
- The original Markdown owner artifact is treated as legacy/import source
  material, with Markdown export available as projection.
- Imported/opened `.md`, `.txt`, DOCX, PDF, and related artifacts converge to
  `.vtext` identity on VText revision paths in current source code.
- Focused comprehensive runtime tests pass for canonical user revision,
  appagent edit, file open, Markdown lineage import, manifest, stale revision,
  and import preservation behavior.
- The public legal-cloud route exports Markdown with compact `source:` markers
  and no `missing source` prose.
- Publication resolve exposes seven source entities and seven transclusions for
  the legal-cloud publication.
- Source cards no longer bunch at the top of the article by default.
- Clicking source markers in the article opens inline source apparatus, and
  clicking a marker inside a Pretext-routed flow remounts one active source note
  instead of nesting a second card.
- Opened source windows now render cleaned Markdown content as reader-mode
  source artifacts with evidence/provenance demoted to collapsed details.
- The appendix table regression was root-caused in the earlier mission to the
  v74 -> v75 transition, and current table-focused tests protect the repair
  path.
- CI and Node B deploy succeeded for the latest behavior-changing commit
  `24cb3cd1`.

## What Is Not Proven Yet

- The post-report simplification pass has not happened.
- Final magazine/academic journal source-note design is not complete.
- Automated source acquisition/cleanup into Markdown reader artifacts is not
  complete.
- Source publication policy for private/licensed/subscription material needs
  dedicated review.
- The owner-facing source repair workflow is still operator-grade in places.
- The current public export filename is a route-derived `.md` filename, not
  necessarily the owner-friendly canonical proposal name.
- The current route proof does not by itself prove every future imported legacy
  file label in Files UI will read clearly to the owner as source artifact
  versus canonical VText.
- Compare/merge robustness on the original owner table-regression transition
  remains a separate open risk from the earlier mission.

## Current System State

The system is no longer a source-demo slice. It now has a real full-length
legal-cloud proposal, canonical `.vtext` behavior, source graph, publication
source records, inline citation expansion, source windows, Markdown export, and
staging proof. The remaining risk has shifted from "does the path exist?" to
"is the path clean enough, policy-safe enough, and simple enough to build on?"

That shift matters. It means the next high-value work is not another narrow
source patch. It is a simplification and product-quality pass that keeps the
staging-proven behavior while removing weak scaffolding.

## Simplification Pass

Do this with tests as guardrails and with small commits:

1. Extract source panel state and actions from `VTextEditor.svelte`.
2. Replace `Repair JSON` as the normal visible workflow with typed source
   repair rows.
3. Audit and delete or fence `writeThroughToFile`.
4. Split source artifact attachment/import UI from the main editor body.
5. Split `internal/runtime/vtext.go` by import/export, source graph,
   diagnosis, compare/merge, and prompt/context boundaries.
6. Extract shared Playwright helpers for VText documents, publications, source
   entities, and desktop-state reset.
7. Keep `vtext-markdown-renderer.ts` as the shared renderer only if both VText
   and source reader continue to use it cleanly; otherwise split source-reader
   rendering from editor rendering.
8. Add a compact resumption section to the mission doc after this report and
   PDF are generated.

## Verification Baseline For Simplification

Before and after simplification, keep at least this baseline green:

- `pnpm --dir frontend build`
- `pnpm --dir frontend exec playwright test frontend/tests/vtext-source-entities.spec.js --project=chromium`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVText(APICreateRevisionCanonicalizesAliasedImportedDocumentTitle|OpenFileResolvesCanonicalAlias|ImportMarkdownLineageCreatesRevisionHistory|ImportMarkdownLineageResolvesCitationMarkers|ImportMarkdownLineageUsesExistingContentItems|ImportMarkdownLineageRejectsMissingContentItem|ImportMarkdownLineageRejectsUnknownCitationEntity|ImportMarkdownLineageRejectsExistingAlias|OpenFilePreservesDocxAndPDFOriginalArtifacts|OpenFileImportsDocxAndPDFBytesFromFilesRoot|EnsureManifestCreatesAliasAndFile|EnsureManifestReusesExistingAlias|CreateRevisionRejectsStaleHead|CreateRevisionRebasesAllowedStaleUserDraft|AppagentEditCanonicalizesAliasedMarkdownTitle)$'`
- Public export route still returns Markdown with compact `source:` markers and
  no `missing source` prose.
- Comet route still opens a source window as reader-mode Markdown and keeps one
  active Pretext source note.

## Landing Recommendation

Do not mark the mission complete yet. The immediate next step is to export this
report to PDF in iCloud, commit the report, then run the simplification pass
against the baseline above. After simplification, push `main`, monitor CI and
Node B deploy, confirm staging identity, and rerun the owner/public Comet proof.
