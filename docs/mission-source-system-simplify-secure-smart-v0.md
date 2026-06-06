# MissionGradient v0: Source System Simplification, Security, And Intelligence

**Status:** comprehensive mission draft  
**Created:** 2026-06-06  
**Primary contract:** [source-external-data-publication.md](source-external-data-publication.md)  
**Related evidence:** [vtext-source-viewer-mission-review-2026-06-06.md](vtext-source-viewer-mission-review-2026-06-06.md), [mission-vtext-source-viewer-reader-mode-hardening-v0.md](mission-vtext-source-viewer-reader-mode-hardening-v0.md), [mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md](mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md)

## Mission Scope

This mission is the next comprehensive source-system mission, not a narrow
Source Viewer patch. It covers the whole path from source acquisition to public
publication and export:

- external web/source fetch policy;
- Web Lens snapshot/import behavior;
- Source Service registry/search/item resolution;
- owner-scoped `ContentItem` artifacts;
- VText `source_entities`, citation markers, and inline transclusions;
- source opening decisions across Source Viewer, owning media apps, and Web
  Lens original/live inspection;
- published VText source records and guest access;
- copy/download/export of documents and sources;
- source UI layout, including magazine/journal wrapping around expanded source
  material;
- source intelligence states used by researcher/VText repair loops;
- code-surface simplification after correctness is demonstrated.

The mission must explicitly review current code, current product behavior, and
current documentation before changing code. A "works for the legal cloud
proposal" result is insufficient unless the same path generalizes to arbitrary
VText documents, imported Markdown/text/doc sources, private owner documents,
published documents, and guest-readable publications.

## Inputs And Prior User Observations

Carry these observations into the initial belief state and verify them against
code/product evidence before deciding fixes:

- Imported `.md`, `.txt`, and other draft files should become canonical
  `.vtext` by the first durable revision (`v0 -> v1`), with export back to
  Markdown as an output format rather than Markdown acting as the canonical
  editable artifact.
- `choir_private_legal_cloud_proposal.md` has been acting like VText despite
  being Markdown. The mission should create or migrate to a true
  `choir_private_legal_cloud_proposal.vtext` and preserve equivalent long-form
  content.
- The owner legal cloud proposal should include real researched sources and
  citation/transclusion points, not "missing source" placeholders except where
  the state is genuinely unresolved.
- Published forms of a VText should publish source records/snapshots according
  to access policy so users allowed to read the publication can inspect the
  supporting sources.
- Source cards currently bunch at the top in some published views. This is not
  acceptable for dozens or hundreds of sources; source UI should improve the
  article, not distract from it.
- Expanded source cards currently waste space and do not let surrounding text
  wrap in a magazine or academic-journal style.
- Source windows have recurring text-over-text layout failures. This applies to
  the Source Viewer class of windows generally, not one ABA Formal Opinion
  source.
- Expanded inline transclusions currently show too little content for the space
  they occupy. Users often click markers but do not open a separate window, so
  the inline expansion must carry enough evidence to maintain reading flow.
- Some source transclusions are only one sentence deep despite occupying a large
  side panel or inline expansion. Default transclusion depth should usually be
  richer: enough quoted/extracted context for a reader to evaluate the claim
  without opening a separate source window.
- Source Viewer reader content can collapse Markdown/cleaned content into one
  large paragraph. Reader-mode source artifacts should render Markdown-like
  structure as readable paragraphs, headings, lists, code blocks, quotes, and
  tables where present, with provenance metadata kept visually secondary.
- The iframe Web Lens path is unreliable for some web sources. If live iframe
  loading fails or is disallowed, the product should not open a broken Web Lens;
  it should show a Source Viewer reader-mode artifact and offer explicit
  original/live inspection only when viable.
- Obscura/Web Lens snapshotting does not yet clean web content well enough. The
  target is raw snapshot plus cleaned Markdown reader artifact with provenance
  and caveats.
- Appendix glossary/table formatting in the legal cloud proposal lost table
  structure at the last row. This must be investigated as a general document
  structure preservation problem, not patched as one glossary special case.
- Before final landing, produce a hard mission review report in `docs/` and a
  PDF copy in the user's iCloud Drive, then simplify the code while preserving
  functionality.

## Required Contracts

The mission is governed by these documents:

- [source-external-data-publication.md](source-external-data-publication.md):
  external source, source artifact, VText metadata, transclusion, publication,
  and export contract.
- [missiongradient-method.md](missiongradient-method.md): mission operation,
  invariant preservation, feedback, rollback, learning side-channel, and stop
  rule.
- [computer-ontology.md](computer-ontology.md) if candidate computers,
  app-change packages, promotion, or persistent computer state become involved.
- [memo-problem-documentation-first.md](memo-problem-documentation-first.md):
  newly discovered platform behavior problems must be documented before code
  fixes.

Earlier dated mission/review docs are evidence, not overriding contracts. When
they conflict with `source-external-data-publication.md`, the source contract
wins.

## Goal

Make Choir's source system simpler, more secure, and smarter by converging the
current source paths into one policy-checked evidence supply chain:

```text
external/source input
-> policy-checked acquisition
-> raw/cleaned hashes and reader artifact
-> owner/private ContentItem or source-service item
-> canonical VText source entity
-> inline transclusion
-> Source Viewer / owning app / explicit Web Lens original
-> publication source records and exports
```

This mission should remove duplicated source-shape interpretation, put URL fetch
behind an explicit security policy, make Source Viewer the default source
reader, preserve selector-rich transclusions through publication, and make source
state legible enough for agents and users to understand whether a claim is
confirmed, refuted, qualified, unsupported, stale, or blocked by access.

## Cognitive Transforms

### 1. Depth Extraction

**Concept:** source system.

**Banal version:** sources are links, cards, iframes, and snippets attached to
documents.

**Deep version:** sources are an evidence supply chain with authority, policy,
selectors, hashes, reader artifacts, and publication consequences.

**Load-bearing variable:** whether every visible citation can be traced to a
policy-valid source artifact without treating untrusted source text as
instructions or leaking private evidence.

**Common failure mode:** improving the source UI while preserving parallel,
contradictory metadata and fetch paths underneath it.

**Operational implication:** optimize the source substrate and opening contract,
then let UI surfaces become projections of that contract.

### 2. Via Negativa

The system becomes simpler by deleting ambiguous paths:

- Do not let URL presence imply Web Lens.
- Do not let Source Viewer fetch live URLs when a reader snapshot exists.
- Do not let frontend, runtime, and platform each normalize source entities
  differently.
- Do not let publication keep only the first selector when the source entity is
  richer.
- Do not let browser-public direct HTTP import bypass source policy.
- Do not let rendered DOM become the source of export truth.

### 3. State-Machine Inversion

The control surface is not "which app opens." The control surface is a
source-resolution state machine:

```text
unresolved
-> candidate
-> policy_checked_fetch
-> raw_snapshot_recorded
-> cleaned_reader_artifact
-> source_entity_bound
-> transclusion_resolvable
-> publication_projected
-> exported_with_policy
```

Apps are renderers of states. Source Viewer renders durable reader artifacts.
Web Lens is an explicit live/original inspection surface, not the default source
resolution fallback.

### 4. Threat-Model Reversal

Assume hostile or malformed sources. Ask what an attacker can do through the
source path:

- make the server fetch internal URLs;
- smuggle prompt instructions through source text;
- cause publication to disclose private source material;
- make frontend and backend disagree about open surface or visibility;
- exploit low-confidence extraction as authoritative evidence;
- degrade reader trust with broken iframes or source windows.

The mission must close these routes before adding more source intelligence.

### 5. Homotopy, Not Ladder

The low-resolution version must be a projection of the real source service, not
a separate shortcut. A small first step is acceptable only if it preserves the
same topology: policy-checked acquisition, reader artifact, source entity,
selector, transclusion, owning surface, publication/export proof.

## Current Belief State

What appears true now:

- `ContentItem` is the owner-scoped source artifact substrate.
- VText stores revision-scoped `source_entities` in metadata.
- Publication projects some source entity metadata and transclusion records.
- Source Viewer can render reader-mode Markdown snapshots and is now the
  conservative default for `open_surface: "source"`.
- Web Lens can produce reader Markdown snapshots and import them as content
  items.
- A separate `internal/sources` / `sourceapi` path exists for source-service
  style registry/search/item records.

Main weaknesses:

- Browser-public `/api/content/import-url` performs direct runtime HTTP fetch
  after only scheme/host validation.
- Source entity shape is interpreted independently in frontend renderer,
  launcher, Source Viewer, VText runtime, and platform publication.
- Publication currently builds transclusion records from the first selector
  rather than preserving selector-rich source structure.
- URL import, Web Lens snapshot import, Source Service items, media refs, and
  source-review repair all produce similar but not identical source artifacts.
- Source evidence state is readable in prose/UI, but not yet a strong typed
  state machine across search, attach, revise, publish, and export.

Highest-impact uncertainty:

- Whether the quickest durable simplification is to introduce shared typed
  source normalization first, or to move URL fetch behind a policy broker first.

Initial belief: secure fetch policy should come first for safety, but shared
`SourceEntity` / `ReaderArtifact` / `SourceOpenPlan` normalization should be
designed in the same pass so security and simplification do not diverge.

## Real Artifact

The real artifact is Choir's source evidence supply chain across:

- source acquisition and cleaning;
- owner-scoped `ContentItem` artifacts;
- Source Service item/search/resolve records;
- VText `source_entities`;
- inline citation/transclusion rendering;
- source opening and app routing;
- publication source records;
- copy/download/export policy.

The artifact is not a single UI component and not a Web Lens iframe.

## Non-Goals

- Do not redesign the entire visual language of Choir.
- Do not build a separate citation manager that bypasses VText/source entities.
- Do not turn Source Viewer into a browser clone.
- Do not make public source access depend on owner-authenticated private
  `ContentItem` reads after publication.
- Do not reintroduce Markdown as canonical once a user-authored/imported text
  document has a durable VText revision.
- Do not fix the legal cloud proposal by hardcoding its document ID, filename,
  glossary headings, or source list.

## Operating Discipline

### Problem Documentation First

Before each code-changing fix, confirm whether the work reveals a new platform
behavior problem. If yes, the first commit for that problem must update this
mission doc or a linked problem report with:

```text
problem:
affected contract/invariant:
evidence:
first observed version/transition:
suspected owner:
why local/UI-only fix is insufficient:
planned proof:
```

Examples that require documentation before code fixes:

- URL fetch can reach a forbidden network target.
- A source entity disappears or changes meaning through edit/save/revise.
- Publication drops selectors or source snapshots.
- A published guest source window reads private owner state.
- The table/glossary regression is rooted in VText/Markdown structure loss.
- Source Viewer opens broken Web Lens instead of a reader artifact.

### Initial Audit Pass

Before implementation, build a source-system map from code and product behavior:

- endpoint inventory for source acquisition, import, opening, publication, and
  export;
- storage inventory for `ContentItem`, source service items, VText revision
  metadata, publication records, source windows, and Web Lens snapshots;
- frontend component inventory for inline citations, expanded transclusions,
  source cards, Source Viewer, Web Lens launcher, published VText, and exports;
- normalizer/helper inventory, including duplicated JSON field extraction and
  fallback logic;
- security inventory for URL parsing, DNS resolution, redirects, content-type
  checks, size limits, timeout behavior, auth forwarding, and private-network
  blocking;
- current test inventory and gaps.

The audit should produce a short architecture diagram or table in this mission
doc before the first behavior-changing code commit.

### Current-State Review

This mission includes a hard review of the whole source system and current
state, not only the final diff. Produce:

- `docs/source-system-mission-review-2026-06-06.md` or a successor dated report;
- a PDF copy in the user's iCloud Drive;
- findings ordered by severity;
- code-surface cleanup opportunities;
- dead/weak/shortcut-style paths to prune;
- tests and staging proofs run;
- residual risks and next realism axes.

The review should happen twice:

1. before major implementation, to expose hidden constraints and weak paths;
2. after correctness, to simplify and reduce the implementation surface.

### Authenticated Staging Proof

If computer-use is available and working, use authenticated staging UI QA with
the owner account `yusefnathanson@me.com` in the Comet/browser path requested by
the operator. If computer-use click/action is unavailable, document the exact
limitation and use the strongest available browser/API backup. Do not claim a
UI proof that was only inferred from API state.

Staging proof must include absolute timestamps, deployed commit identity, owner
identity evidence, tested document IDs, and screenshots or durable traces where
the product path is visual.

## Hard Invariants

- VText remains canonical for document revisions.
- `source_entities` metadata is the source identity authority, not rendered
  prose or DOM.
- External source text is untrusted evidence, never prompt instructions.
- Browser-public source fetch must not reach private, loopback, link-local,
  metadata, or otherwise policy-forbidden network targets.
- Source Viewer is the default source reader for durable reader artifacts.
- Web Lens is explicit live/original inspection, not fallback for broken iframes.
- Public publication must not expose private source artifacts unless export and
  access policy permit it.
- Every visible citation marker is a transclusion point.
- Copy/download/export read canonical artifacts and source metadata, not rendered
  DOM.
- Unknown source entity kinds remain readable and recoverable.
- No hardcoded document-specific fixes.

## Value Criterion

Minimize duplicated source interpretation and unsafe source acquisition while
maximizing provenance-preserving source usefulness:

```text
L = duplicated normalization paths
  + unsafe/public fetch surface
  + selector loss
  + source-opening ambiguity
  + untyped evidence states
  + publication/export drift
```

subject to the hard invariants above.

Moving uphill means fewer source paths, stronger policy gates, better selector
fidelity, more useful reader artifacts, and clearer source state.

## Quality Target

Target quality: **excellent** for architecture boundaries and security; **solid**
for first deployed UI proof.

The mission should include a cleanup pass after first correctness:

- delete obsolete routing branches;
- remove duplicate source-shape helpers;
- centralize normalization tests;
- simplify naming around Source Viewer vs Web Lens;
- document residual risks and remaining source-service gaps.

## Homotopy Parameters

Increase realism along these axes without changing topology:

1. **Fetch policy realism:** unit guard -> local URL blocker -> deployed
   staging proof -> Source Service broker.
2. **Artifact realism:** reader snapshot string -> typed `ReaderArtifact` ->
   raw/cleaned hash pair -> selector-addressable artifact.
3. **Entity realism:** loose JSON -> shared validated `SourceEntity` ->
   generated frontend types -> migration/backfill.
4. **Selector realism:** first text quote -> multiple selectors -> page/table/
   timestamp ranges -> publication-preserved selector set.
5. **Open-surface realism:** app heuristic -> `SourceOpenPlan` -> policy-aware
   public/private open plan -> explicit live/original Web Lens action.
6. **Publication realism:** source entity copy -> transclusion/access/export
   projection -> public guest proof -> private gated proof.
7. **Source intelligence realism:** available/pending -> confirms/refutes/
   qualifies/no-source-needed/stale/blocked -> researcher-verifiable evidence
   states.
8. **VText canonicality realism:** Markdown-looking document -> imported text
   with VText metadata -> `.vtext` canonical revision after `v0 -> v1` -> export
   back to Markdown/PDF/DOCX as projections.
9. **Layout realism:** simple expanded source block -> text-around-source-card
   magazine layout -> multi-column responsive layout -> accessible fallback for
   screen readers, narrow windows, and failed font measurement.

## Representative Owner Document

Use the legal cloud proposal as a realism anchor, not as a special-case target:

- current owner document: `choir_private_legal_cloud_proposal.md`;
- known document ID: `f93cea62-f833-4dae-b414-8e44783d8cbe`;
- required future canonical artifact:
  `choir_private_legal_cloud_proposal.vtext`;
- required content equivalence: preserve the long-form proposal content from
  the earlier Markdown version, including appendix glossary/table content;
- required source behavior: researched source/citation points expand into
  transclusions and can open source windows;
- required publication behavior: allowed publication readers can inspect the
  supporting source records/snapshots.

Acceptance on this document must prove:

- imported Markdown/text becomes canonical `.vtext` on first durable revision;
- export back to Markdown preserves ordinary readable Markdown structure;
- glossary/appendix tables remain tables through focus, edit, save, revise,
  publish, and export;
- the last row of a table is not silently parsed as a paragraph or
  `TermDefinition` artifact;
- bounded table edits preserve table identity and only alter intended cells;
- ordinary prose revisions continue to use focused edit prompts and
  `apply_edits`-style metadata rather than whole-document rewrites;
- citation markers remain transclusion points after revisions.

## Structure Preservation Axis

The appendix table regression is a general VText structure preservation problem
until proven otherwise. Investigate before fixing:

- compare revisions `v70` through `v78` for the owner document;
- identify the first transition where the glossary Markdown table collapses
  into a `TermDefinition` or paragraph artifact;
- inspect the exact input/output VText/Markdown structure at that transition;
- run partial Markdown/VText render tests covering a complete table, a table
  followed by a pipe-prefixed glossary line, a table with no trailing blank
  line, a table followed by a horizontal rule/end marker, bounded edits to one
  cell, and focus/revise prompts that include partial table context;
- determine whether the failure owner is parser, renderer, edit-focus slicing,
  save serialization, VText revise, Markdown import, or export.

Do not fix by adding glossary-specific rules. Durable fixes should preserve
block structure through render/edit/save/revise and should make partial-context
editing aware of table boundaries.

## Source UI And Pretext Axis

The UI target is a magazine/academic-journal reading surface:

- source markers are compact inline controls;
- expanded inline transclusions show enough evidence to keep reading flow,
  usually more than a one-sentence stub when the source artifact contains
  deeper relevant context;
- expanded source material sits beside article text when space allows;
- article text wraps around source excerpts/cards instead of leaving dead
  rectangular whitespace;
- source lists do not bunch at the top when many sources exist;
- source windows are readable, with no text-on-text overlays;
- Source Viewer renders reader Markdown/cleaned Markdown as structured document
  content rather than flattening it into one paragraph;
- metadata is quiet and secondary to evidence content;
- "Open source" is available but not required for the common quick-check path.

Transclusion depth should be a source-selection problem, not a CSS problem. For
each cited claim, the source entity should carry enough selector context to
answer "why is this source attached here?" The default depth can vary by source
kind:

- ordinary factual claim: 2-5 sentences or a coherent paragraph around the
  selected quote;
- technical/configuration claim: relevant paragraph plus command/config snippet
  when present;
- policy/legal claim: quoted operative language plus one surrounding context
  paragraph;
- table/data claim: selected row/cell range plus labels, units, date/vintage,
  and caveat;
- media/transcript claim: timestamp range plus surrounding transcript segment;
- background/low-salience citation: compact excerpt is acceptable if the marker
  is clearly collapsed.

The expansion UI may summarize or truncate for space, but it must make the full
selected excerpt accessible without opening the separate source window.

Pretext research notes from the upstream docs:

- `@chenglou/pretext` provides DOM-free text measurement using canvas-backed
  font measurement and `Intl.Segmenter`; `prepare()` is the one-time expensive
  step and `layout()` is the cheap resize/layout step.
- For text around source cards, use line-by-line APIs such as
  `prepareWithSegments`, `layoutNextLineRange`, and `materializeLineRange` so
  each line can have a different available width.
- For inline citation chips/markers and mixed inline fragments, evaluate
  `@chenglou/pretext/rich-inline`, especially its atomic `break: "never"` and
  caller-owned `extraWidth` behavior.
- Accuracy depends on using named fonts and matching canvas font strings,
  line-height, and letter spacing to CSS. Avoid relying on `system-ui` for
  measured layouts on macOS.
- Pretext is not a full CSS engine; keep an accessible DOM/CSS fallback for
  unsupported browsers, missing `Intl.Segmenter`, screen readers, copy, and
  print/export.

Pretext implementation should be gated by proof:

- prototype with representative legal proposal paragraphs and source excerpts;
- measure desktop, narrow desktop, and mobile;
- prove no text overlap, no clipping, no layout shift on source expansion, and
  no lost selection/copy behavior;
- compare against a simpler CSS float/grid fallback before committing to a
  larger custom layout surface.

References:

- [chenglou/pretext](https://github.com/chenglou/pretext)
- [bluedusk/awesome-pretext](https://github.com/bluedusk/awesome-pretext)

## Dense Feedback

Required local probes:

- URL policy unit tests for loopback, private IPv4/IPv6, link-local, metadata
  IPs, DNS names resolving to blocked IPs, redirects, oversized bodies, and
  allowed public HTTPS.
- Source entity normalization golden tests shared across runtime/platform
  output and frontend fixtures.
- `SourceOpenPlan` tests for URL-only, content-item, source-service item,
  YouTube, PDF, published VText span, public guest source, and explicit browser.
- Reader artifact rendering tests for cleaned Markdown with paragraphs,
  headings, lists, fenced code, blockquotes, links, tables, and frontmatter-like
  metadata separators.
- Transclusion depth tests proving a citation selector can carry surrounding
  context and that expanded inline renderers do not default to one-sentence
  stubs when richer selected context exists.
- Publication tests proving multiple selectors and reader snapshots survive.
- Export tests proving source metadata does not leak UI labels and respects
  export policy.

Required product/staging proof:

- Owner VText with URL-backed source opens Source Viewer and shows durable reader
  artifact; explicit "Open original" can open Web Lens/live web.
- Source Viewer for a Markdown/reader-mode source renders structured content,
  not one large flattened paragraph.
- Expanded source transclusions in the legal cloud proposal show enough relevant
  context to evaluate the claim without opening a separate window.
- Source with blocked/failed iframe still shows reader artifact and does not
  route to broken Web Lens.
- Source Service-style item expands inline and opens Source Viewer/source item
  surface.
- Published guest reader opens source windows from publication-carried source
  snapshots without authenticated content fetch.
- Copy/download/export preserve canonical content and source policy.

Required security probes:

- user-provided URL to `localhost`, `127.0.0.1`, `::1`, RFC1918 ranges,
  link-local ranges, carrier-grade NAT, multicast, and cloud metadata IPs is
  rejected before fetch;
- hostname resolving to blocked IP is rejected after DNS resolution;
- allowed host redirecting to blocked IP is rejected at redirect time;
- malformed URL, suspicious scheme, user-info URLs, alternate IP notations, and
  overlong URLs are rejected;
- fetches enforce timeout, redirect count, response size, and content-type
  policy;
- external source content is stored/handled as untrusted bytes or evidence text,
  never as system/developer/user prompt instructions;
- publication of a private document does not expose private source text unless
  publication source policy explicitly carries the permitted snapshot.

Required source-intelligence probes:

- researcher can attach a source that confirms a claim;
- researcher can attach a source that refutes or qualifies a claim without
  rewriting the claim as confirmed;
- VText can mark `no_source_needed` for claims that should not be sourced;
- missing source UI is replaced with evidence state, candidate research, or no
  source state;
- stale or blocked sources remain visible as caveated evidence rather than
  disappearing;
- source windows show enough evidence text for a reader to evaluate why the
  marker exists.

Required data migration/backfill probes:

- legacy `source_entities` with loose JSON still render and open;
- unknown fields survive read/edit/save/revise/publish/export;
- old source markers can be upgraded lazily without whole-document rewrite;
- imported Markdown/text files convert to VText without losing file lineage or
  export affordances.

Required cleanup probes:

- source helper count decreases or duplicated responsibilities are explicitly
  justified;
- removed code paths have tests proving the replacement path covers them;
- no new document-specific fixtures are required for production logic;
- old Web Lens fallback assumptions are deleted or marked obsolete;
- frontend and backend naming uses the same concepts:
  `SourceEntity`, `ReaderArtifact`, `SourceSelector`, `SourceOpenPlan`,
  `SourceEvidenceState`.

## Forbidden Shortcuts

- Do not fix source routing by adding source-specific conditionals in one UI
  component while leaving backend/publication interpretation divergent.
- Do not solve SSRF by only regex-blocking `localhost`.
- Do not make Web Lens the fallback for failed Source Viewer.
- Do not seed success records or source artifacts through test-only routes as
  acceptance proof.
- Do not publish private source text by default just because a source marker is
  visible.
- Do not collapse source evidence into prose.
- Do not remove source provenance to make UI simpler.

## Receding-Horizon Execution

### Loop 0: Capability And Current-State Audit

1. Verify whether computer-use is available and whether click/action operations
   work. Record the result.
2. If available, prepare authenticated staging UI QA as
   `yusefnathanson@me.com` using the requested Comet/browser path.
3. If unavailable or partially broken, document the limitation and select the
   strongest browser/API backup.
4. Inventory source endpoints, storage shapes, frontend components,
   normalizers, publication/export paths, and tests.
5. Run cognitive transforms again on the current code map:
   - what can be deleted;
   - what is security-critical;
   - what is only a product illusion;
   - what would make the system more general with less code;
   - what would preserve topology while reducing surface area.
6. Update this mission doc with the audit and confirmed problem list before
   behavior-changing code.

### Loop 1: Source Fetch Safety Boundary

1. Document current URL import risk with evidence.
2. Add URL fetch policy package:
   - parse and normalize URL;
   - resolve host;
   - block private/loopback/link-local/metadata ranges;
   - re-check redirect targets;
   - enforce scheme, method, timeout, size, content type, and redirect count.
3. Route `/api/content/import-url` through this policy.
4. Verify with unit and API tests.

### Loop 2: VText Canonicality And Document Structure

1. Document current Markdown/text import behavior and whether `.md` artifacts
   currently behave identically to `.vtext`.
2. Make imported text-like documents become canonical `.vtext` at first durable
   revision while preserving file lineage and export-back-to-Markdown behavior.
3. Root-cause the owner legal cloud proposal appendix table regression by
   comparing `v70` through `v78`.
4. Run partial Markdown/VText render/edit/save/revise tests before fixing.
5. Repair the structural corruption path generally, preserving table/block
   boundaries through focus/edit/save/revise.
6. Prove untouched table survival and bounded table edit survival on staging.

### Loop 3: Canonical Source Shapes

1. Define shared source models for `SourceEntity`, `ReaderArtifact`,
   `SourceSelector`, `SourceEvidenceState`, and `SourceOpenPlan`.
2. Add normalizers with strict preservation of unknown fields.
3. Generate or mirror frontend types from the same shape.
4. Replace frontend ad hoc helpers where safe.
5. Verify source opening, inline rendering, and publication still pass.

### Loop 4: Source Opening Contract

1. Implement a single source-open resolver.
2. Make Source Viewer default for durable source artifacts.
3. Make Web Lens an explicit original/live action.
4. Preserve media app routing for YouTube/video/audio/image/PDF/EPUB.
5. Prove owner and guest source opens on staging.

### Loop 5: Publication Selector Fidelity

1. Preserve all selectors or a typed selector set in publication records.
2. Store reader artifact snapshot and hash where publication policy allows.
3. Ensure public source windows use publication-carried snapshots.
4. Verify exports and public guest source windows.

### Loop 6: Source Intelligence State

1. Introduce typed evidence states:
   `candidate`, `available`, `confirms`, `refutes`, `qualifies`,
   `no_source_needed`, `stale`, `blocked_by_access`, `unavailable`.
2. Make researcher findings and source repair use those states.
3. Render state quietly in inline/source windows.
4. Add stale/source-health caveats to Source Viewer.

### Loop 7: Source UI And Reader Layout

1. Replace source bunching with article-local source placement appropriate for
   dozens or hundreds of sources.
2. Design expanded inline transclusions to show richer evidence excerpts.
3. Repair Source Viewer reader-mode Markdown rendering so cleaned Markdown
   becomes structured readable content, not a single paragraph.
4. Make transclusion excerpt depth derive from source selectors/reader artifacts
   rather than fixed one-sentence UI stubs.
5. Prototype magazine/journal-style text wrapping around source cards using
   Pretext only where it materially improves the reading surface.
6. Fix Source Viewer text-over-text failures across source windows.
7. Ensure Source Viewer remains content-forward with minimal metadata chrome.
8. Verify desktop, narrow desktop, and mobile screenshots.

### Loop 8: Simplification Pass

1. Remove dead or redundant source routing helpers.
2. Collapse duplicate source artifact creation helpers.
3. Delete obsolete Web Lens fallback assumptions.
4. Update docs and tests to name Source Viewer, Web Lens, Source Service, and
   ContentItem boundaries consistently.
5. Run a hard code review of the whole mission state, not only the last diff.
6. Prefer deleting abstractions that only encode historical shortcuts.
7. Keep functions that encode actual contracts: fetch policy, source entity
   normalization, open-plan resolution, publication projection, export policy.

### Loop 9: Review, Report, Landing

1. Produce the mission review Markdown report in `docs/`.
2. Produce the PDF copy in the user's iCloud Drive.
3. Update this mission doc with evidence, residual risks, rollback refs, and
   next realism axis.
4. Land behavior changes using the repo landing loop:
   `commit -> push origin main -> CI -> Node B deploy -> staging identity ->
   deployed proof`.
5. Record final staging owner/guest evidence and acceptance level.

## Evidence Ledger Template

For each promoted change, record:

```text
claim:
evidence:
command/proof:
artifact paths:
result:
caveat:
promotion support:
```

## Rollback Policy

- Keep each loop in a separate commit where possible.
- If fetch policy blocks legitimate sources, rollback should revert only the
  policy change or add a documented allowlist rule, not bypass the broker.
- If shared source normalization breaks publication/source windows, rollback to
  the previous source renderer while keeping safety docs and tests.
- For deployed changes, follow the repo landing loop:

```text
commit -> push origin main -> CI -> Node B deploy -> staging identity -> deployed proof
```

## Stopping Condition

Mission is complete only when:

- Current-state audit is recorded and every newly confirmed behavior problem has
  been documented before its code fix.
- Imported Markdown/text/other text documents become canonical `.vtext` by the
  first durable revision, with export back to Markdown still available.
- The legal cloud proposal has a true `.vtext` canonical version preserving the
  long-form proposal content and appendix structure.
- The appendix table regression has a documented root cause, first failing
  transition, general repair, partial Markdown/VText tests, and staging proof.
- URL import is policy-checked and SSRF-safe under local tests.
- Source entity normalization has one canonical implementation or generated
  shared contract consumed by runtime/platform/frontend.
- Source opening uses a single `SourceOpenPlan` contract.
- Source Viewer is the default for durable source artifacts; Web Lens is
  explicit live/original inspection.
- Publication preserves selector-rich source metadata and reader snapshots
  according to access/export policy.
- Published sources are available to authorized publication readers without
  relying on owner-private authenticated source reads.
- Inline transclusions show useful evidence content and source windows no
  longer have text-over-text failures.
- Source Viewer renders cleaned Markdown/reader artifacts as structured content,
  not one large flattened paragraph.
- Expanded transclusions use selector-backed context depth and do not waste
  large panels on one-sentence stubs when deeper evidence exists.
- Source UI avoids top-bunched source lists and supports a content-forward
  magazine/journal reading path, with Pretext or a simpler fallback justified by
  proof.
- Source evidence states cover confirms/refutes/qualifies/no-source-needed/
  stale/blocked/unavailable without "missing source" as the default user-facing
  state.
- Owner and guest staging proofs pass for URL-backed, content-item, and
  source-service-style sources.
- A simplification pass removes obsolete duplicated routing/normalization code.
- The hard mission review exists as Markdown in `docs/` and as PDF in the
  user's iCloud Drive.
- Mission doc records evidence, residual risks, rollback refs, and next realism
  axis.

If only some loops land, status must be `checkpoint_incomplete`, not complete.

## Initial Review Findings To Carry Into The Run

1. **SSRF risk:** `/api/content/import-url` directly fetches user-provided URLs
   after only basic scheme/host validation.
2. **Duplicated source interpretation:** frontend, runtime, and platform all
   infer source fields and display/open policy separately.
3. **Selector loss:** publication currently derives transclusion from the first
   selector only.
4. **Boundary drift:** runtime URL import and YouTube transcript fetch overlap
   with the Source Service ownership contract.
5. **Dedupe scaling:** URL dedupe scans recent content items in memory instead
   of using indexed canonical URL/hash lookup.
6. **Iframe/Web Lens ambiguity:** live web viewing should be an explicit action,
   not a default source-opening fallback.

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint: 2026-06-06T11:13:00Z, first behavior loops landed,
deployed to staging, and Comet owner-authenticated staging capability verified.

current artifact state: documentation checkpoint commit
`bf7e52df` recorded the source-system audit and first problem records before
behavior changes. Behavior commit `068b6b5f` added runtime source fetch policy
and selector-set publication projection. Test commit `61b89e93` added broader
partial table structure-preservation coverage. Test commit `98fb4d2c` kept
comprehensive source import fixtures policy-aware after the SSRF guard started
blocking loopback fixture servers. The current frontend slice adds an explicit
source-open plan and pins Source Viewer/Web Lens routing behavior. Behavior
commit `c3295ae7` has been pushed and deployed to staging.
Existing unrelated untracked docs are preserved.

what shipped: behavior commit
`c3295ae74914ca304b4c88f7266e974882864c83` was pushed to `origin/main` and
deployed to staging. A later docs-only checkpoint may not appear in Node B
health because docs-only changes intentionally do not trigger deploy.

what was proven:

- Computer Use can see and control Comet on macOS. Comet is installed, running,
  and frontmost as `/Applications/Comet.app/` with bundle id
  `ai.perplexity.comet`.
- The current Comet session on staging is owner-authenticated:
  `https://choir.news/auth/session` visibly returned
  `{"authenticated":true,"user":{"id":"5bd6de97-3b58-408c-bf89-c42c81b083de","email":"yusefnathanson@me.com","created_at":"2026-05-26T08:58:19Z"}}`.
- The Comet staging UI exposes the Choir UI, a VText window titled
  `choir_private_legal_cloud_proposal.vtext`, source citation buttons, an open
  Source Viewer window, and screenshot capture.
- The visible staging document is already the legal cloud proposal as a `.vtext`
  titled `choir_private_legal_cloud_proposal.vtext`; this proves navigable
  product state in Comet, not yet the requested owner-authenticated identity.
- Source-system code inventory confirmed the current path spans
  `internal/runtime/content.go`, `internal/runtime/vtext.go`,
  `internal/runtime/vtext_media_sources.go`, `internal/platform/service.go`,
  `internal/platform/source_metadata.go`, `internal/sourceapi/types.go`,
  `internal/sources/*`, `frontend/src/lib/ContentViewer.svelte`,
  `frontend/src/lib/vtext-source-renderer.ts`,
  `frontend/src/lib/vtext-source-actions.ts`,
  `frontend/src/lib/vtext-source-launcher.ts`, and the VText source tests.
- Focused local tests passed:
  `nix develop -c go test ./internal/runtime -run 'TestValidateSourceFetchURL|TestSourceFetchHostResolution|TestSourceFetchRedirectPolicy'`,
  `nix develop -c go test ./internal/platform -run 'TestBuildPublicationSourceMetadataPreservesSelectorSet|TestBuildPublicationSourceMetadataDefaultsQuotedExcerptToEmbeddedTransclusion|TestPublishVTextCreatesImmutablePublicRecords'`,
  and
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextMarkdownStructureStabilization|TestVTextOpenMarkdownFile|TestVTextMarkdownTableRowParser'`.
- Additional local checks passed after the latest slices:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportURL'`,
  `npm run build` in `frontend`, and
  `npm run e2e -- vtext-source-entities.spec.js -g "VText source URL opens Source Viewer unless browser is explicitly requested"`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27060657873`
  completed successfully for `c3295ae7`, including runtime shards, non-runtime
  tests, frontend build, vet/build, and the Node B staging deploy job.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27060657872`
  completed successfully.
- Staging health at `https://choir.news/health` reported proxy and upstream
  commit/deployed_commit
  `c3295ae74914ca304b4c88f7266e974882864c83` with deployed_at
  `2026-06-06T11:11:00Z`.
- Deployed acceptance passed:
  `npm run e2e -- deployed-origin-auth-shell.spec.js -g "deployed frontend build identity matches proxy health identity"`.
- Deployed source-open acceptance passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-source-entities.spec.js -g "VText source URL opens Source Viewer unless browser is explicitly requested"`.

unproven or partial claims:

- Exact v70-v78 revision comparison for the legal proposal is not yet complete.
  Comet can load the authenticated revisions API for
  `f93cea62-f833-4dae-b414-8e44783d8cbe`, but the response contains full
  revision content and is too bulky for reliable accessibility-tree extraction.
  Comet exposes a Chromium AppleScript tab API, but JavaScript from Apple Events
  is disabled, producing: "Executing JavaScript through AppleScript is turned
  off." A bounded product diagnosis/export endpoint or enabling
  `View > Developer > Allow JavaScript from Apple Events` would allow structured
  extraction.
- CI, deploy identity, and a focused staging source-open acceptance proof have
  been produced for the first behavior slice. The broader mission proofs
  requested by the goal remain incomplete.

belief-state changes:

- The legal proposal appears to have already been migrated or opened as a true
  `.vtext` on staging, but the long-form content, source metadata, table
  survival, export, and publication behavior still need product-path proof.
- Source Viewer is present and currently renders a cleaned reader artifact for
  an open source window in Comet; however, the opening and reader snapshot
  contract is still split across frontend and backend helpers.
- Frontend source opening now has a local `sourceEntityOpenPlan` resolver:
  durable URL/content/source-service artifacts route to Source Viewer by
  default, while explicit `browser`/`web_lens`/`live`/`original` surfaces route
  to Web Lens. This is a frontend contract step, not yet a backend/shared
  schema contract.
- URL source acquisition is now locally policy-checked for literal forbidden
  hosts, redirect targets, and dial-time DNS/IP resolution in the runtime import
  path. This is not yet a full Source Service broker.
- Publication now preserves multi-selector entities as a typed selector set in
  transclusion source-selector JSON while retaining the existing single-selector
  shape for compatibility.

remaining error field:

- URL fetch policy is locally improved but not yet proven on staging and not yet
  converged into Source Service.
- Source entity normalization remains duplicated; source opening has a local
  frontend plan but is not yet shared with runtime/platform/export contracts.
- Publication selector-set projection is locally fixed and tested; frontend and
  export consumers still need broader selector-rich proof.
- Published source windows depend on frontend reconstruction of publication
  records and reader snapshots.
- Source evidence states exist in several ad hoc forms and still include
  missing/gap-oriented UI paths instead of one typed evidence-state contract.
- Table structure preservation now has broader partial-context tests, but the
  v70-v78 staging root-cause comparison is still blocked on structured
  extraction from the authenticated staging revision history.

highest-impact remaining uncertainty: whether to introduce the shared source
contract package first and route all callers through it, or to land the SSRF
policy broker first and then converge source normalization. Current bias:
documented safety risk makes the URL fetch policy the first behavior-changing
fix, with shared contract types designed in the same pass so the fix does not
create another isolated policy path.

next executable probe: either enable a bounded structured extraction route for
the legal proposal v70-v78 comparison, or continue local convergence by
introducing shared `SourceOpenPlan` / source entity normalization tests and
moving frontend source opening away from renderer heuristics.

suggested resume goal string: continue
`docs/mission-source-system-simplify-secure-smart-v0.md` from commits
`bf7e52df`, `068b6b5f`, and `61b89e93` by extracting or otherwise proving the
legal proposal v70-v78 table transition, then converging `SourceOpenPlan` and
source entity normalization without document-specific fixes.

evidence artifact refs:

- Computer Use app inventory: Comet running as `ai.perplexity.comet`.
- Comet owner identity proof:
  `https://choir.news/auth/session` returned authenticated user
  `yusefnathanson@me.com`.
- Computer Use Comet state: staging URL
  `choir.news/pub/vtext/on-this-open-legal-cloud-proposal-source-backed-vtext-document-repair-the-existing-prose-only-pub51a33d8a5`,
  visible VText title `choir_private_legal_cloud_proposal.vtext`, visible
  Source Viewer title `NixOS reproducible configuration and rollback`.
- Code inventory commands:
  `rg -l "import-url|source_entities|open_surface|Source Viewer|Web Lens|content_items|ContentItem|sourceapi|internal/sources|transclusion" internal cmd frontend`,
  targeted reads of runtime, platform, frontend source renderer/launcher,
  content viewer, and markdown table helper files.
- Comet structured extraction blocker:
  `osascript -e 'tell application "Comet" to execute active tab of front window javascript "document.body.innerText"'`
  failed because JavaScript from Apple Events is disabled.
- Behavior/test commits: `068b6b5f`, `61b89e93`, `98fb4d2c`, `c3295ae7`.
- Frontend source-open slice: `sourceEntityOpenPlan`, `ContentViewer`
  live-import guard, and focused Playwright routing proof.
- CI/deploy evidence: GitHub Actions run `27060657873`; FlakeHub publish run
  `27060657872`; staging health deployed commit
  `c3295ae74914ca304b4c88f7266e974882864c83`.

rollback refs: current branch `main`, starting commit
`1af0e8459b78fb31a18fee933a54f6f716a9b067`.

## Current-State Source-System Audit

### Endpoint Inventory

| Surface | Endpoint / file | Current behavior | Contract risk |
| --- | --- | --- | --- |
| Content item CRUD | `GET/POST /api/content/items`, `GET /api/content/items/{id}` in `internal/runtime/content.go` | Owner-authenticated `ContentItem` storage for uploaded/imported artifacts. | Correct substrate, but publication must not depend on owner-private reads. |
| URL import | `POST /api/content/import-url` in `internal/runtime/content.go` | Authenticated runtime performs direct HTTP fetch, extraction, optional SearXNG alternate discovery, then stores a `ContentItem`. | New problem: policy is local to runtime and lacks DNS/private-IP/redirect/content-type enforcement. |
| VText file open | `POST /api/vtext/files/open` in `internal/runtime/vtext.go` | Text-like file opens as canonical `.vtext` title/source path and creates original content lineage. | Contract-shaped path exists; legal proposal equivalence and first durable revision behavior still need staging proof. |
| Markdown lineage import | `POST /api/vtext/markdown-lineage/import` in `internal/runtime/vtext.go` | Imports historical Markdown/content-item versions into VText revisions with source entities and source gaps. | Useful migration surface; source gaps are still a separate missing-source-like state family. |
| Source repair/attachment | `/api/vtext/documents/{id}/source-repairs`, `/api/vtext/documents/{id}/source-artifacts` in `internal/runtime/vtext.go` | Repairs markers and attaches content items as new VText revisions. | Evidence relation/state normalization is ad hoc across UI and backend. |
| Publication | `/api/platform/vtext/publications`, `/api/platform/publications/resolve`, `/api/platform/publications/export` | Publishes canonical VText content, source entities, transclusions, policies, exports. | New problem: transclusion projection currently uses only the first selector. |
| Source Service | `cmd/sourcecycled`, `internal/sourceapi`, `internal/sources` | Registry/search/item records exist for RSS/GDELT/Telegram-style source items. | Service item types are separate from VText/source renderer contracts. |

### Storage Inventory

| Store | File / schema | Role | Contract risk |
| --- | --- | --- | --- |
| VText revisions | `vtext_revisions.metadata_json` in `internal/store/vtext.go` | Revision-scoped `source_entities`, source gaps, migration manifests, policies. | Metadata is authority, but consumers normalize it independently. |
| Content items | `content_items` in `internal/store/vtext.go` | Owner-scoped source artifact substrate with text/media/provenance. | Dedupe scans recent records in runtime; publication needs public-safe snapshots. |
| Platform publication | `publication_source_entities`, `publication_transclusions`, policies in `internal/platform/store.go` and `service.go` | Immutable public/private route artifacts and exports. | Selector set and reader artifact projection are incomplete. |
| Source Service storage | `internal/cycle/storage.go`, `internal/sources/types.go` | Fetch/item records with hashes, source IDs, fetch IDs. | Not yet converged with ContentItem/ReaderArtifact contracts. |

### Frontend Inventory

| Surface | File | Current behavior | Contract risk |
| --- | --- | --- | --- |
| Source rendering | `frontend/src/lib/vtext-source-renderer.ts` | Normalizes source entity IDs, excerpts, display policy, open app, publication entities. | Duplicates backend/platform normalization and has URL fallback to Browser. |
| Source opening | `frontend/src/lib/vtext-source-launcher.ts` | Builds window launch payload and app context from entity. | Needs a typed `SourceOpenPlan`, not renderer guesses. |
| Source actions | `frontend/src/lib/vtext-source-actions.ts` | Imports/creates source content items and source repairs. | Calls URL import directly from frontend action path. |
| Source Viewer | `frontend/src/lib/ContentViewer.svelte` | Renders content item or entity reader snapshot as structured Markdown and offers Open original. | If no content item/snapshot exists, it can still POST URL import from the viewer path. |
| Journal transclusions | `frontend/src/lib/vtext-source-flow.ts` and CSS | Side-note/stacked source flow exists and is tested. | Needs selector-depth contract and overlap/mobile proof on staging. |
| VText editor source UI | `frontend/src/lib/VTextEditor.svelte` | Shows source gaps, source entities, review/repair controls. | Gap UI needs typed evidence states instead of missing-source placeholders. |

### Normalizer / Helper Inventory

- Runtime source entity structs and merge helpers live in
  `internal/runtime/vtext_media_sources.go`.
- Platform publication source normalization lives separately in
  `internal/platform/source_metadata.go`.
- Frontend source rendering/opening normalization lives separately in
  `frontend/src/lib/vtext-source-renderer.ts` and
  `frontend/src/lib/vtext-source-launcher.ts`.
- Source Service item records live separately in `internal/sourceapi/types.go`.
- Markdown table tail repair lives in `internal/markdownstructure/tables.go` and
  is called from VText/publication export paths.

### Security Inventory

- `normalizeHTTPURL` gates scheme/host before runtime URL import, but current
  fetch uses a default `http.Client{Timeout: 30 * time.Second}` with standard
  redirect behavior.
- No confirmed DNS resolution policy blocks hostnames resolving to loopback,
  private, link-local, carrier-grade NAT, multicast, or metadata IPs before
  dialing.
- No confirmed redirect-time policy blocks an allowed public URL redirecting to
  a forbidden target.
- Size limits exist for response and stored extracted text, but oversized bodies
  are truncated with warnings rather than policy-failed.
- Content-type handling detects HTML/text/media after fetch; there is no single
  source acquisition policy object shared with Source Service.

### Test Inventory And Gaps

- Existing tests cover Markdown lineage import, source repair, imported
  Markdown becoming `.vtext`, publication source-service/content-item snapshots,
  Source Viewer structured Markdown, journal source flow, and table tail
  normalization.
- Missing tests: DNS/private-IP URL policy, redirect-to-private URL policy,
  alternate IP notation/user-info/malformed URL policy, multi-selector
  publication projection, shared source normalization golden tests,
  `SourceOpenPlan` golden tests, and v70-v78 legal proposal table transition
  comparison.

## Documented Platform Behavior Problems Before Fixes

### Problem 1: Browser-Public URL Import Is Not A Shared SSRF-Safe Source Policy

problem: `/api/content/import-url` accepts an authenticated browser request and
performs runtime HTTP fetches through a local helper after only URL
scheme/host normalization. The same path may also fetch SearXNG-discovered
alternate URLs.

affected contract/invariant: browser-public source fetch must not reach private,
loopback, link-local, metadata, or otherwise policy-forbidden network targets;
external data acquisition belongs behind source policy.

evidence: `internal/runtime/content.go` constructs `http.Client{Timeout:
30 * time.Second}` in `ImportURLContent`, calls `fetchAndExtractURL`, and
`fetchAndExtractURL` calls `client.Do(req)` without a shared DNS/IP/redirect
policy object.

first observed version/transition: current worktree at
`1af0e8459b78fb31a18fee933a54f6f716a9b067` before this mission's behavior
changes.

suspected owner: runtime content import / future Source Service acquisition
policy.

why local/UI-only fix is insufficient: UI hiding cannot prevent crafted
same-origin requests or redirect/DNS behavior; the server fetch path itself
must enforce policy.

planned proof: unit tests for blocked URL forms, DNS resolution to blocked IPs,
redirects to blocked targets, oversized responses, allowed public HTTPS, and
runtime import tests proving blocked requests fail before body fetch.

### Problem 2: Publication Transclusions Lose Selector Structure

problem: publication source metadata stores full entity JSON, but each
publication transclusion is currently built from only the first selector.

affected contract/invariant: selector-rich transclusions and source snapshots
must survive publication and export; every visible citation marker is a
selector-addressable transclusion point.

evidence: `internal/platform/source_metadata.go` reads
`selectors := sliceValue(m["selectors"])`, chooses `selectors[0]` when present,
marshals only `firstSelector` into `SourceSelector`, and derives
`SnapshotText`/`ContentHash` from that first selector.

first observed version/transition: current worktree at
`1af0e8459b78fb31a18fee933a54f6f716a9b067`.

suspected owner: platform publication source projection.

why local/UI-only fix is insufficient: once publication stores only one selector
in the transclusion table, guest readers and canonical exports cannot recover
the full selector structure from the UI.

planned proof: publication unit and Playwright tests with multiple selectors
including quote plus table/page/timestamp-style selectors, asserting the
published bundle and export preserve all selectors.

### Problem 3: Source Opening Still Depends On Frontend Heuristics

problem: source opening is resolved in frontend rendering code rather than a
shared source-open contract. URL-only entities can still fall through to
Browser, while durable reader artifacts should default to Source Viewer and
live/original Web Lens should be explicit.

affected contract/invariant: Source Viewer is the default source reader for
durable reader artifacts; Web Lens is explicit live/original inspection, not a
fallback for broken iframes.

evidence: `frontend/src/lib/vtext-source-renderer.ts` computes app IDs in
`sourceEntityOpenAppID`; if no requested open surface, content/source-service
targets become `content`, but URL presence falls back to `browser`.
`ContentViewer.svelte` can also fetch/import a URL itself when a source URL is
present without a content item or reader snapshot.

first observed version/transition: current worktree at
`1af0e8459b78fb31a18fee933a54f6f716a9b067`.

suspected owner: shared source open-plan contract across runtime/platform and
frontend launcher.

why local/UI-only fix is insufficient: every source surface needs the same
policy-aware decision so VText, publication, Source Viewer, Web Lens, and export
do not disagree.

planned proof: `SourceOpenPlan` golden tests for URL-only, content item,
source-service item, YouTube, PDF, published VText span, public guest source,
and explicit browser/original opening.

## Suggested `/goal`

```text
/goal Run docs/mission-source-system-simplify-secure-smart-v0.md as a Codex-operated MissionGradient mission. Preserve docs/source-external-data-publication.md as the requirements contract and docs/missiongradient-method.md as the operating method. First verify computer-use/Comet authenticated staging capability for yusefnathanson@me.com, document any limitation, then audit the whole current source system before behavior-changing code. Document each newly confirmed platform behavior problem before fixing it. Convert imported text-like documents to canonical .vtext by first durable revision while preserving export back to Markdown; create/migrate the legal cloud proposal to true VText with equivalent long-form content. Root-cause the owner appendix table regression by comparing v70-v78 and partial Markdown/VText render/edit/save/revise tests, then repair the general structure-preservation path. Make source acquisition policy-checked and SSRF-safe, converge source entity/reader artifact/selector/evidence/open-surface handling into shared contracts used by VText, Source Viewer, Web Lens, publication, and export. Keep Source Viewer the default for durable artifacts, make Web Lens explicit original/live inspection, preserve selector-rich transclusions and source snapshots through publication, and publish allowed source records for authorized publication readers. Replace missing-source placeholders with typed evidence states and researcher-backed confirming/refuting/qualifying/no-source-needed/stale/blocked states. Redesign source UI toward content-forward magazine/journal inline transclusions, using Pretext only where proof shows it improves text wrapping around source material. After first correctness, run adversarial/cognitive review, prune dead/weak/shortcut code paths, and produce a hard mission review report in docs plus PDF in iCloud Drive. Prove on staging with owner and guest source opens, URL-backed/content-item/source-service-style sources, legal proposal table survival, bounded table edit, publication/export source metadata, screenshots/traces, CI, Node B deploy identity, rollback refs, and residual risks.
```
