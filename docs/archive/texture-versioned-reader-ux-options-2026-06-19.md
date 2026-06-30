# Texture Versioned Reader — UX Options

Status: design options, first captured 2026-06-19. Not yet decided.

Context: the **Texture As A Versioned Provenanced Artifact v0** mission
([ledger](mission-texture-versioned-artifact-v0.ledger.md)) landed D1-D5 and
proved at D7 (`runacc-a5baefc8def0e2af4436`, `staging-smoke-level`) that a
published Texture now **is** its full versioned history: the `/pub/texture/...`
resolve bundle carries a `version_history` manifest with per-revision content,
typed provenance, citations, and a content-addressed hash chain. The backend
serves it; the frontend reader does not yet surface it. This doc is the
product/design pass the mission deferred to D7.

## What the backend now serves

`GET /api/platform/publications/resolve?route=/pub/texture/...` returns a bundle
whose `version_history` is:

```jsonc
{
  "schema": "choir.platform.version_history.v0",
  "revision_count": 3,
  "chain_head_hash": "rev1:050dade9…",   // == head revision's revision_hash
  "manifest_hash":  "9a6fa81d…",          // == publish response version_history_hash
  "revisions": [                          // oldest-first
    {
      "revision_id": "…", "parent_revision_id": "", "version_number": 0,
      "author_kind": "owner", "content": "<verbatim prompt>", "citations": …,
      "metadata": …, "provenance": …, "revision_hash": "…", "created_at": "…"
    },
    … // appagent revisions with typed provenance + grounded citations
  ]
}
```

## Current reader gap

`TextureEditor.svelte#loadPublishedContext` (line ~1014) resolves the bundle and
loads only `bundle.artifact.content` into the editor. The published `<article>`
(line ~2213) renders the head with `content_hash` / `source_revision_hash`
data-attrs. **`publishedBundle.version_history` is fetched but never rendered.**
A reader of `/pub/texture/...` sees the latest version only and has no signal
that the artifact is versioned, provenanced, or hash-chained.

## Design principle

Per `choir-doctrine.md` C2/C4: canonical user-facing truth *is* versioned
artifact state, and publication is a projection of the artifact-and-provenance
substrate. The reader should make the versioned, provenanced nature of a
published Texture **legible** without overwhelming a casual reader who just
wants the current text. Head prominent; lineage and provenance discoverable.

## Options

### Option A — Lineage disclosure (minimal)

A collapsible **"Version history"** panel rendered under the head content
(inside the `isPublishedReadOnly` article branch). Shows:

- `revision_count` ("3 revisions"), `manifest_hash`, `chain_head_hash` with a
  small "verified" affordance (the chain head equals the rendered head's
  revision hash — a reader/verifier can confirm integrity).
- A lineage list: one row per revision — version number, `author_kind`/label,
  `created_at`, a one-line provenance summary (e.g. "researcher evidence
  consumed"), and the per-revision `revision_hash`.

Head stays the dominant surface. No per-revision content rendering, no diff.
Surfaces the D5 artifact + hash verification with the smallest UX commitment.

- **Scope:** ~1 new Svelte component + a render slot in `TextureEditor`.
- **Risk:** low. Read-only; no mutation paths touched.
- **Tests:** extend `texture-source-entities.spec.js` / a published-reader spec
  to assert the panel renders `revision_count`, the chain head, and the lineage.
- **Residual:** a reader cannot *read* a prior version's body, only its metadata.

### Option B — Revision browser (medium)

Option A, plus: clicking a lineage row swaps the rendered body to that
revision's `content` (head view is one entry in the list, not special-cased).
The lineage becomes a list/sidebar; the selected revision's content renders in
the main surface; provenance + citations for the selected revision show in the
existing source apparatus.

- **Scope:** Option A + revision selection state + content swap + selected-
  revision source/citation binding.
- **Risk:** medium. Reuses the existing markdown render path; main risk is
  citation/source binding per selected revision (citations are per-revision in
  the chain).
- **Tests:** selection swaps content; selected revision's citations render;
  hash of the viewed revision matches its `revision_hash`.
- **Residual:** no view of *what changed* between revisions (must infer from
  reading two versions).

### Option C — Diff + per-revision sources (full)

Option B, plus: an inline **diff between adjacent revisions** (rendered
markdown diff) and a dedicated **per-revision sources/citations panel**. This is
the mission's stated target: "latest prominent + lineage + per-version sources."

- **Scope:** Option B + a markdown diff renderer + a per-revision sources panel.
  Likely a new diff utility (no markdown-diff dependency currently in the
  frontend).
- **Risk:** higher. Diff rendering over rendered (not source) markdown is
  fiddly; per-revision source snapshots may not all be published (some are
  private-scoped) so the panel needs graceful absence handling.
- **Tests:** diff correctness between known revisions; private-source absence
  handling; full chain traversal.
- **Residual:** largest maintenance + test surface; most opinionated UX.

## Recommendation (non-binding)

Start with **Option A**. It makes the versioned/provenanced nature legible and
surfaces the hash-chain integrity that is the whole point of D1-D5, at minimal
risk and without committing to a reading/diff UX that deserves its own design
iteration. B and C are natural follow-ups once A proves the surfacing shape.

## Non-goals (all options)

- No change to the publish path or manifest schema (D5 settled; this is render-
  only).
- No signing UI (D6, explicitly out of scope).
- No promotion-level workflow (AppChangePackage adoption is separate).
