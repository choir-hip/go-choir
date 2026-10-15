# Overnight landing, 2026-10-15 (briefing, in progress)

Written by the overnight lander at 09:30 UTC; updated at the end of the run.
Staging https://choir.news; paper computer
`vm-1669020adcece51c2f5ff9b6df399ef5`. Dolt compaction, memory hotplug, patch
lanes and P6-P8 have not landed yet (see Skipped / pending).

## What landed

| step | commit on main | what | CI | deploy | proof |
|---|---|---|---|---|---|
| 0 | `f704110ce` (code `da80c5fcc`, raced-activation fix 2) | Texture wake reads `agent.ActiveRunID` | `da80c5fcc` run: shard 6 hung in `internal/server TestHealthHandlerIncludesAddrAfterStart` (unrelated flake), rerun green, then SBOM accept failed because the docs-only `f704110ce` run had advanced the baseline; the `f704110ce` run was fully green and deployed the same code | staging `f704110ce` | tracer `selfdev-jobs-0654`: FAIL (see below), but `recovered raced activation` fired and no poison |
| 0b | `73b14c327` | Autopaper cache routing: chatgpt session/conversation headers, `cached_in=` log field, `prompt_cache` on gateway `/health`. Owner ruling applied before merge: search trimming and run-memory elision are **off by default** (`6f7a764f4`, merged via branch `land/halving-append-only`, because `feat/autopaper-compute-halving` is checked out in another worktree) | green | `73b14c327`, 07:53Z | see cache table |
| 1 | `9c62b209f` | S1 owner-act receipts, observe mode | green | `9c62b209f`, 08:05Z | proxy `owner-act key host-owner-act-4323d8e7e60998de loaded`; corpusd `owner acts: mode=observe`; proxy and corpusd active; `https://choir.news/` 200 |
| 2 | `2eae77de6` | P5 host build and layered apply. The merge broke the build (`restoreRestartBusy` called the old `plannedRestartBusy` signature); fixed in the same push | green | `2eae77de6`, 08:23Z | corpusd `host build: enabled (source /opt/go-choir, installable .#autoputer)`; tracer `p5-go-served-0825` in progress at the time of writing |

## Tracer results

- `selfdev-jobs-0654` (Go drill, build `f704110ce`): `pass=false`,
  `reached=engineering_final_report`. The engineering assignment ran 22
  minutes and was cancelled with no bundle. A capsule cell hit its 30 s timeout
  (`broker.sock: i/o timeout`) and the next call hung 13 minutes. This is the
  jobs-0101 item 2 class (capsule broker RPC desync); its repair
  (`5c5db2027`) is on `fix/selfdev-go-drill-0101` and is **not on main**. The
  raced-activation fix worked: `recovered raced activation` appeared, no
  `dispatcher: poison`, no `mutation authority mismatch`. Problem doc:
  `docs/problems/selfdev-jobs-0654-raced-activation-verified-engineering-budget-2026-10-15.md`.
  Judged not a new failure class, so landing continued.
- `p5-go-served-0825` (P5, build `2eae77de6`): still running when this was
  written; first engineering assignment ended `failed`, Texture re-cast it
  (round 5). Result is appended below if it finished.

## Cache ratio before and after 0b (paper VM, chatgpt vs opencode-go, cached/input per UTC hour)

Before the deploy (07:53Z): chatgpt 03h 23.6%, 04h 37.5%, 05h 42.1%, 06h
39.5%; opencode-go 75-84%.

After the deploy (calls are hourly sums; the 07h row mixes both builds):

| hour | chatgpt calls | chatgpt cached% | opencode-go cached% |
|---|---|---|---|
| 07 | 542 | 35.9 | 81.3 |
| 08 | 728 | 74.7 | 84.5 |
| 09 (partial) | 321 | 87.6 | 89.3 |

Gateway `/health` `prompt_cache` (since the S1 restart at 08:05Z):
chatgpt 81.5% (961 calls), opencode-go 87.5% (467 calls). The step is the
session/conversation header routing; no economizing lever is active.

## Paper health

Paper booted on `2eae77de6` (epoch after each deploy), `status=ready`,
persistent disk 35.7% used (about 16 GiB dolt), below the 75% stop line; no
deploy-time regressions seen.

## Skipped or pending (as of writing)

- Section-page fix `fix/autopaper-section-pages`: queued next at the
  coordinator's direction.
- P4 standing consensus, memory hotplug (known conflict in
  `internal/agentcore/api.go`), Dolt compaction, patch lanes, P6-P8
  (known conflicts in `api.go`, `api_self_development.go`,
  `platform/handlers.go`, `proxy/self_development.go`,
  `scripts/reeng_tracer.mjs`): not yet landed.
- Not landed by instruction: land/account-recovery, cleanup/surface-sweep,
  feat/visual-evidence-slice, feat/s7-0-app-packages,
  land/desk-cell-confinement.

## Needs the owner

- Kernel reboot this morning.
- Keenable overflow spend decision.
- Exa re-enable.
- Account-recovery ruling (`land/account-recovery`).
- S7 decision D1.
- `PATCH_BOT_TOKEN` secret.
- Enable GitHub private vulnerability reporting.
- Whether to land `fix/selfdev-go-drill-0101` (capsule broker re-dial) so the
  self-development Go drill can reach apply and restore.
- S1 stays in observe; flip `ownerActMode` to enforce after a clean window.
