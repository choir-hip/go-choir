# Overnight landing, 2026-10-15 (briefing, in progress)

Written by the overnight lander at 09:30 UTC; last updated 10:55 UTC. Landing
is paused on a coordinator hold (Node B host kernel reboot).
Staging https://choir.news; paper computer
`vm-1669020adcece51c2f5ff9b6df399ef5`. Dolt compaction, memory hotplug, patch
lanes and P6-P8 have not landed yet (see Skipped / pending).

## What landed

| step | commit on main | what | CI | deploy | proof |
|---|---|---|---|---|---|
| 0 | `f704110ce` (code `da80c5fcc`, raced-activation fix 2) | Texture wake reads `agent.ActiveRunID` | `da80c5fcc` run: shard 6 hung in `internal/server TestHealthHandlerIncludesAddrAfterStart` (unrelated flake), rerun green, then SBOM accept failed because the docs-only `f704110ce` run had advanced the baseline; the `f704110ce` run was fully green and deployed the same code | staging `f704110ce` | tracer `selfdev-jobs-0654`: FAIL (see below), but `recovered raced activation` fired and no poison |
| 0b | `73b14c327` | Autopaper cache routing: chatgpt session/conversation headers, `cached_in=` log field, `prompt_cache` on gateway `/health`. Owner ruling applied before merge: search trimming and run-memory elision are **off by default** (`6f7a764f4`, merged via branch `land/halving-append-only`, because `feat/autopaper-compute-halving` is checked out in another worktree) | green | `73b14c327`, 07:53Z | see cache table |
| 1 | `9c62b209f` | S1 owner-act receipts, observe mode | green | `9c62b209f`, 08:05Z | proxy `owner-act key host-owner-act-4323d8e7e60998de loaded`; corpusd `owner acts: mode=observe`; proxy and corpusd active; `https://choir.news/` 200 |
| 2 | `2eae77de6` | P5 host build and layered apply. The merge broke the build (`restoreRestartBusy` called the old `plannedRestartBusy` signature); fixed in the same push | green | `2eae77de6`, 08:23Z | corpusd `host build: enabled (source /opt/go-choir, installable .#autoputer)`; tracer `p5-go-served-0825` FAIL, did not reach apply (below) |
| 3 | `f8007f5d6` (code `978a18d4e` plus final branch head `19a19425d`) | Autopaper section pages: corpusd front list takes `?section=`, front-importance keeps score rows past the newest 100, `-pages N` backfill flag, frontend section pages | green (both pushes) | `f8007f5d6`, 09:54Z | see Section pages |
| 4 | `97e023100` | P4 standing consensus on P5 (nothing autonomous until armed) | CI green; **deploy FAILED** (incomplete) | proxy serves `97e023100`; `/health` deployed_commit still `f8007f5d6` | see Deploy failure; autonomous tracer owed (steps 0 and 2 did not pass; coordinator asked for no more self-development tracers) |

## Tracer results

- `selfdev-jobs-0654` (Go drill, build `f704110ce`): `pass=false`,
  `reached=engineering_final_report`. The engineering assignment ran 22
  minutes and was cancelled with no bundle. A capsule cell hit its 30 s timeout
  (`broker.sock: i/o timeout`) and the next call hung 13 minutes. This is the
  jobs-0101 item 2 class (capsule broker RPC desync). Correction from the
  coordinator: the broker repair IS on main (`b4f778552`, the client drops its
  connection after any transport or framing error and re-dials), so this
  timeout is a remaining or different problem; another agent is tracing
  "engineering never freezes" from the run ledger (see
  `docs/problems/` ledger traces), and no more self-development tracers were
  run. The
  raced-activation fix worked: `recovered raced activation` appeared, no
  `dispatcher: poison`, no `mutation authority mismatch`. Problem doc:
  `docs/archive/problems/selfdev-jobs-0654-raced-activation-verified-engineering-budget-2026-10-15.md`.
  Judged not a new failure class, so landing continued.
- `go-served-1143` (build `7b8944f03`, after the broker re-dial and write-replace
  fixes): **first full self-development loop on staging**: reached `restore`,
  17 of 18 checks passed (Freeze, verify, owner approval, layered apply with one
  planned restart, route receipt, restore, probe gone); only `go_test_job_done`
  failed (jobs count 3, done 2). Problem doc:
  `docs/archive/problems/go-served-1143-first-full-selfdev-loop-test-job-not-done-2026-10-15.md`.
- `p5-go-served-0825` (P5, build `2eae77de6`): FAIL, `pass=false`,
  `reached=engineering_final_report`. Three engineering assignments ended
  `failed` with no bundle (Texture re-cast through round 5); no poison, no
  broker timeout. P5's apply path was therefore not exercised; P5 is deployed
  and dormant. Problem doc:
  `docs/archive/problems/p5-go-served-0825-engineering-never-freezes-2026-10-15.md`.
  Reasons for the failed assignments not read from the ledger (hypothesis:
  jobs-0101 A1-A3 class).

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

## Section pages (step 3) and backfill

Merged `fix/autopaper-section-pages`. After the deploy,
`/api/platform/publications/front?section=us_politics&limit=20` returns 20
stories including the Milei midterms story and the Fox News approval poll (12
before the frontend/backfill, from the newest page only). One-off backfill on
Node B exactly as in the problem doc (`-pages 5 -max-calls 500 -budget 30m`
temporary drop-in, `start --wait`, drop-in removed, timer restarted): `500
stories, 102 kept, 398 scored, 0 failed`, 60 s. Unfiltered front list, first 530
rows, section counts before -> after: tech_ai 27 -> 225, business 16 -> 52,
world 47 -> 171, us_politics 12 -> 52, no section 428 -> 30. Page 1 (newest 100)
unchanged (tech_ai 25, business 16, world 47, us_politics 12). The 10:05 timer
run's carry-forward was not re-checked.

## Deploy failure of P4 (97e023100)

CI run 38717028854 deploy job failed: nixos switch took about 29 min under load
(host load 11-12), then vmctl exited status 1 every 30 s from 10:42:23 to
10:45:43 on `ping ComputerVersion route database: context deadline exceeded`
(7 restarts, start-limit-hit); a start at 10:47:44 was healthy. Staging
`/health` was `degraded` / 502 for about 5 minutes and is `ok` now; the paper
computer kept running and was reattached. Not a P4 code failure: the platform
Dolt server had deadlocked from about 09:49 (0% CPU, queries never returned),
which hung checkpointd, held the switch, and made vmctl's ping time out; the
coordinator restarted it at 10:47:27. Problem doc:
`docs/archive/problems/platform-dolt-deadlock-hangs-checkpointd-and-crashloops-vmctl-2026-10-15.md`.
After the reboot the deploy is rerun for all of main.

## Locally validated, not yet deployed

Merged on main locally and pushed by the coordinator in `a3481814d` (its CI run
38720298222 was cancelled for the reboot): ops/patch-lanes (`01d8c5981`),
land/memory-hotplug (`ce959b2db`; one conflict in `internal/agentcore/api.go`
resolved by keeping both the `EffectiveRelease` and `Memory` fields),
fix/exa-search (`5d16af67c`). Build clean; focused tests pass
(capsule, capsulejob, autoputer, hostboot, yaegikernel, vmctl, vmmanager,
gateway, search, agentcore job/meminfo); both workflow contract tests pass.
Hotplug probe (`scripts/memory_hotplug_probe.py`) and Exa proof (`exa=` in
gateway journal) are owed after the deploy. account-recovery merges cleanly onto
main (owner ruling: land it).

## Owner rulings recorded

- Land `land/account-recovery` (supersedes the earlier delete ruling; do not
  delete `/auth/recovery/*`).
- Keenable stays on free-tier pacing.
- Fix Exa (`fix/exa-search`; per-day cap of 150 stays).
- Halve-compute ruling: do not economize on search; keep append-only prompts.

## Paper health

Paper booted on `2eae77de6` (epoch after each deploy), `status=ready`,
persistent disk 35.7% used (about 16 GiB dolt), below the 75% stop line; no
deploy-time regressions seen.

## Later landings (afternoon) and where it stopped

| step | commit | result |
|---|---|---|
| main with P4 (rerun), patch lanes, hotplug, Exa | `3327ee047` | green, deployed, staging ok |
| capsule broker re-dial + write-replace fix | `a64f28e1f` | green, deployed |
| Dolt guardrails (checkpointd HOME, ctx open, vmctl ping retry, liveness probe) | `7b8944f03` | green, deployed; probe timer active, ok probes; checkpointd no longer panics and publishes checkpoints, but still exits 1 because one computer's replay stops at a commitment record with no verified identity formula (`docs/archive/problems/checkpointd-replay-upcast-missing-identity-formula-2026-10-15.md`) |
| cleanup/delete-choir-assign | within `3327ee047` | green |
| account recovery | `13380efc7` | green, deployed; `POST /auth/recovery/request` for an unknown address returns 200 `{"ok":true}`, auth logs `user_not_found` and no token text |
| Dolt compaction (paper only) | `c92746ed0` | **CI red**: heresy detector I4 (`CALL DOLT_RESET` in `internal/store/dolt_compaction.go`); not deployed (`docs/problems/dolt-compaction-trips-heresy-i4-ci-red-2026-10-15.md`) |

Tracer `go-served-1143` (build `7b8944f03`): first full loop, 17 of 18 checks.
Memory hotplug probe: refused ("computer is not selected for memory hotplug");
the feature is flag-off by design, so there is no hotplug proof yet.
(Corrected 2026-10-21, FP8: the flag was on for class:test from c9621a721.
The refusal is the probe's own message; the likely cause is that vmctl read
the hotplug selection before refreshing the test tier, so a freshly minted
test account cold booted without the device. See P5 in
`docs/archive/definitions/choir-launch-fp8-image-jailer-2026-10-21.md`.) Exa: still
`exa=error` in the gateway search summary after the deploy (4 lines, from the
tracer computer's searches); the failing step is not named in the summary line;
cap 150 stays.

**Open concern:** the paper made no inference calls after 10:56 (stories stop at
10:55); see `docs/archive/problems/paper-silent-since-10-56-after-host-reboot-2026-10-15.md`.

P6-P8 (earlier note, now resolved as above): merging `feat/p6-p8-release-adopt` conflicts in six red files
(`agentcore/api.go`, `api_self_development.go`, `platform/handlers.go`,
`proxy/self_development.go`, `vmctl/handlers.go`, `scripts/reeng_tracer.mjs`)
and, beyond textual conflicts, P5 and P6 both define a different
`effective_release` block under the same JSON key and struct field in
`/health` and observability (P5: startup release, `effectiveReleaseIdentity`;
P6: live identity, `EffectiveRelease`); reconciling them is a design decision
on a red surface.

## Final landings (evening)

| step | commit | result |
|---|---|---|
| paper lost-wake fix | `964c598ca` | green, deployed 13:14Z; paper booted planned; probe timer active with ok lines; 28 schedules active and 0 overdue; paper inference continuous |
| P6-P8 merge (option (a)) | `ff01e0a2f` | CI red: data race in the startup-manifest fields (`docs/archive/problems/p6-effective-release-report-data-race-ci-red-2026-10-15.md`) |
| startup-manifest mutex | `e485b1966` | green incl. race shards, deployed 13:59Z; `/health` on the paper shows the P6 `effective_release` (serving_release_digest, event_head, code_ref, build_commit) |

Tracer `adopt-1400` (build `e485b1966`): publisher applied; tampered and
unpublished records refused unchanged; adoption named release and trajectory;
adopter applied after one planned restart; `adopter_serves_the_change` and
`identity_names_adoption` read false 0.15 s after applied, probably too early
(`docs/archive/problems/adopt-1400-adopter-applied-served-and-identity-read-too-early-2026-10-15.md`).
Note: the paper's identity block carries `tree_error: commit e485b19… not in the
source repository` (the paper has no checkout of the new commit).

Tracer `adopt-1418` (same build, polling tracer): not a passing adopt loop. The
publisher applied and both negative cases were refused unchanged, but the
adopter's desks never opened an operation (one engineering assignment failed in
about 2 minutes, Texture closed `done`), so the polled checks never mattered
(`docs/archive/problems/adopt-1418-adopter-desks-finish-without-an-operation-2026-10-15.md`).

## Afternoon landings (after the adopt drill)

- **First passing P6-P8 adopt loop: `adopt-1523`** (staging `d43dd6301`, after the
  engineering release-copy fix `fix/engineering-release-copy`): `pass: true`,
  `reached: adopter_applied`, no failed checks. The adopter froze, was approved,
  materialized and applied in about 30 s; `adopter_served` held at 81 ms and
  `identity_names_adoption` after 26 s of polling. On the adopter's console: no
  "preserving permissions" line, no failed assignment, no poison, no `not a
  directory`. Both computers stopped.
- **Dolt 2.4.2 for the sql-servers** (main `b92ead151`, deployed 15:40Z): cherry-picked
  `d92dbb20f`, `ca5f0f5e2`, `d893bc998`, `bcd0b458a` only (no embedded Go module bump).
  `nix eval` and `systemctl show` both name `dolt-2.4.2` for the platform and corpus
  servers; both active; liveness probe 31 clean runs in 16 min, no alert; `go test
  ./internal/store -run Texture` passes; staging ok, vmctl ok; checkpointd now runs clean
  (`Deactivated successfully`, 76-88 ms, no panic or exit 1). The CI SBOM candidate job hit a
  network flake (`proxy.golang.org` stream error), was rerun once and passed.
- Paper cache ratio after the frame.go change: 72-76%, but it was already 76% before it
  (`docs/archive/problems/paper-chatgpt-cache-ratio-below-85-after-restarts-2026-10-15.md`).

## Skipped or pending (as of writing)

- Re-run P4's deploy; then patch lanes + hotplug + exa (already on main),
  recovery, Dolt compaction (paper only), P6-P8 (adopt tracer owed until
  engineering freezes again): all after the host reboot.
- Dolt compaction, P6-P8
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
