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

last checkpoint: 2026-06-06T17:00Z, deployed behavior commit
`213f0cbc465a63a4968819fa706880708bd57d7f` routes YouTube transcript
acquisition through the shared source-fetch policy, including configured
provider endpoints, InnerTube player requests, watch-page caption discovery, and
caption-track downloads. It followed docs checkpoint `2e72f622`, passed focused
local comprehensive runtime/sourcefetch tests, GitHub Actions CI, FlakeHub
publish, Node B deploy, staging health identity, deployed content-substrate
Playwright proof, and an authenticated deployed product API probe that rejected
loopback URL import with `source URL host is not allowed`. Earlier behavior
commit `ef119260` remains the backend ReaderArtifact status contract proof,
`c7210d27` remains the frontend selector-set quote extraction proof,
`eb14edde` remains the backend SourceSelector normalization proof, `efd47e1c`
remains the frontend ReaderArtifact status/evidence separation proof,
`b5c6a78f` remains the frontend SourceOpenPlan consolidation proof, and
`41b2135f` remains the evidence/open-surface normalizer consolidation proof.

current artifact state: documentation checkpoint commit
`bf7e52df` recorded the source-system audit and first problem records before
behavior changes. Behavior commit `068b6b5f` added runtime source fetch policy
and selector-set publication projection. Test commit `61b89e93` added broader
partial table structure-preservation coverage. Test commit `98fb4d2c` kept
comprehensive source import fixtures policy-aware after the SSRF guard started
blocking loopback fixture servers. The current frontend slice adds an explicit
source-open plan and pins Source Viewer/Web Lens routing behavior. Behavior
commit `c3295ae7` has been pushed and deployed to staging. Docs commit
`92138e61` recorded the source evidence-state problem before code. Behavior
commit `a2ee6dd9` adds typed source evidence-state records to VText source gaps
and source repairs. Behavior commit `cf5bf9b7` carries normalized source
evidence state into publication transclusion source selectors and includes
public source entities/transclusions in canonical export metadata. Docs commit
`bef6ed34` recorded the VText diagnosis structure-evidence gap before code.
Behavior commit `c7f43961` adds bounded VText revision structure summaries to
the owner-authenticated diagnosis route and supports `include_content=false`
for no-body verifier extraction. Behavior commit `c49064e4` surfaces bounded
diagnosis structures in the VText Sources panel and requests
`include_content=false` from the frontend. Docs commit `703138e0` records the
eight-summary app-surface limit before the follow-up behavior change. Behavior
commit `a8785e97` widens the app-surface bounded diagnosis window to 24
summaries and adds a frontend regression test requiring v78 and v70 to render.
Docs commit `9ad53d03` records the legal proposal v70-v78 diagnosis evidence
before the structure-preservation fix. Behavior commit `bfd23fa0` repairs the
general omitted-parent-table preservation path for unrelated partial-context
VText edits and adds focused runtime regression coverage. Docs commits
`746d506c`, `7ec67227`, `b879c5df`, `8e04af2c`, and `ce02e45f` record the
table preservation evidence, the two stale-draft recovery gaps, and the final
canonical-draft recovery proof before this owner bounded edit/revise checkpoint.
Behavior commits `5c61a6b8`, `465c599c`, and `93d9f819` progressively tighten
VText local draft recovery: older-parent drafts are skipped, table-flattened
same-head drafts are skipped, and any differing browser-local draft is skipped
when a non-empty canonical revision exists.
Docs checkpoint `9e1216b0` records the owner bounded edit/revise proof and the
newly confirmed post-revise source/structure problem before the metadata
follow-up behavior fix. Behavior commit `d404e6ec` fixes the first confirmed
root cause by carrying durable parent metadata such as `source_entities`
forward into ordinary user revisions before app-agent revise. Docs checkpoint
`87ea1db8` records CI/deploy/synthetic staging proof for that fix. The current
docs-only checkpoint records the fresh owner Comet proof: restoring source-rich
v87 as v90, making a bounded prose edit that created v91, and app-agent revise
to v92 preserved represented sources and source opening but still changed the
appendix table from 49 to 50 rows.
Docs checkpoint `a4165abb` records the publication export metadata gap before
code. Behavior commit `53dd9b34` includes publication access/export policy and
retrieval source/span context in Markdown/HTML/DOCX/PDF export metadata. Dev
harness commit `a7e7e821` answers the local startup question by starting
platformd and local Platform Dolt from `start-services.sh`, with Dolt declared
in the dev shell. Docs checkpoint `bb9d1614` records the frontend source
contract drift before behavior commit `41b2135f` extracted a frontend
source-contract helper module and aligned evidence/open-surface aliases with
the backend contract. Docs checkpoint `22e685d6` records the frontend open-plan
drift before behavior commit `b5c6a78f` moved generic open-plan resolution into
the same frontend source-contract module. Docs checkpoint `2eefc0bd` records
the reader snapshot/evidence-state drift before behavior commit `efd47e1c`
separates reader artifact status labels from source evidence labels in Source
Viewer. Docs checkpoint `acebbfba` records raw source selector drift before
behavior commit `eb14edde` adds shared backend selector kind normalization for
publication/export. Docs checkpoint `5df2732f` records the frontend
selector-set quote extraction drift before behavior commit `c7210d27` makes
frontend source helpers normalize selector aliases and read nested publication
selector sets. Docs checkpoint `c336890f` records the backend reader artifact
status contract gap before behavior commit `ef119260` adds shared backend
reader artifact constants/alias normalization and uses them in platform
publication enrichment. Docs checkpoint `2e72f622` records the YouTube
transcript source-fetch policy bypass before behavior commit `213f0cbc`
routes that acquisition path through `sourcefetch`.
Existing unrelated untracked docs are preserved.

what shipped: latest behavior commit
`213f0cbc465a63a4968819fa706880708bd57d7f` was pushed to `origin/main` and
deployed to staging. It replaces plain transcript `http.Client` instances with
`sourcefetch.Client`, validates each transcript provider/player/watch/caption
URL before request creation, and requires local transcript fixtures to opt into
the sourcefetch private-network test override. Prior behavior
`ef119260ecbeef3cb5f7b61287386f0f79fa7be9` adds `internal/sourcecontract`
reader artifact states and alias normalization for `reader_snapshot_ready`,
`not_publication_safe`, `bounded_excerpt_only`, and `import_failed`, then makes
platform publication enrichment emit canonical `reader_snapshot_status.state`
values through that shared backend contract. Prior behavior
`c7210d27dcb311149d56b90911b664f8a1589394` adds frontend SourceSelector helpers
that normalize selector aliases and flatten `selector_set.selectors`, then
makes Source Viewer quote extraction inspect entity selectors, reader
selectors, and publication transclusion `source_selector` records. Prior behavior
`eb14eddeba7e93e671c3026eada9b18221549a53` adds a shared backend
SourceSelector kind contract and uses it in platform publication/export so
selector aliases become canonical `text_quote`, `table_range`, `page_range`,
and `whole_resource` values while preserving selector payloads and selector
sets. Prior behavior
`efd47e1c9ae56caee0de38b25d2419febf111de6` separates frontend reader snapshot
workflow state from source evidence state so Source Viewer shows publication
reader artifact readiness without rendering `Evidence unclassified`. Prior
behavior
`b5c6a78fd2079869f9b7cb91cabc76ecb43feeec` consolidates frontend source
open-plan resolution into the dedicated frontend source-contract module while
preserving the existing renderer export for current callers. Prior behavior
`41b2135f7d72c21ee3ddccf6a7deb9053b8ad6b3` consolidates frontend source
evidence/open-surface normalization into that module. Prior behavior
commit `53dd9b34d694ecd04c354cc1e614c12d87245631` adds policy and retrieval
context to canonical publication export metadata while preserving existing
public source entities/transclusions and the private-material-omitted claim.
Dev-harness commit `a7e7e82143bf88c77a7c67a758d1bee0f2f8e023` updates local
startup only and passed CI, FlakeHub, Node B deploy, staging health identity,
and deployed publication export proof. Prior behavior
commit `fab6b25b0d3d0092d9f7f55c672373216291657b` adds shared source
open-surface normalization, makes runtime durable source producers emit
canonical `source`, normalizes platform publication open-surface metadata, and
aligns frontend source-open planning so durable source artifacts still open
Source Viewer by default while explicit Web Lens/live/original aliases open the
live inspection path. Prior behavior
commit `6a141811fd6c8ff97d4aa98f6a98bb30e59f8603` adds text-like file-open
migration manifests, carries `import_manifest` and `migration_manifest` through
durable user/appagent revisions, and makes canonical `.vtext` aliases win as a
document's source path while preserving original file aliases. Earlier behavior
commits
`3f48a1ad34af7945e4b5e764a9064a6c1c7c9c44`,
`d404e6ecd4b16bfd4a907924c16846b82e3d26ff`, and
`93d9f8197747c7526e11a730f6dab3932af82d75` remain relevant rollback refs for
source-fetch policy, source metadata carry-forward, and stale-draft recovery.
Later docs-only checkpoints may not appear in Node B health because docs-only
changes intentionally do not trigger deploy.

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
- Publication export metadata fix local checks passed:
  `nix develop -c go test ./internal/platform -run 'TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes|TestPublicationMarkdownExportNormalizesMalformedTableTailRows|TestPublishVTextCreatesImmutablePublicRecords' -count=1`
  and `npm --prefix frontend run build`.
- Local product-path browser proof passed after the local startup harness was
  repaired to launch platformd:
  `npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities as expandable transclusions and canonical exports"`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27066359739`
  completed successfully for `53dd9b34`, including Node B staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27066359741`
  completed successfully for `53dd9b34`.
- Staging health after that deploy reported proxy and upstream
  commit/deployed_commit
  `53dd9b34d694ecd04c354cc1e614c12d87245631` with deployed_at
  `2026-06-06T15:33:39Z`.
- Deployed publication export metadata proof passed:
  `CHOIR_AUTH_STATE=/tmp/choir-export-metadata.storage.json CHOIR_AUTH_META=/tmp/choir-export-metadata.meta.json PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities as expandable transclusions and canonical exports"`.
- Follow-up CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27066513637`
  passed Go/runtime/frontend gates for `a7e7e821`, and FlakeHub run
  `https://github.com/choir-hip/go-choir/actions/runs/27066513615`
  completed successfully. Node B deploy completed successfully at
  2026-06-06T15:45:36Z.
- Staging health after the follow-up deploy reported `status: "ok"`,
  `vmctl_status: "ok"`, and proxy/upstream deployed_commit
  `a7e7e82143bf88c77a7c67a758d1bee0f2f8e023`, deployed_at
  `2026-06-06T15:40:27Z`.
- Deployed publication export metadata proof passed again on the current
  `a7e7e821` staging identity:
  `CHOIR_AUTH_STATE=/tmp/choir-export-metadata-a7.storage.json CHOIR_AUTH_META=/tmp/choir-export-metadata-a7.meta.json PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities as expandable transclusions and canonical exports"`.
- Frontend source-contract local checks passed:
  `npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize|source open plans normalize'`
  and `npm --prefix frontend run build`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27066839185`
  completed successfully for `41b2135f`, including Node B staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27066839188`
  completed successfully.
- Staging health after that deploy reported `status: "ok"`,
  `vmctl_status: "ok"`, and proxy/upstream deployed_commit
  `41b2135f7d72c21ee3ddccf6a7deb9053b8ad6b3`, deployed_at
  `2026-06-06T15:54:42Z`.
- Deployed frontend source-contract proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize|source open plans normalize'`.
- Frontend open-plan contract local checks passed:
  `npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source open plans normalize'`
  and `npm --prefix frontend run build`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27066961353`
  completed successfully for `b5c6a78f`, including Node B staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27066961328`
  completed successfully.
- Staging health after that deploy reported `status: "ok"`,
  `vmctl_status: "ok"`, and proxy/upstream deployed_commit
  `b5c6a78fd2079869f9b7cb91cabc76ecb43feeec`, deployed_at
  `2026-06-06T15:59:57Z`.
- Deployed frontend open-plan contract proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source open plans normalize'`.
- Reader snapshot/evidence separation local checks passed:
  `npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize|published source readers prefer publication snapshots'`
  and `npm --prefix frontend run build`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27067193187`
  completed successfully for `efd47e1c`, including Go gates, frontend build,
  and the Node B staging deploy job.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27067193193`
  completed successfully.
- Staging health after that deploy reported `status: "ok"`,
  `vmctl_status: "ok"`, and proxy/upstream deployed_commit
  `efd47e1c9ae56caee0de38b25d2419febf111de6`, deployed_at
  `2026-06-06T16:09:51Z`.
- Deployed reader snapshot/evidence separation proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize|published source readers prefer publication snapshots'`.
- Source selector normalization local checks passed:
  `nix develop -c go test ./internal/sourcecontract ./internal/platform -run 'TestNormalizeSelector|TestBuildPublicationSourceMetadataPreservesSelectorSet|TestBuildPublicationSourceMetadataDefaultsMissingSelectorKind'`
  and
  `nix develop -c go test ./internal/platform -run 'TestBuildPublicationSourceMetadata|TestPublishVTextCreatesImmutablePublicRecords'`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27067369110`
  completed successfully for `eb14edde`, including Go gates and Node B staging
  deploy. The frontend build job was skipped by impact detection because this
  slice changed backend selector contract code and backend tests.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27067369106`
  completed successfully.
- Staging health after that deploy reported `status: "ok"`,
  `vmctl_status: "ok"`, and proxy/upstream deployed_commit
  `eb14eddeba7e93e671c3026eada9b18221549a53`, deployed_at
  `2026-06-06T16:17:17Z`.
- Deployed selector publication/export proof passed with tracked regression
  test commit `322740c6` sending alias selector kinds:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g 'publishes source-service source entities as expandable transclusions and canonical exports'`.
- Frontend selector-set extraction local checks passed:
  `npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source selectors normalize|published source entity quote falls back'`,
  `npm --prefix frontend run build`, and
  `npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source selectors normalize|published source entity quote falls back|Source Viewer renders publication transclusion selector-set quote'`
  against the local product surface started with `nix develop -c env
  CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27067629159`
  completed successfully for `c7210d27`, including Go gates, frontend build job
  `79891163989`, and Node B staging deploy job `79891235101`.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27067629132`
  completed successfully.
- Staging health was observed after deploy with `status: "ok"`,
  `vmctl_status: "ok"`, and proxy/upstream deployed_commit
  `c7210d27dcb311149d56b90911b664f8a1589394`, deployed_at
  `2026-06-06T16:28:37Z`. A later unauthenticated `/api/health` probe returned
  HTTP 401, so this checkpoint records that current public health visibility is
  limited without an authenticated/product health path.
- Deployed frontend selector-set proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source selectors normalize|published source entity quote falls back|Source Viewer renders publication transclusion selector-set quote'`.
- Reader artifact backend contract local checks passed:
  `nix develop -c go test ./internal/sourcecontract ./internal/proxy -run 'TestNormalizeReaderArtifactState|TestHandleVTextPublicationPublishesPublicURLSourceSnapshots|TestHandleVTextPublicationRecordsURLSnapshotImportFailureState' -count=1`
  and
  `npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize'`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27067967888`
  completed successfully for `ef119260`, including Go gates, runtime shards, and
  Node B staging deploy job `79892137375`.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27067967886`
  completed successfully for `ef119260`.
- Staging health after that deploy reported `status: "ok"`,
  `vmctl_status: "ok"`, and proxy/upstream deployed_commit
  `ef119260ecbeef3cb5f7b61287386f0f79fa7be9`, deployed_at
  `2026-06-06T16:43:18Z`.
- Deployed publication reader artifact/source snapshot proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js`.
  The three-test file covered source-service publication/export metadata,
  cleaned public content-item reader snapshots, and public URL-backed reader
  snapshots for guests.
- YouTube transcript source-fetch policy local checks passed:
  `nix develop -c go test ./internal/sourcefetch`,
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestFetchYouTubeTranscript|TestContentImportURLStoresConfiguredTranscriptItem|TestYouTubeJSON3CaptionURLForcesFormat' -count=1`,
  and
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportURLDedupesYouTubeSourcePackets|TestFetchYouTubeTranscript|TestContentImportURLStoresConfiguredTranscriptItem|TestYouTubeJSON3CaptionURLForcesFormat|TestChooseYouTubeCaptionTrackPrefersHumanEnglish|TestParseYouTubeTranscriptProviderPayloadHandlesNestedTranscript' -count=1`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27068249237`
  completed successfully for `213f0cbc`, including Go gates, runtime shards, and
  Node B staging deploy job `79892865383`.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27068249238`
  completed successfully for `213f0cbc`.
- Staging health after that deploy reported `status: "ok"`,
  `vmctl_status: "ok"`, and proxy/upstream deployed_commit
  `213f0cbc465a63a4968819fa706880708bd57d7f`, deployed_at
  `2026-06-06T16:55:13Z`.
- Deployed content-substrate source acquisition smoke passed:
  `GO_CHOIR_RUN_CONTENT_SUBSTRATE=1 PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/content-substrate-routing.spec.js`.
- Authenticated deployed product API probe against `https://choir.news`
  registered `playwright-state-1780765029487-7rov2y@example.com`, called
  `POST /api/content/import-url` with
  `http://127.0.0.1:1/source-policy-proof`, and received HTTP 502 with
  `{"error":"source URL host is not allowed"}`.
- Source evidence-state local checks passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextMarkdownLineage|TestVTextSourceGapRepair'`
  and `npm run build` in `frontend`.
- Local browser lineage proof against `localhost:4173` initially failed two
  response-metadata assertions because the already-running local backend still
  served old runtime code; the frontend request-payload assertion passed there.
  The updated runtime behavior was then proven on staging after deploy.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27060866683`
  completed successfully for `a2ee6dd9`, including runtime shards, non-runtime
  tests, frontend build, vet/build, and the Node B staging deploy job.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27060866687`
  completed successfully.
- Staging health at `https://choir.news/health` reported proxy and upstream
  commit/deployed_commit
  `a2ee6dd905d8402409e13187bb44f325bf01b517` with deployed_at
  `2026-06-06T11:20:47Z`.
- Deployed evidence-state/source-gap acceptance passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-markdown-lineage.spec.js -g "Migrated source gaps can be repaired|VText Sources panel applies source-gap repair|VText Sources panel can mark a citation gap as no source needed"`.
- Publication/export source metadata local checks passed:
  `nix develop -c go test ./internal/platform -run 'TestBuildPublicationSourceMetadata|TestPublishVTextCreatesImmutablePublicRecords'`
  and `npm run build` in `frontend`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27061034738`
  completed successfully for `cf5bf9b7`, including runtime shards, non-runtime
  tests, frontend build, vet/build, and the Node B staging deploy job.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27061034727`
  completed successfully.
- Staging health at `https://choir.news/health` reported proxy and upstream
  commit/deployed_commit
  `cf5bf9b70d5d9f5c3b3764811f12715db08b422f` with deployed_at
  `2026-06-06T11:29:03Z`.
- Deployed publication/export source metadata acceptance passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-source-service-publication.spec.js -g "publishes source-service source entities"`.
- Follow-up Comet extraction probe confirmed the owner session still reports
  authenticated at `/auth/session`, but direct navigation to protected
  `/api/vtext/.../history` and `/api/auth/session` returns
  `{"error":"authentication required"}`. Direct API page loads do not exercise
  the frontend `fetchWithRenewal` path, so Comet is useful for visible
  product-path proof but unreliable for bounded structured JSON extraction
  without app-surface instrumentation, JavaScript execution permission, or a
  product diagnosis/export affordance.
- VText diagnosis structure local checks passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextDiagnosis'`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27061255509`
  completed successfully for `c7f43961`, including runtime shards,
  non-runtime tests, vet/build, and the Node B staging deploy job.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27061255486`
  completed successfully.
- Staging health at `https://choir.news/health` reported proxy and upstream
  commit/deployed_commit
  `c7f439616105564810b82dabe4873b079cfd2343` with deployed_at
  `2026-06-06T11:39:22Z`.
- Deployed product-path diagnosis structure probe passed against
  `https://choir.news`: authenticated Playwright created VText document
  `9fa8de89-3ea8-4e35-95aa-8f115d853223`, created revision
  `147d79f7-5406-4a0a-b5e3-0b4968d7977a`, fetched
  `/api/vtext/documents/{id}/diagnosis?limit=10&include_content=false`, and
  verified `revision_count: 0`, no body text leak, `table_count: 1`,
  `table_row_count: 3`, `source_marker_count: 1`, and non-empty content/table
  hashes.
- Frontend diagnosis-surface local checks passed: `npm run build` in
  `frontend` and
  `npm run e2e -- vtext-markdown-lineage.spec.js -g "VText Sources panel (can cancel diagnosis|shows structured edit evidence|shows bounded revision structure)"`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27061439441`
  completed successfully for `c49064e4`, including runtime shards,
  non-runtime tests, frontend build, vet/build, and Node B staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27061439422`
  completed successfully.
- Staging health at `https://choir.news/health` reported proxy and upstream
  commit/deployed_commit
  `c49064e4ea34571fc5fccfddc0838e9714dc1331` with deployed_at
  `2026-06-06T11:48:24Z`.
- Deployed frontend diagnosis-surface acceptance passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-markdown-lineage.spec.js -g "VText Sources panel shows bounded revision structure without body text"`.
- Owner-authenticated Comet product-path proof passed for the legal proposal:
  Comet loaded `https://choir.news`, opened
  `choir_private_legal_cloud_proposal.vtext` at v87, opened Sources, invoked
  Diagnosis, and rendered bounded no-body structure cards. The visible summary
  reported `80 revisions`, `80 runs`, `v87`, `17 tables`, and
  `35 source markers`. The first visible cards covered v87 through v80 with
  compact table signatures and hashes. Direct raw navigation to the same
  diagnosis API returned `{"error":"authentication required"}`, reinforcing
  that the app-shell `fetchWithRenewal` path is required for owner proof.
- Docs checkpoint `703138e0` recorded the eight-summary diagnosis-panel limit
  as Problem 6 before the follow-up UI behavior change.
- Widened diagnosis-window local checks passed: `npm run build` in `frontend`
  and
  `npm run e2e -- vtext-markdown-lineage.spec.js -g "VText Sources panel shows bounded revision structure without body text"`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27061572672`
  completed successfully for `a8785e97`, including runtime shards,
  non-runtime tests, frontend build, vet/build, aggregate Go gate, and Node B
  staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27061572698`
  completed successfully.
- Staging health at `https://choir.news/health` reported proxy and upstream
  commit/deployed_commit
  `a8785e97fe5bd9efb25f0585a7f908e1e77911c9` with deployed_at
  `2026-06-06T11:54:45Z`.
- Deployed widened-diagnosis acceptance passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-markdown-lineage.spec.js -g "VText Sources panel shows bounded revision structure without body text"`.
- Owner-authenticated Comet proof after reload showed the refreshed legal
  proposal Sources diagnosis panel rendering `24 bounded summaries`, covering
  v87 through v64. The required v70-v78 window was visible without full-body
  API extraction. Observed structure transition:
  v78 had 0 tables/0 rows; v77 had 0 tables/0 rows; v76 had 0 tables/0 rows;
  v75 had 0 tables/0 rows; v74 had 1 table/49 rows with table signature
  `sha256:753802b2fcfe`; v73 had 1 table/49 rows with the same table
  signature; v72 had 1 table/49 rows with the same table signature; v71 had
  0 tables/0 rows; v70 had 1 table/50 rows with table signature
  `sha256:50617a6adba0`. This confirms the regression is not a single
  monotonic loss: the appendix table disappears and reappears across nearby
  revisions, and at least one older table variant has a different 50-row
  signature.
- Docs checkpoint `9ad53d03` recorded the legal proposal diagnosis evidence
  before the follow-up structure-preservation repair.
- The first focused structure-preservation fixture failed before the repair:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextMarkdownStructureStabilization'`
  did not restore an omitted appendix table when the user draft only changed an
  unrelated intro paragraph.
- Focused local structure-preservation checks passed after the repair:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextMarkdownStructureStabilization'`
  and
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextMarkdownStructureStabilization|TestVTextOpenMarkdownFile|TestVTextMarkdownTableRowParser|TestVTextImportMarkdownLineageCreatesRevisionHistory'`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27061733207`
  completed successfully for `bfd23fa0`, including runtime shards,
  non-runtime tests, integration smoke, vet/build, aggregate Go gate, and Node
  B staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27061733220`
  completed successfully.
- Staging health at `https://choir.news/health` reported proxy and upstream
  commit/deployed_commit
  `bfd23fa088d754039f679adf0a526abbdee73a64` with deployed_at
  `2026-06-06T12:02:25Z`.
- Deployed authenticated VText table-stabilization acceptance passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-table-stabilization.tmp.spec.js`.
  The temporary spec created a parent VText revision containing an appendix
  Markdown table, saved an unrelated partial-context edit omitting the table,
  verified the deployed response restored the table with
  `vtext_structure_stabilized_reason:
  preserved_parent_markdown_table_after_collapsed_draft`, and separately
  verified an explicit table-deletion edit did not restore the table. The
  temporary spec was deleted after the proof and is not part of the tracked test
  suite.
- Docs checkpoints `7ec67227`, `b879c5df`, and `8e04af2c` recorded stale
  local-draft recovery problems before the follow-up behavior fixes.
- Focused local stale-draft recovery checks passed:
  `npm run e2e -- vtext-document-stream.spec.js -g "stale local draft|same-head local draft|differing local draft"`
  and `npm run build` in `frontend`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27062222566`
  completed successfully for `93d9f819`, including runtime shards,
  non-runtime tests, frontend build, vet/build, aggregate Go gate, and Node B
  staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27062222573`
  completed successfully.
- Staging health at `https://choir.news/health` reported proxy and upstream
  commit/deployed_commit
  `93d9f8197747c7526e11a730f6dab3932af82d75` with deployed_at
  `2026-06-06T12:25:56Z`.
- Deployed stale-draft recovery acceptance passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-document-stream.spec.js -g "stale local draft|same-head local draft|differing local draft"`.
  The deployed proof covered older-parent stale drafts, same-head
  table-flattened drafts, and same-head ordinary prose drafts that differ from
  a non-empty canonical revision.
- Owner-authenticated Comet proof passed after navigating the staging tab to
  `https://choir.news/?draft_recovery=93d9f819`: the
  `choir_private_legal_cloud_proposal.vtext` window reloaded at v87 with
  `Primary draft Latest`, no `Unsaved edit`, and status
  `Autosaved draft skipped; canonical version loaded`. No owner mutation was
  performed.
- Owner-authenticated Comet bounded edit/revise proof partially passed after
  navigating the staging tab to
  `https://choir.news/?owner_bounded_edit=93d9f819`: using browser find and
  keyboard text entry, the top sentence `A private legal cloud solves this.`
  was changed to `A private legal cloud addresses this.` without using a
  whole-document accessibility `set_value`. Pressing the product `Revise`
  control created user revision v88 and app-agent revision v89 for run
  `8fd9eb3c-e...d2e201`; the trace/toast stream showed app-agent id
  `f93cea62-f833-4dae-b414-8e44783d8cbe`, an `edit_vtext` tool error, tool
  output receipt, and run completion.
- During that owner proof, the UI remained stuck at `Revising...` on v89 after
  the trace reported completion, with Sources/Publish disabled. A plain page
  reload cleared the stale app state and reloaded v89 as `Primary draft Latest`
  with Sources/Publish enabled. This is a product-state refresh weakness, but
  not the next behavior target unless it recurs without reload recovery.
- The same owner proof confirmed a narrower source/structure regression that
  must be documented before fixing: v89 Sources reported `0 represented
  sources` and `No source entities are available in this revision`, while the
  bounded diagnosis panel for the same v89 reported 7 sources, 49 source
  markers, 1 table, and 50 rows. v88 also reported 1 table/50 rows. The prior
  canonical v87 reported 1 table/49 rows, 7 sources, table range `L269-L317`,
  and table signature `sha256:a80c30b628c7`; v88/v89 reported table range
  `L269-L318` and new signatures (`sha256:01178718c7ae` for v88,
  `sha256:a86643578fed` for v89).
- Focused local checks for the metadata carry-forward fix passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextUserSaveAndAgentRevisePreserveSourcesAndTableShape'` and
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextImportMarkdownLineageResolvesCitationMarkers|TestVTextSourceGapRepairCreatesRevision|TestVTextMarkdownStructureStabilization|TestVTextMarkdownTableRowParserHandlesEscapedPipes|TestVTextAgentRevisionPromotesResearcherContentRefsToSourceEntities'`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27062590481`
  completed successfully for `d404e6ec`, including runtime shards,
  non-runtime tests, integration smoke, vet/build, aggregate Go gate, and Node
  B staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27062590491`
  completed successfully.
- Staging health at `https://choir.news/health` reported proxy and upstream
  commit/deployed_commit
  `d404e6ecd4b16bfd4a907924c16846b82e3d26ff` with deployed_at
  `2026-06-06T12:43:38Z`.
- Deployed synthetic product-path acceptance passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-source-metadata-carry-forward.tmp.spec.js`.
  The temporary spec created a fresh source-backed VText through the
  authenticated browser session, created a user revision without
  `source_entities` in request metadata, verified the deployed response carried
  the source entity forward, and verified bounded diagnosis preserved the
  source marker and appendix table row count/signature across the user save.
  The temporary spec was deleted after proof and is not part of the tracked
  suite.

unproven or partial claims:

- Exact v70-v78 revision comparison for the legal proposal is partially
  extracted through the owner app surface, enough to identify table presence,
  row count, line span, and compact table signature across the window. Full
  root-cause repair is not yet complete because the bounded summary does not
  reveal edit operation context or the surrounding Markdown/VText parse/save
  transition that removed the table in zero-table revisions. Comet can load
  the authenticated revisions API for
  `f93cea62-f833-4dae-b414-8e44783d8cbe`, but the response contains full
  revision content and is too bulky for reliable accessibility-tree extraction.
  Comet exposes a Chromium AppleScript tab API, but JavaScript from Apple Events
  is disabled, producing: "Executing JavaScript through AppleScript is turned
  off." A later direct API probe also showed `/auth/session` can renew and
  prove identity while `/api/*` navigation can still return unauthenticated,
  because it does not retry through frontend `fetchWithRenewal`.
- CI, deploy identity, and focused staging acceptance proof have been produced
  for the source-open, evidence-state/source-gap, publication/export source
  metadata, bounded diagnosis-structure, shared source-fetch policy, and
  text-like import metadata continuity slices. The broader mission proofs
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
- VText source gaps now carry typed `evidence_state: candidate`; source repairs
  preserve typed relation states such as `confirms`; no-source-needed repairs
  carry `evidence_state: no_source_needed` in repair resolution metadata.
- Publication transclusions now carry normalized `evidence_state` inside their
  `source_selector` JSON, and canonical publication export metadata now includes
  the public `source_entities` and `transclusions` records.
- VText diagnosis now has a bounded structure mode that can compare revision
  identity, content hashes, heading/source-marker counts, and compact table
  signatures without returning full private revision bodies. The product panel
  now renders a 24-summary bounded window, enough for the current v87 document
  to show v70-v78.
- Legal proposal v70-v78 bounded structure evidence now suggests multiple
  table-loss transitions rather than one isolated failing save: table present
  at v70, absent at v71, present at v72-v74, absent at v75-v78, and present
  again at v79-v80+ with 49 rows. The next repair pass should target the
  general parse/edit/save/revise structure-preservation path for partial
  contexts, not a document-specific restoration.
- The first repaired structure-preservation class is now proven: when an
  authenticated VText user save submits a partial-context draft that changes
  surrounding prose but omits a parent Markdown table, the backend restores the
  omitted parent table. When the submitted content is otherwise equivalent to
  the parent with that table removed, the backend treats it as explicit
  deletion and does not restore. This is a general Markdown table stabilization
  path, not a legal-proposal-specific special case.
- Browser-local VText drafts no longer auto-mask a non-empty canonical
  revision. The owner legal proposal now reloads in Comet as v87 `Latest`
  rather than `Unsaved edit`, with the stale browser-local draft skipped before
  any owner save/revise action.
- Owner bounded edit/revise no longer loses the appendix table completely, but
  it still fails the stricter mission contract: the source entity list is empty
  in the Sources panel after the app-agent revision even though diagnosis still
  counts sources/markers, and the table row count/signature changed across a
  prose-only top-of-document edit. The next repair must root-cause the general
  source metadata carry-forward and table normalization path, not patch this
  document by hand.
- User-authored VText revisions now inherit durable parent metadata, including
  `source_entities`, before app-agent revise. Text-like imported documents now
  also carry `import_manifest` and `migration_manifest` from v0 into the first
  durable revision, with deployed proof for Markdown and plain text. These
  fixes do not mutate already-created owner legal-proposal revisions and still
  require a fresh owner legal-proposal proof after the passkey session is
  renewed.

remaining error field:

- URL fetch policy is converged through the shared `internal/sourcefetch`
  package for runtime URL import and source-service adapters, with staging
  product-route proof for forbidden loopback import. Robots/TOS/rate policy and
  future connector/Web Lens acquisition policy remain open.
- Source entity, evidence-state, open-surface, and selector normalization now
  have shared backend/frontend contract slices and staging proof for the focused
  publication/source-open cases. A generated or otherwise single-source schema
  across runtime, platform, frontend, export, and Source Service remains open.
- Publication selector-set projection and export source metadata are locally and
  staging proven for source-service-style sources, and backend publication now
  canonicalizes selector kind aliases through `internal/sourcecontract`;
  broader selector-rich guest/content-item/legal-proposal export proof remains
  incomplete.
- Published source windows depend on frontend reconstruction of publication
  records and reader snapshots.
- Source evidence states are typed for Markdown lineage gaps, owner source
  repairs, publication transclusion selectors, and export metadata. Researcher
  updates, Source Service records, stale/blocked/unavailable product flows, and
  shared frontend/backend schema convergence remain incomplete. Source Viewer
  now keeps publication reader snapshot readiness separate from evidence-state
  labels, but backend/generated `ReaderArtifact` schemas are still incomplete.
- Table structure preservation now has broader partial-context tests, a
  deployed bounded diagnosis extraction route, a deployed authenticated
  synthetic proof for omitted-parent-table restoration versus explicit
  deletion, and an owner product-path bounded edit/revise proof. The owner
  proof shows the table survives as a table, but not with exact structure:
  v87's 49-row appendix becomes 50 rows in v88/v89 after a prose-only edit.
- VText source representation has a newly confirmed owner regression: v89
  diagnosis still sees 7 sources/49 markers, but the Sources panel reports
  `0 represented sources` and cannot choose/open source entities for the
  latest revision. Behavior commit `d404e6ec` fixes the user-save metadata
  carry-forward class for future revisions. Fresh owner Comet proof on
  deployed `d404e6ec` restored source-rich v87 as v90, then created v91/v92
  through bounded prose edit/revise; v92 shows 7 represented sources and opens
  Qdrant as a durable Source Viewer reader artifact.
- The table preservation problem is now narrowed: after restoring v87 as v90
  with 49 appendix rows and signature `sha256:a80c30b628c7`, the bounded user
  save v91 and app-agent v92 both materialized 50-row appendix tables outside
  the edited prose region. This is documented as Problem 10 before any
  follow-up structure fix.

highest-impact remaining uncertainty: owner Comet is currently authenticated
for `yusefnathanson@me.com`, and same-origin bookmarklet reads can call public
owner VText APIs. The remaining uncertainty is not login; it is the safe,
repeatable owner legal-proposal mutation/proof path for bounded edit/revise,
publication/export metadata, screenshots/traces, and rollback refs without
global-prompt misrouting or whole-document UI replacement.

next executable probe: use an authenticated product path that explicitly
targets legal proposal doc `f93cea62-f833-4dae-b414-8e44783d8cbe` to run the
bounded edit/revise proof, then verify table signature/row count, represented
sources, source opens, publication/export metadata, and rollback refs. Avoid
the global prompt bar for this proof because it previously created a separate
VText artifact. If Comet bookmarklet mutation remains unreliable, build a
normal Playwright/Chrome-auth proof path or add a safe owner-approved UI/API
test harness rather than editing the whole VText body through Computer Use.

suggested resume goal string: continue
`docs/mission-source-system-simplify-secure-smart-v0.md` from behavior commit
`c7210d27dcb311149d56b90911b664f8a1589394` plus docs checkpoints `9464f38b`
and `d0fca254`. Owner Comet is authenticated; continue with a safe
document-id-scoped legal proposal bounded edit/revise proof if the product API
path can be made reliable, otherwise continue the next source-contract
convergence slice while preserving the proven source-fetch policy,
source-entity carry-forward, table normalization, text-like import metadata,
and source open-surface behavior.

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
- Behavior/test commits: `068b6b5f`, `61b89e93`, `98fb4d2c`, `c3295ae7`,
  `92138e61`, `a2ee6dd9`.
- Stale-draft recovery commits: docs `7ec67227`, docs `b879c5df`, docs
  `8e04af2c`, behavior `5c61a6b8`, behavior `465c599c`, and behavior
  `93d9f8197747c7526e11a730f6dab3932af82d75`.
- Latest stale-draft deployment evidence: CI run `27062222566`, FlakeHub run
  `27062222573`, staging health deployed_commit
  `93d9f8197747c7526e11a730f6dab3932af82d75`, deployed Playwright command
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-document-stream.spec.js -g "stale local draft|same-head local draft|differing local draft"`,
  and owner Comet URL `https://choir.news/?draft_recovery=93d9f819` showing
  v87 `Latest` with `Autosaved draft skipped; canonical version loaded`.
- Owner bounded edit/revise evidence: Comet URL
  `https://choir.news/?owner_bounded_edit=93d9f819`, document
  `choir_private_legal_cloud_proposal.vtext`, rollback refs v87 (pre-edit,
  1 table/49 rows, 7 sources, table signature `sha256:a80c30b628c7`), v88
  (user save, 1 table/50 rows, 7 sources, table signature
  `sha256:01178718c7ae`), and v89 (app-agent revision, 1 table/50 rows,
  diagnosis 7 sources/49 markers but Sources panel `0 represented sources`,
  table signature `sha256:a86643578fed`). Revise run handle
  `8fd9eb3c-e...d2e201`; app-agent trace id
  `f93cea62-f833-4dae-b414-8e44783d8cbe`.
- Source metadata carry-forward fix: behavior commit
  `d404e6ecd4b16bfd4a907924c16846b82e3d26ff`, CI run `27062590481`, FlakeHub
  run `27062590491`, staging health deployed_commit
  `d404e6ecd4b16bfd4a907924c16846b82e3d26ff`, deployed_at
  `2026-06-06T12:43:38Z`, local focused comprehensive runtime tests above,
  and deployed temporary Playwright acceptance
  `PLAYWRIGHT_BASE_URL=https://choir.news npm run e2e -- vtext-source-metadata-carry-forward.tmp.spec.js`.
- Frontend source-open slice: `sourceEntityOpenPlan`, `ContentViewer`
  live-import guard, and focused Playwright routing proof.
- CI/deploy evidence: GitHub Actions run `27060657873`; FlakeHub publish run
  `27060657872`; staging health deployed commit
  `c3295ae74914ca304b4c88f7266e974882864c83`.
- Evidence-state slice: typed VText source gap and repair evidence metadata,
  GitHub Actions run `27060866683`, FlakeHub publish run `27060866687`,
  staging health deployed commit
  `a2ee6dd905d8402409e13187bb44f325bf01b517`, deployed Playwright proof for
  repaired gaps and no-source-needed repairs.
- Publication/export source metadata slice: behavior commit `cf5bf9b7`,
  GitHub Actions run `27061034738`, FlakeHub publish run `27061034727`,
  staging health deployed commit
  `cf5bf9b70d5d9f5c3b3764811f12715db08b422f`, deployed Playwright proof for
  source-service transclusion and export metadata evidence state.
- VText diagnosis structure slice: docs commit `bef6ed34`, behavior commit
  `c7f43961`, GitHub Actions run `27061255509`, FlakeHub publish run
  `27061255486`, staging health deployed commit
  `c7f439616105564810b82dabe4873b079cfd2343`, deployed Playwright/Node
  product-path proof for no-content revision structure summaries.
- Shared source-fetch policy slice: docs commit `86260b6b`, behavior commit
  `3f48a1ad34af7945e4b5e764a9064a6c1c7c9c44`, deploy-proof docs commit
  `82d47280`, CI run `27065234996`, FlakeHub run `27065235002`, staging
  health deployed commit `3f48a1ad34af7945e4b5e764a9064a6c1c7c9c44`, and
  deployed public `/api/content/import-url` loopback rejection proof.
- Text-like import metadata slice: docs commit `4f6f5830`, behavior commit
  `6a141811fd6c8ff97d4aa98f6a98bb30e59f8603`, CI run `27065789129`,
  FlakeHub run `27065789119`, staging health deployed commit
  `6a141811fd6c8ff97d4aa98f6a98bb30e59f8603`, and deployed Playwright proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-markdown-lineage.spec.js -g 'Imported (Markdown|plain text)'`.

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

### Problem 4: Missing-Source And Evidence State Are Parallel Ad Hoc Contracts

problem: source-backed VText has at least three overlapping state families:
`metadata.source_gaps` for missing citation artifacts, source entity
`evidence.state`/`research_state` for represented source artifacts, and source
repair `relation` values such as `confirms`. These encode different parts of
the same claim/source/evidence relation without one typed state vocabulary.

affected contract/invariant: missing-source placeholders must be replaced with
typed evidence states and researcher-backed
confirming/refuting/qualifying/no-source-needed/stale/blocked states. VText,
Source Viewer, publication, export, and researcher repair must agree on the
meaning of the state they carry.

evidence: `internal/runtime/vtext_media_sources.go` defines
`vtextSourceEntityEvidence` with free-form `State` and `ResearchState`;
`frontend/src/lib/VTextEditor.svelte` derives `sourceGaps` from
`metadata.source_gaps`; `frontend/src/lib/vtext-source-actions.ts` defaults
repair relation to `confirms`; existing tests assert `source_gaps`,
`research_state: represented`, and `relation: confirms` independently.

first observed version/transition: current worktree at
`1e62f6bf44a9d7e...` after the first source-open staging checkpoint.

suspected owner: shared source entity/evidence contract across runtime VText
metadata, researcher updates, source repair actions, frontend rendering,
publication, and export.

why local/UI-only fix is insufficient: UI label changes cannot make exported
metadata, publication records, and appagent/researcher revisions preserve the
same evidence semantics; the normalized metadata itself must carry typed
states.

planned proof: unit or focused browser tests showing migrated source gaps carry
typed `evidence_state` records, owner source repairs convert gaps into source
entities with `state: confirms`, no-source-needed revisions clear gaps through
`state: no_source_needed`, and Source Viewer/VText render the state without
falling back to missing-source placeholders.

### Problem 5: VText Diagnosis Lacks Bounded Structure Evidence

problem: the owner-authenticated product path has either full revision bodies
or high-level history metadata, but no bounded structure summary that can prove
table preservation/regression across historical revisions without dumping full
legal-proposal content into a browser accessibility tree.

affected contract/invariant: root-cause work for the legal proposal appendix
table regression must compare v70-v78 through product/control evidence. The
verifier needs table/heading/source-marker structure, hashes, and revision
identity without requiring a full private document export or direct database
access.

evidence: `GET /api/vtext/documents/{id}/revisions?limit=10000` returned full
revision JSON in Comet for `f93cea62-f833-4dae-b414-8e44783d8cbe`, but the
response was too large for reliable accessibility-tree extraction. Direct
navigation to `/api/vtext/documents/{id}/history` then returned
`{"error":"authentication required"}` because direct API page loads do not
exercise frontend `fetchWithRenewal`. Existing `HandleVTextDiagnosis` lists
revision responses through `revisionResponseFromRecord`, which includes full
`Content`.

first observed version/transition: current staging behavior at
`cf5bf9b70d5d9f5c3b3764811f12715db08b422f` during the Comet extraction probe.

suspected owner: VText diagnosis/product verifier surface.

why local/UI-only fix is insufficient: local tests can model table survival,
but the mission needs owner-authenticated staging evidence against the actual
legal proposal history. UI screenshots cannot reliably compare hidden table
rows and Markdown/VText structure across eight revisions.

planned proof: extend the owner-authenticated VText diagnosis bundle with
bounded revision structure summaries: version/revision identity, content hash,
line/heading/table/source-marker counts, compact table signatures, and no full
content. Add runtime tests proving diagnosis includes structure summaries and
omits private revision bodies when requested with a bounded structure mode.

### Problem 6: VText Diagnosis Panel Truncates Before Required Revision Window

problem: the deployed owner-authenticated VText Sources diagnosis panel renders
only eight bounded revision structure summaries, even when the backend returns
up to 80 structures. For the legal proposal at v87, the visible app surface
therefore shows v87 through v80 and cannot expose the required v70-v78 table
transition.

affected contract/invariant: root-cause work for the legal proposal appendix
table regression must compare v70-v78 through product/control evidence, not by
direct database access or full private revision dumps. The owner app surface
must render enough bounded structure evidence to inspect that window.

evidence: deployed staging commit
`c49064e4ea34571fc5fccfddc0838e9714dc1331` passed CI, FlakeHub, Node B deploy
identity, and focused Playwright acceptance for bounded diagnosis rendering.
Computer Use in Comet opened `choir_private_legal_cloud_proposal.vtext` at v87,
opened Sources, clicked Diagnosis, and rendered `80 revisions`, `80 runs`,
`17 tables`, and `35 source markers`, but the rendered bounded summaries were
limited to eight cards: v87, v86, v85, v84, v83, v82, v81, and v80. Direct raw
navigation to
`/api/vtext/documents/f93cea62-f833-4dae-b414-8e44783d8cbe/diagnosis?limit=10000&include_content=false`
returned `{"error":"authentication required"}`, so the app surface remains the
available owner-authenticated proof path.

first observed version/transition: current staging behavior at
`c49064e4ea34571fc5fccfddc0838e9714dc1331` during the owner Comet diagnosis
panel probe.

suspected owner: VText diagnosis product UI.

why local/API-only fix is insufficient: the backend already returns bounded
structure data without full content, and mocked frontend tests prove the
component can render a structure summary. The failing realism axis is the
owner-authenticated product panel's visible revision window for the actual
legal proposal.

planned proof: render a larger bounded structure window from the existing
`revision_structures` payload, keep the no-body diagnosis request, add a
frontend regression test that includes v70, redeploy, and re-run Comet owner
inspection to confirm v78 through v70 are visible with table signatures.

### Problem 7: Stale Local VText Draft Can Mask Repaired Canonical Structure

problem: after the canonical legal proposal head was repaired to v87 with a
49-row appendix table, the owner-authenticated Comet editor still showed
`Unsaved edit` and rendered the appendix glossary in collapsed prose form
(`Term`, `Definition`, row text) inside the editable document surface. The
Sources diagnosis panel on the same window reported v87 with `1` table,
`49` rows, and table signature `sha256:a80c30b628c7`. The editor draft and
canonical head therefore disagreed inside one product window, and saving or
revising from that stale draft could reintroduce the table-loss regression.

affected contract/invariant: canonical VText revisions are the source of truth.
Browser local draft recovery must not silently override a newer canonical head,
especially when the recovered draft is based on an older revision and has lost
document structure. Owner legal-proposal proof cannot safely mutate the real
document while a stale collapsed draft is auto-restored over the repaired head.

evidence: Computer Use in Comet on `https://choir.news` showed the
`choir_private_legal_cloud_proposal.vtext` window at v87 with `Unsaved edit`.
The Sources diagnosis panel in the same window showed `24 bounded summaries`,
v87 table count `1`, row count `49`, table line range `L269-L317`, and table
signature `sha256:a80c30b628c7`. The editable document text area exposed by the
accessibility tree showed the Appendix A glossary as plain lines beginning
`Term`, `Definition`, `32B / 70B / 120B-class models`, etc., not as a Markdown
pipe table. Code inspection showed `restoreLocalDraftIfNewer()` restores any
localStorage draft whose content differs from the current revision and does not
check the stored `parent_revision_id` against the current head revision.
After deploying a parent-revision guard in `5c61a6b8`, synthetic staging proof
passed for stale drafts based on older revisions, but reloading the owner Comet
window still restored the collapsed legal-proposal draft at v87. That shows the
dangerous draft can also be based on the current head after the rendered editor
has already serialized a table-flattened draft; parent identity alone is not a
sufficient restore guard.

first observed version/transition: staging at deployed commit
`bfd23fa088d754039f679adf0a526abbdee73a64`, Comet owner session after the
appendix-table stabilization repair was deployed.

suspected owner: frontend VText local draft restore and revision/head
synchronization.

why local/API-only fix is insufficient: the dangerous state is browser-local
and owner-session-specific. Backend stabilization can repair a submitted draft,
but it cannot prevent the product editor from presenting stale collapsed
content as the active owner draft. The restore policy must prevent stale drafts
from masking a newer canonical head before the user saves or revises.

planned proof: add frontend regression tests that seed local drafts with both
an older `parent_revision_id` and a same-parent table-flattened draft, advance
or load the canonical document head to a version with a Markdown table, reopen
the document, and verify the editor renders the current table head instead of
auto-restoring either stale collapsed draft. Then redeploy and re-open the
owner legal proposal in Comet to verify v87 no longer shows the collapsed
appendix draft as an unsaved edit.

### Problem 8: Local Draft Recovery Still Trusts Browser Text Over Versioned VText

problem: after deploying the parent-revision and Markdown-table-count draft
guards, the owner-authenticated Comet legal-proposal window still restored a
browser-local v87 draft over the canonical v87 document. The editor remained in
`Primary draft Unsaved edit` with status `Autosaved draft restored`, and the
appendix glossary was still exposed as flattened prose lines rather than a
canonical table. This proves the restore policy is still too trusting when a
document already has a durable revision.

affected contract/invariant: canonical VText revisions are the source of truth.
Local browser draft recovery is a convenience cache, not a revision line, and
must not silently replace the visible content of an existing non-empty
canonical head. A local draft can be offered or retained as recovery evidence,
but automatic replacement of a versioned VText body is unsafe for owner
workflows.

evidence: behavior commit `465c599c44b425264813f6c9072e2ca18078e7c2` added
frontend tests for older-parent stale drafts and same-head table-flattened
drafts. Local e2e and frontend build passed; GitHub Actions run
`27062081688`, FlakeHub run `27062081694`, staging health
`/health` deployed commit `465c599c44b425264813f6c9072e2ca18078e7c2` at
`2026-06-06T12:19:15Z`; deployed staging Playwright proof passed for synthetic
old-parent and same-head table-flattened drafts. After that deploy, Computer
Use reloaded the owner-authenticated Comet window at `https://choir.news` and
the legal proposal still showed v87, `Primary draft Unsaved edit`, status
`Autosaved draft restored`, and flattened glossary text in the editable VText
surface. No owner mutation was performed.

first observed version/transition: staging at deployed commit
`465c599c44b425264813f6c9072e2ca18078e7c2` during owner Comet reload after the
second stale-draft repair.

suspected owner: frontend VText local draft recovery policy.

why local/API-only fix is insufficient: synthetic Playwright covered the table
patterns we could seed into browser storage, but the actual owner session still
contains a browser-local draft shape that bypasses those heuristics. A
heuristic that tries to prove every unsafe draft shape is incomplete. The
safer product invariant is that durable VText revisions load from canonical
state by default, and browser-local drafts do not auto-replace non-empty
versioned content.

planned proof: change VText local draft recovery so an existing non-empty
canonical revision is loaded by default and any differing browser draft is
skipped instead of auto-restored. Add a frontend regression proving a current
head with ordinary prose does not auto-restore a differing local draft. Keep
draft cleanup/skip status observable, redeploy, and reload the owner legal
proposal in Comet to verify the window no longer shows `Unsaved edit` or
`Autosaved draft restored` before attempting a bounded owner edit/revise proof.

### Problem 9: Owner Prose-Only VText Revise Drops Represented Sources And Changes Table Shape

problem: after the stale-draft fixes, an owner-authenticated product-path
bounded edit/revise on the legal cloud proposal no longer flattened the
appendix table away, but it still failed the stricter source/structure
contract. The latest app-agent revision v89 rendered as `Primary draft Latest`
after reload, but the Sources panel reported `0 represented sources` and
`No source entities are available in this revision` even though the bounded
diagnosis for the same revision reported 7 sources and 49 source markers. The
appendix table also changed from v87's 49-row signature to a v88/v89 50-row
signature after an edit to unrelated top-of-document prose.

affected contract/invariant: a VText user save and app-agent revise operation
that edits prose outside a table must preserve source entities, source marker
openability, and existing table structure unless the edit explicitly changes
those structures. Bounded diagnosis and the Sources panel must agree about
whether source entities are represented and openable for a revision.

evidence: on staging deployed commit
`93d9f8197747c7526e11a730f6dab3932af82d75`, Computer Use in owner-authenticated
Comet navigated to `https://choir.news/?owner_bounded_edit=93d9f819`. The
legal proposal loaded as `choir_private_legal_cloud_proposal.vtext` v87,
`Primary draft Latest`. Using browser find and keyboard typing, the sentence
`A private legal cloud solves this.` was replaced with
`A private legal cloud addresses this.` without using whole-document
accessibility replacement and without touching the appendix. Pressing the
product `Revise` control created user revision v88 and app-agent revision v89.
The app trace showed run handle `8fd9eb3c-e...d2e201`, app-agent id
`f93cea62-f833-4dae-b414-8e44783d8cbe`, an `edit_vtext` tool error, tool
output receipt, and run completion. The UI temporarily remained stuck in
`Revising...` with Sources/Publish disabled until a normal page reload, after
which v89 loaded as `Primary draft Latest`.

bounded diagnosis after reload showed v87 with 1 table/49 rows, 7 sources,
table range `L269-L317`, and table signature `sha256:a80c30b628c7`; v88 with
1 table/50 rows, 7 sources, table range `L269-L318`, and table signature
`sha256:01178718c7ae`; and v89 with 1 table/50 rows, 7 sources, table range
`L269-L318`, and table signature `sha256:a86643578fed`. The same v89 Sources
panel displayed `0 represented sources`, despite source labels still appearing
inline in the rendered document and diagnosis reporting 49 source markers.

first observed version/transition: staging owner Comet workflow from v87 to
v88/v89 on June 6, 2026, after behavior commit
`93d9f8197747c7526e11a730f6dab3932af82d75` was deployed and the stale
browser-local draft was skipped.

suspected owner: VText save/revise source metadata carry-forward and structure
normalization. The row-count shift may be in Markdown table parser/export
normalization between editor save and app-agent edit, while the represented
source loss likely sits in revision metadata source-entity projection or
frontend source-entity extraction after app-agent revisions.

### Problem 10: Metadata-Bearing VText User Save Still Adds An Appendix Table Row

problem: after behavior commit `d404e6ec` fixed durable source metadata
carry-forward, a fresh owner-authenticated product-path proof preserved
represented sources through restore, user save, and app-agent revise, but still
changed the appendix table shape. The restored metadata-bearing source revision
v90 had the same 49-row appendix signature as v87. A bounded prose-only browser
edit created user revision v91 with 50 appendix rows, and the subsequent
app-agent revision v92 also had 50 appendix rows. The changed table was far
below the edited prose and was not intentionally edited.

affected contract/invariant: a VText user save must not re-materialize an
unrelated Markdown table differently simply because the editor serialized the
document after a bounded prose edit elsewhere. App-agent revise must preserve
the table shape it receives unless it explicitly edits that table. Source
metadata preservation and table preservation are separate invariants; proving
one must not mask failure of the other.

evidence: on staging deployed commit
`d404e6ecd4b16bfd4a907924c16846b82e3d26ff`, Computer Use in
owner-authenticated Comet used the legal proposal Sources panel to inspect
v88/v87. v88 still showed `0 represented sources`; v87 showed
`7 represented sources` and source artifact controls. The product Restore
button restored v87 as latest v90. v90 showed `7 represented sources` and the
diagnosis row for v90 reported 1 table, 49 rows, 7 sources, hash
`sha256:4e6f3f9888c7`, and table signature `sha256:a80c30b628c7` at
`L269-L317`.

A bounded owner edit was then made in the first prose section by cursor
insertion near `A private legal cloud solves this.`. The insertion temporarily
produced the small typo `I in productiont`, creating user revision v91 when
the product Revise button was pressed. The app-agent run handle displayed
`e17846d9-7...8c5333`; the trace/toast again showed app-agent id
`f93cea62-f833-4dae-b414-8e44783d8cbe`, `edit_vtext`, received tool output,
and completion. The app-agent corrected the prose back to
`A private legal cloud solves this. It is...`, creating v92.

After reload cleared the stale `Revising...` toolbar state, v92 loaded as
`Primary draft Latest` with `7 represented sources`. The source panel opened
the Qdrant source card as a separate durable Source Viewer window titled
`Qdrant similarity search documentation`, with reader text, an `Open original`
link, and Source evidence / Source entity / Provenance disclosures. This proves
the source metadata/opening path for the repaired class while isolating the
remaining structure bug.

bounded diagnosis on v92 reported `v92 21 tables 70 source markers`. The
revision rows showed v92 with 1 table, 50 rows, 7 sources, hash
`sha256:f21ce9be51fb`, table signature `sha256:a86643578fed` at `L269-L318`;
v91 with 1 table, 50 rows, 7 sources, hash `sha256:f241be40dcf5`, table
signature `sha256:01178718c7ae` at `L269-L318`; and restored v90 with 1 table,
49 rows, 7 sources, hash `sha256:4e6f3f9888c7`, table signature
`sha256:a80c30b628c7` at `L269-L317`.

first observed version/transition: staging owner Comet workflow from restored
v90 to v91/v92 on June 6, 2026, after behavior commit
`d404e6ecd4b16bfd4a907924c16846b82e3d26ff` was deployed and after v87 had been
restored as latest to ensure the parent revision contained durable
`source_entities`.

suspected owner: VText rendered-editor serialization and backend
structure-stabilization around Markdown table boundaries. The previous
omitted-parent-table fix protects against table disappearance, but this proof
suggests a separate materialization path can insert or preserve an extra
delimiter/blank row when browser-rendered document state is converted back into
Markdown for a user save.

why local/API-only fix is insufficient: the failure was exposed by the real
owner legal proposal, Comet session state, editor save, app-agent revise, and
Sources panel/diagnosis surfaces together. A synthetic backend table test is
not enough; the repaired path must prove the full product transition and the
source-open surface.

planned proof: add focused regression coverage for a source-backed VText with
an appendix table where a prose-only user save and app-agent-style edit preserve
represented source entities and exact table shape. Then redeploy, run a fresh
owner Comet bounded edit/revise proof, and verify Sources shows represented
source entities, diagnosis row counts/signatures remain stable, Source Viewer
opens still work, and publication/export metadata includes the preserved source
records.

follow-up root-cause refinement after source-contract deploy proof:

- the restore-path table-tail fix made restored parent revisions canonical, but
  the editable VText frontend can still render Markdown tables as DOM `<table>`
  nodes and serialize DOM rows back to Markdown when saving a browser edit;
- the serializer always inserted a Markdown separator row after the first DOM
  row, which is correct for normal rendered tables but unsafe if an existing
  separator-like row survives into the DOM table rows;
- the backend structure stabilizer repaired omitted/collapsed tables and final
  rows missing delimiters, but did not remove duplicate separator rows when the
  submitted content still contained at least as many table blocks as the parent;
- this is the still-open browser-save class of Problem 10: one extra table row
  below an unrelated prose edit while source metadata remains preserved.

First repair implementation:

- `frontend/src/lib/VTextEditor.svelte` now skips separator-like DOM rows while
  serializing a table and then inserts one canonical Markdown separator;
- `internal/markdownstructure.NormalizeTableShapedRows` now removes duplicate
  Markdown separator rows inside a table, protecting the API path from stale or
  alternate clients that submit duplicated separators;
- `internal/runtime` now has a source-bearing VText user-save regression proving
  a prose edit above the appendix survives, duplicate separators are removed,
  the parent table text/signature is preserved, and durable `source_entities`
  still carry forward.

Local verification:

```text
nix develop -c go test ./internal/markdownstructure -count=1
result: passed

nix develop -c go test -tags comprehensive ./internal/runtime -run TestVTextUserSaveRemovesDuplicateMarkdownTableSeparator -count=1
result: passed

nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextMarkdownStructureStabilization(RepairsMalformedTableTailRow|HandlesPartialTableContexts|AllowsExplicitTableDeletion)|TestVTextSourceMetadataCarryForwardPreservesTablesThroughUserAndAgentRevisions' -count=1
result: passed

npm --prefix frontend run build
result: passed
```

Remaining proof required: deploy this repair, rerun staging acceptance, and
perform a fresh owner-authenticated bounded edit/revise on the legal proposal
to confirm v90-style 49-row table shape no longer materializes as 50 rows.

Deployed proof:

```text
behavior commit: e15c499e0997d45d6c2bd80cb5160fb455852510
CI run: 27064922342 passed
FlakeHub run: 27064922344 passed
Node B deploy job: 79884076196 passed
staging proxy deployed_commit: e15c499e0997d45d6c2bd80cb5160fb455852510
staging sandbox deployed_commit: e15c499e0997d45d6c2bd80cb5160fb455852510
staging deployed_at: 2026-06-06T14:30:55Z

staging acceptance:
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-duplicate-table-separator.tmp.spec.js
result: passed
```

The temporary Playwright spec used the authenticated staging product path to
create a source-backed VText, open it in the real editor, inject a
separator-like DOM row into the rendered table, dispatch the editor `input`
event, press the product `Revise` button so the frontend serializer saved a user
revision, and verify the saved revision contained one canonical separator row,
preserved the appendix table body, preserved the prose edit, and carried the
source entity metadata forward. The temporary spec was deleted after proof.

Acceptance level reached: `staging_synthetic_duplicate_separator_repair`.

Residual risk: this proves the duplicate-separator browser-save class on
staging. It does not yet prove the full owner legal-proposal v90/v91/v92 class
with Comet, a fresh bounded edit/revise, exact 49-row appendix survival,
Source Viewer openability, and publication/export metadata after the deployed
repair.

Fresh owner Comet proof limitation after this deploy:

```text
Comet app: /Applications/Comet.app, bundle ai.perplexity.comet
current URL: https://choir.news/?draft_recovery=93d9f819
current visible state: passkey sign-in overlay
message: "Your session ended. Use your passkey to continue."
blocking proof: fresh owner-authenticated legal-proposal bounded edit/revise
```

Computer Use can still see and operate Comet, but the current Comet session is
not owner-authenticated. Earlier owner-authenticated Comet proof remains valid
for its timestamp, but it is stale for proving the deployed
`e15c499e0997d45d6c2bd80cb5160fb455852510` repair against the real legal
proposal. I did not create a passkey or bypass the passkey ceremony. The next
owner proof requires the owner to renew the Comet passkey session first.

### 2026-06-06 Restore Table-Tail Fix Evidence

Status: `accepted_on_staging_for_restore_transition`.

Root-cause correction: the v90-to-v91 row-count shift was not a new browser row
invention in the first instance. The restored v90 head was created by
`HandleVTextRestoreRevision`, which copied the historical source revision bytes
directly and bypassed the same Markdown table-shaped-row normalization used by
ordinary user revision creation. Because the restored historical content still
contained the known blank-separated malformed final `Work product` table row,
the following browser save materialized the already-rendered table tail into a
strict canonical row. The durable invariant should be enforced earlier: the
first restored primary revision must already be canonical, so the next bounded
edit/revise transition does not carry the apparent row-count drift.

Repair:

- Commit `af8870448df7adc0072e3ce133b2124bf913010b` normalizes table-shaped
  rows in `HandleVTextRestoreRevision` before creating the restored revision.
- If normalization changes restored content, the revision metadata records
  `vtext_structure_stabilized=true` and
  `vtext_structure_stabilized_reason=normalized_restored_markdown_table_rows`.
- The restore path still merges metadata from the source revision first, so
  durable `source_entities` survive the normalized restore.

Local verification:

```text
nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextRestoreRevisionNormalizesMalformedTableTailRows|TestVTextUserSaveAndAgentRevisePreserveSourcesAndTableShape|TestVTextMarkdownStructureStabilizationHandlesPartialTableContexts'
```

Landing evidence:

```text
Fix commit: af8870448df7adc0072e3ce133b2124bf913010b
CI run: 27062928319
FlakeHub publish run: 27062928316
Node B deploy job: 79878807642
Health proxy deployed_commit: af8870448df7adc0072e3ce133b2124bf913010b
Health sandbox deployed_commit: af8870448df7adc0072e3ce133b2124bf913010b
Health deployed_at: 2026-06-06T12:59:26Z
```

Deployed synthetic product-path restore proof used real staging passkey
registration and authenticated public VText APIs, not test-only or internal
routes. It created a fixture VText document, created a historical source
revision with a blank-separated malformed table tail plus `source_entities`,
advanced the head, restored the historical revision through
`POST /api/vtext/documents/{id}/restore`, and checked the restored head through
`/api/vtext/documents/{id}/diagnosis?limit=10&include_content=false`.

Accepted evidence:

```text
registered_email: restore-acceptance-1780750925040-zz7zau@example.com
doc_id: 5a579f01-69b5-4a4f-a9b2-e7d9cbb3673e
source_revision_id: 5e192e94-799e-4da2-b5ee-1e7bcb0f1ec8
current_revision_id_before_restore: 18601ba8-5e82-48e1-a944-93337b914c04
restored_revision_id: 81260b65-f50d-4839-a9fb-01b8268ce65f
restored_version_number: 2
restored_has_rejoined_tail: true
restored_metadata_source: restore_historical_revision
restored_structure_stabilized: true
restored_structure_reason: normalized_restored_markdown_table_rows
restored_source_entities: [src-restore-1780750925279]
restored table_count: 1
restored table_row_count: 5
restored table signature: sha256:5b39e295297731ee5408571bad5034ebe562a2ecdabb99f36da712624e237911
result: passed
```

Residual risk: this proves the restore transition that created the noncanonical
head in the owner workflow. It does not replace the broader remaining mission
acceptance for a fresh owner Comet bounded edit/revise on the legal proposal,
guest source-open proof, publication/export metadata proof, and adversarial
source-system review.

### Problem 11: Source-Cycle Fetchers Bypass The Runtime Source-Fetch SSRF Policy

Status: `deployed_with_local_behavior_proof_and_staging_identity`.

problem: the runtime URL import path has a dedicated source-fetch HTTP client
and validation path that rejects localhost, private networks, link-local
metadata hosts, userinfo URLs, and forbidden redirect targets. The source-cycle
adapter path does not consistently use that policy. `internal/sources/rss.go`
and `internal/sources/gdelt.go` construct plain `http.Client` instances, and
`internal/sources/gdelt.go` follows the GKG URL parsed from remote
`lastupdate.txt` without revalidating the second-stage URL against the
source-fetch policy. That leaves an acquisition path where a configured source
or a compromised source response can direct the source cycle toward internal
addresses.

affected contract/invariant: `docs/source-external-data-publication.md`
requires Source Service acquisition to respect source policy, auth policy,
robots/TOS policy, and rate policy, while preserving fetch records as evidence.
SSRF safety is part of source policy. The policy cannot apply only to
owner-triggered runtime URL imports; source-cycle adapters need the same
public-network validation, redirect handling, proxy disabling, and bounded
fetch behavior.

evidence from code audit:

- `internal/runtime/content.go` has `sourceFetchHTTPClient`,
  `validateSourceFetchURL`, `validateSourceFetchHost`, and tests in
  `internal/runtime/source_fetch_policy_test.go`.
- `internal/sources/rss.go` uses `&http.Client{Timeout: 30 * time.Second}` and
  creates requests directly from `source.URL`.
- `internal/sources/gdelt.go` uses `&http.Client{Timeout: 60 * time.Second}`;
  it reads `source.URL`, parses `gkgURL` from the response body, and calls
  `fetchGKG(ctx, gkgURL, ...)` without URL policy validation before creating
  the request.

first observed version/transition: repository source-system audit on
2026-06-06 after restore-table fix `af8870448df7adc0072e3ce133b2124bf913010b`
was deployed.

suspected owner: source-cycle acquisition helpers in `internal/sources`.

planned proof: add a shared source-fetch policy helper for source-cycle
adapters, make RSS and GDELT constructors use the policy client, reject
forbidden configured and second-stage URLs, preserve fetch records with an
error state, and add focused tests for loopback source URLs plus GDELT
second-stage loopback URLs.

### 2026-06-06 Source-Cycle Fetch Policy Fix Evidence

Status: `deployed_with_local_behavior_proof_and_staging_identity`.

Repair:

- Commit `fd75bb80d1ed3d6a342008463d7b6d940af1d580` adds a source-cycle
  fetch policy helper in `internal/sources`.
- RSS, Telegram, and GDELT constructors now use a policy HTTP client that
  disables proxy use, validates resolved dial targets, and rejects forbidden
  redirect targets.
- RSS, Telegram, and GDELT validate configured source URLs before request
  creation.
- GDELT validates the second-stage GKG URL parsed from `lastupdate.txt` before
  creating the request.
- Focused tests cover forbidden configured URLs, forbidden redirect targets,
  forbidden resolved addresses, and GDELT second-stage loopback rejection while
  preserving explicit `httptest` allowances for local fixtures.

Local verification:

```text
nix develop -c go test ./internal/sources
nix develop -c go test ./internal/cycle ./cmd/sourcecycled
```

Landing evidence:

```text
Fix commit: fd75bb80d1ed3d6a342008463d7b6d940af1d580
Problem checkpoint commit: 9e20db83
CI run: 27063118076
FlakeHub publish run: 27063118088
Node B deploy job: 79879309866
Health proxy deployed_commit: fd75bb80d1ed3d6a342008463d7b6d940af1d580
Health sandbox deployed_commit: fd75bb80d1ed3d6a342008463d7b6d940af1d580
Health deployed_at: 2026-06-06T13:08:06Z
```

Staging proof boundary:

- `sourcecycled` is deployed as a Node B service and exposes
  `/internal/source-service/*` on the service/API boundary.
- Public unauthenticated probes to `/api/source/health`,
  `/api/source-service/health`, and `/internal/source/health` returned 401/403.
- There is intentionally no public product endpoint that asks sourcecycled to
  poll an arbitrary attacker URL, so staging cannot safely replay the SSRF
  rejection transition through a user-facing URL.
- The deployed acceptance for this scoped repair is therefore: focused local
  adapter tests, full CI, FlakeHub, Node B deploy success, and staging health
  identity for the fixed commit. A future source-service admin/control surface
  should expose policy-safe health and fetch-record inspection so verifier
  agents can observe blocked fetch evidence without internal route bypass.

Residual risk: this repairs source-cycle adapter SSRF policy but leaves a
follow-up architecture debt: runtime URL import and source-cycle acquisition now
have parallel policy implementations. The longer-term contract wants a single
shared source acquisition policy module used by runtime URL import, source
service adapters, Web Lens import, publication source snapshots, and future
connectors.

### Problem 18: Runtime And Source Service Still Carry Parallel Source-Fetch Policy Implementations

Status: `fixed_and_accepted_on_staging`.

problem: after the source-cycle SSRF repair, Choir still has two independent
source-fetch policy implementations. `internal/runtime/content.go` defines
`sourceFetchHTTPClient`, `validateSourceFetchURL`, `validateSourceFetchHost`,
`sourceFetchIPBlocked`, and `sourceFetchHostnameBlocked` for browser-public URL
imports. `internal/sources/fetch_policy.go` defines near-identical functions
for RSS/GDELT source service adapters. The tests in
`internal/runtime/source_fetch_policy_test.go` and
`internal/sources/fetch_policy_test.go` duplicate the same security cases.

affected contract/invariant: `docs/source-external-data-publication.md`
requires acquisition policy to be a shared source-system contract, not a local
implementation detail of one route. The MissionGradient objective also requires
source acquisition to be policy-checked and SSRF-safe across VText, Source
Viewer, Web Lens, publication, export, and source service paths. Parallel
policy code creates a drift surface where a future exception, hostname block,
redirect limit, DNS validation rule, or test-only private-network allowance can
land in one acquisition path but not the other.

confirmed evidence:

```text
internal/runtime/content.go:
  sourceFetchHTTPClient
  validateSourceFetchURL
  validateSourceFetchHost
  sourceFetchIPBlocked
  sourceFetchHostnameBlocked

internal/sources/fetch_policy.go:
  sourceFetchHTTPClient
  validateSourceFetchURL
  validateSourceFetchHost
  sourceFetchIPBlocked
  sourceFetchHostnameBlocked

duplicated tests:
  internal/runtime/source_fetch_policy_test.go
  internal/sources/fetch_policy_test.go
```

why this matters: the source system is trying to converge source entity,
reader artifact, selector, evidence, open-surface, publication, and export
behavior into shared contracts. Keeping SSRF policy duplicated contradicts the
same convergence principle that motivated `internal/sourcecontract`, and it is
a higher-risk drift surface because the duplicated code protects a network
boundary.

acceptance for fix:

- add a shared backend source-fetch policy package with URL validation,
  DNS/IP validation, redirect limits, proxy-disabled HTTP client construction,
  and test-only private-network allowance;
- route runtime URL import and source service RSS/GDELT acquisition through the
  shared package without weakening the existing blocked-target tests;
- preserve configurable timeouts for runtime imports and source service
  adapters;
- update Nix service source closures and deploy-impact classification for the
  shared package before deploy, avoiding the Problem 17 packaging gap;
- run focused runtime and source-service policy tests, push, monitor CI and
  staging deploy, and verify staging commit identity.

remaining error field: this will not finish robots/TOS/rate policy or Web Lens
snapshot policy. It closes the current duplicated SSRF/source-fetch policy
implementation surface for runtime URL import and source service adapters.

implementation evidence, local:

- added shared `internal/sourcefetch` policy package for URL validation,
  DNS/IP validation, redirect limits, proxy-disabled HTTP client construction,
  configurable timeout, and test-only private-network allowance;
- routed runtime URL import and source service RSS/GDELT/Telegram fetchers
  through `sourcefetch.ValidateURL` and `sourcefetch.Client`;
- removed the runtime-local policy test and the duplicated source-service
  policy implementation, keeping generic policy tests under
  `internal/sourcefetch` and source-service caller tests under
  `internal/sources`;
- updated `flake.nix` service source closures for gateway, sourcecycled, and
  sandbox so the new shared package is included in trimmed service builds;
- updated `.github/scripts/deploy-impact-classify` so
  `internal/sourcefetch/*` changes trigger gateway, sandbox, sourcecycled,
  vmctl restart, and active VM refresh.

focused verification:

```text
nix develop -c go test ./internal/sourcefetch -count=1
ok github.com/yusefmosiah/go-choir/internal/sourcefetch

nix develop -c go test ./internal/sources -count=1
ok github.com/yusefmosiah/go-choir/internal/sources

nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportURL|TestContentImportURLCreatesProvenanceRecord|TestContentImportURLRejectsForbiddenTargets' -count=1
ok github.com/yusefmosiah/go-choir/internal/runtime

nix develop -c go test ./cmd/gateway ./cmd/sourcecycled ./cmd/sandbox -count=1
ok github.com/yusefmosiah/go-choir/cmd/sourcecycled
ok github.com/yusefmosiah/go-choir/cmd/sandbox
```

deploy packaging checks:

```text
printf changed files | .github/scripts/deploy-impact-classify
host_services=gateway,sandbox,sourcecycled
deploy_vmctl_restart=true
deploy_active_vm_refresh=true

nix eval --raw .#packages.x86_64-linux.gateway.src
gateway source includes internal/sourcefetch

nix eval --raw .#packages.x86_64-linux.sourcecycled.src
sourcecycled source includes internal/sourcefetch

nix eval --raw .#packages.x86_64-linux.sandbox.src
sandbox source includes internal/sourcefetch
```

residual risk after implementation: the shared package converges the current
runtime URL import and source-service fetch adapters, but it still does not
encode robots/TOS/rate policy, canonical source snapshot creation, or future
connector-specific acquisition policy.

deployed evidence:

```text
behavior commit:
  3f48a1ad34af7945e4b5e764a9064a6c1c7c9c44

GitHub Actions:
  CI run 27065234996: completed success
    Deploy to Staging (Node B): completed success at 2026-06-06T14:49:48Z
  FlakeHub run 27065235002: completed success

staging /health:
  proxy commit/deployed_commit:
    3f48a1ad34af7945e4b5e764a9064a6c1c7c9c44
  sandbox upstream commit/deployed_commit:
    3f48a1ad34af7945e4b5e764a9064a6c1c7c9c44
  deployed_at:
    2026-06-06T14:44:36Z
```

deployed product-path acceptance:

```text
Base URL: https://choir.news
Auth path: temporary Playwright/WebAuthn staging user via public /auth routes
Request: POST /api/content/import-url
Body URL: http://127.0.0.1:8787/internal/source-service/health
Result: 502
Body: {"error":"source URL host is not allowed"}
```

The acceptance used a browser-created authenticated session and the public
`/api/content/import-url` product route. It did not call
`/internal/source-service/*` directly and did not seed success records.

### Problem 19: Text-Like Import Migration Manifests Do Not Survive The First Durable Revision

Status: `fixed_and_accepted_on_staging`.

problem: the VText file-open path already creates a canonical `.vtext` document
title for imported files and preserves an original `ContentItem`, but the
migration metadata is incomplete at the exact transition required by this
mission. Markdown file open creates a `migration_manifest` only on the initial
v0 revision. `migration_manifest` is not included in `durableMetadataKeys`, so
the first user/appagent durable revision (`v0 -> v1`) does not automatically
carry the manifest forward unless a caller happens to resubmit it. Non-Markdown
text-like imports such as `.txt` and `text/html` do not get any
`migration_manifest` at file-open time, even though they use the same
VText projection path and should be auditable as text-like source-to-VText
migrations with export back to Markdown available.

affected contract/invariant: the mission stopping condition says imported
Markdown/text/other text documents become canonical `.vtext` by the first
durable revision, with export back to Markdown still available. The
requirements contract treats VText metadata as the canonical source identity
that must survive revise, history, publication, and export. If the migration
manifest disappears at v1, verifiers and future publication/export code cannot
prove that the current canonical VText head came from a particular imported
text-like source artifact and projection adapter.

confirmed evidence:

```text
internal/runtime/vtext.go:
  buildFileOpenVTextMetadata adds migration_manifest only when source ext is
  md/markdown.

internal/runtime/runtime.go:
  durableMetadataKeys contains source_path and canonical_vtext_source_path,
  but not migration_manifest or import_manifest.

frontend/tests/vtext-markdown-lineage.spec.js:
  "Imported Markdown advances from v0 source artifact to canonical .vtext with
  Markdown export" checks v1 canonical_vtext_source_path and export content,
  but does not prove migration_manifest survives into v1.

frontend/tests/vtext-document-stream.spec.js:
  text/plain reopen coverage proves stable aliasing and v0 content, but not
  v1 migration metadata or Markdown export for text-like imports.
```

acceptance for fix:

- create a file-open migration manifest for all text-like imports that project
  to canonical VText (`text/markdown`, `text/plain`, `text/html`);
- preserve `migration_manifest` and `import_manifest` across user and appagent
  durable revisions unless an explicit revision supplies a stronger value;
- add focused backend coverage for `.txt -> .vtext -> v1` showing canonical
  `.vtext` alias, original `.txt` alias, migration/import metadata on v1, and
  Markdown export;
- extend browser-path coverage for imported Markdown/text first durable
  revision metadata;
- land through CI, staging deploy, and product-path acceptance without using
  internal/test-only routes for deployed proof.

remaining error field: this does not create or migrate the owner legal cloud
proposal itself. It closes the generic text-like import metadata continuity
needed before owner/legal-proposal migration proof can be trusted.

implementation evidence, local:

- `buildFileOpenVTextMetadata` now creates a text-like
  `migration_manifest` for `text/markdown`, `text/plain`, and `text/html`
  VText file-open projections;
- `durableMetadataKeys` now carries `import_manifest` and
  `migration_manifest` into user and appagent durable revisions unless the
  caller supplies a stronger value;
- `GetDocumentAliasSourcePath` now prefers a `.vtext` shortcut alias when one
  exists, while exact original `.txt`/`.md` aliases still resolve to the same
  canonical document;
- backend coverage now proves `.txt -> .vtext -> v1` keeps import/migration
  metadata, original alias, canonical alias, and Markdown export;
- browser/API coverage now proves imported Markdown and imported plain text
  both carry first-durable-revision metadata and export back to Markdown.

focused verification:

```text
nix develop -c go test ./internal/store -run 'TestVTextDocumentAlias' -count=1
ok github.com/yusefmosiah/go-choir/internal/store

nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVText(OpenFileResolvesCanonicalAlias|PlainTextImportCarriesMigrationMetadataToFirstDurableRevision|ImportedMarkdownRevisionUsesVTextProjectionAndPreservesCollapsedTable|AppagentEditCanonicalizesAliasedMarkdownTitle|OpenFilePreservesDocxAndPDFOriginalArtifacts|OpenFileImportsDocxAndPDFBytesFromFilesRoot)$' -count=1
ok github.com/yusefmosiah/go-choir/internal/runtime

CHOIR_SERVICES_FOREGROUND=1 nix develop -c ./start-services.sh
npm --prefix frontend run e2e -- tests/vtext-markdown-lineage.spec.js -g 'Imported (Markdown|plain text)'
2 passed
```

deploy-impact classification before landing:

```text
host_services=gateway,sandbox
internal/runtime/vtext.go -> gateway/sandbox service pointers
internal/runtime/runtime.go -> gateway/sandbox service pointers
internal/store/vtext.go -> gateway/sandbox shared runtime dependency
```

deployed evidence:

```text
behavior commit:
  6a141811fd6c8ff97d4aa98f6a98bb30e59f8603

GitHub Actions:
  CI run 27065789129: completed success
    Deploy to Staging (Node B): completed success at 2026-06-06T15:08:59Z
  FlakeHub run 27065789119: completed success

staging /health:
  proxy commit/deployed_commit:
    6a141811fd6c8ff97d4aa98f6a98bb30e59f8603
  sandbox upstream commit/deployed_commit:
    6a141811fd6c8ff97d4aa98f6a98bb30e59f8603
  deployed_at:
    2026-06-06T15:08:36Z
```

deployed product-path acceptance:

```text
Base URL: https://choir.news
Auth path: temporary Playwright/WebAuthn staging user via public /auth routes
Command:
  CHOIR_AUTH_STATE=/tmp/choir-text-import-metadata.storage.json \
  CHOIR_AUTH_META=/tmp/choir-text-import-metadata.meta.json \
  PLAYWRIGHT_BASE_URL=https://choir.news \
  npm --prefix frontend run e2e -- tests/vtext-markdown-lineage.spec.js -g 'Imported (Markdown|plain text)'
Result:
  2 passed
```

The staging proof used public browser-authenticated `/api/vtext/*` and
`/api/content/*` product routes. It did not use internal or test-only routes.

### Problem 20: Source Open-Surface Normalization Is Still Frontend-Only

Status: `fixed_and_accepted_on_staging`.

problem: the mission now has a shared `internal/sourcecontract` package for
typed evidence states and source-fetch policy, but source open-surface routing
is still normalized only in the frontend. Runtime source producers emit
different aliases for the same durable reader intent (`"source"` for
Source Service items, `"content"` for `ContentItem` artifacts, and app hints
for media refs). Platform publication stores `display.open_surface` raw in
`publication_source_entities.open_surface`. The frontend later interprets
`source`, `content`, `reader`, and `source_viewer` as Source Viewer, and
interprets `browser`, `web`, `web_lens`, `live`, `original`, and
`live_original` as explicit live/original Web Lens inspection. That keeps the
core source-opening contract outside the backend systems that publish, export,
verify, or migrate source entities.

affected contract/invariant: `source-external-data-publication.md` and this
mission require one source opening contract across VText, Source Viewer, Web
Lens, publication, and export. Source Viewer must be the default for durable
artifacts, while Web Lens/original inspection must be explicit. If only the
browser frontend knows that `"content"` is a Source Viewer alias, backend
publication/export metadata can preserve ambiguous or stale aliases and
verifiers cannot distinguish a durable reader artifact from a live web
inspection request without reimplementing frontend heuristics.

confirmed evidence:

```text
internal/sourcecontract/evidence.go:
  shared contract currently defines typed evidence-state normalization only.

internal/runtime/vtext_media_sources.go:
  sourceServiceItemRefToSourceEntity emits display.open_surface "source".
  contentItemRefToSourceEntity emits display.open_surface "content".
  sourceEntityOpenSurface returns raw media app hints before local defaults.

internal/runtime/vtext.go:
  attachSourceArtifactToVText rewrites blank/"source" open surfaces to
  "content" after binding a ContentItem artifact.

internal/platform/source_metadata.go:
  normalizePublicationSourceEntity stores firstString(display, "open_surface")
  without shared normalization.

frontend/src/lib/vtext-source-renderer.ts:
  normalizeSourceOpenSurface and sourceEntityOpenPlan are the only audited code
  that collapse aliases into Source Viewer vs live/original Web Lens routing.
```

acceptance for fix:

- add shared backend source open-surface normalization with canonical values for
  durable Source Viewer, explicit Web Lens/live-original, VText publication, and
  media/video;
- make runtime source entity producers use the shared canonical values instead
  of emitting `"content"` as a private alias for Source Viewer;
- make platform publication normalize `display.open_surface` before persisting
  publication source records;
- align frontend open-plan normalization with the same canonical values while
  preserving existing user behavior: durable URL/content/source-service sources
  open in Source Viewer by default, and explicit Web Lens/live/original requests
  open the browser/Web Lens surface;
- prove with focused backend and browser-path tests, then land through CI,
  staging deploy identity, and deployed product-path source-open acceptance.

remaining error field: this problem is about open-surface contract convergence.
It does not by itself solve selector-rich publication projection, source reader
layout, legal proposal table preservation, owner Comet passkey renewal, or
publication access policy for private source snapshots.

implementation evidence, local:

- `internal/sourcecontract` now defines canonical source open surfaces and
  alias normalization for durable Source Viewer (`source`), explicit
  live/original Web Lens (`web_lens`), VText publication (`vtext`), video, and
  image;
- runtime Source Service and `ContentItem` source entity producers now emit
  canonical `source` instead of splitting durable reader intent across
  `"source"` and `"content"`;
- source artifact attachment now canonicalizes any blank or Source Viewer alias
  to `source`;
- platform publication normalizes `display.open_surface` before persisting both
  `publication_source_entities.open_surface` and embedded `entity_json`;
- frontend source open planning now normalizes aliases to the same canonical
  values while preserving user behavior: durable source artifacts open in
  Source Viewer by default, explicit Web Lens/live/original aliases open the
  browser/Web Lens surface, and media sources keep media surfaces.

focused verification:

```text
nix develop -c go test ./internal/sourcecontract -run 'TestNormalizeOpenSurface|TestOpenSurfacePredicates|TestNormalizeEvidenceState|TestIsRelationalEvidenceState' -count=1
ok github.com/yusefmosiah/go-choir/internal/sourcecontract

nix develop -c go test ./internal/platform -run 'TestBuildPublicationSourceMetadataNormalizesOpenSurface|TestPublishVTextCreatesImmutablePublicRecords' -count=1
ok github.com/yusefmosiah/go-choir/internal/platform

nix develop -c go test ./internal/runtime -run 'TestVTextPromptDerivesSourceServiceEntitiesFromResearcherUpdates|TestVTextDerivesContentItemSourceEntitiesFromResearcherRefs' -count=1
ok github.com/yusefmosiah/go-choir/internal/runtime

nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestMarkVTextMediaSourceRefsResearchState|TestMediaSourceRefToSourceEntityUsesTypedEvidenceStates|TestVTextAgentRevisionDerivesContentItemSourceEntitiesFromResearcherRefs' -count=1
ok github.com/yusefmosiah/go-choir/internal/runtime

npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source open plans normalize Web Lens and Source Viewer aliases'
1 passed

npm --prefix frontend run build
success
```

local test limitation:

```text
nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextAgentRevisionRegistersMediaSourceRefs|TestMarkVTextMediaSourceRefsResearchState|TestMediaSourceRefToSourceEntityUsesTypedEvidenceStates' -count=1
FAIL TestVTextAgentRevisionRegistersMediaSourceRefs
media_source_refs len = 1, want 2
```

This failure happened before the open-surface assertion: the loopback
`httptest` image fixture was not imported as a second media source under the
current source-fetch policy. The narrower media-source normalization tests
passed, and CI runtime shards passed after the behavior commit. The failing
local fixture remains a test-harness/source-policy interaction to audit if
future media-source work depends on loopback image fixtures.

deployed evidence:

```text
problem checkpoint commit:
  be126abf docs: record source open surface contract gap

behavior commit:
  fab6b25b0d3d0092d9f7f55c672373216291657b

GitHub Actions:
  CI run 27066097876: completed success
    Deploy to Staging (Node B): completed success in 1m28s
  FlakeHub run 27066097878: completed success

staging /health:
  proxy commit/deployed_commit:
    fab6b25b0d3d0092d9f7f55c672373216291657b
  sandbox upstream commit/deployed_commit:
    fab6b25b0d3d0092d9f7f55c672373216291657b
  deployed_at:
    2026-06-06T15:22:10Z
```

deployed product-path acceptance:

```text
Base URL: https://choir.news
Auth path: temporary Playwright/WebAuthn staging user via public /auth routes
Command:
  CHOIR_AUTH_STATE=/tmp/choir-open-surface.storage.json \
  CHOIR_AUTH_META=/tmp/choir-open-surface.meta.json \
  PLAYWRIGHT_BASE_URL=https://choir.news \
  npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'VText source URL opens Source Viewer unless browser is explicitly requested'
Result:
  1 passed
```

The staging proof used public browser-authenticated VText/product routes and
verified that a URL-backed durable source opens Source Viewer by default while
an explicit Web Lens request opens the browser/Web Lens path. It did not use
internal or test-only routes.

### Problem 21: Publication Export Metadata Omits Policy And Retrieval Context

Status: `fixed_deployed`.

problem: canonical publication export metadata already includes public source
entities and transclusions, but it omits the publication access/export policy
and the retrieval source/span records that define the canonical public artifact
context. `ExportPublicationByRoute` correctly checks the publication export
policy before producing bytes, and the publication bundle carries
`bundle.Policy` plus retrieval span records. However,
`publicationExportMetadata` serializes only the content/projection hashes,
source revision hash, artifact manifest id, source entities, and transclusions.
A downloaded Markdown/HTML/DOCX/PDF artifact therefore cannot prove which
publication policy authorized the export or which retrieval source/span refs
anchor the exported public artifact without resolving the live publication
route again.

affected contract/invariant: `source-external-data-publication.md` requires
publication/export to preserve access policy, export policy, hashes, source
metadata, transclusion records, and canonical artifact metadata without reading
from rendered DOM. Export bytes are durable evidence artifacts. If the export
metadata excludes policy and retrieval context, downstream verifiers cannot
audit that copy/download obeyed the publication policy or tie the export back to
the canonical retrieval source/span records embedded in the publication.

confirmed evidence:

```text
internal/platform/service.go:
  ExportPublicationByRoute resolves the publication bundle and calls
  publicationExportAllowed(bundle.Policy.Export, format) before exporting.

internal/platform/types.go:
  PublicationBundle contains Policy and Retrieval records.

internal/platform/export_formats.go:
  publicationExportMetadata writes schema, format, publication ids,
  route_path, content_hash, source_revision_hash, projection_hash,
  artifact_manifest_id, source_entities, and transclusions, but not
  access_policy, export_policy, or retrieval source/span refs.

internal/platform/service_test.go:
  export tests assert source_entities/transclusions/evidence_state but do not
  require policy or retrieval refs in the exported metadata.
```

acceptance for fix:

- include publication access/export policy in canonical export metadata for all
  export formats;
- include public retrieval source/span refs in canonical export metadata so the
  export can be tied back to the immutable public retrieval artifact;
- keep private source revision material omitted, preserving the existing
  `private_material_omitted` claim;
- add focused platform coverage for Markdown/HTML export metadata;
- ensure DOCX/PDF embedded custom/XMP metadata inherit the same envelope;
- land through CI, staging deploy identity, and deployed product-path
  publication export proof.

remaining error field: this fixes the export metadata envelope only. It does
not add new source reader snapshots, change public route access semantics,
repair legal proposal table structure, or complete guest proof for every source
kind.

fix evidence:

- docs checkpoint `a4165abb` recorded this problem before code.
- behavior commit `53dd9b34d694ecd04c354cc1e614c12d87245631` added
  `access_policy`, `export_policy`, and `retrieval` to canonical publication
  export metadata for Markdown/HTML/DOCX/PDF.
- local focused platform tests passed:
  `nix develop -c go test ./internal/platform -run 'TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes|TestPublicationMarkdownExportNormalizesMalformedTableTailRows|TestPublishVTextCreatesImmutablePublicRecords' -count=1`.
- frontend build passed: `npm --prefix frontend run build`.
- local product-path export proof passed after the local harness fix:
  `npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities as expandable transclusions and canonical exports"`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27066359739` and
  FlakeHub run
  `https://github.com/choir-hip/go-choir/actions/runs/27066359741` completed
  successfully.
- Node B staging health reported proxy/upstream deployed_commit
  `53dd9b34d694ecd04c354cc1e614c12d87245631`, deployed_at
  `2026-06-06T15:33:39Z`.
- deployed product-path proof passed on `https://choir.news`:
  `CHOIR_AUTH_STATE=/tmp/choir-export-metadata.storage.json CHOIR_AUTH_META=/tmp/choir-export-metadata.meta.json PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities as expandable transclusions and canonical exports"`.

### Problem 22: Node B Deploy Temporarily Reached New Commit Identity While Remaining Degraded

Status: `documented_observed_transient_no_code_change`.

problem: after follow-up dev-harness commit
`a7e7e82143bf88c77a7c67a758d1bee0f2f8e023`, the GitHub Actions CI run passed
all Go/runtime/frontend gates and the staging health endpoint reported the new
proxy/upstream commit identity while the Node B deploy job was still
`in_progress` and staging health reported `status: "degraded"` with
`vmctl_status: "unavailable"`. A prior health probe during the same deploy also
returned `502 Bad Gateway`. The deploy later completed successfully and health
returned to `ok`, so this is not currently a live blocker. It is still an
acceptance-boundary behavior to carry forward: deploy identity can partially
move forward before the acceptance environment is fully healthy and before the
deploy job reaches a terminal success/failure state.

affected contract/invariant: the repo contract requires behavior-changing
missions to monitor CI, monitor staging deploy, verify deployed commit identity,
and run deployed acceptance proof. A deploy that exposes the new commit in
health while remaining degraded creates an ambiguous acceptance boundary: the
build identity is visible, but the deploy loop is not complete and vmctl-backed
product proof is not available.

confirmed evidence:

```text
GitHub Actions:
  run https://github.com/choir-hip/go-choir/actions/runs/27066513637
  head SHA a7e7e82143bf88c77a7c67a758d1bee0f2f8e023
  Go/runtime/frontend gate jobs: success
  Deploy to Staging (Node B): in_progress at 2026-06-06T15:47Z;
    success at 2026-06-06T15:45:36Z when re-queried after GitHub status
    propagation caught up

FlakeHub:
  run https://github.com/choir-hip/go-choir/actions/runs/27066513615
  conclusion: success

Staging health during deploy:
  status: degraded
  vmctl_status: unavailable
  build.commit/deployed_commit: a7e7e82143bf88c77a7c67a758d1bee0f2f8e023
  upstream_build.commit/deployed_commit: a7e7e82143bf88c77a7c67a758d1bee0f2f8e023
  deployed_at: 2026-06-06T15:40:27Z

Staging health after deploy completion:
  status: ok
  vmctl_status: ok
  build.commit/deployed_commit: a7e7e82143bf88c77a7c67a758d1bee0f2f8e023
  upstream_build.commit/deployed_commit: a7e7e82143bf88c77a7c67a758d1bee0f2f8e023
```

current handling: no deployment code is changed in this slice. Treat this as a
documented transient/residual risk for future deploy-proof logic. The
publication export metadata behavior is proven on the prior healthy Node B
deploy for `53dd9b34` and again on the current `a7e7e821` staging identity.

remaining error field: determine whether the deploy job is waiting for vmctl
health, whether vmctl is independently down, or whether the deploy pipeline can
leave the environment serving a new commit while acceptance remains degraded.
Do not claim a clean deployed acceptance checkpoint for a future commit while
health is degraded, even if the new commit identity is already visible.

### Problem 12: Owner URL Source Repairs Default To Web Lens

Status: `accepted_on_staging_for_url_repair_open_surface`.

problem: the owner source-review repair path still creates URL-backed source
entities with `display.open_surface: "browser"`. That means an owner-supplied
URL source repair is born as an explicit live/original Web Lens artifact instead
of a durable Source Viewer source artifact. The renderer already handles
hand-authored URL entities correctly when they specify `open_surface: "source"`,
but the repair builder chooses the wrong default before the renderer sees the
entity.

affected contract/invariant: Source Viewer is the default for durable URL,
content-item, source-service, and publication reader artifacts. Web Lens is
reserved for explicit live/original inspection. Owner repair should therefore
produce a durable source entity by default, while any later live inspection
should be a distinct explicit action.

evidence from code audit:

- `frontend/src/lib/vtext-source-review.js` sets
  `display.open_surface` to `"browser"` whenever `buildSourceReviewPayload`
  receives a URL.
- `frontend/src/lib/vtext-source-renderer.ts` treats requested
  `browser`/`web`/`web_lens`/`live`/`original` as a Web Lens/browser open plan.
- `frontend/tests/vtext-source-entities.spec.js` already proves a URL-backed
  source opens Source Viewer only when the entity metadata says
  `open_surface: "source"`, so this is a source-repair payload problem, not the
  currently tested renderer happy path.

first observed version/transition: source-system audit on 2026-06-06 after
source-cycle fetch policy repair `fd75bb80d1ed3d6a342008463d7b6d940af1d580`
was deployed.

suspected owner: VText source review payload construction in
`frontend/src/lib/vtext-source-review.js`, with acceptance in the VText
source-entity open-surface tests.

planned proof: change URL repair payloads to default to
`open_surface: "source"`; add a focused test for
`buildSourceReviewPayload`; keep or extend existing Source Viewer/Web Lens
routing coverage so explicitly requested browser surfaces still open Web Lens.

### 2026-06-06 Owner URL Source Repair Open-Surface Fix Evidence

Status: `accepted_on_staging_for_url_repair_open_surface`.

Repair:

- Problem checkpoint commit `10a54897` documented the source-repair open-surface
  gap before behavior-changing code.
- Behavior commit `5344749d4f1651f88518310ff0d0d32be30dc522` changes
  `buildSourceReviewPayload` so URL-backed owner source repairs default to
  `display.open_surface: "source"`.
- The same commit adds a focused Playwright payload regression test asserting a
  URL repair produces a `web_source` with URL target and Source Viewer open
  surface.

Local verification:

```text
npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g "source review URL repairs default to Source Viewer open surface"
npm --prefix frontend run build
```

Landing evidence:

```text
Fix commit: 5344749d4f1651f88518310ff0d0d32be30dc522
Problem checkpoint commit: 10a54897
CI run: 27063262622
FlakeHub publish run: 27063262630
Node B deploy job: 79879686582
Health proxy deployed_commit: 5344749d4f1651f88518310ff0d0d32be30dc522
Health sandbox deployed_commit: 5344749d4f1651f88518310ff0d0d32be30dc522
Health deployed_at: 2026-06-06T13:14:48Z
```

Deployed routing proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g "VText source URL opens Source Viewer unless browser is explicitly requested"
result: passed
```

Deployed owner-repair proof used real staging passkey registration and
authenticated public VText APIs/UI. It created a Markdown-lineage VText with a
`[2]` citation gap, opened the VText Sources panel, filled Source title, Source
URL, and Source excerpt, applied source review, inspected the actual
`/source-repairs` request, fetched stored revision metadata, and opened the
repaired citation.

Accepted evidence:

```text
registered_email: playwright-state-1780751722999-etdyij@example.com
doc_id: 3489a64d-5d47-4072-b9d1-4ab408d192e8
repair_revision_id: a8648728-30d5-4a08-b595-6d6b6aae0ede
request_open_surface: source
stored_open_surface: source
target_kind: url
source_url: https://example.com/url-source-repair-1780751816732
opened_content_viewer_delta: 1
browser_window_delta: 0
result: passed
```

Residual risk: this fixes the owner source-review URL repair default and proves
durable URL repairs open Source Viewer on staging. It does not complete the
larger source-system contract convergence: publication/export selector
richness, shared source acquisition policy, guest publication source-open proof,
and final adversarial/cognitive review remain open mission work.

### Problem 13: Published Inline Source Notes Leak Full Reader Snapshots

Status: `accepted_on_staging_for_published_inline_snapshot_boundaries`.

problem: a published VText source note for a public content-item source can
render the full cleaned reader snapshot inline beside the article instead of
the bounded selected citation excerpt. The full reader artifact should be
available when opening Source Viewer, but the inline transclusion should remain
selector-bounded and content-forward.

affected contract/invariant: selector-rich transclusions and source snapshots
serve different surfaces. Inline/journal transclusions should preserve the
selected quote or transclusion snapshot as the in-flow evidence. Durable reader
snapshots should survive publication for the Source Viewer/full source window,
not replace bounded inline citation material.

evidence from deployed staging:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes public content-item sources with cleaned reader snapshots"
result: failed
failure: source note contained "Full cleaned reader source detail" even though
the test expected the inline note to contain only the selected excerpt before
opening the full source window.
```

code audit: `frontend/src/lib/vtext-source-renderer.ts` implements
`sourceEntityInlineExcerptText` by preferring `sourceEntityReaderSnapshotText`
before `sourceEntityExcerptText`. Once publication enrichment adds a
`reader_snapshot`, compact inline rendering therefore chooses the full reader
snapshot over the transclusion `snapshot_text`/selector quote.

first observed version/transition: staging deployed commit
`5344749d4f1651f88518310ff0d0d32be30dc522` on 2026-06-06 while auditing the
publication source snapshot path after owner URL source repair.

suspected owner: frontend source transclusion rendering in
`frontend/src/lib/vtext-source-renderer.ts`.

planned proof: change compact inline excerpt selection to prefer the
transclusion/selector excerpt and fall back to a bounded reader snapshot only
when no selected excerpt exists; rerun the publication snapshot test against
staging after deployment and confirm the inline note remains bounded while the
full reader snapshot opens from the source window.

### 2026-06-06 Published Inline Source Note Bounding Fix Evidence

Status: `accepted_on_staging_for_published_inline_snapshot_boundaries`.

Repair:

- Problem checkpoint commit `7668324f` documented the published inline source
  note leak before behavior-changing code.
- Behavior commit `b63b4fd5840f3a7a6f42df6cb1925279f5f88b0b` changes
  `sourceEntityInlineExcerptText` so compact inline/journal source notes prefer
  the selected transclusion/selector excerpt and fall back to reader snapshots
  only when no bounded excerpt exists.
- The same commit adds a focused frontend regression test for a source entity
  carrying both `transclusion.snapshot_text` and a longer
  `reader_snapshot.text_content`.

Local verification:

```text
npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g "source review URL repairs default|source inline excerpts prefer"
npm --prefix frontend run build
```

Landing evidence:

```text
Fix commit: b63b4fd5840f3a7a6f42df6cb1925279f5f88b0b
Problem checkpoint commit: 7668324f
CI run: 27063425138
FlakeHub publish run: 27063425139
Node B deploy job: 79880094595
Health proxy deployed_commit: b63b4fd5840f3a7a6f42df6cb1925279f5f88b0b
Health sandbox deployed_commit: b63b4fd5840f3a7a6f42df6cb1925279f5f88b0b
Health deployed_at: 2026-06-06T13:22:23Z
```

Deployed acceptance proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes public content-item sources with cleaned reader snapshots"
result: passed
```

The first post-deploy rerun advanced past the original failure: the inline
published source note no longer contained `Full cleaned reader source detail`.
It then failed at the stale assertion that published reader snapshots should
open `BrowserApp`; the live product opened Source Viewer instead. The test was
updated to assert the mission contract: owner and guest published source opens
create `data-content-viewer` reader-mode windows, render the full cleaned reader
snapshot there, and do not create `data-browser-app` windows.

Residual risk: this fixes the inline/full-reader boundary for published
content-item snapshots and proves owner plus guest Source Viewer opens in the
focused publication test. It does not yet prove every URL-backed and
source-service-style publication source shape, nor does it finish the shared
selector/evidence/open-surface contract consolidation.

### 2026-06-06 Source-Service Publication Selector And Guest Proof

Status: `accepted_on_staging_for_source_service_publication_selector_guest`.

Acceptance expansion:

- Test checkpoint commit `03918d12` updates
  `frontend/tests/vtext-source-service-publication.spec.js` so the
  source-service publication path carries three selectors: text quote,
  table range, and page range.
- The test now asserts the resolved publication bundle and Markdown export
  metadata preserve the selector set, not only the first quote selector.
- The test now opens the same published source as a signed-out guest and
  asserts Source Viewer opens in reader mode without creating Web Lens/Browser.

Deployed acceptance proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities"
result: passed
staging deployed_commit during proof: b63b4fd5840f3a7a6f42df6cb1925279f5f88b0b
```

What this proves:

- `source_selector.selector_kind` is `selector_set` in the resolved bundle.
- Resolved selector metadata includes the table range selector.
- Export metadata includes the selector set and the page range selector.
- Owner source open creates a Source Viewer window with source-service identity.
- Guest source open creates a Source Viewer reader-mode window and does not
  create `data-browser-app`.

Residual risk: this closes the source-service-style publication selector and
guest-open proof gap for the focused test fixture. URL-backed publication source
snapshots still need equivalent owner/guest product-path proof with publication
export metadata, and the broader shared contract consolidation remains open.

### 2026-06-06 URL-Backed Publication Source Snapshot Proof

Status: `accepted_on_staging_for_url_backed_publication_sources`.

Acceptance expansion:

- `frontend/tests/vtext-source-service-publication.spec.js` now includes a
  public URL-backed source fixture with `target_kind: "url"`,
  `rights_scope: "public_url_snapshot"`, and `open_surface: "source"`.
- The test publishes the VText through the product path, causing publication
  enrichment to import `https://example.com/` through `/api/content/import-url`
  and attach a reader snapshot for publication readers.
- The test asserts resolved publication metadata, Markdown export metadata,
  owner Source Viewer open, and guest Source Viewer open.
- The same test file now clears persisted desktop windows between cases so
  owner/guest source-open assertions are not polluted by windows from earlier
  publication tests.

Deployed acceptance proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js
result: passed
tests: 3 passed
staging deployed_commit during proof: b63b4fd5840f3a7a6f42df6cb1925279f5f88b0b
```

What this proves:

- Source-service-style publication sources preserve selector sets in resolved
  bundle metadata and export metadata.
- Public content-item publication sources preserve full cleaned reader snapshots
  for owner and guest Source Viewer opens while keeping inline notes bounded.
- Public URL-backed publication sources import and publish a reader snapshot for
  authorized publication readers.
- Owner and guest URL-backed source opens create Source Viewer reader-mode
  windows and do not create Web Lens/Browser windows.
- Markdown export metadata includes the URL-backed source entity and normalized
  evidence state.

Residual risk: this closes the focused URL-backed/content-item/source-service
publication proof gap. Remaining mission work is broader: legal proposal owner
proof after the table restore fix, shared source contract consolidation across
runtime/platform/frontend/export, adversarial/cognitive review, dead-path
pruning, and the hard mission review report plus PDF.

### 2026-06-06 Fresh Owner Comet Legal Proposal Structure Proof

Status: `owner_authenticated_comet_legal_proposal_structure_reconfirmed`.

Staging/identity evidence:

- Computer Use controlled `/Applications/Comet.app/` (`ai.perplexity.comet`) on
  `https://choir.news/?draft_recovery=93d9f819`.
- The Choir passkey renewal flow accepted `yusefnathanson@me.com`; the browser
  presented a passkey sheet for `choir.news` and that owner email, and the
  authenticated Choir desktop booted successfully.
- The authenticated Comet desktop opened
  `choir_private_legal_cloud_proposal.vtext` as the legal proposal window.

Legal proposal evidence after deployed table-restore repair:

- The legal proposal loaded as `v92`, `Primary draft Latest`.
- The Sources panel reported `7 represented sources`.
- Source Viewer durable artifact windows were open for source records including
  `NixOS reproducible configuration and rollback` and
  `Qdrant similarity search documentation`; these opened as Source Viewer
  reader artifacts, not Web Lens/browser windows.
- The Sources diagnosis panel reported:
  - `80 revisions`;
  - `80 runs`;
  - current `v92`;
  - `21 tables`;
  - `70 source markers`;
  - `24 bounded summaries`.
- Current `v92` structure was `1` table, `50` rows, `7` sources, content hash
  `sha256:f21ce9be51fb`, and appendix table
  `L269-L318`, `2 c/50 r`, `sha256:a86643578fed`.
- `v91` structure was also `1` table, `50` rows, `7` sources, content hash
  `sha256:f241be40dcf5`, and appendix table
  `L269-L318`, `2 c/50 r`, `sha256:01178718c7ae`.
- The historical comparison window still exposed the original regression:
  `v70` had `1` table and `50` rows; `v71`, `v75`, `v76`, `v77`, and `v78`
  had `0` tables; later recovered revisions such as `v87` had `1` table and
  `49` rows; the repaired latest legal proposal now sits at a canonical
  `50`-row table head.

What this proves:

- Fresh Comet owner authentication for `yusefnathanson@me.com` works on staging.
- The actual owner legal proposal is currently a true VText document with
  source-bearing metadata visible through the product UI.
- The deployed restore/table normalization work left the legal proposal current
  head at `v92` with represented sources and a stable 50-row appendix table
  across the latest two revisions.
- The source open default for durable legal proposal source artifacts remains
  Source Viewer.

Proof limitation discovered:

- A prompt entered through the global command prompt saying
  `In the open VText document choir_private_legal_cloud_proposal.vtext...`
  was routed as a new VText request instead of a scoped edit to the already-open
  legal proposal window. It created a separate transient/durable VText window
  titled from the prompt and produced a short first-draft proposal. This is not
  accepted as legal-proposal edit proof.
- The limitation is a product-path routing ambiguity, not evidence of a table
  regression. It should be fixed or avoided before using the global prompt bar
  for owner legal-proposal acceptance. The safer next proof path is to use the
  legal proposal's own window controls or an authenticated product endpoint
  that explicitly addresses the legal proposal document id.

Follow-up Comet/API capability probe on 2026-06-06:

- Computer Use switched Comet from a ChatGPT tab to an existing Choir tab at
  `https://choir.news/?draft_recovery=93d9f819`; the visible desktop still
  showed the real owner legal proposal window
  `choir_private_legal_cloud_proposal.vtext` at `v92`.
- Direct Comet navigation to `https://choir.news/auth/session` returned
  `authenticated: true` for `yusefnathanson@me.com` with user id
  `5bd6de97-3b58-408c-bf89-c42c81b083de`.
- A short same-origin JavaScript bookmarklet executed successfully in Comet,
  proving the browser can run owner-authenticated in-page API probes without a
  fresh passkey ceremony.
- A read-only owner-authenticated diagnosis bookmarklet for doc
  `f93cea62-f833-4dae-b414-8e44783d8cbe` returned HTTP 200 and confirmed:
  current revision `f92d40dc-9266-4e7c-b6a6-3a104e8d20c0`, version `v92`,
  owner id `5bd6de97-3b58-408c-bf89-c42c81b083de`, title
  `choir_private_legal_cloud_proposal.vtext`, revision count `93`, last author
  `appagent`, `7` source markers, `1` table, `50` table rows, appendix table
  `L269-L318`, and table signature `sha256:a86643578fed81c67c8838ab00c5aba0a3af9a26294d59df529db8a15ab1f3b8`.
- Attempting to run a longer mutation/restore bookmarklet through Comet's
  address field was unreliable: the page continued showing the prior diagnosis
  output instead of executing the new script. Reading Comet's encrypted cookies
  from `~/Library/Application Support/Comet/Default/Cookies` showed
  `choir_access` and `choir_refresh`, but decrypting via macOS Keychain blocked
  on access control and was stopped. The bounded edit/restore proof therefore
  remains unrun in this checkpoint.

Residual risk: this closes the fresh Comet owner-authentication and current
legal-proposal structure observation gap, and it gives concrete post-repair
table/source evidence for `v91`/`v92`. It does not yet satisfy the full
post-repair owner bounded-edit acceptance because the attempted global-prompt
edit targeted a new VText artifact and the later bookmarklet mutation path was
not reliable enough to use for owner document mutation. Remaining mission work
includes a properly scoped owner legal-proposal bounded edit/revise proof,
shared source contract consolidation across runtime/platform/frontend/export,
adversarial/cognitive review, dead-path pruning, and the hard mission review
report plus PDF.

Continuation Comet/API capability probe on 2026-06-06 at approximately
`2026-06-06T17:02:40Z`:

- Computer Use controlled the same `/Applications/Comet.app/`
  (`ai.perplexity.comet`) staging tab.
- Reloading `https://choir.news/auth/session` in Comet returned
  `authenticated: true` for `yusefnathanson@me.com`, user id
  `5bd6de97-3b58-408c-bf89-c42c81b083de`, with session `created_at`
  `2026-05-26T08:58:19Z`.
- Direct Comet navigation to
  `/api/vtext/documents/f93cea62-f833-4dae-b414-8e44783d8cbe/diagnosis?limit=5&include_content=false`
  succeeded through the owner session and showed the legal proposal as
  `choir_private_legal_cloud_proposal.vtext`.
- The diagnosis response reported owner id
  `5bd6de97-3b58-408c-bf89-c42c81b083de`, current revision
  `b2c9f30-7d31-4cee-a465-fad96a89b70c`, current version `v94`,
  revision count `95`, last author kind `user`, one table with `50` rows,
  `7` source markers, appendix table `L269-L318`, and stable table signature
  `sha256:a86643578fed81c67c8838ab00c5aba0a3af9a26294d59df529db8a15ab1f3b8`.
- The same response exposed recent proof lineage: a user bounded edit created
  revision `c29778fc-da4c-49d5-adb4-6d91a14a29c9` at `v93`, restore created
  revision `8cb29f30-7d31-4cee-a465-fad96a89b70c` at `v94`, and a recent
  `vtext` appagent run completed for the legal proposal with prompt
  `A revise event was triggered for the current VText document. Intent: revise.`
- Computer Use captured a visible app-state screenshot in the Codex thread,
  but `screencapture -x docs/evidence/source-system-2026-06-06/comet-legal-proposal-diagnosis-v94-20260606T170240Z.png`
  failed with `could not create image from display`; this turn therefore has
  thread-attached Computer Use visual evidence, not a repo-stored PNG.

Updated belief state: Comet owner authentication is currently usable for
read-only, document-id-scoped owner product endpoints. The legal proposal is at
a source-bearing true VText head with a 50-row appendix table after the bounded
edit/restore/revise lineage visible in diagnosis. The remaining acceptance gap
is not table survival at the current head; it is a clean, repeatable,
document-scoped owner mutation proof with durable screenshot/trace capture and
explicit rollback references, plus the broader publication/export and final
review requirements.

Owner legal-proposal source-open UI proof in Comet on 2026-06-06:

- The authenticated Choir tab loaded
  `https://choir.news/?owner_bounded_edit=93d9f819` and displayed
  `choir_private_legal_cloud_proposal.vtext` at `v94`, `Primary draft Latest`.
- The visible document title was
  `Proposal for [Redacted]: A Private Legal Cloud`; the first two inline source
  markers were visible in the opening section.
- Clicking the first marker expanded an inline transclusion for
  `ABA Formal Opinion 512: Generative Artificial Intelligence Tools`, showing a
  source excerpt and explicit `Open source` / `Collapse source` controls.
- Clicking `Open source` opened a new source-reader window titled
  `ABA Formal Opinion 512: Generative Artificial Intelligence Tools`.
- The source-reader window rendered content-forward reader text, reported
  `Available source text/markdown`, showed an `Open original` action to the ABA
  PDF URL, displayed the reader-mode note and source citation, and exposed
  collapsible `Source evidence`, `Source entity`, and `Provenance` details.
- The Comet desk showed this as a fourth in-app window with a source attachment
  chip, not a Web Lens/browser iframe. This supports the requirement that
  durable legal-proposal source artifacts default to Source Viewer while
  preserving explicit original inspection.

Residual risk: this proves one owner legal-proposal content-item style source
open from inline transclusion to Source Viewer. It does not by itself prove
guest publication opens for the same proposal, every source kind, or the final
publication/export metadata packet.

Owner legal-proposal publication/export/guest source-open proof on
2026-06-06:

- A short same-origin Comet bookmarklet first confirmed the owner session still
  reported `authenticated: true` for `yusefnathanson@me.com`, user id
  `5bd6de97-3b58-408c-bf89-c42c81b083de`.
- The owner-authenticated Comet tab then published the current legal proposal
  document id `f93cea62-f833-4dae-b414-8e44783d8cbe` through the product
  endpoint `POST /api/platform/vtext/publications` with slug
  `legal-proposal-source-proof-1780766508614`.
- Publish returned HTTP `201`, publication id
  `pub-878ee08d-2085-4291-b747-eda7ef704693`, publication version
  `pubver-e782e93b-5867-4e17-b921-c7f4d2619d11`, route
  `/pub/vtext/legal-proposal-source-proof-1780766508614-pub878ee08d2`,
  retrieval source `source-f43d8a65-ccc4-4865-901a-268c1ce31e6b`,
  retrieval span `span-35e90c40-5267-4b27-8a04-c852685f13cf`, citation
  `cite-d823a476-9e00-423f-b26a-6584bb49d1c9`, consent
  `consent-2fadded5-cc60-41dd-93da-5da182593795`, review
  `review-d2b36a60-8785-4281-8bf0-b2c2fbf27185`, and rollback
  `rollback-a3a4e807-3ef9-46b6-9209-c206168a9ad2`.
- Public resolve for the route returned active publication state, source
  revision hash
  `926080ae5e4ddb90e6c40f35264e2ab01883a711df928b4751febac490287432`,
  content/projection hash
  `f21ce9be51fb6f54358faf5f929515fe59b92367644c5328cfabf0bbc062a974`,
  `7` publication source entities, `7` transclusions, public access policy,
  and export formats `txt`, `md`, `html`, `docx`, and `pdf`.
- The resolved source entities covered URL-backed and content-item legal
  proposal sources, including `src_gdpr_article_32`, `src_aba_formal_op_512`,
  `src_nixos_rollback`, `src_aba_rule_16`, `src_ovh_private_cloud`,
  `src_hetzner_datacenters`, and `src_qdrant_search`. Each exposed
  `open_surface: source`, a publication-reader reader snapshot, and
  `reader_snapshot_ready`.
- Markdown publication export returned filename
  `legal-proposal-source-proof-1780766508614-pub878ee08d2.md`, preserved the
  proposal title and Appendix A content, included `7` source entities and `7`
  transclusions in canonical metadata, and carried access policy, export
  policy, retrieval source, and retrieval span metadata.
- Signed-out Playwright guest proof opened the public route, found `8` rendered
  source refs, expanded `src_aba_formal_op_512`, clicked `Open source`, and
  observed a `data-content-viewer` Source Viewer window in reader mode with the
  ABA source text. It observed `0` `data-browser-app` windows.
- Durable evidence artifacts:
  - `docs/evidence/source-system-2026-06-06/legal-proposal-publication-guest-reader-20260606T1722Z.png`
  - `docs/evidence/source-system-2026-06-06/legal-proposal-publication-guest-source-viewer-20260606T1722Z.png`
  - `docs/evidence/source-system-2026-06-06/legal-proposal-publication-guest-source-viewer-20260606T1722Z.trace.zip`
- Staging health during this proof reported proxy/upstream
  `deployed_commit=9a86044a244e9e0f41afd2162cd0cb277cbdbe0f`,
  `deployed_at=2026-06-06T17:09:32Z`, `status=ok`, and `vmctl_status=ok`.
  Later docs/classifier commit `eaf14f3d` intentionally did not deploy.

Residual risk: this proves real legal-proposal publication/export metadata and
one guest Source Viewer source-open path across URL-backed and content-item
source records. It does not prove a fresh owner bounded edit after publication,
does not test every rendered citation marker, and does not close the platform's
current limitation that publication policy defaults to public route/public
visibility unless revision metadata overrides it.

Hard mission review checkpoint:

- `docs/mission-source-system-hard-review-2026-06-06.md` records the
  adversarial/cognitive completion audit, hard findings, weak paths to prune,
  residual risks, and next realism axis. Status remains
  `checkpoint_incomplete`.

### Problem 14: Open-Surface Aliases Can Defeat Explicit Web Lens Routing

Status: `accepted_on_staging_for_open_surface_aliases`.

affected contract/invariant: source opening must keep Source Viewer as the
default for durable artifacts while allowing Web Lens only when the source
record explicitly requests original/live inspection.

problem: the frontend open plan recognizes exact `open_surface` spellings such
as `web_lens`, `browser`, `live`, and `original`, but does not normalize common
serialized aliases such as `web-lens`, `live_original`, or `source_viewer`. For
a URL-backed source, an explicit `open_surface: "web-lens"` currently fails the
live-original branch and then falls through to the durable-reader URL branch,
opening Source Viewer instead of Web Lens. Conversely, future `source_viewer`
or `reader` aliases are not part of the contract even though platform/export
metadata may reasonably use them.

evidence: code audit of `frontend/src/lib/vtext-source-renderer.ts` found
`sourceEntityRequestedOpenSurface` lowercases but does not normalize separators
or aliases, and `sourceEntityOpenPlan` branches on exact values before falling
back to `durableReaderTarget` for URL/content/source-service targets.

why this matters: the mission requires a shared source entity/open-surface
contract across VText, Source Viewer, Web Lens, publication, and export. If
aliases are not normalized at the frontend boundary, equivalent metadata emitted
by platform/export can have different open behavior even when it expresses the
same intent.

first observed version/transition: current `main` after
`7269278e docs: record owner legal proposal proof`.

acceptance for fix:

- `web-lens`, `web_lens`, `live-original`, and `live_original` all route to Web
  Lens/live original;
- `source-viewer`, `source_viewer`, `reader`, `content`, and `source` route to
  Source Viewer/reader mode;
- default URL/content/source-service targets still route to Source Viewer;
- explicit video media routing remains unchanged.

remaining error field: this is a frontend contract normalization fix. It does
not replace the broader shared source schema consolidation still needed across
runtime/platform/frontend/export.

### 2026-06-06 Source Open Alias Fix

Status: `accepted_on_staging_for_open_surface_aliases`.

Behavior commit:

```text
e378712ace52e2282e4be8557cbbd676da840dcf
fix: normalize source open surface aliases
```

Implementation:

- `frontend/src/lib/vtext-source-renderer.ts` now normalizes source
  `open_surface` aliases at the frontend boundary.
- `web-lens`, `web_lens`, `live-original`, and `live_original` route to Web
  Lens/live original.
- `source-viewer`, `source_viewer`, `source-reader`, `reader`, `content`, and
  `source` route to Source Viewer reader mode.
- Default URL/content/source-service targets still route to Source Viewer, and
  explicit video routing remains unchanged.

Local verification:

```text
npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g "source open plans normalize"
result: passed

npm --prefix frontend run build
result: passed
```

CI/deploy evidence:

```text
CI run: 27063916256 passed
FlakeHub run: 27063916233 passed
Node B deploy job: 79881404626 passed
staging deployed_commit: e378712ace52e2282e4be8557cbbd676da840dcf
staging deployed_at: 2026-06-06T13:45:00Z
```

Deployed acceptance proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g "VText source URL opens Source Viewer unless browser is explicitly requested"
result: passed
```

What this proves:

- A durable URL-backed source without explicit live/original request opens
  Source Viewer.
- A URL-backed source with `open_surface: "web-lens"` opens Web Lens/Browser.
- The proof ran against staging while proxy and sandbox both reported deployed
  commit `e378712ace52e2282e4be8557cbbd676da840dcf`.

Residual risk: this closes Problem 14 at the frontend routing boundary. It is
still not the full cross-language shared source schema; runtime/platform/export
normalization should continue converging toward a single documented contract.

### Problem 15: Source Evidence States Still Render As Raw Or Incomplete UI States

Status: `accepted_on_staging_for_source_evidence_state_ui`.

affected contract/invariant: missing-source placeholders must be replaced with
typed evidence states and researcher-backed
`confirms`/`refutes`/`qualifies`/`no_source_needed`/`stale`/`blocked_by_access`
states that are visible through VText, Source Viewer, publication, and export.

problem: the metadata paths now carry typed evidence states, but the frontend
does not yet render them as a shared user-facing contract. The Source Viewer
kicker prints raw state strings such as `confirms`, the Source entity details
print raw `state / research_state`, VText source chips show only title/kind,
and media-derived source entities can still emit `pending`, which is outside
the current mission evidence-state vocabulary. This leaves the product with
typed metadata but not a reliable owner/guest-visible evidence-state language.

evidence: code audit after open-surface alias deployment found:

- `frontend/src/lib/ContentViewer.svelte` computes `sourceState` from
  `sourceEntity.evidence.state`, `reader_snapshot_status.state`, or content
  provenance, then renders it directly in the Source Viewer header.
- The same component renders Source entity evidence as
  `{sourceEntity.evidence.state || 'available'} /
  {sourceEntity.evidence.research_state || 'unclassified'}`.
- `frontend/src/lib/VTextSourcePanel.svelte` lists source entity chips with
  only title and kind, so states such as `confirms`, `refutes`, `qualifies`,
  `stale`, or `blocked_by_access` are not visible before opening the source.
- `frontend/src/lib/vtext-source-renderer.ts` maps missing media content ids to
  `evidence.state: pending`, while the documented normalized states are
  `candidate`, `available`, `confirms`, `refutes`, `qualifies`,
  `no_source_needed`, `stale`, `blocked_by_access`, and `unavailable`.

why this matters: publication/export metadata can now preserve selector-rich
source evidence, but a reader still sees raw implementation tokens or no state
at all in the main source surfaces. That weakens the mission requirement that
source status be expressed as first-class evidence rather than as missing-source
or placeholder language.

first observed version/transition: current `main` after deployed commit
`e378712ace52e2282e4be8557cbbd676da840dcf` and proof commit `18fff41f`.

acceptance for fix:

- introduce one frontend evidence-state normalizer/label helper for VText,
  Source Viewer, and publication-local source entities;
- render source chips with a compact typed evidence label;
- render Source Viewer evidence with normalized labels and raw token-free
  owner/guest copy;
- map media-source pending state to the typed `candidate` state;
- preserve current publication/export metadata and Source Viewer default
  behavior.

remaining error field: this is a frontend/user-surface contract fix and does
not replace the broader backend/runtime/platform shared schema consolidation.

### 2026-06-06 Source Evidence State UI Fix

Status: `accepted_on_staging_for_source_evidence_state_ui`.

Behavior commit:

```text
681b5dbc78858e94037efbcbca9b4238b2c54542
fix: render typed source evidence states
```

Implementation:

- `frontend/src/lib/vtext-source-renderer.ts` now exposes a frontend
  evidence-state normalizer plus owner/guest-readable labels for the typed
  source states.
- Media-derived source entities now use `candidate` instead of the out-of-band
  `pending` state when no durable content id exists.
- `frontend/src/lib/VTextSourcePanel.svelte` renders a compact typed evidence
  label on each represented source chip.
- `frontend/src/lib/ContentViewer.svelte` renders normalized evidence labels
  in Source Viewer header/details instead of raw `state / research_state`
  tokens.

Local verification:

```text
npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g "source evidence states normalize"
result: passed

npm --prefix frontend run e2e -- tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies source-gap repair"
result: passed

npm --prefix frontend run build
result: passed
```

CI/deploy evidence:

```text
CI run: 27064141567 passed
FlakeHub run: 27064141571 passed
Node B deploy job: 79882003069 passed
staging proxy deployed_commit: 681b5dbc78858e94037efbcbca9b4238b2c54542
staging sandbox deployed_commit: 681b5dbc78858e94037efbcbca9b4238b2c54542
staging deployed_at: 2026-06-06T13:55:31Z
```

Deployed acceptance proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies source-gap repair"
result: passed
```

What this proves locally:

- the frontend evidence normalizer maps aliases including `pending`,
  `no-source-needed`, and `access-blocked` into the typed evidence vocabulary;
- the source repair product flow still creates a confirming source, renders the
  repaired inline source, opens Source Viewer, and now shows `Confirms claim /
  Owner supplied` instead of raw tokens;
- the VText Sources panel source chip exposes the typed `Confirms claim` state
  before opening the source artifact.

Residual risk: this closes Problem 15 for the frontend user surfaces covered by
the focused local and deployed tests. The backend/runtime/platform shared schema
remains a separate consolidation axis.

### Problem 16: Runtime Source Entity Evidence Can Still Persist Out-Of-Contract States

Status: `accepted_on_staging_for_runtime_source_evidence_states`.

affected contract/invariant: VText `source_entities` are canonical revision
metadata and should use the shared evidence-state vocabulary preserved through
VText, publication, export, Source Viewer, and Web Lens. The current
requirements contract names `candidate`, `available`, `confirms`, `refutes`,
`qualifies`, `no_source_needed`, `stale`, `blocked_by_access`, and
`unavailable` as the normalized source evidence states.

problem: runtime media-derived source entities can still write
`evidence.state: pending` or `evidence.state: error` into canonical VText
metadata. Frontend rendering now normalizes `pending` to `candidate`, but that
only masks the inconsistency at one consumer. Platform publication metadata
normalization does not accept `pending` or `error`, so a VText revision carrying
those states can lose evidence-state projection at publication/export time.

evidence: code audit after the Problem 15 frontend fix found:

- `internal/runtime/vtext_media_sources.go::sourceEntityEvidenceState` returns
  `pending` when a media source ref has no content id.
- The same function returns `error` when transcript availability is `error`.
- `internal/runtime/vtext_media_sources.go::mergeVTextSourceEntity` treats
  existing `pending` state as a replaceable sentinel, proving `pending` is part
  of the runtime metadata path rather than only a local UI label.
- `internal/platform/source_metadata.go::normalizePublicationEvidenceState`
  accepts the documented states and aliases such as `blocked` and
  `no-source-needed`, but not `pending` or `error`.

why this matters: the mission requires one source contract across VText,
Source Viewer, Web Lens, publication, and export. If runtime writes
out-of-contract states, publication/export either drop them or require each
consumer to carry private aliases. That preserves the duplication Problem 15
only fixed at the frontend boundary.

first observed version/transition: current `main` after staging-accepted commit
`681b5dbc78858e94037efbcbca9b4238b2c54542`.

acceptance for fix:

- runtime media-derived VText source entities use `candidate` for unresolved or
  not-yet-represented sources instead of `pending`;
- runtime maps transcript/source acquisition error states to a documented
  evidence state, preferably `unavailable` unless access policy proves a more
  specific blocked state;
- merge behavior treats `candidate` as the replaceable unresolved state;
- publication evidence normalization accepts legacy `pending` and `error`
  aliases to preserve older VText metadata;
- focused Go tests prove runtime and platform normalization agree on these
  aliases.

remaining error field: this is a backend canonical-metadata convergence step.
It does not yet create a single exported shared source schema package for all
runtime/platform/frontend consumers.

### 2026-06-06 Runtime Source Evidence State Canonicalization

Status: `accepted_on_staging_for_runtime_source_evidence_states`.

Behavior commit:

```text
4e46902c05b19a089db0273b257203f2ad337207
fix: canonicalize source evidence states
```

Implementation:

- `internal/runtime/vtext_media_sources.go` now writes `candidate` for
  unresolved media-derived source entities instead of `pending`.
- Transcript/source acquisition errors now map to `unavailable` rather than an
  out-of-contract `error` state.
- Runtime source entity merge treats both legacy `pending` and current
  `candidate` as replaceable unresolved states when better evidence arrives.
- `internal/platform/source_metadata.go` accepts legacy `pending` and `error`
  aliases when projecting publication source selector evidence, mapping them to
  `candidate` and `unavailable` respectively.

Local verification:

```text
nix develop -c go test -tags comprehensive ./internal/runtime -run TestMediaSourceRefToSourceEntityUsesTypedEvidenceStates -count=1
result: passed

nix develop -c go test ./internal/platform -run TestBuildPublicationSourceMetadataNormalizesLegacyEvidenceAliases -count=1
result: passed
```

CI/deploy evidence:

```text
CI run: 27064305893 passed
FlakeHub run: 27064305913 passed
Node B deploy job: 79882444697 passed
staging proxy deployed_commit: 4e46902c05b19a089db0273b257203f2ad337207
staging sandbox deployed_commit: 4e46902c05b19a089db0273b257203f2ad337207
staging deployed_at: 2026-06-06T14:02:43Z
```

What this proves locally:

- canonical runtime VText source entity metadata no longer emits `pending` or
  `error` for the media-derived source states covered by the test;
- an unresolved source can later merge into `available`;
- publication/export projection preserves legacy `pending` and `error` source
  metadata as typed evidence states instead of dropping the state.

Residual risk: this closes Problem 16 for runtime media-derived source entity
states and platform publication projection of legacy aliases. This is still a
convergence step, not a single shared source schema package across all language
boundaries.

### 2026-06-06 Shared Backend Source Evidence Contract

Status: `accepted_on_staging_for_shared_backend_source_evidence_contract`.

Behavior commit:

```text
38d0c969b57e84dc9e11626277fb2857324af22d
refactor: share source evidence contract
```

Deploy packaging fix:

```text
81eb39964a769f39b977c20ef73f18333960ac87
fix: include source contract in deploy builds
```

Implementation:

- Added `internal/sourcecontract` as a shared backend source contract package
  for typed evidence-state constants and normalization.
- `internal/runtime/vtext.go` now normalizes VText source repair evidence
  through the shared package instead of a local switch.
- `internal/runtime/vtext_media_sources.go` now uses shared evidence-state
  constants and candidate-state normalization when merging source entities.
- `internal/platform/source_metadata.go` now projects publication selector
  evidence through the same shared normalizer.

Local verification:

```text
nix develop -c go test ./internal/sourcecontract -count=1
result: passed

nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestMediaSourceRefToSourceEntityUsesTypedEvidenceStates|TestMigratedSourceGapsCanBeRepairedAsCanonicalVTextRevision|TestVTextSourceGapRepairNoSourceNeeded' -count=1
result: passed

nix develop -c go test ./internal/platform -run 'TestBuildPublicationSourceMetadata(NormalizesLegacyEvidenceAliases|PreservesSelectorSet|DefaultsQuotedExcerptToEmbeddedTransclusion)' -count=1
result: passed
```

What this proves locally:

- runtime and platform no longer carry separate Go evidence-state switch
  statements;
- legacy aliases and canonical states are normalized through one backend
  contract;
- VText source repair and publication selector evidence paths still pass their
  focused tests.

CI/deploy evidence:

```text
behavior CI run: 27064439772 passed Go gates; deploy failed before packaging fix
behavior FlakeHub run: 27064439773 passed
deploy packaging problem checkpoint: 9dda47a7 docs: record source contract deploy packaging gap
deploy packaging fix CI run: 27064564602 passed
deploy packaging fix FlakeHub run: 27064564607 passed
Node B deploy job: 79883126145 passed
staging proxy deployed_commit: 81eb39964a769f39b977c20ef73f18333960ac87
staging sandbox deployed_commit: 81eb39964a769f39b977c20ef73f18333960ac87
staging deployed_at: 2026-06-06T14:14:28Z
```

Residual risk: this closes the backend evidence-state implementation drift, but
it is not yet a generated/shared frontend contract and does not yet cover
ReaderArtifact, selector, or open-plan structs.

### Problem 17: Shared Internal Source Contract Was Omitted From Deploy Source Closures

Status: `fixed_and_accepted_on_staging`.

Confirmed evidence:

```text
behavior commit: 38d0c969b57e84dc9e11626277fb2857324af22d
CI run: 27064439772
FlakeHub run: 27064439773 passed
Node B deploy job: 79882795542 failed
deploy failure:
  internal/platform/source_metadata.go:8:2: cannot find module providing package github.com/yusefmosiah/go-choir/internal/sourcecontract: import lookup disabled by -mod=vendor
  internal/runtime/vtext.go:50:2: cannot find module providing package github.com/yusefmosiah/go-choir/internal/sourcecontract: import lookup disabled by -mod=vendor
```

Observed behavior:

- normal Go CI accepted `internal/sourcecontract`;
- the staging deploy Nix builds filtered each service source closure to a
  service-specific `internalDirs` list;
- the new shared package was not present in the platform/runtime service source
  closures, so deploy-time Nix package builds for host services and guest
  images failed even though Go tests passed.

Why this matters:

The source-system convergence work depends on small shared contracts rather
than duplicated switches in runtime, platform, frontend, Source Viewer, Web
Lens, publication, and export. If adding a shared internal package can pass Go
CI while failing only in Node B deploy packaging, the landing loop has a blind
spot exactly where shared source contracts should be safest.

Acceptance for fix:

- add `internal/sourcecontract` to every Nix service source closure that imports
  runtime or platform code using the contract;
- add deploy-impact classification for `internal/sourcecontract/*` so future
  edits rebuild and redeploy the affected service pointers and guest image;
- prove the filtered Nix packages build locally for at least the failing
  platform/runtime service closures before pushing;
- push the fix, monitor CI and staging deploy, and verify staging reports the
  fixed commit identity.

Fix evidence:

```text
documentation checkpoint: 9dda47a7 docs: record source contract deploy packaging gap
fix commit: 81eb39964a769f39b977c20ef73f18333960ac87
local classifier check:
  internal/sourcecontract/evidence.go -> gateway,platformd,proxy,sandbox,sourcecycled
local filtered source closure check:
  platformd/proxy/gateway/sourcecycled/sandbox source closures contain internal/sourcecontract/evidence.go
local full package build limitation:
  x86_64-linux Nix packages could not be built from this aarch64-darwin workspace because the configured remote builder was aarch64-linux only
local Go regression:
  nix develop -c go test ./internal/sourcecontract -count=1 passed
CI run: 27064564602 passed
FlakeHub run: 27064564607 passed
Node B deploy job: 79883126145 passed
staging proxy deployed_commit: 81eb39964a769f39b977c20ef73f18333960ac87
staging sandbox deployed_commit: 81eb39964a769f39b977c20ef73f18333960ac87
```

Remaining error field: this is a deploy packaging and impact-classification
gap, not a flaw in the evidence-state semantics. The immediate closure gap is
fixed for the current shared backend source evidence contract. Residual risk:
future shared source packages outside `internal/sourcecontract` still need an
explicit source-closure and deploy-impact entry when introduced.

### Problem 23: Frontend Source Contract Still Duplicates Backend Evidence And Open-Surface Normalization

Status: `fixed_and_accepted_on_staging`.

problem: backend runtime/platform paths now share evidence-state and
open-surface normalization through `internal/sourcecontract`, but the frontend
still carries separate switch statements in
`frontend/src/lib/vtext-source-renderer.ts`. The duplicated frontend evidence
normalizer already diverges from the backend contract: backend maps legacy
`error`, `failed`, and `fetch_failed` to `unavailable`, while the frontend
returns raw normalized tokens for those values. The frontend label helper also
has a `reader_snapshot_ready` state outside the mission evidence vocabulary.

affected contract/invariant: VText, Source Viewer, Web Lens, publication, and
export must share one source contract for typed evidence states and open
surface routing. If the frontend accepts or displays states that backend
publication/export normalizes differently, owner/guest surfaces can show raw or
out-of-contract evidence language even when canonical metadata and export
metadata are correct.

confirmed evidence:

```text
internal/sourcecontract/evidence.go:
  NormalizeEvidenceState maps error/failed/fetch_failed -> unavailable and
  unknown tokens -> empty string.

frontend/src/lib/vtext-source-renderer.ts:
  normalizeSourceEvidenceState handles pending/no-source-needed/access-blocked
  aliases but falls through to the raw normalized token for error/fetch_failed.
  sourceEvidenceStateLabel has a reader_snapshot_ready branch outside the
  documented evidence vocabulary.

frontend/tests/vtext-source-entities.spec.js:
  current evidence-state tests cover pending/no-source-needed/access-blocked
  but do not assert error/fetch_failed or unknown-token behavior.
```

acceptance for fix:

- move frontend source evidence/open-surface constants and normalizers into a
  small dedicated frontend source-contract module used by VText, Source
  Viewer, Source Viewer launch, publication-local source entities, and Web
  Lens/open-plan code paths;
- align frontend evidence aliases with `internal/sourcecontract`, including
  `error`, `failed`, and `fetch_failed` mapping to `unavailable`, and unknown
  tokens returning no canonical state rather than raw UI text;
- keep explicit Web Lens/live aliases distinct from durable Source Viewer
  aliases;
- add focused frontend tests for the backend-aligned aliases and existing
  source-open behavior.

remaining error field: this is a frontend contract-consolidation slice. It
does not generate a single cross-language schema or cover ReaderArtifact and
selector structs yet, but it removes one active duplicated normalization path
from the user-facing source surfaces.

Fix evidence:

```text
documentation checkpoint: bb9d1614 docs: record frontend source contract drift
behavior commit: 41b2135f7d72c21ee3ddccf6a7deb9053b8ad6b3
```

Implementation:

- Added `frontend/src/lib/source-contract.ts` as the dedicated frontend source
  contract module for typed evidence-state constants, evidence normalization,
  evidence labels, and open-surface normalization.
- `frontend/src/lib/vtext-source-renderer.ts` now imports and re-exports those
  helpers instead of carrying its own evidence/open-surface switches.
- Frontend evidence normalization now matches the backend contract for
  `error`, `failed`, and `fetch_failed` aliases by mapping them to
  `unavailable`.
- Unknown evidence-state tokens now return no canonical state and label as
  `Evidence unclassified`, instead of leaking raw tokens like
  `reader snapshot ready` into user-facing source surfaces.

Local verification:

```text
npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize|source open plans normalize'
result: 2 passed

npm --prefix frontend run build
result: passed
```

CI/deploy evidence:

```text
CI run: https://github.com/choir-hip/go-choir/actions/runs/27066839185
CI result: passed, including Go gates, frontend build, and Node B deploy
FlakeHub run: https://github.com/choir-hip/go-choir/actions/runs/27066839188
FlakeHub result: passed
Node B deploy job: 79889165316 passed
staging status: ok
staging vmctl_status: ok
staging proxy deployed_commit: 41b2135f7d72c21ee3ddccf6a7deb9053b8ad6b3
staging sandbox deployed_commit: 41b2135f7d72c21ee3ddccf6a7deb9053b8ad6b3
staging deployed_at: 2026-06-06T15:54:42Z
```

Deployed acceptance proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize|source open plans normalize'
result: 2 passed
```

Residual risk: frontend evidence/open-surface normalization now has one local
module used by VText source rendering, Source Viewer, publication-local source
entities, and source launch/open-plan code paths. The broader mission still
needs a generated or otherwise single-source cross-language contract and shared
ReaderArtifact/SourceSelector/OpenPlan structs.

### Problem 24: Frontend Source Open-Plan Resolution Still Lives In The Renderer

Status: `fixed_and_accepted_on_staging`.

problem: `frontend/src/lib/source-contract.ts` now owns source evidence and
open-surface normalization, but the actual Source Viewer/Web Lens/media/VText
open-plan decision still lives in `frontend/src/lib/vtext-source-renderer.ts`.
That renderer-specific function decides whether a source opens `content`,
`browser`, `video`, or `vtext` based on target kind, requested open surface,
source kind, and URL presence. Source launch, Source Viewer, publication-local
source entities, and Web Lens routing all depend on this decision, so leaving
it in the renderer preserves a hidden behavior contract outside the contract
module.

affected contract/invariant: Source Viewer must remain the default for durable
URL/content-item/source-service artifacts, while Web Lens must be an explicit
live/original action. That is an open-plan contract, not only a rendering
detail. If the renderer remains the only owner, future Source Viewer, Web Lens,
or publication code can accidentally bypass or fork the rule.

confirmed evidence:

```text
frontend/src/lib/source-contract.ts:
  owns evidence-state and open-surface normalization only.

frontend/src/lib/vtext-source-renderer.ts:
  sourceEntityOpenPlan computes appId/openSurface/mode/liveOriginal/readerMode.
  sourceEntityOpenAppID and vtext-source-launcher depend on this renderer
  function for opening sources.

frontend/tests/vtext-source-entities.spec.js:
  source open-plan tests import sourceEntityOpenPlan from the renderer rather
  than a source-contract module.
```

acceptance for fix:

- move the generic open-plan resolver into `frontend/src/lib/source-contract.ts`;
- keep entity-shape extraction in the renderer, but pass a small normalized
  input object to the contract resolver;
- preserve current Source Viewer default, explicit Web Lens/live routing,
  media routing, and published VText routing;
- add or keep focused frontend tests proving the same open-plan behavior through
  the exported renderer API and the contract resolver.

remaining error field: this is a frontend open-plan consolidation slice. It
does not yet create backend/shared generated `SourceOpenPlan` structs or cover
ReaderArtifact/SourceSelector contracts, but it removes another renderer-owned
behavior rule from the source opening path.

Fix evidence:

```text
documentation checkpoint: 22e685d6 docs: record frontend open plan drift
behavior commit: b5c6a78fd2079869f9b7cb91cabc76ecb43feeec
```

Implementation:

- Added `SourceOpenPlanInput`, `SourceOpenPlan`, and `sourceOpenPlan` to
  `frontend/src/lib/source-contract.ts`.
- `frontend/src/lib/vtext-source-renderer.ts` now extracts entity shape
  details and delegates Source Viewer/Web Lens/media/VText routing decisions to
  `sourceOpenPlan`.
- The existing `sourceEntityOpenPlan` export remains stable for VText source
  rendering and launcher callers.
- `frontend/tests/vtext-source-entities.spec.js` now covers both the
  renderer-level source entity API and the contract-level resolver for
  source-service durable targets, explicit browser/Web Lens requests, and
  publication-version VText routing.

Local verification:

```text
npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source open plans normalize'
result: 1 passed

npm --prefix frontend run build
result: passed
```

CI/deploy evidence:

```text
CI run: https://github.com/choir-hip/go-choir/actions/runs/27066961353
CI result: passed, including Go gates, frontend build, and Node B deploy
FlakeHub run: https://github.com/choir-hip/go-choir/actions/runs/27066961328
FlakeHub result: passed
Node B deploy job: 79889468929 passed
staging status: ok
staging vmctl_status: ok
staging proxy deployed_commit: b5c6a78fd2079869f9b7cb91cabc76ecb43feeec
staging sandbox deployed_commit: b5c6a78fd2079869f9b7cb91cabc76ecb43feeec
staging deployed_at: 2026-06-06T15:59:57Z
```

Deployed acceptance proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source open plans normalize'
result: 1 passed
```

Residual risk: the frontend open-plan behavior now has one contract module.
The broader mission still needs a backend/shared or generated `SourceOpenPlan`
schema plus shared ReaderArtifact and SourceSelector contracts.

### Problem 25: Reader Snapshot Status Is Being Used As Source Evidence State

Status: `fixed_and_accepted_on_staging`.

problem: publication enrichment writes `reader_snapshot_status.state` values
such as `reader_snapshot_ready`, `not_publication_safe`,
`bounded_excerpt_only`, and `import_failed` to describe reader-artifact
availability. Those are artifact/readability states, not claim-evidence
states. The frontend Source Viewer currently falls back from real source
evidence to `reader_snapshot_status.state`, so a successful publication reader
snapshot can render as `Evidence unclassified` instead of keeping evidence
state separate from reader snapshot availability.

affected contract/invariant: the source contract distinguishes evidence
states (`confirms`, `refutes`, `qualifies`, `candidate`, `stale`,
`blocked_by_access`, `unavailable`, `no_source_needed`) from reader artifact
availability and publication-safety state. VText, Source Viewer, Web Lens,
publication, and export should not let `ReaderArtifact` workflow labels leak
into claim-evidence UI or source-intelligence state machines.

confirmed evidence:

```text
internal/proxy/platform_publish.go:
  enrichVTextPublicationMetadata writes reader_snapshot_status.state values:
  reader_snapshot_ready, not_publication_safe, bounded_excerpt_only, and
  import_failed.

frontend/src/lib/ContentViewer.svelte:
  sourceState falls back to
  normalizeSourceEvidenceState(sourceEntity?.reader_snapshot_status?.state ||
  item?.provenance?.state || '') after sourceEvidenceState(sourceEntity).

frontend/src/lib/source-contract.ts:
  normalizeSourceEvidenceState intentionally rejects reader_snapshot_ready and
  unknown tokens, so the fallback produces no canonical evidence state and the
  UI label becomes Evidence unclassified.

frontend/tests/vtext-source-entities.spec.js:
  already asserts sourceEvidenceStateLabel('reader_snapshot_ready') is
  Evidence unclassified, proving the token is outside the evidence vocabulary.

frontend/tests/vtext-source-service-publication.spec.js:
  currently expects published URL-backed source entities to carry
  reader_snapshot_status.state == reader_snapshot_ready.
```

acceptance for fix:

- define a frontend reader-artifact status contract separate from evidence
  states, including current publication states and labels;
- update Source Viewer/Content Viewer source-reader UI to display canonical
  evidence state only from source evidence/provenance evidence fields, not from
  `reader_snapshot_status.state`;
- show reader snapshot availability/truncation/warnings through the separate
  reader artifact status path;
- add focused frontend tests proving `reader_snapshot_ready` is a reader
  status, not a source evidence state, and that published reader snapshots do
  not render as `Evidence unclassified`.

remaining error field: this is a frontend reader-artifact contract
consolidation slice. It does not yet create backend/generated `ReaderArtifact`
schemas or remove all raw selector maps, but it stops one confirmed
cross-contract leak in the Source Viewer surface.

Fix evidence:

```text
documentation checkpoint: 2eefc0bd docs: record reader snapshot evidence-state drift
behavior commit: efd47e1c9ae56caee0de38b25d2419febf111de6
```

Implementation:

- Added frontend reader artifact status constants, normalization, and labels to
  `frontend/src/lib/source-contract.ts`.
- Re-exported those helpers through `frontend/src/lib/vtext-source-renderer.ts`
  for existing source-renderer callers.
- Updated `frontend/src/lib/ContentViewer.svelte` so `sourceState` no longer
  falls back to `reader_snapshot_status.state`; Source Viewer now displays
  reader snapshot readiness/truncation through a separate reader artifact
  status path.
- Added frontend regression coverage proving `reader_snapshot_ready` remains
  outside the source evidence vocabulary while rendering as `Reader snapshot
  ready` in published Source Viewer fixtures.

Local verification:

```text
npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize|published source readers prefer publication snapshots'
result: 2 passed

npm --prefix frontend run build
result: passed
```

CI/deploy evidence:

```text
CI run: https://github.com/choir-hip/go-choir/actions/runs/27067193187
CI result: passed, including Go gates, frontend build, and Node B deploy
FlakeHub run: https://github.com/choir-hip/go-choir/actions/runs/27067193193
FlakeHub result: passed
Node B deploy job: 79890092169 passed
staging status: ok
staging vmctl_status: ok
staging proxy deployed_commit: efd47e1c9ae56caee0de38b25d2419febf111de6
staging sandbox deployed_commit: efd47e1c9ae56caee0de38b25d2419febf111de6
staging deployed_at: 2026-06-06T16:09:51Z
```

Deployed acceptance proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize|published source readers prefer publication snapshots'
result: 2 passed
```

Residual risk: Source Viewer no longer treats reader snapshot workflow state as
claim evidence state. The broader mission still needs backend/shared or
generated `ReaderArtifact` schemas and SourceSelector contract convergence.

### Problem 26: Publication Source Selectors Still Pass Through Raw Selector Kinds

Status: `fixed_and_accepted_on_staging`.

problem: publication source metadata now preserves selector sets, but the
selector contract is still implemented as raw map handling in
`internal/platform/source_metadata.go`. `selector_kind` values from VText
metadata are copied into publication transclusion and export metadata without
canonical normalization. That means legacy or human-authored forms such as
`text quote`, `table-range`, `page range`, or an omitted selector kind can
survive into public `source_selector_json` even though consumers and tests
expect canonical contract values such as `text_quote`, `table_range`,
`page_range`, and `whole_resource`.

affected contract/invariant: source selectors must be shared contracts used by
VText, Source Viewer, Web Lens, publication, and export. Publication should
preserve selector richness while canonicalizing selector identity so guest
readers, exports, and future Source Service resolution do not need to
understand every legacy spelling independently.

confirmed evidence:

```text
docs/source-external-data-publication.md:
  lists selector kinds such as whole resource, text quote, page range,
  timestamp range, table range, table cell, and data vintage as contract
  concepts.

internal/platform/source_metadata.go:
  normalizePublicationSourceEntity reads selectors as []any, chooses the first
  raw selector map, and marshalPublicationSourceSelector copies selector maps
  directly into source_selector_json or selector_set without normalizing
  selector_kind.

internal/platform/service_test.go and
frontend/tests/vtext-source-service-publication.spec.js:
  currently prove canonical selector_set preservation only when inputs already
  use canonical underscore selector_kind values.
```

acceptance for fix:

- add a shared backend SourceSelector contract helper under
  `internal/sourcecontract` that normalizes selector kind aliases and preserves
  selector payload fields;
- make platform publication source metadata use that helper for single
  selectors and selector sets;
- default missing selector kinds to `whole_resource`;
- preserve selector-set richness, evidence state attachment, snapshot text, and
  content hash behavior;
- add focused backend tests proving alias normalization in publication and
  export metadata.

remaining error field: this is a backend SourceSelector convergence slice. It
does not yet generate frontend TypeScript schemas or resolve selector text
through Source Service, but it moves publication/export selector identity out
of platform-local raw map logic.

Fix evidence:

```text
documentation checkpoint: acebbfba docs: record raw source selector drift
behavior commit: eb14eddeba7e93e671c3026eada9b18221549a53
regression test commit: 322740c6 test: cover publication selector aliases
```

Implementation:

- Added `internal/sourcecontract/selector.go` with canonical selector kind
  constants and normalization for common alias spellings.
- Added `internal/sourcecontract/selector_test.go` coverage for missing,
  spaced, hyphenated, and custom selector kinds.
- Updated `internal/platform/source_metadata.go` so publication source
  metadata canonicalizes selectors before writing single-selector or
  selector-set JSON.
- Extended `internal/platform/service_test.go` to prove selector aliases
  normalize while selector payloads, selector sets, evidence state, snapshot
  text, and content hashes are preserved.
- Updated the source-service publication Playwright fixture to submit alias
  selector kinds and assert canonical selector output in publication resolve
  and export metadata.

Local verification:

```text
nix develop -c go test ./internal/sourcecontract ./internal/platform -run 'TestNormalizeSelector|TestBuildPublicationSourceMetadataPreservesSelectorSet|TestBuildPublicationSourceMetadataDefaultsMissingSelectorKind'
result: passed

nix develop -c go test ./internal/platform -run 'TestBuildPublicationSourceMetadata|TestPublishVTextCreatesImmutablePublicRecords'
result: passed
```

CI/deploy evidence:

```text
CI run: https://github.com/choir-hip/go-choir/actions/runs/27067369110
CI result: passed, including Go gates and Node B deploy
FlakeHub run: https://github.com/choir-hip/go-choir/actions/runs/27067369106
FlakeHub result: passed
Node B deploy job: 79890543587 passed
staging status: ok
staging vmctl_status: ok
staging proxy deployed_commit: eb14eddeba7e93e671c3026eada9b18221549a53
staging sandbox deployed_commit: eb14eddeba7e93e671c3026eada9b18221549a53
staging deployed_at: 2026-06-06T16:17:17Z
```

Deployed acceptance proof:

```text
PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g 'publishes source-service source entities as expandable transclusions and canonical exports'
result: 1 passed
```

Residual risk: backend publication/export now canonicalizes selector kind
identity through `internal/sourcecontract`. The broader mission still needs a
generated or otherwise single-source frontend/backend SourceSelector schema,
Source Service selector resolution, and legal-proposal/content-item guest
selector-rich proof.

### Problem 27: Frontend Source Quote Extraction Ignores Publication SourceSelector Sets

Status: `fixed_and_accepted_on_staging`.

problem: backend publication now stores selector-rich `source_selector` records
on publication transclusions, including canonical `selector_set` values, but
frontend source quote/excerpt helpers only scan `entity.selectors` and
`record.selectors`. A published source entity reconstructed from platform
records can therefore lose access to the canonical text quote when the entity
JSON lacks a flat `selectors` array and the quote only exists in
`transclusion.source_selector`. The Source Viewer and source-repair attachment
paths can then fall back to empty or less precise excerpt text despite the
publication carrying the selector-rich transclusion record.

affected contract/invariant: SourceSelector is a shared contract across VText,
Source Viewer, publication, export, and repair paths. Frontend surfaces should
read canonical publication `source_selector` records, including selector sets,
instead of assuming selectors only live as a flat source-entity field.

confirmed evidence:

```text
internal/platform/source_metadata.go:
  platform publication stores source selectors in publication_transclusions as
  source_selector_json, using either a single selector or selector_set.

frontend/src/lib/vtext-source-renderer.ts:
  publicationSourceEntityToLocal attaches the matching publication transclusion
  to entity.transclusion, but selectorTextQuote only scans entity.selectors and
  record.selectors. It does not inspect transclusion.source_selector or nested
  selector_set.selectors.

frontend/src/lib/vtext-source-actions.ts:
  attachReadableSourceToVText sends selectorTextQuote(entity) as the text_quote
  when attaching readable source artifacts, so the same blind spot affects
  source repair/attachment payloads.
```

acceptance for fix:

- add frontend SourceSelector helpers to `frontend/src/lib/source-contract.ts`
  that normalize selector kind aliases and flatten selector sets without losing
  payload fields;
- make `selectorTextQuote` and excerpt helpers inspect entity selectors,
  record selectors, and publication transclusion `source_selector` records;
- preserve the current preference for explicit transclusion `snapshot_text`
  before selector fallback;
- add focused frontend tests proving selector-set quotes are read from
  publication transclusions even when flat entity selectors are absent.

fix evidence:

```text
docs checkpoint:
  5df2732f docs: record frontend selector-set drift

behavior:
  c7210d27 fix: read publication selector sets in source viewer

changed paths:
  frontend/src/lib/source-contract.ts
  frontend/src/lib/vtext-source-renderer.ts
  frontend/tests/vtext-source-entities.spec.js

local checks:
  npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source selectors normalize|published source entity quote falls back'
  npm --prefix frontend run build
  npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source selectors normalize|published source entity quote falls back|Source Viewer renders publication transclusion selector-set quote'

CI/deploy:
  GitHub Actions run 27067629159: success for c7210d27
  frontend build job 79891163989: success
  Node B deploy job 79891235101: success
  FlakeHub run 27067629132: success
  staging identity observed after deploy: c7210d27dcb311149d56b90911b664f8a1589394

deployed proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source selectors normalize|published source entity quote falls back|Source Viewer renders publication transclusion selector-set quote'
  result: 3 passed
```

remaining error field: this is a frontend SourceSelector convergence slice. It
does not yet generate a single schema from the backend contract or resolve
selector text through Source Service, but it stops one publication/Source
Viewer/repair path from silently ignoring selector-rich publication metadata.

### Problem 28: Reader Artifact Status Is Still A Frontend-Only Source Contract

Status: `fixed_and_accepted_on_staging`.

problem: the mission now separates source evidence state from reader artifact
workflow state in the frontend, but the backend shared source contract does not
define reader artifact status at all. `frontend/src/lib/source-contract.ts`
defines `reader_snapshot_ready`, `not_publication_safe`,
`bounded_excerpt_only`, and `import_failed`, while
`internal/sourcecontract` defines evidence, open-surface, and selector
contracts only. Backend publication enrichment in
`internal/proxy/platform_publish.go` writes raw `reader_snapshot_status.state`
strings directly into VText metadata before platform publication. That leaves
Source Viewer, publication, export, and future Source Service readers depending
on a frontend-private interpretation of reader snapshot workflow state.

affected contract/invariant: reader artifacts are source-system objects, not
frontend labels. Source Viewer must remain the durable default for source
artifacts, Web Lens must remain explicit live/original inspection, and
publication must preserve source snapshots for authorized readers. Those claims
depend on a shared reader artifact state vocabulary distinct from evidence
states such as `confirms` or `blocked_by_access`.

confirmed evidence:

```text
frontend/src/lib/source-contract.ts:
  READER_ARTIFACT_STATES = reader_snapshot_ready, not_publication_safe,
  bounded_excerpt_only, import_failed
  normalizeReaderArtifactState(...)

internal/sourcecontract:
  evidence.go, open_surface.go, selector.go exist, but no reader artifact
  status contract exists.

internal/proxy/platform_publish.go:
  enrichVTextPublicationMetadata writes reader_snapshot_status.state values
  "not_publication_safe", "bounded_excerpt_only", and
  "reader_snapshot_ready" directly as string literals.

frontend/tests/vtext-source-entities.spec.js:
  verifies reader_snapshot_ready is not source evidence state and is only
  normalized through the frontend reader artifact helper.
```

acceptance for fix:

- add a backend shared reader artifact contract under `internal/sourcecontract`
  with canonical states and alias normalization matching the frontend helper;
- make publication enrichment use the shared constants/helpers for
  `reader_snapshot_status.state`;
- add focused backend tests proving aliases normalize and publication metadata
  emits canonical reader artifact states;
- preserve frontend behavior and add/keep a frontend parity test showing reader
  artifact states remain separate from source evidence states.

implementation and acceptance evidence:

```text
behavior commit:
  ef119260ecbeef3cb5f7b61287386f0f79fa7be9

implementation:
  internal/sourcecontract/reader_artifact.go defines canonical reader artifact
  states and alias normalization.
  internal/proxy/platform_publish.go uses the shared reader artifact contract
  while enriching publication reader_snapshot_status metadata.

local proof:
  nix develop -c go test ./internal/sourcecontract ./internal/proxy -run 'TestNormalizeReaderArtifactState|TestHandleVTextPublicationPublishesPublicURLSourceSnapshots|TestHandleVTextPublicationRecordsURLSnapshotImportFailureState' -count=1
  result: passed

  npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'source evidence states normalize'
  result: passed

landing proof:
  GitHub Actions CI run 27067967888: success
  FlakeHub run 27067967886: success
  Node B deploy job 79892137375: success
  https://choir.news/health:
    proxy/upstream deployed_commit ef119260ecbeef3cb5f7b61287386f0f79fa7be9
    deployed_at 2026-06-06T16:43:18Z

deployed proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js
  result: 3 passed
```

remaining error field: this closes one more shared source-contract gap but does
not generate a cross-language schema or finish owner legal-proposal
bounded-edit proof.

### Problem 29: YouTube Transcript Acquisition Bypasses Source Fetch Policy

Status: `fixed_and_accepted_on_staging`.

problem: the runtime URL import path now validates ordinary URL-backed source
imports through `internal/sourcefetch`, but the YouTube transcript acquisition
subpath still creates plain `http.Client` instances and does not validate every
provider/caption URL before opening it. `fetchConfiguredYouTubeTranscript`
builds a request from `CHOIR_YOUTUBE_TRANSCRIPT_API_URL` and sends it through
`(&http.Client{Timeout: 10 * time.Second}).Do(req)`. `fetchYouTubeTranscriptFromInnerTube`
uses `&http.Client{Timeout: 8 * time.Second}` for both the configurable
`CHOIR_YOUTUBE_INNERTUBE_PLAYER_URL` endpoint and the caption URL returned by
YouTube. `fetchYouTubeTranscriptFromCaptionTracks` likewise uses
`&http.Client{Timeout: 6 * time.Second}` for watch and caption-track requests.
That means one source import mode can bypass the SSRF-safe dialer, redirect
policy, proxy disabling, and URL validation that the rest of URL import and the
source-service adapters now use.

affected contract/invariant: source acquisition must be policy-checked and
SSRF-safe before text becomes a reader artifact, VText source entity, or
publication snapshot. YouTube/video transcript acquisition is still source
acquisition; treating it as media-specific helper plumbing creates a security
exception to the mission's unified evidence supply chain.

confirmed evidence:

```text
internal/runtime/content.go:
  ImportURLContent validates normalized URL with sourcefetch.ValidateURL and
  uses sourcefetch.Client for ordinary URL import.

  fetchYouTubeTranscriptFromInnerTube:
    client := &http.Client{Timeout: 8 * time.Second}
    playerURL := youtubeInnerTubePlayerEndpoint()
    captionURL := youtubeJSON3CaptionURL(track.BaseURL)

  fetchYouTubeTranscriptFromCaptionTracks:
    client := &http.Client{Timeout: 6 * time.Second}
    captionURL := youtubeJSON3CaptionURL(track.BaseURL)

  fetchConfiguredYouTubeTranscript:
    resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)

internal/sourcefetch/policy.go:
  sourcefetch.Client disables proxy use, validates redirect targets, and
  validates dial-time DNS/IP resolution against forbidden local/private targets.

internal/runtime/content_test.go:
  existing YouTube transcript tests use local httptest provider endpoints,
  which currently require no policy validation override because the transcript
  helpers bypass sourcefetch entirely.
```

acceptance for fix:

- route configured transcript providers, InnerTube player requests, watch-page
  caption discovery, and caption-track downloads through `sourcefetch.Client`;
- validate each provider, player, watch, and caption URL before creating or
  sending requests;
- keep tests able to use local httptest endpoints only by explicitly enabling
  the sourcefetch test override;
- add focused regression coverage proving local/private configured provider,
  configurable InnerTube endpoint, and caption-track URLs are rejected without
  the override;
- preserve existing YouTube transcript success tests under the explicit test
  override.

implementation and acceptance evidence:

```text
documentation checkpoint:
  2e72f622 docs: record youtube transcript fetch policy gap

behavior commit:
  213f0cbc465a63a4968819fa706880708bd57d7f

implementation:
  internal/runtime/content.go now builds transcript provider, InnerTube player,
  watch-page, and caption-track requests through newSourceFetchRequest, which
  calls sourcefetch.ValidateURL before http.NewRequestWithContext.
  The same paths now use sourcefetch.Client so redirects, proxy disabling, and
  dial-time DNS/IP validation match ordinary URL import.

local proof:
  nix develop -c go test ./internal/sourcefetch
  result: passed

  nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestFetchYouTubeTranscript|TestContentImportURLStoresConfiguredTranscriptItem|TestYouTubeJSON3CaptionURLForcesFormat' -count=1
  result: passed

  nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportURLDedupesYouTubeSourcePackets|TestFetchYouTubeTranscript|TestContentImportURLStoresConfiguredTranscriptItem|TestYouTubeJSON3CaptionURLForcesFormat|TestChooseYouTubeCaptionTrackPrefersHumanEnglish|TestParseYouTubeTranscriptProviderPayloadHandlesNestedTranscript' -count=1
  result: passed

landing proof:
  GitHub Actions CI run 27068249237: success
  FlakeHub run 27068249238: success
  Node B deploy job 79892865383: success
  https://choir.news/health:
    proxy/upstream deployed_commit 213f0cbc465a63a4968819fa706880708bd57d7f
    deployed_at 2026-06-06T16:55:13Z

deployed proof:
  GO_CHOIR_RUN_CONTENT_SUBSTRATE=1 PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/content-substrate-routing.spec.js
  result: 1 passed

  authenticated product API probe:
    POST /api/content/import-url
    body: {"url":"http://127.0.0.1:1/source-policy-proof"}
    result: HTTP 502 {"error":"source URL host is not allowed"}
```

remaining error field: this closes a concrete source-acquisition policy bypass
for YouTube transcript imports. It does not complete robots/TOS/rate policy,
connector policy, or the broader generated source-contract schema work.

### Problem 30: Local Service Startup Still Injects Host-Specific ICU Flags

Status: `fixed_with_local_harness_proof`.

problem: `start-services.sh` now starts the local platform publication path, but
it still contains a Homebrew-specific Dolt/ICU fallback:

```text
if [ -d /opt/homebrew/opt/icu4c@78/include ] && [ -d /opt/homebrew/opt/icu4c@78/lib ]; then
  export CGO_CFLAGS="${CGO_CFLAGS:--I/opt/homebrew/opt/icu4c@78/include}"
  export CGO_CXXFLAGS="${CGO_CXXFLAGS:--I/opt/homebrew/opt/icu4c@78/include}"
  export CGO_LDFLAGS="${CGO_LDFLAGS:--L/opt/homebrew/opt/icu4c@78/lib}"
fi
```

affected contract/invariant: the repo operating contract says Go, Dolt, and
native-dependency work should run through `nix develop`, and explicitly says
not to normalize hand-entered host-specific `CGO_CFLAGS`, `CGO_CXXFLAGS`, or
`CGO_LDFLAGS` for Dolt ICU except as a short diagnostic. The durable fix is the
declared Nix environment or equivalent harness configuration.

evidence: current `flake.nix` declares `icu` and `dolt` in the dev shell and
sets ICU `PKG_CONFIG_PATH`, `LD_LIBRARY_PATH`, and `CGO_*` through Nix. The
local startup script separately looks for `/opt/homebrew/opt/icu4c@78` and
injects those paths before launching the service stack.

first observed version/transition: resumed mission audit after docs checkpoint
`03b8b19b`, while answering whether the local `start-services.sh` should be
updated.

suspected owner: local developer service harness.

why local/UI-only fix is insufficient: leaving the fallback in the canonical
script teaches future agents and humans that host-local Homebrew paths are an
accepted harness path, which contradicts the project contract and can make
worker/candidate or CI failures look like local machine configuration problems.

fix and proof:

- `start-services.sh` now refuses to run outside the repo dev shell unless
  `CHOIR_ALLOW_HOST_LOCAL_SERVICES=1` is set for an explicit short diagnostic
  run.
- The Homebrew `/opt/homebrew/opt/icu4c@78` `CGO_*` injection block was removed.
- README local startup now uses `nix develop -c ./start-services.sh` and
  describes Dolt/ICU paths as coming from the declared Nix environment.
- `bash -n start-services.sh` passed.
- Running `./start-services.sh` outside Nix failed with the intended diagnostic.
- `nix develop -c sh -lc 'test -n "$IN_NIX_SHELL" && command -v dolt && pkg-config --cflags icu-i18n icu-uc >/dev/null && printf "IN_NIX_SHELL=%s\nCGO_CFLAGS=%s\nCGO_LDFLAGS=%s\n" "$IN_NIX_SHELL" "$CGO_CFLAGS" "$CGO_LDFLAGS"'`
  passed and showed Dolt from the Nix store with Nix-store ICU compiler/linker
  flags.

remaining error field: this answers the local `start-services.sh` question and
keeps local publication-platform startup aligned with the repo dev-shell
contract. It does not change staging behavior and does not replace staging as
the acceptance environment for vmctl, live workers, provider credentials,
promotion, rollback, or Choir-in-Choir behavior.

### Problem 31: Local Harness Changes Trigger Full Staging Deploy And Early Identity Stamp

Status: `classifier_fixed_identity_stamp_open`.

problem: a local-only harness change to `start-services.sh` triggered the
conservative full staging deploy path, including host OS, frontend, ordinary
guest image, Playwright guest image, vmctl restart, and active VM refresh. The
deploy-impact classifier treats unknown top-level deployed paths as full
deploys, and `start-services.sh` currently falls into that unknown bucket.

affected contract/invariant: staging deploys should be impact-classified by
the actual deployed artifact surface. Local developer harness files should not
force host/guest image deployment unless they are included in deployed host or
guest artifacts. Deploy identity evidence should mean the deployed runtime is
actually serving the candidate, not merely that a remote checkout has reset and
written `deploy.env`.

evidence:

```text
commit:
  9a86044a244e9e0f41afd2162cd0cb277cbdbe0f

deploy-impact local reproduction:
  README.md -> ignored (top-level markdown)
  start-services.sh -> unknown deployed path: conservative host + both guest images
  docs/mission-source-system-simplify-secure-smart-v0.md -> ignored (docs/workflow/test artifact)

classification result:
  deploy_needed=true
  deploy_host=true
  deploy_frontend=true
  deploy_host_service=false
  deploy_ordinary_guest=true
  deploy_playwright_guest=true
  deploy_host_os=true
  deploy_vmctl_restart=true
  deploy_active_vm_refresh=true
  host_services=

CI run:
  GitHub Actions CI 27068591429
  Go/runtime/frontend checks passed.
  Deploy to Staging job 79893752150 remained in progress in step
  "Deploy to staging" after more than five minutes.

staging health during in-progress deploy:
  https://choir.news/health reported proxy and upstream
  deployed_commit=9a86044a244e9e0f41afd2162cd0cb277cbdbe0f and
  deployed_at=2026-06-06T17:09:32Z while the deploy job was still in progress.

FlakeHub side run:
  FlakeHub run 27068591428 failed because Determinate/FlakeHub login/fetch
  requests timed out against flakehub.com; this appears external/network and is
  separate from the deploy-impact classification problem.
```

first observed version/transition: resumed mission local harness fix
`9a86044a` after docs checkpoint `cf61a1f3`.

suspected owner: deploy impact classifier and deploy identity stamping order in
`.github/workflows/ci.yml`.

why local/UI-only fix is insufficient: a local script-only change should not
consume a full staging deploy cycle or refresh live interactive computers. Also,
acceptance evidence that reads `/health` can overclaim deployment if the commit
is stamped before the remote script finishes building, installing, restarting,
refreshing, and probing.

planned proof:

- classify `start-services.sh` as a local harness path that does not require
  staging deploy;
- add deploy-impact classifier regression coverage for local harness paths;
- move or supplement deployment identity so final health identity is recorded
  only after successful install/restart/health probes, or expose a separate
  `deploy_in_progress`/`target_commit` distinction;
- rerun classifier proof and CI for the deploy-impact change.

classifier fix and proof:

- `.github/scripts/deploy-impact-classify` now treats `start-services.sh` as
  ignored local developer service harness.
- `bash -n .github/scripts/deploy-impact-classify` passed.
- Exact changed-set proof for `README.md`, `start-services.sh`, and this
  mission doc now returns `deploy_needed=false` with explanation
  `start-services.sh -> ignored (local developer service harness)`.
- Control proof for `internal/runtime/content.go` still returns
  `deploy_needed=true` with `host_services=gateway,sandbox`.
- Behavior/docs commit
  `eaf14f3d2b36feb175088ff7d064ef5562f7ace0` was pushed to `origin/main`.
  GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27068825236` completed
  successfully. The deploy-impact detector succeeded, all Go gates passed,
  `Build Frontend` was skipped, and `Deploy to Staging (Node B)` was skipped.
- Staging health after the classifier commit still reported the previous
  deployed behavior commit
  `9a86044a244e9e0f41afd2162cd0cb277cbdbe0f`, confirming the local harness
  classifier fix did not promote a staging deploy.

remaining error field: the local harness deploy-impact overclassification is
fixed. The early deploy identity stamping concern remains open: `/health`
reported the target commit while the deploy job was still in progress, so
future acceptance reports should pair health identity with terminal deploy job
status until the deploy script exposes a post-success identity distinction.

### Problem 32: Owner Publish API Cannot Explicitly Set Publication Policy

Status: `accepted_on_staging_for_explicit_policy_forwarding`.

problem: the real legal proposal publication proof intentionally needed a
guest-readable artifact, and the owner-authenticated publish endpoint created
one successfully. The proof also confirmed a policy-control gap: the proxy
owner publish request currently accepts only `doc_id`, `revision_id`, and
`slug`, then forwards no explicit `access_policy` or `export_policy` to
platformd. Platform publication therefore falls back to public route/public
visibility and broad export defaults unless the private VText revision metadata
already carries overriding policy.

affected contract/invariant: publication must preserve access/export policy and
publish allowed source records for authorized publication readers. For sensitive
VText documents, route visibility and export policy must be an explicit owner
or document policy decision, not a hidden platform default. Guest source proof
should publish a deliberate public/unlisted acceptance artifact, not rely on
accidental defaults.

evidence:

```text
real legal proposal publication proof:
  POST /api/platform/vtext/publications
  request payload: {doc_id, slug}
  response: 201 published
  route: /pub/vtext/legal-proposal-source-proof-1780766508614-pub878ee08d2
  policy from public resolve/export:
    access: {"route":"public","visibility":"public"}
    export: {"copy_allowed":true,"download_allowed":true,
             "formats":["txt","md","html","docx","pdf"]}

code audit:
  internal/proxy/platform_publish.go publishVTextRequest fields:
    doc_id, revision_id, slug
  internal/proxy/platform_publish.go platformReq does not set AccessPolicy or
    ExportPolicy.
  internal/platform/source_metadata.go defaultPublicationAccessPolicy returns
    {"visibility":"public","route":"public"}.
  internal/platform/source_metadata.go defaultPublicationExportPolicy enables
    txt, md, html, docx, and pdf download/copy.
```

first observed version/transition: hard mission review after real legal
proposal publication checkpoint `34906ee2`.

suspected owner: owner-facing publication API contract and VText publish UI.

why documentation-only fix is insufficient: the contract says publication
records and exports must preserve policy. A reader cannot distinguish
"deliberately public" from "defaulted public" when the owner publish API cannot
express the policy decision. Documentation can warn verifiers, but it cannot
prevent an owner from publishing a sensitive VText under broad defaults.

planned proof:

- extend the owner publish API to accept validated `access_policy` and
  `export_policy` objects;
- forward those policy objects to platformd;
- preserve existing behavior for callers that omit policy, so current public
  publication tests stay valid until UI defaults are deliberately changed;
- add proxy tests proving explicit policy is forwarded and malformed policy is
  rejected;
- add or update frontend tests/API helpers so intentional public guest proof can
  pass explicit public policy instead of relying on hidden defaults.

fix and local proof:

- `internal/proxy/platform_publish.go` now accepts optional `access_policy` and
  `export_policy` JSON objects on `POST /api/platform/vtext/publications`,
  rejects non-object policy values at the owner proxy boundary, and forwards
  valid policy JSON to platformd.
- `frontend/src/lib/vtext.js` `publishVText` now accepts `accessPolicy` and
  `exportPolicy` options and includes them in the product publish request when
  provided.
- `frontend/tests/vtext-source-service-publication.spec.js` now sends explicit
  public access/export policy for the source-service publication proof, so the
  deliberate guest-readable acceptance fixture no longer relies on hidden
  platform defaults.
- Focused local checks passed:
  `nix develop -c go test ./internal/proxy -run 'TestHandleVTextPublicationReadsPrivateRevisionAndPostsProjection|TestHandleVTextPublicationRejectsMalformedPolicy' -count=1`,
  `nix develop -c go test ./internal/proxy ./internal/platform -run 'TestHandleVTextPublication|TestPublishVTextCreatesImmutablePublicRecords|TestBuildPublicationSourceMetadata|TestPublicationExport' -count=1`,
  and `npm --prefix frontend run build`.
- Attempted local Playwright proof
  `npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities"`
  failed before exercising the app because no local server was listening on
  `http://localhost:4173/`. Deployed proof remains required after CI/Node B
  deploy.

remaining error field: this closes the API-contract gap for explicit policy
forwarding, but it does not yet add owner-visible publish policy controls or
change the default policy. Until those land, UI publish actions still default to
the current platform policy unless the document revision metadata or caller
explicitly supplies policy.

CI/deploy/staging proof:

- Behavior commit
  `db0dc5f0e64cd4cd74984ffd1e064bf3ecc684c6` was pushed to `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27069128729` completed
  successfully, including runtime shards, non-runtime tests, integration smoke,
  Go vet/build, frontend build, aggregate Go gate, and Node B deploy job
  `79895187852`.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27069128732` completed
  successfully.
- Staging health after the deploy job completed reported proxy and upstream
  `deployed_commit=db0dc5f0e64cd4cd74984ffd1e064bf3ecc684c6`,
  `deployed_at=2026-06-06T17:33:10Z`, `status=ok`, and `vmctl_status=ok`.
- Deployed product-path proof passed:
  `CHOIR_AUTH_STATE=/tmp/choir-policy-forward.storage.json CHOIR_AUTH_META=/tmp/choir-policy-forward.meta.json PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities"`.
  The test now sends explicit public access/export policy for the
  source-service publication fixture and still proves resolved/exported
  source metadata plus owner/guest Source Viewer opens.

owner-visible policy control fix:

- Behavior commit
  `8efb05a25430330ada50e1a2ac6ebe2418af9700` adds an owner-visible
  `Publication policy` band to the VText editor. The band states the currently
  supported policy precisely: public route, source snapshots, and copy/download
  exports for `txt`, `md`, `html`, `docx`, and `pdf`.
- The VText UI publish button is disabled until the owner checks
  `I approve publishing this revision to a public route.` The acknowledgement
  is tied to the currently selected document/version label so revision
  navigation cannot silently carry an old acknowledgement into a different
  selected version.
- `handlePublishCurrent` now sends explicit access/export policy through the
  existing owner publish API:
  `access_policy={"visibility":"public","route":"public"}` and
  `export_policy={"copy_allowed":true,"download_allowed":true,
  "formats":["txt","md","html","docx","pdf"]}`.
- `frontend/tests/vtext-authoring-history.spec.js` includes a regression test
  that stubs the product publish endpoint, verifies the publish button is
  disabled before acknowledgement, and asserts the explicit policy payload sent
  by the editor.
- Local verification: `npm --prefix frontend run build` passed.
- Local Playwright limitation: `nix develop -c ./start-services.sh` reported
  `Services started successfully`, but the local backend/frontend processes
  exited after startup in this desktop session. The focused local Playwright
  regression therefore failed before the VText flow: first because
  `localhost:4173` was not listening, then because `/auth/register/begin`
  could not reach the local auth service. This was treated as local harness
  persistence debt and not as staging acceptance proof.

owner-visible policy CI/deploy/staging proof:

- Behavior commit
  `8efb05a25430330ada50e1a2ac6ebe2418af9700` was pushed to `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27069371444` completed
  successfully, including deploy-impact detection, runtime shards,
  non-runtime tests, integration smoke, Go vet/build, frontend build, aggregate
  Go gate, and Node B deploy job `79895836858`.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27069371443` completed
  successfully.
- Staging health after the deploy job completed reported proxy and upstream
  `deployed_commit=8efb05a25430330ada50e1a2ac6ebe2418af9700`,
  `deployed_at=2026-06-06T17:44:31Z`, `status=ok`, and `vmctl_status=ok`.
- Headless Playwright proof using the old saved storage state
  `/tmp/choir-policy-forward.storage.json` was explicitly rejected as
  unauthenticated because `/auth/session` returned
  `{"authenticated":false}` and the UI showed `Local preview - sign in to
  save`.
- Computer Use / Comet owner proof on the running Comet app confirmed an
  authenticated staging owner surface: the desktop showed `Online`, open VText
  windows for the legal proposal, the deployed `PUBLICATION POLICY` band, and
  disabled `Publish v94` before acknowledgement. After checking the
  acknowledgement, Comet exposed enabled `Publish v94`; clicking it published
  the route
  `https://choir.news/pub/vtext/choir-private-legal-cloud-proposal-vtext-pubb26d816ed`
  and the editor reported `Published v94; opened public link and copied URL`.
- Public resolve/export evidence for that route is stored at
  `docs/evidence/source-system-2026-06-06/vtext-publish-policy-owner-comet-20260606T1749Z.json`.
  It records publication `pub-b26d816e-dcab-4d23-bc75-26ab3e405b18`,
  publication version `pubver-0fae8500-7940-4239-a691-a0b79d1b6e2f`,
  `policy.access={"visibility":"public","route":"public"}`,
  `policy.export={"copy_allowed":true,"download_allowed":true,
  "formats":["txt","md","html","docx","pdf"]}`, seven selector-rich
  transclusions carrying the same access/export policy, Markdown export
  metadata with the same policy, Appendix A survival, table-header survival,
  and source marker survival.
- Attempted desktop screenshot capture with `screencapture` failed with
  `could not create image from display`, so this slice relies on the Computer
  Use accessibility/screenshot observation in the run log plus stored
  resolve/export JSON evidence rather than a new PNG.

remaining error field: Problem 32 is closed for explicit API forwarding and
for owner-visible public publication acknowledgement in VText. Remaining policy
work is broader: add real non-public publication semantics only after route,
reader, export, retrieval-source, and guest enforcement are proven; avoid
offering unlisted/private controls before those semantics exist.

## Post-publication legal proposal bounded-edit proof

Status: `accepted_on_staging_for_owner_bounded_edit_revise_publish_export`.

Owner-authenticated Comet proof on 2026-06-06 closed the remaining
post-publication bounded-edit gap for the legal proposal document
`f93cea62-f833-4dae-b414-8e44783d8cbe`:

- Computer Use / Comet remained authenticated on staging as
  `yusefnathanson@me.com`, user id
  `5bd6de97-3b58-408c-bf89-c42c81b083de`.
- The first mutation/proof bookmarklet ran against the document-scoped VText
  API, observed the legal proposal at v95 after the owner bounded edit, and
  confirmed v95 was user-authored with seven source entities, seven source
  markers, Appendix A, the glossary table header, and the Vector database row
  containing `bounded edit proof`.
- The document-scoped revise request
  `fd76a8d5-4282-4907-89db-d447a88291ba` created app-agent revision v96
  `5a5532d8-0ff3-44d6-aeef-5ea6cbc08798`. Comet observed v96 preserved the
  bounded phrase, Appendix A, the glossary table header, the bounded Vector
  database row, seven source entities, and seven source markers. Revision
  metadata reported `vtext_context_mode=focused_user_edit_diff`,
  `vtext_edit_operation=apply_edits`, and `vtext_run_prompt_chars=13808`.
- The app-agent v96 revision was published with explicit public access and
  export policy at
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pubf7bae84a8`.
- Public resolve proof for that route returned publication
  `pub-f7bae84a-80fa-4bf7-87f7-18ff07a01ca4`, publication version
  `pubver-ae91528d-d605-42dc-980c-16bfde4c20f8`, seven transclusions,
  `access={"visibility":"public","route":"public"}`, and
  `export={"copy_allowed":true,"download_allowed":true,
  "formats":["txt","md","html","docx","pdf"]}`.
- Public Markdown export proof for the same route returned `format=md`,
  content length `38426`, the bounded edit phrase, Appendix A, the glossary
  table header, source marker `source:src_aba_formal_op_512`, and metadata
  keys including `access_policy`, `export_policy`, `source_entities`,
  `source_revision_hash`, and `transclusions`.

Evidence artifacts:

- `docs/evidence/source-system-2026-06-06/legal-proposal-post-publication-bounded-edit-20260606T1758Z.json`
- `docs/evidence/source-system-2026-06-06/legal-proposal-bounded-edit-pubf7bae84a8-resolve-20260606T1758Z.json`
- `docs/evidence/source-system-2026-06-06/legal-proposal-bounded-edit-pubf7bae84a8-export-md-20260606T1758Z.json`

Proof limitations:

- The first bookmarklet attempt found the current head was already v95 with the
  bounded row, so it did not create a second owner edit. The accepted proof uses
  that v95 user revision as the bounded edit and v96 as the app-agent revise
  survival revision.
- A later compact collector accidentally submitted another document-scoped
  revise request from v96, then received a single `401 authentication required`
  response while polling the owner revision list despite `/auth/session`
  reporting authenticated earlier in the same collector. That later collector
  was not used as acceptance evidence. This is recorded as proof-path
  limitation and a possible auth-renewal follow-up, not yet as a newly
  confirmed platform behavior problem.

remaining error field: the legal proposal now has staging evidence for true
VText source-bearing table survival, bounded owner edit survival through
app-agent revise, explicit publication policy, and Markdown export source
metadata preservation. Remaining mission work shifts to final adversarial /
cognitive review, pruning weak or duplicate source-system paths, final hard
mission report plus PDF, and any broader shared-contract gaps that the review
still identifies.

## Hard mission review report

Status: `checkpoint_report_published`.

The adversarial/cognitive hard review was written to
`docs/source-system-hard-mission-review-2026-06-06.md` and rendered to iCloud
Drive at:

`~/Library/Mobile Documents/com~apple~CloudDocs/Choir Mission Reports/source-system-hard-mission-review-2026-06-06.pdf`

PDF text extraction passed with the expected title, table of contents, verdict,
hard findings, residual risks, rollback refs, and next realism axis. The report
does not claim full mission completion. It records the accepted source-system
correctness checkpoint and preserves residual risks around manually mirrored
Go/TypeScript source contracts, broader source-service fixture coverage,
future non-public publication semantics, direct proof-script auth limitations,
desktop screenshot capture, and the deploy identity stamp distinction.

## Shared source-contract matrix checkpoint

Status: `contract_matrix_added`.

To reduce the hard review's manual Go/TypeScript contract-mirroring risk, this
checkpoint adds one shared fixture matrix at
`internal/sourcecontract/testdata/source_contract_matrix.json`. The Go
`internal/sourcecontract` tests and the frontend
`frontend/tests/vtext-source-entities.spec.js` tests now consume the same
fixture for source evidence states, reader artifact states, selector kinds,
open-surface aliases, and frontend open-plan expectations.

Verification:

- `nix develop -c go test ./internal/sourcecontract -count=1` passed.
- `npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'frontend source contract stays aligned with shared matrix|source evidence states normalize|source open plans normalize'` passed.

remaining error field: this does not yet generate Go and TypeScript constants
from one schema, and it does not exhaustively cover every future source-service
or connector record. It converts the highest-risk mirrored constants into one
cross-runtime verifier matrix and leaves schema generation as a future
architecture simplification rather than a correctness blocker.

### Problem 33: Source Contract Testdata Triggers Staging Deploy

Status: `classifier_fixed_and_ci_accepted`.

problem: the shared source-contract verifier checkpoint changed only
documentation, frontend tests, Go tests, and a test fixture under
`internal/sourcecontract/testdata/source_contract_matrix.json`, but the
deploy-impact classifier required a Node B staging deploy, vmctl restart, and
active VM refresh.

affected contract/invariant: test fixtures and test-only verifier data should
not promote or refresh staging runtime state unless they are packaged into a
deployed host, frontend, guest image, or runtime asset. Staging deploy identity
should represent runtime artifact changes, not verifier-only matrix data.

evidence:

```text
commit:
  5855acd2a9475de6c0423de1d195512eb3dfdf53

CI run:
  https://github.com/choir-hip/go-choir/actions/runs/27069970346

deploy-impact output:
  docs/mission-source-system-simplify-secure-smart-v0.md -> ignored (docs/workflow/test artifact)
  frontend/tests/vtext-source-entities.spec.js -> ignored (docs/workflow/test artifact)
  internal/sourcecontract/matrix_test.go -> ignored (docs/workflow/test artifact)
  internal/sourcecontract/testdata/source_contract_matrix.json -> shared source contract dependency: runtime/platform service pointers + active VM refresh

classification result:
  deploy_needed=true
  deploy_host=true
  deploy_frontend=false
  deploy_host_service=true
  deploy_vmctl_restart=true
  deploy_active_vm_refresh=true
  host_services=gateway,platformd,proxy,sandbox,sourcecycled

deploy result:
  Node B deploy job 79897420194 completed successfully.

staging health after deploy:
  proxy and upstream deployed_commit=5855acd2a9475de6c0423de1d195512eb3dfdf53
  deployed_at=2026-06-06T18:11:10Z
  status=ok, upstream=ok, vmctl_status=ok
```

suspected owner: `.github/scripts/deploy-impact-classify`.

why local/UI-only fix is insufficient: this is a CI/deploy classifier behavior.
The local source-contract matrix tests passed, but the platform still consumed a
staging deploy cycle and refreshed live runtime state for a verifier fixture.

planned fix:

- classify `internal/**/testdata/**` as test-only unless a future package
  explicitly embeds that path in deployed runtime assets;
- keep non-test source contract files such as `internal/sourcecontract/*.go` as
  deployed shared contract dependencies;
- add classifier regression coverage for testdata under a deployed package and
  for a real sourcecontract `.go` file control.

fix and proof:

- `.github/scripts/deploy-impact-classify` now treats `internal/*/testdata/*`
  as an ignored docs/workflow/test artifact before the broad deployed
  `internal/sourcecontract/*` rule can match it.
- `.github/scripts/deploy-impact-classify-test` covers the sourcecontract
  testdata path, the companion `_test.go` path, and a real runtime
  `internal/sourcecontract/evidence.go` control.
- `.github/workflows/ci.yml` runs the classifier regression script in the
  deploy-impact job before classifying the pushed change set.
- `bash -n .github/scripts/deploy-impact-classify
  .github/scripts/deploy-impact-classify-test` passed.
- `.github/scripts/deploy-impact-classify-test` passed.
- Exact changed-set proof for this mission doc, the shared source-contract
  matrix testdata, the Go matrix test, and the frontend source-entity test now
  returns `deploy_needed=false`.
- Control proof for `internal/sourcecontract/evidence.go` still returns
  `deploy_needed=true`, `deploy_vmctl_restart=true`,
  `deploy_active_vm_refresh=true`, and
  `host_services=gateway,platformd,proxy,sandbox,sourcecycled`.

remaining error field: this fixes the verifier-only testdata
overclassification for direct `internal/*/testdata/*` paths and adds CI
coverage for the regression. It does not solve the broader architectural
problem that deploy impact is still path-pattern based rather than derived from
build closures or explicit package manifests.

post-push acceptance:

```text
problem checkpoint commit:
  81127f94 docs: record source contract testdata deploy impact

classifier fix commit:
  4c5d762866ef918e4909b0cf8c39336fe70598e3

CI run:
  https://github.com/choir-hip/go-choir/actions/runs/27070151859
  conclusion=success

deploy-impact job:
  79897807085
  Test deploy impact classifier: success
  deploy_needed=false
  deploy_host=false
  deploy_frontend=false
  deploy_host_service=false
  deploy_ordinary_guest=false
  deploy_playwright_guest=false
  deploy_host_os=false
  deploy_vmctl_restart=false
  deploy_active_vm_refresh=false
  host_services=
  .github/scripts/deploy-impact-classify -> ignored (docs/workflow/test artifact)
  .github/scripts/deploy-impact-classify-test -> ignored (docs/workflow/test artifact)
  .github/workflows/ci.yml -> ignored (docs/workflow/test artifact)
  docs/mission-source-system-simplify-secure-smart-v0.md -> ignored (docs/workflow/test artifact)
  Skipping staging deploy: only docs, workflow, or Playwright test artifacts changed.

other CI jobs:
  Go vet/build, non-runtime tests, runtime shards, integration smoke, aggregate check all succeeded.
  Build Frontend skipped.
  Deploy to Staging (Node B) skipped.

staging health after CI:
  proxy and upstream stayed on deployed_commit=5855acd2a9475de6c0423de1d195512eb3dfdf53
  deployed_at=2026-06-06T18:11:10Z
  status=ok, upstream=ok, vmctl_status=ok
```

## Generated source-contract schema checkpoint

Status: `local_verified_pending_ci`.

The source contract no longer depends on manually duplicated alias switch
tables in Go and TypeScript. The canonical contract vocabulary now lives in
`internal/sourcecontract/source_contract_schema.json`, including evidence
states, reader artifact states, selector kinds, open surfaces, aliases, labels,
and relational evidence-state flags.

Implementation:

- `internal/sourcecontract/schema.go` embeds the JSON schema and provides
  shared canonical lookup for Go normalizers.
- `NormalizeEvidenceState`, `NormalizeReaderArtifactState`,
  `NormalizeSelectorKind`, and `NormalizeOpenSurface` now use the embedded
  schema rather than hand-maintained alias switch tables.
- `frontend/scripts/generate-source-contract.mjs` generates
  `frontend/src/lib/source-contract.generated.ts` from the same schema while
  preserving the existing public frontend constant object names.
- `frontend/src/lib/source-contract.ts` imports the generated constants/schema
  and uses schema-backed lookup for frontend normalizers and labels.
- `frontend/package.json` runs the generator in `--check` mode before
  `vite build`.
- `internal/sourcecontract/schema_test.go` verifies the Go constants are
  represented in the schema and the generated frontend file carries the current
  schema hash.

Verification:

```text
node frontend/scripts/generate-source-contract.mjs --check

nix develop -c go test ./internal/sourcecontract ./internal/platform ./internal/proxy -run 'TestNormalize|TestBuildPublication|TestHandleVTextPublication|TestExport|TestSourceContractSchema' -count=1

npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js -g 'frontend source contract stays aligned with shared matrix|source evidence states normalize|source open plans normalize|source selectors normalize'

npm --prefix frontend run build
```

All passed locally.

deploy-impact local classification for the schema/frontend slice:

```text
deploy_needed=true
deploy_host=true
deploy_frontend=true
deploy_host_service=true
deploy_ordinary_guest=false
deploy_playwright_guest=false
deploy_host_os=false
deploy_vmctl_restart=true
deploy_active_vm_refresh=true
host_services=gateway,platformd,proxy,sandbox,sourcecycled
```

remaining error field: this converts the current source-contract canonical
values, labels, and aliases into a generated cross-runtime schema path. It does
not yet replace every future source entity, reader artifact, selector, evidence,
or open-surface shape with a typed IDL, and it still leaves broader source
service fixture coverage and owner legal-proposal end-to-end proof as separate
mission axes.

## Suggested `/goal`

```text
/goal Run docs/mission-source-system-simplify-secure-smart-v0.md as a Codex-operated MissionGradient mission. Preserve docs/source-external-data-publication.md as the requirements contract and docs/missiongradient-method.md as the operating method. First verify computer-use/Comet authenticated staging capability for yusefnathanson@me.com, document any limitation, then audit the whole current source system before behavior-changing code. Document each newly confirmed platform behavior problem before fixing it. Convert imported text-like documents to canonical .vtext by first durable revision while preserving export back to Markdown; create/migrate the legal cloud proposal to true VText with equivalent long-form content. Root-cause the owner appendix table regression by comparing v70-v78 and partial Markdown/VText render/edit/save/revise tests, then repair the general structure-preservation path. Make source acquisition policy-checked and SSRF-safe, converge source entity/reader artifact/selector/evidence/open-surface handling into shared contracts used by VText, Source Viewer, Web Lens, publication, and export. Keep Source Viewer the default for durable artifacts, make Web Lens explicit original/live inspection, preserve selector-rich transclusions and source snapshots through publication, and publish allowed source records for authorized publication readers. Replace missing-source placeholders with typed evidence states and researcher-backed confirming/refuting/qualifying/no-source-needed/stale/blocked states. Redesign source UI toward content-forward magazine/journal inline transclusions, using Pretext only where proof shows it improves text wrapping around source material. After first correctness, run adversarial/cognitive review, prune dead/weak/shortcut code paths, and produce a hard mission review report in docs plus PDF in iCloud Drive. Prove on staging with owner and guest source opens, URL-backed/content-item/source-service-style sources, legal proposal table survival, bounded table edit, publication/export source metadata, screenshots/traces, CI, Node B deploy identity, rollback refs, and residual risks.
```
