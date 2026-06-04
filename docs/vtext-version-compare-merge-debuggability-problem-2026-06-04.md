# VText Version Compare, Merge, And Debuggability Problem Record - 2026-06-04

Status: problem documentation checkpoint before behavior-changing fixes

## Problem

VText has outgrown linear revision navigation. Long documents need usable
historical versions, semantic compare, concept merge, and durable provenance.
The current system makes historical versions too hard to act on, ordinary long
document revisions too expensive, and retrospective debugging too dependent on
manual node-b filesystem inspection.

## Evidence

### Historical Versions Are Product-Useful But UI-Limited

Owner review found that an older version (`v44` in the legal-cloud proposal)
could contain better structure than the latest version, but the current UI
treated historical versions primarily as read-only snapshots. The product needs
to support publishing a selected historical revision, restoring it as latest,
comparing it to latest, and merging selected concepts from it.

### Very High Version Numbers Can Be A Bug Signal

Fresh VTexts should not easily reach `v44`. That number emerged from the
autosave/version-flooding bug where new canonical versions were created after a
short interval of user edits. A substantive session may reasonably reach
roughly `v5`-`v20` through intentional revisions, compare previews, merge
acceptance, restore actions, and publish snapshots. Ordinary typing must not
advance canonical history every few seconds or every few words.

### Long-Document Revision Cost Is Too High

Node-b inspection of yusef's user-computer VText store and gateway logs showed
that appagent runs for `choir_private_legal_cloud_proposal.md` carried prompts
roughly 34k-65k characters, with gateway model calls in the 12k-35k input-token
range and large tool-call outputs. Several appagent revisions took about one to
two minutes.

The system already has both `replace_all` and `apply_edits`, but the observed
store contained many appagent `replace_all` revisions as well as `apply_edits`
revisions. The problem is not merely absence of an edit tool; it is weak
defaults, full-context prompt pressure, and insufficient guardrails/evidence
around whole-document rewrite.

### Autosave Was Advancing Canonical History

The existing problem record
`docs/vtext-user-draft-versioning-problem-2026-06-03.md` documented that
frontend autosave called the canonical revision endpoint. Dirty local changes
in this worktree already move autosave toward a local draft cache and update a
stream test expectation. Those changes should be preserved and completed
rather than reverted.

### Persistent Logs Exist But Are Hard To Query

The per-user VM `data.img` is the correct persistence boundary. Node-b
inspection of yusef's VM image found persistent Dolt VText state containing
documents, revisions, runs, events, and channel messages, plus persistent Zot
session files under the per-user filesystem. The missing product surface is a
supported owner/admin retrospective query and diagnosis export path. Manual
`debugfs` extraction of a Dolt store is evidence that data exists, not an
acceptable debugging product.

### Unicode Persistence Failure

Node-b vmctl logs showed runtime reconciliation failures like:

```text
runtime: reconcile doc c84d0007-f084-4352-9da9-66adb74d7b7d:
start reconciled vtext revision: persist child run: insert run:
Error 1105: Incorrect string value: '\xE2\x86...\x0A...' for column 'prompt'
```

This indicates the retrospective run/event store can reject at least one prompt
payload during VText reconciliation. Root cause is not yet proven. The mission
must fix it or record a precise blocker before claiming tracing reliability.

## Desired State

- `Primary draft` is the default draft line label.
- Historical versions can be published exactly, restored, compared, or used as
  merge sources.
- Semantic compare identifies meaningful changes in sections, concepts,
  citations, source entities, metadata, and transclusions.
- Concept merge creates reviewable previews and accepted revisions with
  provenance.
- VText defaults to structured line/section edits for ordinary long-document
  revisions.
- Whole-document rewrite is explicit, exceptional, and recorded with rationale.
- Autosave preserves draft text without advancing canonical history.
- Owner/admin debugging can query/export per-user `data.img` evidence through a
  supported path.

## Remaining Error

The exact data-model boundary for draft lines, compare records, and merge
previews must be shaped against current VText persistence. The smallest
acceptable implementation may encode draft-line and merge provenance in
revision metadata while adding durable compare/merge records if needed for
queryability. Staging proof must use real product paths and computer-use/browser
QA, not local-only API calls.

### Staging Acceptance Blocker: Merge Preview Accept Did Not Prove Head Advance

After commit `ca25041dd16ec84c9fc4a3dd9b87e147fa84cae3` deployed to staging,
a deployed Playwright probe against `https://choir.news` created a real long
VText with:

- source historical revision `ac899cc6-4ba7-4cec-852d-049d9e202643`
  (`11081` chars);
- target latest revision `5003313a-9145-4086-b902-c1e6eeccf435`
  (`10441` chars);
- document `f0da790e-c37d-4c64-825c-479d4319b0f7`.

The probe confirmed staging health/build identity at `ca25041d`, successfully
published the historical version via the product route, and reached the visible
merge preview UI with `Primary draft`, `v2 preview`, `Accept`, `Discard`,
compare findings, and provenance suggestions visible. The historical publish
created publication `pub-5433a120-d776-4127-ab04-b5cf03a13c87` and route:

```text
/pub/vtext/staging-long-compare-merge-proof-1780613758834-pub5433a120d
```

However, after the probe clicked `Accept`, polling
`/api/vtext/documents/{doc_id}` for a new current revision did not observe the
expected accepted merge revision within 60 seconds. Screenshot evidence was
written to:

```text
/tmp/vtext-merge-staging-proof-1780613749937.png
```

The current uncertainty is whether this was a test sequencing issue after
leaving the historical publication result panel visible while entering merge
preview, a frontend state issue where `publishResult` and `mergePreview`
coexist and confuse the accept flow, an API failure from `/accept-merge` that
was not captured by the first probe, or a backend revision/head update problem.
This blocker must be resolved before claiming the mission complete.
