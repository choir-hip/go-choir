# Texture Audit Can Misreport Committed Mutations

**Date:** 2026-08-05  
**Status:** observed; repair not included in this checkpoint  
**Classification:** canonical evidence boundary and ordinary Texture mutation behavior  
**Mutation class of a repair:** red

## Observation

Independent review of the deletion-first Texture cleanup candidate at staged diff
`d603984f9c609dd82d5456b88816e3f96eba58e5901b3a41bff56b3be54045f0`
rejected its compact audit integration while accepting the supervision-OS
removal itself.

The audit append occurs after the ordinary Texture or lifecycle mutation has
committed. Several callers propagate an append failure as the product mutation's
failure. A transient pin, CAS, or appender failure can therefore produce an HTTP
500 or tool error after the document head, lifecycle trajectory, title, or
archive state already changed. Retrying a non-lifecycle revision can mint a
second revision. Evidence availability has become a false second success
authority without atomic fate-sharing.

The event idempotency key is also derived only from the command ID while the
event store scopes idempotency to the computer. Lifecycle command identity is
owner-scoped, so two owners on one computer can legitimately reuse a command ID
and collide. Title audit identity is derived from the target title rather than a
committed mutation identity, so `A -> B -> A` can suppress the second `A` event
or reject it as a changed command.

Finally, merge acceptance and historical revision restore are significant
owner-visible revision commits but do not invoke the compact audit path.

## Evidence

- `internal/textureowner/texture.go`: post-commit audit errors are returned from
  title, archive, lifecycle revision, and ordinary revision handlers.
- `internal/textureowner/tools_texture.go`: post-commit audit errors are returned
  before mutation completion.
- `internal/textureowner/coagent_route.go` and
  `internal/textureowner/texture_handoff.go`: lifecycle start can commit and then
  be reported failed by the audit append.
- `internal/agentcore/texture_audit.go`: event idempotency is based on command ID.
- `internal/store/computer_events.go`: idempotency lookup is computer-scoped.
- `internal/textureowner/texture_merge.go` and the historical restore path in
  `internal/textureowner/texture.go`: revision commits lack the audit call.
- Frozen independent review outputs:
  `/tmp/choir-cleanup-consensus-2/omp-gpt56-sol.out`,
  `/tmp/choir-cleanup-consensus-2/codex.out`, and
  `/tmp/choir-cleanup-consensus-2/cursor.out`.

## Required Repair Boundary

Keep audit append-only and non-authoritative:

1. Once the semantic mutation commits, audit failure must be logged and must not
   change the mutation's external success result.
2. Derive private event idempotency from a deterministic digest including the
   owner, computer, action, document, and committed mutation/command identity.
3. Use committed mutation identity for title changes, including `A -> B -> A`.
4. Cover merge acceptance and historical restore with the same non-gating
   `revision_committed` audit.
5. Add failure-path, cross-owner, title-cycle, merge, and restore contract tests.

Do not add an outbox, reservation protocol, write-mode switch, projection, or
second state authority. Stronger audit fate-sharing is outside this cleanup and
would require separate owner direction.

## Protected Surfaces And Rollback

Protected surfaces are canonical `ComputerEventAppender` writes, encrypted
private payload pinning, and ordinary Texture mutation result semantics.
Rollback is a source revert of the compact audit hook and its tests; no schema,
route, or persisted Texture state migration is authorized.

## Heresy Delta

- `discovered`: post-commit evidence failure acting as product-write failure;
  computer-scoped idempotency collisions; target-value identity suppressing a
  later mutation; missing significant mutation coverage.
- `introduced`: none by this evidence checkpoint.
- `repaired`: none; documentation precedes repair.
