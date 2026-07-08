# Choir Git Statistics Report: 2026-06-16 to 2026-06-23

## Summary

This report covers the last seven days of work on the Choir repository. The period was dominated by the Texture structured-document cutover, the maild multi-tenancy migration, the object-graph conceptual refactor, and the PPTX renderer integration.

| Metric | Value |
|---|---|
| Date range | 2026-06-16 to 2026-06-23 |
| Total commits | 296 |
| Total files changed | 1,637 |
| Total insertions | 67,395 |
| Total deletions | 25,053 |
| Net lines added | 42,342 |
| Unique authors | 1 |

The single author is Yusef Mosiah Nathanson.

## Day-by-day breakdown

| Date | Commits | Files changed | Insertions | Deletions | Net lines | Notes |
|---|---|---|---|---|---|---|
| 2026-06-16 | 79 | 621 | 16,955 | 10,238 | +6,717 | Texture identity and product-loop recovery |
| 2026-06-17 | 39 | 200 | 7,779 | 1,616 | +6,163 | Texture platform and schema work |
| 2026-06-18 | 50 | 158 | 8,983 | 2,660 | +6,323 | Texture structured-document cutover |
| 2026-06-19 | 4 | 20 | 746 | 758 | -12 | Model policy and reader UX |
| 2026-06-20 | 2 | 29 | 2,688 | 384 | +2,304 | Texture durable mailbox turns |
| 2026-06-21 | 73 | 372 | 16,208 | 7,446 | +8,762 | Source-centric update_coagent cutover |
| 2026-06-22 | 46 | 236 | 13,715 | 1,951 | +11,764 | Maild migration and source-entity fixes |
| 2026-06-23 | 3 | 1 | 321 | 0 | +321 | Object-graph conceptual refactor docs |

## Notable patterns

### 1. High commit velocity on Texture work

June 16, 18, and 21 were the heaviest days. Each produced 50–79 commits. The work was the Texture structured-document cutover: moving from markdown-based sources to structured source entities, source refs, and transclusions. This is the largest semantic change in the period.

### 2. Net lines are positive but deletions are large

The net addition of 42,342 lines is large, but so are the deletions (25,053). This reflects a hard cutover: legacy code and docs were removed while new structured-document code was added. The negative net on June 19 shows a day of cleanup and policy tightening.

### 3. June 22 was the most efficient net-line day

June 22 added 11,764 net lines with only 1,951 deletions. This was the maild multi-tenancy migration and the source-entity carry-forward fix. The work was mostly additive: per-owner mailbox databases and the runtime fix for durable source entities.

### 4. June 23 was documentation-heavy

Only three commits on June 23, all docs. They produced the object-graph conceptual refactor docset, the synthesis report, and the Parallax paradocs. No code was changed on main after the PPTX merge except the docs commits.

### 5. PPTX merge landed at the end

The PPTX renderer integration from the `cascade/new-cascade-a40b71` worktree was merged into main at the end of the period. It is captured in the merge commit but the line-count impact is mostly in the `frontend/src/lib/SlidesApp.svelte` and `frontend/package.json` changes from the worktree.

## Top themes by commit subject

### Texture and structured documents

The most frequent terms in commit subjects:

- `texture`
- `source`
- `structured`
- `cutover`
- `docs: record`
- `docs: checkpoint`

This confirms the dominant work was the Texture structured-document cutover and the source-entity migration.

### Maild and multi-tenancy

June 22 included the maild migration: `maild: migrate legacy shared mailbox data to per-owner databases` and the follow-up `fix(texture): carry source entities in run metadata`.

### Object-graph refactor

June 23 introduced the conceptual refactor:

- `docs: add object graph conceptual refactor docset`
- `docs: add conceptual refactor report`
- `docs: add parallax paradocs for parallel object-graph work`

### PPTX renderer

The merge commit brought in the PPTX renderer integration, though the original commits were made earlier in the `cascade/new-cascade-a40b71` worktree.

## Files changed

The 1,637 file changes span:

- Frontend Svelte components (`SlidesApp.svelte`, `EmailApp.svelte`, etc.)
- Runtime Go code (`runtime.go`, `texture_agent_revision.go`, `texture_evidence_sources.go`, etc.)
- Documentation (`docs/*.md`)
- Tests (`*_test.go`)
- Prompt overlays (`internal/runtime/textureprompts/overlays/*.yaml`)
- Maild storage (`internal/maild/store.go`)
- Platform code (`internal/platform/*.go`)

## Implications

The statistics reflect a system in transition. The high volume of docs commits (`docs: record`, `docs: checkpoint`) is characteristic of a complex cutover where the documentation is tracking the changes as closely as the code. The large deletion count shows that the cutover was not purely additive: old representations (markdown source links, vtext naming, legacy source sidecars) were removed.

The week ended with a pivot from implementation to design: the object-graph conceptual refactor docset and the Parallax paradocs lay out the next phase of work. The implementation stats will likely shift in the coming week from Texture-only changes to object-graph migrations across mail, web captures, and slide decks.

## Methodology

Stats generated with:

```bash
git log --since='2026-06-16 00:00' --until='2026-06-23 23:59' \
  --format='%ad%n%H%n%h%n%s%n%an' --date=short --shortstat
```

Parsed and aggregated by a Python script. Commit counts, file changes, insertions, and deletions are derived from git's shortstat output.
