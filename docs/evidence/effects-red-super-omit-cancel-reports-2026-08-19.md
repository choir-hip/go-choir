# Effects Super omit claimed cancel reports — 2026-08-19

**Boundary:** deployed re-probe. Not freeze. Not promote. No live send.
No restore. No HTTP Super-start / operations POST. No
`maxToolLoopIterations` patch. Super `f515dd0f` left running.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Repair:** `9a55b756` (`fix(agentcore): omit claimed cancel reports from
replacement Super`). Replacement Super stamps
`cosuper_replacement_omit_reports` and skips those claimed
producer_reports in `pendingCoagentUpdatesForRun`. Claim retirement
requires both `cosuper_replacement_requested` and omit-reports, so
Super `e4141127` (assign prompt, still dumped cancel bodies) cannot
pin authorship closed.

**Prior diagnosis:** `docs/evidence/effects-red-super-assign-prompt-max-tokens-2026-08-19.md`
(`aa727aa6`).

## Deploy identity

Push CI https://github.com/choir-hip/go-choir/actions/runs/32310498242
succeeded, including Deploy to Staging (Node B) job `96256756889`.
Heresy Detector finished in 40s.

Staging `/health` at 2026-08-19T23:18Z:

- proxy `deployed_commit=9a55b75636a8104d1033845799fbacc7b68afdf4`
- `deployed_at=2026-08-19T23:18:07Z`

Authenticated `GET https://choir.news/` before owner refresh still
served autoputer `5e01ac3a5ab8d699cc65eed1ebde6b66bc08e545`. Guest
image changed; retained computer stayed epoch **329** until refresh.

## Owner refresh (guest image changed)

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer refresh \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --idempotency-key effects-super-omit-reports-refresh-2026-08-19T2319Z
```

LifecycleReceipt `01a01c52-84bd-70dc-9161-5157553e900b`:
`active@329` → `active@330`. No HTTP operations POST.

Authenticated `GET https://choir.news/` with
`X-Choir-Computer: computer-03335285269bdba4f94377e56879f9e6`
returned HTTP 200; `x-choir-build-commit` is
`9a55b75636a8104d1033845799fbacc7b68afdf4` (`autoputer`).

Mode remains `propose_only` generation 1. Effects remain OFF.
Pre-A checkpoint `99949fe2` remains the restore fence.

## Replacement Super (no HTTP Super-start)

Boot reconcile unpinned `e4141127` and started one Super without a
new Texture `execution_request` and without HTTP operations POST.

| Actor | ID | State | Times |
|---|---|---|---|
| Omit-reports Super | `f515dd0f-ae2a-4bf4-9a64-4cbbf9f6ea02` | `running` | 23:19:19Z– (updated 23:19:24Z) |
| Prior dump Super | `e4141127-26aa-44d2-b08a-5a1995a0e2df` | `failed` | 22:35:29Z–22:38:16Z |
| CoSuper | `run:assignment-97191e37-657c-5acf-af18-f1c80d09def2` | `cancelled` | 17:45:16Z–17:57:48Z |

`f515dd0f` metadata (verbatim fields at 23:19:35Z):

- `request_source=lifecycle_texture_control`
- `requested_by_agent_id=co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2`
- prompt: `Prior implementation CoSuper assignment is terminal. Open a fresh implementation CoSuper assignment with assign_co_super.`
- `cosuper_replacement_requested=true`
- `cosuper_replacement_omit_reports=true`
- `producer_report_ids`: nine cancel-report IDs
- `worker_update_ids`: null

No new CoSuper through 23:30:53Z. Newest Super remains `f515dd0f`.
No restorm. Operation `selfdev-ccf0f1ec0e851750f253fe5f5ed97974`
remains `executing`, empty `bundle_digest`, `updated_at` still
17:45:05Z.

`updated_at` stayed `23:19:24Z` for 11+ minutes while state stayed
`running` — first tool-loop iteration has not committed. Do not
cancel this Super.

## What this proves

- `e4141127` omit-less claim did not pin authorship closed.
- One omit-reports Super started with the assign prompt.
- No restorm from the prior dump Super.

## What this does not prove

- Super `f515dd0f` has not called `assign_co_super` yet.
- Capsule authorship of solitaire is unpaid.

## Forbidden

- freeze / propose / promote
- live send / restore
- HTTP Super-start / operations POST
- raising `maxToolLoopIterations` or `maxTokenContinuationRetries`
- SQL-empty or replace the retained computer
- cancelling Super `f515dd0f`

## Rollback

`git revert 9a55b756`. Refresh receipt `01a01c52` is a forward
record. Checkpoint `99949fe2` remains the pre-A fence. This file
is docs-only.
