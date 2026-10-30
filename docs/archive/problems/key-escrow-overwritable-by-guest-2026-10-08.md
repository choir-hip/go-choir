# A computer's escrowed privacy key can be overwritten with a different key

Date: 2026-10-08. Found while designing station SH slice 2 (guaranteed
escrow) against operational invariant O21. Mutation class of any fix: red
(privacy key custody).

## Evidence (code reading)

- `internal/platform/key_escrow.go` `UpsertKeyEscrow`:
  `INSERT … ON DUPLICATE KEY UPDATE wrapped_key_json=VALUES(...), key_digest=VALUES(key_digest)`.
  The key digest of an existing escrow record is replaced, not checked.
- `internal/platform/key_escrow_http.go` `HandleKeyEscrow` (PUT) accepts the
  write from any caller holding the computer's `event:append` capability
  (any realization of that computer) or an internal caller.
- The guest's lazy upload (`internal/autoputer/key_escrow.go`
  `EnsureCustodianEscrow`) skips when a custodian record exists, so the
  normal path never overwrites. Nothing on the platform side enforces it.

Staging 2026-10-08: all 113 computers with a canonical chain have a
custodian escrow record (`computer_key_escrows`), so the escrow copy is
now the only off-realization copy of each key.

## Problem

Under O21 the escrow record is the durable home of the key. A realization
that holds a different key (a bug that mints a fresh key, a compromised
guest, or a future desktop realization) can replace the escrowed wrap. The
computer's encrypted history then becomes unreadable from escrow
(checkpoint replay, delivery to a new realization, two-approval reveal)
while everything still looks escrowed. Nothing records the replacement in
the key-escrow transparency log.

## Fix direction

Escrow is write-once per (computer, protector, key digest): re-uploading
the same digest is idempotent; a different digest is refused (409) and
never replaces the record. Key rotation, when it exists, is a separate
audited operation that keeps the old wrap.

## Status (2026-10-09)

Fixed in `c990bf50` + `001e73cb` (write-once escrow once a chain exists;
transactional pre-genesis replacement with a transparency entry; guest
compares the escrowed digest; escrow required before genesis). Deployed
`001e73cb`. Staging: fresh signup `computer-26e96cd2…` escrowed at
00:40:36 with genesis at 00:40:36.26; chained computers without escrow: 0.
Evidence: [`evidence/sh-slice2-panel-fixes-staging-2026-10-09.log`](../evidence/sh-slice2-panel-fixes-staging-2026-10-09.log)
(same run re-verifies the slice-1 refusal with Retry-After 300). Status:
**fixed-verified** for escrow-before-genesis; write-once 409 path is
unit-verified only.
