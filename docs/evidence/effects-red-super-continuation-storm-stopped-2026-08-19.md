# Effects Super continuation storm stopped — 2026-08-19

**Boundary:** deployed re-probe. Not freeze. Not promote. No live send.
No restore. No HTTP Super-start / operations POST. No
`maxToolLoopIterations` patch.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Repair:** `3654d925` (`fix(agentcore): do not restorm Super from
claimed CoSuper producer_report`). Continuation Super records
`producer_report_ids`; `listPendingPersistentSuperAdmissibleReports`
skips IDs claimed by a terminal `lifecycle_texture_control` Super.
Cancel reports stay undelivered for the 2026-08-18 injector.

**Prior diagnosis:** `docs/evidence/effects-red-super-continuation-storm-after-cosuper-cancel-2026-08-19.md`
(`28bdd3b5`).

## Deploy identity

Push CI `32293816782` hung on Heresy Detector `Install ripgrep` for
66 minutes (no logs) and was cancelled so a dispatch could start.
Force-deploy dispatch `32299981780` failed on the known
`TestCancelRunTrajectoryDrainsMoreThanOneActivePage` Dolt scan
deadline (`trajectory_test.go:336`). Retry dispatch
https://github.com/choir-hip/go-choir/actions/runs/32302197967
succeeded, including Deploy to Staging (Node B) job `96231708715`.

Staging `/health` at 2026-08-19T21:34Z:

- proxy `deployed_commit=3654d9255606cf90f76a213cca1bbe3bba142d35`
- `deployed_at=2026-08-19T21:33:35Z`

Authenticated `GET https://choir.news/` before owner refresh still
served autoputer `51b18f5440d9b3acec2713f71786db695263c37c`. Guest
image changed; retained computer stayed epoch **327** until refresh.

## Owner refresh (guest image changed)

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer refresh \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --idempotency-key effects-super-storm-claim-refresh-2026-08-19T2135Z
```

LifecycleReceipt `01a01bf4-2a0a-7d36-b7a8-d83a659299b7`:
`active@327` → `active@328`. No HTTP operations POST.

Authenticated `GET https://choir.news/` with
`X-Choir-Computer: computer-03335285269bdba4f94377e56879f9e6`
returned HTTP 200; `x-choir-build-commit` is
`3654d9255606cf90f76a213cca1bbe3bba142d35` (`autoputer`). Proxy
`/health` reports the same SHA.

Mode remains `propose_only` generation 1, ModeReceipt
`01a0091b-bf12-771e-97e7-9a42752ad036`. Effects remain OFF.
Pre-A checkpoint `99949fe2` remains the restore fence.

## Claimed continuation Super (no HTTP Super-start)

Boot reconcile on the new guest started one continuation Super
without a new Texture `execution_request` and without HTTP
operations POST.

| Actor | ID | State | Times |
|---|---|---|---|
| Claimed Super | `bab919a0-3e05-4860-bce6-88f040698db9` | `failed` | 21:36:19Z–21:48:33Z |
| Prior storm Super | `c6bd2000-84d6-451d-ba1b-bb67cd9de85e` | `passivated` | 21:21:48Z–21:36:16Z (refresh) |
| CoSuper | `run:assignment-97191e37-657c-5acf-af18-f1c80d09def2` | `cancelled` | 17:45:16Z–17:57:48Z |

`bab919a0` metadata (verbatim fields):

- `request_source=lifecycle_texture_control`
- `requested_by_agent_id=co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2`
- prompt: `Process pending coagent update packets for privileged execution.`
- `lifecycle_work_item_id=fd43ecca-cb82-53cf-91b5-dbe6f2412f97`
- `assignment_trajectory_id=5242ca03-7513-5809-be58-4d43cbeab18f`
- `work_item_ids`: `fd43ecca`, `4671d318`, `38b96770`
- `worker_update_ids`: null (no Control bind)
- `producer_report_ids`: nine `assignment-report:cancel-report:sha256:…` IDs

Error: `tool loop: exceeded 200 iterations without end_turn`.

## No restorm

Pre-repair interval was one second (`b57705fd` failed 19:11:37Z →
`999bd208` created 19:11:38Z).

`bab919a0` failed 21:48:33Z. Polls at 21:48:53Z, 21:49:59Z, and
21:50:32Z show it remains the newest Super. No successor Super
from CoSuper `97191e37`. No HTTP operations POST. No new CoSuper.

Operation `selfdev-ccf0f1ec0e851750f253fe5f5ed97974` remains
`executing`, empty `bundle_digest`, `updated_at` still 17:45:05Z.

## What this does not prove

- Super `bab919a0` still 200-failed without `assign_co_super`.
  Capsule authorship of solitaire is unpaid.
- `maxToolLoopIterations=200` remains. Do not raise it.
- Freeze, five bundle refs, proposal, consensus, promotion, mail,
  restore remain unpaid.

## Residual

- Super 200-iter looping without `assign_co_super` (separate from
  the storm).
- `deploy-impact` still classifies last push, not last-deployed SHA.
- Heresy Detector `apt-get install ripgrep` can hang a main CI
  concurrency slot.
- Race-excluded trajectory scale test can Dolt-deadline a
  force-deploy dispatch.

## Forbidden

- freeze / propose / promote
- live send
- restore
- HTTP Super-start / operations POST
- raising `maxToolLoopIterations`
- SQL-empty or replace the retained computer
- cancelling a live Super to force a new CoSuper

## Rollback

`git revert 3654d925`. Refresh receipt `01a01bf4` is a forward
record. Checkpoint `99949fe2` remains the pre-A fence. This file
is docs-only.
