# Effects capsule broker bash job-control, missed deploy, and live re-probe — 2026-08-19

**Boundary:** diagnose then re-probe. Not freeze. Not promote.
No live send. Do not restore. Do not self-promote.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Mutation class:** red — capsule exec broker, guest boot closure, CI deploy
impact vs last-deployed SHA. This receipt is docs-only; the `sh -c` repair
already existed as `651d86bc`.

## Failure (guest `d1763eef`)

Staging `/health` at the failure reported proxy
`deployed_commit=d1763eef4f396b46c9013c612bf6a5d5f70c6673`
(`deployed_at` 2026-08-19T13:04:11Z). Node B git HEAD matched that SHA.

Retained computer `computer-03335285269bdba4f94377e56879f9e6` was active at
realization epoch 325, mode `propose_only` generation 1. Pre-A checkpoint
`99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7` remains the
published restore fence. Effects remain OFF.

Texture Super `48fa7e72-f1de-490b-ab5a-737e2dff39ba` completed at
`2026-08-19T13:15:24Z` after binding implementation CoSuper
`assignment-03a70627-541f-5050-b946-19a3c77bbb69` for operation
`selfdev-3f842968bd296fcc85400d53ce1023a6`. Historical operation
`selfdev-b090bcd72d300fed17cb3f5a142f8595` is still `executing` with empty
`bundle_digest` (residue, not the live assignment).

CoSuper `run:assignment-03a70627-541f-5050-b946-19a3c77bbb69` completed at
`2026-08-19T13:27:24.771788982Z` with no freeze and no proposal. Every
`capsule_exec` failed:

```text
bash: initialize_job_control: getpgrp failed: Success
```

The CoSuper reported the shell as a fundamental blocker and stopped. No
SourceTreeRef, BuildRecipeRef, TestReceipts, RuntimeArtifactRef, or
DependencyToolchainRefs were bound. No mail. No promotion.

## Root cause

Deployed `cmd/capsule-broker` on `d1763eef` still prefers `bash` and passes
`+m` as a separate argv (`bash --noprofile --norc +m -c …`). Inside the
capsule PID namespace, bash still calls `initialize_job_control`; `getpgrp`
fails and every exec returns that error. `+m` does not prevent the startup
path.

Source repair already existed on `main` as `651d86bc`
(`fix(capsule-broker): use sh -c and clean shell args without +m for
non-interactive exec`): prefer `sh -c`, drop `+m`. It was not on the guest
until the forced deploy below.

## Why the repair did not land on first CI

| SHA | CI | Deploy |
|---|---|---|
| `651d86bc` | run 32258368089 **failure** | none |
| `495f147c` (test-only) | run 32259223519 **cancelled** | none |
| `d33f245c` (CI shard timeout script) | run 32260671043 **success** | **skipped** |

`deploy-impact` on push compares `github.event.before` to `github.sha`. For
`d33f245c` that range is only:

- `internal/agentcore/api_self_development_test.go` (ignored)
- `scripts/go-test-non-runtime-shards` (ignored)

The classifier therefore set `deploy_needed=false` even though `651d86bc`
(`cmd/capsule-broker/main.go`) sits between last-deployed `d1763eef` and HEAD.
Local replay of the same three paths against the classifier confirms
`cmd/capsule-broker/main.go` would have selected host + guest boot refresh.

First forced recovery `gh workflow run CI --ref main -f force_staging_deploy=true`
as run `32277975820` hung on Heresy Detector “Install ripgrep” and was
cancelled. Second dispatch `32280364496` succeeded including Node B deploy.

## Repair in source (now deployed)

`cmd/capsule-broker/main.go` `handleExec`:

1. Prefer `sh -c` when `sh` is on PATH.
2. Bash fallback args are `--noprofile --norc -c` only (no `+m`).
3. Last resort `/bin/sh -c`.

This is not a new code change in this receipt.

## Live identity after force-deploy + refresh

Observed 2026-08-19T17:43Z–17:57Z:

- Staging `/health` `build.deployed_commit=d33f245c3e9bf0ec9bfb72451eb275b07acddbaa`
  at `2026-08-19T17:43:06Z`.
- Owner refresh idempotency `effects-capsule-sh-refresh-2026-08-19T17:43Z`
  advanced retained computer epoch **325 → 326**. LifecycleReceipt
  `01a01b1f-699f-719a-a95d-2c64603691c6`.
- Mode remains `propose_only` generation 1, ModeReceipt
  `01a0091b-bf12-771e-97e7-9a42752ad036` (idempotency
  `effects-red-propose-only-2026-08-16T05:45Z`).
- Prior CoSuper `run:assignment-03a70627-541f-5050-b946-19a3c77bbb69` cancelled
  at refresh `2026-08-19T17:43:56Z`.

Start POST without `mode_receipt` returned HTTP 409
`current signed mode does not authorize proposal`. Start POST with the current
signed ModeReceipt, prompt, and idempotency
`effects-solitaire-start-2026-08-19T17:44Z` returned 201.

Live operation (not residue):

- `selfdev-ccf0f1ec0e851750f253fe5f5ed97974`
- trajectory `trajectory-539a7bc96f058c6209e187d5987b697b`
- request_commitment `ef1bca527ddc4914b25cebbdf4a80db4716185d9e5e1570d332b93f5c7dce555`
- prompt_artifact_ref `artifact:sha256:d172677707c588690c92d3089b64e4e8df470c10f9e9640c5af6796353730cef`
- state `executing` at GET 2026-08-19T17:56Z; no `bundle_digest`

Runs:

- Super `f009f383-c31c-41ca-8d87-6ea2e6deb581` **completed** 2026-08-19T17:45:27Z
  after `assign_co_super` of assignment `assignment-97191e37-657c-5acf-af18-f1c80d09def2`.
- CoSuper `run:assignment-97191e37-657c-5acf-af18-f1c80d09def2` **running** since
  2026-08-19T17:45:16Z; capsule `capsule-c5e35066-0930-5f35-9471-a891cf541ad6`.
- Texture `d0502969-a820-5540-932d-6088b74bb8dd` **running**; document
  `c273a57b-a253-5234-888d-6139024a6cf1` still `current_version_number=0`
  (created 17:45:04Z). Supervision revision while work is open remains unpaid.

Guest journal (vmctl console, PID 1046): tool loop continued at ~1 tool / 3–4s
with **no** `initialize_job_control` / `getpgrp`. File writes previously
succeeded on the broken bash path; this re-probe is the first live exec path
on `sh -c`.

The loop did not freeze. At 2026-08-19T17:57:48Z CoSuper
`run:assignment-97191e37-657c-5acf-af18-f1c80d09def2` became `cancelled` with:

```text
tool loop: exceeded 200 iterations without end_turn
```

`internal/toolregistry/toolloop.go` `maxToolLoopIterations = 200` is a
temporary stability ceiling. Capsule authorship of solitaire exhausted it
without `end_turn`, freeze, or proposal. No five bundle refs.

As of 2026-08-19T18:00Z:

- Super `f009f383` remains `completed` at 17:45:27Z (it ended after
  `assign_co_super` with `work_disposition=open`).
- Texture `d0502969` remains `running` since 17:45:04Z; document
  `c273a57b` still `current_version_number=0`.
- Operation `selfdev-ccf0f1ec` still `executing`; no `bundle_digest`.
- No Super rewake / attempt-2 CoSuper in `/api/runs`.

Raising the 200 cap and Super-start-from-scratch are unpaid. Document this
failure class before any repair-code commit.

## Residual (unpaid code)

1. `deploy-impact` still classifies the last push range, not
   `/health deployed_commit`..HEAD.
2. `maxToolLoopIterations = 200` aborts capsule authorship before freeze.
3. Super completed after assign and did not rewake on CoSuper cancel; Texture
   remains running with no supervision revision.

Do not patch in this docs receipt.

## Forbidden until freeze

- freeze / propose / promote from this host
- live send
- restore
- Super start-from-scratch until Super-non-rewake is diagnosed in a receipt
- raising `maxToolLoopIterations` before that diagnosis
- SQL-empty or replace the retained computer

## Rollback

No product-state rollback in this receipt. `git revert 651d86bc` would restore
the bash `+m` path. Checkpoint `99949fe2` remains the pre-A fence.
