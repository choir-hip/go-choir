# A repeated rollback to the same target stays rollback_pending (2026-10-27)

Status: open. Observed on d@e (computer-59c902c5…) on staging, train 29
(cba66873b). Mutation class of any fix: red (self-development rollback,
tape). Documented before any fix (CLAUDE.md, problem documentation first).

## Evidence

1. **00:07:25 UTC.** FP0 ran `choir self-dev rollback --to-operation
   selfdev-4585da37… --wait`. This was the first owner-reachable rollback
   on staging. It settled `rolled_back` within 15 s
   (rollback-fe946ac2…). Its materialization receipt is 51b56776…, its
   effective head 5240937e…, and its note reads "rolled back to a plain
   release; its base claim dropped by design (base commit 0b99a404b…)".
   The R7 fix (a rollback records its own success) works on this path.
2. **00:07:47.** FP0 re-ran the same command to capture its output. This
   started a second rollback to the same target (rollback-4dca8039…). The
   cause is that the CLI's default idempotency key is
   `cli-rollback:<target>:<current effective head>`. The head moved after
   the first rollback, so the key changed. The "target is current" check
   did not refuse either: it compares the target's effective head with the
   head's, and a rollback to a plain release has a different head from the
   target operation's.
3. **After 00:20,** the second operation still reads `rollback_pending`.
   It already carries a materialization receipt (e6c8bb4b…) and the same
   effective head as the first (5240937e…). The head reads
   `pending_transition_ref=""`, desired fcddd72b… (this rollback's
   decision event) and effective ce1737e7….

## Hypotheses (not confirmed by trace)

- **H1.** A rollback whose materialization equals the current one never
  appends its applied event, so the operation never settles. The applied
  event may be skipped as a no-op, or the settle may wait for a route
  move that never comes.
- **H2.** The settle path matches on the pending transition ref, which is
  already empty. So nothing closes the operation, and desired stays ahead
  of effective.

The next observation is the guest journal and tape events for
rollback-4dca8039…, and the settle code in `internal/agentcore`'s rollback
path.

## Impact

d@e (a test computer) has an open rollback operation, and its desired
head is ahead of its effective head. Later applies on d@e may refuse until
this is closed. No owner computer is affected.

## Fix candidates

- **CLI.** Refuse a rollback whose target equals the last `rolled_back`
  operation's target when nothing has been applied since. Also make the
  default idempotency key stable across one rollback: target plus the
  last applied operation, not the head.
- **Guest.** A rollback that is a no-op settles `rolled_back` (or
  `refused: already at target`) immediately and never stays pending.
