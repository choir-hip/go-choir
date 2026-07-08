# Parallax: M3 — Base Journal + Tree Derivation

**Conjecture (C13):** An append-only event journal with parent-event
chaining can derive consistent trees at any cursor position, proving
the tape is tamper-evident and replayable.

**Class:** yellow — new packages, tests only, no production behavior change
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m3-base-journal
**Branch:** orchestrator/m3-base-journal
**Depends on:** M2 (model types now on main)

## Spec

Build `internal/base/journal` and `internal/base/tree`:

### Journal (`internal/base/journal/`)
- Append-only event store (in-memory + SQLite for tests)
- Event types: Create, Update, Delete, Move, BlobUpload
- Each Event has: EventID, ParentEventID, CursorSeq, ItemType, ItemID,
  BlobRef, Timestamp, SubjectID (author identity)
- ParentEventID forms Merkle chain (tamper-evident tape)
- CursorSeq: monotonic sequence for ordering
- Device cursors: track per-device sync position

### Tree Derivation (`internal/base/tree/`)
- Rebuild consistent tree from journal events at any cursor
- Tree is a snapshot of items at a point in time
- Derivation is pure: given events → produce tree
- Handle: create → update (latest wins), delete (tombstone), move
  (parent change)

## Invariants
- No I/O in tree derivation (pure function)
- Journal is append-only (no mutation of past events)
- ParentEventID chain is verifiable (hash chain)
- Conflicts preserve both sides (from M2 planner)

## Acceptance Criteria
- `go test ./internal/base/journal/...` passes
- `go test ./internal/base/tree/...` passes
- `go build ./...` passes
- No imports of `os`, `net`, or wall-clock `time` in tree package
- Journal tests cover: append, chain verification, cursor tracking
- Tree tests cover: derive from empty, derive after create, derive
  after update, derive after delete, derive after move, derive at
  intermediate cursor

## Verification
Run from the worktree:
```
cd /Users/wiz/.windsurf/worktrees/go-choir/m3-base-journal
nix develop -c go test ./internal/base/journal/... ./internal/base/tree/...
nix develop -c go build ./...
```

Return: conjecture verdict (SUPPORTED/REFUTED/PARTIAL), test output,
and list of files created.
