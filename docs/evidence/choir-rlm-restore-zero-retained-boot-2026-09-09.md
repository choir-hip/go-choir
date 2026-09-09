# Restore-Zero: retained-store boot of snapshotting contract — 2026-09-09

Class: green (problem-closed verification; no further runtime mutation in this receipt).
Authority: `docs/definitions/choir-rlm-restore-zero-2026-09-08.md`.
Source identity: `9341b5d18d5b962a4440671233efd402038a8b8a`.

This is the owner-reachable retained-store proof. Guest `/health` `ready` is supporting context, not the acceptance line.

## Actuation (product path)

CI deploy refresh was not used. G4 constructed-computer skip was not weakened. SSH `POST /internal/vmctl/refresh` was not issued. `recover_current` was not issued.

Owner-scoped product path:

```text
POST https://choir.news/api/computers/computer-03335285269bdba4f94377e56879f9e6/lifecycle/refresh
{"idempotency_key":"restore-zero-9341b5d1-owner-refresh-20260909"}
```

- Started: `2026-09-09T08:16:45Z`
- HTTP 201 at `2026-09-09T08:17:00Z` (elapsed 15.1s)
- `receipt_kind=LifecycleReceipt`, `action=refresh`
- `prior_realization_epoch=893` → `resulting_realization_epoch=894`
- `prior_lifecycle_state=active` → `resulting_lifecycle_state=active`
- `receipt_id=01a0853d-e18f-74a3-8170-8fa77ca8e183`
- `canonical_payload_sha256=8b131d274279526cb47dd17a0834144a9e631f5a4c1381b1c7967cd57e9228f1`

GET `/api/computers/.../lifecycle/status` after actuation: `state=active`, `realization_epoch=894`.

## Computer after refresh

| field | value |
| --- | --- |
| computer_id | `computer-03335285269bdba4f94377e56879f9e6` |
| vm_id | `candidate-fleet-e15cb89f25d963c220319b7b` |
| snapshot_kind | `constructed-computer-version` (identity preserved) |
| epoch | **894** |
| computer_url | `http://10.200.12.2:8085` (moved off `10.200.6.2`) |
| guest commit | **`9341b5d18d5b962a4440671233efd402038a8b8a`** |
| deployed_at | `2026-09-09T08:16:51Z` |
| status / runtime_health | `ready` / `ready` |
| persistent disk used | `16605167616` (~15.5 GiB; retained, not sparse blank) |

W and H unchanged through this boot:

- W=`148431`, `base_ref=6099cf6998ac87c7e763cdbbb4e2ae051ae097815e49bf795dc5b008d61fc599`
- H=`148431`

## Acceptance line (PlanRecovery + reconstruct)

`journalctl -u go-choir-vmctl` for this Firecracker console:

```text
2026/09/09 08:16:59 autoputer: projection recovery resume for computer-03335285269bdba4f94377e56879f9e6 (local=148431 W=148431 H=148431 tail=0)
2026/09/09 08:16:59 autoputer: computer event authority reconstructed
2026/09/09 08:17:00 vmctl: refreshed VM candidate-fleet-e15cb89f25d963c220319b7b for user 5bd6de97-3b58-408c-bf89-c42c81b083de desktop primary (new_epoch=894, deploy-image-refresh)
```

Interpretation against the Restore-Zero contract:

- Decision: **resume**, not rebase, not genesis, not refuse.
- `local = W = H = 148431`, `tail = 0`.
- Reconstruct completed immediately after the resume line.
- No `replay page fetch after=` lines in this boot window (prefix enumerated/fetched/reduced for seq ≤ W = 0; applied = H−W = 0).
- No `ProjectionBase rebased`, `required projection base refused`, `projection recovery genesis`, or `after=0` lifetime replay for this computer.

That is prefix=0 retained-store recovery on the snapshotting binary.

## Explicitly not this proof

- Host/proxy SHA `9341b5d1` without this guest boot (already true before 08:16Z).
- Genesis guest `computer-bb0f4fa5` (`local=0 W=0 H=0`).
- W=1 / W=13 publication, HTTP 200 watermark advertisement, or guest `/health` `ready` alone.
- Proxy 502 `failed to resolve user autoputer` / boot-HTML slice (deferred).
- Periodic host rebuild to keep W within 10000 of future H (follow-on S1-B).
- Action 8 scoped failure-injection controls (still a recorded blocked prerequisite).

## Rollback

Docs-only for this receipt. Runtime rollback would be another owner-scoped refresh onto a prior guest image; not performed. Advertised W=148431 and retained disk are unchanged by this documentation.

## Heresy delta

- discovered: 0
- introduced: 0
- repaired: retained owner computer lifetime-replayed because boot skipped a nonempty store; owner-scoped refresh of `9341b5d1` resumed at W=H with tail=0.
