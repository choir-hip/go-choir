# Effects Super assign-prompt continuation on 5e01ac3a — 2026-08-19

**Boundary:** deployed re-probe. Not freeze. Not promote. No live send.
No restore. No HTTP Super-start / operations POST. No
`maxToolLoopIterations` patch.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Repair:** `5e01ac3a` (`fix(agentcore): continue Super from CoSuper cancel
with assign_co_super`). Report-continuation Super uses the Texture
rewake note as prompt and stamps `cosuper_replacement_requested`.
`claimedPersistentSuperProducerReportIDs` ignores terminal claims
that never requested a replacement, so pre-repair Super `bab919a0`
cannot pin authorship closed.

**Prior diagnosis:** `docs/evidence/effects-red-super-200-iter-without-assign-2026-08-19.md`
(`d85dad01`).

## Deploy identity

Push CI https://github.com/choir-hip/go-choir/actions/runs/32307105221
succeeded, including Deploy to Staging (Node B) job `96247041856`.
Heresy Detector finished in 37s. No race-shard Dolt flake.

Staging `/health` at 2026-08-19T22:34Z:

- proxy `deployed_commit=5e01ac3a5ab8d699cc65eed1ebde6b66bc08e545`
- `deployed_at=2026-08-19T22:34:00Z`

Authenticated `GET https://choir.news/` before owner refresh still
served autoputer `3654d9255606cf90f76a213cca1bbe3bba142d35`. Guest
image changed; retained computer stayed epoch **328** until refresh.

## Owner refresh (guest image changed)

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer refresh \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --idempotency-key effects-super-assign-refresh-2026-08-19T2235Z
```

LifecycleReceipt `01a01c2a-60d4-7dc4-9b06-b9978921029e`:
`active@328` → `active@329`. No HTTP operations POST.

Authenticated `GET https://choir.news/` with
`X-Choir-Computer: computer-03335285269bdba4f94377e56879f9e6`
returned HTTP 200; `x-choir-build-commit` is
`5e01ac3a5ab8d699cc65eed1ebde6b66bc08e545` (`autoputer`). Proxy
`/health` reports the same SHA.

Mode remains `propose_only` generation 1, ModeReceipt
`01a0091b-bf12-771e-97e7-9a42752ad036`. Effects remain OFF.
Pre-A checkpoint `99949fe2` remains the restore fence.

## Replacement Super (no HTTP Super-start)

Boot reconcile on the new guest started one continuation Super
without a new Texture `execution_request` and without HTTP
operations POST.

| Actor | ID | State | Times |
|---|---|---|---|
| Replacement Super | `e4141127-26aa-44d2-b08a-5a1995a0e2df` | `failed` | 22:35:29Z–22:38:16Z |
| Prior claimed Super | `bab919a0-3e05-4860-bce6-88f040698db9` | `failed` | 21:36:19Z–21:48:33Z |
| CoSuper | `run:assignment-97191e37-657c-5acf-af18-f1c80d09def2` | `cancelled` | 17:45:16Z–17:57:48Z |

`e4141127` metadata (verbatim fields):

- `request_source=lifecycle_texture_control`
- `requested_by_agent_id=co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2`
- prompt: `Prior implementation CoSuper assignment is terminal. Open a fresh implementation CoSuper assignment with assign_co_super.`
- `cosuper_replacement_requested=true`
- `producer_report_ids`: nine `assignment-report:cancel-report:sha256:…` IDs
- `worker_update_ids`: null
- `lifecycle_work_item_id=fd43ecca-cb82-53cf-91b5-dbe6f2412f97`
- `assignment_trajectory_id=5242ca03-7513-5809-be58-4d43cbeab18f`

Error: `tool loop: model stopped at max_tokens after 3 continuation attempts (iteration 7)`.

No new CoSuper. Newest Super at 22:42:10Z is still `e4141127`.
No restorm. Operation `selfdev-ccf0f1ec0e851750f253fe5f5ed97974`
remains `executing`, empty `bundle_digest`, `updated_at` still
17:45:05Z.

## What this proves

- Pre-repair claims from `bab919a0` did not pin authorship closed.
- One replacement Super started with the assign prompt and
  `cosuper_replacement_requested`.
- A flagged replacement Super does not restorm after terminal.

## What this does not prove

- Super `e4141127` never called `assign_co_super`. Capsule
  authorship of solitaire is unpaid.
- Prompt change is not sufficient while nine cancel reports are
  still cold-injected. See
  `docs/evidence/effects-red-super-assign-prompt-max-tokens-2026-08-19.md`.

## Forbidden

- freeze / propose / promote
- live send / restore
- HTTP Super-start / operations POST
- raising `maxToolLoopIterations`
- SQL-empty or replace the retained computer
- cancelling Super to force a new assignment

## Rollback

`git revert 5e01ac3a`. Refresh receipt `01a01c2a` is a forward
record. Checkpoint `99949fe2` remains the pre-A fence. This file
is docs-only.
