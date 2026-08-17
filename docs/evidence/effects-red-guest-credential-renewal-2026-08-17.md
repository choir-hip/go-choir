# Effects operations retry blocked by guest credential renewal — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` reports `cec68e23afda3b2bc2554384eeb9f87e160faf5f`.

## State

CoSuper `run:assignment-fa38b037` is terminal (`cancelled`, capsule revoked) after `cec68e23`. The operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` is still `executing` with no bundle. The next defined action is to retry the same operations POST.

## Retry attempt and result

The exact POSTed prompt was recovered (the first Super run `cdf0af4c`'s prompt minus the 204-byte "Self-development operation … Preserve this exact operation identity …" header) and verified: recomputing `request_commitment = sha256(computerID + "\x00" + idempotency_key + "\x00" + sha256(prompt))` matches the stored `9c27a75a50e407d658c815156e7ba6e114aae2c0336b35f1ac85c113e73044c4`.

`POST /api/computers/{computer}/self-development/operations` with idempotency key `effects-solitaire-start-2026-08-16T20:08Z` and that prompt returned HTTP 409:

```
unbind terminal self-development Super: store: append projection batch: computer event appender: resolve head for new event: computer event client: capability: guest credential: renewal refused
```

## Blocker

`ensureSelfDevelopmentRun` correctly attempts to unbind the terminal self-development Super and start a fresh one, but the unbind's `UpdateRun` projection append cannot resolve the new event head: the computer event client's guest credential capability renewal is refused by the platform. This is the auth/session-renewal protected surface. It is outside the product-API path.

Operation stays `executing`, no bundle. Constructed freeze `7122f279` unchanged. Mode `propose_only` generation 1. No mail.

## Next

Owner/platform action: resolve why the guest credential capability renewal is refused (expired credential, or corpusd renewal authority), or grant the renewal. Do not self-promote, CAS `qualified_consensus`, send mail, or use OwnerRecovery `663540be` for promotion.
