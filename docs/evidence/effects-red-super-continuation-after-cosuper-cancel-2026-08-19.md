# Effects Super continuation after CoSuper cancel — 2026-08-19

**Boundary:** deployed re-probe. Not freeze. Not promote. No live send.
No restore. No HTTP Super-start / operations POST. No
`maxToolLoopIterations` patch.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Repair:** `9bc99f90` (`fix(agentcore): continue Super on CoSuper
system-cancel producer_report`), deployed as HEAD `51b18f54`.

## Deploy identity

CI push run https://github.com/choir-hip/go-choir/actions/runs/32287354580
succeeded, including Deploy to Staging (Node B) job `96185841995`.

Staging `/health` at 2026-08-19T18:57Z:

- proxy `deployed_commit=51b18f5440d9b3acec2713f71786db695263c37c`
- `deployed_at=2026-08-19T18:54:54Z`
- `vmctl_status=ok`

Deploy-impact on that push classified `deploy_needed=true`,
`host_services=gateway,autoputer`, `deploy_vmctl_restart=true`,
`deploy_active_vm_refresh=true` because `github.event.before` was
`a618a03d` (the docs commit that cancelled the `9bc99f90` CI run), so
the range included the agentcore repair.

Guest image pointer updated:

```text
go-choir guest image pointer updated to /nix/store/lfda1znx02pcni7p8d6dihirbx32bgzl-go-choir-guest-image
```

Deploy then printed `No mutable active interactive computers need
refresh` at 18:54:50Z. Retained computer stayed epoch **326** with
autoputer still `d33f245c` (dual `x-choir-build-commit` headers:
proxy `51b18f54`, autoputer `d33f245c`).

## Owner refresh (guest image changed)

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer refresh \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --idempotency-key effects-super-continuation-refresh-2026-08-19T1857Z
```

LifecycleReceipt `01a01b63-d244-7f3d-afa3-728c0ba6bc55`:
`active@326` → `active@327`. No HTTP operations POST.

Authenticated `GET https://choir.news/` with
`X-Choir-Computer: computer-03335285269bdba4f94377e56879f9e6`
returned HTTP 200; both `x-choir-build-commit` headers are
`51b18f5440d9b3acec2713f71786db695263c37c` (`proxy` and `autoputer`).

Mode remains `propose_only` generation 1, ModeReceipt
`01a0091b-bf12-771e-97e7-9a42752ad036`. Effects remain OFF.
Pre-A checkpoint `99949fe2` remains the restore fence.

## Super continuation (no HTTP Super-start)

Boot reconcile on the new guest started persistent Super without a
new Texture `execution_request` and without HTTP operations POST.

| Actor | ID | State | Times |
|---|---|---|---|
| Super continuation | `b57705fd-6e39-4fc6-9a2a-4aa8f0caac3d` | `running` | created 18:58:39Z; updated frozen 18:58:42Z as of 19:06Z |
| Texture rewake | `88fd1e14-c3f9-48af-aa10-91aa3b72f6df` | `passivated` | 18:58:41Z–18:58:49Z |
| Prior CoSuper | `run:assignment-97191e37-657c-5acf-af18-f1c80d09def2` | `cancelled` | 17:45:16Z–17:57:48Z; 200 iterations |
| Prior Super | `f009f383-c31c-41ca-8d87-6ea2e6deb581` | `completed` | 17:45:04Z–17:45:27Z |
| Prior Texture | `d0502969-a820-5540-932d-6088b74bb8dd` | `passivated` | created 17:45:04Z; passivated 18:58:39Z |

Continuation Super metadata (verbatim fields):

- `request_source=lifecycle_texture_control`
- `requested_by_agent_id=co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2`
- `requested_by_profile=co-super`
- `lifecycle_work_item_id=fd43ecca-cb82-53cf-91b5-dbe6f2412f97` (same Super work as `f009f383`)
- prompt: `Process pending coagent update packets for privileged execution.`
- model `deepseek-v4-flash`

No later CoSuper run exists in `/api/runs`. Operation
`selfdev-ccf0f1ec0e851750f253fe5f5ed97974` remains `executing` with
empty `bundle_digest`; `updated_at` still 17:45:05Z.

Texture document `c273a57b-a253-5234-888d-6139024a6cf1` advanced
`current_version_number` 0→1 at 18:58:47Z (revision
`02b3917a-cd48-5742-a250-3c780d213587`). Prose is still
`Supervise self-development on this computer.` The Texture edit
rationale claims Super bound a fresh CoSuper assignment; `/api/runs`
does not show one. That rationale is not a freeze or assignment
receipt.

## What this proves

The 9bc99f90 join is live: a pending admissible CoSuper
`producer_report` woke persistent Super after guest refresh onto the
repair, without HTTP Super-start.

## What this does not prove

- Super `b57705fd` has not terminalized. As of 19:06:56Z it is still
  `running` with no `input_tokens`/`output_tokens` and `updated_at`
  frozen at 18:58:42Z (~8 minutes). First-token gateway hang is
  unpaid diagnosis, not a reason to HTTP Super-start.
- No new CoSuper assignment. Capsule authorship of solitaire is
  still unpaid. `maxToolLoopIterations=200` remains.
- Freeze, five bundle refs, proposal, consensus, promotion, mail,
  restore remain unpaid.

## Residual

- Deploy-time active VM refresh skipped this computer (`No mutable
  active interactive computers need refresh`) even though the guest
  image pointer changed. Owner refresh recovered it.
- `deploy-impact` still classifies last push, not last-deployed SHA.
- Super `b57705fd` later failed at 200 iterations (19:11:37Z) without a
  new CoSuper; Super `999bd208` started one second later from the same
  pending report. See
  `docs/evidence/effects-red-super-continuation-storm-after-cosuper-cancel-2026-08-19.md`.

## Forbidden

- freeze / propose / promote
- live send
- restore
- HTTP Super-start / operations POST
- raising `maxToolLoopIterations` while Super has not bound a new
  assignment
- SQL-empty or replace the retained computer

## Rollback

Refresh receipt `01a01b63` is a forward record. Checkpoint `99949fe2`
remains the pre-A fence. This file is docs-only.
