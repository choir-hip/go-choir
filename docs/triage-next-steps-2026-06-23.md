# Triage: Next Steps in Order

## Context

This is the ordered triage after reviewing the user's worktrees and the
second-opinion review. The Texture/source citation fixes are the highest
priority because they unblock the core product experience. Other work is
ordered around supporting or following that.

## Priority 1: Fix Texture source citations and style

- **Why:** The user's QA showed sources not appearing as inline citation points
  and the writing regressing to a Q&A format. This is the core product
  experience and must come first.
- **Artifacts in progress:**
  - `docs/design-source-ref-expansion-2026-06-23.md`
  - `docs/prompt-revisions-needed-2026-06-23.md`
- **Next steps:**
  1. Finalize the design doc: remove `source_embed`, add `expanded_ref` to
     `source_ref`, add `mark_source_unused`, make style textures toolbar sources.
  2. Edit `internal/runtime/textureprompts/overlays/run_system.yaml` to remove
     `source_embed`, the `WireTexture` branch, and the negative format list.
  3. Edit `internal/runtime/textureprompts/overlays/revision_source_entities_intro.yaml`
     and `revision_policy.yaml` to remove control flow and update citation
     guidance.
  4. Implement the schema/tool/renderer changes in `internal/texturedoc` and
     `internal/runtime`.
  5. Update tests to match the new model.
  6. Land this work before returning to the other worktrees.

## Priority 2: Land email freeze fix

- **Worktree:** `/Users/wiz/.windsurf/worktrees/go-choir/go-choir-<email-freeze>`
- **Why:** Real bug fix, real test, verifier-accepted.
- **Blockers:** Second opinion notes the new Playwright spec was not proven
  locally or on staging.
- **Next steps:**
  1. Run the new Playwright spec locally or on staging.
  2. Run the landing loop: `commit -> push origin main -> monitor CI -> monitor
     staging deploy -> verify staging commit identity -> run deployed acceptance
     proof`.
  3. If the spec passes, land. If it fails, fix and re-run.

## Priority 3: Understand and fix object service prototype

- **Worktree:** `/Users/wiz/.windsurf/worktrees/go-choir/go-choir-29131320`
- **Why:** User wants to understand more before deciding. The API surface is
  right, but it has a compile failure and weak owner semantics.
- **Blockers (from second opinion):**
  - P1: `internal/objectgraph/dolt_store.go:8` imports `path/filepath` unused.
  - P2: `internal/objectgraph/service.go:72` has no explicit owner parameter and
    falls back to `"system"` at line 95.
- **Next steps:**
  1. Fix the unused import so the package compiles.
  2. Review the owner/computer identity semantics. Choir objects cannot rely on
     implicit `"system"` fallback.
  3. Write a short DESIGN.md for the object service explaining the API,
     versioning, and ownership model.
  4. Decide whether to land the foundation or continue iterating.

## Priority 4: Fix docs checker and add doc-pruning pressure

- **Worktree:** `/Users/wiz/.windsurf/worktrees/go-choir/go-choir-6b7967c1`
- **Why:** Good direction, but the current implementation is warning suppression
  more than doctrine repair. The docs checker should also be the place where we
  enforce doc-pruning pressure, since the repo has 240+ `docs/*.md` files and
  only periodic stop-the-world garbage collection.
- **Blockers (from second opinion):**
  - P1: `docs/choir-doctrine.md:560` and `:864` split detector tokens into
    prose fragments to avoid string warnings. Do not land.
  - P2: `docs/doc-authority-manifest.yaml:403` archives a doc while the same
    manifest still uses it as a live witness at line 116.
- **Next steps:**
  1. Revert the detector-token string-splitting workarounds.
  2. Fix the underlying doccheck issue properly (either update the detector or
     update the text to match).
  3. Resolve the manifest archive/witness conflict.
  4. Re-run the focused runtime model-policy tests and the doccheck workflow.
  5. Land only after the P1 blockers are gone.

### Add doc-pruning pressure to the docs checker

The docs checker should enforce lifecycle rules that prevent unchecked doc
accumulation:

1. **Manifest states:** every `docs/*.md` file must have an entry in
   `docs/doc-authority-manifest.yaml` with one of these states:
   - `current` — actively maintained, linked from a root or mission.
   - `historical` — read-only evidence, linked to a settled mission.
   - `draft` — work in progress, tied to an open mission.
   - `candidate` — proposed design, must be promoted or archived within 30 days.
   - `archived` — moved out of `docs/` into `docs/archive/`.
   A doc without a manifest entry is `orphan` and emits a warning.

2. **Freshness warnings:**
   - `current` doc not edited or revalidated in 90 days.
   - `draft` or `candidate` doc not promoted/archived in 30 days.
   - `historical` doc still referenced by an active mission.

3. **Linkage requirement:** every `current` or `draft` doc must have an incoming
   link from a root doc, active mission, or code citation. Unlinked docs after
   30 days become candidates for archival.

4. **Per-mission doc budget:** a mission gets a default budget of 4 docs
   (paradoc, design, evidence, optional). Settlement is blocked until the
   mission's docs are promoted, archived, or deleted.

5. **Replacement default:** a new doc that supersedes an old one should archive the
   old one. The author must explain why both are needed.

6. **CI report:** the doccheck workflow should emit a report listing:
   - orphans
   - stale current docs
   - expired drafts
   - over-budget missions

This makes doc hoarding visible before it requires a stop-the-world cleanup.

## Priority 5: Extract PPTX renderer learnings, discard prototype

- **Worktree:** `/Users/wiz/.windsurf/worktrees/go-choir/go-choir-f4fdeb09`
- **Why:** Throwaway learning prototype. Keep the library/API learning, discard
  the code.
- **Blockers:** None, just cleanup.
- **Next steps:**
  1. Extract the API and integration lessons into a short `docs/design-pptx-rendering-2026-06-23.md`.
  2. Delete the `pptx-prototype/` directory.
  3. Commit the design doc and deletion.
  4. Note the P3 finding from second opinion: `viewer.presentation?.slides` is a
     private field; the public `viewer.slideCount` getter should be used in any
     future implementation.

## Priority 6: Extract source entity migration learnings into design doc

- **Worktree:** `/Users/wiz/.codex/worktrees/2bae/go-choir`
- **Why:** The paradoc is too bloated to be useful as-is, but the design
  artifact is strong.
- **Blockers (from second opinion):**
  - P2: `docs/paradoc-source-entity-migration.md:11` says `status: blocked`,
    while the suggested goal and mission graph say `open_handoff`. Reconcile.
- **Next steps:**
  1. Reconcile the status field in the paradoc.
  2. Distill the schema and plan into a concise design doc.
  3. Note how this relates to the current `source_embed` -> `source_ref`
     expansion work.
  4. Archive or delete the bloated paradoc after the design doc is extracted.

## Priority 7: Extract universal wire diagnosis learnings

- **Worktree:** (universal wire diagnosis)
- **Why:** Strong diagnosis/design artifact. The 502 is not reproducible; the
  route works. The value is the findings and the `choir.web_capture` schema.
- **Next steps:**
  1. Extract the findings and `choir.web_capture` schema into a concise design
     doc.
  2. Close the diagnosis mission as settled.

## Priority 8: Defer Qdrant indexing pipeline

- **Worktree:** `/Users/wiz/.windsurf/worktrees/go-choir/go-choir-87c664e7`
- **Why:** User wants to defer until universal wire and self-development are
  working.
- **Blockers (from second opinion):**
  - P1: `internal/qdrant/pipeline.go:70` uses `CanonicalID` like `obj:...` as the
    Qdrant point ID. Qdrant only supports `uint64` or UUID strings. Derive a
    stable UUID from the canonical ID.
  - P1: `internal/qdrant/client.go:143` sends `update_alias` with
    `new_collection_name`. The Qdrant alias API expects `create_alias`,
    `delete_alias`, or `rename_alias`. Use delete+create in one request or the
    official client.
  - P3: `gofmt` needed in three Qdrant files.
- **Next steps:**
  1. Document the known P1 issues in the design doc or a TODO.
  2. Do not run the integration until after universal wire and self-development
     are stable.
  3. When returning, fix the point ID and alias API issues before connecting to
     a real Qdrant service.
  4. Think through how Qdrant runs as a service on the Choir host.

## Priority 9: Worktree hygiene

- **Why:** Multiple worktrees have uncommitted changes on detached HEADs or
  snapshot branches.
- **Next steps:**
  1. For the two `codex` worktrees on detached HEADs with uncommitted changes:
     commit or stash the changes, or promote them to a named branch.
  2. For the Windsurf worktrees on `cascade/new-cascade-*` branches with
     uncommitted working changes: commit or stash before cleanup.
  3. For worktrees whose code is being discarded (PPTX, etc.), ensure the
     design doc is committed before deleting the prototype.

## Cross-cutting principle

Paradoc files are being used as live execution logs. They should be distilled
into proper design docs at settlement, keeping the paradoc as a concise mission
control document. Apply this to:

- source entity migration
- universal wire diagnosis
- any future paradoc that grows past ~200 lines

## Current session work

The `source_embed` -> `source_ref` expansion design
(`docs/design-source-ref-expansion-2026-06-23.md`) and the prompt revisions
needed (`docs/prompt-revisions-needed-2026-06-23.md`) are the top-priority work.
They are the extracted design artifact for the source entity/Texture citation
work and should be implemented and landed before returning to the other
worktrees.

## Recommended first 3 actions

1. **Finish and land the Texture source citation fixes.** This is the highest
   priority product work. Edit `run_system.yaml`, implement schema/tool changes,
   update tests, run staging QA.
2. **Land the email freeze fix.** Run the Playwright spec and the full landing
   loop.
3. **Do not land the docs-checker branch.** Revert the detector-token splitting
   and fix the underlying issue.
