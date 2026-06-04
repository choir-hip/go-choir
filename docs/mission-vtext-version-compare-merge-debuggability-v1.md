# MissionGradient v1: VText Version Compare, Concept Merge, And Debuggability

Status: draft for owner review
Date: 2026-06-04
Requirements contract: [vtext-version-compare-merge-debuggability-spec.md](vtext-version-compare-merge-debuggability-spec.md)

## Goal String

```text
/goal Run docs/mission-vtext-version-compare-merge-debuggability-v1.md as a Codex-operated MissionGradient mission. The requirements contract is docs/vtext-version-compare-merge-debuggability-spec.md. Build VText-native semantic versioning for long documents: historical versions remain publishable, draft lines are product-labeled with Primary draft as the default, Compare produces durable semantic findings over sections/concepts/citations/metadata, Merge into draft creates reviewable previews with provenance, and accepting a preview creates the next canonical VText revision. Repair the slow long-document revision path by making structured line/section edits the default VText operation, making whole-document rewrite explicit and exceptional, grouping multiple user edits into one canonical revision instead of autosave flooding version history, and recording prompt size, tokens, latency, edit operation, and delta evidence for every appagent revision. Improve debuggability by providing supported owner/admin retrospective query and diagnosis export over each user computer's persistent data.img state, including VText docs/revisions/runs/events/channel messages/Zot sessions, and root-cause the current Unicode prompt persistence failure before claiming tracing is reliable. Prove on staging with a real long VText: publish a historical version, compare v44-like historical content to latest, merge selected concepts into Primary draft as a preview, accept as a new version with provenance, and show deployed evidence that ordinary revisions use structured edits rather than full rewrites.
```

## Thesis

VText has crossed the point where linear version navigation is enough.
Long-form knowledge work needs version memory: the user should be able to keep
the better glossary from an older version, richer citations from a newer
version, clearer framing from another draft line, and publish any exact
historical state when that is the right artifact.

This is not a Git UI. Choir should expose product language: versions, draft
lines, Primary draft, historical version, compare, merge into draft, preview,
accept, publish. The internal implementation may use a graph, but the user
should not have to think like a developer.

The same mission must repair the cost and debuggability substrate. Semantic
versioning is not credible if normal revisions rewrite 10-page documents
through full-context prompts, if autosave floods the revision list, or if
owner/admin debugging requires manual filesystem archaeology inside
`data.img`.

## Real Artifact

The artifact is a deployed VText versioning and merge system over real user
computer state:

```text
long VText document
  -> meaningful canonical versions
  -> named draft lines
  -> semantic compare records
  -> reviewable concept merge previews
  -> accepted revision with provenance
  -> publish/export path over selected versions
  -> retrospective evidence from persistent data.img
```

## Hard Invariants

- VText remains the canonical artifact-level writing surface.
- Only the VText agent writes canonical `.vtext` files.
- Historical versions are first-class artifacts, not dead read-only snapshots.
- `Primary draft` is the default visible draft line. Do not expose Git terms as
  the primary product model.
- Compare and merge operate over VText structure, concepts, citations, source
  entities, metadata, and transclusions, not only raw text.
- Merge creates a preview or new revision with provenance; it must not silently
  mutate source or target revisions.
- Ordinary VText revisions on long docs default to structured edits.
- Whole-document rewrite is exceptional and requires explicit rationale.
- Autosave preserves draft text without advancing canonical version history.
- User-computer runtime evidence remains inside the user's persistent
  `data.img`; supported owner/admin query/export may inspect it with proper
  authorization.
- Staging is the acceptance environment.

## Value Criterion

Minimize semantic loss and revision latency for long VTexts while preserving
canonical provenance, user-understandable version navigation, source/citation
metadata, and retrospective debuggability from persistent per-user state.

The system gets better when:

- the user can recover valuable structure from older versions;
- VText revisions touch only the sections that need to change;
- version history becomes more meaningful, not noisier;
- compare/merge decisions are explainable and reversible;
- owner/admin diagnosis can answer why a revision was slow or failed without
  raw filesystem spelunking.

## Belief State

Current evidence:

- yusef's user-computer `data.img` contains persistent VText Dolt state with
  documents, revisions, runs, events, and channel messages.
- Zot session logs persist under the same per-user file system.
- Large legal-cloud VText revisions carried prompts roughly 34k-65k characters
  and took about one to two minutes per appagent revision.
- Appagent VText revisions use both `replace_all` and `apply_edits`; the issue
  is not absence of edit tooling, but weak defaults, full-context prompting,
  and insufficient evidence/guardrails.
- Autosave currently advances canonical versions, creating noisy histories.
- A fresh VText reaching very high version numbers such as `v44` is a symptom
  of that autosave/version flooding bug. A single substantive session may
  reasonably reach something like `v5`-`v20` through intentional revisions and
  accepted previews, but ordinary typing must not create a new canonical
  version every few seconds or every few words.
- Historical VText UI currently prevents some useful historical-version
  operations.
- A Unicode/store persistence failure has been observed while reconciling a
  VText worker wake.

Highest-impact uncertainty:

Whether the current VText tool/prompt path can be tightened enough around
structured edits, or whether the tool schema needs a stronger section-aware
patch primitive before long-document latency meaningfully improves.

## Homotopy Axes

Increase realism along these axes without changing the artifact topology:

- compare depth: raw diff -> structural diff -> semantic findings -> mergeable
  concept regions;
- merge authority: preview-only -> accept preview as revision -> draft-line
  branching and multi-source merge;
- edit granularity: replace all -> text replace -> section selectors -> source
  and metadata-aware patch operations;
- debug access: manual node-b inspection -> supported owner/admin query ->
  bounded diagnosis bundle -> product-visible Zot/Super Console diagnosis;
- UX density: mobile compact drawer -> merge sheet -> desktop side-by-side
  semantic rail.

## Required Work

### 1. Document And Reproduce Current Problems

Before behavior-changing code, update the problem record with:

- the historical-version publish/edit limitation;
- long-document prompt/revision latency evidence;
- replace_all/apply_edits distribution evidence;
- autosave canonical-version flooding;
- Unicode prompt persistence failure;
- current debug-access gap around per-user `data.img`.

### 2. Revision And Draft-Line Model

Implement or extend the VText data model for:

- draft line id/name;
- default `Primary draft`;
- selected historical version state;
- publish exact selected revision;
- restore selected revision as latest;
- create draft line from selected revision;
- compare source and target revision ids;
- merge preview and accepted merge provenance.

Do not make Git terminology the user-facing model.

### 3. Version Toolbar UX

Build the product-shaped toolbar inspired by the mockups:

- `v44`-style selected version pill;
- previous/next version controls;
- `Primary draft` selector;
- status label such as `Historical version`, `Comparing to v49`, or
  `v50 preview`;
- contextual actions: `Compare`, `Merge into draft`, `Publish v44`, `Accept`,
  `Edit`, `Discard`.

Keep the current VText visual language unless the existing design system
requires a targeted adjustment. The mockups are concept references, not styling
requirements.

### 4. Semantic Compare

Add a durable semantic compare operation and UI projection.

The compare output should identify:

- stronger/weaker sections;
- concept moves;
- glossary/list/table changes;
- citation/source entity changes;
- metadata/transclusion changes;
- conflicts and formatting regressions;
- suggested merge candidates.

Raw diff can exist as supporting evidence but should not be the primary
surface.

### 5. Concept Merge Preview

Add `Merge into draft` flow:

- open a merge sheet with source/target versions;
- list merge candidates with status labels;
- allow user selection;
- create preview with provenance strip and inline `from v44` / `from v49`
  style badges;
- accept preview into a new canonical revision;
- discard preview without advancing canonical history.

### 6. VText Edit Operation Defaults

Update tools/prompts/tests so ordinary long-document revisions use structured
edits.

Required direction:

- structured edits first in prompts and examples;
- full rewrite renamed or wrapped as explicit `rewrite_entire_document`
  semantics;
- full rewrite requires rationale, especially on long docs;
- revision metadata records edit operation, edit count, prompt size, token
  count, latency, and delta size;
- tests prove ordinary edits do not use full rewrite for a long document.

### 7. Autosave Canonicalization

Stop autosave from creating canonical revisions.

Draft persistence should be durable, but canonical history advances only on:

- explicit save/revise/publish snapshot;
- VText appagent revision;
- accepted merge preview.

### 8. Per-User Debuggability

Build supported retrospective access over per-user `data.img`.

At minimum, provide an owner/admin query/export path that can inspect:

- VText documents and revisions;
- runs and events;
- channel messages and worker updates;
- revision metadata;
- gateway/provider timing evidence;
- Zot session logs;
- store/persistence errors.

Root-cause and fix, or precisely block, the Unicode prompt persistence error.

## Evidence Plan

Evidence must include:

- staging commit identity;
- deployed VText UI screenshots or browser proof for historical publish,
  compare, merge preview, and accept;
- computer-use/browser QA on staging for mobile and desktop VText flows,
  including version navigation, compare, merge sheet, preview accept/discard,
  and historical publish;
- API/store evidence for draft line, compare, merge preview, accepted revision,
  and provenance metadata;
- long-document revision evidence showing structured edit operation, prompt
  size, tokens, latency, and delta;
- autosave evidence showing no canonical revision flood during typing;
- version-number evidence showing a new VText does not rapidly climb to
  `v44`-style histories from passive autosave, while deliberate session actions
  can still produce a reasonable `v5`-`v20` range;
- owner/admin retrospective query/export over the relevant per-user `data.img`;
- regression test results for backend and frontend surfaces touched.

## Anti-Goodhart Constraints

- Do not satisfy compare with only a text diff.
- Do not satisfy merge by copying the entire source version over the target.
- Do not claim latency improvement without long-document deployed evidence.
- Do not bury branch/draft-line semantics in labels only; provenance must be
  durable.
- Do not replace manual `debugfs` with another unsupported manual SSH ritual.
- Do not preserve noisy autosave versions as a hidden compatibility path.

## Rollback Policy

Versioning changes must preserve existing documents and revisions.

If semantic compare/merge fails:

- disable compare/merge actions behind feature capability while keeping normal
  VText read/edit/publish behavior available;
- keep historical publish behavior if already proven safe;
- preserve all created preview/merge records for diagnosis.

If draft-line migration fails:

- default existing revisions to `Primary draft`;
- do not rewrite historical content;
- provide a reversible migration or lazy projection.

## Stopping Condition

Stop only when staging proves all core flows on a real long VText:

- view historical version;
- publish that exact historical version;
- compare historical to latest with semantic findings;
- merge selected concepts into `Primary draft` as a preview;
- accept preview as a new canonical version with provenance;
- run ordinary VText revision on a long doc using structured edits by default;
- demonstrate autosave no longer floods canonical versions;
- query/export retrospective evidence from the relevant per-user `data.img`;
- root-cause and fix or precisely block the Unicode persistence failure.

## Suggested First Probe

Use the existing legal-cloud VText history as the product-path seed. Record
version counts, selected historical revision, current head, appagent operation
metadata, prompt sizes, token/latency evidence, and autosave behavior. Then
document the exact problem state before making code changes.
