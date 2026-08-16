# Effects restore-eligible pre-A checkpoint — 2026-08-16

**Boundary:** execute (route map 9 red). Not live proof. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `a9e4af419aa96018410cb13840cc0ee94afe39cb` (`deployed_at` 2026-08-16T20:03:42Z)
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## Actuation

CI https://github.com/choir-hip/go-choir/actions/runs/31968160935 succeeded, including Node B deploy. G4 preserved `snapshot_kind=constructed-computer-version`. Constructed freeze remains `7122f2799be4458f4b925be11990321c7e70ffc4`.

Owner-scoped refresh (not rematerialize, not restart):

`choir computer refresh --computer computer-03335285269bdba4f94377e56879f9e6 --idempotency-key effects-checkpoint-baseline-spa-refresh-2026-08-16T20:04Z --host https://choir.news --timeout 8m`

LifecycleReceipt `01a00c2c-efdb-7d1c-b98d-3a13ded2bcd0`: epoch **275 → 276**.

Then:

`choir computer checkpoint --computer computer-03335285269bdba4f94377e56879f9e6 --host https://choir.news`

Replay completeness at the same epoch: live_head == replay_head sequence **32**; `eligibility.eligible=true`; `result.status=equivalent`; no differences.

## Published restore-set

| Field | Value |
|---|---|
| checkpoint_eligible | true |
| checkpoint_digest | `663540be56e6d5c89f5215c50efe219f1e6dea57da5fa1909b00a83121bef3c1` |
| owner_recovery | **true** |
| idempotency | `owner-recovery-0ac0e8dea679e0de4ac3cbd4bf9e5cf1f8b1eddb20a23e17a5e3c8e7c8b0c084` |
| accepted_event_head | `0ac0e8dea679e0de4ac3cbd4bf9e5cf1f8b1eddb20a23e17a5e3c8e7c8b0c084` |
| effective_event_head | `a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44` |
| effective_state_commitment | `40df35913994fab47d2dd2c450a7f9d3958ea639ec9fb2002b8b8073534fe091` |
| event_head_receipt_id | `01a00c2c-ee6e-7824-91e2-beffa0568f6d` |
| release_digest | `0d2b0d61b4f818e2b3ee6f6911f06d5cf00890ff4f6b32bd32e1c299f82c991f` |
| reconstruction_digest | `11ee70e379ea297e46c90fd871550a287778bb0c09b99becd01e87fcc39fb595` |
| frontend derivation | `release` |
| frontend digest | `7ec8cb9fb1f30262f6386499a537c93f62c30c0ee84d0d6874ac0b916841f9e0` |
| code_ref | `code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380` |
| artifact_program_ref | `artifact-program:sha256:9d90c8666a1d9a69f46daca644bb9470505831bb9926e21d2a577d0bd9aa5a6f` |
| reducer_version | 1 |
| verifier fields | absent (owner-recovery class) |
| receipt issuer | corpusd / platform-control `868f96cca8726f99` |
| issued_at | 2026-08-16T20:04:37.605295369Z |

Witness: database `texture`; content_root `5ee628f9e93b4bbcaab6c4f350444e5f1a409943f010d84acd4ed995e126881f`; derivability `4308e3223076d827b958435624812be1e54a87efe0338fdc74069c6f5d48e55b`.

Non-empty projected tables: `computer_event_index`, `computer_event_projection_heads`, `desktop_app_instances`, `desktop_sessions`, `desktop_window_placements`, `desktop_workspaces`, `og_objects`. `desktop_state` remains empty. EmptyUntilSupported was not weakened. Tables were not SQL-emptied.

## After checkpoint

| Call | Result |
|---|---|
| `choir computer status` | active, epoch **276** |
| mode GET | 200 `propose_only` generation 1 |
| genesis POST | 409 `self-development effects are disabled` |
| Super | not started |
| outbox | `Armed=false`; no mail sent |

This checkpoint is the current restore-eligible pre-A fence. It is **OwnerRecovery**. Route projection refuses OwnerRecovery for promotion. Do not use `663540be` as a promotion checkpoint. Paid historical OwnerRecovery `70f9ce2b` (epoch 261) is still not this fence.

Named solitaire prompt remains unposted. ModeReceipt was not presented.
