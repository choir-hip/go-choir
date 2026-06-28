# Parallax: M23 — Bounded Inbox + Backpressure

**Conjecture (C22):** Actor mailboxes can be bounded with backpressure
on Send, preventing unbounded memory growth and durable log growth
under burst, without changing the existing actor runtime API.

**Class:** orange — runtime behavior change
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m23-bounded-inbox
**Branch:** orchestrator/m23-bounded-inbox
**Depends on:** M8 Phase 1 (runtime dead code deleted, cleaner substrate)

## Spec

Bound actor mailboxes and add backpressure:

1. **Bounded inbox:** each actor's mailbox has a configurable capacity
   (default: 1000 messages). When full, Send returns an error or blocks
   (configurable).

2. **Backpressure on Send:** when an actor's inbox is full:
   - Non-blocking Send returns `ErrInboxFull` immediately
   - Blocking Send waits with a timeout (configurable, default 5s)
   - The sender gets feedback, not silent drop

3. **Actor failure observability:** when an actor dies silently (panic,
   unrecoverable error), the supervisor is notified. Currently silent
   actor deaths are undebuggable.

4. **Graceful shutdown drain:** on shutdown, in-flight handlers get a
   cancellation context. Partial side effects are logged, not silently
   dropped.

## Invariants
- Existing actor runtime API unchanged (additive options only)
- Default inbox size is configurable via option (WithInboxCapacity)
- Backpressure is opt-in (existing behavior = unbounded, for backward
  compat during migration)
- Actor failure notifications go to supervisor, not to the actor itself
- Use `nix develop -c` for all go commands

## Acceptance Criteria
- `nix develop -c go test -race ./internal/runtime/...` passes
- `nix develop -c go build ./...` passes
- Tests cover: bounded inbox fills, backpressure on Send, actor failure
  notification, graceful shutdown drain

Return: conjecture verdict, test output, files modified.
