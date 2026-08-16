# Effects red named mode — propose_only live, qualified_consensus named

**Boundary:** execute (route map 9 red). Not live proof. Not a live send. Not promote. Not restore.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `96dead51` (docs stamp of live refresh after verifier wiring)
**Deploy:** `https://choir.news/health` 2026-08-16T05:46Z `deployed_commit` `5557840c` (`deployed_at` 2026-08-16T05:32:15Z, `built_at` 20260816051712)
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **271**

## Named modes for red promote+restore

| Role | Mode | Policy |
|---|---|---|
| Default / refuse | `off` | no proposal |
| Start / propose | **`propose_only`** | signed ModeReceipt required; no bindings |
| Decision (this Definition) | **`qualified_consensus`** | exact operation, bundle, heads, pending transition, commitments, `policy_digest`, `consensus_receipt_digest`, future UTC expiry |
| Reversible policy | `reversible-selfdev-v1` | digest `c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7`; human_seat absent |
| Owner gate (kept, not this rehearsal) | `accept_once` | exact owner bindings; no consensus fields |
| Not named for start | `audit_only` | does not authorize proposal |

Approve consumes `qualified_consensus` (or `accept_once`) back to `propose_only`. `external-owner:` remains an accepted decision authority until deployed acceptance of consensus. OwnerRecovery remains inadmissible. Genesis stays proxy-409. Outbox `Armed=false`. Restore consume is `choir computer restore` (tape-recovery), never rematerialize-from-tape.

## CLI

`choir self-dev mode set` now sends `qualified_consensus` bindings including `--policy-digest` and `--consensus-receipt-digest`. `accept_once` is unchanged. Test: `TestSelfDevelopmentModeCLIQualifiedConsensusCASBody`.

## Live CAS (2026-08-16T05:46Z)

`choir self-dev mode set --computer computer-03335285269bdba4f94377e56879f9e6 --mode propose_only --expected-generation 0 --idempotency-key effects-red-propose-only-2026-08-16T05:45Z`

| Field | Value |
|---|---|
| old | `off` generation 0 |
| new | **`propose_only`** generation **1** |
| receipt_id | `01a0091b-bf12-771e-97e7-9a42752ad036` |
| issuer | corpusd / platform-control `868f96cca8726f99` |
| Super started | no |
| operation created | no |

## After CAS

| Call | Result |
|---|---|
| `POST .../genesis` | 409 `self-development effects are disabled` |
| `POST .../operations` without `mode_receipt` | 409 `current signed mode does not authorize proposal` |
| `GET .../kernel-capabilities` | 200 signed KernelCapabilityReceipt |
| `GET .../mode` | 200 `propose_only` generation 1 |
| `GET .../operations/operation-red-rehearsal` | 404 |
| `choir computer status` | active, epoch **271** |

No mail was sent. Restore was not invoked. Rematerialize was not invoked. Owner gates were not deleted.

Presenting the signed ModeReceipt on start would create an operation and launch Super. This receipt does **not** do that. Policy for reversible promotion is selected before panel outputs, not before authorship; the next unpaid product slice is still a frozen bundle plus `qualified_consensus` promote+restore.

## What this is not

This is not red promote+restore. This is not route map 10 live proof. Orange in-process rehearsal remains the only promote/outbox composition proof. `propose_only` is proposal authority, not Armed and not a live send.

## Next

Do not start Super until the start prompt and policy-before-outputs path are named for the solitaire candidate. Then freeze, select `reversible-selfdev-v1`, CAS `qualified_consensus`, promote, and consume tape-recovery restore. Keep genesis 409. Keep Armed=false. Do not send live mail. Do not rematerialize. Do not use OwnerRecovery for promotion. Do not delete `external-owner:` / `accept_once` / `awaiting_approval`.
