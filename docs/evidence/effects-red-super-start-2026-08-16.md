# Effects Super start with named solitaire prompt — 2026-08-16

**Boundary:** execute (route map 9 red). Not live proof. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `a9e4af419aa96018410cb13840cc0ee94afe39cb` (`deployed_at` 2026-08-16T20:03:42Z)
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **276**

## Actuation

Pre-A OwnerRecovery checkpoint `663540be` was already paid at sequence 32. Mode was already `propose_only` generation 1. This slice presented that signed ModeReceipt with the named solitaire prompt. Mode was not CAS'd.

`POST /api/computers/computer-03335285269bdba4f94377e56879f9e6/self-development/operations`

Idempotency `effects-solitaire-start-2026-08-16T20:08Z`. HTTP **201**.

| Field | Value |
|---|---|
| operation_id | `selfdev-b090bcd72d300fed17cb3f5a142f8595` |
| request_commitment | `9c27a75a50e407d658c815156e7ba6e114aae2c0336b35f1ac85c113e73044c4` |
| trajectory_id | `trajectory-6235753c4abf1d67789796e165736f91` |
| base_head | `0ac0e8dea679e0de4ac3cbd4bf9e5cf1f8b1eddb20a23e17a5e3c8e7c8b0c084` |
| desired_head / effective_head | `a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44` |
| prompt_artifact_ref | `artifact:sha256:8b0f31715e9ee3c675785414042aa62ada26c6063313d237e25cb8ce20a41eb2` |
| state | `executing` |
| created_at | 2026-08-16T20:07:13.099981Z |
| verifier_refs | empty |
| bundle_digest | empty |
| checkpoint_ref | empty |

Prompt (posted): solitaire with a headless play API, durable persistence, and play history; no web UI; select `reversible-selfdev-v1` (`c34ddf07…`) before panel outputs; do not send email; do not self-promote; wait for `qualified_consensus`.

## After start

| Call | Result |
|---|---|
| operation GET | 200 `executing` |
| mode GET | 200 `propose_only` generation 1 |
| genesis POST | 409 `self-development effects are disabled` |
| `choir computer status` | active, epoch **276** |
| outbox | not armed; no mail sent |

This is not a frozen bundle. This is not qualified-consensus CAS. This is not promotion. OwnerRecovery checkpoint `663540be` remains inadmissible for promotion. Super must freeze and propose; it must not self-promote.
