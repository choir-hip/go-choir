# VText Version Compare, Concept Merge, And Debuggability Spec

Status: draft requirements contract for mission execution
Date: 2026-06-04

## Purpose

VText needs a first-class version system for long knowledge-work documents.
The product should let the owner inspect historical versions, publish a
historical version, restore or branch from it, intelligently compare versions,
and merge selected concepts from one version into another without flattening
the document into a raw text diff.

The same work must repair the current slow-revision failure mode. Long VTexts
are normal in Choir. Sending the whole document through a full rewrite path for
ordinary edits makes revisions take minutes, increases model cost, and raises
the chance that good structure in one version is lost in a later version.

## Product Principles

- VText versions are semantic document artifacts, not just file snapshots.
- The visible user model is "draft lines" and "versions", not Git branches.
- `Primary draft` is the default draft line label. Avoid exposing `main`,
  `HEAD`, `fork`, or other Git terms in ordinary UI.
- Historical versions remain useful. They can be published, compared, restored,
  or used as concept sources.
- Ordinary revisions should default to section/line edits. Entire-document
  rewrite is exceptional and should be explicit.
- Multiple local edits may become one canonical revision. Autosave must not
  create noisy canonical version history.
- A brand-new VText should not easily reach `v44`. Very high version numbers
  from one editing session are evidence of autosave/version flooding. A
  substantive session reaching roughly `v5`-`v20` can be reasonable when the
  user intentionally revises, compares, previews, merges, and publishes.
- Debuggability belongs inside the affected user computer's persistent
  `data.img`, with owner/admin retrospective access over that state.

## Cognitive Transforms

Current uncertainty or obstacle:

VText is simultaneously a writing surface, a versioned artifact ledger, and an
agentic revision loop. The recent mockups show useful UI directions, but the
real risk is implementing visual affordances without repairing the underlying
version semantics, prompt cost, and traceability.

Selected transforms:

1. Depth extraction: the banal feature is "diff two versions"; the deeper
   feature is "let a document remember and recombine its best semantic
   structures."
2. Via negativa: delete accidental complexity from revision flow: noisy
   autosave versions, routine whole-document rewrites, and hidden trace
   spelunking.
3. Audience translation: users should see `Primary draft`, `Historical
   version`, `Merge into draft`, and `Publish v44`; implementers may keep graph
   terminology internally.
4. Evidence-first debugging: every VText revision should be explainable from
   durable per-computer evidence: prompt size, edit operation, token use,
   latency, source revision, target revision, and resulting delta.

Route-changing insights:

- The comparison surface must be semantic first, with raw diff available as
  supporting evidence.
- The merge output should usually be a new preview revision, not a mutation of
  either compared version.
- The VText agent needs a stronger tool contract that makes line/section edits
  the default and whole rewrites auditable exceptions.
- Version UI, VText tools, prompts, and debug evidence should be implemented as
  one coherent system, not separate UI polish and backend cleanup tickets.

## UX Model

### Version Toolbar

The preferred visual direction is the `Primary draft` mockup rather than the
Git-inspired `main` mockup.

Required toolbar concepts:

- selected version pill, for example `v44`;
- previous/next version controls;
- draft-line selector, defaulting to `Primary draft`;
- status label, for example `Historical version`, `Comparing to v49`, or
  `v50 preview`;
- contextual actions.

Contextual actions:

- latest version on primary draft: `Revise`, `Compare`, `Publish`;
- historical version: `Compare`, `Merge into draft`, `Publish v44`;
- merge preview: `Accept`, `Edit`, `Discard`;
- optional overflow actions: `Restore as latest`, `Create draft line`,
  `Compare to latest`, `View raw diff`.

### Compare Panel

A compare panel should answer "what changed?" at the semantic level before
showing raw text differences.

The compact mobile form should support a drawer like:

```text
What changed since v44

- Glossary structure is stronger in v44
- Latest citations are richer
- Executive framing in v44 is clearer
- Recent conclusion is stronger

One-tap merge recommendations:
- Restore glossary structure
- Keep latest source citations
- Use v44 executive framing
- Create merged v50
```

The desktop/high-density form may use a side-by-side semantic compare with a
middle merge rail. This is appropriate for expert review but should not become
the only compare experience.

### Merge Sheet

`Merge into draft` opens a reviewable merge sheet, not an immediate mutation.

Each merge candidate should include:

- source version;
- target draft/version;
- concept label;
- short excerpt or structural preview;
- confidence/status: `Clean merge`, `Needs review`, `Conflicts with latest`;
- selection checkbox or toggle.

Primary actions:

- `Preview merged draft`;
- `Apply as v50`.

`Apply as v50` creates a new canonical revision only after the preview is
accepted or when the user explicitly chooses the direct apply path.

### Merge Preview

A merge preview is a temporary or draft-state VText projection showing:

- version label such as `v50 preview`;
- banner: `Merged from v44 + v49`;
- provenance strip, for example `Glossary: v44`, `Sources: v49`, `Intro: v44`,
  `Conclusion: v49`;
- inline badges on merged sections such as `from v44` and `from v49`;
- visible citations and source/transclusion markers preserved.

Actions:

- `Accept`: create the new canonical revision on the target draft line;
- `Edit`: continue editing the preview before canonicalizing;
- `Discard`: abandon preview without advancing canonical history.

## Semantic Compare Requirements

The compare system should produce and store a durable comparison record.

It should classify:

- section additions, removals, moves, and renames;
- paragraph-level text edits;
- concept additions, removals, weakenings, strengthenings, and moves;
- citation/source entity additions, removals, replacements, and confidence
  changes;
- formatting or structure regressions;
- glossary/table/list changes;
- metadata and transclusion changes;
- conflicts between source and target.

Comparison output should include:

- source revision id;
- target revision id;
- draft-line ids if present;
- summary findings;
- raw text diff or structured edit diff;
- semantic regions with stable selectors;
- merge suggestions;
- confidence/caveats;
- model/provider/run evidence used to generate the semantic comparison.

Raw text diff is supporting evidence. It should not be the primary product
surface for ordinary use.

## Concept Merge Requirements

The merge system should create a new target revision or preview by applying
selected semantic regions from source versions into a target draft line.

Supported merge intents:

- restore section structure from an older version;
- preserve newer citations/source entities while restoring older prose;
- take an older introduction/framing and keep newer body evidence;
- merge glossary definitions from one version into another;
- preserve latest conclusion while restoring earlier organization;
- create a new draft line from a historical version;
- compare and merge across two non-latest versions.

Merge records should store:

- source revision ids and target base revision id;
- selected semantic regions;
- rejected regions if the user deselected them;
- conflict decisions;
- preview revision id if created;
- accepted revision id if accepted;
- author and run metadata;
- source/citation/transclusion provenance.

## Draft Lines And Branching

The product term is `draft line`.

Initial supported draft line:

- `Primary draft`.

Future or optional draft line examples:

- `Client draft`;
- `Short version`;
- `Style rewrite`;
- `Published version`;
- `Litigation memo`;
- `Executive summary`.

Internal implementation may use a version graph, but UI should present draft
lines as named document trajectories.

Required behaviors:

- viewing historical versions must not force the user back to latest before
  publishing;
- historical versions can be published exactly as they are;
- historical versions can be restored as latest by creating a new revision on
  the selected draft line;
- historical versions can seed a new draft line;
- draft line selector should be visible near the version label and navigation
  controls.

## VText Agent Editing Contract

The default VText behavior is structured edit, not full rewrite.

For ordinary revision requests, the VText agent should use line/section edits:

- replace a paragraph;
- replace a heading;
- replace a section body;
- insert before/after a stable selector;
- append to a section;
- move a section;
- update citations/source entities;
- update metadata/transclusion display policy.

Full rewrite is allowed only when the user intent implies the whole document is
being transformed:

- rewrite in a different style throughout;
- compress a long document into a short summary;
- expand a short outline into a long draft;
- reorganize the entire document;
- replace the document with a substantially different artifact.

The full-rewrite path should require explicit rationale in metadata, especially
for large documents.

Tool/prompt implications:

- put structured edits first in VText prompt examples;
- make whole-document rewrite visibly exceptional;
- consider renaming `replace_all` to `rewrite_entire_document` or wrapping it
  with a clearer operation contract;
- record edit operation, edit count, target selectors, rationale, prompt size,
  token counts, latency, and delta size in revision/run metadata;
- add guardrails for large documents so routine requests cannot silently choose
  a full rewrite.

## Autosave And Canonical Versions

Autosave should preserve in-progress user text durably without advancing
canonical VText version history.

Canonical revisions should be created by explicit user action or accepted agent
operation:

- user saves or revises;
- user accepts a merge preview;
- VText appagent writes a revision;
- publish flow explicitly snapshots a pending user draft into one revision.

This prevents normal typing from creating dozens of canonical versions and
makes version navigation meaningful.

Version-number expectation:

- a newly created VText should usually start near `v1`;
- ordinary typing must not increment the canonical version every few seconds or
  every few words;
- a focused session may reasonably produce `v5`-`v20` through intentional
  revisions, compare previews, merge accepts, restore actions, and publish
  snapshots;
- reaching `v44` quickly should be treated as a regression signal unless the
  session contains many deliberate canonical operations.

## Debuggability Requirements

Per-user runtime state belongs in that user's persistent `data.img`.

Required retrospective access:

- owner/admin can identify the active user computer and its `data.img`;
- owner/admin can query VText docs, revisions, runs, events, channel messages,
  and Zot sessions without manual `debugfs` spelunking;
- query path respects owner/admin authorization;
- export path produces a bounded diagnosis bundle for one document/run/version
  range.

Required VText debugging fields:

- document id and title;
- selected revision id and current head id;
- draft line id/name;
- parent revision id;
- author kind/label;
- edit operation and edit count;
- prompt size;
- input/output tokens;
- provider/model/reasoning;
- wall-clock latency;
- source/target revisions for compare/merge;
- merge preview and acceptance ids;
- error records, including Unicode/store persistence errors.

Known issue to repair:

Runtime reconciliation in yusef's VM logged failures like:

```text
runtime: reconcile doc c84d0007-f084-4352-9da9-66adb74d7b7d:
start reconciled vtext revision: persist child run: insert run:
Error 1105: Incorrect string value: '\xE2\x86...\x0A...' for column 'prompt'
```

This must be root-caused and fixed before claiming retrospective VText tracing
is reliable.

## Acceptance Criteria

The mission is successful only when staging proves:

- user can view a historical VText version and publish that exact version;
- user can compare two versions and see semantic findings;
- user can select at least one concept from a historical version and merge it
  into the primary draft as a preview;
- accepting the preview creates a new canonical revision with provenance;
- VText agent ordinary edits on a long doc use structured edits by default;
- full rewrite is reserved for explicit whole-document transformations and is
  recorded with rationale;
- autosave no longer floods canonical version history;
- computer-use/browser QA exercises the deployed mobile and desktop VText
  flows, including version navigation, compare, merge preview, accept/discard,
  and historical publish;
- owner/admin can query/export retrospective VText evidence from the relevant
  per-user `data.img` through a supported path;
- the Unicode prompt persistence failure is fixed or precisely blocked with
  root-cause evidence.

## Common Failure Modes To Avoid

- Shipping only a raw diff viewer.
- Exposing Git terminology as the primary user model.
- Making historical versions read-only dead ends.
- Creating merge output without provenance.
- Treating local/manual `debugfs` extraction as the debug product.
- Hiding full rewrites behind an operation named like an ordinary edit.
- Letting autosave continue to create canonical versions.
- Claiming VText is faster without measuring prompt size, tokens, latency, and
  edit operation on a long document.
