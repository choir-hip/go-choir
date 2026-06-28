# Parallax: M5 — Desktop Sync (Base Sync Wired into Desktop App)

**Conjecture (C23):** Base sync can be wired into the existing Wails
desktop app using the Base API (M4) and API key auth (M1), providing a
background sync loop that scans a local folder, compares with the
remote tree, plans actions, and executes uploads/downloads/deletes —
with conflicts surfaced in the UI, not silently resolved.

**Class:** orange — desktop app behavior change
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m5-desktop-sync
**Branch:** orchestrator/m5-desktop-sync
**Depends on:** M1 (API auth — on main), M4 (Base API — on main)

## Spec

Wire Base sync into the existing Wails desktop app (`cmd/desktop/`).

Phase 4 from `docs/spec-choir-desktop-wails-v3-2026-06-22.md`:

1. **Desktop app authenticates with API key** (created via WebAuthn, one time)
   - Store API key securely (OS keychain on macOS, file on Linux)
   - Use API key for all Base API calls

2. **Background sync loop:**
   - Scan local folder → build local tree
   - Fetch remote tree via `GET /api/base/delta?cursor=...`
   - Compare with synced tree using planner (M2)
   - Plan actions: uploads, downloads, deletes, moves
   - Execute actions via Base API (M4)
   - Update synced cursor

3. **Conflict surfacing:**
   - Show conflicts in desktop UI (not silently resolve)
   - User can choose: keep local, keep remote, keep both
   - Conflicts are events in the journal, not silent resolutions

4. **Sync status:**
   - Per-item state visible in desktop UI
   - Overall sync progress indicator
   - Last sync timestamp

## Invariants
- No silent conflict resolution (Base invariant from M2)
- API key stored securely (OS keychain on macOS)
- Sync loop is cancellable (graceful shutdown)
- Conflicts surfaced to user, not auto-resolved
- Use `nix develop -c` for all go commands

## Acceptance Criteria
- `nix develop -c go test ./cmd/desktop/... ./internal/base/...` passes
- `nix develop -c go build ./...` passes
- Tests cover: sync loop, conflict detection, API key storage, cursor
  management, graceful cancellation

Return: conjecture verdict, test output, files created/modified.
