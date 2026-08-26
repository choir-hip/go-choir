# Track K Keys Acceptance — Deployed Proof on Staging (2026-08-26)

- **Definition:** docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md, Track K — Key Escrow & Wrap Hierarchy
- **Evidence class:** deployed proof on test computer (staging)
- **Commits:** `f0e68b0a` (custodian escrow: host wrap table, two-approval gate, guest lazy per-boot escrow), `983b8995` (passkey PRF 1b), `28d352c1` + `84e4daee` (guest capability auth fixes), `84ca50e6` (operator credentials wiring)
- **Deployed identity at proof time:** staging `/health` `deployed_commit 84e4daee`; corpusd :8086; guest runtime package `84e4daee`

## Acceptance item 1 — DEK escrowed on computer creation

The guest generates its 32-byte XChaCha20 DEK exactly as before
(`internal/computerevent/privacy.go`, crypto unchanged). After cipher load the
guest now seals the DEK under the host X25519 escrow public key
(`internal/keyescrow`: ephemeral ECDH → HKDF-SHA256 → XChaCha20-Poly1305,
computer identity as AEAD associated data) and uploads the wrap:

```
GET /internal/computers/keys/escrow/status?computer_id=computer-bb0f4fa583c0cde14334818d946e6378
{"escrows":[{"protector":"custodian",
 "key_digest":"914ea9b263a705d93253b3deeddda7383b2568f71183d26a73c4eed2089473db",
 "escrowed_at":"2026-08-26T11:26:10Z"}]}
```

Guest console: `autoputer: custodian key escrow uploaded for
computer-bb0f4fa583c0cde14334818d946e6378`. The wrap is bound to this
computer; opening it requires the host escrow private key.

## Acceptance item 2 — backfill of existing active computers

The proof computer `candidate-fleet-49ee3bd0ec6f366a164c02d2`
(`computer-bb0f4fa583c0cde14334818d946e6378`) is a pre-existing active
computer: it was hibernated/resumed onto the new runtime and escrowed on boot
via the lazy per-boot upgrade path. All existing active computers receive the
same upgrade on their next boot; no big-bang re-key (design §3.2 migration
clamp).

## Acceptance item 3 — recovery under two-approval gate

Operator credentials are deploy-created outside git
(`/var/lib/go-choir/secrets/corpusd-escrow-operators.env`, mode 600; wired via
`EnvironmentFile` in nix/node-b.nix commit `84ca50e6`). Ceremony against
staging corpusd (:8086), all results verbatim:

| Step | Result |
|---|---|
| Create unwrap request (op1) | `201`, request `11117481-…`, requested_by pinned to authenticated operator |
| Self-approval (op1 approving own request) | `403 {"error":"self approval is not allowed"}` |
| Approval by op2 | `200` |
| Premature reveal after 1 approval | `409 {"error":"unwrap request is not approved"}` |
| Approval by op3 | `200` |
| Reveal | DEK returned; `sha256(dek) == 914ea9b2… == escrow record digest`, 32 bytes |

Repeat ceremony (`request d5c9459d-…`, requester op3): self-approval `403`,
approvals op1+op2, reveal verified identical digest match.

Transparency log advanced `{"head_hash":"","seq":0}` → seq 2 with chained
hashes (`4d041cc6…`, `534966c5…`); every reveal appends an entry carrying
request_id, computer_id, key_digest, approvals, revealed_at. The gate is
closed (503) when no operators are configured — observed live before
credentials were provisioned.

## Acceptance item 4 — passkey PRF derivation wrap (Track K 1b)

Implemented and proven at handler/unit level (`983b8995`):
`DerivePRFWrapKey` (HKDF over the authenticator PRF secret, owner-bound salt),
`SealRoot/OpenRoot`, registration PRF availability probe, login-time root mint
+ wrap + unwrap ceremony, self-approval-free by construction (ROOT wraps are
per-credential). Tests: TestPasskeyPRFRootWrap, TestOwnerRootWrapPersistence,
TestLoginFinishPRFRootUnwrap, TestRootStatusIsOwnerScoped.

**Not yet deployed-proven:** a real authenticator exercising the browser
ceremony against staging (CDP virtual authenticator supports `hasPrf`;
go-webauthn v0.16.4 carries PRF through generic extension maps). This is the
residual gap between unit-proven and deployed-proven for protector 1.

## Incidents during proof (fixed in-flight)

1. Guest escrow endpoints initially required `X-Internal-Caller`, which guests
   cannot carry → 403 on first boot (`28d352c1` added signed-capability auth).
2. A stale internal-only guard survived ahead of the capability check → still
   403 (`84e4daee`). Both caught by this deployed acceptance loop.
3. First dump-style SQL full-scan OOM-killed the dolt server earlier the same
   day (separate receipt); unrelated to Track K.

## CI note (root-cause clustering candidate)

`internal/actorruntime TestAdapterSQLiteInjectionAppendRecoveryExecutesWithoutSnapshot`
failed twice in CI today with SQLITE_BUSY under shard contention (passed locally
both times); plus one earlier trajectory-test Dolt timeout flake. Three
same-class CI substrate flakes in two days meets the AGENTS.md clustering
threshold — assessment owed before the next landing.
