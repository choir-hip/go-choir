# Effects red pre-A checkpoint ineligible at epoch 272

**Boundary:** execute (route map 9 red). Not live proof. Not a live send. Super was not started.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `81952e13` (named propose_only / qualified_consensus CLI)
**Deploy:** `https://choir.news/health` 2026-08-16T05:52Z still `deployed_commit` `5557840c`
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## Named start prompt (not presented)

The Definition `finish.deliver` already names the solitaire candidate. The start prompt, when ModeReceipt is later presented, is:

Author, build, test, freeze, and propose a reversible source change on this computer: solitaire with a headless play API, durable persistence, and play history. Do not ship a web UI.

Select policy `reversible-selfdev-v1` (digest `c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7`) before any panel outputs. Freeze eligible seats and independence domains before verification outputs exist.

Do not send email. Do not call `outbox.send`. Do not rematerialize. Do not invent `choir computer create`. Do not use OwnerRecovery checkpoints for promotion. Do not delete `external-owner:`, `accept_once`, or `awaiting_approval`. Genesis stays disabled. Outbox remains unarmed.

Assigned CoSuper may freeze via `commit_transaction`, `inspect_self_development_bundle`, and `record_self_development_verification`. Do not self-promote. Propose the frozen bundle and wait for `qualified_consensus` bound to that exact subject.

Blast radius is this computer's VM-local solitaire tables and release pointer. Platform store, cycle state, other computers, trusted outbox, and host frontend are out.

This receipt does **not** POST that prompt. Presenting the signed ModeReceipt would launch Super.

## Credential expiry then owner-scoped refresh

First `choir computer checkpoint` at epoch 271 returned 409 `guest credential: renewal refused`. Guest event capabilities TTL 5 minutes; renewal grace 60 seconds. Mode GET still worked because platform-control mode CAS is a different capability.

`choir computer refresh --computer computer-03335285269bdba4f94377e56879f9e6 --idempotency-key effects-red-pre-a-refresh-2026-08-16T05:55Z`

| Field | Value |
|---|---|
| receipt_id | `01a00920-e65a-72cb-8e61-f545fe7d6965` |
| prior | active / epoch **271** |
| resulting | active / epoch **272** |
| rematerialize | not invoked |
| restart | not invoked |
| mode | still **`propose_only` generation 1** receipt `01a0091b-bf12-771e-97e7-9a42752ad036` |

## After refresh (2026-08-16T05:52Z)

| Call | Result |
|---|---|
| `GET .../mode` | 200 `propose_only` generation 1 |
| `GET .../kernel-capabilities` | 200 signed KernelCapabilityReceipt boot_id `e76f63dc-f3ca-40e4-bbf7-59994058d1de` |
| `POST .../genesis` | 409 `self-development effects are disabled` |
| `choir computer status` | active, epoch **272** |
| `choir computer replay-completeness` | 200; live_head == replay_head sequence 26; `eligibility.eligible=false` |
| `choir computer checkpoint` | 409 `replay is ineligible: behavior-bearing direct-write tables are non-empty without reducers` |

Unsupported non-empty `ReplayEmptyUntilSupported` tables: `desktop_app_instances`, `desktop_sessions`, `desktop_window_placements`, `desktop_workspaces`, `og_objects`.

The gate is correct. Weakening it would create a checkpoint while behavior-bearing local rows are not event-derivable (`finish.not_done_when`). Desktop rows were not deleted. The paid tape-recovery `checkpoint_witness` (`70f9ce2b…`, epoch 261, `owner_recovery: true`) is not a promotion authority and is not a current pre-A fence for this excursion.

No mail was sent. Restore was not invoked. Rematerialize was not invoked. Super was not started. Owner gates were not deleted. Outbox `Armed=false`.

## What this is not

This is not red promote+restore. This is not permission to start Super. This is not permission to empty desktop tables by hand. This is not permission to treat OwnerRecovery checkpoint `70f9ce2b` as the effects restore fence.

## Next

Do not present the ModeReceipt until a restore-eligible pre-A checkpoint can be taken without weakening eligibility, rematerializing, or using OwnerRecovery for promotion. Keep genesis 409. Keep Armed=false. Do not send live mail.
