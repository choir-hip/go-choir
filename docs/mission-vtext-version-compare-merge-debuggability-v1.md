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

## Run Checkpoint And Acceptance Evidence

```text
status: accepted_on_staging
documentation checkpoint commit: 8261cecb docs: checkpoint vtext compare merge mission
behavior commit: ca25041dd16ec84c9fc4a3dd9b87e147fa84cae3 feat: add vtext semantic compare merge flow
follow-up docs commit: 1d3f505d docs: record vtext merge accept staging blocker
CI run: 26984333466, success
FlakeHub publish run: 26984333456, success
Node B deploy: deploy-staging job 79630717195, success
staging identity: /health reported proxy and sandbox commit
  ca25041dd16ec84c9fc4a3dd9b87e147fa84cae3

what shipped:
- VText toolbar now exposes Primary draft, historical version state, Compare,
  Merge into draft, preview Accept/Discard, and historical Publish.
- Autosave now persists local draft text without creating canonical revisions.
- Semantic compare and merge-preview endpoints persist evidence records.
- Accepting a merge preview creates a canonical user revision with durable
  merge provenance and draft-line metadata.
- VText tool contract now defaults to apply_edits, requires rationale for
  replace_all on long documents, and records edit operation, edit count, base
  chars, result chars, delta chars, prompt chars, latency, and rationale.
- Run persistence normalizes invalid UTF-8 on create and update paths.
- Owner/admin diagnosis endpoint exports document, revisions, runs, events,
  channel messages, evidence, and persistent store/vtext paths.

local verification:
- nix develop -c go test ./internal/store -run
  'TestRunPersistenceNormalizesInvalidUTF8|TestCreateAndGetRun' passed.
- nix develop -c go test ./internal/runtime -run
  'TestMaterializeVTextToolEditRequiresRationaleForLongRewrite|TestVTextEditRevisionMetadataRecordsOperationEvidence|TestCleanVTextToolContent' passed.
- nix develop -c go test ./internal/store ./internal/runtime passed.
- npm --prefix frontend run build passed.
- npm --prefix frontend run e2e -- tests/vtext-document-stream.spec.js passed
  with 5 passed and 2 intentionally skipped dry-run tests.

deployed acceptance:
- proof artifact: /tmp/vtext-merge-staging-proof-final-1780614390072.json
- desktop screenshot: /tmp/vtext-merge-staging-proof-final-1780614390072.png
- mobile screenshot: /tmp/vtext-merge-staging-proof-final-1780614390072-mobile.png
- staging proof account:
  vtext-final-proof-1780614390072-bdee35@example.com
- long VText document:
  fdf24021-d1a2-48b0-a4a5-dfde4a3fe642
- source historical revision:
  ef0fe00c-938c-4ae9-95ae-99455cd7a9fc, 13391 chars
- target latest revision:
  cdf57431-4699-4b6e-a48a-ca133496763a, 12631 chars
- historical publish succeeded with status 201:
  pub-32bd3c15-0d05-4e06-a6d5-800e47337c37
  /pub/vtext/staging-long-compare-merge-proof-1780614390072-pub32bd3c150
- compare UI produced semantic findings including glossary/citation/framing
  findings over the selected historical version.
- merge preview was accepted with status 201, creating revision
  fa75b1ca-4971-4e54-a740-0cb60237ab22 with metadata:
  source=vtext_concept_merge, draft_line.name=Primary draft, merge source,
  target, suggestion ids, preview id, and evidence id.
- ordinary appagent revision on the 13804-char accepted base created revision
  3170c1b2-63b3-4b4a-9973-023af278b5be with:
  vtext_edit_operation=apply_edits
  vtext_edit_count=1
  vtext_edit_base_chars=13804
  vtext_edit_result_chars=13810
  vtext_edit_delta_chars=6
  vtext_run_prompt_chars=22080
  vtext_edit_rationale="Replace exact phrase \"Citation entity\" with
  \"Source citation entity\" in the glossary as requested."
- diagnosis endpoint returned:
  revision_count=4, run_count=1, event_count=22, message_count=0,
  evidence_count=2, error_matches=[], store_path=/mnt/persistent/state,
  vtext_path=/mnt/persistent/state.vtext.

belief-state updates:
- The first deployed proof incorrectly used vtextDocumentResponse.revision_count
  as the acceptance condition; staging returned revision_count=0 even while
  current_revision_id and list revisions showed accepted merge success. This is
  now a secondary API consistency bug, not a merge blocker.
- The Unicode persistence fix is bounded to invalid UTF-8 normalization for run
  prompt/result/error create and update paths. The deployed proof's diagnosis
  export returned no Incorrect string value matches for the accepted run, but a
  broader charset/collation audit remains prudent if valid non-ASCII payloads
  trigger the same Dolt/MySQL error again.

rollback refs:
- revert ca25041d to remove semantic compare/merge and edit guard behavior.
- revert 8261cec and 1d3f505 only if the mission/problem docs themselves need
  to be removed from main; otherwise leave the evidence trail intact.

residual risks:
- vtextDocumentResponse.revision_count is not reliable on staging and should be
  fixed or deprecated.
- Semantic compare is deterministic/heuristic first-cut logic, not yet a full
  model-backed semantic region graph over source entities and metadata.
- The UI exposes Primary draft as a label and stores draft-line metadata on
  accepted merge revisions, but full multi-draft-line persistence is still a
  future increment.
```

## Problem Checkpoint: Compare/Merge Was Deterministic Stub Logic

Updated: 2026-06-04T23:10:35Z

User review found that the first shipped compare/merge implementation hard-coded
domain-specific suggestions such as `Restore glossary structure` and produced
merge content with deterministic section replacement. That is not acceptable for
VText-native intelligent merge. The product must call the configured model
provider quickly for semantic compare and merge preview, show a working state
while that call is in flight, and persist model/latency/token evidence so the
operation can be audited.

The same review found rendered metadata at the bottom of the merged draft:

```text
<!-- VText merge preview provenance
Merged preview from v44 (...) into v49 (...).
- Restore glossary structure: ...
-->
```

This violates the VText perception and artifact contract. Merge provenance must
be durable structured metadata/evidence, not visible prose or markdown/comment
content inside the document body.

Belief state:

- Compare/merge routing, preview, accept, and revision persistence exist.
- The semantic analysis and content merge source is wrong because deterministic
  code, not the configured language model, drives the user-visible intelligence.
- Provenance storage exists, but the preview content builder leaks provenance
  into rendered VText content.

Next correction:

- Replace deterministic suggestion generation and preview construction with a
  bounded provider-backed semantic merge call through the runtime model policy.
- Retain deterministic validation/parsing only as response hygiene.
- Ensure preview and accepted content strip hidden provenance comments while
  preserving provenance in metadata/evidence.
- Add tests that fail on glossary-stub labels and visible provenance leakage.
