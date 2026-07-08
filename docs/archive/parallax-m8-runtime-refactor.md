# Parallax: M8 Phase 1 — Runtime Dead Code Deletion

**Conjecture (C17):** At least 3K lines of dead code can be deleted from
`internal/runtime/` without breaking any live tests, per the deletion
policy in the runtime extraction plan.

**Class:** red — protected surface (runtime, execution substrate)
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m8-runtime-refactor
**Branch:** orchestrator/m8-runtime-refactor
**Spec:** `docs/runtime-deletion-and-extraction-plan-2026-06-27.md`

## Scope — Phase 1 ONLY

This mission covers ONLY dead code deletion, not extraction. The plan
says "delete first, refactor second." We delete:

1. **Legacy compatibility shims** — old field names, old data formats,
   backward-compat type aliases
2. **Disabled tools** — tools marked as disabled, retired, or "kept for
   rollback reference"
3. **Dead functions** — functions with no live callers (only test callers
   or no callers at all)
4. **Legacy soft paths** — code paths that only handle runs without
   metadata fields that are now always set
5. **"Temporary" code without a timeline** — code marked temporary without
   a specific deletion condition

## Deletion Policy (from the plan)

- Hard cutover, no legacy
- No "retained for rollback reference" — git history is the reference
- No "legacy soft path"
- Delete first, refactor second
- When in doubt, delete. If no live caller, delete. If only caller is a
  test, delete the test too (or rewrite for the live path).

## Invariants

- All live tests must pass after deletion
- No external API changes (deletion only, not refactoring)
- `nix develop -c go test ./internal/runtime/...` must pass
- `nix develop -c go build ./...` must pass
- Document each deletion category with line counts

## Acceptance Criteria

- At least 3K lines deleted from `internal/runtime/`
- `nix develop -c go test ./internal/runtime/...` passes
- `nix develop -c go build ./...` passes
- Deletion summary: what categories were deleted, line counts per category

## Verification
```
cd /Users/wiz/.windsurf/worktrees/go-choir/m8-runtime-refactor
nix develop -c go test ./internal/runtime/... 2>&1
nix develop -c go build ./... 2>&1
git diff --stat
```

Return: conjecture verdict (SUPPORTED/REFUTED/PARTIAL), lines deleted,
test output, deletion summary by category.
